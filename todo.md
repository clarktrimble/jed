
- Todo: check that jed/transship commands make sense (some transship should be jed or vice versa)
- Todo: round out commands with CRUD
- Todo: generally smooth and regularize
- Todo: deal with data/jed.db (for example) more easily, envvar perhaps?
- Todo: think thru cooperation with montage, setup/deploy in particular
- Todo: consider tmpfs mount (for grafana)
- Todo: improve deploy UX with an optional rendered-intent preview before deploy; avoid plan/apply abstractions until there is a concrete need
- Todo: add a store/data migration path, perhaps explicit `jed export > services.yaml` and `jed import services.yaml` commands
- Todo: add focused coverage where it buys confidence, e.g. `internal/dockerlog`, swarm restart `any`, and cmd smoke-ish behavior
- Todo: cut back on nil checks, lets rely on New instead
- Todo: add store HTTP routes for intents if/when the HTTP API needs parity
- Todo: add CLI intent commands only if the CLI becomes active again
- Todo: consider a UI-friendly enabled-service view that joins Intent with the selected Service
- Todo: finish removing/rewriting stale docs around old service-level desired state as they surface

## Network-volume branch review follow-ups

Captured from branch review; these are future work, not implemented changes.

- [ ] A-and fix the garbo user docs generated in this branch, sigh
- [ ] A-and fix the superpower planning docs botched in this branch (tasks not marked as done)
- [ ] **Remove the deprecated standalone container runtime.** Standalone is deprecated; the direction is removal, not adding NetworkVolumes support or a new rejection guard. Inventory callers, CLI paths, tests, and documentation before removing `container/` and updating affected references. Current risk until removal: standalone Deploy ignores NetworkVolumes because it only consumes Volumes; Redeploy removes the existing container before calling Deploy. Do not use standalone for services declaring network storage.
- [ ] **Revisit test-style consistency only if worthwhile.** `network_volume_test.go` uses same-package stdlib table tests for private helpers, as prescribed by the implementation plan. This is not the first such test: `spec_render_test.go` predates this feature. Existing tests stay for now; any future style cleanup should preserve helper and clone-isolation coverage without exporting internals merely for tests.
- [ ] **Consider simplifying server validation.** Current validation covers hostname labels and bracketed IPv6 as well as mount-option injection prevention. Decide whether that full input contract is still desired before simplifying; preserve safe option construction, whitespace/delimiter rejection, and intentional IPv6 parsing behavior. This is optional cleanup, not a confirmed defect.
- [ ] **Verify the fixed NFS options on the first real deployment.** The convention emits `nfsvers=3,rw`, with neither `nolock` nor `soft`. Mock tests verify the outgoing payload, not successful NAS mounts. Confirm worker/NAS prerequisites, NFSv3 locking support (including lockd/statd where required), permissions, and representative workload behavior on eligible workers. If the environment needs `nolock`, revisit the convention and its locking implications explicitly; do not add `soft` casually because of its data-integrity risks.
- [ ] **Experiment with existing named-volume reuse.** Reviewer hypothesis, not yet verified here: Docker reuses an existing named volume and ignores newly requested driver options, leaving it on the old export after NETWORK_ROOT changes. Use disposable names in an approved isolated Docker environment; create with one option set, request the same name with changed options, and inspect the actual retained configuration. Check worker-local behavior relevant to Swarm, and record Docker version, procedure, and observations. Update the documented limitation with evidence before relying on retargeting. Choosing mismatch detection, new naming, migration, or replacement is a separate decision; do not alter or delete production volumes as part of the experiment. Related scope: prefix changes and derived-name collisions. See `docs/superpowers/specs/2026-11-05-network-volumes-design.md` and `docs/superpowers/plans/2026-11-05-network-volumes.md` for the original deferred investigation.

## better group support

 Caveat: this uses Docker socket GID as the container’s primary group. Longer-term, the cleaner model would be adding supplementary groups
 support to Jed service definitions, mapping to Docker ContainerSpec.Groups, e.g.:

 ```yaml
   user: "65534:65534"
   groups:
     - "{{DOCKER_GID}}"
 ```
