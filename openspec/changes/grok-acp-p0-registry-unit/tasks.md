# Tasks: P0 — Grok ACP Registry & Unit Tests

## Overview

Unit tests and registry integration for the Grok ACP Chat driver binding.
No live integration; gates all subsequent phases.

---

## Task 0.1: Create grokacp Package Scaffolding

**File**: `backend/internal/adapters/chatdriver/grokacp/driver.go`

Create the package with `New()`, `configure()`, `sessionMode()`, and
`sessionOptions()` functions as specified in design.md.

**Verification**:
```bash
cd backend && go build ./internal/adapters/chatdriver/grokacp/...
```

---

## Task 0.2: Write driver_test.go Unit Tests

**File**: `backend/internal/adapters/chatdriver/grokacp/driver_test.go`

Tests to implement:

| Test | Assertion |
|------|-----------|
| `TestHarness` | `driver.Harness() == domain.HarnessGrok` |
| `TestConfigure_DefaultPermissions` | Args: `["--no-auto-update", "agent", "stdio"]` |
| `TestConfigure_BypassPermissions` | Args include `--always-approve` |
| `TestConfigure_AcceptEdits` | Args do NOT include `--always-approve` |
| `TestConfigure_Auto` | Args do NOT include `--always-approve` |
| `TestConfigure_ModelOverride` | Args include `["--model", "grok-code-fast"]` |
| `TestSessionMode_Default` | Returns `""` |
| `TestSessionMode_AcceptEdits` | Returns `"acceptEdits"` |
| `TestSessionMode_Auto` | Returns `"auto"` |
| `TestSessionMode_BypassPermissions` | Returns `"bypassPermissions"` |
| `TestSessionOptions_AdvertisedModel` | `"grok-code-fast"`, `"grok-4.5"` → one `model` option, id unchanged |
| `TestSessionOptions_EmptyModel` | `""` → no options |
| `TestBareModelReachesLaunch` | Start/Resume with `"grok-code-fast"` is not rejected by AO |

**Verification**:
```bash
cd backend && go test -v ./internal/adapters/chatdriver/grokacp/...
```

---

## Task 0.3: Register Driver in Registry

**File**: `backend/internal/adapters/chatdriver/registry/registry.go`

1. Add import: `"github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/grok"`
2. Add import: `"github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/grokacp"`
3. Add to `Build()`: `grokacp.New(grok.New(), log),`

**Verification**:
```bash
cd backend && go build ./internal/adapters/chatdriver/registry/...
```

---

## Task 0.4: Update Registry Test

**File**: `backend/internal/adapters/chatdriver/registry/registry_test.go`

Add `domain.HarnessGrok` to the expected Chat drivers list in `TestShippedChatDrivers`.

**Verification**:
```bash
cd backend && go test -v ./internal/adapters/chatdriver/registry/...
```

---

## Task 0.5: Run Full CI Suite

```bash
# From repo root
mise run lint

# Backend specific
cd backend
gofmt -l .
go build ./...
go vet ./...
go test ./...
go test -race ./...
```

**Verification**: All checks pass with zero errors.

---

## Exit Criteria Checklist

### G1: Independent Deep Review
- [ ] Review conducted by non-author
- [ ] Review note created at `reviews/g1-review.md`
- [ ] Review verdict: PASS or FAIL with rationale

### G2: Full CI Suite
- [ ] `gofmt -l .` returns no files
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `mise run lint` passes (golangci-lint v2.13.2)

### G3: Live Mini-Task (WAIVED)
- [x] **WAIVED** for P0 — unit tests only, no live Grok required

### G4: AO Frontend/CLI UI (WAIVED)
- [x] **WAIVED** for P0 — unit tests only, no UI verification required

---

## Reference Files

Existing native ACP drivers to mirror:
- `backend/internal/adapters/chatdriver/opencodeacp/driver.go`
- `backend/internal/adapters/chatdriver/ompacp/driver.go`
- `backend/internal/adapters/chatdriver/kimchiacp/driver.go`

Existing Grok agent adapter to reuse:
- `backend/internal/adapters/agent/grok/grok.go`
- `backend/internal/adapters/agent/grok/auth.go`
