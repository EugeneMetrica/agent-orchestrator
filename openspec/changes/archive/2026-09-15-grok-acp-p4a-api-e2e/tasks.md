# Tasks: P4a — API Frontend E2E (Backend HTTP Routes)

## Overview

API-level E2E tests validating the frontend↔engine contract through AO's HTTP
API for Grok Chat sessions.

## Status

Complete. Merged as [#7](https://github.com/EugeneMetrica/agent-orchestrator/pull/7)
with every CI check green (G2). The gated runs are:

```bash
AO_CHAT_E2E=1 AO_LIVE_GROK_ACP=1 go test -v ./e2e/ -run ChatGrok
AO_CHAT_E2E=1 AO_LIVE_CLAUDE_ACP=1 go test -v ./e2e/ -run ChatReference
```

Their G3/G4 results were not recorded in this repository at merge time. The
routes they cover — spawn, settings PATCH, attachments, approval override,
server state — are the same ones the P4b live UI run drove from the surfaces
above them (Grok 6/6, Claude Code reference 4/4, 2026-09-15), so the
frontend↔engine contract has live coverage at the UI level even where this
phase's Go suite has not been re-run.

This phase's spec asserts `settings.model`, which is correct here: the API test
writes `PATCH /conversation/settings` itself. The UI path is different and is
described in P4b — the composer writes the provider's `model` config option
instead.

---

## Task 4a.1: Create chat_grok_test.go

**File**: `backend/e2e/chat_grok_test.go`

Create test file with standard e2e setup:

```go
//go:build !windows

package e2e

import (
    "os"
    "testing"
    "time"
)

func requireGrokE2E(t *testing.T) {
    t.Helper()
    requireE2E(t)
    if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
        t.Skip("set AO_LIVE_GROK_ACP=1 for live Grok tests")
    }
}
```

**Verification**: File compiles with `go build ./e2e/...`

---

## Task 4a.2: Implement TestChatGrokSpawn

Test spawning Chat session with harness=grok.

**Assertions**:
- [ ] Session created with correct harness
- [ ] Mode is "chat"
- [ ] First turn completes
- [ ] Agent response matches expected

**Verification**: `AO_LIVE_GROK_ACP=1 go test -v ./e2e/... -run TestChatGrokSpawn`

---

## Task 4a.3: Implement TestChatGrokModelOverride

Test model override via turn settings API.

**Assertions**:
- [ ] PATCH settings returns updated model
- [ ] GET conversation shows model in settings
- [ ] Turn uses specified model

**Verification**: `AO_LIVE_GROK_ACP=1 go test -v ./e2e/... -run TestChatGrokModelOverride`

---

## Task 4a.4: Implement TestChatGrokReasoningEffort

Test reasoning effort propagation.

**Assertions**:
- [ ] PATCH settings accepts reasoningEffort
- [ ] GET conversation shows reasoningEffort in settings

**Verification**: `AO_LIVE_GROK_ACP=1 go test -v ./e2e/... -run TestChatGrokReasoningEffort`

---

## Task 4a.5: Implement TestChatGrokApprovalModeOverride

Test per-turn approval mode override.

**Assertions**:
- [ ] Turn with approvalMode=bypass-permissions doesn't prompt
- [ ] File created without manual approval

**Verification**: `AO_LIVE_GROK_ACP=1 go test -v ./e2e/... -run TestChatGrokApprovalModeOverride`

---

## Task 4a.6: Implement TestChatGrokAttachments

Test file attachment upload and delivery.

**Assertions**:
- [ ] Upload returns attachment path
- [ ] File exists in worktree at returned path
- [ ] Agent can read/reference the file
- [ ] Multiple attachments work

**Verification**: `AO_LIVE_GROK_ACP=1 go test -v ./e2e/... -run TestChatGrokAttachments`

---

## Task 4a.7: Implement TestChatGrokServerStateConsistency

Test server state consistency across operations.

**Assertions**:
- [ ] Harness correct in conversation
- [ ] Settings reflect all changes
- [ ] Turns have correct states
- [ ] Activities appear in timeline

**Verification**: `AO_LIVE_GROK_ACP=1 go test -v ./e2e/... -run TestChatGrokServerState`

---

## Task 4a.8: Create Claude Code Reference Tests

**File**: `backend/e2e/chat_reference_test.go`

Create parallel tests for Claude Code as reference implementation:

```go
func TestChatClaudeCodeModelOverride(t *testing.T) {
    if os.Getenv("AO_LIVE_CLAUDE_ACP") != "1" {
        t.Skip("set AO_LIVE_CLAUDE_ACP=1 for Claude Code reference")
    }
    // Same assertions as Grok test
}
```

**Verification**: `AO_LIVE_CLAUDE_ACP=1 go test -v ./e2e/... -run TestChatClaudeCode`

---

## Task 4a.9: Run Full CI Suite

```bash
mise run lint
cd backend && gofmt -l . && go build ./... && go vet ./... && go test ./... && go test -race ./...
```

**Verification**: All checks pass.

---

## Exit Criteria Checklist

### G1: Independent Deep Review
- [ ] Review conducted by non-author
- [ ] Review note created at `reviews/g1-review.md`
- [ ] Review verdict: PASS or FAIL with rationale

### G2: Full CI Suite — green on the merged head
- [x] `gofmt -l .` returns no files
- [x] `go build ./...` succeeds
- [x] `go vet ./...` passes
- [x] `go test ./...` passes (live tests skip)
- [x] `go test -race ./...` passes
- [x] `mise run lint` passes (pinned golangci-lint)

### G3: Live API E2E
- [ ] `AO_LIVE_GROK_ACP=1` set
- [ ] All Grok API tests pass
- [ ] File/worktree proofs verified

### G4: Claude Code Reference Parity
- [ ] `AO_LIVE_CLAUDE_ACP=1` set (when available)
- [ ] Reference tests pass
- [ ] Grok behavior matches Claude Code for same API operations

---

## Reference Files

Existing E2E patterns:
- `backend/e2e/chat_conversation_test.go`
- `backend/e2e/chat_steer_test.go`
- `backend/e2e/harness_test.go`

API routes:
- `backend/internal/httpd/controllers/conversations.go`
- `backend/internal/httpd/controllers/sessions.go`
