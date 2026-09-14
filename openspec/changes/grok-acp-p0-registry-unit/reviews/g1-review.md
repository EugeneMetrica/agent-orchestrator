# G1 — Independent acceptance review (P0: grokacp Chat driver + Go 1.27.1)

- **Subject**: [PR #3](https://github.com/EugeneMetrica/agent-orchestrator/pull/3) `P0: grokacp Chat driver + CI Go 1.27.1`
- **Head reviewed**: `9d07a80f58f6205892bddd160fab3574d40d30a6` (`cursor/grokacp-chat-driver-p0-9bc3`, after the S1 fixes)
- **Base**: `main` @ `cc3a3c0c9`
- **Reviewer role**: independent, non-author
- **Verdict**: **PASS_WITH_NITS** on the P0 acceptance criteria
- **Recommendation**: **REQUEST_CHANGES** — limited to the developer-environment
  and documentation regressions the toolchain switch introduced (findings 3–5);
  the driver itself needs no rework.

## Acceptance criteria

| # | Criterion | Result | Evidence |
|---|-----------|--------|----------|
| 1 | Thin `grokacp` via `nativeacp` + `agent/grok`; spawns `grok --no-auto-update agent […] stdio`; no credential / `GROK_HOME` injection | PASS | `driver.go` is 70 lines of `nativeacp.Config`; `configure` returns a nil env overlay, and `agent/grok` implements no `AugmentRuntimeEnv`, so the launch inherits the session env unchanged. `auth.go` only *reads* `GROK_HOME` for probing. Arg order matches upstream ("agent options go after `agent` and before the mode name"): `--no-auto-update agent [--always-approve] [--model M] stdio`. |
| 2 | Bare model ids only; no provider/model format gate; availability from Grok discovery | PASS | No `ValidateTurnSettings`; `sessionOptions` forwards the id verbatim. `modelcatalog` discovers models with `grok models` + `parseGrokModels`, and `grok` is `CustomModelEntryDirect` — no AO allowlist. Availability is enforced by the shared transport, which rejects ids the ACP session does not advertise (`conversation.go` → `ErrChatConfigOptionInvalid`). |
| 3 | Registry registers `HarnessGrok`; `TestShippedChatDrivers` includes it | PASS | `registry.go` adds `grokacp.New(grok.New(), log)`; `registry_test.go` adds `domain.HarnessGrok`. Chat capability is fully registry-derived (`settings`/`chat` services, `session_manager`), so no second list needed. |
| 4 | Go 1.27.1; `mise.toml` pins go + golangci-lint; CI lint reads the mise pin; local gate `mise run lint`; `npm run lint` retired; no `go run` golangci; lint actually enforces | PASS | `1.27.1` in `backend/go.mod`, `cloud/go.mod`, `go.work`, `mise.toml`; `golang:1.27-bookworm` in `test/cli/Dockerfile` (tag exists). Verified locally with a real mise install: both pins resolve (`go 1.27.1`, `golangci-lint 2.13.2`, built with go1.27.0), `mise run lint` → **0 issues**, `npm run lint` → **exit 1**, no `go run …golangci-lint` anywhere. Enforcement probe: adding one undocumented exported function made `golangci-lint run` exit 1 (`revive`), so the gate is live, and `backend/.golangci.yml` is untouched. |
| 5 | No P1–P4 scope creep; shared `acp`/`persistenthost` untouched | PASS | Code diff is only `grokacp/` (new) + 2 registry lines. `acp/`, `nativeacp/`, `persistenthost/` unchanged. P1–P4 edits are doc-only (lint-command text, arg-order fix). |
| 6 | `go test -race ./internal/adapters/chatdriver/grokacp/ ./internal/adapters/chatdriver/registry/` | PASS | Both `ok`. |

## Additional verification run

- `go build ./...`, `go vet ./...`, `gofmt -l .` — clean (backend), `go vet` + `go test ./...` clean (cloud, under 1.27.1).
- Full `cd backend && go test ./...`: 4 failures, all `RepoOriginURL`-shaped
  (`observe/scm`, `service/importer`, `service/project`). All reproduce
  identically on `origin/main` — the review VM rewrites GitHub remotes to a
  token URL, so these are environment artifacts, not PR regressions.
- `TestServerShutdownEndpoint` did not fail in this run.
- Full `go test -race ./...` was not run to completion locally; CI's race job must confirm it.

## Findings

1. **Medium — AO's standing instructions are silently dropped in Grok Chat, and
   the design states a mechanism that does not exist.** `configure` ignores
   `cfg.SystemPrompt`. `design.md:209` says "`--rules` not used (system prompt
   via ACP `session/new` metadata)", but `nativeacp.Config` exposes no
   `SessionMeta` hook, so a native ACP binding cannot put anything in
   `session/new` metadata today (`acpdriver.Config.SessionMeta` exists but is
   unreachable from `nativeacp`). Every other Chat driver forwards the system
   prompt (`--append-system-prompt` for droid/kimchi/omp, a rules file for
   cursor/pi/kimi, config content for opencode, `developerInstructions` for
   codex). `TestConfigureAddsNoEnvironmentOverlay` even passes a
   `SystemPrompt` and asserts nothing about it. The only tracking is a
   conditional line in P3 tasks ("If Grok requires `_meta.rules` …").
   *Action:* correct `design.md:209` now, and record the deferral explicitly in
   P0/P1 rather than leaving it implicit.

