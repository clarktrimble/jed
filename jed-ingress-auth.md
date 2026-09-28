# Request to jed: reference the ingress auth middleware

Montage now guards itself and the Traefik dashboard with a basicauth middleware
named `ingress-auth`. It is declared once, in a label on the Traefik service,
and any router that names it gets the same password prompt. We would like every
workload routed through the `traefik:` shorthand to name it too.

## What we want

When a service has a `traefik:` block, put `ingress-auth` at the front of its
router's `middlewares` label.

Without strip:

```yaml
traefik:
  port: "3031"
```

```
traefik.http.routers.example.middlewares=ingress-auth
```

With strip:

```yaml
traefik:
  port: "3031"
  path_prefix_strip: true
```

```
traefik.http.routers.example.middlewares=ingress-auth,example-strip
```

The strip middleware's own declaration label is unchanged. Nothing declares
`ingress-auth`; it is a reference to a middleware Traefik already has.

## What we have checked

We put the label on a running workload by hand, both directly on the swarm
service and through an explicit `labels:` entry in its stored definition. In
both cases Traefik picked it up, unauthenticated requests went from 200 to 401,
and the strip still applied after auth. So the label above is known to work;
this request is only about jed generating it.

## Open to your judgement

Whether the name `ingress-auth` is hardcoded or comes from `jed.Config` is
yours to decide. If it is config, Montage will pass the name when it constructs
jed, and an empty value can mean no auth.

## Think about

Opt out.

# and another thing

deleteService from montage

```
// DeleteService removes a stored definition unless intent points at it.
func (op *Operator) DeleteService(ctx context.Context, name, image string) (err error) {

	// Todo: push this to jed

	intent, err := op.jd.Store().GetIntent(ctx, name)
	if err == nil && intent.Image == image {
		return errors.New("enabled service definition cannot be deleted")
	}

	var notFound jed.NotFoundError
	if err != nil && !errors.As(err, &notFound) {
		return
	}

	return op.jd.Store().DelService(ctx, name, image)
}
```
