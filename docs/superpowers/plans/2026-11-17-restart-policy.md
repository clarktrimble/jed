# Restart Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render unspecified restart conditions as `any` and configure Swarm's runtime-wide restart policy without expanding service definitions.

**Architecture:** Jed resolves the default condition in its shared spec-loading path. Swarm projects that condition and applies configuration-owned delay, attempts, and window settings. The existing public direct constructor remains for downstream tests.

**Tech Stack:** Go 1.25.1 module, Ginkgo/Gomega, standard `time.Duration`, existing environment-config default tags, Docker Swarm JSON API.

**Spec:** `docs/superpowers/specs/2026-11-17-restart-policy-design.md` (approved).

## Global Constraints

- Keep `Service.Restart` a string; no store schema change or migration.
- Empty restart renders as `jed.RestartAny` in both `Jed.Spec` and `Jed.ProvisionalSpec`.
- Add `RestartDelay time.Duration` default `5s`, `RestartMaxAttempts int` default `5`, and `RestartWindow time.Duration` default `2m` to `swarm.Config`.
- For `any`, apply configured delay, attempts, and window equally to explicit and defaulted conditions.
- Preserve `on-failure`: configured delay, one retry, no explicit window.
- Preserve `none`: no delay, attempts, or window fields in the Docker payload.
- Remove Swarm's empty-condition fallback; do not add a special empty-condition error.
- Keep `swarm.New(client, logger)` and its signature, using the new runtime defaults.
- `Config.New` uses supplied config values, including zeros; default tags are populated by environment configuration, not by `New`.
- No change to manual restart, per-service tuning, backoff, or automatic resumption after retry exhaustion.
- Do not undertake the deferred constructor cleanup or `on-failure` investigation in `notes/todo.md`.

## Review Focus

1. Stored or provisional input with empty restart: output is `any`, but stored definitions remain unchanged (Task 1).
2. Explicit `none` and `on-failure` under nondefault runtime settings: no new retry budget/window leaks into them (Task 2).
3. Explicit zero configuration: no constructor fallback changes zero to a default; zero attempts remains unlimited and zero timing fields remain omitted (Task 2).
4. Non-integer-second durations: Docker receives the exact nanosecond values, without truncation or extra scaling (Task 2).
5. Directly constructed specs with empty restart: no adapter default or new error, and the wire condition remains empty (Task 2).

## File Map

- `jed.go`: default the condition in `(*Jed).spec`, alongside existing default user/network handling.
- `service.go`: correct the `Service.Restart` field comment.
- `spec_test.go`: stored/provisional rendering and input-preservation coverage.
- `swarm/config.go`: runtime settings and config-based initialization.
- `swarm/swarm.go`: retained public constructor and per-instance restart settings.
- `swarm/spec.go`: receiver-aware projection and Docker `Window` field.
- `swarm/config_test.go`: default-tag processing, custom and zero config, duration precision.
- `swarm/spec_test.go`: condition-specific policies, default direct constructor, empty pass-through, JSON omissions.
- `service-yaml.md`, `swarm/README.md`, `design.md`: document agreed behavior and compatibility.
- No new production packages, dependencies, or test-only exports.

---

### Task 1: Resolve the default restart condition during Jed rendering

**Files:** Modify `jed.go`, `service.go`, `spec_test.go`, and `service-yaml.md` (restart condition/default description only).

**Interfaces:**
- Consumes unchanged `Service.Restart`, `RestartAny`, and existing `Store` helpers.
- Preserves `(*Jed).Spec(context.Context, string) (Spec, error)` and `(*Jed).ProvisionalSpec(context.Context, Intent, Env) (Spec, error)`.
- Produces explicit `Spec.Service.Restart == RestartAny` for empty stored definitions, without writing to the store.

- [x] **Step 1: Add failing rendering tests.**

In `spec_test.go`, add `Describe("restart condition rendering", ...)`, using `newFakeStore`, a valid stored service and intent, and a configured `Jed`. Exercise both public rendering paths for each row:

| Stored restart | Expected rendered restart |
| --- | --- |
| `""` | `jed.RestartAny` |
| `jed.RestartAny` | `jed.RestartAny` |
| `jed.RestartOnFailure` | `jed.RestartOnFailure` |
| `jed.RestartNone` | `jed.RestartNone` |

Each generated example must make these assertions (with `input` and `want` from its row):

```go
storedSpec, err := j.Spec(ctx, "app")
Expect(err).NotTo(HaveOccurred())
Expect(storedSpec.Service.Restart).To(Equal(want))
preview, err := j.ProvisionalSpec(ctx, intent, env)
Expect(err).NotTo(HaveOccurred())
Expect(preview.Service.Restart).To(Equal(want))
stored, err := store.GetService(ctx, "app", "local/app:v1")
Expect(err).NotTo(HaveOccurred())
Expect(stored.Restart).To(Equal(input))
```

