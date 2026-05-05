# Web UI Planning Session

## Vision
A web UI for managing a small pack of services on a single node using htmx/templ stack.

## Current State (What We Accomplished Today)
- ✅ Cleaned up `Containers()` method - added leading slash to regex pattern
- ✅ Renamed `NewJed` → `New` (more idiomatic Go)
- ✅ Changed `Services()` to return `map[string]Service` (was `[]Service`)
- ✅ Implemented deep copy in `Services()` using `maps.Clone()` for all map fields
- ✅ Cleaned up test naming - all `Describe` blocks match methods
- ✅ Renamed test directories: `cntr-cfg` → `svc-cfg`
- ✅ Updated README and TODO to reflect current state
- ✅ All 32 tests passing, 86.9% coverage

## Web UI Design

### Core Views
1. **Services Dashboard** - Shows all defined services with status
   - Card/table view of services from `Services()`
   - Each shows: name, image, state (running/stopped/missing)
   - Quick actions: Deploy, Undeploy, View Logs

2. **Service Detail** - Drill into specific service
   - Container info from `Containers()`
   - Full config (ports, volumes, labels, env)
   - Live logs with streaming
   - Deploy/Undeploy buttons
   - **Edit Environment Variables**

3. **Logs Viewer** - Tail logs from running containers
   - Real-time streaming (needs streaming logs feature)
   - Filter by service
   - Tail size control

### API Endpoints Needed
```
GET  /api/services                    # List all services
GET  /api/containers                  # Get running containers
POST /api/services/:name/deploy       # Deploy a service
POST /api/services/:name/undeploy     # Undeploy
GET  /api/services/:name/logs         # Get logs (maybe SSE for streaming)
GET  /api/services/:name/env          # Get env vars as map
POST /api/services/:name/env          # Add new env var
POST /api/services/:name/env/:key     # Update env var
DELETE /api/services/:name/env/:key   # Delete env var
```

## Extra YAML Fields Issue

Current services.yaml has fields jed doesn't use:
- `auto_deploy` - Deploy on startup?
- `env_form` - Form template name?
- `features` - UI feature flags like [settings, metrics, edit_config, api_docs]

### Options Discussed:

**Option 1: Passthrough Field** (Recommended for flexibility)
```go
type Service struct {
    Name, Image, Network, Restart string
    Env, Ports, Labels, Volumes   map[string]string
    Extra                         map[string]any  // passthrough
}
```

**Option 2: Extend Service Struct** (Recommended for simplicity)
```go
type Service struct {
    Name, Image, Network, Restart string
    Env, Ports, Labels, Volumes   map[string]string

    // UI-specific fields (ignored by jed core)
    AutoDeploy bool              `json:"auto_deploy,omitempty"`
    EnvForm    string            `json:"env_form,omitempty"`
    Features   []string          `json:"features,omitempty"`
}
```

**Decision:** Lean toward Option 2 (extend struct) since building one specific UI app.

## Environment Variable Editing (Key Feature!)

### The Big Simplifier
Ability to edit `.env` files via web UI with guardrails.

### UX Approach: Key-Value Form (Not Raw Textarea)
Reasons:
- User-friendly with guardrails
- Hard to make syntax errors
- Can add inline validation
- Easy to add/delete variables

### UI Flow with htmx:
```html
<div id="env-editor">
  <!-- Each env var -->
  <div class="env-var" id="env-TRAEFIK_API_DASHBOARD">
    <input type="text" name="key" value="TRAEFIK_API_DASHBOARD" readonly>
    <input type="text" name="value" value="true"
           hx-post="/api/services/traefik/env/TRAEFIK_API_DASHBOARD"
           hx-trigger="blur">
    <button hx-delete="/api/services/traefik/env/TRAEFIK_API_DASHBOARD"
            hx-target="#env-TRAEFIK_API_DASHBOARD"
            hx-swap="outerHTML">Delete</button>
  </div>

  <!-- Add new var -->
  <form hx-post="/api/services/traefik/env"
        hx-target="#env-editor"
        hx-swap="beforeend">
    <input type="text" name="key" placeholder="VARIABLE_NAME" required>
    <input type="text" name="value" placeholder="value" required>
    <button>Add</button>
  </form>

  <!-- Redeploy prompt (shown after changes) -->
  <div class="alert" id="redeploy-prompt">
    Environment changed.
    <button hx-post="/api/services/traefik/deploy">Redeploy Now</button>
  </div>
</div>
```

### New Methods Jed Needs:

```go
// Get env vars as structured data (not raw file)
func (jed *Jed) GetEnv(serviceName string) (map[string]string, error)

// Save env vars (jed handles formatting to .env)
func (jed *Jed) SaveEnv(serviceName string, env map[string]string) error

// Reload service configs after .env change
func (jed *Jed) Reload(ctx context.Context) error
```

### Implementation Details:

**Data Flow:**
1. Read → Parse `.env` file into `map[string]string`
2. Edit → Form with add/delete/modify
3. Save → Validate and write back to `.env` format
4. Apply → Optionally redeploy container

**Validation Rules:**

Key validation:
- Must match `^[A-Z0-9_]+$` (uppercase, numbers, underscores)
- No duplicates
- Can't be empty

Value validation:
- Can be anything (strings, numbers, booleans as strings)
- No newlines (would break .env format)
- Trim whitespace

**Design Decisions Made:**
- ✅ **Save strategy**: Auto-save on blur for edits, button for add
- ✅ **Comments**: Lose them (add UI note: "Comments will be removed")
- ✅ **Redeploy**: Show prompt, let user decide when
- ✅ **Filesystem**: Store path in jed for cleaner API

**Filesystem Approach:**
```go
type Jed struct {
    client Client
    logger Logger
    svcs   []Service
    fsPath string  // NEW: store path for later writes
}
```

## Next Steps

### Immediate (For Env Editing):
1. Add `fsPath string` to `Jed` struct
2. Implement `GetEnv(serviceName string) (map[string]string, error)`
3. Implement `SaveEnv(serviceName string, env map[string]string) error`
4. Implement `Reload(ctx context.Context) error`
5. Add validation for env var keys/values
6. Write tests for new methods

### Soon After:
1. Decide on extra YAML fields approach (Option 1 vs 2)
2. Consider authentication for web UI
3. Plan streaming logs (SSE or WebSocket)
4. Multi-service operations (deploy/undeploy multiple)

### From TODO (Still Valid):
- [ ] Test missing image error path in New
- [ ] Test Deploy error paths
- [ ] Test Logs error paths
- [ ] Add hondo package var for testable random source
- [ ] Allow choosing stdout and/or stderr in Logs
- [ ] Add Containers helper methods: State(), Exists()
- [ ] Streaming logs - return io.ReadCloser
- [ ] Add usage examples in godoc

## Questions for Next Session
1. Should we store the filesystem as `string` path or keep `fs.FS`?
2. How to handle file writes with `embed.FS` (read-only)?
3. Need two filesystems - one for reading embedded, one for writing?
4. Authentication strategy for web UI?
5. Hot reload - watch services.yaml for changes?
