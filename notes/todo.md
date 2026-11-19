
## mostly olden stuff here yeah

- Todo: check that jed/transship commands make sense (some transship should be jed or vice versa)
- Todo: round out commands with CRUD
- Todo: generally smooth and regularize
- Todo: deal with data/jed.db (for example) more easily, envvar perhaps?
- Todo: think thru cooperation with montage, setup/deploy in particular
- Todo: consider tmpfs mount (for grafana)
- Todo: improve deploy UX with an optional rendered-intent preview before deploy; avoid plan/apply abstractions until there is a concrete need
- Todo: add a store/data migration path, perhaps explicit `jed export > services.yaml` and `jed import services.yaml` commands
- Todo: add focused coverage where it buys confidence, e.g. swarm restart `any` and cmd smoke-ish behavior (`Swarm.TaskLogs` decoder coverage is now in place)
- Todo: investigate why swarm maps `on-failure` to one restart attempt and what relies on that behavior. Preserve the limit for now; changing the default restart condition to `any` does not authorize changing this limit.
- Todo: consolidate Swarm construction around `swarm.Config.New` and consider an `export_test.go` helper for local mock-client injection. Keep public `swarm.New(client, logger)` for now: downstream tests use it, and their migration needs to be addressed before removing it.
- Todo: cut back on nil checks, lets rely on New instead
- Todo: add store HTTP routes for intents if/when the HTTP API needs parity
- Todo: add CLI intent commands only if the CLI becomes active again
- Todo: consider a UI-friendly enabled-service view that joins Intent with the selected Service
- Todo: finish removing/rewriting stale docs around old service-level desired state as they surface

## Network-volume branch review follow-ups

**Experiment with existing named-volume reuse.** Reviewer hypothesis, not yet verified here: Docker reuses an existing named volume and ignores newly requested driver options, leaving it on the old export after NETWORK_ROOT changes. Use disposable names in an approved isolated Docker environment; create with one option set, request the same name with changed options, and inspect the actual retained configuration. Check worker-local behavior relevant to Swarm, and record Docker version, procedure, and observations. Update the documented limitation with evidence before relying on retargeting. Choosing mismatch detection, new naming, migration, or replacement is a separate decision; do not alter or delete production volumes as part of the experiment. Related scope: prefix changes and derived-name collisions. See `docs/superpowers/specs/2026-11-05-network-volumes-design.md` and `docs/superpowers/plans/2026-11-05-network-volumes.md` for the original deferred investigation.

## better group support

 Caveat: this uses Docker socket GID as the container’s primary group. Longer-term, the cleaner model would be adding supplementary groups
 support to Jed service definitions, mapping to Docker ContainerSpec.Groups, e.g.:

 ```yaml
   user: "65534:65534"
   groups:
     - "{{DOCKER_GID}}"
 ```
