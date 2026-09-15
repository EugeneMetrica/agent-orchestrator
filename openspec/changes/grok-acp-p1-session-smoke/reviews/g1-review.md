# G1 Acceptance Review: P1 — Grok ACP Live Session Smoke

**PR**: https://github.com/EugeneMetrica/agent-orchestrator/pull/4
**Branch**: `cursor/grok-acp-p1-session-smoke-d368` (head `3b65d44`, base `df41f4d4`)
**Reviewer**: independent acceptance reviewer (not the author)
**Date**: 2026-09-15

## Verdict: PASS_WITH_NITS — Recommendation: MERGE

Verified change counts against the PR's actual base (`git diff --numstat df41f4d4...3b65d44`):

```text
Code changes: +0 -0
Tests: +210 -0
Others: +15 -3 (docs)
```

## Scope checks

| # | Item | Result |
|---|------|--------|
| 1 | `live_test.go` gated by `AO_LIVE_GROK_ACP=1`; Probe→Start→SendTurn; `GROK_STANDING_TOKEN`; `proof.txt` == `grok-acp-ok`; helpers mirror cursoracp | PASS |
| 2 | `testmain_test.go` present and matching the `acp` TestMain pattern | PASS |
| 3 | `docs/STATUS.md` documents Grok Chat; registered count honest | PASS |
| 4 | No P2–P4 scope creep; production driver unchanged | PASS |
| 5 | G2 suite: gofmt, vet, `go test` grokacp (live skipped), `-race` on grokacp + registry, pinned golangci-lint | PASS |
| 6 | G3 live run | SKIP (no Grok CLI or credentials on the review VM) |

### 1. Live test shape

`TestLiveGrokACP` skips unless `AO_LIVE_GROK_ACP=1`, builds the driver from the
real Grok agent plugin (`New(grok.New(), nil)`), probes, starts against a
`t.TempDir()` workspace with `SystemPrompt` carrying `GROK_STANDING_TOKEN`, sends
one turn, and asserts three independent facts: a non-empty
`ProviderConversationID()`, the standing token inside the accumulated
`ChatEventMessageDelta` text, and `os.ReadFile(<workspace>/proof.txt)` equal to
exactly `grok-acp-ok`. `waitForLiveTurn` fails the test unless
`TurnState == domain.TurnStateCompleted`, so a stalled or errored turn cannot be
read as success. The proof assertion fails on both a read error and a content
mismatch, so a run where Grok only describes the work does not pass.

`sendLiveTurn`, `waitForLiveTurn`, `liveEnvMap`, and `liveDataDir` mirror
`cursoracp/live_test.go`; the approval choice is factored into
`allowOnceDecision`, which prefers `allow_once` so a live run never widens the
user's standing Grok permissions.

### 2. TestMain is required, not incidental

`acp.Driver.connectProcess` always goes through `persistenthost.ConnectOrStart`,
which re-execs `os.Executable()` as `chat-host …`. In a test binary that is the
test binary itself, so the package needs the re-exec handler.

Verified empirically in a throwaway worktree of `main`: disabling the `chat-host`
branch of `acp/driver_test.go`'s `TestMain` and running
`TestPersistentACPDriverSurvivesRealProcessDetach/claude-code` fails with

```text
Start: chat driver unavailable: persistent ACP host: start chat host:
open …/chat-hosts/persistent-acp-e2e/host.json: no such file or directory
```

`grokacp/testmain_test.go` reproduces `acp/driver_test.go`'s handler logic
exactly (same argv arity check, same `ProtocolACP` fingerprint/separator
handling, same `persistenthost.Run` config, same exit codes), so the live test
can actually reach `SendTurn`.

### 3. Documentation honesty

`registry.Build` ships ten Chat drivers: Codex on its native app-server plus nine
ACP bindings (claudeacp, cursoracp, opencodeacp, droidacp, kimiacp, kimchiacp,
piacp, ompacp, grokacp). "All nine registered ACP Chat providers" matches.

The coverage caveat is also accurate:
`acp.TestPersistentACPDriverSurvivesRealProcessDetach` enumerates eight
identities (Claude Code, Cursor, OpenCode, Droid, Kimi, Kimchi, Pi, OMP) and does
not include Grok, which is what "eight of the nine ACP identities (Grok's
detached-host coverage is still pending)" claims.

