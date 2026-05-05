# cmd/transship

`transship` operates Docker Swarm using service and env definitions from a Jed store.

Deploy flow:

1. Open the Jed store.
2. Load render vars from `_global` with `jed.New(ctx, store, "_global")`.
3. Render the named service with `j.Spec(ctx, name)`.
4. Deploy the rendered `jed.Spec` with `Swarm.Deploy`.

`_global` is optional because missing envs return an empty env.

## Flags

```text
-d, --db string       path to jed store (default "jed.db")
-s, --socket string   docker socket path (default "/var/run/docker.sock")
```

## Commands

Version:

```sh
transship --version
```

Deploy and service inspection:

```sh
transship deploy NAME          # create or update service
transship ls-services          # list swarm services
transship inspect NAME         # print Docker's service spec JSON
transship delete-service NAME  # delete swarm service
```

Tasks, logs, and events:

```sh
transship tasks NAME
transship logs [-n N] TASK_ID
transship events
```

Secrets and configs are immutable; create commands write the next versioned swarm resource for the base name:

```sh
printf 'secret-value' | transship create-secret db_password
transship ls-secrets
transship create-config app_config ./config.yaml
transship ls-configs
```

Networks:

```sh
transship create-network [-a] [-e] NAME
```

Options:

```text
-a, --attachable   allow manual container attachment (default true)
-e, --encrypted    encrypt overlay traffic
```

## Common Setup

Use [`cmd/jed`](../jed/README.md) to edit stored service and env definitions before deploying:

```sh
jed set-svc reauth-acp.yaml
jed set-env reauth-acp reauth-acp.env
jed set-env _global global.env
transship deploy reauth-acp
```
