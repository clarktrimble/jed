# Jed

Just Enough Docker: shared service definitions, stores, and Docker runtimes.

The root `jed` package defines runtime-neutral desired state:

- `jed.Service` describes editable service configuration.
- `jed.Env` holds service environment variables.
- `jed.Spec` is rendered intended state: `Service + Env` after template expansion.
- `jed.Store` persists service and env definitions.

Runtime packages consume those definitions:

- `swarm` deploys services to Docker Swarm.
- `container` deploys services as standalone Docker containers.
- `transship` loads a named service from a store and applies it to Swarm.

## Quick Start: Swarm

```go
store, _ := bbolt.New("jed.db")
sw := swarm.New(dockerClient, logger)

global, _ := store.GetEnv(ctx, "_global")
deployer := &transship.Deployer{Store: store, Swarm: sw, Vars: global.Vars}
spec, id, created, err := deployer.Deploy(ctx, "postgres")
if err != nil {
    // handle error
}
_ = spec // rendered intended state
if created {
    // id is the new swarm service ID
}
```

For direct runtime use:

```go
svc, _ := store.GetService(ctx, "postgres")
env, _ := store.GetEnv(ctx, "postgres")
global, _ := store.GetEnv(ctx, "_global")

spec, _ := jed.NewSpec(svc, env, global.Vars)
id, created, err := sw.Deploy(ctx, spec)
_ = created
```

## CLIs

### jed

Manage service and env definitions in the store.

```text
jed ls-svc                          # list services
jed get-svc postgres                # show service details
jed set-svc postgres.yaml           # create/update from YAML
jed set-env postgres postgres.env   # set env from .env file
```

### transship

Deploy to Docker Swarm using definitions from the jed store.

```text
transship deploy reauth-acp         # deploy/update a swarm service
transship ls-secrets                # list swarm secrets
transship create-secret db_password # create a versioned secret
transship create-config app_cfg cfg.yaml # create a versioned config
transship ls-services               # list running swarm services
```

## Documentation

- [Swarm](swarm/README.md)
- [Service YAML](service-yaml.md)
- [Store](store/README.md)
- [Design](design.md)
