# README Sections Removed for Brevity

## Testing

- **49 passing tests**
- **85.0% coverage**
- Uses Ginkgo/Gomega for BDD-style tests
- Mock Docker client and logger
- Real Docker log data for decoder tests
- Store implementations in `helper_test.go` for testing

```bash
make test       # Run tests
make cover      # Coverage report
make lint       # Run golangci-lint
```

## Container Configuration

### containers.yaml

```yaml
- name: postgres
  image: postgres:14
  network: app-net
  restart: always
  ports:
    5432/tcp: "5432"
  volumes:
    /var/lib/postgresql/data: /data
  labels:
    app: myapp

- name: redis
  image: redis:7
  network: app-net
  restart: always
```

### .env Files

Optional per-container environment files (e.g., `postgres.env`):

```
POSTGRES_USER=admin
POSTGRES_PASSWORD=secret
POSTGRES_DB=mydb
```

## Implementation Details

### Random Suffix Naming

Containers are deployed with random 7-character suffixes to avoid name collisions:
- `postgres` � `postgres-x7y9z2n`
- Uses `hondo.Rand(7)` for generation
- Suffix pattern: `^/(.+)-[a-zA-Z0-9]{7}$` (regex for parsing Docker names with leading slash)

### Label-Based Management

All deployed containers get the `managed_by=jed` label:
- Filters container lists via Docker API
- Distinguishes jed-managed containers from others

### Log Decoding

Decodes Docker's multiplexed log format:
- 8-byte headers (stream type + size)
- Efficient buffer reuse (64KB pre-allocated)
- Supports both stdout and stderr streams

## Environment Variable Management

Environment variables flow through envStore:

```go
// Set env vars
err := j.SetEnv(ctx, "postgres", map[string]string{
    "POSTGRES_PASSWORD": "secret123",
})

// Services() automatically populates env from envStore
services, _ := j.Services(ctx)
svc := services["postgres"]  // svc.Env contains env from envStore

// Deploy uses the env
j.Deploy(ctx, svc)  // Container created with env vars

// DeleteService cleans up env
j.DeleteService(ctx, "postgres")  // Removes service + env
```

**Key Points:**
- EnvStore is required when creating Jed
- `Services()` populates `Service.Env` from envStore
- `SetEnv()` validates service exists
- `GetEnv()` returns empty map for unknown services (no error)
- Env vars stored separately from service definitions
