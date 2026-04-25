# swarm

Deploy services to Docker Swarm via socket API.

## Usage

```go
sw := swarm.New(client) // client implements swarm.Client interface

// Setup
sw.CreateNetwork(ctx, "svc-net", true, true) // attachable, encrypted
sw.CreateSecret(ctx, "db_password", []byte("hunter2")) // creates db_password_v1

// Deploy (creates or updates)
store, err := bbolt.New("jed.db")
svc, err := store.GetService(ctx, "myapp")
env, err := store.GetEnv(ctx, "myapp")
id, err := sw.Deploy(ctx, svc, env)

// Inspect
services, err := sw.ListServices(ctx)
tasks, err := sw.ServiceTasks(ctx, "myapp")
logs, err := sw.TaskLogs(ctx, tasks[0].ID, "100")
state, err := sw.Status(ctx, "myapp") // "stopped", "running", or "error"

// Teardown
sw.DeleteService(ctx, "myapp")
```

See [store README](../store/README.md) for store docs.

## Secrets and Configs

Swarm secrets and configs are immutable. This package uses versioned naming (`db_password_v1`, `db_password_v2`, etc.):

```go
sw.CreateSecret(ctx, "db_password", []byte("hunter2"))  // creates db_password_v1
sw.CreateSecret(ctx, "db_password", []byte("hunter3"))  // creates db_password_v2

sw.CreateConfig(ctx, "app_config", []byte("key=value")) // creates app_config_v1
```

In `jed.Service`, list base names in `Secrets` and map base names to mount paths in `Configs`. Deploy resolves both to the latest version automatically.

## Defaults and Hardcoded

| Setting          | Value                                          |           |
|------------------|------------------------------------------------|-----------|
| Resources        | 0.5 CPU / 128MB limit, 0.1 CPU / 64MB reserve  | default   |
| User             | 1001                                           | default   |
| Replicas         | 0 (stopped)                                    | default   |
| Root filesystem  | Read-only                                      | hardcoded |
| Restart          | on-failure, 5s delay, max 3 attempts           | hardcoded |
| Update order     | stop-first, rollback on failure                | hardcoded |
| Log driver       | json-file, 10MB max, 3 files                   | hardcoded |

See [service-yaml.md](../service-yaml.md) for `jed.Service` field reference.