- [x] **Step 2: Confirm the empty-input example fails.**

Run `go test . -ginkgo.focus='restart condition rendering' -count=1`. Expected: failure because the rendered condition is empty, not `any`.

- [x] **Step 3: Implement the default and document it.**

In `(*Jed).spec(ctx context.Context, intent Intent, env Env) (Spec, error)`, set the loaded service's empty restart to `RestartAny` before validation/rendering, alongside the existing user/network defaults. Do not modify `Service.Validate` to reject empty raw definitions. Update `Service.Restart`'s comment and the restart section of `service-yaml.md`: empty means `any` after Jed rendering; one-shot services can specify `none`. Retain the one-retry description for `on-failure`.

- [x] **Step 4: Format, diagnose, and test.**

Run `gofmt -w jed.go service.go spec_test.go`, `gopls check jed.go service.go spec_test.go`, and `go test . -count=1`. Expected: no diagnostics and passing tests. Review changed expected full-spec assertions if the new explicit field legitimately affects them; do not weaken unrelated assertions.

- [x] **Step 5: Commit the rendering deliverable.**

Run `git add jed.go service.go spec_test.go service-yaml.md && git commit -m "feat: default rendered restart condition to any"` after reviewing the diff.

### Task 2: Configure and project the Swarm restart policy

**Files:** Modify `swarm/config.go`, `swarm/swarm.go`, `swarm/spec.go`, `swarm/config_test.go`, `swarm/spec_test.go`, `swarm/README.md`, `service-yaml.md`, and `design.md`.

**Interfaces:**
- Consumes Task 1's explicit rendered condition, while preserving empty pass-through for direct callers.
- Adds the three config fields listed in Global Constraints, with JSON names `restart_delay`, `restart_max_attempts`, and `restart_window`, default tags `5s`, `5`, and `2m`, and `split_words:"true"` tags so envconfig recognizes the documented underscore-separated environment names.
- Preserves `(*Config).New(logger.Logger) *Swarm`, `New(Client, logger.Logger) *Swarm`, and `(*Swarm).Spec(context.Context, jed.Spec) (Spec, error)`.
- Adds `Window int64` with JSON tag `Window,omitempty` to `swarm.RestartPolicy`.
- Changes private helpers to `(*Swarm).buildSpec(jed.Spec, []resolvedSecret, []resolvedConfig) (Spec, error)` and `(*Swarm).restartPolicy(jed.Service) RestartPolicy` so projection uses instance settings, preserving the existing resolved-resource arguments.

- [x] **Step 1: Add failing runtime-policy tests.**

Extend `swarm/config_test.go` using `envconfig.Process("swarm", cfg)` and `nopLogger{}`. Tests must isolate the relevant `SWARM_*` environment variables with cleanup, so external settings cannot alter default assertions. Exercise `cfg.New(...).Spec(...)` with a minimal explicit-condition spec containing no secrets/configs; this path performs no Docker requests and needs no new injection seam.

For default-tag processing, verify these exact field values and projected policy:

```go
Expect(cfg.RestartDelay).To(Equal(5 * time.Second))
Expect(cfg.RestartMaxAttempts).To(Equal(5))
Expect(cfg.RestartWindow).To(Equal(2 * time.Minute))
Expect(body.TaskTemplate.RestartPolicy).To(Equal(swarm.RestartPolicy{
    Condition: "any", Delay: int64(5 * time.Second),
    MaxAttempts: 5, Window: int64(2 * time.Minute),
}))
```

Also test environment overrides `SWARM_RESTART_DELAY=1500ms`, `SWARM_RESTART_MAX_ATTEMPTS=3`, and `SWARM_RESTART_WINDOW=90s` through config creation and payload projection. For a directly supplied config, use `RestartDelay: 1500*time.Millisecond + 7*time.Nanosecond`, `RestartMaxAttempts: 3`, and `RestartWindow: 90*time.Second + 11*time.Nanosecond`; assert the exact integer nanoseconds reach the payload. An all-zero restart config must produce zero delay, attempts, and window without substituting defaults.

In `swarm/spec_test.go`, replace the old empty-means-none example with these cases:

| Constructor/settings | Input condition | Expected policy |
| --- | --- | --- |
| retained `swarm.New` | `any` | delay `5000000000`, attempts `5`, window `120000000000` |
| custom config above | `any` | exact configured values |
| custom config above | `on-failure` | configured delay, attempts `1`, window omitted |
| retained `swarm.New` | `on-failure` | delay `5000000000`, attempts `1`, window omitted |
| custom config above | `none` | condition only |
| retained `swarm.New` | empty | empty condition, no new error |

