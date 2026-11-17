# Restart policy defaults and Swarm configuration

Status: approved by the user.

## Intent

Make persistent services recover automatically by default while keeping Jed's
service configuration simple: Just Enough Docker. Keep the existing restart
string rather than introducing a service-level policy object.

## Agreed behavior

### Jed rendering

- An empty `Service.Restart` renders as `jed.RestartAny`.
- Explicit `any`, `on-failure`, and `none` retain their meanings as conditions.
- Apply the default through the shared rendering path used by `Jed.Spec` and
  `Jed.ProvisionalSpec`.
- Stored definitions remain unchanged. Rendered specs expose the chosen condition.

### Swarm policy

Add runtime-wide settings to `swarm.Config`:

| Proposed field | Type | Default |
| --- | --- | --- |
| `RestartDelay` | `time.Duration` | `5s` |
| `RestartMaxAttempts` | `int` | `5` |
| `RestartWindow` | `time.Duration` | `2m` |

Field names and types above are implementation proposals; the durations use
human-readable default tags, following the existing environment-config pattern.
`time.Duration` already counts nanoseconds; Docker payload fields need only an
`int64` conversion, not scaling.

Project conditions as follows:

| Condition | Delay | Max attempts | Window |
| --- | --- | --- | --- |
| `any` | configured delay | configured attempts | configured window |
| `on-failure` | configured delay | 1 | omitted |
| `none` | omitted | omitted | omitted |

- Defaulted and explicitly selected `any` use exactly the same policy.
- Preserve the existing one-retry/no-explicit-window behavior of `on-failure`.
  Investigation of its rationale and dependencies is recorded in `notes/todo.md`.
- Remove Swarm's empty-condition fallback. Do not add a special empty-condition
  validation error; Swarm projects the condition it receives.
- Keep the public `swarm.New(client, logger)` signature because downstream tests
  use it. Direct construction uses the same new runtime defaults; config-based
  construction supplies its settings. Constructor consolidation and a possible
  local `export_test.go` helper are deferred in `notes/todo.md`; migrating
  downstream test callers must be addressed before removing the public constructor.
- Preserve the existing convention that environment configuration populates
  `Config` default tags; `Config.New` does not silently substitute defaults for
  explicitly supplied zero values.
- Manual `Swarm.Restart` remains unchanged.

## Retry semantics and compatibility

The window is a restart-count lookback period, not a cooldown. Five retries can
be exhausted before two minutes have elapsed. An exhausted service does not
resume automatically merely because the window expires.

This policy applies to each Swarm service slot, not as one shared budget across
all replicas.

Existing empty restart settings acquire `any` behavior on their next deployment.
Existing explicit `any` settings acquire the bounded retry policy on their next
deployment. These are intentional behavior changes, with no legacy mode.
Running services are not changed until deployed. One-shot services should use
explicit `none` when successful completion must not trigger another run.

No service YAML shape change, store schema change, or stored-data migration is
needed. Explicit `none` and the existing `on-failure` retry budget are preserved.

## Approaches considered

- **Service-level restart policy object:** permits per-service tuning, but expands
  the editable model and requires configuration compatibility work. Not selected;
  the discussion favored simplicity and runtime-wide settings.
- **Fixed adapter policy:** minimal configuration, including the option of changing
  the fixed delay. Not selected for these runtime controls; the chosen approach
  keeps five seconds while moving tuning into `swarm.Config`.
- **Runtime-wide policy configuration (selected):** keep service conditions simple,
  with shared delay and an attempts/window pair for `any`.
- **Default only in Swarm, or in both layers:** avoids requiring Jed-rendered input
  but leaves previews implicit or duplicates defaulting. Instead, Jed resolves the
  condition and Swarm projects it, without a fallback or extra empty-value error.

## Verification and documentation

- Cover empty-to-`any` defaulting in both stored and provisional spec rendering;
  verify stored input is not mutated and explicit conditions are preserved.
- Cover default and custom Swarm runtime settings, constructor behavior, and
  nanosecond payload representation.
- Cover `any` attempts/window output and preserve `on-failure`'s one-attempt,
  omitted-window output and `none`'s omitted timing/budget fields.
- Remove the old test expectation that Swarm defaults empty restart to `none`;
  verify projection has no empty-condition fallback or new special rejection.
- Document defaults, condition-specific settings, retry exhaustion, and the
  distinction from manual service restarts.
- Run focused tests during implementation, then relevant diagnostics and broad
  tests before claiming completion.

## Non-goals

No per-service tuning, policy object, new restart condition, exponential backoff,
automatic recovery after budget exhaustion, or change to manual restart behavior.
Do not change the `on-failure` retry budget or add a window to it.

## Workflow

- [x] Explore current implementation and history.
- [x] Discuss approaches and policy choices.
- [x] Write proposed design.
- [x] Obtain design approval.
- [x] Write implementation plan using the writing-plans skill.
- [ ] Implement and verify through the normal Superpowers workflow.
