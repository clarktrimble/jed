# Short network volume/share names

Approved in conversation: readable service/last-directory names, always hashed, with path normalization before hashing.

## Contract
- Validate the container path first using existing rules (reject dot/parent segments), then normalize with path.Clean.
- Stem: existing flattened service name + `_` + path.Base(normalized container path). Preserve the existing ASCII volume-name character restrictions; validate the complete stem before shortening so invalid characters are not hidden by truncation.
- Identity: original full service name, one NUL byte, normalized full container path (UTF-8 bytes).
- Suffix: low 24 bits of FNV-1a 64-bit(identity), formatted as exactly six lowercase hex characters (zero-padded). Use Go's standard-library hash/fnv.
- Final name: first at most 25 ASCII characters of the stem + `_` + suffix; maximum 32 characters, no padding. Always hash, including short names.
- Use the same final name for the Docker volume and NAS share. Export path: path.Join(NFS_ROOT, name). The hash algorithm and encoding are part of the persistent storage naming contract.
- Remove NETWORK_VOLUME_PREFIX; retain global NFS_HOST and NFS_ROOT and existing NFS options.
- Preserve normalized-target deduplication and explicit/local conflict behavior. Detect final-name collisions between distinct targets within one rendered service; global identity/ownership checks belong to the upstream NAS provisioner.

## Approaches
Selected: always hash, with a six-hex suffix, to distinguish identical basenames even without truncation. Rejected: reject all overlong stems (32 is too restrictive), truncation alone (collisions), and hashing only truncated names (short-name ambiguity). Eight hex characters were considered; six preserves more readability at the agreed deployment scale.

## Non-goals and compatibility
No upstream provisioner implementation, share creation, automatic migration, or volume replacement. Existing NAS paths and Docker names change; existing storage must be migrated explicitly before redeploying affected workloads. No local-volume naming changes or render-vars configuration refactor. Keep unrelated user edits intact.
