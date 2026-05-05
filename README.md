# Jed

Just Enough Docker: shared service definitions, stores, and Docker runtimes.

## Concepts

- `jed.Service` is editable service configuration. It may contain `{{VAR}}` templates in command args and labels.
- `jed.Env` is the container/runtime environment for a service.
- `jed.Spec` is rendered, runtime-neutral intended state: `Service + Env` after template expansion.
- `jed.Store` persists services and envs.
- `jed.Jed` loads stored state and renders specs by service name.

Runtime packages consume Jed model values:

- `swarm` deploys rendered `jed.Spec` values to Docker Swarm.
- `container` deploys standalone Docker containers from raw `jed.Service` and `jed.Env` values.

## Quick Start: Swarm

```go
store, err := bbolt.New("jed.db")
if err != nil {
    // handle error
}

sw := swarm.New(dockerClient, logger)

// Load render vars from a named env, then render the named service from the store.
j, err := jed.New(ctx, store, "_global")
if err != nil {
    // handle error
}

spec, err := j.Spec(ctx, "postgres")
if err != nil {
    // handle error
}

id, created, err := sw.Deploy(ctx, spec)
if err != nil {
    // handle error
}
if created {
    // id is the new swarm service ID
}
```


## CLIs

- [`cmd/jed`](cmd/jed/README.md): edit service and env definitions in the store.
- [`cmd/transship`](cmd/transship/README.md): deploy and operate stored services on Docker Swarm.

## Documentation

- [Swarm](swarm/README.md)
- [Service YAML](service-yaml.md)
- [Store](store/README.md)
- [Design](design.md)
