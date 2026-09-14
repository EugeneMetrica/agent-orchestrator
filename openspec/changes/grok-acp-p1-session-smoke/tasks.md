# Tasks: P1 — Grok ACP Session Smoke Test

## Overview

Live smoke test proving Probe→Start→SendTurn works with real Grok CLI and
creates verifiable files on disk.

---

## Task 1.1: Create live_test.go

**File**: `backend/internal/adapters/chatdriver/grokacp/live_test.go`

Implement `TestLiveGrokACP` following `cursoracp/live_test.go` pattern:

1. Skip unless `AO_LIVE_GROK_ACP=1`
2. Create driver via `New(grok.New(), nil)`
3. Probe for binary + auth
4. Start conversation with standing instruction
5. Send turn asking to create `proof.txt`
6. Wait for turn completion
7. Verify standing instruction token in response
8. Verify `proof.txt` exists with correct content

**Verification**:
```bash
AO_LIVE_GROK_ACP=1 go test -v ./internal/adapters/chatdriver/grokacp/...
```

---

## Task 1.2: Implement Helper Functions

**File**: `backend/internal/adapters/chatdriver/grokacp/live_test.go`

Add helper functions (may inline or reference cursoracp):

- `liveEnvMap()` — returns current environment as map[string]string
- `liveDataDir(t)` — returns AO data dir path
- `sendLiveTurn(ctx, t, conv, text)` — sends user message and starts turn
- `waitForLiveTurn(ctx, t, conv, turnID, approve)` — waits for completion

**Verification**: Helpers compile and work in TestLiveGrokACP.

---

## Task 1.3: Update docs/STATUS.md

**File**: `docs/STATUS.md`

Add Grok Chat driver to the status documentation:

```markdown
### Grok (xAI Grok Build)

- **Status**: Chat mode available
- **Transport**: Native ACP via `grok agent stdio`
- **Auth**: Uses existing ~/.grok credentials (XAI_API_KEY, auth.json, config.toml)
- **Live test**: `AO_LIVE_GROK_ACP=1 go test ./internal/adapters/chatdriver/grokacp/...`
```

**Verification**: Documentation renders correctly, links work.

---

## Task 1.4: Run Full CI Suite

```bash
# From repo root
npm run lint

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

## Task 1.5: Run Live Smoke Test (G3)

**Prerequisite**: Grok CLI installed, valid XAI credentials.

```bash
AO_LIVE_GROK_ACP=1 go test -v ./internal/adapters/chatdriver/grokacp/...
```

**Verification**:
- [ ] Probe succeeds
- [ ] Start returns conversation with provider ID
- [ ] Turn completes with state=completed
- [ ] Response contains `GROK_STANDING_TOKEN`
- [ ] `proof.txt` exists with content `grok-acp-ok`

---

## Task 1.6: Verify AO Frontend (G4)

Manual verification using AO desktop app:

1. Start daemon: `cd backend && go run ./cmd/ao start`
2. Open AO frontend
3. Create/select project with Grok harness
4. Create Chat session (not TUI)
5. Send: "Create a file named ui-proof.txt containing hello-from-grok"
6. Verify timeline shows:
   - [ ] User message bubble
   - [ ] Assistant response
   - [ ] Tool activity (command execution)
7. Verify harness shows "Grok"
8. Verify file exists: `<workspace>/ui-proof.txt` with expected content

**Verification**: Screenshot or recording of working UI for review.

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
- [ ] `golangci-lint run` passes (v2.12.2)
- [ ] `npm run lint` passes

### G3: Live Mini-Task
- [ ] `AO_LIVE_GROK_ACP=1` set
- [ ] Test passes with proof.txt verification
- [ ] Standing instruction token verified in response

### G4: AO Frontend/CLI UI
- [ ] Chat session spawned with harness=grok
- [ ] Timeline displays correctly
- [ ] Tool activities visible
- [ ] File created in worktree matches expectation

---

## Reference Files

Live test pattern:
- `backend/internal/adapters/chatdriver/cursoracp/live_test.go`

Status documentation:
- `docs/STATUS.md`
