# Renaming Plan

## Current State → Proposed Changes

### Type Renames

| Current | Proposed | Rationale |
|---------|----------|-----------|
| `Svc` | `Jed` | Package name, clearer identity |
| `Container` | `Service` | Matches docker-compose terminology |
| `Status` | `Container` | Matches Docker API terminology |
| `Statii` | `Containers` | Map of containers keyed by service name |

## Detailed API Changes

### Types

**Before:**
```go
type Config struct{}
type Svc struct {
    client Client
    logger Logger
    cntrs  []Container
}
type Container struct {
    Name, Image, Network, Restart string
    Env, Ports, Labels, Volumes   map[string]string
}
type Status struct {
    Id, Image, State, Status string
    Names                    []string
    Labels                   map[string]string
    Created                  int64
}
type Statii map[string]Status
```

**After:**
```go
type Config struct{}
type Jed struct {
    client Client
    logger Logger
    svcs   []Service  // renamed field
}
type Service struct {
    Name, Image, Network, Restart string
    Env, Ports, Labels, Volumes   map[string]string
}
type Container struct {
    Id, Image, State, Status string
    Names                    []string
    Labels                   map[string]string
    Created                  int64
}
type Containers map[string]Container  // keyed by service name
```

### Constructor

**Before:**
```go
func (cfg *Config) NewSvc(ctx context.Context, client Client, lgr Logger, cfs fs.FS) (*Svc, error)
```

**After:**
```go
func (cfg *Config) NewJed(ctx context.Context, client Client, lgr Logger, cfs fs.FS) (*Jed, error)
```

### Methods

**Before:**
```go
func (svc *Svc) Containers() []Container
func (svc *Svc) Deploy(ctx context.Context, cntr *Container) (id string, err error)
func (svc *Svc) Undeploy(ctx context.Context, cntr *Container) error
func (svc *Svc) Statii(ctx context.Context) (Statii, error)
func (svc *Svc) Logs(ctx context.Context, id, tail string) ([]byte, error)
```

**After:**
```go
func (jed *Jed) Services() []Service
func (jed *Jed) Deploy(ctx context.Context, svc *Service) (id string, err error)
func (jed *Jed) Undeploy(ctx context.Context, svc *Service) error
func (jed *Jed) Containers(ctx context.Context) (Containers, error)
func (jed *Jed) Logs(ctx context.Context, id, tail string) ([]byte, error)
```

### Containers Helper Methods

**Before:**
```go
func (statii Statii) DeployName(baseName string) (string, error)
func (statii Statii) Id(baseName string) (string, error)
```

**After:**
```go
func (containers Containers) DeployName(serviceName string) (string, error)
func (containers Containers) Id(serviceName string) (string, error)
```

## Internal Changes

### Private types/functions

| Current | Proposed |
|---------|----------|
| `containerConfig` | `containerConfig` (unchanged - Docker API config) |
| `loadContainers()` | `loadServices()` |
| `(cntr *Container).config()` | `(svc *Service).config()` |
| `(cntr *Container).validate()` | `(svc *Service).validate()` |
| `newStatii()` | `newContainers()` |

### Field renames

| Struct | Current Field | Proposed Field |
|--------|---------------|----------------|
| `Jed` (was `Svc`) | `cntrs []Container` | `svcs []Service` |

### Variable naming conventions

- `cntr` → `service` (service configs)
- `statii` → `containers` (runtime containers)
- `baseName` → `serviceName` (in helper methods)

## Files to Update

1. **jed.go** - Main service, all method receivers
2. **container.go** → **service.go** (rename file)
3. **status.go** → **container.go** (rename file)
4. **docker.go** - Variable names only
5. **jed_test.go** - All test code
6. **status_test.go** → **container_test.go** (rename file)
7. **README.md** - All documentation
8. **TODO.md** - References to types

## Step-by-Step Plan

### Phase 3: Rename Status → Container
1. Rename `status.go` to `container.go` (conflict with existing!)
2. Move existing `container.go` content to temp location
3. Rename `Status` → `Container` in moved status.go
4. Rename `Statii` → `Containers`
5. Update `newStatii()` → `newContainers()`
6. Update helper method parameter names

### Phase 2: Rename Container → Service
1. Create `service.go` from old container.go content
2. Rename `Container` → `Service` throughout
3. Update `loadContainers()` → `loadServices()`
4. Update method receivers and variables

### Phase 1: Rename Svc → Jed
1. Rename `Svc` → `Jed` in jed.go
2. Update `NewSvc()` → `NewJed()`
3. Update all method receivers
4. Update field name `cntrs` → `svcs`

### Phase 4: Update Tests
1. Rename `status_test.go` → `container_test.go`
2. Update all variable names
3. Update all type references
4. Regenerate mocks if needed

### Phase 5: Update Documentation
1. Update README.md
2. Update TODO.md
3. Update comments in code

## Potential Issues

1. **File rename conflicts** - `container.go` exists, need to rename to `service.go` first
2. **Import cycles** - Shouldn't be an issue (all in same package)
3. **Test compilation** - May need multiple passes to fix all references
4. **Mock regeneration** - `go generate` after type renames
5. **Git history** - Consider using `git mv` to preserve history

## Validation Steps

After each phase:
- [ ] `go build ./...` - Check compilation
- [ ] `go test ./...` - All tests pass
- [ ] `golangci-lint run ./...` - No lint errors
- [ ] Review git diff - Changes look correct
