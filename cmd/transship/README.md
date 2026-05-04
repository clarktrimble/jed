# cmd/transship

`transship` operates Docker Swarm using service and env definitions from a Jed store.

The deploy path is:

1. Open the Jed store.
2. Load render vars from `_global` env with `jed.New(ctx, store, "_global")`.
3. Render the named service with `j.Spec(ctx, name)`.
4. Deploy the rendered `jed.Spec` with `swarm.Deploy`.

`_global` is optional because missing envs return an empty env.

## Flags

```text
-d, --db string       path to jed store (default "jed.db")
-s, --socket string   docker socket path (default "/var/run/docker.sock")
```

Example:

```sh
transship --db prod.db --socket /var/run/docker.sock ls-services
```

## Deploy

```sh
transship deploy reauth-acp
```

Deploy creates the swarm service if it does not exist and updates it if it does.

On success, deploy prints a short summary:

```text
service: reauth-acp (local/reauth-acp:3c070f2)
  secrets: [aruba_client_secret]
  env: 4 vars
created reauth-acp (abc123def456)
```

or:

```text
updated reauth-acp
```

## Services

### List swarm services

```sh
transship ls-services
```

Output:

```text
abc123def456	reauth-acp
```

### Inspect a swarm service

```sh
transship inspect reauth-acp
```

Prints Docker's stored swarm service spec as formatted JSON.

### Delete a swarm service

```sh
transship delete-service reauth-acp
```

## Tasks and Logs

### Show tasks for a service

```sh
transship tasks reauth-acp
```

Output includes timestamp, task ID, state, image, and error:

```text
2026-02-27T22:52:41	chw715vucthx	running	local/app:v1	
```

### Show logs for a task

```sh
transship logs chw715vucthx
transship logs -n 500 chw715vucthx
```

The argument is a swarm task ID, not a service name.

### Stream Docker events

```sh
transship events
```

## Secrets

Swarm secrets are immutable. `transship create-secret` creates a new versioned secret using the base name you provide.

```sh
printf 'secret-value' | transship create-secret db_password
```

If stdin is a terminal, `transship` prompts for the secret value.

List secrets:

```sh
transship ls-secrets
```

## Configs

Swarm configs are immutable. `transship create-config` creates a new versioned config from a file.

```sh
transship create-config app_config ./config.yaml
```

List configs:

```sh
transship ls-configs
```

## Networks

Create an overlay network:

```sh
transship create-network svc-net
```

Options:

```text
-a, --attachable   allow manual container attachment (default true)
-e, --encrypted    encrypt overlay traffic
```

## Related Commands

Use [`cmd/jed`](../jed/README.md) to edit stored service and env definitions before deploying.

Common setup flow:

```sh
jed set-svc reauth-acp.yaml
jed set-env reauth-acp reauth-acp.env
jed set-env _global global.env
transship deploy reauth-acp
```
