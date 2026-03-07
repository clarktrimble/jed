# Swarm Test Data

Responses captured from Docker Swarm API via unix socket for use by `swarm` package tests.

## Docker API Documentation

API docs by version: https://docs.docker.com/reference/api/engine/version/v1.52/

Use web search to find specific endpoints/fields:
```
site:docs.docker.com engine API v1.52 ServiceInspect ServiceStatus
```

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
- `get-services-filtered.json` - filtered services list with ServiceStatus (used by GetService)
- `get-secrets.json` - list of secrets
- `get-configs.json` - list of configs (empty `[]` when none exist)
- `get-tasks-traefik.json` - tasks for a service