2. **Medium — the ACP permission-mode mapping is asserted, not evidenced.**
   `sessionMode` returns `acceptEdits` / `auto` / `bypassPermissions` and the
   comment claims these are "Grok's own mode ids, … the same strings the TUI
   adapter passes to `--permission-mode`". Upstream Grok agent-mode docs list
   only `-m/--model`, `--always-approve` (alias `--yolo`), `--reauth`,
   `--agent-profile`, `--leader` for `grok agent`, and give `_meta.yoloMode` on
   `session/new` as the ACP approval knob; ACP session modes are not documented.
   `grokacp` is the only native ACP binding besides `claudeacp` that sets
   `SessionMode` — the other seven fix approval at launch and guard mid-session
   changes with `ValidateTurnSettings` (see `ompacp`). Because `accept-edits` and
   `auto` add no launch flag, if Grok's ACP has no `session/set_mode` those modes
   either no-op or fail the turn with `ErrACPSetterUnsupported`. Unit tests
   cannot detect either outcome. *Action:* close this in P2's live matrix; add a
   `ValidateTurnSettings` guard if `session/set_mode` turns out to be absent.

3. **Medium — `nix develop` no longer provides Go.** `flake.nix` drops
   `pkgs.go_1_25` and the `GOROOT` export but adds neither mise nor any Go, so
   the dev shell now ships `gotools` with no toolchain behind it. The new comment
   ("the dev shell and CI both resolve the version declared in go.mod") does not
   hold: the shell resolves nothing. `docs/development.md` still advertises
   "`nix develop` drops you into a shell with all deps". *Action:* add
   `pkgs.mise` to `buildInputs` (and `mise install` guidance), or keep a nixpkgs
   Go.

4. **Medium — `.envrc` adopts a mechanism mise has deprecated.** `use mise` is
   documented upstream as "deprecated and no longer supported"; it requires a
   manual `mise direnv activate > ~/.config/direnv/lib/use_mise.sh`, and mise's
   own guidance is not to let direnv and mise both manage `PATH`. Contributors
   who have direnv but not that generated function get an `.envrc` error, and no
   doc in this PR mentions the prerequisite. *Action:* drop the line and document
   `mise activate` (or `mise install` + `mise exec`).

5. **Low — `docs/stack.md:23` is now false.** It still reads
   "Backend language | Go 1.25.7 | … Matches `backend/go.mod`" after the bump.
   `docs/development.md` was updated; this row was missed.

6. **Low — the CI lint pin is resolved by an unvalidated `sed`.**
   `sed -n 's/^golangci-lint = "\(.*\)"$/\1/p' mise.toml` yields an empty string
   if the pin is ever reformatted (single quotes, trailing comment), and the job
   then passes `version: v` to the action. *Action:* assert a non-empty match, or
   let mise itself read the pin (`jdx/mise-action`) so CI and the local gate share
   the reader, not just the version string.

7. **Low — `--no-auto-update` placement is unverified against a real binary.**
   It is not in the upstream `grok agent` flag table; it is a root flag the TUI
   adapter already uses, and placing it before the subcommand is the right guess
   (and the S1 fix), but only P1's live smoke can prove `grok --no-auto-update
   agent stdio` parses.

8. **Nit — openspec test tables drift from the code.** `design.md` / `tasks.md`
   still list `TestConfigure_DefaultPermissions`, `TestSessionMode_*`,
   `TestHarness`; the implementation uses table-driven equivalents
   (`TestConfigureSpawnsGrokACPStdio`, `TestSessionModeUsesGrokPermissionModeIDs`,
   …) and folds the harness assertion into `TestDriverReusesGrokPluginForProbe`.
   Coverage is equal or better; only the names drift. Separately,
   `TestBareModelNameReachesGrokLaunch` asserts only that binary resolution was
   reached, so it proves "no gate before launch" and nothing more — the table
   test is what actually pins the args.

9. **Nit — `docs/STATUS.md` does not list Grok among shipped Chat drivers**
   although the driver is registered and therefore user-reachable the moment this
   merges. The update is P1 Task 1.3, so this is a deliberate deferral, but until
   then STATUS.md under-reports what `main` ships.

## Bottom line

The P0 deliverable is correct: the binding is genuinely thin, model handling is
provider-owned end to end, registration is complete, and the lint gate is real
(verified by making it fail). The change requests are confined to the toolchain
switch's collateral — a nix dev shell with no Go, a deprecated direnv hook, and
two stale/incorrect doc claims — all small edits that need no re-review of the
driver. Findings 1 and 2 are the substantive product risks and must not be lost
in the P1/P2 gates.
