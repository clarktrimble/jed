# swarm

Deploy rendered Jed specs to Docker Swarm via the Docker socket API.

## Usage

```go
sw := swarm.New(client, logger) // client implements swarm.Client

// Setup
sw.CreateNetwork(ctx, "svc-net", true, true)         // attachable, encrypted
sw.CreateSecret(ctx, "db_password", []byte("hunter2")) // creates db_password_v1

// Deploy from stored Jed state
store, err := (&bbolt.Config{Path: "jed.db"}).New()
j, err := jed.New(ctx, store, "deploy-vars", logger)
spec, err := j.Spec(ctx, "myapp")
body, err := sw.Spec(ctx, spec) // optional: inspect exact Docker service payload
_ = body
id, created, err := sw.Deploy(ctx, spec)
_ = created

// Restart/roll existing tasks without changing stored Jed service config
err = sw.Restart(ctx, "myapp")

// Inspect
services, err := sw.ListServices(ctx)
tasks, err := sw.ServiceTasks(ctx, "myapp")
logs, err := sw.TaskLogs(ctx, tasks[0].ID, "100")
state, err := sw.Status(ctx, "myapp") // "stopped", "running", "pending", or "error"

// Teardown
err = sw.DeleteService(ctx, "myapp")
```

See [store README](../store/README.md) for store docs.

## Deploy Input

Swarm deploy consumes a rendered `jed.Spec`:

```go
id, created, err := sw.Deploy(ctx, spec)
```

`Swarm.Deploy` does not perform template expansion. It validates the rendered service, resolves swarm secrets/configs, builds the Docker service payload, and creates or updates the Docker service. See [service-yaml.md](../service-yaml.md#template-expansion) for render-template behavior.

Use `Swarm.Spec` to inspect the exact Docker service payload without creating or updating the service:

```go
body, err := sw.Spec(ctx, spec) // body has type swarm.Spec
```

`Swarm.Spec` still contacts Docker to resolve latest versioned secrets/configs.

Return values:

- `id`: the Docker service ID after create or update.
- `created == true`: a new service was created.
- `created == false`: an existing service was updated.

To build a spec from stored state, load render vars from whichever env name your application uses:

```go
j, err := jed.New(ctx, store, "deploy-vars", logger)
spec, err := j.Spec(ctx, "myapp")
```

## Network Volumes

Use `network_volumes` in the service declaration and global render vars `NFS_HOST` (for example `nas`) and `NFS_ROOT` (the parent path of the NAS shares, for example `/volume1`). Both NFS globals are required only for network volumes; service env values do not override them. IPv6 hosts use bare addresses without brackets. `Jed.Spec()` derives volume names and remote paths; `Swarm.Spec`/`Swarm.Deploy` emits the NFSv3 read/write driver configuration for Docker to mount on the worker. No resolved-volume records are stored.

Always render through `Jed.Spec()` or `Jed.SpecWithEnv()` first. Passing unresolved `Service.NetworkVolumes` directly to the Swarm adapter is rejected before secret/config lookups. Existing explicit volumes and local bind mounts retain their formats and behavior; explicit mounts override network targets, while local/network target conflicts are errors.

Names are `<service_lastdir, capped at 25 ASCII characters>_<6 hex hash>`, always hashed and at most 32 characters. The suffix is the low 24 bits of FNV-1a 64-bit over the full original service name, a NUL separator, and the validated/normalized full container path, formatted as six zero-padded lowercase hex digits. The same name is used for the Docker volume and the NAS share, exported at `NFS_ROOT/<name>`. `NETWORK_VOLUME_PREFIX` is no longer used. Jed detects collisions within a service; upstream provisioning must enforce share ownership by full service/path identity across services.

Prepare the NAS shares/exports, permissions, and worker connectivity/NFS support outside Jed. **This changes existing volume names and NAS paths; migrate data explicitly before redeploying.** Jed does not rename or migrate storage. This feature neither provisions nor replaces storage. Existing named-volume behavior after a root change is **uninvestigated**; do not assume configuration changes retarget existing volumes or that Jed detects mismatches. Standalone container deployment is outside this feature's scope.

See [network-volume configuration and naming](../service-yaml.md#network-volumes-swarm).

## Restarting Services

`Swarm.Restart(ctx, name)` forces Docker Swarm to roll an existing service's tasks without changing `jed.Service` or persisted store state. It reads the current Docker service spec, increments Docker's `TaskTemplate.ForceUpdate`, and updates the service with the current Docker service version.

Normal `Swarm.Deploy` updates preserve Docker's existing `ForceUpdate` value so a later deploy does not accidentally reset the restart counter.

```go
err := sw.Restart(ctx, "myapp")
```

## Secrets and Configs

Swarm secrets and configs are immutable. This package uses versioned naming:

```go
sw.CreateSecret(ctx, "db_password", []byte("hunter2"))  // creates db_password_v1
sw.CreateSecret(ctx, "db_password", []byte("hunter3"))  // creates db_password_v2

sw.CreateConfig(ctx, "app_config", []byte("key=value")) // creates app_config_v1
```

In `jed.Service`, list secret base names in `Secrets` and map config base names to mount paths in `Configs`. Deploy resolves both to the latest version automatically.

## Defaults and Constraints

| Setting          | Value                                          | Kind      |
|------------------|------------------------------------------------|-----------|
| Resources        | 0.5 CPU / 128MB limit, 0.1 CPU / 64MB reserve  | default   |
| User             | 1001:1001                                      | default   |
| Replicas         | 0 (stopped)                                    | default   |
| Root filesystem  | Read-only                                      | hardcoded |
| Restart          | none by default; optional condition, 5s delay  | default   |
| Update order     | stop-first, pause on failure                   | hardcoded |
| Log driver       | json-file, 10MB max, 3 files                   | hardcoded |

See [service-yaml.md](../service-yaml.md) for `jed.Service` fields.
