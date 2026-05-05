# TODO

## High Priority

- [ ] Test missing image error path in New (coverage: New at 88.9%)
- [ ] Test Deploy error paths - create/start failures (coverage: Deploy at 75.0%)
- [ ] Test Logs error paths - SendJson/ReadAll failures (coverage: Logs at 66.7%)

## Medium Priority

- [x] Make suffixPattern more specific: `^/(.+)-[a-zA-Z0-9]{7}$` with leading slash
- [ ] Add hondo package var for testable random source (predictable suffixes in tests)
- [ ] Allow choosing stdout and/or stderr in Logs method
- [ ] Consider adding more Containers helper methods:
  - `State(serviceName) string` - get container state
  - `Exists(serviceName) bool` - check if container exists

## Low Priority

- [x] Rename Status struct? (Docker calls this a "container")
- [ ] Demajic the .env file path construction in loadEnv
- [ ] Expose streaming logs - return io.ReadCloser instead of []byte
- [ ] Add Logs options: timestamps, since, until, follow

## Ideas / Future Considerations

- [ ] Image pulling if missing (vs current fail-fast approach)
- [ ] Batch operations (deploy/undeploy multiple containers)
- [ ] Container health checking
- [ ] Volume management helpers
- [ ] Network management helpers
- [ ] Support for docker-compose.yml format
- [ ] Container restart/pause/unpause operations
- [ ] Resource limits (CPU, memory)
- [ ] Event streaming for container state changes

## Code Quality

- [x] Remove redundant validate calls
- [x] Fix linter warnings
- [x] Add findCall helper for tests
- [x] Add testContainerList helper to reduce test duplication
- [x] Move managed_by label addition to loadServices

## Documentation

- [x] Create README with API documentation
- [x] Document architecture and design decisions
- [x] Add godoc comments to all exported types/functions
- [ ] Add usage examples in godoc
- [x] Document containers.yaml schema
