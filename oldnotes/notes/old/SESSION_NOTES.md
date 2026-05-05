# Jed Development Session Notes

## What We Accomplished

### 1. Docker Log Decoder
- Implemented `logdecoder.go` with efficient buffer reuse
- Decodes Docker's multiplexed stream format (8-byte headers)
- Buffer optimization: pre-allocated 64KB buffer, reused via slice reslicing
- Tests passing with real Docker log data

### 2. Container Management Refactoring
- Refactored method signatures to be consistent:
  - `create()`, `start()`, `stop()`, `delete()` all take simple `string` names
  - Created `containerConfig` type as `map[string]any`
  - Moved config building to `Container.config()` method

### 3. Label-Based Naming Scheme (COMPLETED)
- **Replaced prefix-based filtering with labels**: `managed_by=jed`
- **Random suffix for collision avoidance**: Names like `postgres-k7m9x2n` using `hondo.Rand(7)`
- **Deploy flow**: Adds suffix, creates/starts container
- **Undeploy flow**: Uses `findDeployName()` to match base name to deployed name via prefix
- **Statii filtering**: Docker API filter by label: `filters={"label":["managed_by=jed"]}`
- **State management**: `svc.statii []Status` maintained by caller, Docker is source of truth
- Removed unused `prefix` field from `Svc`

### 4. Container Loading from Filesystem (COMPLETED)
- **NewSvc signature**: Takes `fs.FS` parameter - supports both embed and runtime
  ```go
  func (cfg *Config) NewSvc(client Client, lgr Logger, cfs fs.FS) (svc *Svc, err error)
  ```
- **Loads containers.yaml**: Unmarshals to `[]Container`
- **Loads .env files**: Using `godotenv.Unmarshal`, optional (returns empty map if missing)
- **Validation**: Validates each container (required fields: image, name, network, restart)
- **Label addition**: `managed_by=jed` added in `Container.config()` method when deploying
- **Env conversion**: `envLines()` converts `map[string]string` to Docker's `[]string` format

### 5. Test Structure
- Restructured tests with parent "Jed" Describe block for shared setup
- Separate Describe blocks for NewSvc and Logs
- Test data in `test/data/cntr-cfg/` with real containers.yaml and .env files
- Added `Containers()` method (using `slices.Clone` for immutability)

## Current Status

### Working
- ✅ Container loading from fs
- ✅ Env file loading (optional)
- ✅ Label-based management
- ✅ Random suffix naming
- ✅ Log decoding
- ✅ Most tests passing (7/8)

### Current Issue
**Test Failure**: NewSvc test checking for `managed_by` label is failing

**Why**:
- `Containers()` returns containers as loaded from YAML
- The `managed_by=jed` label is added in `Container.config()` during Deploy
- So containers don't have the label until they're actually deployed

**Fix Needed**:
- Update test to check what's actually loaded from YAML (names, env, etc.)
- Don't check for `managed_by` label in NewSvc tests (that's added during Deploy)
- Test file locations:
  - Tests: `/Users/trimble/proj/libbb/jed/jed_test.go` lines 56-77

## Next Steps

1. **Fix failing test** (lines 56-77 in jed_test.go):
   ```go
   It("should load 4 containers", func() {
       Expect(svc.Containers()).To(HaveLen(4))
   })

   It("should load container names correctly", func() {
       cntrs := svc.Containers()
       names := []string{}
       for _, cntr := range cntrs {
           names = append(names, cntr.Name)
       }
       Expect(names).To(ContainElements("traefik", "axis-camera-1", "logscale-1", "borken-1"))
   })

   It("should load env files for containers that have them", func() {
       cntrs := svc.Containers()
       var traefik *jed.Container
       for i, cntr := range cntrs {
           if cntr.Name == "traefik" {
               traefik = &cntrs[i]
               break
           }
       }
       Expect(traefik).NotTo(BeNil())
       Expect(traefik.Env).NotTo(BeEmpty())
   })
   ```

2. **Add more NewSvc tests**:
   - Missing containers.yaml → error
   - Invalid YAML → error
   - Validation errors → error (missing required fields)
   - Missing env file → succeeds with empty env (already works)

3. **Consider**:
   - Should `managed_by` label be added during load instead of config?
   - Update test data YAML to match current Container schema (remove obsolete fields like `env_form`, `auto_deploy`, `features`)

## Key Design Decisions

- **Container is public**: Used as input to Deploy/Undeploy
- **Containers() returns copy**: Uses `slices.Clone()` for immutability Todo: is this correct???
- **Env files optional**: Missing .env returns empty map, no error
- **Docker is source of truth**: Don't maintain state, query via Statii()

## File Structure

```
jed/
├── jed.go              # Main service, Deploy/Undeploy/Statii/Logs
├── jed_test.go         # Tests (NewSvc, Logs)
├── container.go        # Container type, loading, config building
├── docker.go           # Docker API wrappers (create/start/stop/delete)
├── logdecoder.go       # Docker log stream decoder
├── test/
│   └── data/
│       ├── cntr-cfg/   # Test container configs
│       │   ├── containers.yaml
│       │   ├── traefik.env
│       │   └── *.env
│       └── raw-log.bin # Test log data
```

## Public API

```go
// Types
type Config struct{}
type Svc struct { ... } // Todo: maybe dont need this???
type Container struct { ... }
type Status struct { ... }

// Interfaces (user implements)
type Client interface { SendObject, SendJson }
type Logger interface { Info, Debug, Error }

// Methods
func (cfg *Config) NewSvc(client Client, lgr Logger, cfs fs.FS) (*Svc, error)
func (svc *Svc) Containers() []Container
func (svc *Svc) Deploy(ctx, *Container) (id string, err error)
func (svc *Svc) Undeploy(ctx, *Container) error
func (svc *Svc) Statii(ctx) ([]Status, error)
func (svc *Svc) Logs(ctx, id, tail string) ([]byte, error)
```

## Important TODOs in Code

- `jed.go:42` - Ensure statii kept up-to-date by caller (refresh strategy?)
- `jed.go:43` - Consider regex validation of suffix format in findDeployName
- `jed.go:49-50` - Rename Status struct? Think through structure relative to services
- `jed.go:98` - Handle "created but not started" case in Undeploy
- `jed.go:117` - Add timestamps, since, until, follow options to Logs
- `jed.go:133-135` - Expose streaming logs (return io.ReadCloser)
- `container.go:67` - Add managed_by=jed label when loading containers (currently added in config())
- `container.go:159` - Demajic the .env file path construction
