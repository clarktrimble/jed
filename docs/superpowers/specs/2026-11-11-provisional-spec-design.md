# ProvisionalSpec

Approved in conversation; user requested implementation inline.

Provide two explicit APIs: Spec(ctx, name) renders stored state; ProvisionalSpec(ctx, intent, env) renders supplied state. Remove SpecWithEnv rather than retain convenience wrappers.

Spec loads the stored service env and intent, preserving existing missing-env/intent errors. Both APIs call spec(ctx, intent, env). ProvisionalSpec validates the supplied intent and checks intent.Name == env.Name before calling the shared renderer. The renderer looks up the service using intent.Name and intent.Image, then applies existing defaults, service validation, rendering, and volume resolution. Spec retains its previous stored-state behavior without revalidating intent or matching embedded names. The result retains the supplied intent including replica count.

ProvisionalSpec reads service definitions and global render vars only; it neither loads nor saves the service env or intent. Supplied env replaces, rather than merges with, stored env. Existing render clone/isolation behavior remains. This is a render preview, not an Enable authorization/guard check or a guarantee of a later write.

## Approaches
Selected: stored-state versus supplied-state API pair. Rejected: SpecWithIntent alongside SpecWithEnv, or retaining SpecWithEnv as a convenience wrapper; explicit inputs avoid a growing method family.

No store schema, HTTP endpoint, runtime, or enabling behavior changes. Update callers/tests/current docs; historical docs remain unchanged. Leave changes uncommitted.
