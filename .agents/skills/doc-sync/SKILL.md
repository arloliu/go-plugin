---
name: doc-sync
description: Audits and updates README.md and package-level Godoc in the go-plugin library to match the current exported API, field names, and behavioral contracts. Does not invent changes — only syncs what is verifiably out of date.
---

# Doc-Sync Skill

**Assumed Role:** Maintainer who owns documentation accuracy for go-plugin library consumers.

**Goal:** Ensure `README.md` and every package-level Godoc comment in the public surface accurately reflects the *current* source. Flag discrepancies, then apply targeted fixes. Do not rewrite prose for style — only correct factual errors, stale signatures, missing symbols, and outdated behavioral descriptions.

---

## Scope

By default, audit:
- `README.md` — getting-started walkthrough, feature list, architecture description, examples
- Root `plugin` package Godoc — all exported types, functions, methods, constants, variables
- `runner/` package Godoc — `Runner` interface and any exported helpers
- `testing.go` — `TestPluginRPCConn` / `TestPluginGRPCConn` doc comments
- `examples/` — verify code compiles and matches what README describes

You may narrow scope: `README only`, `Godoc only`, or a specific symbol (e.g., `ClientConfig`).

---

## Phase 1 — Inventory

### 1a. Ground truth from source

For each file in scope (not `internal/`, not `_test.go`), extract:
- All exported symbols: types, functions, methods, constants, variables.
- Struct field names and types (for `ClientConfig`, `ServeConfig`, `HandshakeConfig`, `ReattachConfig`, `SecureConfig`).
- Sentinel error variable names.
- Interface method signatures.

### 1b. Claims from docs

From `README.md` and each Godoc comment, extract:
- Every code block (` ```go `) — type signatures, function calls, config fields.
- Every bullet or table row naming an exported symbol, config field, or feature.
- Any behavioral claim (e.g., "Kill is safe to call multiple times", "gRPC is preferred over net/rpc").

---

## Phase 2 — Diff: Docs vs. Source

Compare docs against source. Produce findings in this format:

```
[FILE] README.md
  STALE   Line 42: ClientConfig field shown as `Cmd *exec.Cmd` but renamed/removed
  PHANTOM Line 88: references plugin.Discover() with wrong signature
  MISSING         GRPCBrokerMultiplex field not mentioned anywhere

[FILE] client.go (Godoc)
  MISSING ErrGRPCBrokerMuxNotSupported has no doc comment
  STALE   Kill() doc says "SIGTERM" but code sends os.Interrupt on Windows
```

Categories:
- **STALE** — documented differently from source (wrong name, wrong type, wrong behavior).
- **MISSING** — exists in source but not documented.
- **PHANTOM** — documented but no longer exists in source.
- **OK** — accurate; no change needed.

---

## Phase 3 — Fix

For each STALE, MISSING, or PHANTOM finding:

1. Read the relevant source file to confirm current truth.
2. Apply a targeted edit — only the affected lines.
3. Preserve intent — if old text explained *why*, keep the why, fix only the *what*.

### Fix rules
- **Signatures:** Match source exactly. No paraphrasing of type names.
- **Field names:** Match struct tags or field identifiers verbatim.
- **Removed symbols:** Delete their doc sections. Add a one-line "renamed to `NewName`" note where relevant.
- **New symbols:** Add a stub entry matching the existing Godoc style in the file.
- **Do not:** restructure sections, change prose tone, add new examples beyond what source makes self-evident, or modify historical design notes.

---

## Phase 4 — Report

```
## Doc-Sync Report

### Changes Applied
- README.md: corrected ClientConfig.Cmd description, added GRPCBrokerMultiplex field
- client.go: added Godoc for ErrGRPCBrokerMuxNotSupported, corrected Kill() platform note

### Still Needs Attention (manual review required)
- README.md line 110: behavioral claim about yamux session reuse — verify against source

### No Changes Needed
- runner/runner.go — accurate
- testing.go — accurate
```

Mark "needs attention" when:
- The correct fix requires understanding intent not derivable from source alone.
- Multiple plausible fixes exist and the wrong choice could mislead users.
- The claim is in a design note that may be intentionally forward-looking.

---

## Constraints
- **Never fabricate behavior.** If source is ambiguous, mark "needs attention" and quote both the doc claim and the source.
- **Minimal diffs.** Edit only lines that are wrong.
- **`internal/` is off-limits** for making claims about public behavior. If a doc says "internally uses yamux" and you can verify that from `internal/`, leave the note — but don't update it based on internal code changes alone.
