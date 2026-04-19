---
trigger: always_on
glob: "**/*.go"
description: Run linter after modifying Go files
---

# Lint After Write

After modifying any `.go` file:

1. **Run:** `make lint`
2. **Fix:** All reported issues before committing.
3. **Re-run:** Until clean.

## Common Fixes
| Lint Error | Fix |
|------------|-----|
| `goimports` | Run `goimports -w file.go` |
| `errcheck` | Handle the error or explicitly ignore with `_ =` |
| `govet` | Fix type/format mismatches |
| `staticcheck` | Follow the suggested fix; check the SA/ST rule ID for context |
| `gosec` | Fix the underlying issue; suppress with `//nolint:gosec // <reason>` only when justified |
| `unused` | Remove dead code |
| `misspell` | Fix the typo |
