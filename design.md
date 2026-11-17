# Jed Design Decisions

Jed is organized around one shared service model, store-backed desired state,
explicit rendering, and a Docker Swarm runtime.

This document records design decisions and tradeoffs. Usage details live in the
package READMEs and [service-yaml.md](service-yaml.md).

## Package Boundaries

- `jed`: runtime-neutral service model, Store interface, spec rendering
- `store`: Store contract tests and helpers
- `store/bbolt`: persistent Store implementation
- `store/memo`: in-memory Store implementation
- `swarm`: Docker Swarm runtime
- `scanner`: registry image listing and config
- `extractor`: file extraction from Docker images
- `logger`: logging interface

## Desired State vs Rendered Specs

`jed.Service` is the service definition as stored and edited. Any string value
in it may contain templates.

`jed.Env` holds container/runtime environment variables separately from the
service definition.

`jed.Spec` is what a runtime deploys. Jed resolves its own conventions first:
templates, default user and network, and volumes.

Why separate raw desired state from rendered runtime input?

- Store implementations persist editable definitions, not deployment artifacts.
- Missing template variables fail before runtime deployment.
- Runtime packages do not need to know about Jed render context.
- Downstream callers can compose workflows from stable steps: load, render,
  inspect, deploy.

## Store Decisions

`jed.Store` persists service definitions, environment variable sets, and
intents. Service definitions are keyed by logical service name and image, so one
logical service can have multiple runnable image definitions.

`Store.GetEnv` returns an empty `Env` with an initialized `Vars` map when no env
has been stored for a service. A service with no environment variables is valid,
and deploy code does not need to special-case missing env. Store failures remain
distinguishable as errors.

Tradeoff: callers cannot distinguish “no env exists” from “empty env exists”
without additional store-level conventions.

The store does not enforce referential integrity. Env can exist without a
matching service, and deleting a service does not inherently delete env unless a
caller chooses to do both. This keeps the store simple and flexible, but
stricter applications must enforce their own policy.

`jed.Intent` is persisted desired state for a logical service. It records the
selected service image and desired replica count; no intent means disabled.

Switching images or disabling requires zero replicas. The service definition an
intent selects cannot be deleted.

## Rendering Decisions

Rendering works from stored state by logical service name. An unsaved intent
and env can also be rendered against stored state to preview before saving.

The global env is the store env named by `Config.VarsEnvName`.

Rendering rules:

- The global env and `env.Vars` participate in rendering service values.
- `env.Vars` wins on key collisions when rendering service values.
- Templates in service env values are filled from the global env, not from the
  service env itself.
- Global env vars are not added to `Spec.Env`.
- Expansion is single-pass.
- Missing vars fail before runtime deploy.
- All string values in `Service` participate in rendering, including nested
  structs, slices, maps, and map keys.

## Volume Conventions

Services declare container paths only. Jed derives where the data lives:

- Local volumes become bind mounts under `LOCAL_ROOT`.
- Network volumes become Docker-managed NFSv3 volumes on `NFS_HOST` under
  `NFS_ROOT`.

This keeps service definitions short and avoids a volume inventory. Derived
mounts exist only in rendered specs and are never stored.

Jed does not create NAS shares or host directories. Something else must
provision them with usable permissions.

Network volume names are a readable stem plus a hash of the service name and
container path. The same name serves as the Docker volume and the NAS share.
The naming scheme is a storage contract. Changing it strands existing data.

An explicit volume wins over a derived one at the same target. Local and network
volumes cannot share a target.

## Swarm Runtime Projection

`swarm.Swarm` consumes rendered `jed.Spec` values. It does not perform Jed-level
template expansion.

Projecting a spec into a Docker service payload is separate from applying it,
so callers can inspect the exact payload. Deploy creates the service or updates
it in place.

The swarm package builds Docker API payloads from assembled Go values rather
than JSON templates.

Why Go values over templates?

- Payload shape is easier to test.
- There is no placeholder convention to keep in sync.
- Runtime-specific mapping stays inside the swarm package.

## Service Status

`Swarm.Status` and `Swarm.Statuses` report observed runtime state, separately
from the desired replica count:

