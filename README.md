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
    "embed"
    "github.com/clarktrimble/jed"
)

//go:embed containers.yaml *.env
var containerFS embed.FS

// Create Jed instance
cfg := &jed.Config{}
j, err := cfg.NewJed(ctx, httpClient, logger, containerFS)

// Deploy a service
svc := j.Services()[0]
id, err := j.Deploy(ctx, &svc)

// Check container status
containers, err := j.Containers(ctx)
deployName, _ := containers.DeployName("postgres")

// Get logs
logs, err := j.Logs(ctx, id, "100")

// Undeploy
err = j.Undeploy(ctx, &svc)
```

## Architecture

### File Structure

```
jed/
├── jed.go           # Public API: NewJed, Deploy, Undeploy, Containers, Logs
├── service.go       # Service loading, validation, config building
├── container.go     # Container types and helpers
├── docker.go        # Docker API wrappers
├── logdecoder.go    # Docker log stream decoder
└── test/
    └── data/        # Test configurations
```

### Key Design Decisions

**Containers as Map**
- `Containers()` returns `map[string]Container` keyed by service name
- Automatically strips random suffixes (e.g., `postgres-k7m9x2n` → `postgres`)
- No caching - Docker is always the source of truth

**Stateless Design**
- No internal state tracking
- Caller controls when to refresh container status
- Prevents stale data issues

**Optional .env Files**
- Missing .env files return empty map (no error)
- Per-service environment configuration

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
```

### Methods

```go
// Create Jed instance
func (cfg *Config) NewJed(ctx context.Context, client Client, lgr Logger, cfs fs.FS) (*Jed, error)

// Get loaded services
func (jed *Jed) Services() []Service

// Service lifecycle
func (jed *Jed) Deploy(ctx context.Context, svc *Service) (id string, err error)
func (jed *Jed) Undeploy(ctx context.Context, svc *Service) error

// Container status and logs
func (jed *Jed) Containers(ctx context.Context) (Containers, error)
func (jed *Jed) Logs(ctx context.Context, id, tail string) ([]byte, error)

// Containers helpers
func (containers Containers) DeployName(serviceName string) (string, error)
func (containers Containers) Id(serviceName string) (string, error)
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

## Testing

- **32 passing tests**
- **87.8% coverage**
- Uses Ginkgo/Gomega for BDD-style tests
- Mock Docker client and logger
- Real Docker log data for decoder tests

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
- Suffix pattern: `-[^-]+$` (regex for stripping)

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
