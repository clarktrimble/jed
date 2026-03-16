# Event Stream for Deploy Feedback

## Problem

When user saves config, montage deploys to swarm and waits with a 2.5s sleep hack. No feedback during deploy, no confirmation of success/failure.

## Solution

Expose Docker's event stream through jed so montage can provide real-time deploy feedback via SSE.

## Docker Events API

```bash
curl --unix-socket /var/run/docker.sock \
  --get \
  --data-urlencode 'filters={"type":["service"],"service":["<name>"]}' \
  "http://localhost/events"
```

Returns newline-delimited JSON:

```json
{"Type":"service","Action":"update","Actor":{"Attributes":{"name":"tag","updatestate.new":"updating"}}}
{"Type":"service","Action":"update","Actor":{"Attributes":{"name":"tag","updatestate.new":"completed","updatestate.old":"updating"}}}
```

## Proposed jed/swarm API

```go
// Events streams Docker events for a service, filtered by name.
// Caller should range over channel until context cancelled.
func (s *Swarm) Events(ctx context.Context, serviceName string) (<-chan Event, error)

type Event struct {
    Type        string          // "service", "container"
    Action      string          // "update", "create", "start", "destroy"
    Service     string          // service name
    TaskID      string          // if container event
    UpdateState string          // "updating", "completed", "rollback_started", etc.
    Time        time.Time
    Raw         json.RawMessage // original event if needed
}
```

## Key Events for UI

| Docker Event | Meaning | UI Action |
|--------------|---------|-----------|
| `updatestate.new: "updating"` | Rollout started | Show "Deploying..." |
| `container start` | New task running | Optional: "Container started" |
| `updatestate.new: "completed"` | Rollout done | Success toast, refresh |
| `updatestate.new: "rollback_started"` | Deploy failed | Error toast |
| `updatestate.new: "paused"` | Deploy paused | Warning state |

## Montage Integration

1. POST `/services/{name}/config` saves config, kicks off deploy, returns immediately with "deploying" state
2. Frontend opens SSE: `GET /services/{name}/events`
3. Montage streams jed events to browser
4. On "completed" → close stream, show success, refresh detail
5. On error → close stream, show error

## Event Types

### Service Events (`"Type":"service"`)

Swarm-level, about the service as a whole:

```json
{
  "Type": "service",
  "Action": "update",
  "Actor": {
    "ID": "ypd2azr6t0d2qhhcrgr2jahqm",
    "Attributes": {
      "name": "tag",
      "updatestate.new": "completed",
      "updatestate.old": "updating"
    }
  },
  "scope": "swarm",
  "time": 1773324491
}
```

- `Action: "update"` with `updatestate.new/old` tells you rollout progress
- Scope: `"swarm"` - cluster-wide concept
- Key for knowing when deploy is done/failed

### Container Events (`"Type":"container"`)

Node-level, about individual task containers:

```json
{
  "Type": "container",
  "Action": "start",
  "Actor": {
    "ID": "503107e596fb...",
    "Attributes": {
      "com.docker.swarm.service.id": "ypd2azr6t0d2qhhcrgr2jahqm",
      "com.docker.swarm.service.name": "tag",
      "com.docker.swarm.task.id": "dtgfx84lbp4ivawzew8jsf5w3",
      "com.docker.swarm.task.name": "tag.1.dtgfx84lbp4ivawzew8jsf5w3",
      "com.docker.swarm.node.id": "ludb8um9kkarh19ja6g7kvwfq",
      "image": "local/tag:c197117"
    }
  },
  "scope": "local",
  "time": 1773324486
}
```

- Actions: `create`, `start`, `stop`, `destroy`, `die`, etc.
- Scope: `"local"` - happens on a specific node
- Swarm labels correlate container to service/task

### Correlating Container Events to Service

Docker labels every swarm container with:

| Label | Example | Purpose |
|-------|---------|---------|
| `com.docker.swarm.service.name` | `"tag"` | Service it belongs to |
| `com.docker.swarm.service.id` | `"ypd2azr6t0d2..."` | Service ID |
| `com.docker.swarm.task.id` | `"dtgfx84lbp4i..."` | Specific task |
| `com.docker.swarm.task.name` | `"tag.1.dtgfx84..."` | Human-readable (service.slot.taskid) |
| `com.docker.swarm.node.id` | `"ludb8um9kkar..."` | Which node it's on |

Filter container events client-side by matching `com.docker.swarm.service.name`.

### Which Events for What

| Need | Use |
|------|-----|
| "Is deploy done?" | Service events (`updatestate.new`) |
| "What's happening right now?" | Container events (`create`, `start`, `destroy`) |
| "Which task failed?" | Container `die` event + task ID |

**Recommendation:** Start with service events only for "completed/failed" signal. Add container events later for granular "spinning up container..." feedback.

## Nice to Have

- `since` parameter to replay recent events (Docker supports this)
- Filter by event type (service only vs service+container)
- Timeout/keepalive for long-running connections

# What we did in jed/swarm

  giant - new StreamLines(ctx, path) (<-chan []byte, error)
  - Channel-based, no cleanup needed
  - Context cancellation closes channel

  swarm/swarm.go
  - Added Logger interface (Info, Debug, Error with context)
  - New(client, logger) now takes logger

  swarm/client.go - Event streaming
  type Event struct {
      Type    string          // "service", "container"
      Service string          // for filtering
      Time    time.Time       // timestamp
      Payload json.RawMessage // ServiceEvent or ContainerEvent
  }

  type ServiceEvent struct {
      Action      string
      UpdateState string  // "updating", "completed", "paused"
  }

  type ContainerEvent struct {
      Action   string  // "create", "start", "die", "destroy"
      TaskID   string  // links to logs
      TaskName string
      Image    string
      ExitCode string
      ExecDur  string
  }

  func (d *Swarm) Events(ctx context.Context) (<-chan Event, error)

  - swarmEvent.ToEvent() helper for clean conversion
  - Filters to service/container events only
  - Proper error wrapping and logging

  transship - events command for testing

  Design decisions:
  - Channel over iter/callback - fits montage's service architecture
  - No service filter in swarm - montage buffers all, filters client-side
  - Type + Payload pattern - browser gets clean JSON, no Docker internals
  - No duplication - Service/Time in Event only, not in payloads

  Ready for montage to consume with ring buffer and SSE.

## and Status(es) makeover
  - Status type with constants (StatusRunning, StatusStopped, StatusPending, StatusError)
  - Status() returns (Status, string, error) - now includes message
  - Statuses() - batch status for all services in one API call
  - computeStatus() - shared logic
  - "pending" state covers: deploying, stopping, fresh service starting
