# Documentation Strategy

## Purpose

Document Jed for two audiences:
- **Future Me**: Coming back after months, need to remember "why did I do it this way?"
- **Future Claude**: Using MCP tools to understand and help modify the code

## Three-Layer Approach

### 1. Godoc (package comments + type/function docs)

**Purpose:** API reference and architectural overview

**What belongs here:**
- Package-level comment explaining core concepts
- Architecture overview (Store, Services, Containers, Env)
- How the pieces fit together
- Documentation for every exported type/function/method

**Why this layer:**
- Standard Go documentation
- Accessible via `go_package_api` MCP tool (Future Claude)
- Displayed on pkg.go.dev
- Lives with the code - less likely to drift

### 2. README.md

**Purpose:** Quick start and project landing page

**What belongs here:**
- Brief description of what Jed is
- Installation instructions
- One basic usage example showing the happy path
- Links to design.md
- Any GitHub-specific info

**Why this layer:**
- First thing people see
- Gets you productive quickly
- Displayed on both GitHub and pkg.go.dev
- Focus: "get started in 5 minutes"

### 3. design.md

**Purpose:** Design decisions and rationale

**What belongs here:**
- Why slice-based APIs instead of maps?
- Why decouple env from services?
- Why just-in-time env loading in Deploy()?
- Tradeoffs considered
- Things that might seem odd but are intentional
- Evolution of design (what changed and why)

**Why this layer:**
- Answers "what was I thinking?"
- Helps understand intent behind non-obvious choices
- Useful when considering changes
- Not API details - those are in godoc

## What NOT to duplicate

- Full API listings (godoc has this)
- Type definitions (godoc has this)
- Method signatures (godoc has this)
- Basic usage (README has this)

## Workflow

When making changes:
1. Update code + godoc comments (always in sync)
2. Update design.md if decision/rationale changes
3. Update README only if quick start example changes
