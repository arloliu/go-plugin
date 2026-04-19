# 600 - Performance & Security

## Performance

Hot paths in this library are the per-RPC call dispatch and the broker connection setup. Apply these where message throughput or connection latency matters:

- **Allocations on RPC paths:** Avoid `fmt.Sprintf` and heap allocations inside per-call code paths. Pre-allocate slices with `make([]T, 0, expectedCap)` when size is predictable.
- **gRPC dial options:** Passed once at connection setup, not per call — keep `dialGRPCConn` options small and avoid repeated reflection.
- **Broker connections:** `MuxBroker` and `GRPCBroker` open additional streams on demand; this is intentional but not free. Don't open broker sub-connections unless the plugin interface genuinely requires it.
- **Streaming:** For high-throughput plugins, prefer bidirectional gRPC streams over unary RPCs. See `examples/streaming/` and `examples/bidirectional/`.
- **Benchmarking:** Use `for b.Loop()` (Go 1.24+) in benchmark tests. Profile with `pprof` before optimizing.

## Security

- **Handshake validation:** The `HandshakeConfig` (magic cookie key/value pair + protocol version) is the primary plugin identity check. Never skip or weaken it. Mismatched configs must return a clear error — not silently fall back to an older protocol.
- **mTLS:** When `ClientConfig.TLSConfig` / `ServeConfig.TLSConfig` is set, the library enforces mutual authentication. When adding new transport code, always thread the `*tls.Config` through — never open an unencrypted connection to a plugin that the caller configured for TLS.
- **SecureConfig (binary checksum):** `ClientConfig.SecureConfig` verifies the plugin binary hash before launching it. Never bypass this check. If `SecureConfig` is non-nil and the hash does not match, return `ErrChecksumsDoNotMatch` — do not launch the subprocess.
- **Subprocess launch paths:** Plugin binary paths must be resolved and validated before passing to `exec.Command`. Do not construct paths from user input without sanitizing for traversal (`..`) and absolute-path injection. The `discover.go` helpers handle this — prefer them over raw `exec.LookPath`.
- **Plugin process isolation:** Plugins run as separate OS processes. The host must never trust data from the plugin subprocess without validation. Treat plugin stdout/stderr as untrusted; the stdio forwarding code already buffers and limits line lengths.
- **gosec:** The `gosec` linter is enabled. Treat its warnings as real. Suppress with `//nolint:gosec // <reason>` only when the reason is specific and documented (e.g., `G204` for intentional subprocess exec with a validated path).
- **Secrets:** Never log TLS private keys, plugin credentials, or checksum secrets. Never commit `.env` files.
