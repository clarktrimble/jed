# store

The `store` package provides contract tests for `jed.Store` implementations. Implementations call `store.RunStoreContractTests(...)` in their test files to verify they meet the interface contract.

The interface itself lives in `jed.Store`. See `go doc jed.Store`.

Stores persist service definitions, envs, and intents. An intent records the selected image and desired replica count for an enabled logical service; a missing intent means disabled.

## Packages

| Package       | Description                        |
|---------------|------------------------------------|
| `store`       | Contract tests for implementations |
| `store/bbolt` | Persistent BoltDB storage          |
| `store/memo`  | In-memory for testing              |

