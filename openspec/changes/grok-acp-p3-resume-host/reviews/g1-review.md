# G1 — Independent Deep Review: P3 Grok ACP Resume & Persistent Host

- **PR**: [#6](https://github.com/EugeneMetrica/agent-orchestrator/pull/6)
- **Branch / head**: `cursor/grok-acp-p3-resume-host-4357` @ `97bc1289` — _test(grokacp): cover terminate and resume against a live Grok session_
- **Reviewer**: independent (non-author) cloud agent
- **Verdict**: **PASS_WITH_NITS**
- **Recommendation**: **MERGE**

## Change under review

3 files, +93/-1:

| File | Nature |
| --- | --- |
| `backend/internal/adapters/chatdriver/grokacp/live_test.go` | +82, new gated `TestLiveGrokACPResume` |
| `backend/internal/adapters/chatdriver/grokacp/driver.go` | +7, comment only (SessionMeta decision) |
| `docs/STATUS.md` | +4/-1, resume note + live-test coverage note |

No production code path changes: the `driver.go` diff is entirely comment lines above `sessionMeta`, and no other package is touched. No P4 e2e scope creep.

## Spec coverage

Every requirement in `specs/chat-drivers/spec.md` maps to an assertion that fails loudly when unmet:

| Spec requirement | Assertion | Line |
| --- | --- | --- |
| Resume capability advertised | `conv.Capabilities()[ports.ChatCapabilityResume]` | live_test.go:100 |
| Provider conversation ID captured | non-empty `ProviderConversationID()` | live_test.go:96 |
| Resume restores session | `driver.Resume(...)` with stored id, `t.Fatalf` on error | live_test.go:122 |
| Files persist across resume | content `== "before-value"` before, unchanged after | live_test.go:110, 146 |
| History visible after resume | resumed answer contains `ALPHA` | live_test.go:136 |
| Standing instructions apply on resume | resumed answer contains `GROK_STANDING_RESUME` | live_test.go:139 |
| SessionMeta decision documented | `driver.go` comment (tasks.md 3.2 permits inline comments) | driver.go:57-62 |

Alignment with `tasks.md` / `design.md` is exact on the load-bearing details: terminate→resume on the same provider id, codeword `ALPHA`, `before.txt` = `before-value`, and distinct standing tokens across the two deliveries. The implementation is stricter than `design.md`, which used `t.Errorf`; the test uses `t.Fatalf` throughout.

## Would the assertions actually catch a regression?

The live test could not be executed here (no `grok` on PATH, no `XAI_API_KEY`, no `~/.grok/auth.json`), so the assertions were audited by reading the code and the transport they depend on.

**Missing `ALPHA` → caught, and cannot false-pass.** Two escape hatches were checked and both are closed:

1. `ALPHA` does not appear in the resume prompt, so a parroting model cannot satisfy it.
2. `session/load` transcript replay — which does contain `ALPHA` — never reaches `conv.Events()`. `conversation.emit` short-circuits through `captureHistoryEvent` into the history buffer (`acp/conversation.go:946`, `acp/history.go:139`), and `acp/driver_test.go:1678` asserts that history does not leak onto the live stream. This matters because `waitForLiveTurn` does *not* filter events with an empty `ProviderTurnID` (`live_test.go:244`), so a leak would have silently satisfied the assertion. It does not leak, and replayed turns additionally carry synthetic non-empty ids (`acp/history.go:250`).

**Missing `before.txt` → caught.** `os.ReadFile` error or content mismatch is fatal both before terminate (line 110) and after resume (line 146).

**Missing new standing token → caught, and cannot false-pass.** `GROK_STANDING_RESUME` exists only in the resume-time `SystemPrompt`; the recovered transcript can only supply `GROK_STANDING_START`. Using two distinct tokens is what makes this a real re-delivery proof rather than a stale-context proof.

## Is the SessionMeta decision correct?

The comment's central claim — that the shared transport re-sends `_meta` on resume, so no `nativeacp` or Grok-specific transport change is needed — is accurate for the path this test exercises. `acp.Driver.Resume` builds `launchCfg` with `SystemPrompt: cfg.SystemPrompt` (`acp/driver.go:284`) and passes the resulting `meta` to both branches: `session/load` (`acp/driver.go:361`) and `session/resume` (`acp/driver.go:376`). Which branch runs depends on Grok's runtime `AgentCapabilities.LoadSession`; both carry the metadata, so the decision holds either way.

`Terminate()` reaches the non-live branch as the test assumes: it calls `closeProvider(true)` → `persistenthost.Shutdown`, which stops the provider and removes the descriptor (`acp/conversation.go:875`, `persistenthost/host.go:484`). The next `Resume` therefore starts a fresh host with `Reconnected: false`, so `connect` returns `live == nil` and the metadata-bearing path runs.

The `_meta.yoloMode` half of the decision is sound: `sessionMeta` returns only `rules`, and bypass-permissions is already expressed by the `bypassPermissions` session mode plus launch-time `--always-approve`. Keeping one expression of approval-widening is the right call.

## Gates

| Gate | Result |
| --- | --- |
| G1 | This note — PASS_WITH_NITS |
| G2 | PASS locally: `gofmt -l .` clean, `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./internal/adapters/chatdriver/grokacp/`, `golangci-lint 2.13.2 --path-mode=abs` → **0 issues**. Go 1.27.1 matches the `mise.toml` pin. |
| G3 | **SKIP — not verifiable here.** No Grok CLI or credentials on this VM. Parent proves G3 on the box. |
| G4 | Out of this review's scope (parent, on the box). Not held against the PR. |

G2 caveat: the full `go test ./...` initially reported 4 failures in `internal/observe/scm`, `internal/service/importer`, and `internal/service/project`. All are artifacts of this VM's `~/.gitconfig`, which rewrites GitHub URLs to an `x-access-token` form via `url.<...>.insteadOf`, so tests asserting a clean `RepoOriginURL` see a token-prefixed remote. Re-running those three packages with `GIT_CONFIG_GLOBAL=/dev/null` passes. Unrelated to this PR — none of those packages are touched.

## Nits (none blocking)

1. **The test comment overstates which host lifecycle it reproduces.** It opens with "the daemon restarts", but `Terminate()` destroys the persistent host, so what the test exercises is the *cold* restore path: a brand-new host plus `session/load`/`session/resume`. A real daemon restart uses `Close()`, which deliberately only detaches and leaves the host alive; `Resume` then takes the `live != nil` reattach branch (`acp/driver.go:294-330`), which returns early and never calls `SessionMeta`. The cold path is the stronger choice for proving provider-side history recovery, so the test is right — the framing just claims a scenario it does not run, and it is worth noting that the warm reattach path is precisely what G4 still has to cover.

2. **`docs/STATUS.md` blurs provider-side recall with AO-side replay.** "Grok recovers the transcript" is true in the sense the test proves (the model remembers `ALPHA`), but AO's own typed transcript replay is gated on `loadSession`: `caps[ports.ChatCapabilityHistory] = init.AgentCapabilities.LoadSession` (`acp/driver.go:574`), and without it `ReadHistory` returns `ErrChatHistoryUnavailable` (`acp/history.go:156`). If Grok advertises only `session/resume`, the model keeps its context while the UI has no history to render. Distinguishing the two would make the note precise, and asserting `resumed.Capabilities()[ports.ChatCapabilityHistory]` in the live test would pin which branch Grok actually takes — cheap, and directly informative for G4.

3. **No `defer Terminate()` on the first conversation.** A `t.Fatalf` between `Start` (line 88) and `Terminate` (line 115) leaves the detached host running with its descriptor at `<dataDir>/chat-hosts/live-grok-resume/host.json`. There is no idle timeout in `persistenthost`, and because `SessionID` is fixed while `dataDir` defaults to the real `~/.ao`, the next run would attach to that survivor and fail with `ErrChatRecoveryInconclusive` — "fresh ACP start found an existing initialized session" (`acp/driver.go:206`) — a confusing follow-on failure that hides the original one. `Terminate` is `sync.Once`-guarded (`acp/process.go:111`), so an extra `defer` is safe. The same shape already exists in `cursoracp/live_test.go`, so this is a pre-existing pattern rather than a regression introduced here.

4. **`strings.Contains(resumeAnswer, "before.txt")` is trivially satisfiable**, since the prompt names the file and any acknowledgement echoes it. Harmless: the real proof is the `os.ReadFile` comparison two statements later, and `design.md` prescribes this assertion verbatim.

5. The change is titled "resume & **persistent host**", but the decision comment only addresses `nativeacp`/transport. Stating that the persistent host needs no Grok-specific wiring because it is inherited from `acpdriver.New` (`acp/driver.go:114`, `acp/driver.go:510`) would close the loop for a future reader.

## Conclusion

The test asserts the right things, the assertions are sound against the transport they depend on, the documented SessionMeta decision matches what the code actually does, and production behavior is unchanged. G2 is green with the pinned linter. G3 is unverifiable in this environment and is explicitly left to the parent.

**PASS_WITH_NITS — MERGE.** All five nits are comment, docs-wording, or test-hygiene items; none needs to block, and none touches shipped behavior.
