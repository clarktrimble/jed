# Jed Design Decisions

This document explains the rationale behind key design decisions in Jed.

Todo: this doc needs a good beating :)
- rename rationale.md
- overall: no compose api, want just enough
- enumerate key design choices

## Slice-Based APIs

Methods like `Services()` and `Containers()` return slices, not maps:

```go
func (jed *Jed) Services(ctx context.Context) (Services, error)  // Returns []Service
func (jed *Jed) Containers(ctx context.Context) (Containers, error)  // Returns []Container
```

**Why slices?**
- Simpler to work with for iteration (the common case)
- The slice types (`Services`, `Containers`) provide `Find()` methods for lookups
- Avoids imposing a key structure on callers
- More flexible - can be easily converted to maps if needed

**Tradeoff:**
- Lookups are O(n) instead of O(1), but service counts are typically small

## Separated Service and Env Storage

Service definitions and environment variables are stored separately in the Store:

```go
type Store interface {
    GetService(ctx, name) (Service, error)
    GetEnv(ctx, name) (Env, error)
    // ...
}
```

**Why separate?**
- Allows updating environment variables without modifying service definitions
- Env can be updated independently (e.g., rotating secrets)
- Different access patterns: services are relatively static, env may change frequently

## Just-In-Time Env Loading

Environment variables are loaded from the Store during `Deploy()`, not stored in the Service struct:

```go
func (jed *Jed) Deploy(ctx context.Context, service Service) (id string, err error) {
    env, err := jed.store.GetEnv(ctx, service.Name)  // Loaded here
    cfg, err := service.config(env)
    // ...
}
```

**Why just-in-time?**
- Ensures latest env vars are used when deploying
- No stale data - always reads fresh from Store
- Supports workflow: update env with `SetEnv()`, then `Redeploy()` to pick up changes

**Tradeoff:**
- Extra Store query on each Deploy() call
- Deploy() depends on Store being available

## No Internal Caching

Jed maintains no internal cache of services or environment variables:

**Why stateless?**
- Configuration changes are immediately visible without needing to refresh Jed
- Simpler implementation - no cache invalidation logic
- Multiple Jed instances can coexist safely (e.g., CLI and web UI)
- Store is already optimized for reads (e.g., BoltDB caching)

**Tradeoff:**
- More Store queries (but typically not a bottleneck)
- Slightly higher latency for operations

## Random Suffix for Container Names

Deployed containers get a random suffix appended to the service name:

```
Service: "postgres"  →  Container: "postgres-k7m9x2n"
```

**Why suffix instead of prefix?**
- Keeps the service name prominent in listings and `docker ps` output
- Human-readable name comes first
- Common Docker pattern (e.g., Kubernetes pods)

**Why random instead of sequential?**
- Avoids name collisions when containers linger after failed Undeploy
- Allows fresh Deploy without cleaning up old container names
- No state needed to track "next number"

**Limitation:**
- Multiple deploys of the same service will fail due to port conflicts, even though container names are unique
- The random suffix enables unique names, not multiple instances

## GetEnv Returns Empty (Not Error) For Missing Env

```go
// Store.GetEnv returns empty Env when service has no environment variables
GetEnv(ctx context.Context, name string) (Env, error)
```

**Why return empty instead of error?**
- Not having environment variables is a valid state
- Simplifies Deploy() logic - no need to handle "not found" specially
- Distinguishes "no env" (normal) from "storage failure" (error)

**Tradeoff:**
- Can't distinguish "service has no env" from "service doesn't exist" via GetEnv alone
- Callers must check service existence separately if needed

## Store as Dependency

Jed requires a Store implementation to be passed to `New()`:

```go
type Store interface {
    // Service operations
    GetService(ctx, name) (Service, error)
    // Env operations
    GetEnv(ctx, name) (Env, error)
    // ...
}
```

**Why interface instead of concrete implementation?**
- Allows different backends (in-memory for testing, BoltDB for production)
- Users can implement custom storage (e.g., database, etcd)
- Testability - easy to mock or provide test doubles

**Provided implementations:**
- `store/memo`: In-memory store for testing/development
- `store/bbolt`: Persistent BoltDB store for production

## Known Limitations

### Multiple Deployments Not Supported

Despite unique container names, you cannot deploy the same service multiple times because:
- Port bindings conflict (host ports can only be used once)
- Volume mounts may conflict
- Network aliases would collide

The random suffix solves container name uniqueness, not resource conflicts.

### Partial Failure in DeleteService

`DeleteService()` deletes both service definition and env vars, but can leave inconsistent state:
- If service deletion succeeds but env deletion fails, env vars are orphaned
- No transaction/rollback mechanism currently

### Service and Env Kept In Sync Manually

The Store interface doesn't enforce referential integrity:
- You can set env for a non-existent service
- Deleting a service requires manually deleting env
- This is by design for flexibility, but requires careful management
