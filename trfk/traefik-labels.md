# Traefik Labels

Labels to route traffic through Traefik to your service.

## Example (whoami)

```yaml
labels:
  traefik.enable: "true"
  traefik.http.routers.whoami.rule: "PathPrefix(`/whoami`)"
  traefik.http.routers.whoami.entrypoints: "websecure"
  traefik.http.routers.whoami.tls: "true"
  traefik.http.routers.whoami.middlewares: "whoami-strip"
  traefik.http.middlewares.whoami-strip.stripprefix.prefixes: "/whoami"
  traefik.http.services.whoami.loadbalancer.server.port: "80"
```

## Labels Explained

| Label | Purpose |
|-------|---------|
| traefik.enable | Opt-in for routing (required) |
| traefik.http.routers.whoami.rule | Match requests starting with /whoami |
| traefik.http.routers.whoami.entrypoints | Listen on port 443 (websecure) or 80 (web) |
| traefik.http.routers.whoami.tls | Terminate TLS at traefik |
| traefik.http.routers.whoami.middlewares | Apply middleware before forwarding |
| traefik.http.middlewares.whoami-strip.stripprefix.prefixes | Remove prefix from path |
| traefik.http.services.whoami.loadbalancer.server.port | Container port to forward to |

The `whoami` in the label names is an identifier - usually your service name.
