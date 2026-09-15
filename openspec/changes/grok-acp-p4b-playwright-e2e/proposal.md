# Proposal: P4b — Playwright UI E2E with Claude Code Reference

## Why

P4a validates the API contract programmatically. P4b validates the full user
experience: drive AO desktop/web UI like a real user, verify that UI actions
produce expected engine and worktree outcomes, and confirm Grok behavior matches
Claude Code as the reference implementation.

## Problem Statement

API tests cannot catch:
1. UI elements that don't trigger correct API calls
2. Model selector that appears to work but doesn't apply changes
3. Attachment uploads that fail silently in the UI
4. Timeline rendering issues that hide tool activities
5. Divergence between Grok and Claude Code UI behavior

## Proposed Solution

Create Playwright E2E tests that:

1. **Pick harness from UI**: Select Grok in new task dialog
2. **Switch models from UI**: Use model dropdown, verify change applied
3. **Set reasoning effort**: Use effort selector, verify propagation
4. **Upload files via UI**: Drag/drop or file picker, verify in worktree
5. **Run small coding task**: Send task, verify timeline + on-disk files
6. **Claude Code reference**: Run same suite with Claude Code via ai.metrica.pro
   router (model alias substitution) as reference implementation

## Key Constraints

- **Real browser**: Playwright drives actual AO desktop/web UI
- **Live providers**: `AO_LIVE_GROK_ACP=1` for Grok, `AO_LIVE_CLAUDE_ACP=1` for
  Claude Code reference
- **Claude via router**: Claude Code tests use ai.metrica.pro as custom router
  for model alias substitution
- **Harness matrix**: Same test suite runs against both harnesses

## Success Criteria

1. Playwright tests select Grok harness successfully
2. Model switch from UI is reflected in engine session
3. File attachments appear in worktree
4. Timeline shows tool activities correctly
5. Same tests pass for Claude Code reference
6. Grok path matches Claude Code reference behavior

## Non-Goals

- Testing ai.metrica.pro router itself (treated as given)
- Performance benchmarking

## Gates (P4b)

| Gate | Description | Waiver |
|------|-------------|--------|
| G1 | Independent deep review → review note PASS/FAIL in `reviews/` | Required |
| G2 | Full suite: gofmt, go build/vet, `go test -race ./...`, mise run lint / golangci-lint v2.13.2 (pinned in mise.toml) | Required |
| G3 | Live Playwright: `AO_LIVE_GROK_ACP=1` UI tests with file proof | Required |
| G4 | Claude Code reference: same suite via ai.metrica.pro router | Required |

## Impact

- **Scope**: `frontend/e2e/chat-grok-e2e.spec.ts` (new),
  `frontend/e2e/chat-reference-e2e.spec.ts` (new)
- **Risk**: Medium. Requires coordinating daemon, UI, and live providers.
- **Dependencies**: P4a complete, Playwright infrastructure, live credentials.
