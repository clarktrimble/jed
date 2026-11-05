# NetworkVolumes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let Swarm services declare container paths whose NFSv3 mounts Jed derives by convention.

**Architecture:** Preserve Service.Volumes and the existing local-volume fold. Add Service.NetworkVolumes declarations and transient, runtime-neutral Spec.NetworkMounts descriptors, resolved using global vars. The Swarm adapter converts those descriptors into Docker driver options; no resolved mounts are persisted.

**Tech Stack:** Go, existing Ginkgo/Gomega and standard testing, Docker Swarm HTTP API, existing stores. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-11-05-network-volumes-design.md`

## Global Constraints

- Existing `volumes` and `local_volumes` formats stay unchanged.
- NETWORK_ROOT combines the NFS server and export root in one value.
- NETWORK_VOLUME_PREFIX defaults to `jed` when unset.
- Driver and NFSv3 read/write options are fixed by the convention.
- Explicit Volumes wins over a conventional mount at the same container target; log the conflict.
- LocalVolumes and NetworkVolumes declaring the same container target is an error.
- Initial scope is Docker Swarm. Do not change standalone container deployment.
- Do not persist resolved mounts or introduce a volume inventory.
- No NAS provisioning, remote directory creation, ownership changes, volume replacement, or storage migration.
- Existing named-volume behavior remains an investigation, not a selected policy. No mismatch probes or recreation logic in this implementation.

## Review Focus

- Service env values must not override global NETWORK_ROOT/NETWORK_VOLUME_PREFIX configuration — Task 2.
- Templates and trailing slashes must not hide a local/network target conflict — Task 2.
- Repeated rendering must not mutate stored slices or retain an earlier root/prefix — Tasks 1–2.
- Malformed server/prefix values must not inject NFS mount options or invalid Docker names — Task 1.
- Passing unresolved NetworkVolumes directly to the Swarm adapter must fail rather than silently omit requested storage — Task 3.

---

## File Map and Implementation Calls

- `service.go`: add `NetworkVolumes []string` tagged `json:"network_volumes,omitempty"`; validate targets like LocalVolumes, allowing templates before rendering.
- `spec.go`: clone the new slice; add `NetworkMounts []NetworkMount` tagged `json:"network_mounts,omitempty"` to Spec.
- New `network_volume.go`: convention, validation, transient descriptor, conflict preflight, network resolution.
- `jed.go`: preflight conventional target conflicts, retain local resolution, then resolve network declarations.
- `swarm/spec.go`: optional Docker VolumeOptions/DriverConfig types and conversion into Mounts.
- Tests: new `network_volume_test.go`; existing `service_test.go`, `spec_test.go`, `swarm/spec_test.go`, `swarm/deploy_test.go`, `store/contract.go`.
- Documentation: `service-yaml.md`, `swarm/README.md`, and this plan's investigation note.

Transient descriptor (implementation choice, not a stored service schema):

```go
type NetworkMount struct {
    Name   string `json:"name"`
    Target string `json:"target"`
    Server string `json:"server"`
    Export string `json:"export"`
}
```

Default empty prefix to `jed`, including an explicitly empty var. Normalize trailing slashes and reuse flatPathPart/cleanLocalPath. Hostnames/IPv4 and bracketed IPv6 roots are supported; reject whitespace, mount-option delimiters, and malformed server components. Unwrap IPv6 brackets for the Server value. Generated Docker names must match `[A-Za-z0-9][A-Za-z0-9_.-]*`.

Check local/network intersections before either fold clears its declaration list, even when an explicit mount also targets that path. Compare cleaned targets for the new conflict rules without changing legacy explicit/local behavior. Deduplicate identical network targets; reject distinct targets that flatten to the same network-volume name within one spec. Cross-service/existing-volume name collisions belong to the deferred investigation.

## Execution Preparation

- [ ] Use the using-git-worktrees skill before implementation; preserve these planning documents in the isolated workspace and leave user files alone.
- [ ] Establish a baseline with `go test ./... -count=1`; investigate/report pre-existing failures before editing.
- [ ] Inspect `gopls symbols` for each central file before editing. Run `gopls references -d spec.go:12:6` and `gopls references -d swarm/spec.go:136:6` to identify consumers of the extended exported structures.

### Task 1: Model, Convention, and Persistence Compatibility

**Files:** create `network_volume.go`, `network_volume_test.go`; modify `service.go`, `spec.go`, `service_test.go`, `store/contract.go`.

**Interfaces:**
- Consumes existing `cleanLocalPath(kind, value string) (string, error)` and `flatPathPart(kind, value string) (string, error)`.
- Produces `NetworkMount`, `Spec.NetworkMounts`, `Service.NetworkVolumes`.
- Produces `networkVolumeMount(networkRoot, prefix, serviceName, target string) (NetworkMount, error)`.

- [ ] **Step 1: Add failing model and convention tests.**

In same-package `network_volume_test.go`, `TestNetworkVolumeMount` asserts:

```go
got, err := networkVolumeMount("nas:/exports/jed", "", "postgres", "/var/lib/postgresql/data")
want := NetworkMount{
    Name: "jed-postgres-var_lib_postgresql_data",
    Target: "/var/lib/postgresql/data",
    Server: "nas",
    Export: "/exports/jed/postgres/_var_lib_postgresql_data",
}
if err != nil || got != want { t.Fatalf("got %#v, %v; want %#v", got, err, want) }
```

Add table cases named `custom_prefix`, `trailing_slashes`, `flattened_service`, and `ipv6`: respectively expect `team-postgres-var_lib_postgresql_data`, the same cleaned default result, `jed-team_app-data` plus `/exports/jed/team_app/_data`, and Server `2001:db8::10` from `[2001:db8::10]:/exports/jed`.

`TestNetworkVolumeMountInvalid` must return an error for empty root, missing server/export, relative export, export `/`, dot segments in export/target/service, targets `data`, `/`, and empty, server `nas,rw`, whitespace in server, prefix `.bad`, and a target generating an invalid Docker name such as `/bad:name`.

In `service_test.go`, add named NetworkVolumes cases that reject the same untemplated bad paths as LocalVolumes, accept `{{VOL_PATH}}` before rendering, and round-trip `network_volumes` through JSON and YAML. Assert empty declarations omit the field. Test `cloneService` in the same-package test file: editing the clone's NetworkVolumes must leave the original unchanged.

Extend the existing service fixture in `store/contract.go` with `NetworkVolumes: []string{"/var/lib/postgresql/data"}` so all store implementations prove declaration round-trip equality.

- [ ] **Step 2: Run failing tests.**

Run `go test . ./store/...`; expect compilation failure for missing NetworkVolumes/networkVolumeMount before implementation. Do not accept unrelated failures as the red result.

- [ ] **Step 3: Implement the declared interfaces and validation.**

Add the additive fields, slice clone, and pure convention helper; reuse existing path helpers. Validate both before rendering where possible and after expansion through the convention helper. Leave DBSchemaVersion unchanged: this is an additive optional service field.

- [ ] **Step 4: Format, diagnose, and verify.**

Run `gofmt -w network_volume.go network_volume_test.go service.go spec.go service_test.go store/contract.go`, then `gopls check` on those files, then `go test . ./store/...`. Expect no diagnostics and PASS.

- [ ] **Step 5: Commit this tested unit.**

`git add network_volume.go network_volume_test.go service.go spec.go service_test.go store/contract.go && git commit -m "feat: define network volume convention"`

### Task 2: Resolve Network Declarations During Spec Rendering

**Files:** modify `network_volume.go`, `jed.go`, `spec_test.go`.

**Interfaces:**
- Consumes Task 1's `networkVolumeMount` and `Spec.NetworkMounts`.
- Produces `validateConventionalVolumeTargets(service Service) error`.
- Produces `(j *Jed) resolveNetworkVolumes(ctx context.Context, spec *Spec, networkRoot, prefix string) error`.
- Public `Jed.Spec` and `Jed.SpecWithEnv` signatures remain unchanged.

- [ ] **Step 1: Add failing `When("the service has network volumes")` cases inside the existing Spec fixture in `spec_test.go`.**

Use service `app`, image `local/app:v1`, global NETWORK_ROOT `nas:/exports/jed`, and declaration `/var/lib/app`. Assert:

```go
Expect(spec.NetworkMounts).To(Equal([]jed.NetworkMount{{
    Name: "jed-app-var_lib_app", Target: "/var/lib/app",
    Server: "nas", Export: "/exports/jed/app/_var_lib_app",
}}))
Expect(spec.Service.NetworkVolumes).To(BeNil())
stored, getErr := store.GetService(ctx, "app", "local/app:v1")
Expect(getErr).NotTo(HaveOccurred())
Expect(stored.NetworkVolumes).To(Equal([]string{"/var/lib/app"}))
```

Add named cases with these exact assertions:
- `uses global network configuration`: service env sets both network config keys differently; mount still uses global root/prefix.
- `reloads network configuration`: render again after changing global root to `other:/exports/new` and prefix to `team`; expect Server `other`, Export `/exports/new/app/_var_lib_app`, Name `team-app-var_lib_app`, with stored service unchanged.
- `renders paths before deriving mounts`: `{{VOL_PATH}}` becomes `/var/lib/app`; a rendered dot segment errors.
- `requires root only for network declarations`: missing/malformed root errors for the network fixture; no network declarations with invalid network vars still renders ordinary/local services.
- `explicit volumes win`: `/explicit` mapped to `/var/lib/app`; expect no NetworkMounts, explicit entry retained, declaration cleared, one conflict error logged.
- `local and network targets conflict`: local `/var/lib/app/` and network templated `/var/lib/app` return an error before LOCAL_ROOT resolution, including when an explicit volume also uses that target.
- `keeps unrelated local mounts`: local `/cache` still becomes `/opt/bastille/app/_cache`, while network mount remains present.
- `deduplicates network targets`: `/data` and `/data/` produce one mount.
- `rejects flattened network name collisions`: `/a/b` and `/a_b` return an error, not two mounts sharing a name.
- `SpecWithEnv resolves network paths`: supplied env renders the target without saving that env or mutating the service.

- [ ] **Step 2: Run the focused suite to verify failure.**

Run `go test . -run '^TestJed$' -ginkgo.focus='network volumes' -count=1`; expect assertions to fail because NetworkMounts are absent.

- [ ] **Step 3: Implement preflight and network resolution.**

Make preflight and network resolution no-ops when NetworkVolumes is empty. After `render`, call the preflight before existing `resolveLocalVolumes`; then resolve network declarations with `varsEnv.Vars["NETWORK_ROOT"]` and `varsEnv.Vars["NETWORK_VOLUME_PREFIX"]`. Skip explicit-target conflicts with an error log; append derived NetworkMounts for remaining targets and clear only the rendered NetworkVolumes slice. Do not put generated named sources in Service.Volumes too, which would create duplicate Docker mounts. No store writes or Docker calls.

- [ ] **Step 4: Format, diagnose, and verify.**

Run `gofmt -w network_volume.go jed.go spec_test.go`, `gopls check network_volume.go jed.go spec_test.go`, and `go test . -count=1`. Expect no diagnostics and PASS, including all legacy local-volume cases.

- [ ] **Step 5: Commit this tested unit.**

`git add network_volume.go jed.go spec_test.go && git commit -m "feat: resolve network volumes in rendered specs"`

### Task 3: Emit Swarm NFS Mounts and Document Usage

**Files:** modify `swarm/spec.go`, `swarm/spec_test.go`, `swarm/deploy_test.go`, `service-yaml.md`, `swarm/README.md`.

**Interfaces:**
- Consumes `jed.Spec.NetworkMounts []jed.NetworkMount` from Tasks 1–2.
- Add `Mount.VolumeOptions *VolumeOptions` tagged `json:"VolumeOptions,omitempty"`.
- Add `VolumeOptions.DriverConfig *DriverConfig` tagged `json:"DriverConfig,omitempty"`.
- Add `DriverConfig.Name string` and `DriverConfig.Options map[string]string` tagged `Name` and `Options`.
- Produce `swarmNetworkMounts(networkMounts []jed.NetworkMount) []Mount` in `swarm/spec.go`.
- Existing public Swarm.Spec/Deploy signatures stay unchanged.

- [ ] **Step 1: Add failing adapter and outgoing-request tests.**

In `swarm/spec_test.go`, add a separate `Describe("network volumes")` using newSpecMock, nopLogger, and no secrets. Feed a jed.Spec with the Task 1 postgres descriptor. Marshal the returned body and assert the mount contains:

```go
Expect(mount["Type"]).To(Equal("volume"))
Expect(mount["Source"]).To(Equal("jed-postgres-var_lib_postgresql_data"))
Expect(mount["Target"]).To(Equal("/var/lib/postgresql/data"))
Expect(options).To(Equal(map[string]any{
    "type": "nfs", "o": "addr=nas,nfsvers=3,rw",
    "device": ":/exports/jed/postgres/_var_lib_postgresql_data",
}))
```

Here `mount` is the decoded Mounts entry and `options` is its VolumeOptions.DriverConfig.Options; also assert DriverConfig.Name is `local`.

Named cases: `keeps existing mounts` includes `/srv/certs` → `/certs` and `ordinary-data` → `/ordinary` alongside NFS, comparing without ordering assumptions; both ordinary entries omit VolumeOptions. `no network declarations` preserves existing wire format. `rejects unresolved network declarations` passes Service.NetworkVolumes without resolved descriptors; expect an error and zero Docker client calls.

In `swarm/deploy_test.go`, add a network case calling Deploy with a resolved descriptor. Capture POST `/services/create` and assert its marshaled Mounts contains the same driver configuration. Assert no requests to `/volumes` or other provisioning endpoints. This tests the real outgoing payload without an NAS or live Docker daemon.

- [ ] **Step 2: Run focused tests to verify failure.**

Run `go test ./swarm -run '^TestSwarm$' -ginkgo.focus='network volumes' -count=1`; expect missing network-mount assertions to fail before adapter implementation.

- [ ] **Step 3: Implement conversion and fail-closed unresolved input.**

Append converted network mounts to existing `swarmMounts(service.Volumes)` in buildSpec. Construct `local` driver options exactly as asserted. In Swarm.Spec, reject remaining Service.NetworkVolumes before secret/config lookups, instructing callers to use Jed.Spec. Do not create/check/delete Docker volumes, mount NFS on the host, or call the NAS.

- [ ] **Step 4: Document the declaration and convention.**

Add `network_volumes` to the YAML example/table and explain global NETWORK_ROOT, prefix/default, derived names, explicit/local conflict rules, external directory/permission preparation, and Swarm-only scope. Explain that generated settings are not stored. Mark existing named-volume reuse after root changes as uninvestigated: changing configuration is not a documented retargeting mechanism. Do not claim replacement or mismatch protection.

- [ ] **Step 5: Format, diagnose, and run final validation.**

Run `gofmt -w swarm/spec.go swarm/spec_test.go swarm/deploy_test.go` and `gopls check` on every changed Go file. Then run:

```bash
go test ./... -count=1
go test -race . ./swarm ./store/... -count=1
go vet ./...
staticcheck ./...
git diff --check
```

Expect PASS/exit 0 for each; staticcheck is the configured Makefile lint alongside go vet and is available in the inspected workspace. Do not run destructive `make clean` or unnecessary generation. No live NAS test is authorized or implied by mock success.

- [ ] **Step 6: Commit this tested unit.**

`git add swarm/spec.go swarm/spec_test.go swarm/deploy_test.go service-yaml.md swarm/README.md && git commit -m "feat: emit conventional NFSv3 mounts for swarm"`

## Deferred Investigation Receipt

Existing named Docker volumes may retain creation-time driver settings. Follow-up work must determine behavior on each worker when NETWORK_ROOT/prefix or a derived name's backing export changes, including cross-service name collisions. This plan intentionally neither resolves that question nor authorizes volume replacement. Report the limitation with the implementation.

## Plan Self-Review

- Spec coverage: declaration/configuration/naming → Tasks 1–2; transient output and Swarm scope → Task 3; conflicts → Task 2; persistence → Tasks 1–2; deferred investigation → documentation and receipt above.
- Step scan: every implementation signature, output value, focused test command, and commit boundary is specified.
- Type consistency: NetworkMount.Name/Target/Server/Export flow unchanged from the root convention to swarmNetworkMounts; Docker-only types stay in swarm.
- Review Focus: all five conditions have named tests in their owning tasks.
- Proportion: three units, with no function-body prescriptions beyond the agreed wire contract; no unrelated refactor or persistent mount schema.
