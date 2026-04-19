# 200 - Coding Standards & Conventions

## Go Style
- **Idioms:** Follow [Effective Go](https://go.dev/doc/effective_go). Use `goimports`.
- **Types:** Use `any` instead of `interface{}`.
- **Collections:** Use `slices` and `maps` packages from stdlib where available.
- **Context:** Use `context.Context` for cancellation and request-scoped values.
- **Sync:** Prefer `sync/atomic` for simple counters and flags; `sync.Mutex` for compound state.

## Error Handling
- **Static:** `errors.New("message")`.
- **Wrap:** `fmt.Errorf("context: %w", err)`.
- **Check:** `errors.Is()` and `errors.As()` — never string-compare error messages.
- **Naming:**
    - Sentinel: `var ErrFoo = errors.New(...)` (prefix `Err`)
    - Types: `type FooError struct{...}` (suffix `Error`)
- **Type Assert:** Always use comma-ok: `v, ok := x.(Type)`.
- **Return:** Errors are always the last return value. Use early returns to reduce nesting.
- **Explicit ignore:** When ignoring a returned error intentionally, use `_ = expr` rather than a bare call, so readers know it's deliberate.

## Interface Assertions
- Pattern: `var _ Interface = (*Type)(nil)` — place immediately after the type definition.

## Naming
- **Packages:** Short, lowercase — e.g., `plugin`, `runner`.
- **Exported:** CamelCase. **Unexported:** camelCase.
- **Receivers:** Short, consistent with existing usage in the same file.
- **Test helpers:** Accept `testing.TB` not `*testing.T` so they work in benchmarks too.

## Loop Patterns (Go 1.22+)
- Index needed: `for i := range slice`
- No index: `for range slice`
- Simple N: `for range N`
- Benchmarks: `for b.Loop()` (Go 1.24+)
- Don't declare a loop variable you don't use.

## Concurrency
- Document goroutine ownership: where a goroutine is started, what stops it, who owns the `WaitGroup` or `errgroup`.
- Always pair `go func()` spawns with a shutdown path — context cancellation, channel close, or `sync.WaitGroup.Done`.
- Avoid goroutine leaks in tests: use `t.Cleanup` to kill plugin clients and stop servers.
