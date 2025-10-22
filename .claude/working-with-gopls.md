# Working with Gopls MCP

This guide helps Claude Code work efficiently with this Go codebase using gopls MCP tools.

## Recommended Startup Sequence

When starting work on this codebase, follow this sequence:

1. **`go_workspace`** - Always start here to understand the workspace structure (module, workspace, or GOPATH)

2. **`go_package_api`** - Get the package-level godoc and all exported types/functions
   - Shows the mental model and architecture overview
   - Much more efficient than reading files one by one
   - Example: `go_package_api({"packagePaths": ["github.com/clarktrimble/jed"]})`

3. **`go_search`** - When looking for a specific symbol
   - Uses fuzzy matching on fully qualified names
   - Example: `go_search({"query": "server"})` finds types, functions, variables
   - More efficient than grepping when you don't know exact location

4. **`go_file_context`** - After reading a file for the first time
   - Shows what the file depends on from other files in the same package
   - Helps understand intra-package dependencies
   - Example: `go_file_context({"file": "/path/to/jed.go"})`

5. **Read specific files** - Only after understanding the architecture
   - Use the Read tool for implementation details
   - Now you have context from steps 1-4

## Why This Sequence Matters

The godoc package comments explain the mental model - abstractions, lifecycle, key behaviors. Getting this overview first (via `go_package_api`) gives you the conceptual framework before diving into implementation details.

## During Editing

When modifying code:

1. **`go_symbol_references`** - Before changing any symbol definition
   - Find all references to understand impact
   - Example: `go_symbol_references({"file": "/path/to/jed.go", "symbol": "Deploy"})`

2. **`go_diagnostics`** - After every code modification
   - Check for build and analysis errors
   - Pass paths of files you edited
   - Example: `go_diagnostics({"files": ["/path/to/jed.go"]})`

3. **Fix errors and re-run diagnostics** - Iterate until clean

4. **Run tests** - Only after diagnostics are clean

## Common Mistakes

- **Jumping straight to reading files** - Misses the architecture overview in godoc
- **Skipping `go_workspace`** - Don't assume you know the project structure
- **Not using `go_file_context`** - Leads to missing important dependencies
- **Using grep instead of `go_search`** - gopls is faster and more accurate for symbols
