# Tasks: P2 — Grok ACP Permission Mode Matrix

## Overview

Live permission matrix test validating all four AO permission modes work
correctly with Grok ACP.

---

## Task 2.1: Implement TestLiveGrokACPPermissionModes

**File**: `backend/internal/adapters/chatdriver/grokacp/live_test.go`

Add permission matrix test following `cursoracp/live_test.go` pattern:

```go
func TestLiveGrokACPPermissionModes(t *testing.T) {
    // Test all four modes: default, accept-edits, auto, bypass-permissions
    // Each creates mode.txt with file editing tool
    // Verify file exists after turn completes
}
```

**Verification**:
```bash
AO_LIVE_GROK_ACP=1 go test -v -run PermissionModes ./internal/adapters/chatdriver/grokacp/...
```

---

## Task 2.2: Add sendLiveTurnWithSettings Helper

**File**: `backend/internal/adapters/chatdriver/grokacp/live_test.go`

Add helper for sending turns with specific approval settings:

```go
func sendLiveTurnWithSettings(
    ctx context.Context,
    t *testing.T,
    conv ports.ChatConversation,
    text string,
    settings ports.ChatTurnSettings,
) ports.ChatTurnRef
```

**Verification**: Helper compiles and works in permission matrix test.

---

## Task 2.3: Run Full CI Suite

```bash
# From repo root
mise run lint

# Backend specific
cd backend
gofmt -l .
go build ./...
go vet ./...
go test ./...          # Live tests skip automatically
go test -race ./...
```

**Verification**: All checks pass.

---

## Task 2.4: Run Live Permission Matrix (G3)

**Prerequisite**: Grok CLI installed, valid XAI credentials.

```bash
AO_LIVE_GROK_ACP=1 go test -v -run PermissionModes ./internal/adapters/chatdriver/grokacp/...
```

**Verification**:
- [ ] default mode: turn completes, mode.txt created
- [ ] accept-edits mode: turn completes, mode.txt created
- [ ] auto mode: turn completes, mode.txt created
- [ ] bypass-permissions mode: turn completes, mode.txt created

---

## Task 2.5: Verify AO Frontend Permission UI (G4)

Manual verification:

1. Start daemon and frontend
2. Create Grok Chat session
3. For each permission mode in UI:
   - [ ] Select mode from dropdown
   - [ ] Send task requiring approval
   - [ ] Verify approval behavior matches mode
4. Document results in review

**Verification**: Screenshot/recording showing mode affects behavior.

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
- [ ] `go test ./...` passes (live tests skip)
- [ ] `go test -race ./...` passes
- [ ] `mise run lint` passes (golangci-lint v2.13.2)

### G3: Live Permission Matrix
- [ ] `AO_LIVE_GROK_ACP=1` set
- [ ] All 4 permission modes pass
- [ ] Each mode creates proof file

### G4: AO Frontend/CLI UI
- [ ] Permission mode UI selector works
- [ ] Mode affects session approval behavior
- [ ] Documented with screenshot/recording

---

## Reference Files

Permission matrix pattern:
- `backend/internal/adapters/chatdriver/cursoracp/live_test.go` (TestLiveCursorACPPermissionModes)
