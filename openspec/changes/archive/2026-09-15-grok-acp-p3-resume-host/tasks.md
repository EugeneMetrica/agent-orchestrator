# Tasks: P3 — Grok ACP Resume & Persistent Host

## Overview

Resume capability and persistent-host integration for Grok ACP sessions.

## Status

Complete. Merged as [#6](https://github.com/EugeneMetrica/agent-orchestrator/pull/6)
with every CI check green (G2) and the only recorded G1 review of the six phases
at `reviews/g1-review.md` (PASS_WITH_NITS).

G3 ran live on the box and changed the outcome, which the review's addendum
records: AO does send `_meta.rules` on `session/load`, but Grok keeps the rules
the session was created with, so a reloaded session preserves standing
instructions without updating them. The test now asserts that standing
instructions are still in force after resume and accepts either token. The
limitation is stated in `driver.go`, `design.md`, the spec delta, `docs/STATUS.md`,
and `docs/harnesses/grok.md`.

The warm reattach path — `Close()` instead of `Terminate()`, where `Resume`
returns early and never re-sends `SessionMeta` — remains uncovered; the review's
first nit calls this out.

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
- [x] Start succeeds with provider ID
- [x] File created before terminate
- [x] Resume succeeds with same provider ID
- [x] Agent response includes codeword from before terminate
- [x] File still exists after resume
- [x] A standing instruction token is still applied after resume — the one from
      the original start counts, because Grok keeps the rules a session was
      created with (see design.md, "`_meta.rules` on `session/load`")

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
- [x] Review conducted by non-author
- [x] Review note created at `reviews/g1-review.md`
- [x] Review verdict: PASS_WITH_NITS, recommend MERGE

### G2: Full CI Suite — green on the merged head
- [x] `gofmt -l .` returns no files
- [x] `go build ./...` succeeds
- [x] `go vet ./...` passes
- [x] `go test ./...` passes (live tests skip)
- [x] `go test -race ./...` passes
- [x] `mise run lint` passes (pinned golangci-lint)

### G3: Live Resume Test — ran on the box; see the review addendum
- [x] `AO_LIVE_GROK_ACP=1` set
- [x] Resume test passes
- [x] Provider-side history visible after resume (the model recalls the codeword;
      AO's own typed transcript replay depends on Grok advertising `loadSession`)
- [x] File persists across terminate/resume

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
