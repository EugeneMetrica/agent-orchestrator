# G1 re-acceptance — PR #3, head `19d6966`

**Verdict: FAIL**
**Recommendation: REQUEST_CHANGES** — one blocker, narrowly scoped and cheap to fix.
Reviewed as a non-author, after the unblock, against the P0 acceptance criteria.

Everything the earlier round asked for landed and holds: the driver is a genuinely
thin `nativeacp` + `agent/grok` binding, the model gate is gone, the registry
change is complete, the toolchain switch is coherent, and the docs no longer
claim a SessionMeta channel. The blocker is new and specific: the `--rules`
delivery this gate was reopened for does not reach the agent process, so AO's
standing instructions are still dropped — now while the spec asserts they are
delivered.

## Criteria

| # | Criterion | Result |
|---|-----------|--------|
| 1 | Thin `grokacp` via `nativeacp` + `agent/grok`; `grok --no-auto-update [--rules …] agent […] stdio`; no credential / `GROK_HOME` injection | PASS |
| 2 | Non-empty `cfg.SystemPrompt` → `--rules` before `agent`, append semantics; empty → no flag; unit tests prove it | **FAIL** (argv is correct and tested; the flag is inert in ACP mode — finding 1) |
| 3 | Bare model ids; no provider/model gate; availability from Grok discovery | PASS |
| 4 | Registry registers `HarnessGrok`; `TestShippedChatDrivers` covers it | PASS |
| 5 | mise Go 1.27.1 + golangci pin; CI lint reads the mise pin; `mise run lint`; npm lint retired; flake/nix/doc honesty | PASS |
| 6 | `go test -race` on `grokacp` + `registry` | PASS |

## Finding 1 — S1 blocker: `grok agent stdio` never reads `--rules`

`configure` puts `--rules <prompt>` ahead of the `agent` subcommand, which is the
only position that parses — but the value is discarded by Grok before agent mode
starts. Verified against upstream `xai-org/grok-build` at `3794978`:

- `crates/codegen/xai-grok-pager/src/app/cli.rs:523-525` — `rules` is an ordinary
  field of the root `PagerArgs` struct (`416-771`). It is **not** `global = true`,
  unlike `--leader-socket`, `--debug`, `--debug-file` in the same struct.
- `crates/codegen/xai-grok-pager-bin/src/main.rs:2168-2189` — the agent dispatch is
  `Command::Agent(agent_args) => run_agent_command(agent_args, args.permission_mode_flag,
  args.trust, args.no_auto_update, args.disable_web_search, &update_config)`. The
  signature (`1154-1161`) has no rules parameter. `--no-auto-update` is forwarded;
  `--rules` is not, so the AO argv analogy between the two flags does not hold.
- `args.rules` has exactly two consumers, neither of them agent mode: the TUI
  connect path (`xai-grok-pager/src/app/mod.rs:900`) and the headless `-p` path
  (`xai-grok-pager-bin/src/main.rs:2399`).
- `AgentArgs` (`cli.rs:253-304`) has no `rules` field either, and the `HeadlessArgs`
  it flattens (`cli.rs:342-347`) is only WebSocket URL overrides — so the flag
  cannot be moved after `agent` instead.
- Agent-side, the only appended-rules channel is ACP `_meta`:
  `crates/codegen/xai-grok-shell/src/agent/mvp_agent/mod.rs:982-1006`
  (`build_spawn_system_prompt`) folds `_meta.rules` — session meta first, then
  initialize meta — into `<human_rules>` at session creation. Grok's own TUI is an
  ACP client and performs exactly that translation from its `--rules` flag:
  `xai-grok-pager/src/acp/mod.rs:415-431` (`build_initialize_meta`).
- Upstream docs agree: the agent-mode flag table lists only `-m/--model`,
  `--always-approve`, `--reauth`, `--agent-profile`, `--leader`, while "Session
  `_meta` options" documents `rules` as "Extra rules appended to the system prompt".

Parse behaviour, checked with a clap 4.5.20 model of those structs rather than
assumed:

```
grok --no-auto-update --rules "ao rules" agent stdio   → OK, root rules = Some("ao rules")
grok --no-auto-update agent --rules "ao rules" stdio   → error: unexpected argument '--rules' found
```

