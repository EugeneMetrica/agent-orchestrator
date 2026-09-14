# Proposal: P2 — Grok ACP Permission Mode Matrix

## Why

P1 proves the driver works with default permissions. P2 validates that all four
AO permission modes (default, accept-edits, auto, bypass-permissions) are
correctly mapped to Grok's ACP session modes and produce the expected approval
behavior.

## Problem Statement

Each permission mode changes agent behavior:
- **default**: Agent prompts for tool approvals
- **accept-edits**: Auto-approve file edits, prompt for others
- **auto**: Auto-approve all tools (Grok config-dependent)
- **bypass-permissions**: Skip all approval prompts via `--always-approve`

Without testing each mode against real Grok, permission mapping bugs could cause:
- Security issues (auto-approving when user expected prompts)
- UX issues (prompting when user wanted auto-approve)

## Proposed Solution

1. Extend `live_test.go` with `TestLiveGrokACPPermissionModes` that runs a
   file-writing task under each permission mode.
2. Verify each mode produces the expected file output (proof that the agent
   actually executed with that mode).
3. Document mode mapping in the driver's inline comments.

## Key Constraints

- **Live test only**: Permission behavior is provider-side; unit tests cannot
  verify it.
- **All four modes**: default, accept-edits, auto, bypass-permissions.
- **File proof per mode**: Each mode writes a unique proof file.

## Success Criteria

1. `TestLiveGrokACPPermissionModes` passes for all four modes.
2. Each mode produces its expected proof file.
3. No permission prompts block the test when mode should auto-approve.
4. Full CI suite passes.

## Non-Goals

- Resume/persistent-host (that is P3).
- Desktop E2E (that is P4).

## Gates (P2)

| Gate | Description | Waiver |
|------|-------------|--------|
| G1 | Independent deep review → review note PASS/FAIL in `reviews/` | Required |
| G2 | Full suite: gofmt, go build/vet, `go test -race ./...`, npm run lint / golangci-lint v2.12.2 | Required |
| G3 | Live permission matrix: all 4 modes create proof files | Required |
| G4 | AO frontend: verify mode selector affects session behavior | Required |

## Impact

- **Scope**: `backend/internal/adapters/chatdriver/grokacp/live_test.go` (extend).
- **Risk**: Low. Pattern mirrors `cursoracp/live_test.go` permission matrix.
- **Dependencies**: P1 complete.
