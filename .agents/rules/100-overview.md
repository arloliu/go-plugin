# 100 - Project Overview & Prime Directives

## Identity
- **Project:** go-plugin — Go plugin system over RPC/gRPC
- **Module:** `github.com/arloliu/go-plugin`
- **Origin:** Maintained fork of [`hashicorp/go-plugin`](https://github.com/hashicorp/go-plugin)
- **Language:** Go ≥1.24
- **Linting:** `golangci-lint` (via `make lint`, configured in `.golangci.yaml`)

## What go-plugin Does
go-plugin lets host programs run plugins as out-of-process subprocesses and communicate with them over net/rpc or gRPC — as if the plugin were running in the same process. The host launches the plugin binary, performs a handshake (version negotiation, optional mTLS), and then dispenses typed interface implementations over the transport. Plugins can be reattached across host restarts and can communicate bidirectionally via `MuxBroker` / `GRPCBroker`.

## Project Structure
```
go-plugin/
├── *.go                     # Root plugin package (public API)
│   ├── plugin.go            # Plugin, GRPCPlugin interfaces
│   ├── client.go            # Client, ClientConfig — host side
│   ├── server.go            # Serve, ServeConfig — plugin side
│   ├── grpc_broker.go       # GRPCBroker — multiplexed gRPC connections
│   ├── mux_broker.go        # MuxBroker — net/rpc multiplexing
│   ├── grpc_client.go       # GRPCClient — gRPC transport client
│   ├── grpc_server.go       # GRPCServer — gRPC transport server
│   ├── grpc_controller.go   # Lifecycle control over gRPC
│   ├── grpc_stdio.go        # Stdout/stderr forwarding over gRPC
│   ├── mtls.go              # mTLS helpers
│   ├── stream.go            # Streaming support
│   ├── testing.go           # TestPluginRPCConn / TestPluginGRPCConn helpers
│   ├── discover.go          # Plugin binary discovery utilities
│   └── process.go           # Subprocess management helpers
├── runner/                  # Runner interface for pluggable subprocess launchers
├── internal/                # Private implementation (NOT public surface)
│   ├── cmdrunner/           # Cross-platform subprocess runner (exec, reattach)
│   ├── grpcmux/             # gRPC connection multiplexer
│   └── plugin/              # Generated protobuf (broker, controller, stdio)
├── examples/                # Reference implementations
│   ├── basic/               # net/rpc plugin example
│   ├── grpc/                # gRPC plugin example
│   ├── streaming/           # Streaming gRPC example
│   ├── bidirectional/       # Bidirectional gRPC example
│   └── negotiated/          # Protocol negotiation example
└── docs/                    # Additional documentation
```

## Architecture Notes
- **Two transports:** net/rpc (legacy, via `Plugin` interface) and gRPC (preferred, via `GRPCPlugin` interface). New plugins should use gRPC.
- **Plugin interface contract:** The host implements `HandshakeConfig` and a `PluginMap`; the plugin binary calls `Serve`. The host creates a `Client` pointing at the plugin binary and calls `Client.Client()` → `ClientProtocol.Dispense(name)` to get the typed implementation.
- **MuxBroker / GRPCBroker:** Enable bidirectional communication by opening additional multiplexed connections for complex argument types (e.g., `io.Reader`, nested interfaces).
- **Reattach:** A plugin process can survive host restarts. `ClientConfig.Reattach` takes a `ReattachConfig` (network address) to reconnect to a running plugin subprocess.
- **mTLS:** `ClientConfig.TLSConfig` and `ServeConfig.TLSConfig` carry `*tls.Config`. `mtls.go` provides helpers for generating ephemeral cert pairs.
- **Subprocess lifecycle:** `Client.Kill` sends SIGTERM (with configurable grace period) then SIGKILL. The `runner/` package abstracts the launch mechanism so tests and alternative launchers can inject their own runner.
- **Streaming:** Plugins can expose streaming RPCs (client-streaming, server-streaming, bidirectional) over gRPC; see `stream.go` and `examples/streaming/`.

## Prime Directives
1. **Small diffs:** Break work into small, verifiable chunks. Do not rewrite files unnecessarily.
2. **Preserve backward compatibility:** The plugin protocol (`CoreProtocolVersion`, `HandshakeConfig.ProtocolVersion`) is a hard contract. Do not change wire behavior without a deliberate version bump and changelog entry.
3. **Respect the internal boundary:** `internal/` packages are implementation details. Do not reference them in public API surfaces, examples, or docs.
4. **Subprocess safety:** Any code that launches, kills, or reattaches to subprocesses must be tested across platforms and must not leak processes. Use `t.Cleanup` / `defer client.Kill()` in tests.
5. **Dependencies:** Check `go.mod`. Prefer stdlib. Ask before adding new deps.

## Key Dependencies
- **Logging:** `github.com/hashicorp/go-hclog`
- **Multiplexing:** `github.com/hashicorp/yamux`
- **gRPC / Protobuf:** `google.golang.org/grpc`, `google.golang.org/protobuf`, `github.com/golang/protobuf`
- **Process groups:** `github.com/oklog/run`
- **Proto reflection:** `github.com/jhump/protoreflect`
