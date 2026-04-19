# go-plugin — Agent Rules Index

> **CONTEXT**: This is a Go plugin system library over RPC/gRPC (`github.com/arloliu/go-plugin`), a maintained fork of `hashicorp/go-plugin`.
> **ACTION**: Read the files below in order before beginning work.

## Rule Index

### 0. Working Principles
- **[050-principles.md](050-principles.md)**
  *Behavioral guidelines: surface uncertainty, minimize changes, verify with code, define success criteria.*

### 1. Core Directives
- **[100-overview.md](100-overview.md)**
  *Identity, project structure, architecture notes, dependencies, and prime directives.*

### 2. Standards
- **[200-coding-style.md](200-coding-style.md)**
  *Go idioms, error handling, naming, loop patterns.*
- **[300-testing.md](300-testing.md)**
  *Test organization, **CRITICAL** async testing rules, make targets.*
- **[400-documentation.md](400-documentation.md)**
  *Godoc format and README standards for a library.*

### 3. Workflow & Safety
- **[500-workflow.md](500-workflow.md)**
  *Git conventions, pre-commit checks, make targets reference.*
- **[600-perf-sec.md](600-perf-sec.md)**
  *Performance (RPC/gRPC paths) and security (handshake, mTLS, subprocess launch).*
- **[700-lint-after-write.md](700-lint-after-write.md)**
  *Automated linting workflow and common fixes.*

---
*Rules are split for readability and context optimization.*
