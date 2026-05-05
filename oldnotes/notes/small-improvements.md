  ---
  Code Review: jed

  What's Working Well

  1. Clean separation of concerns - standalone containers (jed) vs swarm (swarm/), with shared Service/Env types
  2. Contract testing for Store - store/contract.go ensures implementations behave consistently
  3. Solid test coverage - Ginkgo specs with mocks, real test data files
  4. Consistent error handling - pkg/errors throughout with wrapping
  5. Good package documentation - jed.go header explains the mental model

  Suggestions for Improvement

  1. Hardcoded values in swarm/deploy.go

  "User":     "1001",
  "ReadOnly": true,
  These should probably be configurable per-service. Not every image runs as UID 1001, and some need write access.

  2. Fragile 404 detection (swarm/deploy.go:33)

  if !strings.Contains(verErr.Error(), "404") {
  The TODO says it - this could break if the error message format changes. Consider a typed error or status code check from the HTTP client.

  3. Duplicate envLines function

  - service.go:227 and swarm/deploy.go:211 have identical implementations. Could extract to a shared location.

  4. errors.Wrapf on nil (bbolt.go:115, 133, 149, 177)

  err = errors.Wrapf(err, "failed to list services")
  When err is nil, this returns nil, so it works. But it's slightly confusing - looks like it always wraps.

  5. Shallow copy warning in memo store

  Comments warn about shallow copies for GetService/Services, but SetService doesn't clone maps. Could lead to subtle bugs if caller mutates
  after storing.

  6. Config struct is empty

  type Config struct{}
  If it's always empty, consider removing the indirection and just having func New(...).

  7. No validation in swarm deploy

  jed.Deploy calls service.validate(), but swarm.Deploy doesn't. Inconsistent validation paths.

  8. Unused deployNamePattern in swarm context

  The pattern in jed.go:39 is for standalone containers. Swarm services use different naming. Clear enough, but worth noting if someone gets
  confused.

  9. Memory unit only supports "M"

  parseMem in deploy.go only handles M suffix. Consider supporting G for larger allocations, or document the limitation.

  Minor / Style

  - container.go:27 returns a string error message instead of an error - unusual pattern
  - Some files use // unexported comments, others don't - minor inconsistency
  - docker.go:9 has an unused empty variable (used, but could be inline)

  ---
