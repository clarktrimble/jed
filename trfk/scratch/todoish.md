
## trfk
1. HTTPS/TLS - you commented it out, but eventually you'll want certs. Let's Encrypt via traefik's ACME, or manual certs mounted.
2. Labels for service discovery - other services need traefik labels to be routed. The test data has examples (traefik.http.routers...).
3. Access logs - enabled in env, but going where? Probably stdout, which is fine for transship logs.
4. Health check - swarm can restart traefik if unhealthy. Not configured yet.
5. Replicas - currently hardcoded to 1 in deploy.go:129. For HA you'd want more, but with publish_mode: host that gets tricky (port
  conflicts).
6. The docker GID - you hardcoded 967 which is your system's docker group. Won't be portable to other machines.

## gen
1. Hardcoded values in swarm/deploy.go - User: "1001", ReadOnly: true - fixed (User now configurable)
2. Fragile 404 detection (swarm/deploy.go:33) - strings.Contains(verErr.Error(), "404") - still there
3. Duplicate envLines function - in service.go:227 and swarm/deploy.go:211 - still duplicated
4. errors.Wrapf on nil in bbolt.go - wraps even when err is nil - still there
5. Shallow copy warning in memo store - SetService doesn't clone maps - still there
6. Config struct is empty - type Config struct{} - still there
7. No validation in swarm deploy - jed.Deploy validates, swarm.Deploy doesn't - still there
8. Memory unit only supports "M" - parseMem doesn't handle G - still there
