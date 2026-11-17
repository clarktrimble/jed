# Jed

Just Enough Docker: shared service definitions, stores, and a Docker Swarm runtime.

## Concepts

- `jed.Service` is editable service configuration for one runnable image. Most service string values may contain `{{VAR}}` templates.
- `jed.Intent` is desired state for a logical service: selected image plus replica count. Missing intent means disabled.
- `jed.Env` is the container/runtime environment for a service. Env values may contain templates filled from the global env.
- `jed.Spec` is rendered, runtime-neutral intended state: `Service + Env + Intent` after template expansion.
- `jed.Store` persists services, envs, and intents.
- `jed.Jed` loads stored state, renders specs by service name, and updates intent state.

The `swarm` package deploys rendered `jed.Spec` values to Docker Swarm, the supported deployment runtime.

## Quick Start: Swarm

```go
store, err := (&bbolt.Config{Path: "jed.db"}).New()
if err != nil {
    // handle error
}

// Store a service definition; Enable selects among stored definitions.
err = store.SetService(ctx, jed.Service{Name: "postgres", Image: "postgres:16"})
if err != nil {
    // handle error
}

swCfg := &swarm.Config{}
_ = envconfig.Process("swarm", swCfg)
sw := swCfg.New(logger)

jedCfg := &jed.Config{}
_ = envconfig.Process("jed", jedCfg)
j := jedCfg.New(store, logger)

// Select the image to run, scale it, then render the named service from the store.
err = j.Enable(ctx, "postgres", "postgres:16")
if err != nil {
    // handle error
}
err = j.Scale(ctx, "postgres", 1)
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
    // id is the created swarm service ID
} else {
    // id is the updated swarm service ID
}

// or to preview an unsaved intent and env:
preview, err := j.ProvisionalSpec(ctx,
    jed.Intent{Name: "postgres", Image: "postgres:16", Replicas: 1},
    jed.Env{Name: "postgres", Vars: map[string]string{"POSTGRES_DB": "app"}},
)
```

## CLIs

Note: these have not been maintained, much

Todo: resuscitate or remove please

- [`cmd/jed`](cmd/jed/README.md): edit service and env definitions in the store.
- [`cmd/transship`](cmd/transship/README.md): deploy and operate stored services on Docker Swarm.
- cmd/scan-registry

## Documentation

- [Swarm](swarm/README.md)
- [Service YAML](service-yaml.md)
- [Store](store/README.md)
- [Design](design.md)
