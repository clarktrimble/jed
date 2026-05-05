# Using gopls MCP for Go Development

## Quick Reference

Instead of running `go build` or `go test` to check for errors, use the gopls MCP tool:

```
mcp__gopls-mcp__go_diagnostics
```

This provides:
- Instant feedback without bash execution
- Better error messages with line numbers
- Type errors, undefined symbols, etc.
- Works even if code doesn't compile yet

## When to Use

- **After refactoring** - Check what broke
- **Before running tests** - See if code will compile
- **During development** - Quick syntax/type checking
- **Instead of go build** - Faster, no side effects

## Other gopls MCP Tools

- `mcp__gopls-mcp__go_workspace` - Understand workspace layout
- `mcp__gopls-mcp__go_file_context` - See file dependencies
- `mcp__gopls-mcp__go_search` - Find symbols
- `mcp__gopls-mcp__go_symbol_references` - Find all references
- `mcp__gopls-mcp__go_package_api` - See package API

## Example Output

```
File `/Users/.../jed_test.go` has the following diagnostics:
120:19-120:31: [Error] undefined: jed.ServiceStore
148:54-148:62: [Error] too many arguments in call to cfg.New
```

Much clearer than compiler errors!

## Remember

Use gopls diagnostics FIRST before trying to build/test. It's faster and gives better feedback.