So the launch still succeeds — this is a silent drop, not a spawn failure, which
is why green unit tests and green CI cannot see it. Impact: every Grok chat
session runs without AO's standing instructions. There is no second carrier:
`ports.ChatStartConfig` has `SystemPrompt` and no `SystemPromptFile` (unlike the
TUI `ports.LaunchConfig`), and `grokacp` writes no workspace rules file the way
`cursoracp` and `kimiacp` do.

Secondary effect — the honesty regression this round was supposed to close:
`specs/chat-drivers/spec.md` now states as a requirement that the driver "SHALL
deliver AO's standing instructions with the global `--rules` flag … so they add
to … the system prompt", and `design.md` says `--rules` "carries AO's standing
instructions in both modes". Both describe behaviour the provider does not
implement. That is the same class of claim as the removed `session/new` metadata
sentence, pointed at a different mechanism.

**Fix, and it is small.** `acpdriver.Config.SessionMeta` already exists; the gap
is only that `nativeacp.Config` does not expose it. Add the pass-through field,
have `grokacp` return `{"rules": <prompt>}` when `cfg.SystemPrompt` is non-empty,
and drop the argv flag once the meta path carries it. That is unit-testable in
the same table style as the current argv cases, needs no live Grok, and keeps the
binding thin. If P0 is not the place for a `nativeacp` field, the alternative is
to delete the `--rules` argv and the delivery claims from the spec, design, and
PR body and defer delivery to P1 — honest, but it leaves this gate's requirement
unmet.

## What passes, with the evidence

- **Thin binding, no injection.** `New` is `nativeacp.New(plugin, Config{Harness,
  Configure, SessionMode, SessionOptions})` with the plugin coming from
  `agent/grok`; argv is `grok --no-auto-update [--rules …] agent [--always-approve]
  [--model …] stdio`. `configure` returns a nil env overlay, `agent/grok`
  implements no `AugmentRuntimeEnv`, and the only `GROK_HOME` reference in the
  Grok adapter is a read in `auth.go` for probing. `--always-approve` and its
  position after `agent` match the upstream agent-mode flag table.
- **Argv construction and the `--rules` cases are properly tested.** Both
  mutations fail loudly: deleting the `--rules` append breaks three cases, and
  moving it after `agent` breaks the same three including the explicit ordering
  assertion. The empty/whitespace prompt case is covered.
- **No model gate.** No `ValidateTurnSettings`, no AO model list; ids are
  forwarded verbatim, `modelcatalog` discovers them via `grok models`
  (`CustomModelEntryDirect`), and the shared transport still rejects ids the ACP
  session does not advertise.
- **Registry.** `grokacp.New(grok.New(), log)` is in `registry.Build`, and
  `TestShippedChatDrivers` asserts `domain.HarnessGrok`.
- **Toolchain.** With a real mise install: `go 1.27.1` and
  `golangci-lint 2.13.2 (built with go1.27.0)` both resolve; `mise run lint` →
  `0 issues`; `npm run lint` exits 1 with the retirement message; the CI pin
  extraction (`sed -n 's/^golangci-lint = "\(.*\)"$/\1/p' mise.toml`) yields
  `v2.13.2`, and the CI log confirms the lint job ran `version: v2.13.2` under
  `GOVERSION=go1.27.1`. `flake.nix` no longer pins a nixpkgs Go and fails loudly
  if `mise install` fails. Residual `npm run lint` mentions are only in archived
  plan/review documents, not in live instructions.
- **Tests.** `go build ./...`, `go vet ./internal/adapters/chatdriver/...`,
  `gofmt -l` clean; `go test -race` passes for `grokacp` and `registry`. Full
  `go test ./...` failed in three packages (`observe/scm`, `service/importer`,
  `service/project`), all `RepoOriginURL`-shaped; `observe/scm` reproduces
  identically on `origin/main` in the same VM, whose git config rewrites GitHub
  remotes to a token URL. Not attributable to this PR; all 14 CI checks are green
  on `19d6966`, including `build-test` (`go test -race -timeout=20m ./...`) and
  `lint`. `TestServerShutdownEndpoint` did not fail here.

## Non-blockers

- ACP `acceptEdits` / `auto` live mapping — P1.
- Spec and task tables still name test functions (`TestHarness`,
  `TestConfigure_DefaultPermissions`, …) that do not exist under those names in
  `driver_test.go`; the behaviour is covered by the table-driven equivalents. Nit.
