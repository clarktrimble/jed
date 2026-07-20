# Jed Design Decisions

Jed is organized around one shared service model, store-backed desired state, explicit rendering, and small Docker runtime packages.

This document records design decisions and tradeoffs. Usage details live in the package READMEs and [service-yaml.md](service-yaml.md).

## Package Boundaries

```text
jed           service model, Store interface, spec rendering
store         Store contract tests and helpers
store/bbolt   persistent Store implementation
store/memo    in-memory Store implementation
swarm         Docker Swarm runtime
container     standalone Docker container runtime
cmd/jed       CLI for editing stored desired state
cmd/transship CLI for operating swarm state
```

The root `jed` package is intentionally runtime-neutral. It contains the shared service model, store contract, and rendering helpers. Runtime packages such as `swarm` and `container` own Docker API details and deployment mechanics.

The standalone runtime lives in `container`, making it a peer of `swarm` while keeping `jed.Service` as the central downstream API.

## Desired State vs Rendered Specs

`jed.Service` is editable and persisted desired state. It describes what should run: image, command, ports, volumes, network, labels, secrets, configs, resources, user, replicas, and related metadata. Any string value in the service may contain templates.

`jed.Env` holds container/runtime environment variables separately from the service definition.

`jed.Spec` is rendered intended state: a `Service` and `Env` after Jed-level template expansion. It is still runtime-neutral.

Why separate raw desired state from rendered runtime input?

- Store implementations persist editable definitions, not deployment artifacts.
- Missing render variables fail before runtime deployment.
- Runtime packages do not need to know about Jed render context.
- Downstream callers can compose workflows from stable steps: load, render, inspect, deploy.

## Store Decisions

`jed.Store` persists service definitions and environment variable sets. It is an interface so production can use `store/bbolt`, tests and embedded users can use `store/memo`, and store behavior can be checked with `store.RunStoreContractTests`.

`Store.GetEnv` returns an empty `Env` with an initialized `Vars` map when no env has been stored for a service. A service with no environment variables is valid, and deploy code does not need to special-case missing env. Store failures remain distinguishable as errors.

Tradeoff: callers cannot distinguish “no env exists” from “empty env exists” without additional store-level conventions.

The store does not enforce referential integrity. Env can exist without a matching service, and deleting a service does not inherently delete env unless a caller chooses to do both. This keeps the store simple and flexible, but stricter applications must enforce their own policy.

`Jed.Enable(ctx, name)` and `Jed.Disable(ctx, name)` toggle whether a service is allowed to run; disabled services must have zero replicas.

`Jed.Scale(ctx, name, count)` updates the stored replica count for callers that manage desired scale separately from service YAML editing. It changes desired state only; runtimes still apply the new count on a later deploy.

## Rendering Decisions

There are two rendering entry points:

- `jed.Render(service, env, vars)` renders raw values supplied by the caller.
- `jed.New(ctx, store, varsEnvName, logger)` plus `j.Spec(ctx, name)` loads from a store and renders by service name.

`jed.New(ctx, store, varsEnvName, logger)` records the named env to use for render vars, and `j.Spec(ctx, name)` reloads that env when rendering so store edits are visible to existing `Jed` values. The env name is explicit. `_global` is a `cmd/transship` CLI convention, not a magic library default.

Rendering rules:

- Caller-supplied vars and `env.Vars` participate in rendering service values.
- `env.Vars` wins on key collisions when rendering service values.
- Env values render from caller-supplied vars only; env vars do not template each other.
- Render vars are not added to `Spec.Env`.
- Expansion is single-pass.
- Missing vars fail before runtime deploy.
- All string values in `Service` participate in rendering, including nested structs, slices, maps, and map keys.

## Swarm Runtime Projection

`swarm.Swarm` consumes rendered `jed.Spec` values. It does not perform Jed-level template expansion.

`Swarm.Spec(ctx, spec)` validates the rendered service, resolves latest versioned swarm secrets/configs, and returns the Docker service payload as `swarm.Spec` without creating or updating anything. This is the introspection seam for dry-run/preview-style consumers.

`Swarm.Deploy(ctx, spec)` uses the same projection and then creates the service if it does not exist or updates it if it does. It returns the Docker service ID for both creates and updates, and returns `created=true` only for creates. Missing service detection uses `swarm.ErrServiceNotFound` and `errors.Is` rather than string matching.

The swarm package builds Docker API payloads from assembled Go values rather than JSON templates.

Why Go values over templates?

- Payload shape is easier to test.
- There is no placeholder convention to keep in sync.
- Runtime-specific mapping stays inside the swarm package.

## Versioned Swarm Resources

Docker Swarm secrets and configs are immutable. Jed treats names in `Service.Secrets` and `Service.Configs` as base names and resolves them to the latest versioned swarm resource at spec/deploy time:

```text
s3_secret_key -> s3_secret_key_v1, s3_secret_key_v2, ...
app_config    -> app_config_v1, app_config_v2, ...
```

Creating a secret or config creates the next version. Deploying resolves the highest version.

Tradeoff: `Swarm.Spec` is an introspection method, but it still contacts Docker to resolve current secret/config versions.

## Runtime Defaults and Policy

The swarm runtime currently supplies defaults and constraints such as resource defaults, read-only root filesystem, default user, restart mapping, update behavior, and log driver settings. These are documented in [swarm/README.md](swarm/README.md) rather than repeated here.

Traefik support is modeled on `jed.Service` as high-level routing intent. Swarm projects that intent into Docker-provider labels. If another runtime is added later, it can project the same intent differently, such as Kubernetes ingress resources.

## Standalone Container Runtime

The standalone Docker runtime lives in `container`. It names deployed containers with a random suffix, e.g. `postgres-k7m9x2n`, and labels them with `managed_by=jed`.

Why random suffixes?

- Avoids name collisions when containers linger after failed undeploy.
- Keeps the service name readable in Docker listings.
- Requires no persisted sequence state.

Limitation: multiple deployments of the same service are still rejected by Jed and would often conflict on ports, volumes, and network aliases anyway.

## Known Limitations

### Service Model Still Has Runtime-Specific Fields

`jed.Service` remains the shared desired-state model. Some fields are only used by one runtime today, such as swarm secrets/configs and port publish mode. Keeping them on the shared model is pragmatic while the project remains small.

### No Full Runtime Abstraction Yet

There is no broad runtime interface, plan/apply abstraction, or runtime-neutral deploy result. Current APIs expose concrete building blocks (`jed.Spec`, `swarm.Spec`, `Swarm.Deploy`) and leave orchestration to callers.

This avoids over-design while there is only one primary deployment runtime. If another runtime appears, the existing render/projection seams should provide useful pressure points.

### Service and Env Changes Are Applied at Deploy Time

Runtimes do not watch the store. Updating stored service/env data only affects a running deployment when a caller deploys or redeploys the service.
