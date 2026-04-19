# 500 - Development Workflow

## Before Commit
1. Run `make lint` — fix all issues.
2. Run `make test` — all packages must pass with the race detector.
3. If `internal/cmdrunner/testdata/` binaries changed, run `make testdata` and commit the rebuilt binaries.
4. Update `README.md` and Godoc if a public-surface API changed.

## Git Conventions
- **Branches:** `feat/`, `fix/`, `docs/`, `chore/`, `test/`. Scope is welcome: `feat/grpc-broker`, `fix/reattach`.
- **Commits:** Conventional format. Present tense. First line < 50 chars.
    - `feat(broker): support multiplexed gRPC streams`
    - `fix(client): prevent double-kill on reattach`
    - `docs: update README handshake config example`
- **Never** add `Co-Authored-By` or other attribution trailers.

## Code Review Checklist
- [ ] Correctness — does it do what it says?
- [ ] Protocol backward compatibility preserved (no silent wire-format changes)
- [ ] Subprocess safety — no leaked processes, `Kill` / `Cleanup` paths exercised
- [ ] Concurrency — all goroutines have a documented shutdown path
- [ ] `internal/` not referenced from public API, examples, or docs
- [ ] Tests added or updated for changed behavior
- [ ] README / Godoc updated for exported API changes

## Make Targets Reference
```bash
make help          # Print all targets
make fmt           # Run go fmt (rewrites files)
make fmt-check     # Fail if any file is not gofmt-clean (CI gate)
make vet           # Run go vet
make lint          # Run golangci-lint
make test          # Full test suite with race detector
make test-short    # All packages, no race detector (quick iteration)
make cover         # Tests with coverage profile + summary
make cover-html    # Open coverage HTML in browser
make build         # Compile all packages
make check         # Full CI gate: fmt-check + vet + lint + test + build
make testdata      # Rebuild cross-platform test binaries in internal/cmdrunner/testdata
make clean         # Remove coverage artifacts
make update-pkg-cache  # Warm the Go module proxy for the latest git tag
```

## CI Gate
`make check` runs the same steps as `.github/workflows/test.yaml`. Run it locally before pushing to confirm the build will pass.
