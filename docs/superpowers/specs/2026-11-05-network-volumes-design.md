# NetworkVolumes: convention-based NFSv3 mounts

Status: approved. Global render vars and initial Swarm-only scope confirmed.

## Intent

Give services a container-path-only declaration for NFSv3-backed storage, analogous to LocalVolumes. Jed derives the mount configuration; Docker mounts it. Export preparation belongs to another service.

## Declaration and configuration

```yaml
name: postgres
network_volumes:
  - /var/lib/postgresql/data
```

Add `Service.NetworkVolumes []string` with JSON field `network_volumes,omitempty`. Existing `volumes` and `local_volumes` formats stay unchanged.

Use two global render vars, alongside LOCAL_ROOT:

```text
NETWORK_ROOT=nas:/exports/jed
NETWORK_VOLUME_PREFIX=jed
```

NETWORK_ROOT combines the NFS server and export root in one value. NETWORK_VOLUME_PREFIX defaults to `jed` when unset.

## Convention

Reuse the local-volume service-name/path flattening rules. For the example above:

```text
Docker volume: jed-postgres-var_lib_postgresql_data
NAS directory: /exports/jed/postgres/_var_lib_postgresql_data
Container:     /var/lib/postgresql/data
```

The Docker name omits the leading underscore used in the NAS directory's final component. Derive all names after template expansion. Apply the existing local-path safety rules to network targets and the export root; validate the server and generated Docker volume name too. Only services declaring NetworkVolumes require NETWORK_ROOT.

## Generated output, not stored records

Generate the equivalent of this entry under Docker's TaskTemplate.ContainerSpec.Mounts:

```json
{
  "Type": "volume",
  "Source": "jed-postgres-var_lib_postgresql_data",
  "Target": "/var/lib/postgresql/data",
  "VolumeOptions": {
    "DriverConfig": {
      "Name": "local",
      "Options": {
        "type": "nfs",
        "o": "addr=nas,nfsvers=3,rw",
        "device": ":/exports/jed/postgres/_var_lib_postgresql_data"
      }
    }
  }
}
```

Driver and NFSv3 read/write options are fixed by the convention. The generated fields exist only in the in-memory rendering/output pipeline; do not persist resolved mounts or introduce a volume inventory. Choose the exact transient Go representation during implementation planning, preserving the runtime-neutral root package and existing explicit/local mount behavior.

## Conflicts

- Explicit Volumes wins over a conventional mount at the same container target; log the conflict, matching current local-volume behavior.
- LocalVolumes and NetworkVolumes declaring the same container target is an error.

## Approaches

- Pre-mounted NFS host paths could reuse bind mounts directly, but require node-side mount setup. The chosen direction is Docker-managed NFS mounts with convention-generated driver settings.
- Replacing the persisted Volumes map with structured entries would change existing configuration/API. Instead, retain existing declarations and generate richer, transient output for NetworkVolumes.

## Scope and non-goals

Initial scope is Docker Swarm. Do not add NAS provisioning, host mount preparation, remote directory creation, ownership changes, storage migration, or a persistent resolved-volume model. The external storage setup must provide usable remote directories and permissions; eligible workers need NFS support and connectivity.

Do not change standalone container deployment as part of this feature. Document that NetworkVolumes support is in the Swarm spec pipeline, as LocalVolumes resolution already depends on the Jed.Spec pipeline.

## Deferred investigation

Investigate Docker's behavior when a derived named volume already exists, particularly after NETWORK_ROOT changes. This design does not yet choose mismatch detection, recreation, or an operator-managed policy. Do not silently treat that question as resolved or add volume replacement behavior without a follow-up decision.

## Verification goals

Cover configuration/defaults, naming and path derivation, template expansion, invalid paths/configuration, conflicts, unchanged stored declarations, and the outgoing NFSv3 mount payload. Existing explicit/local mount tests must continue to pass.