### 4. No scope creep

The diff is two test files in `grokacp` plus `docs/STATUS.md`; `grokacp/driver.go`
and every other production file are untouched. The live test asserts the
advertised `ChatCapabilityResume` flag but never calls `Resume`, and it does not
walk the permission-mode matrix, so P2/P3/P4 stay out.

### 5. G2 evidence (this VM, Go 1.27.1, golangci-lint 2.13.2 per `mise.toml`)

- `gofmt -l internal/adapters/chatdriver/grokacp/` — clean
- `go vet ./internal/adapters/chatdriver/grokacp/... ./internal/adapters/chatdriver/registry/...` — clean
- `go test -count=1 ./internal/adapters/chatdriver/grokacp/...` — 18 tests pass, `TestLiveGrokACP` SKIP with the documented gate message
- `go test -race -count=1 ./internal/adapters/chatdriver/grokacp/... ./internal/adapters/chatdriver/registry/...` — pass
- `golangci-lint run --path-mode=abs` (full backend, pinned 2.13.2) — 0 issues
- `go test ./...` and `go test -race -timeout=20m ./...` (full backend) — 167 packages ok; the only failures are pre-existing and environmental, see below

Pre-existing failures on this VM, each reproduced unchanged on `origin/main`
(`df41f4d4`) in a separate worktree: `observe/scm`
(`TestPoll_DiscoversCrossForkPRFromUpstreamRemote`,
`TestDiscoverSubjects_BackfillsRepoOriginURL`), `service/importer`
(`TestPrepareGitGitHubRepositoryRepairsExistingOriginWithoutURL`), and, under
`-race`, `service/project` (`TestManager_AddPopulatesRepoOriginURL`). All four
assert bare `https://github.com/...` origins and observe
`https://x-access-token:***@github.com/...` because the review VM sets a global
`url.<token>@github.com/.insteadOf` rewrite. Unrelated to this PR.

`windows-workspace` cannot run here (no Windows runner); it is green on the PR's
remote checks.

### 6. G3 live run: SKIP, not FAIL

No `grok` on `PATH`, no `~/.grok`, and no `XAI_API_KEY` on this VM, so the gated
run was not attempted. Per the phase instructions this is a SKIP: the parent
already demonstrated G3 on a machine with Grok, and the commit that carries the
`TestMain` handler is what makes that run reachable. The assertions above were
read line by line to confirm the test would fail on a missing or wrong
`proof.txt` rather than pass silently.

## Nits (none blocking)

1. **PR body change counts are stale.** The description says `Tests: +168 -0`,
   which was correct before `3b65d44` added the 42-line `testmain_test.go`; the
   net diff is `Tests: +210 -0`. `.agents/skills/pr-description/SKILL.md` requires
   recalculating after the final push. Description-only fix.
2. **`docs/research/persistent-acp-chat-hosts.md:29`** still says "All eight ACP
   bindings use this path" — with Grok registered it is nine. Line 71 ("all eight
   harness identities") is about test coverage and stays correct. STATUS.md links
   this note as the coverage tracker, so the two now disagree.
3. **`#### Grok (xAI Grok Build)` is the only `####` heading in `docs/STATUS.md`.**
   Every other provider's specifics are prose bullets under
   `### Backend (Go daemon)`, so the new level reads as privileging Grok.
   Cosmetic.
4. **`waitForLiveTurn` ignores `ChatEventInputRequested`.** If Grok raises a
   structured input request the loop blocks until the five-minute context
   deadline instead of failing fast with a clear message. cursoracp separates
   that wait; worth copying when P2/P3 extend this test.
5. **The deferred `Terminate()` error is dropped** while `liveDataDir` defaults to
   the real `~/.ao`. A failed shutdown leaves a detached host under
   `~/.ao/chat-hosts/live-grok-acp` that a later session with that id could
   adopt. cursoracp has the same shape, so this is a shared follow-up rather than
   a regression.

G4 (frontend/CLI UI verification) is out of this review's scope and is not
counted against the verdict.
