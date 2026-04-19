# 400 - Documentation Standards

## General
- **Godoc:** All exported symbols MUST have doc comments.
- **First line:** Start with the symbol name. One-line summary ending in a period.
- **README:** Keep `README.md` accurate — signatures, config fields, and example code must match the current source.
- **Package doc:** The root `plugin` package has a package-level comment in `plugin.go`; keep it updated if the package surface changes significantly.

## Godoc Style

Match the existing style in the codebase — concise, direct, no elaborate parameter/return sections unless the behavior is genuinely non-obvious:

```go
// Client handles the lifecycle of a plugin application. It launches
// plugins, connects to them, dispenses interface implementations, and handles
// disconnect and cleanup.
type Client struct { ... }

// NewClient creates a new plugin client which manages the lifecycle of an external
// plugin process and allows dispensing an interface to that plugin.
func NewClient(config *ClientConfig) (c *Client) { ... }

// Dispense returns the implementation of the requested plugin. If a plugin
// with the given name is not found, an error is returned.
func (c *RPCClient) Dispense(name string) (any, error) { ... }
```

When a parameter or return value has a non-obvious constraint, add a short note in the body — not a formal Parameters/Returns section:

```go
// Kill ends the plugin subprocess. If the subprocess does not exit within the
// configured shutdown timeout, it is force-killed with SIGKILL.
// Kill is safe to call multiple times.
func (c *Client) Kill() { ... }
```

## Examples
- Place runnable examples in `examples/<name>/` with a `main.go` that demonstrates both the host and plugin side.
- For Godoc examples (`Example*` functions in `*_test.go`), only add them for non-obvious usage patterns — the existing examples directory is the primary learning path.

## What Not to Document
- Don't add comments explaining what the code obviously does.
- Don't document unexported symbols unless they implement a subtle invariant a future reader would miss.
- Don't write `// TODO` comments — open a real issue instead.
