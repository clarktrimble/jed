# Upstream Guide: Services, Intents, and Specs

This guide summarizes how upstream callers should interact with Jed after the service/intent split.

## Concepts

- `jed.Service` is a runnable service definition for one image.
  - Keyed by logical service name and image.
  - Multiple service definitions may share the same name when they represent different images.
- `jed.Intent` is desired state for a logical service.
  - Records the selected image and desired replica count.
  - Missing intent means the service is disabled.
- `jed.Spec` is the rendered runtime input.
  - Built from the current `Intent`, selected `Service`, service env, and render vars.

## Common upstream flow

### Add or update an image definition

Store a `jed.Service` for each image the user may choose:

```go
svc := jed.Service{
    Name:    "postgres",
    Image:   "postgres:16",
    Network: "svc-net",
}

err := store.SetService(ctx, svc)
```

### Enable a service image

Enable selects which image is active for a logical service. It creates or updates the service intent with zero replicas.

```go
err := j.Enable(ctx, "postgres", "postgres:16")
```

If another image is already selected and has replicas, `Enable` fails. Scale to zero before switching images.

### Start or stop a service

Scale updates the current intent replica count:

```go
err := j.Scale(ctx, "postgres", 1) // start
err := j.Scale(ctx, "postgres", 0) // stop
```

Scaling requires an intent. If there is no intent, the service is disabled.

### Disable a service

Disable removes the intent:

```go
err := j.Disable(ctx, "postgres")
```

Disable is idempotent when no intent exists. If the intent has replicas, disable fails; scale to zero first.

### Render for deploy

```go
spec, err := j.Spec(ctx, "postgres")
```

`Spec` loads the current intent, selected service image, service env, and render vars. Missing intent means disabled and returns an `intent not found` error.

## User-facing state model

| Intent exists | Replicas | Meaning |
|---------------|----------|---------|
| no            | n/a      | disabled |
| yes           | 0        | enabled, stopped |
| yes           | >0       | enabled, desired running |

## Image switching

Recommended flow:

```go
err := j.Scale(ctx, "postgres", 0)
err = j.Enable(ctx, "postgres", "postgres:17")
err = j.Scale(ctx, "postgres", 1)
```

This keeps version/image switches explicit and avoids changing a running service underneath the user.

## Listing choices

Use store primitives for definitions and intents:

```go
services, err := store.Services(ctx, "postgres") // image choices for one logical service
all, err := store.AllServices(ctx)                // every stored image definition
intent, err := store.GetIntent(ctx, "postgres")   // selected image and scale
```

For UI grouping:

```go
byName := jed.Services(all).ByName()
```

## Notes

- Store methods persist data; `Jed` methods enforce service/intent behavior.
- Environment variables remain keyed by logical service name.
- The HTTP API does not yet expose intent routes; upstream callers should use Go methods directly.
