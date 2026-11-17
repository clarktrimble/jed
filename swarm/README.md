# swarm

Deploy rendered Jed specs to Docker Swarm via the Docker socket API.

## Usage

```go
sw := swarm.New(client, logger) // client implements swarm.Client

// Setup
sw.CreateNetwork(ctx, "svc-net", true, true)         // attachable, encrypted
sw.CreateSecret(ctx, "db_password", []byte("hunter2")) // creates db_password_v1

// Deploy a spec rendered by Jed (setup in the top-level README)
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

`Swarm.Deploy` does not perform template expansion. It validates the rendered
service, resolves swarm secrets/configs, builds the Docker service payload, and
creates or updates the Docker service. See
[service-yaml.md](../service-yaml.md#template-expansion) for render-template
behavior.

Use `Swarm.Spec` to inspect the exact Docker service payload without creating or
updating the service:

```go
body, err := sw.Spec(ctx, spec) // body has type swarm.Spec
```

`Swarm.Spec` still contacts Docker to resolve latest versioned secrets/configs.

Return values:

- `id`: the Docker service ID after create or update.
- `created == true`: a new service was created.
- `created == false`: an existing service was updated.

## Network Volumes

Jed resolves network volumes into `NetworkMounts` while rendering. Swarm turns
those into NFSv3 volume driver config, and Docker mounts them on the worker.
Swarm rejects a spec with unresolved `NetworkVolumes`.

Prepare the NAS shares, permissions, and worker NFS support outside Jed.

See
[network-volume configuration and naming](../service-yaml.md#network-volumes).

## Automatic Restart Policy

Jed renders an omitted restart condition as `any`. Swarm projects that condition
without defaulting it again. The policy for explicit `any` is the same as for a
condition defaulted by Jed.

Automatic restart settings belong to `swarm.Config`, not individual service
definitions:

| Field | Environment variable | Default | Applies to |
| --- | --- | --- | --- |
| `RestartDelay` | `SWARM_RESTART_DELAY` | `5s` | `any` and `on-failure` |
| `RestartMaxAttempts` | `SWARM_RESTART_MAX_ATTEMPTS` | `5` | `any` only |
| `RestartWindow` | `SWARM_RESTART_WINDOW` | `2m` | `any` only |

For example, populate default tags and environment overrides before construction:

```go
cfg := &swarm.Config{}
if err := envconfig.Process("swarm", cfg); err != nil {
    // handle error
}
sw := cfg.New(logger)
```

`RestartDelay` and `RestartWindow` are Go `time.Duration` values; environment
values use duration syntax such as `1500ms` or `2m`. `Config.New` uses supplied
values as-is, including zeros; it does not populate default tags. Zero attempts
means unlimited retries. Zero window counts all retries rather than a bounded
lookback. Zero-valued timing fields are omitted from the Docker payload.
`swarm.New(client, logger)` remains available and uses the defaults above.

With the defaults, `any` allows five restart attempts within a two-minute
lookback period per service slot. Rapid failures can exhaust those attempts
before two minutes elapse. The window is not a cooldown: an exhausted service
does not automatically resume when the window expires.

`on-failure` retains one restart attempt and no explicit window, using only the
configured delay. `none` disables automatic restart and omits delay, attempts,
and window. Use explicit `none` for one-shot services that must not repeat after
successful completion.

**Compatibility:** existing empty conditions now render as `any`; existing
explicit `any` conditions now use the bounded retry policy. Both changes take
effect on the next deployment, not immediately on running services. Service
YAML and the store schema are unchanged.

## Restarting Services Manually

`Swarm.Restart(ctx, name)` forces Docker Swarm to roll an existing service's
tasks without changing `jed.Service` or persisted store state. It reads the
current Docker service spec, increments Docker's `TaskTemplate.ForceUpdate`, and
updates the service with the current Docker service version.

Normal `Swarm.Deploy` updates preserve Docker's existing `ForceUpdate` value so
a later deploy does not accidentally reset the restart counter.

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

In `jed.Service`, list secret base names in `Secrets` and map config base names
to mount paths in `Configs`. Deploy resolves both to the latest version
automatically.

## Defaults and Constraints

| Setting          | Value                                          | Kind      |
|------------------|------------------------------------------------|-----------|
| Resources        | 0.5 CPU / 128MB limit, 0.1 CPU / 64MB reserve  | default   |
| Root filesystem  | Read-only                                      | hardcoded |
| Restart          | Jed defaults to any; 5s delay, 5 attempts, 2m window | default   |
| Update order     | stop-first, pause on failure                   | hardcoded |
| Log driver       | json-file, 10MB max, 3 files                   | hardcoded |

See [service-yaml.md](../service-yaml.md) for `jed.Service` fields.
