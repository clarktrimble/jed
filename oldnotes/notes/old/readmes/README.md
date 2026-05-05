# Jed

A Go library for managing Docker containers with a simple, type-safe API.

## Features

- **Container lifecycle management** - Deploy, undeploy, and monitor containers
- **Label-based identification** - Uses `managed_by=jed` labels to track containers
- **Random suffix naming** - Avoids name collisions with suffixes like `postgres-k7m9x2n`
- **Store-based configuration** - Pluggable ServiceStore and EnvStore interfaces
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

See [design.md](design.md) for architecture details, design decisions, and full API reference.
