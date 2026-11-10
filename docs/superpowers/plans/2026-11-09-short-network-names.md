# Short Network Names Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Derive matching NAS share and Docker volume names within 32 ASCII characters.

**Architecture:** Replace networkVolumeMount naming with a bounded service/basename stem and deterministic FNV-1a suffix. Keep existing transient mounts and resolver collision checks; remove prefix configuration from the render pipeline.

**Tech Stack:** Go standard library (hash/fnv), stdlib table tests, Ginkgo/Gomega.

**Spec:** `docs/superpowers/specs/2026-11-09-short-network-names-design.md`

## Global Constraints
- Always use the low 24 bits of FNV-1a 64-bit, zero-padded to six lowercase hex characters; stem capped at 25 ASCII characters.
- Hash original service name + NUL + validated, normalized full container path.
- Export directly under NFS_ROOT using the final Docker/share name.
- No NETWORK_VOLUME_PREFIX, local-volume changes, live provisioning, or automatic migration.
- Preserve user edits and historical documents. Execute inline in the current checkout as previously requested; leave uncommitted unless authorized.

## Review Focus
- Equivalent paths produce the same name; unsafe dot segments still fail.
- Identical basenames under different parents must remain distinct.
- Long identities sharing a truncated stem must still hash their full inputs.
- A real six-hex collision must fail rather than silently share storage.
- Invalid characters beyond the truncation boundary must not escape validation.

### Task 1: Apply the naming contract throughout the render pipeline

**Files:** `network_volume.go`, `jed.go`, `network_volume_test.go`, `spec_test.go`, `swarm/spec_test.go`, `swarm/deploy_test.go`, `service-yaml.md`, `swarm/README.md`.

**Interfaces:**
- Consumes existing global NFS_HOST/NFS_ROOT and normalized network targets.
- Produces `networkVolumeMount(server, root, serviceName, target string) (NetworkMount, error)` and `(*Jed).resolveNetworkVolumes(ctx context.Context, spec *Spec, server, root string) error`.
- NetworkMount structure and Swarm options remain unchanged.

- [x] Update tests with independently calculated fixtures: postgres /var/lib/postgresql/data => postgres_data_d2546c; app /data => app_data_97d481; app /var/lib/app => app_app_42fbde; app /other/data => app_data_34370a; team/app /data => team_app_data_848b15. Assert export is NFS_ROOT/name. Test normalized repeated/trailing slashes, service flattening, IPv4/IPv6, and unchanged invalid host/root/target behavior.
- [x] Add literal fixtures at stem lengths 24, 25, and 26 and for identical truncated stems with distinct long service/path identities. Assert exact hash/name, <=32 characters, complete stem validation including invalid characters beyond position 25, and original service identity distinguished from a flattened alias.
- [x] Update Spec tests to remove prefix assumptions and verify obsolete prefix values have no effect. Replace old flattening-collision rejection with successful distinct mounts; test matching basenames and actual hash collision: service app targets /dir9979/data and /dir11804/data both yield app_data_c6e29b, requiring an error. Preserve deduplication, conflicts, rendering, and storage isolation coverage.
- [x] Run `go test .` before production edits; expect failures showing old names/exports and old collision behavior. Keep helper signature migration until implementation if needed to get behavioral rather than compilation failures.
- [x] Implement interfaces above, FNV-1a 64-bit identity hash and bounded stem in network_volume.go; remove prefix lookup in jed.go while preserving the user's TODO. Retain resolver final-name collision detection and no-network no-op.
- [x] Update Swarm spec/deploy fixtures to the new postgres name/export, preserving exact NFS option assertions. Format changed Go files and run `gopls check` on each, `go test . ./swarm`; expect no diagnostics and all tests passing.
- [x] Update current user docs with algorithm/identity encoding, examples, single-share export layout, removed prefix, scope of collision detection, upstream ownership requirement, and explicit storage migration warning. Leave historical docs alone.
- [x] Run `go test ./...`, `go vet ./...`, configured lint if available, `git diff --check`; inspect the complete diff and remaining prefix references. Expect passing checks, prefix only in history or intentional obsolete-config tests.
- [x] Self-review against the spec (no subagent tool), record verification and mark progress. Leave changes uncommitted for user review.

## Verification
- Baseline suite passed. New helper fixtures and 11 Spec cases failed against old naming; all passed after implementation.
- `gopls check` on all changed Go files, `go test ./...`, `go vet ./...`, and `git diff --check` passed.
- Configured lint skipped: no golangci-lint configuration in repo. No live NAS deployment performed.
- Final self-review (no subagent tool): no findings. Current-checkout changes left uncommitted; existing TODO preserved.
