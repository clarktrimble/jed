# Service YAML

Define a `jed.Service` for deployment.

## Example

```yaml
name: reauth-acp
integration:
  name: aruba
  desc: Aruba access-control integration
  versions:
    - v1.2.3
image: local/reauth-acp:3c070f2

restart: on-failure

ports:
  3031/tcp: "8012"

volumes:
  svc-data: /data

local_volumes:
  - /qkview-history

network_volumes:
  - /var/lib/app

secrets:
  - aruba_client_secret

configs:
  myapp_config: /etc/myapp/config.yaml

hosts:
  - "10.35.44.41 container4"

resources:
  cpu_limit: "0.5"
  mem_limit: "128M"
  cpu_reserve: "0.1"
  mem_reserve: "64M"
```

## Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Service name |
| `integration` | no | Integration name, optional description, and versions |
| `image` | yes | Docker image with tag |
| `about` | no | User-facing description, links, and notes |
| `network` | no | Swarm network name; defaulted to `svc-net` |
| `command` | no | Override container command |
| `labels` | no | Container labels |
| `ports` | no | Container port to host port mapping |
| `volumes` | no | Named volume or host path to container path |
| `local_volumes` | no | Container paths backed by convention-derived local bind mounts from `LOCAL_ROOT` |
| `network_volumes` | no | Container paths backed by convention-derived NFSv3 mounts from global `NFS_HOST` and `NFS_ROOT` (Swarm only) |
| `secrets` | no | Swarm secret base names; latest version resolved at deploy |
| `configs` | no | Swarm config base names mapped to target mount paths |
| `hosts` | no | Extra `/etc/hosts` entries |
| `resources` | no | CPU and memory limits/reservations |
| `restart` | no | Restart policy (`none`, `on-failure`, `any`) |
| `publish_mode` | no | Port publish mode: `host` for direct binding; default is ingress |
| `user` | no | Container user, e.g. `1001` or `1000:967`; default is `1001` |
| `traefik` | no | Traefik routing config; generates labels automatically |

## Integration

`integration` identifies the integration name and versions shared by related services. `name` must be an integration slug using lower-case letters, digits, and single hyphens, e.g. `aruba` or `reauth-acp`. `versions` must contain one or more full semantic versions with leading `v`, e.g. `v1.2.3`. Duplicates are rejected.

`desc` is an optional human-readable description of the integration, distinct from `about.desc`, which describes the individual service. Empty descriptions are omitted when serialized. Like the other integration fields, `desc` is literal metadata and is not template-expanded. Descriptions are stored per service; consistency across services with the same integration name is not enforced.

```yaml
integration:
  name: reauth-acp
  desc: Reauthorization access-control integration
  versions:
    - v1.0.13
```

## Template Expansion

String values in a service definition may contain `{{VAR}}` placeholders. They are rendered by `jed.Jed.Spec` before runtime deploy. Integration fields are literal metadata: they are validated before rendering and are not template-expanded.

```yaml
command:
  - "prometheus"
  - "--web.external-url=https://{{VHOST}}/prometheus"
labels:
  prometheus_host: "{{PROM_HOST}}"
```

Service values render from caller-supplied vars plus the service env. Service env wins on collisions. Env values render from caller-supplied vars only; env vars do not template each other. Caller-supplied render vars are not added to the container environment.

Expansion is single-pass. Missing variables fail spec rendering. CLI-specific render-var conventions are documented with the CLIs.

## Restart

Restart condition is one of `none`, `on-failure`, or `any`. Empty means `none`. For swarm, `on-failure` uses one restart attempt.

```yaml
restart: on-failure
```

## Resources

CPU values are decimal strings, e.g. `"0.5"` for half a CPU or `"2.0"` for two CPUs.

Memory values are strings with an `M` suffix, e.g. `"128M"`.

| Field | Default | Description |
|-------|---------|-------------|
| `cpu_limit` | `0.5` | Maximum CPU |
| `mem_limit` | `128M` | Maximum memory |
| `cpu_reserve` | `0.1` | Reserved CPU |
| `mem_reserve` | `64M` | Reserved memory |

## Ports

Format: `"container_port/protocol": "host_port"`.

```yaml
ports:
  3031/tcp: "8012"
  53/udp: "5353"
```

Set `publish_mode: host` for direct host-mode publishing. Empty or omitted uses Docker Swarm ingress/routing mesh behavior.

## Volumes

Named volumes use a simple name. Bind mounts use an absolute host path.

```yaml
volumes:
  svc-data: /data          # named volume
  /host/path: /container   # bind mount
```

`local_volumes` lists absolute container paths whose host-side bind paths are derived from the global `LOCAL_ROOT` render var, service name, and a flattened container path:

