# Jed

A Go library for managing Docker containers with a simple, type-safe API.

## Features

- **Container lifecycle management** - Deploy, undeploy, and monitor containers
- **Label-based identification** - Uses `managed_by=jed` labels to track containers
- **Random suffix naming** - Avoids name collisions with suffixes like `postgres-k7m9x2n`
- **Filesystem-based configuration** - Load containers from YAML with optional .env files
- **Stateless design** - Docker is the source of truth, no stale state
- **Log retrieval** - Decode Docker's multiplexed log format

## Installation

```bash
go get github.com/clarktrimble/jed
```

## Quick Start

```go
import (
    "context"
    "github.com/clarktrimble/jed"
)

// Implement ServiceStore and EnvStore interfaces
// (see test/helper_test.go for example implementations)
var serviceStore jed.ServiceStore = // your implementation
var envStore jed.EnvStore = // your implementation

// Create Jed instance
cfg := &jed.Config{}
j, err := cfg.New(ctx, httpClient, logger, serviceStore, envStore)

// Set environment variables
err = j.SetEnv(ctx, "postgres", map[string]string{
    "POSTGRES_PASSWORD": "secret123",
    "POSTGRES_USER": "admin",
})

// Get services (env auto-populated from envStore)
services, err := j.Services(ctx)
svc := services["postgres"]

// Deploy a service
id, err := j.Deploy(ctx, svc)

// Check container status
containers, err := j.Containers(ctx)
deployName, _ := containers.DeployName("postgres")

// Get logs
logs, err := j.Logs(ctx, id, "100")

// Undeploy
err = j.Undeploy(ctx, svc)
```

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

### Implementing Stores

Jed exports only interfaces - you implement the stores. Example implementations are in `helper_test.go`:

**Example: Filesystem Stores**
```go
// See helper_test.go for complete implementations
type FSServiceStore struct {
    fs fs.FS
}

func (s *FSServiceStore) List(ctx) ([]Service, error) {
    // Load from services.yaml
}

type FSEnvStore struct {
    fs fs.FS
}

func (s *FSEnvStore) Get(ctx, serviceName) (map[string]string, error) {
    // Load from {serviceName}.env file
}
```

**Example: In-Memory Stores**
```go
// See helper_test.go for complete implementations
type MemoryServiceStore struct {
    mu   sync.RWMutex
    svcs map[string]Service
}

type MemoryEnvStore struct {
    mu   sync.RWMutex
    envs map[string]map[string]string
}
```

**Your Custom Stores**
Implement `ServiceStore` and `EnvStore` for your backend:
- Database (PostgreSQL, MySQL, etc.)
- Key-value store (Redis, etcd, etc.)
- Cloud storage (S3, GCS, etc.)
- Any storage system you need

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

## Implementation Details

### Random Suffix Naming

Containers are deployed with random 7-character suffixes to avoid name collisions:
- `postgres` → `postgres-x7y9z2n`
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

## License

[Your License Here]
