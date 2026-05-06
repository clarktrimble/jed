# swarm

Deploy rendered Jed specs to Docker Swarm via the Docker socket API.

## Usage

```go
sw := swarm.New(client, logger) // client implements swarm.Client

// Setup
sw.CreateNetwork(ctx, "svc-net", true, true)         // attachable, encrypted
sw.CreateSecret(ctx, "db_password", []byte("hunter2")) // creates db_password_v1

// Deploy from stored Jed state
store, err := bbolt.New("jed.db")
j, err := jed.New(ctx, store, "deploy-vars")
spec, err := j.Spec(ctx, "myapp")
body, err := sw.Spec(ctx, spec) // optional: inspect exact Docker service payload
_ = body
id, created, err := sw.Deploy(ctx, spec)
_ = created

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
j, err := jed.New(ctx, store, "deploy-vars")
spec, err := j.Spec(ctx, "myapp")
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
