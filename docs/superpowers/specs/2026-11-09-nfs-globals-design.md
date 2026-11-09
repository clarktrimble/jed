# Separate NFS globals

Approved in conversation.

## Intent
Match upstream configuration by replacing combined `NETWORK_ROOT` with separate global `NFS_HOST` and `NFS_ROOT` values.

## Design
- Require both values only for services declaring `network_volumes`.
- `NFS_HOST` accepts a hostname or IP address, including bare IPv6. Preserve hostname validation and rejection of whitespace, delimiters, and port suffixes; reject bracketed hosts because brackets are no longer needed for parsing.
- `NFS_ROOT` is an absolute export path below `/`, with existing path safety rules, including rejection of dot segments and NUL.
- Use only global render vars; service env values do not override either setting.
- Remove `NETWORK_ROOT` support outright; no fallback or compatibility precedence.
- Keep `NETWORK_VOLUME_PREFIX`, derived names/directories, transient mounts, and Swarm NFS options unchanged.
- Update tests and current user docs. Historical design/plan documents remain historical.

## Approaches
Selected: clean replacement, because existing configuration needs no transition. Rejected: temporary dual-format support, which adds unnecessary precedence and migration logic.

## Non-goals
No Docker-volume replacement, retargeting guarantee, NAS provisioning, prefix redesign, or standalone-runtime changes. Existing-volume reuse remains unresolved.
