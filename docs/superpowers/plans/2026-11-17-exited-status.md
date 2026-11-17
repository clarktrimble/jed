# Exited Service Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Report cleanly completed, inactive services as `exited` rather than `pending`.

**Architecture:** Keep the existing status precedence and pure `computeStatus` entry point. Replace historical-failure detection with latest-task outcome classification, and preserve timestamps in the bulk status path.

**Tech Stack:** Go, standard `testing` table tests, existing Ginkgo/Gomega mocked API tests.

**Spec:** `docs/superpowers/specs/2026-11-17-exited-status-design.md`

## Global Constraints

- Existing stop, update/rollback, and running decisions keep their current priority.
- “Latest” means newest parsed Docker status timestamp, using `time.RFC3339Nano`, not response order or string comparison.
- Missing or malformed timestamps count as zero time; ties retain the first encountered task.
- Do not change restart policy, add recovery automation, or redesign partial multi-replica status.
- No new dependencies, public timestamp type changes, API errors, or stored-data migration.

## Review Focus

- History ordering: newest outcome wins even when Docker returns newest first; Task 1 tests both orders.
- Equivalent/variable timestamp formats: compare instants, not strings; Task 1 tests fractions and offsets.
- Incomplete timestamps: valid times outrank malformed/missing times and ties retain the first; Task 1 pins the fallback.
- Active tasks and Docker update states: historical completion must not override their precedence; Task 1 exercises these combinations.
- Bulk task grouping: timestamps and service IDs must survive projection; Task 2 tests mixed services with adversarial task order.

## File Map

- Create `swarm/status_internal_test.go`: same-package unit tests for private `computeStatus`.
- Modify `swarm/status.go`: new constant, classification, timestamp projection, and comments.
- Modify `swarm/status_test.go`: existing external-package API regression tests.
- Modify `swarm/README.md` and root `design.md`: public status vocabulary and semantics.

### Task 1: Classify the latest inactive task

**Interfaces:** Preserve `computeStatus(ss ServiceStatus, us UpdateStatus, tasks []Task) Status` and `isActive(state string) bool`. Add exported `StatusExited Status = "exited"`; remove private `hasFailed(tasks []Task) bool`. No caller signatures change.

- [ ] **Write `TestComputeStatus` in `swarm/status_internal_test.go` (package `swarm`).** Use a standard table with `name`, `ss`, `us`, `tasks`, and `want`. Initially express the new expected value as `Status("exited")` so the regression fails behaviorally before the constant exists. For every row assert:

  ```go
  if got := computeStatus(tt.ss, tt.us, tt.tasks); got != tt.want {
      t.Errorf("computeStatus() = %q, want %q", got, tt.want)
  }
  ```

  Unless otherwise noted, use desired=1, running=0, empty update state. Define older=`2026-11-17T12:00:00.9Z`, newer=`2026-11-17T12:00:00.91Z`. Include:

  | Input | Expected |
  | --- | --- |
  | Desired/running 0/0, completed task | `stopped` |
  | Desired/running 0/1, completed task | `pending` |
  | `updating` or `rollback_started`, completed task | `pending` |
  | `paused` or `rollback_paused`, completed task | `error` |
  | Desired/running 1/1, completed task | `running` |
  | Desired/running 2/1, active task and completed task | `pending` |
  | Each active state, mixed with newer failed/completed tasks, in both orders | `pending` |
  | No tasks | `pending` |
  | Single `complete` | `exited` |
  | Single `failed` or `rejected` | `error` |
  | Single `shutdown`, `orphaned`, or unknown state | `pending` |
  | Older failed/rejected, newer complete; both slice orders | `exited` |
  | Older complete, newer failed/rejected; both slice orders | `error` |
  | Older failed, newer shutdown | `pending` |
  | Older complete at `2026-11-17T12:00:00Z`, newer failed at `2026-11-17T12:00:00.1Z` | `error` |
  | Failed at `2026-11-17T13:00:00+01:00`, complete at `2026-11-17T12:00:00.1Z` | `exited` |
  | Missing/malformed failed timestamp, valid completed timestamp; both orders | `exited` |
  | All timestamps missing/malformed, first task complete | `exited` |
  | Equal instants (`.9Z` and `.900Z`), differing states | First task's outcome |

  Active states: `new`, `pending`, `assigned`, `accepted`, `preparing`, `ready`, `starting`, `running`.

