# cmd/jed

`jed` edits service and env definitions in a Jed store. It does not talk to Docker; use `transship` to deploy stored services to Docker Swarm.

## Store

```text
-d, --db string          path to bbolt database (default "jed.db")
--skip-schema-check      open database without validating schema version
```

## Commands

Version:

```sh
jed --version
```

Service definitions:

```sh
jed ls-svc
jed get-svc NAME
jed set-svc SERVICE.yaml
jed del-svc NAME
```

`set-svc` parses YAML into `jed.Service`, validates it, and stores it by the service's `name` field. See [../../service-yaml.md](../../service-yaml.md) for the YAML format.

Environment definitions:

```sh
jed ls-env
jed get-env NAME
jed set-env NAME ENVFILE
jed del-env NAME
```

Env files use `.env` syntax. Env is stored separately from service definitions, participates in template rendering, and is passed to the container runtime environment.

## Render Vars

Env entries can also render `{{VAR}}` placeholders in service command args, labels, and about link URLs. The `transship` CLI convention is to load deployment-level render vars from `_global` when rendering:

```sh
jed set-env _global global.env
```

Service env wins over deployment-level render vars on key collisions. Deployment-level render vars are not added to the container environment unless they are also present in the service's own env.
