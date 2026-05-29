
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

## better group support

 Caveat: this uses Docker socket GID as the container’s primary group. Longer-term, the cleaner model would be adding supplementary groups
 support to Jed service definitions, mapping to Docker ContainerSpec.Groups, e.g.:

 ```yaml
   user: "65534:65534"
   groups:
     - "{{DOCKER_GID}}"
 ```
