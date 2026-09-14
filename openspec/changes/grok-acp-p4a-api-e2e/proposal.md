# Proposal: P4a — API Frontend E2E (Backend HTTP Routes)

## Why

P0-P3 prove the Chat driver works in isolation. P4a validates the full
frontend↔engine contract through AO's HTTP API: spawn session, set model,
configure reasoning effort, send turns with attachments, and verify server
state + worktree. This ensures UI-triggered operations actually reach the
engine and produce expected results.

## Problem Statement

The driver passing unit and live tests does not prove:
1. AO HTTP routes correctly dispatch to the Grok driver
2. Model switches from API are applied to the engine session
3. Reasoning effort / context settings propagate through the stack
4. File attachments uploaded via API are delivered to the worktree
5. Turn settings override (model, approval mode) works end-to-end

Without API E2E tests, bugs in the HTTP→service→driver chain would not surface
until users report broken UI behavior.

## Proposed Solution

Create `backend/e2e/chat_grok_test.go` with tests covering:

1. **Spawn with harness=grok**: Verify session creates successfully
2. **Model override via turn settings**: Set model, verify in conversation state
3. **Reasoning effort / context settings**: Set via API, verify propagation
4. **Send turn with attachments**: Upload file, reference in message, verify
   worktree contains attachment
5. **Server state assertions**: Conversation controller state, turn state,
   activity timeline matches expectations

## Key Constraints

- **Real daemon**: Tests run against actual daemon with Grok driver registered
- **Gated**: `AO_LIVE_GROK_ACP=1` required for tests involving real Grok
- **Claude Code reference**: Some tests should run with Claude Code to establish
  reference behavior for parity comparison

## Success Criteria

1. `TestChatGrokSpawn` passes with correct harness assignment
2. `TestChatGrokModelOverride` verifies model applied to engine
3. `TestChatGrokTurnSettings` validates reasoning effort propagation
4. `TestChatGrokAttachments` confirms file delivery to worktree
5. Full CI suite passes

## Non-Goals

- Browser/UI automation (that is P4b).

## Gates (P4a)

| Gate | Description | Waiver |
|------|-------------|--------|
| G1 | Independent deep review → review note PASS/FAIL in `reviews/` | Required |
| G2 | Full suite: gofmt, go build/vet, `go test -race ./...`, npm run lint / golangci-lint v2.12.2 | Required |
| G3 | Live API E2E: `AO_LIVE_GROK_ACP=1` tests with file/worktree proof | Required |
| G4 | Reference comparison: same tests pass for Claude Code (when available) | Required |

## Impact

- **Scope**: `backend/e2e/chat_grok_test.go` (new)
- **Risk**: Medium. Requires coordinating HTTP routes, service layer, and driver.
- **Dependencies**: P3 complete, daemon running with Grok registered.
