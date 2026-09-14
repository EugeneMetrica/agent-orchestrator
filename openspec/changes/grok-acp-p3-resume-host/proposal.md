# Proposal: P3 — Grok ACP Resume & Persistent Host

## Why

P1-P2 prove Probe→Start→SendTurn and permission modes work. P3 validates the
Resume capability and optional persistent-host integration for session
continuity across daemon restarts.

## Problem Statement

Users expect to:
1. Close AO and reopen without losing their Grok conversation
2. Have the conversation history available after daemon restart
3. Continue working with the same provider conversation ID

Without Resume support, Grok Chat sessions would be ephemeral and lose context
on every daemon restart.

## Proposed Solution

1. Extend `live_test.go` with `TestLiveGrokACPResume` that:
   - Starts a session, sends a turn creating `before.txt`
   - Terminates the conversation
   - Resumes with the same provider conversation ID
   - Sends a turn that references/verifies `before.txt` exists
2. Document any Grok-specific `_meta` requirements (yoloMode, rules) if needed.
3. If Grok's ACP requires SessionMeta extension, document the decision:
   - Prefer canonical `nativeacp` SessionMeta extension if small
   - Or document why piacp-style `acpdriver.New` is needed

## Key Constraints

- **Provider conversation ID**: Must be captured from Start and reused in Resume.
- **File persistence**: Verify files created before terminate are visible after
  resume.
- **History visibility**: Agent should see conversation history after resume.

## Success Criteria

1. `TestLiveGrokACPResume` passes with file proof across terminate/resume.
2. Agent response after resume references content from before terminate.
3. Full CI suite passes.
4. SessionMeta decision documented if Grok needs `_meta.rules` or `_meta.yoloMode`.

## Non-Goals

- Desktop E2E (that is P4).

## Gates (P3)

| Gate | Description | Waiver |
|------|-------------|--------|
| G1 | Independent deep review → review note PASS/FAIL in `reviews/` | Required |
| G2 | Full suite: gofmt, go build/vet, `go test -race ./...`, npm run lint / golangci-lint v2.12.2 | Required |
| G3 | Live resume test: file proof before/after terminate | Required |
| G4 | AO frontend: session survives daemon restart with history | Required |

## Impact

- **Scope**: `backend/internal/adapters/chatdriver/grokacp/live_test.go` (extend),
  possibly `nativeacp` or driver design if SessionMeta needed.
- **Risk**: Low. Pattern mirrors `cursoracp/live_test.go` resume test.
- **Dependencies**: P2 complete.
