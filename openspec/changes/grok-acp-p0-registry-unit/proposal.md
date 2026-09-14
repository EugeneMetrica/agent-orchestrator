# Proposal: P0 — Grok ACP Registry & Unit Tests

## Why

Before any live integration work, the Grok ACP Chat driver must be registered in
the daemon's Chat driver registry and pass unit tests that validate its binding
configuration. This phase gates all subsequent phases by ensuring the driver
compiles, integrates with `nativeacp`, and correctly constructs spawn commands.

## Problem Statement

`registry.SupportsChat(domain.HarnessGrok)` returns `false` because no Grok Chat
driver exists. Without a registered driver, Chat-mode sessions cannot be spawned
for the `grok` harness, and AO cannot offer Chat mode to users who prefer it over
TUI for Grok Build.

## Proposed Solution

1. Create `backend/internal/adapters/chatdriver/grokacp/` package with a thin
   `nativeacp` binding.
2. Implement `configure()` to build `grok --no-auto-update agent
   [--always-approve] [--model M] stdio` spawn command.
3. Forward the model id verbatim (bare ids such as `grok-code-fast` included);
   availability stays the ACP session's advertised model catalog.
4. Register `grokacp.New(grok.New(), log)` in `registry.Build()`.
5. Write unit tests covering:
   - Harness identity
   - Spawn command construction for each permission mode
   - Model forwarding (bare id, qualified id, empty)
   - Session mode mapping

## Key Constraints

- **No credential injection**: reuse existing `grok.Plugin` auth probe.
- **Unit tests only**: do not duplicate shared `acp`/`nativeacp`/`persistenthost`
  tests; test only the Grok-specific binding logic.
- **No live calls**: P0 tests must pass without Grok CLI installed.

## Success Criteria

1. `registry.SupportsChat(domain.HarnessGrok)` returns `true`.
2. `TestShippedChatDrivers` includes `domain.HarnessGrok`.
3. `driver_test.go` unit tests pass for configure, sessionMode, sessionOptions.
4. Full CI suite passes: `gofmt`, `go build`, `go vet`, `go test -race ./...`,
   `mise run lint` (golangci-lint v2.13.2, pinned in mise.toml).

## Non-Goals

- Live Probe/Start/SendTurn (that is P1).
- Permission mode live verification (that is P2).
- Resume/persistent-host (that is P3).
- Desktop E2E (that is P4).

## Gates (P0)

| Gate | Description | Waiver |
|------|-------------|--------|
| G1 | Independent deep review → review note PASS/FAIL in `reviews/` | Required |
| G2 | Full suite: gofmt, go build/vet, `go test -race ./...`, mise run lint / golangci-lint v2.13.2 (pinned in mise.toml) | Required |
| G3 | Live mini-task with real Grok | **WAIVED** (unit tests only) |
| G4 | AO frontend/CLI UI verification | **WAIVED** (unit tests only) |

## Impact

- **Scope**: `backend/internal/adapters/chatdriver/grokacp/` (new),
  `backend/internal/adapters/chatdriver/registry/` (registration + test update).
- **Risk**: Low. Pattern mirrors existing `opencodeacp`, `ompacp`, `kimchiacp`.
- **Dependencies**: Existing `grok.Plugin`, `nativeacp` package.
