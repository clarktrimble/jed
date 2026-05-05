# Store Interface Refactor

## Summary

Consolidated ServiceStore and EnvStore into a single Store interface.

## Changes Made

### Core Interface (jed.go)
- Removed separate `ServiceStore` and `EnvStore` interfaces
- Added unified `Store` interface with methods:
  - GetService, SetService, DelService, Services (returns []Service)
  - GetEnv, SetEnv, DelEnv, Envs (returns []Env)
- Updated `Jed` struct to use single `store Store` field
- Updated `New()` signature to take single `Store` parameter

### Implementation (store/bbolt)
- Created bbolt-backed Store implementation
- Uses two buckets: "services" and "envs"
- JSON serialization for both Service and Env structs
- Full test coverage in bbolt_test.go

## Known Issues / TODOs

1. **GetEnv error handling in Services()**
   - `store.GetEnv()` returns error when env not found
   - `Services()` currently ignores all GetEnv errors
   - Need to distinguish "not found" from actual errors
   - Consider: return empty Env{} instead of error for not found case

2. **GetEnv method API friction**
   - `Store.GetEnv()` returns `(Env, error)` with Env.Vars containing the map
   - `Jed.GetEnv()` returns `(map[string]string, error)` for backward compat
   - Currently extracting `.Vars` from Env struct
   - Consider: change Jed.GetEnv to return Env, or change Store interface

## What Still Needs Updating

### Test Files
- `jed_test.go` - Update all `cfg.New()` calls to pass single Store
- `container_test.go` - Update `cfg.New()` calls
- `helper_test.go` - Update or create unified test store implementations
  - MemoryServiceStore and MemoryEnvStore need to be combined into MemoryStore
  - Or create adapter that wraps both into single Store interface

### Documentation
- `design.md` - Update to reflect single Store interface
- `README.md` - Already uses design.md link, should auto-update

## MCP Diagnostics Usage

When refactoring, use MCP gopls diagnostics instead of `go build`:

```
mcp__gopls-mcp__go_diagnostics
```

This provides better error messages and doesn't require bash execution.