| Status | Meaning |
| --- | --- |
| `stopped` | No replicas wanted; none running. |
| `running` | All wanted replicas running. |
| `exited` | Replicas wanted, no active task, latest task completed cleanly. |
| `error` | Update/rollback failed, or no active task and the latest task failed or was rejected. |
| `pending` | Transitioning, or no conclusive outcome yet. |

Existing stop/stopping decisions take priority, followed by update/rollback
state and then fully running counts. Only when these do not settle the status
does task history decide: any active task means `pending`; otherwise the latest
task's outcome determines `exited`, `error`, or the default `pending`. No tasks
also means `pending`.

Latest means newest parsed Docker status timestamp, not task-list order or
string order. Missing or malformed timestamps count as zero time; ties retain
the first task encountered. An older failure does not override a newer clean
completion. A newer task in another state, such as `shutdown`, likewise does
not inherit an older failure.

`exited` is not an explicit stop, a healthy running service, or a request for
recovery. It distinguishes clean completion from ongoing convergence without
changing restart policy. Downstream callers must accept this fifth status.
Partly exited multi-replica services with an active task still report `pending`
when the wanted count is not met; per-slot classification and job modes are not
covered by this model.

## Versioned Swarm Resources

Docker Swarm secrets and configs are immutable. Jed treats names in
`Service.Secrets` and `Service.Configs` as base names and resolves them to the
latest versioned swarm resource at spec/deploy time:

```text
s3_secret_key -> s3_secret_key_v1, s3_secret_key_v2, ...
app_config    -> app_config_v1, app_config_v2, ...
```

Creating a secret or config creates the next version. Deploying resolves the
highest version.

Tradeoff: inspecting a payload still contacts Docker to resolve current
secret/config versions.

## Runtime Defaults and Policy

The swarm runtime supplies its own defaults and constraints. These are
documented in [swarm/README.md](swarm/README.md).

Restart condition remains a simple string on `jed.Service`. Jed resolves an
empty condition as `any` during spec rendering, without rewriting the stored
service. Swarm projects the supplied condition without an empty-value fallback
or a new empty-value validation error.

Delay and retry budgeting are runtime-wide policy on `swarm.Config`. The defaults
are a five-second delay and, for `any`, five attempts within a two-minute
lookback period per service slot. Explicit and defaulted `any` are treated
identically. `on-failure` retains one retry and no explicit window, with the
configured delay; `none` disables automatic restart. One-shot services can use
explicit `none` to avoid repeating after successful completion.

The window does not ensure two minutes of retries or resume an exhausted service
automatically. This favors bounded crash-loop recovery over unlimited retries.
Manual `Swarm.Restart` remains a separate rollout operation.

These changes apply on deployment: existing empty settings acquire `any`, and
explicit `any` acquires the configured retry budget. Running services do not
change immediately. No service-format or store-schema migration is required.

Traefik support is modeled on `jed.Service` as high-level routing intent. Swarm
projects that intent into Docker-provider labels. If another runtime is added
later, it can project the same intent differently, such as Kubernetes ingress
resources.

## Known Limitations

### Service Model Still Has Runtime-Specific Fields

`jed.Service` remains the shared desired-state model. Some fields are only used
by one runtime today, such as swarm secrets/configs and port publish mode.
Keeping them on the shared model is pragmatic while the project remains small.

### No Full Runtime Abstraction Yet

There is no broad runtime interface, plan/apply abstraction, or runtime-neutral
deploy result. Current APIs expose concrete building blocks and leave
orchestration to callers.

This avoids over-design while there is only one primary deployment runtime. If
another runtime appears, the existing render/projection seams should provide
useful pressure points.

### Service and Env Changes Are Applied at Deploy Time

Runtimes do not watch the store. Updating stored service/env data only affects a
running deployment when a caller deploys or redeploys the service.

### Existing Network Volumes May Not Be Retargeted

Docker likely reuses an existing named volume and ignores new driver options.
Changing `NFS_HOST` or `NFS_ROOT` may leave services on the old export. This is
unverified. Jed does not detect or replace mismatched volumes.
