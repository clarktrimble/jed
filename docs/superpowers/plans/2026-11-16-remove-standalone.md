# Remove Standalone Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove standalone deployment and house the remaining log decoder in Swarm.

**Architecture:** Delete the unused standalone runtime and make its formerly shared decoder Swarm-local. Preserve behavior with public TaskLogs tests before the structural change.

**Tech Stack:** Go 1.25.1, Ginkgo/Gomega, gopls, staticcheck.

**Spec:** `docs/superpowers/specs/2026-11-16-remove-standalone-design.md`

## Global Constraints
- Decoder filename: `swarm/dockerlog.go`.
- Preserve existing decoding behavior, framing errors, and the 65536-byte frame limit.
- No running-container cleanup, Docker calls, migrations, compatibility shim, decoder behavior redesign, or unrelated documentation rewrite.
- Preserve the user's existing server-validation checkbox change in `todo.md`.
- Leave historical plans, specs, and old notes unchanged.

## Review Focus
- Empty Docker logs return empty bytes without error.
- Mixed stdout/stderr frames and buffered payload tails are not lost or reordered.
- Truncated headers and payloads return contextual decoding errors.
- Frames at the size limit succeed; frames above it fail before payload allocation.
- Docker transport failures remain contextual retrieval errors, not decoding errors.

## File Map
- Delete: `container/{config,container,docker,mock_test,runtime,runtime_test}.go`.
- Move/localize: `internal/dockerlog/decoder.go` -> `swarm/dockerlog.go`.
- Modify: `swarm/task.go` (local decoder call); `swarm/task_test.go` (public regressions).
- Modify: `README.md`, `design.md` (current runtime documentation); `todo.md` (completion and decoder reference).
- Preserve: `test/data/raw-log.bin`, extractor, CLIs, dependencies used elsewhere.

### Task 1: Pin public log-decoding behavior

**Files:** `swarm/task_test.go`; existing `test/data/raw-log.bin`.

**Interfaces:** consumes `(*swarm.Swarm).TaskLogs(ctx context.Context, taskID, tail string) ([]byte, error)` and existing `ClientMock`. Produces public regression tests, with no API additions.

- [x] **Step 1:** Establish baseline with `go test ./...`. Record failures before editing.
- [x] **Step 2:** Add Ginkgo contexts under `TaskLogs`, retaining the existing single-frame test. Use `encoding/binary` for frame sizes and mock `SendJsonFunc`; assert the request remains `GET /v1.52/tasks/task-123/logs?stdout=true&stderr=true&tail=100`.
- [x] **Step 3:** Port the real-fixture case: `os.ReadFile("../test/data/raw-log.bin")`; assert no error, 51 newline-split elements, and first line contains `"app_id":"rsh"`.
- [x] **Step 4:** Add these framing/error assertions through the same public method (not tests of a newly exported decoder):
  - Empty bytes: `Expect(err).NotTo(HaveOccurred()); Expect(logs).To(BeEmpty())`.
  - Stdout frame `strings.Repeat("a", 4096)` followed by stderr `"tail\n"`: `Expect(string(logs)).To(Equal(strings.Repeat("a", 4096) + "tail\n"))` and no error.
  - Three-byte header: `Expect(err).To(MatchError(And(ContainSubstring("failed to decode logs for task"), ContainSubstring("failed to read docker log header"))))`.
  - Header declaring 4 payload bytes followed by only 2 bytes: error contains `failed to decode logs for task` and `failed to read docker log frame`.
  - 65536-byte payload: decoded output equals input payload, no error. Header declaring 65537 bytes without payload: error contains `docker frame exceeds decoder buffer size: 65536`.
  - Transport returns `errors.New("docker unavailable")`: error contains `failed to get logs for task` and `docker unavailable`.
- [x] **Step 5:** Format `swarm/task_test.go`, run `gopls check swarm/task_test.go`, then `go test ./swarm -ginkgo.focus='TaskLogs'`. All cases should pass before moving code; this refactor preserves behavior, so do not manufacture a failing test.
- [x] **Step 6:** Commit the focused regression tests only: `test: preserve Docker log decoding coverage`.

### Task 2: Remove standalone and localize the decoder

**Files:** remaining File Map entries.

**Interfaces:** consumes Task 1's public tests; produces private `decodeDockerLogs(src io.Reader) io.Reader` in `swarm/dockerlog.go`, with unchanged public `Swarm.TaskLogs` signature. Removes the entire `jed/container` package.

- [x] **Step 1:** Recheck status; use `gopls references -d` on `container.New` and `dockerlog.Decode` before deletion/move. Confirm no new production callers. Keep unrelated working changes intact.
- [x] **Step 2:** Move the decoder into `swarm/dockerlog.go`, change its package to `swarm`, remove the obsolete package comment, and rename `Decode` to `decodeDockerLogs`. Update `swarm/task.go` and remove its internal-package import. Do not change the decoder algorithm or errors.
- [x] **Step 3:** Delete all five tracked `container/` files plus its ignored generated mock and the now-empty internal decoder directories. Keep the shared raw-log fixture.
- [x] **Step 4:** Update README runtime summary/list and design package boundaries to name only Swarm; remove the standalone design section. Preserve the root's runtime-neutral model and the future-runtime discussion.
- [x] **Step 5:** Mark only the standalone-removal checkbox complete, preserving the user-edited validation checkbox. Replace `internal/dockerlog` in the earlier coverage TODO with `Swarm.TaskLogs`, noting the new decoder coverage while keeping unrelated coverage work outstanding.
- [x] **Step 6:** Format the changed Go files and run `gopls check swarm/dockerlog.go swarm/task.go swarm/task_test.go`; rerun focused TaskLogs tests. Expect no diagnostics and passing tests.
- [x] **Step 7:** Confirm `go list ./...` contains neither `jed/container` nor `jed/internal/dockerlog`; search active Go/current docs for imports or claims that either package still exists. Historical documents and the completed historical TODO may retain context.
- [x] **Step 8:** Run `go test ./...`, `go test -race ./...`, `go build ./...`, `go vet ./...`, and `staticcheck ./...`. Expect success; distinguish pre-existing failures from regressions rather than expanding scope.
- [x] **Step 9:** Review `git diff --check`, diff/stat and status against the spec. Commit only intended changes as `refactor: remove standalone container runtime`; do not accidentally commit the user's pre-existing TODO edit (stage only our hunks or leave TODO uncommitted and report it).

## Plan Self-Review
- All design requirements map to the single structural removal task; its safety regressions are Task 1.
- Each Review Focus input is explicitly exercised in Task 1, before the move.
- The private helper name and public TaskLogs signature are consistent across tasks.
- No new dependency, runtime abstraction, or decoder feature is introduced.
