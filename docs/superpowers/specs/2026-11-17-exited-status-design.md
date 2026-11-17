# Exited service status

Status: approved by the user.

## Purpose

A service can want a running replica even though its task exited cleanly and
Docker will not restart it. Jed currently calls that `pending`, suggesting
progress that is not happening. Add `exited` to describe this condition.

This changes the status Jed reports, not what Jed or Docker does to the service.

## The five service statuses

| Status | Meaning |
| --- | --- |
| `stopped` | No replicas are wanted and none are running. |
| `running` | All wanted replicas are running. |
| `exited` | Replicas are wanted, no task is active, and the latest task completed cleanly. |
| `error` | An update/rollback failed, or no task is active and the latest task failed or was rejected. |
| `pending` | The service is transitioning, or there is not enough evidence for another status. |

These are meanings, not an evaluation order. Existing stop, update/rollback,
and running decisions keep their current priority. For example, a paused update
still reports `error` even if an earlier task completed cleanly.

## What changes

When those existing decisions do not settle the status:

- **Any active task:** report `pending`, as today.
- **No active task:** use the latest task's outcome:

  | Latest task | Service status |
  | --- | --- |
  | Completed cleanly (`complete`) | `exited` |
  | Failed or rejected | `error` |
  | Anything else, including no task | `pending` |

“Active” keeps its current meaning: a task is running or progressing toward
running. We do not change that set of task states.

The important difference is that history does not outweigh the latest outcome:

- Older failure, newer clean completion → `exited`.
- Older clean completion, newer failure → `error`.
- Older failure, newer `shutdown` → `pending`.
- Any of these with a task still active → `pending`.

`exited` does not mean explicitly stopped, healthy, or automatically recoverable.
It reports clean completion while replicas remain wanted.

## Why a new status

Keeping `pending` leaves the problem unresolved. Calling this `stopped` would
ignore that replicas are still wanted; calling it `error` would describe a clean
completion as failure. A separate `exited` status preserves those distinctions.
This is the proposed approach for approval.

## Implementation constraints

Both `Status()` and `Statuses()` must return the same classification. Preserve
timestamps in `Statuses()`, as `ServiceTasks()` already does, and replace
`hasFailed` with latest-outcome classification.

“Latest” means newest parsed Docker status timestamp, using `time.RFC3339Nano`,
not response order or string comparison. Docker varies fractional-second
precision, so lexical ordering is not reliable.

Proposed fallback for unspecified input: missing or malformed timestamps count
as zero time; ties retain the first encountered task. Do not add a new API error
or change the public timestamp field's type.

## Compatibility and limits

Downstream callers must accept the new `exited` value. Existing `error` results
based only on an older failure can change to `exited` or `pending`, according to
the latest task. No configuration or stored-data migration is required.

Do not change restart policy, add recovery automation, or redesign partial
multi-replica status. Services with some tasks still active retain existing
behavior. Job-mode support and unrelated timestamp sorting are out of scope.

## Tests and documentation

Add direct table tests for status precedence, active/no tasks, latest outcomes,
and timestamp ordering, including variable fractional precision. Cover the
fallback for missing, malformed, and equal timestamps. Add mocked API coverage
for `exited` and timestamp preservation in `Statuses()`.

Document `StatusExited`, broaden the `StatusError` comment, update the status
list and README example, and record the resulting contract in root `design.md`.
Use test-first implementation and run formatting, diagnostics, tests, and
available repository lint checks before claiming completion.

## Workflow

- [x] Inspect current behavior and discuss the design change.
- [x] Write the proposed design.
- [x] Obtain design approval.
- [x] Write the implementation plan.
- [ ] Implement, review, and verify.
