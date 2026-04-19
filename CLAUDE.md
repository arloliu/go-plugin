# go-plugin — Claude Code Configuration

## What This Project Is

**go-plugin** (`github.com/arloliu/go-plugin`) is a Go plugin system over RPC/gRPC — a maintained fork of [`hashicorp/go-plugin`](https://github.com/hashicorp/go-plugin). It lets host programs launch plugins as subprocesses and communicate with them over net/rpc or gRPC, as if the plugin were running in-process.

Public API surface (the contracts library consumers and plugin authors care about):
- **Root `plugin` package** — `Plugin`, `GRPCPlugin`, `Client`, `ClientConfig`, `Serve`, `ServeConfig`, `MuxBroker`, `GRPCBroker`, `HandshakeConfig`, `ReattachConfig`, `SecureConfig`.
- **`runner/`** — `Runner` interface for pluggable subprocess launchers.
- **`testing.go`** — `TestPluginRPCConn` / `TestPluginGRPCConn` helpers for in-process plugin testing.
- **`examples/`** — reference implementations (basic, grpc, streaming, bidirectional, negotiated).

Everything under `internal/` is private — do not reference it in docs, examples, or user-facing material.

## Working Principles

- **Surface uncertainty before coding.** State assumptions explicitly. If multiple interpretations exist, present them — don't pick silently. If something is unclear, stop and ask.
- **Minimum change that solves the problem.** No speculative features, unnecessary abstractions, or unasked-for flexibility. Every changed line should trace directly to the request.
- **Don't guess — verify with code.** When uncertain about behavior (API semantics, concurrency, edge cases), write a small test or prototype to confirm rather than assuming. For performance assumptions, benchmark before and after — don't refactor for speed based on intuition alone.
- **Define verifiable success criteria before implementing.** Transform vague tasks ("fix the bug") into concrete checks ("write a test that reproduces it, then make it pass"). For multi-step tasks, state a brief plan with verification steps.

## Git Conventions

**Never add `Co-Authored-By` or any other attribution trailers to git commit messages.**

## How to Work in This Codebase

All coding rules, testing conventions, documentation standards, workflow steps, and performance/security guidelines are in numbered rule files. **Read them before making changes.**

All agent skills are invocable capabilities for structured reviews — use them when asked.

@AGENTS.md

## Invoking Skills

To run a skill, ask Claude to use it by name:

- `/go-api-review` — Review the exported API (`plugin` package, `runner/`, `testing.go`) for clarity, completeness, and developer experience for both host programs and plugin implementations.
- `/qa-review` — Review for correctness, fault tolerance, error propagation, and concurrency safety around plugin lifecycle, handshake, reattach, mTLS, and RPC/gRPC communication.
- `/doc-sync` — Audit and update `README.md` and package-level Godoc to match the current exported API.
