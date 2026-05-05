# Renaming Progress

## Completed Phases

### ✅ Phase 1: Rename Svc → Jed
- Renamed `Svc` type to `Jed`
- Updated `NewSvc()` → `NewJed()`
- Updated all method receivers from `svc` to `jed`
- Updated field name `cntrs` → `svcs`
- Updated all files: jed.go, docker.go, jed_test.go, status_test.go
- **Result**: All tests passing, no diagnostics

### ✅ Phase 2: Rename Container → Service
- Renamed file: `container.go` → `service.go`
- Renamed type: `Container` → `Service`
- Updated `loadContainers()` → `loadServices()`
- Updated method receivers from `cntr` to `service`
- Updated `Jed.svcs` field type to `[]Service`
- Updated `Containers()` → `Services()` method
- Updated `Deploy()` and `Undeploy()` parameters to `*Service`
- Updated all test files to use `jed.Service`
- **Note**: Kept "containers.yaml" filename (file format unchanged)
- **Result**: All tests passing, no diagnostics

## Current Status: Ready for Phase 3

### Phase 3: Rename Status → Container (NEXT)
Need to:
1. Rename file: `status.go` → `container.go`
2. Rename type: `Status` → `Container`
3. Rename type: `Statii` → `Containers`
4. Update `newStatii()` → `newContainers()`
5. Update helper methods: `DeployName()`, `Id()` parameter names
6. Update `Statii()` method to return `Containers`
7. Update all references in jed.go, docker.go
8. Rename test file: `status_test.go` → `container_test.go`
9. Update all test code

### Remaining Phases
- Phase 4: Update Tests
- Phase 5: Update Documentation (README.md, TODO.md)

## Files Modified So Far
- ✅ jed.go
- ✅ docker.go
- ✅ container.go → service.go (renamed)
- ⏳ status.go (pending → container.go)
- ✅ jed_test.go
- ⏳ status_test.go (pending → container_test.go)
