# G1 — Independent review: P2 Grok ACP permission mode matrix

- **Change**: `openspec/changes/grok-acp-p2-permissions`
- **PR**: [#5](https://github.com/EugeneMetrica/agent-orchestrator/pull/5) —
  branch `cursor/grok-acp-p2-permissions-81da`, head `382aff1a1`
  (`test(grokacp): add gated live permission mode matrix`)
- **Reviewer**: independent (non-author) agent
- **Verdict**: PASS_WITH_NITS
- **Recommendation**: MERGE

## Scope check

| Requirement | Result |
| --- | --- |
| `TestLiveGrokACPPermissionModes` gated by `AO_LIVE_GROK_ACP=1` | Yes — `live_test.go:76-79`, skips otherwise |
| All four AO modes covered | Yes — default, accept-edits, auto, bypass-permissions as table subtests |
| `Permissions=mode` at Start | Yes — `ChatStartConfig.Permissions: test.mode` |
| `Settings.Approval=mode` per turn | Yes — `ports.ChatTurnSettings{Approval: test.mode}` |
| Proof through the file-edit tool, not the shell | Prompt-level only (see nit 3) |
| Per-mode proof file `mode-<name>.txt` with content `ok` | Yes — read from the subtest's own `t.TempDir()` |
| `sendLiveTurnWithSettings` helper, `sendLiveTurn` delegates | Yes — delegates with empty settings |
| `driver.go` change is comment-only | Yes — the diff touches only the `sessionMode` doc comment |
| Existing `sessionMode` / `--always-approve` unit tests intact | Yes — `TestSessionModeUsesGrokPermissionModeIDs` and `TestConfigureSpawnsGrokACPStdio` unchanged and green |
| No P3/P4 scope creep | Yes — the P2 commit touches only `driver.go` and `live_test.go` |

The per-turn `Settings.Approval` is not decorative: the shared ACP conversation
turns a non-empty `Approval` into a `session/set_mode` call
(`acp/conversation.go:452-462`), and `SendTurn` propagates
`ErrACPSetterUnsupported` when the agent answers `-32601`
(`TestACPDriverSendTurnPropagatesMethodNotFound`). Start-time mode application
is tolerant of `-32601`, so the per-turn setting is the only place the live
matrix can catch Grok dropping `session/set_mode`.

## Verification performed

Clean worktree at `382aff1a1`, Go 1.27.1 and golangci-lint 2.13.2 (the
`mise.toml` pins), Linux x86_64.

| Check | Command | Result |
| --- | --- | --- |
| gofmt | `gofmt -l .` | clean |
| build | `go build ./...` | pass |
| vet | `go vet ./internal/adapters/chatdriver/grokacp/...` | pass |
| package tests | `go test -count=1 -v ./internal/adapters/chatdriver/grokacp/...` | pass; both live tests SKIP |
| package race | `go test -race -count=1 ./internal/adapters/chatdriver/grokacp/...` | pass |
| lint | `golangci-lint run --path-mode=abs` (v2.13.2) | 0 issues |
| backend suite | `go test -count=1 ./...` | pass except three pre-existing environment failures (below) |
| backend race suite | `GIT_CONFIG_GLOBAL=/dev/null go test -race -timeout=20m ./...` | pass except one pre-existing load-sensitive flake (below) |

### Pre-existing environment failures (not caused by this PR)

`internal/observe/scm`, `internal/service/importer`, and
`internal/service/project` fail on this review VM because its global git config
carries `url.https://x-access-token:***@github.com/.insteadOf`, so the
assertions that expect a clean `https://github.com/o/r.git` origin see the
rewritten URL. The same tests fail identically on `main` at `df41f4d44`, and
they pass once the rewrite is neutralised with `GIT_CONFIG_GLOBAL=/dev/null`.

The race suite added one more failure, `internal/httpd/TestServerShutdownEndpoint`
("graceful shutdown exceeded 5s"). It is load-sensitive, not related to this
change: it failed while the full race suite saturated the VM, and on an idle VM
it passes 5/5 under `-race` on the PR worktree while failing on `main` at
`df41f4d44` in the same conditions. The whole `internal/httpd` package is green
under `-race` on the PR worktree once the machine is idle.

CI's `build-test` job is the authority for all four.

### G3 (live matrix): SKIPPED, not verified

No `grok` binary on PATH and no `~/.grok` credentials or `XAI_API_KEY` on this
VM, so `AO_LIVE_GROK_ACP=1 go test -run PermissionModes` cannot be an
acceptance signal here. This is a gap, not a pass. G3 must be run on a box with
the user's Grok installation.

What was verified instead: with the gate set and no provider present, all four
subtests execute and fail loudly at `Start` (`grok: agent: binary not found on
PATH`) rather than silently passing, so the matrix cannot report success without
a real session. Reading the assertion, a missing proof file makes `os.ReadFile`
return an error and a wrong proof makes `strings.TrimSpace(content) != "ok"`
true; both reach the same `t.Fatalf`, so neither can be mistaken for a pass.

### G4 (frontend permission UI): out of scope for this review

Handled by the parent on the box with the real desktop app.

## Nits (none blocking)

1. **Spec/tasks naming drift.** `tasks.md` still shows `mode.txt` and the spec's
   bypass scenario asks for `mode-bypass.txt`, while the test writes
   `mode-bypass-permissions.txt`. The implementation's per-mode naming is the
   better behaviour; the OPSX text should be updated to `mode-<name>.txt` so the
   artifacts stop disagreeing.
2. **Spec requirement "Turn Settings Override" has no coverage.** The matrix
   sets the session mode and the turn mode to the same value, so it never
   exercises "session default, turn overrides, subsequent turns revert". The
   shared ACP transport also does not revert: `applyTurnSettings` assigns
   `c.permissionMode` and a later turn with an empty `Approval` leaves it in
   place. Either drop/soften that scenario in the P2 delta or move it to a phase
   that actually implements it.
3. **Auto-approving modes are not asserted to be prompt-free.** All four
   subtests call `waitForLiveTurn(..., approve=true)`, so an accept-edits, auto,
   or bypass session that still raised an approval request would be
   auto-answered and the test would pass. The proposal's success criterion 3
   ("no permission prompts block the test when mode should auto-approve") would
   be enforced by passing `approve=false` for auto and bypass, which turns any
   prompt into a failure. Kept as a nit because provider-side prompts outside
   the edit path could make a strict assertion flaky, and because the accepted
   `cursoracp` reference has the same looseness.
4. **File-edit vs shell is a request, not an assertion.** The prompt asks for
   the editing tool, but nothing checks the tool kind on the event stream, so a
   shell-written proof would pass. Matters most for accept-edits, the one mode
   whose semantics are edit-specific. Same limitation as the `cursoracp`
   reference.
5. **`sendLiveTurn` lost its own `t.Helper()`.** Failures inside the helper now
   report the delegation line instead of the caller's line. Identical to
   `cursoracp/live_test.go:178-180`, so leaving it keeps the two files in sync;
   only worth changing if both move together.
6. **Leaked host from an interrupted live run.** The ownership fingerprint
   includes the workspace path and each subtest uses a fresh `t.TempDir()`, so a
   still-alive host from an earlier interrupted run under the same
   `live-grok-mode-<name>` session id makes `Start` fail with "provider launch
   configuration changed". Operational note for whoever runs G3; the normal path
   is covered by `defer Terminate()`.

## Conclusion

Production behaviour is unchanged: the only non-test edit is a `sessionMode`
comment, and the mapping it documents (`""` / `acceptEdits` / `auto` /
`bypassPermissions`, plus launch-time `--always-approve` for bypass) matches
both the code and `TestSessionModeUsesGrokPermissionModeIDs`. The live matrix is
correctly gated, covers the four modes, proves work on disk per mode, and fails
loudly when the provider or the proof is missing. G2 passes locally on the
pinned toolchain; G3 is unverified here for lack of a Grok installation and G4 is
the parent's to prove. MERGE.
