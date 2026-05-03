# Jed Design Decisions

Jed is organized around one shared service model and multiple Docker runtimes.
The root `jed` package is intentionally runtime-neutral; runtime packages such
as `swarm` and `container` consume `jed.Service`, `jed.Env`, and `jed.Store`.

## Package Boundaries

```text
jed          shared service model and Store interface
store        Store contract tests and helpers
store/bbolt  persistent Store implementation
store/memo   in-memory Store implementation
swarm        Docker Swarm runtime
container    standalone Docker container runtime
transship    store-to-swarm orchestration
cmd/jed      CLI for editing stored desired state
cmd/transship CLI for operating swarm state
```

**Why this shape?**

The original root package mixed the service model, store contract, and
standalone container runtime. Swarm later became the primary runtime, which made
`jed/swarm` feel bolted on. Moving the standalone runtime to `container` makes
both runtimes peers while keeping `jed.Service` as the central downstream API.

## Service as Desired State

`jed.Service` defines what should run: image, command, ports, volumes, network,
labels, secrets, configs, resources, user, replicas, and related metadata.

`jed.Env` holds environment variables separately from the service definition.

**Why separate definition from runtime?**

- The same service model can be applied by different runtimes.
- Store implementations only persist desired state.
- Runtime packages own Docker API details and deployment mechanics.
- Downstream callers can compose their own workflows from stable building blocks.

## Store as Dependency

The `jed.Store` interface persists service definitions and environment variable
sets:

```go
type Store interface {
    GetService(ctx context.Context, name string) (Service, error)
    SetService(ctx context.Context, svc Service) error
    DelService(ctx context.Context, name string) error
    Services(ctx context.Context) ([]Service, error)

    GetEnv(ctx context.Context, name string) (Env, error)
    SetEnv(ctx context.Context, env Env) error
    DelEnv(ctx context.Context, name string) error
    Envs(ctx context.Context) ([]Env, error)
}
```

**Why an interface?**

- Production can use `store/bbolt`.
- Tests and embedded users can use `store/memo` or their own implementation.
- The store contract is testable via `store.RunStoreContractTests`.

## Separated Service and Env Storage

Service definitions and environment variables are stored separately.

**Why separate?**

- Env can change without rewriting the service definition.
- Secret rotation and environment updates have different lifecycles than image,
  port, or volume changes.
- `transship.Deployer` can load fresh env at deploy time.

## GetEnv Returns Empty Env for Missing Env

`Store.GetEnv` returns an empty `Env` with an initialized `Vars` map when no env
has been stored for a service.

**Why not an error?**

- A service with no environment variables is valid.
- Deploy code does not need special-case missing env.
- Store failures remain distinguishable as errors.

**Tradeoff:** callers cannot distinguish “no env exists” from “empty env exists”
without additional store-level conventions.

## Slice-Based APIs

Collection APIs return slices, not maps. For example:

```go
func (s Services) Find(name string) (Service, error)
func (rt *container.Runtime) Containers(ctx context.Context) (container.Containers, error)
```

**Why slices?**

- Iteration is the common case for CLI and UI use.
- The slice types can provide `Find` helpers for lookup.
- Callers can build maps if they need O(1) lookup.

## Transship Orchestration

The common swarm workflow is:

1. Load `jed.Service` from a `jed.Store`.
2. Validate it.
3. Load service env by service name.
4. Render a `jed.Spec` with caller-supplied vars.
5. Call `swarm.Deploy`.

That workflow lives in the `transship` package so downstream users do not have
to copy CLI glue from `cmd/transship`.

## Swarm Runtime

`swarm.Swarm` deploys a rendered `jed.Spec` to Docker Swarm:

```go
spec, err := jed.NewSpec(service, env, vars)
id, created, err := sw.Deploy(ctx, spec)
```

`Deploy` creates the service if it does not exist and updates it if it does.
This matches how the swarm functionality is used operationally.

### Swarm Spec Built Programmatically

The swarm package builds Docker API specs from typed Go data rather than JSON
templates.

**Why typed over templates?**

- Spec shape is easier to test.
- There is no placeholder convention to keep in sync.
- Runtime-specific mapping stays inside the swarm package.

### Swarm Defaults

The swarm runtime currently supplies these defaults/constraints:

- Resources default to 0.5 CPU / 128MB limit and 0.1 CPU / 64MB reservation.
- Log driver is `json-file` with 10m max size and 3 files.
- Root filesystem is read-only.
- User defaults to `1001:1001`.
- Replicas default to `0` unless set on `jed.Service`.
- Restart is disabled unless `Service.Restart.Condition` is set.
- Enabled restart uses the configured condition, 5s delay, and configurable attempts.
- Updates use stop-first order and pause on failure.

These should become clearer service fields in the next breaking model cleanup.

## Versioned Secrets and Configs

Docker Swarm secrets and configs are immutable. Jed treats the names in
`Service.Secrets` and `Service.Configs` as base names and resolves them to the
latest versioned swarm resource at deploy time:

```text
s3_secret_key -> s3_secret_key_v1, s3_secret_key_v2, ...
app_config    -> app_config_v1, app_config_v2, ...
```

Creating a secret or config creates the next version. Deploying resolves the
highest version.

## Standalone Container Runtime

The legacy standalone Docker runtime now lives in `container`:

```go
rt := container.New(client, logger)
id, err := rt.Deploy(ctx, service, env)
```

It preserves the original behavior of naming deployed containers with a random
suffix, e.g. `postgres-k7m9x2n`, and labeling them with `managed_by=jed`.

**Why random suffixes?**

- Avoids name collisions when containers linger after failed undeploy.
- Keeps the service name readable in Docker listings.
- Requires no persisted sequence state.

**Limitation:** multiple deployments of the same service are still rejected by
Jed and would often conflict on ports, volumes, and network aliases anyway.

## Known Limitations

### Service Model Still Has Runtime-Specific Fields

`jed.Service` remains the shared desired-state model. Some fields are still only
used by one runtime today, such as swarm secrets/configs and port publish mode.
Keeping them on the shared model is a pragmatic choice while the project remains
small.

### Store Does Not Enforce Referential Integrity

The store does not require env to have a matching service. You can set env for a
non-existent service, and deleting a service does not inherently delete env
unless a caller chooses to do both.

This is flexible, but callers that need stricter behavior must enforce it.

### Service and Env Changes Are Applied at Deploy Time

Runtimes do not watch the store. Updating stored service/env data only affects a
running deployment when a caller deploys or redeploys the service.
