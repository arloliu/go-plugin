# 300 - Testing Guidelines

## Organization
- **Unit tests:** Co-located `*_test.go` files in the root `plugin` package and `runner/`.
- **Cross-platform test binaries:** `internal/cmdrunner/testdata/` — rebuild with `make testdata` after changing subprocess launcher behavior.
- **In-process plugin testing:** Use `TestPluginRPCConn` / `TestPluginGRPCConn` from `testing.go` to wire a plugin host and plugin server together without spawning a subprocess.
- **No separate integration/ or e2e/ directories** — the test suite runs entirely with `go test ./...`.

## Rules
- **Context:** Use `t.Context()`.
- **Assertions:** Use `testify` (`require`, `assert`).
- **Cleanup:** Always use `t.Cleanup()` or `defer` for resource cleanup — plugin clients (`client.Kill()`), gRPC connections, temp dirs.
- **Subprocess tests:** Any test that launches a real plugin subprocess must defer `client.Kill()` and verify the subprocess exits cleanly.
- **No time.Sleep:** See Async Testing below.

## Async Testing (CRITICAL)
- **NEVER** use `time.Sleep()` to wait for plugin state.
- Use the testing helpers (`TestPluginRPCConn`, `TestPluginGRPCConn`) which synchronize the handshake before returning.
- For subprocess-based tests that must wait for readiness, use `client.Client()` which blocks until the plugin is ready or returns an error.
- For goroutine completion, use `sync.WaitGroup` or channel receives with a `select` + `time.After` deadline and `t.Fatal` on timeout.

## Test Patterns
**Table-Driven** — use only for multiple cases:
```go
tests := []struct{ name string; input X; want Y }{ ... }
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

**Simple** — for single cases:
```go
func TestOneThing(t *testing.T) {
    got := Do()
    require.Equal(t, want, got)
}
```

**In-process plugin pair:**
```go
func TestMyPlugin(t *testing.T) {
    c, s := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
        "foo": &MyPlugin{},
    })
    defer c.Close()
    raw, err := c.Dispense("foo")
    require.NoError(t, err)
    impl := raw.(MyInterface)
    // ...
}
```

## Running Tests
```bash
make test        # Full test suite with race detector
make test-short  # All packages, no race detector (quick iteration)
make cover       # Tests with coverage profile + summary
make cover-html  # Open coverage HTML in browser
make testdata    # Rebuild cross-platform test binaries in internal/cmdrunner/testdata
```
