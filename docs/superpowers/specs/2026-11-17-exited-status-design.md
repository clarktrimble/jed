# Exited service status

Status: proposed; awaiting user approval.

## Intent

Add `swarm.StatusExited` (`"exited"`) to Jed's public service-status contract.
A service can still want replicas while its latest task has completed cleanly
and no task is active. That observation is neither an explicit stop nor an
in-progress transition, and should not be reported as `pending` indefinitely.

The downstream example is a single-replica service using `restart: on-failure`:
a Docker restart left its task in `complete` after exit 0, with zero running
replicas and one desired replica. Not restarting is acceptable; mislabeling the
observed state is the problem.

## Scope and design significance

This is a change to Jed's observable runtime-state model, not to its desired
state, deployment policy, or restart policy. `exited` describes an observation;
it does not mean that the desired replica count is satisfied, that the service
is healthy, or that Jed should recover it automatically.

The public status vocabulary becomes `stopped`, `pending`, `error`, `running`,
and `exited`. Both `Swarm.Status` and `Swarm.Statuses` must implement the same
contract through the existing pure `computeStatus` function.

No change is needed to `jed.Service`, `jed.Intent`, rendered specs, the store
schema, Docker payloads, or existing API signatures.

## Approaches

- **Keep `pending` for completed tasks:** preserves the current vocabulary but
  does not distinguish a clean exit from ongoing convergence. Rejected because
  it leaves the downstream problem unresolved.
- **Treat completion as `stopped` or `error`:** avoids a new status but conflates
  wanted replicas with an explicit stop, or clean completion with failure.
  Neither matches the requested semantics.
- **Add `exited` and classify inactive services by their latest task:** the
  downstream proposal and recommended approach. It expands the public status
  vocabulary and intentionally replaces historical-failure aggregation with
  latest-task classification. This is the sole proposed implementation approach;
  approval of this document approves that choice.

## Status contract

Preserve the existing precedence, in this order:

| Condition | Result |
| --- | --- |
| Desired tasks = 0 and running tasks = 0 | `stopped` |
| Desired tasks = 0 and running tasks > 0 | `pending` (stopping) |
| Update state is `updating` or `rollback_started` | `pending` |
| Update state is `paused` or `rollback_paused` | `error` |
| Running tasks = desired tasks | `running` |

Only after those conditions, evaluate task history:

1. Any active task means `pending`, irrespective of task timestamps or older
   terminal states.
2. No tasks means `pending`, including a newly created service.
3. With no active tasks, select the latest task by its parsed status timestamp.
4. Latest state `failed` or `rejected` means `error`.
5. Latest state `complete` means `exited`.
6. All other latest states mean `pending`.

Active states remain `new`, `pending`, `assigned`, `accepted`, `preparing`,
`ready`, `starting`, and `running`, as defined by the current `isActive` helper.

For example, an older failed task followed by a newer completed task becomes
`exited`, not `error`. Conversely, a newer failed task following a completed task
is `error`. A newer `shutdown` task following a failed task is `pending`, not an
error inferred from history. These changes follow the latest-task rule.

## Task ordering and data flow

`ServiceTasks` already copies Docker's `Status.Timestamp` into `Task.Timestamp`.
`Statuses` currently copies only the task state; it must also copy the timestamp
when grouping tasks by service ID.

Parse task timestamps with `time.RFC3339Nano` and compare the resulting
`time.Time` values, not the strings. Docker's variable fractional-second
precision makes lexical ordering incorrect. Task-list response order must not
be used as a proxy for chronology. There is no need to sort or mutate the input
slice, nor to change the public string type of `Task.Timestamp`.

Builder call for unspecified inputs: treat missing or malformed timestamps as
zero time; retain the first encountered task when parsed times are equal. Thus
valid Docker timestamps outrank missing or malformed ones, and existing
single-state fixtures without timestamps remain usable. Neither API gains a
new timestamp-parse error. This fallback is not a stronger chronology guarantee
for malformed or indistinguishable input.

Replace `hasFailed`; keep `isActive` and the existing high-priority status rules.
The implementation may scan the tasks once, returning `pending` if any task is
active while retaining the newest inactive candidate.

## Compatibility and documentation

Downstream status consumers must accept the new string `exited`. External
callers that assume a closed four-value set may need updates; no in-repository
consumer requiring such a change was found during reference inspection.

Add a documented `StatusExited` constant. Expand the `StatusError` comment and
`Status()` status list to cover failed/rejected tasks as well as paused
updates/rollbacks. Update the `Statuses` task-fetch comment to include completed
tasks, and update the status example in `swarm/README.md`.

Record the service-status contract and its desired-state/observed-state
distinction in the root `design.md` as part of implementation. The dated design
records this change; the root document should describe the resulting design.

## Verification

Add a same-package table test for the pure `computeStatus` function, which is
currently covered only indirectly by external-package API tests. Cover:

- Existing stopped, stopping, updating, rollback, paused, and running precedence,
  including precedence over completed task history.
- Every active state, including active tasks mixed with terminal task history.
- Empty task history.
- Latest `failed`, `rejected`, `complete`, and other terminal/unknown states.
- An old failure followed by a clean exit, and the reverse chronology.
- Unsorted task lists in both orders.
- Variable fractional-second precision and timestamp offsets, using parsed
  instants rather than textual ordering.
- Missing, malformed, and equal timestamps according to the builder call above.

Extend mocked `Status` and `Statuses` coverage to verify the new public value.
For `Statuses`, include mixed services and an older failure/newer completion
whose result depends on preserving timestamps, not response order.

Use test-first implementation, then formatting, Go diagnostics, package tests,
broad tests, and the repository's available vet/staticcheck checks. No live
Docker change or restart experiment is required to verify this classification.

## Non-goals and anti-goals

- No recovery automation or restart-policy changes.
- No per-slot/multi-replica redesign: a partly exited service with any active task
  retains its existing status behavior.
- No job-mode support or new interpretation of desired replica counts.
- No task timestamp API redesign or unrelated task-list sorting cleanup.
- Do not call a clean exit an explicit stop, a failure, or a healthy running
  service merely to avoid adding a new status.

## Workflow

- [x] Inspect current status computation, task projection, tests, and design docs.
- [x] Discuss the requested status change and its design significance.
- [x] Write the proposed design.
- [ ] Obtain design approval.
- [ ] Write the implementation plan using the writing-plans skill.
- [ ] Implement, review, and verify.
