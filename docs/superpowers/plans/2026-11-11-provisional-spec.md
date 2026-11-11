# ProvisionalSpec Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render proposed intent/env without storing either, with an explicit API pair.

**Architecture:** Move stored-intent loading to Spec; ProvisionalSpec validates supplied intent and matching env name; shared spec renderer consumes the Intent. Replace SpecWithEnv with ProvisionalSpec.

**Tech Stack:** Go, Ginkgo/Gomega, existing fakeStore.

**Spec:** docs/superpowers/specs/2026-11-11-provisional-spec-design.md

## Global Constraints
- Expose Spec(ctx, name) and ProvisionalSpec(ctx, intent, env); remove SpecWithEnv.
- Name/image selection comes from intent; validate supplied intent and matching env name in ProvisionalSpec only.
- Service/global vars remain stored; provisional intent/env are neither fetched nor written.
- Preserve rendering, defaults, volumes, and Enable guards; no unrelated changes or commits.

## Review Focus
- Missing stored intent/env must not block provisional rendering.
- Stored intent with different image/replicas must not override the proposal.
- Empty name/image and negative replicas must fail before service lookup.
- Supplied env must not merge with stored env or mutate inputs/storage.
- Existing Spec missing-env/intent behavior and volume resolution must remain intact.

### Task 1: Replace the supplied-state rendering API

**Files:** Modify jed.go, spec_test.go, swarm/README.md; add current usage documentation in README.md.

**Interfaces:** `ProvisionalSpec(ctx context.Context, intent Intent, env Env) (Spec, error)`; private `spec(ctx context.Context, intent Intent, env Env) (Spec, error)`; existing Spec signature unchanged.

- [x] Adapt SpecWithEnv suite/network test to provisional intent input. Add assertions for absent stored intent/env, original supplied replica count, different stored image/replicas left untouched, replacement (not merge) of stored env, invalid name/image/replicas, missing selected service/global vars, defaults, and input/result isolation.
- [x] Run `go test .`; expect missing ProvisionalSpec until implemented (public API migration). Verify behavioral RED by temporarily using the old renderer under the new name if needed.
- [x] Move GetIntent to Spec after GetEnv, replace SpecWithEnv with ProvisionalSpec delegating to shared spec(ctx,intent,env), and validate supplied intent/name matching at the ProvisionalSpec boundary. Preserve intent in result and all remaining render stages.
- [x] Format, run gopls diagnostics and `go test .`; expect success.
- [x] Update current docs/callers; document no env merge, no writes, stored service/global dependencies, and distinction from Enable guards.
- [x] Run `go test ./...`, `go vet ./...`, and `git diff --check`; inspect semantic references and diff. Configured lint if present.
- [x] Self-review (no subagent tool), mark checks, leave uncommitted.

## Verification
Baseline suite passed. New API tests first failed to compile; minimally exposing the new signature then produced 16 behavioral failures on the old stored-intent dependency. Final shared-renderer implementation passes gopls diagnostics, `go test ./...`, `go vet ./...`, and `git diff --check`. No configured lint (no repo config). Self-review completed without production findings; strengthened clone-isolation assertion with an independent literal. Follow-up: moved intent validation and env-name matching into ProvisionalSpec only, preserving Spec's stored-state behavior. Name-mismatch and validation-precedence regressions verified RED→GREEN. Final `go test ./...`, `go vet ./...`, and `git diff --check` pass after user documentation edits. Changes remain uncommitted.
