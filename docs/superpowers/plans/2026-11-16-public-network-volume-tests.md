# Public Network-Volume Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace private-helper stdlib network-volume tests with Ginkgo/Gomega tests of public rendering APIs, preserving coverage.

**Architecture:** Use the existing external `jed_test` suite, a real `memo.Store`, and `Jed.Spec`/`Jed.ProvisionalSpec`. Keep the existing Swarm payload tests and all production code unchanged.

**Tech Stack:** Go, Ginkgo/Gomega, existing in-memory store.

**Spec:** The short design approved in conversation: migrate naming/hash cases, invalid-input cases, and clone-isolation coverage to public interfaces, then replace its private-helper tests. Execute now on this branch; leave uncommitted.

## Global Constraints
- No production changes or exported test hooks.
- Preserve all 15 valid-name fixtures and 23 invalid-input fixtures.
- Prove isolation through templated declarations, repeated rendering, and stored-state assertions independent of potentially aliased fixtures.
- Preserve existing user edits and historical plans.

## Review Focus
- Normalization, original service identity, truncation boundaries, full-path hashing, and leading-zero suffixes retain exact literal expectations.
- Invalid hosts, roots, service names, and targets fail through the public API rather than incidental missing fixtures.
- Rendering cannot mutate stored declaration slices or retain a previous render's target.

### Task 1: Migrate network-volume coverage

**Files:** Rewrite `network_volume_test.go` in external package `jed_test`, preserving the production-file/test-file naming pair. Replacement coverage was first verified in a separate file before replacing the private-helper tests.

**Interfaces:** Consume `memo.New()`, store setters/getters, `Jed.Spec(ctx, name)`, and `Jed.ProvisionalSpec(ctx, intent, env)`; assert public `Spec.NetworkMounts` results.

- [x] Add Ginkgo DescribeTables carrying every original valid and invalid fixture; assert complete literal mount descriptors, the 32-character bound, and meaningful error categories.
- [x] Add a public-rendering isolation regression that checks stored template declarations remain unchanged across different supplied environments and independent results.
- [x] Run the replacement cases while original tests remain. Since this is a test-only refactor, expect GREEN against existing behavior, not a fabricated missing-feature RED.
- [x] Verify replacement tests catch wrong-name and slice-aliasing regressions using temporary source overlays, without changing tracked production files.
- [x] Remove the old private-helper tests, format, and run gopls diagnostics, `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, `staticcheck ./...`, and `git diff --check`.
- [x] Self-review fixture parity and production-code scope; mark steps complete and leave changes uncommitted. No subagent tool is available.

## Execution outcome

Review rejected the initial table-based migration. It was replaced with behavior-focused `When`/`It` specifications through public APIs, retaining the naming fixtures and validation coverage and separating the isolation checks.

Subsequent approval added tests for an overlong NFS hostname with valid individual labels and an invalid templated local path alongside network volumes. A further approved scope change extended the original test-only plan: `networkVolumeTargets` now returns validated, normalized targets to mount construction, removing the duplicate target validation in `networkVolumeMount`. Public APIs and stored declarations remain unchanged.

At completion, full tests, race tests, gopls diagnostics, vet, staticcheck, build, and whitespace checks passed. Every statement in `network_volume.go` was exercised through public-interface tests.
