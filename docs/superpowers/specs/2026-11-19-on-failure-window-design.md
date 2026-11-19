# On-failure retry window

Status: approved.

## Intent

Allow recovery from isolated failures without permitting a rapid crash loop.
The current lifetime retry count can strand a service after two failures weeks apart.

## Behavior

- Keep `on-failure` at one retry with the configured delay.
- Apply `swarm.Config.RestartWindow` to `on-failure` as well as `any`.
- The default window remains two minutes.
- Preserve explicit zero: omit the window and count all retries.
- Leave `RestartMaxAttempts` exclusive to `any`.
- Leave `any` and `none` unchanged.
- Exhaustion still requires intervention. Window expiry does not restart a stopped service.
- The change takes effect on the next deployment.

## Approaches

- Keep the lifetime limit: compatible but retains the surprising failure mode.
- Share the five-attempt budget: consistent but changes more than needed.
- Add the shared window while keeping one retry: selected as the focused fix.

## Scope

Update policy projection and its tests. Correct configuration help and current
restart documentation. Resolve the stale investigation note. Keep historical
specs unchanged.

No service model change. No store migration. No new settings. No scheduler changes.

## Verification

Cover both constructors and environment configuration. Check default and custom
windows in the Go payload and serialized JSON. Include explicit zero and confirm
that custom `RestartMaxAttempts` cannot increase the `on-failure` retry count.
Retain regression coverage for `any` and `none`.

Tests verify Docker policy projection rather than simulate Swarm scheduling.

## Documentation

Keep prose tight. Avoid unnecessary comma-separated phrasing and repetition.
