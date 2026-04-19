---
name: qa-review
description: QA-focused review of go-plugin for correctness, fault tolerance, and concurrency safety — plugin subprocess lifecycle, handshake/reattach, mTLS, gRPC broker, and error propagation.
---

# QA Review — go-plugin Robustness and Correctness

**Assumed Role:** QA engineer responsible for go-plugin's reliability in production host programs.

**Scope:** The full go-plugin library — both the host-side (Client, brokers, reattach) and plugin-side (Serve, gRPC/RPC transport). When narrowing scope, specify by area (e.g., `broker`, `reattach`, `mtls`, `streaming`).

---

## 1. Plugin Lifecycle Correctness

1. **Subprocess launch and handshake:**
   - Is the handshake magic-cookie check enforced before any plugin interface is dispensed?
   - On a protocol version mismatch, does the error message clearly identify the conflict (host version vs. plugin version)?
   - If the plugin binary fails to start (not found, bad checksum, permission denied), is the error propagated cleanly without leaving a zombie subprocess?

2. **Kill and cleanup:**
   - Is `Client.Kill` idempotent? Can it be called concurrently from multiple goroutines?
   - Does `Kill` wait for the graceful shutdown timeout before escalating to SIGKILL?
   - Does `CleanupClients` (the global cleanup triggered by signal handling) safely interact with `Kill` on individual clients?
   - Are `defer client.Kill()` patterns in tests sufficient, or are there cases where the subprocess outlives the test?

3. **Reattach:**
   - Is `ReattachConfig` validated before `Client.Client()` attempts to connect?
   - What happens if the reattach target process has already exited? Is the error specific (`ErrProcessNotFound`) and actionable?
   - Does reattaching to a running plugin correctly resume the existing handshake state, or does it re-handshake?

4. **Stdout/stderr forwarding:**
   - Does gRPC stdio forwarding (`grpc_stdio.go`) buffer correctly under high output volume?
   - If the plugin subprocess crashes mid-stream, does the stdio goroutine exit cleanly without blocking or panicking?

---

## 2. Fault Tolerance and Error Handling

1. **Error propagation:**
   - Are all errors wrapped with `%w` so callers can use `errors.Is` / `errors.As`?
   - Can callers distinguish: subprocess launch failure vs. handshake failure vs. RPC transport error vs. plugin-returned error?
   - Are sentinel errors (`ErrProcessNotFound`, `ErrChecksumsDoNotMatch`, `ErrSecureNoChecksum`, `ErrSecureNoHash`, `ErrSecureConfigAndReattach`, `ErrGRPCBrokerMuxNotSupported`) all reachable via `errors.Is`?

2. **Plugin crashes:**
   - If the plugin subprocess dies mid-RPC, does the host-side `Dispense`d implementation return a typed error or panic?
   - Is the `doneCtx` (passed to `GRPCPlugin.GRPCClient`) canceled promptly when the plugin exits?
   - Does the broker (`GRPCBroker`, `MuxBroker`) clean up its internal streams when the underlying connection drops?

3. **Broker sub-connections:**
   - What happens when a `GRPCBroker.NextId()` sub-connection is never accepted by the plugin? Is there a timeout?
   - Does `MuxBroker.AcceptAndServe` correctly handle the case where the client closes the connection before `Accept` completes?

4. **Streaming:**
   - For bidirectional streams, what happens when the plugin side returns an error mid-stream?
   - Are stream goroutines guaranteed to exit when the context is canceled or the connection drops?

---

## 3. Concurrency and Resource Management

1. **Thread safety:**
   - Is `Client` safe for concurrent calls to `Client()`, `Kill()`, and `Exited()`?
   - Are dispensed plugin implementations (returned by `Dispense`) documented as concurrent-safe or not?
   - Is `GRPCBroker` safe for concurrent `NextId` / `AcceptAndServe` calls?

2. **Goroutine lifecycle:**
   - Every `go func()` in the library must have a documented shutdown path. Audit: broker run loops, stdio forwarders, ping/keepalive goroutines, controller streams.
   - Are all goroutines started in `newGRPCClient` guaranteed to exit when `doneCtx` is canceled?
   - Does `Client.Kill` wait for all library-owned goroutines to exit, or only for the subprocess?

3. **Resource cleanup:**
   - Are gRPC `ClientConn` objects closed in the right order (broker before transport)?
   - Are yamux sessions closed when the plugin exits?
   - Are temporary Unix socket files cleaned up on both normal shutdown and plugin crash?

---

## 4. Security Correctness

1. **SecureConfig enforcement:**
   - Is the binary checksum verified *before* `exec.Command` is called? (Never launch first, verify second.)
   - Is the hash computation streaming or all-in-memory? Is there a risk of OOM on a large plugin binary?

2. **mTLS:**
   - When `TLSConfig` is set, does the library reject connections that present an invalid or missing client cert?
   - Is there a test that verifies the TLS handshake fails with a mismatched CA?

3. **Plugin path handling:**
   - Are plugin paths resolved and validated before being passed to `exec.Command`?
   - Can a caller pass a path with `..` components or shell metacharacters that would bypass intended restrictions?
