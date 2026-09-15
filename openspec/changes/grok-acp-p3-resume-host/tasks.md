# Tasks: P3 — Grok ACP Resume & Persistent Host

## Overview

Resume capability and persistent-host integration for Grok ACP sessions.

---

## Task 3.1: Implement TestLiveGrokACPResume

**File**: `backend/internal/adapters/chatdriver/grokacp/live_test.go`

Add resume test following design.md:

1. Start session, create file, establish memory
2. Capture providerID, terminate
3. Resume with same providerID
4. Verify history and file visible

**Verification**:
```bash
AO_LIVE_GROK_ACP=1 go test -v -run Resume ./internal/adapters/chatdriver/grokacp/...
```

---

## Task 3.2: Document SessionMeta Decision

**File**: `backend/internal/adapters/chatdriver/grokacp/driver.go` (inline comments)
**OR**: Design document if significant

If Grok requires `_meta.rules` or `_meta.yoloMode`:

1. Test whether standard ACP metadata works
2. If not, decide: nativeacp extension OR custom acpdriver
3. Document rationale in code comments or design.md update

**Verification**: Decision documented with rationale.

---

## Task 3.3: Run Full CI Suite

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

**Verification**: All checks pass.

---

## Task 3.4: Run Live Resume Test (G3)

**Prerequisite**: Grok CLI installed, valid XAI credentials.

```bash
AO_LIVE_GROK_ACP=1 go test -v -run Resume ./internal/adapters/chatdriver/grokacp/...
```

**Verification**:
- [ ] Start succeeds with provider ID
- [ ] File created before terminate
- [ ] Resume succeeds with same provider ID
- [ ] Agent response includes codeword from before terminate
- [ ] File still exists after resume

---

## Task 3.5: Verify AO Frontend Resume (G4)

Manual verification:

1. Start daemon and frontend
2. Create Grok Chat session
3. Send message: "Remember codeword BETA and create resume-test.txt"
4. Verify file created
5. Stop daemon (Ctrl+C or `ao stop`)
6. Restart daemon
7. Reopen same session
8. Send: "What was the codeword? Does resume-test.txt exist?"
9. Verify:
   - [ ] History visible in UI
   - [ ] Agent response includes BETA
   - [ ] Agent confirms file exists

**Verification**: Screenshot/recording of working resume.

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

### G3: Live Resume Test
- [ ] `AO_LIVE_GROK_ACP=1` set
- [ ] Resume test passes
- [ ] History visible after resume
- [ ] File persists across terminate/resume

### G4: AO Frontend/CLI UI
- [ ] Session survives daemon restart
- [ ] History visible in UI
- [ ] Can continue conversation after restart
- [ ] Files created persist

---

## Reference Files

Resume pattern:
- `backend/internal/adapters/chatdriver/cursoracp/live_test.go` (resume section)

Persistent host:
- `backend/internal/adapters/chatdriver/acp/persistenthost/` (if needed)
