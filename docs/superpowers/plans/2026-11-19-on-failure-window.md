# On-failure Retry Window Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `on-failure` a lookback window without increasing its one-retry limit.

**Architecture:** Reuse `swarm.Config.RestartWindow` in the existing policy projection. Preserve constructor defaults and explicit zero settings. Keep Docker responsible for retry scheduling.

**Tech Stack:** Go with Ginkgo and Gomega.

**Spec:** `docs/superpowers/specs/2026-11-19-on-failure-window-design.md`

## Global Constraints

- The default window remains two minutes.
- Leave `RestartMaxAttempts` exclusive to `any`.
- Leave `any` and `none` unchanged.
- No service model change. No store migration. No new settings. No scheduler changes.
- Keep prose tight. Avoid unnecessary comma-separated phrasing and repetition.
- Execute inline on the current branch as requested by the user.

## Review Focus

- Direct construction must supply the two-minute window.
- Envconfig defaults must reach `on-failure` projection.
- Environment overrides must reach `on-failure` projection.
- Custom settings must preserve nanoseconds without increasing the retry count.
- Explicit zero must omit the window without removing the one-retry limit.

## Task 1: Project the window and update current documentation

**Files:**
- Modify: `swarm/spec.go` — `restartPolicy` projection.
- Modify: `swarm/config.go` — window comment and help description.
- Test: `swarm/spec_test.go` — direct-constructor defaults.
- Test: `swarm/config_test.go` — configured policy and JSON output.
- Modify: `swarm/README.md` — policy settings and compatibility.
- Modify: `service-yaml.md` — concise restart behavior.
- Modify: `design.md` — current policy decision.
- Modify: `notes/todo.md` — remove the resolved investigation item.

**Interfaces:**
- Consumes: `swarm.Config.RestartWindow time.Duration` and `jed.RestartOnFailure`.
- Produces: unchanged `(*Swarm).restartPolicy(service jed.Service) RestartPolicy` signature.
- Public tests use `(*Swarm).Spec(context.Context, jed.Spec) (swarm.Spec, error)`.

- [x] **Step 1: Update policy expectations and add missing cases.**
  In `swarm/spec_test.go` assert direct construction yields `Window == int64(120000000000)` and JSON contains that value for `on-failure`.
  In `swarm/config_test.go` extend the envconfig default and override cases to cover `on-failure`. Expected policies:
  ```go
  // Envconfig defaults.
  swarm.RestartPolicy{Condition: "on-failure", Delay: 5000000000, MaxAttempts: 1, Window: 120000000000}
  // Environment overrides already set by the existing test.
  swarm.RestartPolicy{Condition: "on-failure", Delay: 1500000000, MaxAttempts: 1, Window: 90000000000}
  // Custom durations and RestartMaxAttempts: 3.
  swarm.RestartPolicy{Condition: "on-failure", Delay: 1500000007, MaxAttempts: 1, Window: 90000000011}
  // Explicit zero settings.
  swarm.RestartPolicy{Condition: "on-failure", MaxAttempts: 1}
  ```
  Check serialized `Window` for each nonzero case. For explicit zero assert JSON equals `map[string]any{"Condition": "on-failure", "MaxAttempts": float64(1)}`. Retain the existing `any` and `none` assertions.

- [x] **Step 2: Verify the new expectations fail.**
  Run `go test ./swarm -ginkgo.focus='Config|restart policy'`.
  Expected: failures caused by the missing `on-failure` window.

- [x] **Step 3: Apply the minimal production change.**
  Set `restart.Window = int64(d.restartWindow)` in the existing `jed.RestartOnFailure` branch of `swarm/spec.go`. Keep `MaxAttempts = 1`. Update only the window comment and help description in `swarm/config.go` to include both retrying conditions.

- [x] **Step 4: Format and verify the policy.**
  Run `gofmt -w swarm/spec.go swarm/config.go swarm/spec_test.go swarm/config_test.go`.
  Run `gopls check swarm/spec.go swarm/config.go swarm/spec_test.go swarm/config_test.go`.
  Run `go test ./swarm -ginkgo.focus='Config|restart policy'` then `go test ./swarm`.
  Expected: no diagnostics and passing tests.

- [x] **Step 5: Update current documentation.**
  Make the window apply to both retrying conditions in the settings table. Keep the attempts setting exclusive to `any`. Replace obsolete no-window claims. State the next-deployment boundary. Keep the existing exhaustion warning without duplicating it. Remove the resolved investigation note. Do not rewrite historical specs.

- [x] **Step 6: Validate and review.**
  Run `go test ./...` then `go test -race ./swarm` and `go vet ./...`.
  Run `staticcheck ./...` if available; otherwise record the skip.
  Run `git diff --check` and inspect the full diff against the spec. Check current docs for stale no-window claims. Record results and complete the checklist.
  Expected: passing checks with any unavailable tooling explicitly reported.

- [x] **Step 7: Commit the reviewed change.**
  Stage only the files listed above and these design/plan documents. Commit as `fix: apply restart window to on-failure`.