```yaml
local_volumes:
  - /qkview-history        # ${LOCAL_ROOT}/qkview/_qkview-history:/qkview-history
  - /qkview-sites          # ${LOCAL_ROOT}/qkview/_qkview-sites:/qkview-sites
  - /var/lib/app           # ${LOCAL_ROOT}/qkview/_var_lib_app:/var/lib/app
```

If an explicit `volumes` entry already mounts the same container path, the explicit volume wins and Jed logs an error.

### Network volumes (Swarm)

Declare only the container paths; Jed derives the NFSv3 mount settings:

```yaml
name: postgres
network_volumes:
  - /var/lib/postgresql/data
```

Set these values in the global render-vars env (`_global` by default), alongside `LOCAL_ROOT`:

```yaml
name: _global
vars:
  NFS_HOST: nas
  NFS_ROOT: /exports/jed
  NETWORK_VOLUME_PREFIX: jed
```

`NFS_HOST` is the NAS hostname or IP address; `NFS_ROOT` is the absolute export root. Both are required only for services declaring network volumes. Use bare IPv6 addresses, for example `NFS_HOST: '2001:db8::10'`. `NETWORK_VOLUME_PREFIX` defaults to `jed` when absent or empty. Service env values do not override these global configuration values. The former combined `NETWORK_ROOT` setting is no longer supported.

The example derives:

```text
Docker volume: jed-postgres-var_lib_postgresql_data
NAS directory: /exports/jed/postgres/_var_lib_postgresql_data
Container:     /var/lib/postgresql/data
```

Directory naming follows `LocalVolumes`: service name, then the container path flattened with underscores and prefixed with `_`. The Docker name joins the prefix, flattened service name, and flattened target without that leading underscore. Template expansion happens first; absolute paths below `/` are required, and `.`/`..` segments are rejected. Derived Docker names and the prefix must use letters, digits, `_`, `.`, or `-`, starting with a letter or digit.

An explicit `volumes` entry at the same target wins and Jed logs the conflict. A target declared in both `local_volumes` and `network_volumes` is an error, even if an explicit mount also uses it. Repeated equivalent network targets are deduplicated; distinct targets that flatten to the same volume name within a spec are rejected.

`Jed.Spec()` generates transient `NetworkMounts` descriptors, and the Swarm adapter emits Docker `local` driver settings with `type=nfs`, `nfsvers=3`, and read/write access. Stored service declarations remain unchanged; resolved mounts are not persisted. The standalone `container` runtime does not resolve or support `network_volumes`.

The external storage setup must prepare the NAS export, derived directories, and permissions. Eligible Swarm workers need NFS support and connectivity. Jed does not create directories, adjust ownership, or provision/replace volumes.

**Deferred investigation:** existing Docker volumes may retain creation-time NFS settings. Changing `NFS_HOST` or `NFS_ROOT` is not a documented way to retarget an existing volume. Reuse, cross-service naming collisions, mismatch detection, and replacement policy remain to be investigated; this feature provides no mismatch protection or automatic recreation.

## Secrets

List swarm secret base names. The latest version, e.g. `aruba_client_secret_v3`, is resolved at deploy time and mounted using the base name as the secret filename.

```yaml
secrets:
  - aruba_client_secret
  - db_password
```

## Configs

Map swarm config base names to target mount paths. The latest version, e.g. `myapp_config_v2`, is resolved at deploy time and mounted at the specified path.

```yaml
configs:
  myapp_config: /etc/myapp/config.yaml
```

## User

`user` may be a UID or `UID:GID`. If omitted, swarm deploy uses `1001:1001`.

```yaml
user: "1000:967"
```

## Traefik

When `traefik` is present, routing labels are generated automatically for PathPrefix routing at `/{service name}` with TLS on the `websecure` entrypoint.

```yaml
traefik:
  port: "8080"
```

Generated labels include:

```yaml
labels:
  traefik.enable: "true"
  traefik.http.routers.{name}.rule: "PathPrefix(`/{name}`)"
  traefik.http.routers.{name}.entrypoints: "websecure"
  traefik.http.routers.{name}.tls: "true"
  traefik.http.routers.{name}.middlewares: "ingress-auth,{name}-strip"
  traefik.http.middlewares.{name}-strip.stripprefix.prefixes: "/{name}"
  traefik.http.services.{name}.loadbalancer.server.port: "{port}"
```

`ingress-auth` is a shared middleware declared by Traefik; jed only references it. By default, jed strips the matched path prefix after applying auth. To preserve it instead:

```yaml
traefik:
  port: "8080"
  preserve_path_prefix: true
```

When preserving the prefix, the router middleware label is `ingress-auth` and no strip middleware is generated.

To bypass the shared ingress authentication middleware:

```yaml
traefik:
  port: "8080"
  bypass_auth: true
```

This still generates the strip middleware unless `preserve_path_prefix` is also true.

Explicit labels in `labels:` take precedence over generated Traefik labels. Set `traefik.http.routers.{name}.middlewares` explicitly (including to an empty value) to supply custom middleware.