- [ ] **Run the red test:** `go test ./swarm -run '^TestComputeStatus$' -count=1`. Expect failures for completed/latest-outcome cases, not build errors.
- [ ] **Implement classification in `swarm/status.go`.** Add the documented constant. Keep the existing switch through the fully-running branch, then scan tasks for activity and the newest parsed timestamp. Return pending for activity or no tasks; otherwise map the newest state to exited/error/pending. Treat parse errors explicitly as zero time and retain the first candidate on ties. Remove `hasFailed`, preserve `isActive`, and do not sort/mutate the task slice.
- [ ] **Verify green:** `gofmt -w swarm/status.go swarm/status_internal_test.go`, `gopls check swarm/status.go swarm/status_internal_test.go`, then `go test ./swarm -run '^TestComputeStatus$' -count=1` and `go test ./swarm -count=1`. Expect no diagnostics and passing tests.
- [ ] **Commit:** `git add swarm/status.go swarm/status_internal_test.go && git commit -m "feat: classify cleanly completed services as exited"`.

### Task 2: Expose the contract consistently through both APIs

**Interfaces:** Consume `StatusExited` and the unchanged `computeStatus` from Task 1. Preserve `(*Swarm).Status(context.Context, string) (Status, error)` and `(*Swarm).Statuses(context.Context) (map[string]Status, error)`.

- [ ] **Extend `swarm/status_test.go` using its existing mock-response helpers.** Add a single-service context with desired=1/running=0 and older failed/newer complete tasks, asserting `Expect(err).NotTo(HaveOccurred())` and `Expect(status).To(Equal(swarm.StatusExited))`.
  Extend the bulk fixture to contain running, rejected, and exited services. Give the exited service an older failed task first and its newer completed task second, with the fractional timestamps from Task 1. Assert the exact returned map includes `"good": swarm.StatusRunning`, `"bad": swarm.StatusError`, and `"exited": swarm.StatusExited`. This must fail if bulk projection drops timestamps; grouping must keep other services independent.
- [ ] **Run the red API test:** `go test ./swarm -run '^TestSwarm$' -ginkgo.focus='Status' -count=1`. Expect the new single-service case to pass but the bulk exited result to fail because timestamps are discarded.
- [ ] **Fix bulk projection in `swarm/status.go`.** Construct grouped tasks with both `State: t.Status.State` and `Timestamp: t.Status.Timestamp`. Update the fetch comment to mention both failed and completed tasks.
- [ ] **Update documentation.** Broaden the `StatusError` comment and `Status()` list to cover failed/rejected tasks and add `StatusExited`. Add `exited` to the README example. Add a Service Status section to root `design.md` explaining the five meanings, unchanged precedence, latest-outcome behavior, and distinction between wanted replicas and clean exit; do not change restart-policy documentation.
- [ ] **Verify the focused change:** `gofmt -w swarm/status.go swarm/status_test.go`; `gopls check swarm/status.go swarm/status_internal_test.go swarm/status_test.go`; `go test ./swarm -count=1`. Expect clean diagnostics and all tests passing.
- [ ] **Run final verification:** `go test ./... -count=1`, `go test -race ./swarm -count=1`, `go vet ./...`, `staticcheck ./...`, and `git diff --check`. Expect exit 0 for each; report any pre-existing failure separately rather than expanding scope. The Makefile configures vet/staticcheck, not golangci-lint.
- [ ] **Review the complete diff against the approved spec.** Confirm no restart, service/store schema, or unrelated sorting changes, both APIs preserve their signatures, and the documented table matches behavior. A separate reviewer is preferable; if unavailable, explicitly report self-review rather than claiming independent review.
- [ ] **Commit the verified changes:** stage only the intended API-test, projection, and documentation files and commit as `feat: expose exited status consistently across swarm APIs`. Update workflow checkboxes to reflect verified progress.
