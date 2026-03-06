# store

The `store` package provides contract tests for `jed.Store` implementations. Implementations call `store.RunStoreContractTests(...)` in their test files to verify they meet the interface contract.

The interface itself lives in `jed.Store`. See `go doc jed.Store`.

## Packages

| Package       | Description                        |
|---------------|------------------------------------|
| `store`       | Contract tests for implementations |
| `store/bbolt` | Persistent BoltDB storage          |
| `store/memo`  | In-memory for testing              |

