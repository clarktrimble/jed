# Swarm Test Data

Responses captured from Docker Swarm API via unix socket for use by `swarm` package tests.

## Capturing data

```bash
curl -s --unix-socket /var/run/docker.sock http://localhost/v1.52/services | jq .
curl -s --unix-socket /var/run/docker.sock http://localhost/v1.52/services/tag | jq .
curl -s --unix-socket /var/run/docker.sock http://localhost/v1.52/secrets | jq .
curl -s --unix-socket /var/run/docker.sock http://localhost/v1.52/configs | jq .

# Note: filters must be URL-encoded
curl -s --unix-socket /var/run/docker.sock 'http://localhost/v1.52/tasks?filters=%7B%22service%22%3A%5B%22traefik%22%5D%7D' | jq .
```

## Files

- `get-services.json` - list of all services
- `get-service-tag.json` - single service detail
- `get-secrets.json` - list of secrets
- `get-configs.json` - list of configs (empty `[]` when none exist)
- `get-tasks-traefik.json` - tasks for a service
