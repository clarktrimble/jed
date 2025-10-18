# TODO

## High Priority

- [ ] Test missing image error path in NewSvc (coverage: NewSvc at 88.9%)
- [ ] Test Deploy error paths - create/start failures (coverage: Deploy at 75.0%)
- [ ] Test Logs error paths - SendJson/ReadAll failures (coverage: Logs at 66.7%)

## Medium Priority

- [x] Make suffixPattern more specific: `-[a-zA-Z0-9]{7}$` instead of `-[^-]+$` - COMPLETED
- [ ] Add hondo package var for testable random source (predictable suffixes in tests)
- [ ] Allow choosing stdout and/or stderr in Logs method
- [ ] Consider adding more Containers helper methods:
  - `State(serviceName) string` - get container state
  - `Exists(serviceName) bool` - check if container exists

## Low Priority

- [x] Rename Status struct? (Docker calls this a "container") - COMPLETED
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

- [x] Remove redundant validate calls (completed)
- [x] Fix linter warnings (completed)
- [x] Add findCall helper for tests (completed)
- [x] Add testContainerList helper to reduce test duplication (completed)
- [x] Move managed_by label addition to loadServices (completed)

## Documentation

- [x] Create README with API documentation (completed)
- [x] Document architecture and design decisions (completed)
- [ ] Add godoc comments to all exported types/functions
- [ ] Add usage examples in godoc
- [ ] Document container.yaml schema
