# Proposal: P1 — Grok ACP Session Smoke Test

## Why

P0 validates the driver compiles and passes unit tests, but does not prove the
driver can actually communicate with a real Grok CLI over ACP. P1 adds a live
smoke test that exercises the full Probe→Start→SendTurn cycle with file-based
proof that the agent actually executed work.

## Problem Statement

Unit tests cannot catch ACP protocol mismatches, spawn command errors, or
authentication issues that only manifest when talking to the real provider.
Without a live smoke test gated behind `AO_LIVE_GROK_ACP=1`, the driver could
pass all unit tests yet fail immediately when a user tries Chat mode.

## Proposed Solution

1. Create `backend/internal/adapters/chatdriver/grokacp/live_test.go` with tests
   gated behind `AO_LIVE_GROK_ACP=1`.
2. Implement `TestLiveGrokACP` that:
   - Probes binary/auth availability
   - Starts a conversation with a standing instruction token
   - Sends a turn asking Grok to write `proof.txt` via shell
   - Verifies the file exists on disk with expected content
3. Update `docs/STATUS.md` to document Grok Chat driver availability.
4. Add harness documentation if other harnesses have it.

## Key Constraints

- **File-based proof**: Must verify actual file creation on disk, not just
  process spawn or API response. Pattern follows `cursoracp/live_test.go`.
- **Gate environment variable**: `AO_LIVE_GROK_ACP=1` required; CI never runs
  these tests automatically.
- **Uses real credentials**: Test consumes actual Grok API usage from user's
  account.

## Success Criteria

1. `AO_LIVE_GROK_ACP=1 go test -v ./internal/adapters/chatdriver/grokacp/...`
   passes with proof.txt verification.
2. Probe, Start, SendTurn all succeed against real Grok CLI.
3. Standing instruction token appears in agent response.
4. Full CI suite passes (live tests skipped in normal CI).

## Non-Goals

- Permission mode matrix (that is P2).
- Resume/persistent-host (that is P3).
- Desktop E2E (that is P4).

## Gates (P1)

| Gate | Description | Waiver |
|------|-------------|--------|
| G1 | Independent deep review → review note PASS/FAIL in `reviews/` | Required |
| G2 | Full suite: gofmt, go build/vet, `go test -race ./...`, mise run lint / golangci-lint v2.13.2 (pinned in mise.toml) | Required |
| G3 | Live mini-task: `AO_LIVE_GROK_ACP=1` test with **proof.txt on disk** | Required |
| G4 | AO frontend/CLI UI: session harness=grok, timeline shows turn | Required |

## Impact

- **Scope**: `backend/internal/adapters/chatdriver/grokacp/live_test.go` (new),
  `docs/STATUS.md` (update).
- **Risk**: Low. Pattern mirrors `cursoracp/live_test.go`.
- **Dependencies**: Grok CLI installed, valid XAI credentials.
