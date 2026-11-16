# Remove standalone deployment

Approved in conversation, including the decoder filename `swarm/dockerlog.go`.

## Intent
Remove the deprecated standalone Docker deployment API and keep Docker log decoding with its remaining consumer, Swarm. This is source cleanup, not an operation against Docker or persisted state.

## Approaches
Selected: delete `container/` outright and move the decoder into Swarm as an unexported implementation detail. A compatibility stub would retain the unwanted deployment API. A top-level `dockerlog` package would expose a reusable abstraction with only one remaining consumer. Neither is needed.

## Scope
- Delete the six tracked files in `container/`, including runtime tests and generated mocks.
- Move `internal/dockerlog/decoder.go` to `swarm/dockerlog.go`; change its package to `swarm` and rename `Decode(src io.Reader) io.Reader` to unexported `decodeDockerLogs(src io.Reader) io.Reader` (implementation naming is the builder's call).
- Update `swarm/task.go` to use the local helper. Preserve existing decoding behavior, framing errors, and the 65536-byte frame limit.
- Preserve the real-log fixture regression from the deleted container tests in public `Swarm.TaskLogs` coverage, alongside focused framing/error regressions.
- Update `README.md` and `design.md` to describe Swarm as the sole supported deployment runtime. Keep the root service model runtime-neutral.
- Mark the standalone-removal TODO complete and update the decoder coverage TODO so it does not point at a deleted package. Preserve the user's existing server-validation checkbox change.
- Leave historical plans, specs, and old notes unchanged.

## Boundaries
- Preserve Swarm deployment and CLI behavior, image extraction (including its temporary stopped containers), shared service types, and dependencies still used by those packages.
- No running-container cleanup, Docker calls, migrations, compatibility shim, decoder behavior redesign, or unrelated documentation rewrite.
- External consumers importing `github.com/clarktrimble/jed/container` will need to migrate; no in-repo production caller exists.

## Verification
Exercise decoding through public `Swarm.TaskLogs`: the existing example, real-log fixture, empty stream, multiple frames including payloads larger than the reader's initial buffer, truncated headers/payloads, oversized frames, and transport errors. Check active source references and package discovery after deletion. Run formatting, Go diagnostics, full tests, race tests, build, `go vet`, and configured `staticcheck`.
