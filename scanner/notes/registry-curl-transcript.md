# Registry Label Scan Notes

Companion notes for `registry-curl-transcript.txt`.

## What the transcript showed

The registry API flow for finding image labels is straightforward:

1. List repositories:
   - `GET /v2/_catalog`
2. List tags for a repository:
   - `GET /v2/<repo>/tags/list`
3. Fetch the tag manifest:
   - `GET /v2/<repo>/manifests/<tag>`
4. If the tag points at an index / manifest list, choose the runnable platform manifest, e.g. `linux/amd64`, and fetch that manifest by digest.
5. Read `.config.digest` from the image manifest.
6. Fetch the config blob:
   - `GET /v2/<repo>/blobs/<config-digest>`
7. Labels are at:
   - `.config.Labels`

The scanner needs to handle both forms seen in the transcript:

- OCI index / Docker manifest list
- Single-platform image manifest directly

When reading an index, ignore attestation manifests with `unknown/unknown` platform unless we later decide we need them.

## Go approach

Use Go's registry HTTP API directly rather than shelling out to `curl`/`jq`.

Keep it simple:

- one shared `http.Client`
- bounded worker pool if scanning many images
- parse only the manifest/config fields we need
- cache digest-addressed config blobs

Rough flow:

```text
for each repo:tag:
    fetch manifest for repo:tag
    if manifest is index/list:
        select linux/amd64 manifest digest
        fetch selected manifest by digest
    read config digest
    get labels from config cache, or fetch blob and cache it
    emit repo:tag + labels if labels are present
```

## Concurrency

We probably do not need anything fancy.

A small bounded pool is enough if scans cover a few hundred images:

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(16)

for _, ref := range refs {
    ref := ref
    g.Go(func() error {
        return scanImage(ctx, client, ref)
    })
}

return g.Wait()
```

Start with a limit like `8` or `16`; increase only if measurement says it matters.

## Caching

Cache immutable things by digest.

Most useful cache:

```text
config digest -> labels
```

Config blobs are content-addressed and effectively immutable, so this is safe and simple.

Tag lookups are different:

```text
repo:tag -> manifest digest
```

Tags can move, so either do not cache them or use a short TTL/revalidation. The main win should come from config digest caching, especially across repeated scans.

## Output

For now, newline-delimited JSON is probably enough:

```json
{"image":"tag-phosphorous-s3:be4a6d3","labels":{"org.bastille.bip.service.env.path":"/service.env","org.bastille.bip.service.spec.path":"/service.yml"}}
```

No need to build a large abstraction until we know we need it.
