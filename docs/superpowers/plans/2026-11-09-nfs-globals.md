# Separate NFS Globals Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace NETWORK_ROOT with upstream-compatible NFS_HOST and NFS_ROOT globals.

**Architecture:** Pass separate global host/root values through Jed.Spec to the existing resolver. Validate them separately and retain transient NetworkMount and Swarm output contracts.

**Tech Stack:** Go, stdlib tests, Ginkgo/Gomega.

**Spec:** `docs/superpowers/specs/2026-11-09-nfs-globals-design.md`

## Global Constraints
- No NETWORK_ROOT fallback.
- Both new values are global and required only for network declarations.
- Keep NETWORK_VOLUME_PREFIX, mount naming, export derivation, and NFS options unchanged.
- No provisioning, volume replacement, or standalone changes.
- Preserve unrelated todo.md edits; leave historical plans/specs intact.

## Review Focus
- Bare IPv6 must reach the unchanged Swarm mount payload without combined-string parsing.
- Missing globals must fail with the relevant setting name, even when legacy NETWORK_ROOT exists.
- Service-specific values must not override the globals.
- Malformed host/port/options and unsafe export paths must remain rejected.
- Services without network declarations must ignore invalid or missing NFS configuration.

### Task 1: Replace the NFS configuration contract

**Files:**
- Modify: `network_volume.go` (validation and resolver parameters), `jed.go` (global lookup).
- Test: `network_volume_test.go`, `spec_test.go`.
- Modify: `service-yaml.md`, `swarm/README.md` (current configuration docs).

**Interfaces:**
- Consumes: global render-vars map and existing Service.NetworkVolumes.
- Produces: `networkVolumeMount(server, root, prefix, serviceName, target string) (NetworkMount, error)` and `(*Jed).resolveNetworkVolumes(ctx context.Context, spec *Spec, server, root, prefix string) error`.
- NetworkMount and Swarm driver options are unchanged.

- [x] Update helper tests to pass separate hosts/roots: hostname, IPv4, bare IPv6, default/custom prefix, flattened service, trailing slashes. Preserve expected NetworkMount values. Reject empty host/root, bracketed/malformed hosts, host ports/options/whitespace, relative/root/dot/NUL export paths, and existing invalid target/name cases.
- [x] Update Spec tests for separate global configuration, reloads, service-env isolation, independently invalid/missing host/root, legacy-only configuration rejection, and no-network no-op. Verify bare IPv6 survives Spec resolution. Preserve existing conflict, rendering, and storage-isolation coverage.
- [x] Run `go test .` and verify the new tests fail because the separate configuration contract is not implemented.
- [x] Implement the signatures above. Validate server with net.ParseIP before rejecting colons; otherwise preserve DNS validation. Validate root directly with cleanLocalPath("NFS_ROOT", root) and reject NUL. Read only varsEnv.Vars["NFS_HOST"] and varsEnv.Vars["NFS_ROOT"] in jed.go. Do not parse or read NETWORK_ROOT.
- [x] Run gofmt on changed Go files, `gopls check network_volume.go jed.go network_volume_test.go spec_test.go`, and `go test .`; expect no diagnostics and passing tests.
- [x] Update current docs with separate NFS_HOST/NFS_ROOT examples, bare IPv6 syntax, global-only behavior, and existing-volume caveat. Do not rewrite historical docs or unrelated todo.md.
- [x] Run `go test ./...`, configured lint if available, `git diff --check`, and inspect the diff and remaining NETWORK_ROOT references. Expect passing tests, clean diff, and legacy references only in historical documents or intentional rejection tests.
- [x] Review changes against the design and mark completed steps. Changes left uncommitted; commit only when authorized.

## Execution notes
- Baseline `go test ./...` passed; updated Spec tests then failed on the missing separate-global contract (13 failures), and passed after implementation.
- `gopls check`, `go test ./...`, `go vet ./...`, and `git diff --check` passed. Configured lint skipped: no golangci-lint configuration in this repo.
- Final self-review (no subagent tool): no findings. Unrelated `todo.md` changes preserved. No live NFS deployment performed.
