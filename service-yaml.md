# Service YAML

Define a service for deployment to Docker Swarm.

## Example

```yaml
name: reauth-acp
image: local/reauth-acp:3c070f2
network: svc-net
restart: on-failure

ports:
  3031/tcp: "8012"

volumes:
  svc-data: /data

secrets:
  - aruba_client_secret

hosts:
  - "10.35.44.41 container4"

resources:
  cpu_limit: "0.5"
  mem_limit: "128M"
  cpu_reserve: "0.1"
  mem_reserve: "64M"
```
Todo: looks like we have "string" in the above where float/int would be better?

## Fields

| Field | Required | Description |
|-------|----------|-------------|
| name | yes | Service name |
| image | yes | Docker image with tag |
| network | yes | Docker network name |
| restart | yes | Restart policy (on-failure, always, unless-stopped) |
| ports | no | Container port to host port mapping |
| volumes | no | Named volume or host path to container path |
| secrets | no | List of swarm secret base names (latest version resolved automatically) |
| hosts | no | Extra /etc/hosts entries |
| command | no | Override container command |
| labels | no | Container labels |
| resources | no | CPU and memory limits/reservations |
| publish_mode | no | Port publish mode: "host" for direct binding, default is ingress |
| user | no | Container user (e.g., "1001", "1000:967"). Default is "1001" |
| traefik | no | Traefik routing config (generates labels automatically) |

## Resources

CPU values are decimals (e.g., "0.5" for half a CPU, "2.0" for two CPUs).

Memory values require M suffix (e.g., "128M" for 128 megabytes).

| Field | Default | Description |
|-------|---------|-------------|
| cpu_limit | 0.5 | Maximum CPU |
| mem_limit | 128M | Maximum memory |
| cpu_reserve | 0.1 | Reserved CPU |
| mem_reserve | 64M | Reserved memory |

## Ports

Format: `"container_port/protocol": "host_port"`

```yaml
ports:
  3031/tcp: "8012"
  53/udp: "5353"
```

## Volumes

Named volumes use a simple name. Bind mounts use an absolute path.

```yaml
volumes:
  svc-data: /data          # named volume
  /host/path: /container   # bind mount
```

## Secrets

List secret base names. The latest version (e.g., `aruba_client_secret_v3`) is resolved at deploy time and mounted at `/run/secrets/<base_name>`.

```yaml
secrets:
  - aruba_client_secret
  - db_password
```

## Traefik

When `traefik` is present, routing labels are generated automatically for PathPrefix routing at `/{service name}` with TLS on the websecure entrypoint, plus stripprefix middleware.

```yaml
traefik:
  port: "8080"
```

Generates these labels:

```yaml
labels:
  traefik.enable: "true"
  traefik.http.routers.{name}.rule: "PathPrefix(`/{name}`)"
  traefik.http.routers.{name}.entrypoints: "websecure"
  traefik.http.routers.{name}.tls: "true"
  traefik.http.routers.{name}.middlewares: "{name}-strip"
  traefik.http.middlewares.{name}-strip.stripprefix.prefixes: "/{name}"
  traefik.http.services.{name}.loadbalancer.server.port: "{port}"
```

Explicit labels in `labels:` take precedence over generated traefik labels.
