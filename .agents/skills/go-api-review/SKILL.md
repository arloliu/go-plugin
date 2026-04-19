---
name: go-api-review
description: Reviews the go-plugin library's exported API for clarity, completeness, and developer experience — evaluated from the perspective of both host-program authors and plugin implementors. Reads only exported API (Godoc) and README/examples.
---

# Go API Review Skill

As an agent using this skill, your task is to evaluate the go-plugin library's exported API as a user would encounter it: someone writing a host program that manages plugin subprocesses, or a plugin author implementing `Plugin` / `GRPCPlugin`. Rely exclusively on the exported API (Godoc), `README.md`, and `examples/`. **Do not read `internal/` to answer questions about the public contract.**

---

## Scope

The public surface of go-plugin has two sides:

**Host side** (program that launches and manages plugins):
- `Client` / `ClientConfig` — subprocess lifecycle, handshake, reattach, TLS, checksum
- `ClientProtocol` / `RPCClient` / `GRPCClient` — dispensing plugin interfaces
- `HandshakeConfig`, `ReattachConfig`, `SecureConfig`
- `CleanupClients`, `Discover`

**Plugin side** (the subprocess being loaded):
- `Plugin` interface (net/rpc: `Server`, `Client`)
- `GRPCPlugin` interface (`GRPCServer`, `GRPCClient`)
- `NetRPCUnsupportedPlugin` — embed to opt out of net/rpc
- `Serve` / `ServeConfig`

**Shared / advanced**:
- `MuxBroker` — multiplexed net/rpc connections for complex argument types
- `GRPCBroker` — multiplexed gRPC connections
- `runner.Runner` — pluggable subprocess launcher interface
- `TestPluginRPCConn` / `TestPluginGRPCConn` — in-process test helpers

---

## 1. Interface Design and API Quality

1. **Interfaces:**
   - Are `Plugin` and `GRPCPlugin` single-purpose and clearly named?
   - Does each interface method have enough Godoc to understand when it is called and what it must return?
   - Is it clear when to embed `NetRPCUnsupportedPlugin` vs. implementing both `Plugin` and `GRPCPlugin`?

2. **Config structs:**
   - Does `ClientConfig` document every field — including optional vs. required, zero-value behavior, and mutual-exclusion constraints (e.g., `Reattach` + `SecureConfig` can't coexist)?
   - Does `ServeConfig` document the expected `HandshakeConfig`, how `PluginMap` is keyed, and when `GRPCServer` / `GRPCOpts` are used?
   - Are deprecated or discouraged fields called out as such?

3. **Functional options and variadic parameters:**
   - Are there any `WithX` option functions? If so, do they follow idiomatic Go conventions?
   - Is `grpc.DialOption` / `grpc.ServerOption` passthrough well-documented?

4. **Error handling:**
   - Are sentinel errors (`ErrProcessNotFound`, `ErrChecksumsDoNotMatch`, etc.) exported and usable with `errors.Is`?
   - Can callers distinguish handshake failure from subprocess launch failure from protocol mismatch?

---

## 2. Documentation and Learning Curve

1. **Package-level doc:**
   - Does the package doc in `plugin.go` give a reader a working mental model in under a minute?
   - Does it explain: (a) host vs. plugin role, (b) which interface to implement for gRPC vs. net/rpc, (c) where to start?

2. **README:**
   - Does it show the minimal host-side and plugin-side wiring end-to-end?
   - Are `HandshakeConfig`, `PluginMap`, `Serve`, `NewClient`, and `Dispense` all demonstrated?
   - Does it point to `examples/` for the full reference implementations?
   - Are protocol negotiation, reattach, and mTLS mentioned (even briefly)?

3. **Examples:**
   - Do `examples/basic/`, `examples/grpc/`, `examples/streaming/`, `examples/bidirectional/`, `examples/negotiated/` compile and run correctly?
   - Does each example demonstrate its intended feature clearly, with minimal noise?
   - Can a new plugin author copy an example and adapt it to their use case without reading source?

4. **Testing helpers:**
   - Is `TestPluginRPCConn` / `TestPluginGRPCConn` documented well enough that a plugin author can write unit tests for their own plugin implementation?

---

## 3. Potential Misuse and Ambiguity

1. **Lifecycle ordering:**
   - Is it clear that `client.Client()` must succeed before `Dispense`?
   - Is it clear that `client.Kill()` is idempotent and safe to defer?
   - Is it documented that `Serve` blocks and that plugin binaries should call it from `main()`?

2. **Concurrency expectations:**
   - Is it documented that dispensed implementations are safe for concurrent use (or not)?
   - Is `MuxBroker.AcceptAndServe` / `GRPCBroker.AcceptAndServe` documented with goroutine expectations?

3. **Protocol backward compatibility:**
   - Is it clear to plugin authors when they must increment `ProtocolVersion` to force a re-handshake?
   - Is `CoreProtocolVersion` documented as an internal library version (not under user control)?

4. **Platform behavior:**
   - Are Windows/Unix differences (e.g., socket type, process group signals) surfaced anywhere for users who care?
