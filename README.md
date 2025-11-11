# Jed

Manage Docker containers as services with persistent configuration and environment.

## Quick Start

```go
    cfg := &jed.Config{}
    j, _ := cfg.New(ctx, dockerClient, logger, store)

    service := jed.Service{
        Name:    "postgres",
        Image:   "postgres:16",
        Network: "mynet",
        Restart: "unless-stopped",
        Ports:   map[string]string{"5432/tcp": "5432"},
    }
    j.CreateService(ctx, service)

    store.SetEnv(ctx, jed.Env{
        Name: "postgres",
        Vars: map[string]string{"POSTGRES_PASSWORD": "secret"},
    })

    id, _ := j.Deploy(ctx, service)
    //j.Undeploy(ctx, service)
```

## Documentation

- [Reference](https://pkg.go.dev/github.com/clarktrimble/jed)
- [Design](design.md)
