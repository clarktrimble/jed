# Jed Design

## Architecture

### File Structure

```
jed/
├── jed.go           # Public API: New, Deploy, Undeploy, Services, Containers, Logs
├── service.go       # Service loading, validation, config building
├── container.go     # Container types and helpers
├── docker.go        # Docker API wrappers
├── logdecoder.go    # Docker log stream decoder
└── test/
    └── data/        # Test configurations
```

### Key Design Decisions

**Store-Based Architecture**
- `ServiceStore` is the source of truth for service definitions (image, ports, volumes, etc.)
- `EnvStore` is the source of truth for environment variables (required, not optional)
- `Services()` queries both stores and assembles complete Service structs
- No caching - stores are always queried fresh
- You implement the stores - Jed only defines interfaces

**Environment Variable Flow**
- `Services()` populates `Service.Env` from `envStore.Get()` for each service
- `Deploy()` uses `Service.Env` when creating containers
- `SetEnv()` / `GetEnv()` manage env vars in envStore
- `DeleteService()` automatically cleans up env vars

**Services and Containers as Maps**
- `Services(ctx)` returns `map[string]Service` keyed by service name
- `Containers(ctx)` queries Docker and returns `map[string]Container` keyed by service name
- Automatically strips random suffixes (e.g., `postgres-k7m9x2n` → `postgres`)

**Stateless Design**
- No internal state caching
- Stores and Docker are the sources of truth
- Caller controls when to refresh data
- Prevents stale data issues

**Interface-Only Public API**
- Jed exports only interfaces (ServiceStore, EnvStore)
- Store implementations are in `helper_test.go` for testing
- Users implement their own stores (filesystem, database, Redis, etc.)

**Best-Effort Undeploy**
- Stop errors are logged but don't fail undeploy
- Handles "created but not started" containers gracefully

## Public API

### Types

```go
type Config struct{}

type Jed struct {
    // Unexported fields
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

type Containers map[string]Container
```

### Interfaces (User Implements)

```go
type Client interface {
    SendObject(ctx context.Context, method, path string, snd, rcv any) error
    SendJson(ctx context.Context, method, path string, body io.Reader) ([]byte, error)
}

type Logger interface {
    Info(ctx context.Context, msg string, kv ...any)
    Debug(ctx context.Context, msg string, kv ...any)
    Error(ctx context.Context, msg string, err error, kv ...any)
}

type ServiceStore interface {
    Get(ctx context.Context, serviceName string) (Service, error)
    Set(ctx context.Context, svc Service) error
    Del(ctx context.Context, serviceName string) error
    List(ctx context.Context) ([]Service, error)
}

type EnvStore interface {
    Get(ctx context.Context, serviceName string) (map[string]string, error)
    Set(ctx context.Context, serviceName string, env map[string]string) error
    Del(ctx context.Context, serviceName string) error
}
```

### Methods

```go
// Create Jed instance
func (cfg *Config) New(ctx context.Context, client Client, lgr Logger,
                       serviceStore ServiceStore, envStore EnvStore) (*Jed, error)

// Query services from store
func (jed *Jed) Services(ctx context.Context) (map[string]Service, error)

// Service management
func (jed *Jed) CreateService(ctx context.Context, svc Service) error
func (jed *Jed) DeleteService(ctx context.Context, serviceName string) error

// Environment management
func (jed *Jed) SetEnv(ctx context.Context, serviceName string, env map[string]string) error
func (jed *Jed) GetEnv(ctx context.Context, serviceName string) (map[string]string, error)

// Container lifecycle
func (jed *Jed) Deploy(ctx context.Context, svc Service) (id string, err error)
func (jed *Jed) Undeploy(ctx context.Context, svc Service) error
func (jed *Jed) Redeploy(ctx context.Context, svc Service) error

// Container status and logs
func (jed *Jed) Containers(ctx context.Context) (Containers, error)
func (jed *Jed) Logs(ctx context.Context, id, tail string) ([]byte, error)

// Containers helpers
func (containers Containers) DeployName(serviceName string) (string, error)
func (containers Containers) Id(serviceName string) (string, error)
```
