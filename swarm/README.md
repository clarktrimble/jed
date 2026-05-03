# swarm

Deploy services to Docker Swarm via socket API.

## Usage

```go
sw := swarm.New(client, logger) // client implements swarm.Client interface

// Setup
sw.CreateNetwork(ctx, "svc-net", true, true) // attachable, encrypted
sw.CreateSecret(ctx, "db_password", []byte("hunter2")) // creates db_password_v1

// Deploy (creates or updates)
store, err := bbolt.New("jed.db")
svc, err := store.GetService(ctx, "myapp")
env, err := store.GetEnv(ctx, "myapp")
globalEnv, _ := store.GetEnv(ctx, "_global")
id, err := sw.Deploy(ctx, svc, env, globalEnv.Vars)

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
| Restart          | none by default; optional condition, 5s delay  | default   |
| Update order     | stop-first, pause on failure                   | hardcoded |
| Log driver       | json-file, 10MB max, 3 files                   | hardcoded |

See [service-yaml.md](../service-yaml.md) for `jed.Service` field reference.

## Template Expansion

Command args and labels support `{{VAR}}` expansion at deploy time. Variables are resolved from service env vars merged with global template vars (passed as the fourth arg to `Deploy`). Service env wins on collision. Global vars are not passed to the container.

```yaml
# service YAML
command:
  - "prometheus"
  - "--web.external-url=https://{{VHOST}}/prometheus"
labels:
  prometheus_host: "{{ES_HOST}}"
```

Use `_global` env in the jed store for deployment-level values:

```
jed set-env _global global.env   # VHOST=mon.example.com
```

Missing variables cause deploy to fail with a clear error. Expansion is single-pass — values are not re-expanded.