For the empty-input case, assert only successful projection and `Condition == ""`, including `"Condition":""` in serialized output; do not introduce a new policy guarantee for unresolved input. Preserve existing invalid-condition rejection.

Marshal payloads and inspect the `TaskTemplate.RestartPolicy` map. Verify `Window` is present for default/custom nonzero `any`, absent for `on-failure`, all three optional fields are absent for `none`, and zero-valued optional fields are absent for explicit zero config. Example assertions:

```go
Expect(wirePolicy).NotTo(HaveKey("Window")) // on-failure
Expect(wirePolicy).To(HaveLen(1))           // none: Condition only
Expect(wirePolicy["Condition"]).To(Equal("none"))
```

- [x] **Step 2: Confirm the tests fail before implementation.**

Run `go test ./swarm -ginkgo.focus='Config|restart policy' -count=1`. Expected initially: compile errors for missing config/Window fields; after those declarations, assertions must expose missing configuration wiring/defaults. Do not count unrelated failures as the red test.

- [x] **Step 3: Implement config storage and condition-specific projection.**

Add private `restartDelay time.Duration`, `restartMaxAttempts int`, and `restartWindow time.Duration` fields to `Swarm`. Retained `New(client Client, logger logger.Logger) *Swarm` initializes them to five seconds, five attempts, and two minutes. `Config.New` retains existing client creation, then copies all three config values into the instance returned by `New`, including zeros. Copy values rather than retain a mutable config pointer.

Convert `buildSpec` and `restartPolicy` to the receiver methods defined in Interfaces and update their call sites. Remove the empty-condition substitution. For `none`, return condition only. For `on-failure`, use configured delay and fixed attempts `1`, leaving window zero/omitted. For `any`, use all three configured values. Convert durations using `int64(duration)`. Add no special empty-condition error and no new zero-value fallback.

Before changing Go symbols, run `gopls symbols` on the three production files and semantic references for the helpers and public types being changed. The planning pass found only one production caller each for `buildSpec` and `restartPolicy`; confirm before editing.

- [x] **Step 4: Format, diagnose, and run Swarm tests.**

Run `gofmt -w swarm/config.go swarm/swarm.go swarm/spec.go swarm/config_test.go swarm/spec_test.go`, `gopls check swarm/config.go swarm/swarm.go swarm/spec.go swarm/config_test.go swarm/spec_test.go`, and `go test ./swarm -count=1`. Expected: no diagnostics and passing tests, including the unchanged manual-restart tests. No Docker daemon or live deployment is required.

- [x] **Step 5: Document the complete policy.**

Update `swarm/README.md` with a configuration table showing `SWARM_RESTART_DELAY=5s`, `SWARM_RESTART_MAX_ATTEMPTS=5`, and `SWARM_RESTART_WINDOW=2m`, explicitly distinguishing `any` from the preserved `on-failure` budget/window. Update `service-yaml.md` and `design.md` to explain Jed's rendering default, explicit `none` for one-shot services, and the intentional redeploy-time behavior changes for empty and explicit `any`. Explain that the two-minute lookback does not guarantee two minutes of retries or automatic resumption after exhaustion. Keep manual restart documentation distinct. Make no claim about how Docker handles a directly supplied empty condition.

- [x] **Step 6: Review, verify, and commit the runtime deliverable.**

Run `git diff --check`, `go test ./... -count=1`, `go vet ./...`, `staticcheck ./...`, and `go test -race ./... -count=1`. Expected: all pass; report any tool/environment limitation instead of claiming success. Search active docs/comments for stale statements that empty restart means `none` or `any` is always unlimited; do not rewrite historical design/plan documents. Review the diff against all five Review Focus entries.

Commit only the Task 2 files with message `feat: configure swarm restart policy`. Leave the approved design, plan, and deferred follow-up notes intact.

## Completion and review

- [x] Check both tasks against the approved design and record actual verification results.
- [x] Obtain whole-change review using available review tools; if no independent reviewer is available, state that limitation and perform a self-review without presenting it as independent.
- [x] Do not implement deferred constructor cleanup, add a test-only export, change the store schema, or alter manual restart semantics.

## Execution results

- Task 1: `10ac96e`; the empty-condition test failed as expected, then root tests and diagnostics passed.
- Task 2: `feb988d`; missing-field and runtime-policy assertions failed as expected before implementation, then Swarm and full tests passed.
- Verification passed: `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, `staticcheck ./...`, changed-file `gopls check`, and `git diff --check`.
- Whole-change author self-review found no blocking or minor findings. No independent reviewer tool was available; no live Docker deployment was performed.
- Ruling: the original plan accidentally omitted the existing resolved-secret/config arguments from the proposed `buildSpec` signature. Keep those arguments when adding the receiver, avoiding unrelated secret/config projection regressions. The Interfaces block above is corrected accordingly.
