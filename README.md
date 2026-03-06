# Jed

Manage Docker containers as services with persistent configuration and environment.
Note: this is a little stale.  See swarm/README.md.

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

## CLIs

### jed

Manage service and env definitions in the store.

```
jed ls-svc                          # list services
jed get-svc postgres                # show service details
jed set-svc postgres.yaml           # create/update from YAML
jed set-env postgres postgres.env   # set env from .env file
```

### transship

Deploy to Docker Swarm using definitions from the jed store.

```
transship deploy reauth-acp         # deploy/update a swarm service
transship ls-secrets                # list swarm secrets
transship create-secret db_password # create a versioned secret
transship ls-services               # list running swarm services
```

## Documentation

- [Reference](https://pkg.go.dev/github.com/clarktrimble/jed)
- [Design](design.md)
