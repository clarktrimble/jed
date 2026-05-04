# cmd/jed

`jed` edits service and env definitions in a Jed store.

It does not talk to Docker. Use `transship` to deploy stored services to Docker Swarm.

## Store

By default, `jed` uses `jed.db` in the current directory.

```text
-d, --db string   path to bbolt database (default "jed.db")
```

Example:

```sh
jed --db prod.db ls-svc
```

## Service Commands

### List services

```sh
jed ls-svc
```

Output:

```text
postgres	postgres:16
redis	redis:7
```

### Get a service

```sh
jed get-svc postgres
```

Prints the stored service as YAML.

### Set a service

```sh
jed set-svc postgres.yaml
```

The YAML is parsed into `jed.Service`, validated, and stored by `name`.

See [../../service-yaml.md](../../service-yaml.md) for the service YAML format.

### Delete a service

```sh
jed del-svc postgres
```

## Env Commands

Env is stored separately from service definitions. Service env participates in template rendering and is passed to the container runtime environment.

### List envs

```sh
jed ls-env
```

Output:

```text
postgres	3 vars
_global	1 vars
```

### Get env

```sh
jed get-env postgres
```

Prints `.env` style lines:

```text
POSTGRES_USER=app
POSTGRES_DB=app
```

### Set env

```sh
jed set-env postgres postgres.env
```

The file is parsed as `.env` data and stored under the provided name.

### Delete env

```sh
jed del-env postgres
```

## Render Vars

Env entries can also be used as render vars for `{{VAR}}` placeholders in service command args and labels. The `transship` CLI convention is to load deployment-level render vars from `_global`:

```sh
jed set-env _global global.env
```

Example `global.env`:

```text
VHOST=app.example.com
```

Service env wins over deployment-level render vars on key collisions. Deployment-level render vars are not added to the container environment unless they are also present in the service's own env.
