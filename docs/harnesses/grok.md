# Grok

AO supports xAI's Grok Build agent (binary `grok`) in Terminal UI mode through
its interactive TUI and in Chat mode through Grok's own ACP server. Both modes
launch the user's existing installation inside the session worktree; AO never
bundles, downloads, or substitutes the CLI.

## Install

Install Grok Build with xAI's official installer, then make sure `grok` is on
`PATH`:

```bash
curl -fsSL https://x.ai/cli/install.sh | bash
```

On Windows the official installer is the PowerShell script at
`https://x.ai/cli/install.ps1`.

Verify the install:

```bash
grok --version
```

AO resolves `grok` from:

- `PATH`
- `/usr/local/bin/grok`, `/opt/homebrew/bin/grok`
- Node-managed global bin directories
- `~/.grok/bin/grok`
- `%AppData%\npm\grok.cmd`, `%AppData%\npm\grok.exe`, and
  `%USERPROFILE%\.grok\bin\grok.exe` on Windows

AO's in-app installer offers the same official installers and links
[xAI's docs](https://docs.x.ai/build/overview).

## Supported AO Modes

Grok is exposed in Terminal UI mode and in Chat mode.

**Terminal UI.** A fresh prompted session launches
`grok --no-auto-update [--permission-mode <mode>] [--model <model>] --rules <text>`
and AO delivers the task through the terminal once the TUI is ready. Grok's
`-p`/`--single` flag runs one headless turn and exits, and a bare positional
prompt is parsed as a subcommand, so neither can carry the first task. Activity
tracking uses Claude Code-shaped hooks written to
`.claude/settings.local.json` in the worktree, which Grok reads through its
documented Claude Code compatibility layer; the installed commands are
`ao hooks grok …`, so activity stays attributed to the Grok harness. Restore
prefers the hook-captured native session id via `grok … -r <id>`.

**Chat.** Chat mode uses Grok's native ACP server:

```bash
grok --no-auto-update agent [--always-approve] [--model <model>] stdio
```

The `agent` subcommand selects ACP mode and `stdio` selects the JSON-RPC
transport. `--no-auto-update` is a global flag and keeps its position ahead of
the subcommand, so an AO-managed session never self-updates mid-run.
`--always-approve` is added only for a bypass-permissions spawn.

## Auth

AO checks Grok auth using local-only signals, in this order:

1. `XAI_API_KEY` in the environment.
2. A configured per-model `api_key` or a populated `env_key` in
   `~/.grok/config.toml`. Other config values — model ids, base URLs — are not
   evidence of authentication.
3. A session entry with an access or refresh token in `~/.grok/auth.json`.

`GROK_HOME` overrides the whole directory, and AO follows it for both modes so
Chat and TUI sessions never disagree about where Grok keeps its state. These
checks are advisory: an inconclusive probe is not proof the session cannot run,
and the ACP handshake remains authoritative. A later model call can still fail
on quota or model availability.

## Standing Instructions

Chat delivers AO's standing instructions through the ACP session's
`_meta.rules` field. Grok folds that field into the `<human_rules>` section of
its own system prompt, so AO's instructions are appended to — never a
replacement for — what the user's installation configures.

Grok's argv `--rules` flag is **not** the Chat path. It is consumed by the TUI
and `-p` paths only, and `agent` mode ignores it, so Chat carries nothing in
argv for this.

`_meta.rules` is the only `_meta` field AO sends. Grok's other documented
extension, `_meta.yoloMode`, stays unused: AO already expresses
bypass-permissions through the `bypassPermissions` session mode plus
`--always-approve`, and duplicating it in metadata would give the session a
second, silent way to widen the user's approvals.

## Models

AO does not keep a Grok model list. Chat and TUI model ids both come from the
installation itself — `grok models`, parsed by `modelcatalog.parseGrokModels` —
and are bare ids such as `grok-code-fast` or `grok-4.5`. AO forwards a model
override verbatim, as `--model` on the launch and as the ACP `model` session
option, and imposes no id format; availability is answered by the models the
ACP session advertises.

In Chat, Grok advertises a `model` config option, so the composer's model menu
is driven by the provider catalog: selecting a model writes
`PATCH /conversation/config-options/{id}` and the engine records the choice as
that option's `currentValue`. The native `settings.model` path applies only to
harnesses that advertise no such option.

## Permission Modes

AO's approval vocabulary maps onto Grok's own mode ids, which are the same
strings the TUI adapter passes to `--permission-mode`:

| AO mode | Grok session mode | Launch flag |
| --- | --- | --- |
| `default` | `""` — the user's `~/.grok` config default | — |
| `accept-edits` | `acceptEdits` | — |
| `auto` | `auto` | — |
| `bypass-permissions` | `bypassPermissions` | `--always-approve` |

bypass-permissions is the one mode expressed twice: the session mode alone would
still route some tools through Grok's approval UI, and the launch flag is what
suppresses it.

Grok still surfaces `session/request_permission` for approval-mode changes made
after spawn, because a mid-session PATCH only gets `SetSessionMode` and never
the launch flag. AO resolves those requests from the current mode rather than
parking them: accept-edits answers allow-once for edit, delete, and move tools
and leaves everything else to the user, while auto and bypass-permissions prefer
allow-always and fall back to allow-once.

None of these modes is OS or network containment. Grok's autonomous settings are
its own; AO does not add a sandbox.

## Resume

A terminated Chat session reopens from its stored provider conversation id.
Grok recovers the transcript, and the model still answers from the history the
session accumulated.

**Grok does not apply updated rules on reload.** `_meta.rules` is a
`session/new` input. A reloaded session keeps the rules it was created with, and
a `session/load` carrying different rules is observably ignored — proved on the
wire by `TestLiveGrokACPResume`, where the resumed answer still bore the token
from the original start. So resume preserves standing instructions but does not
update them, and a session whose standing instructions changed needs a new
provider session. AO keeps re-sending `_meta.rules` on load for protocol
correctness, and does not depend on Grok honouring the newer copy.

Two further limits are worth knowing:

- AO's own typed transcript replay is gated on Grok advertising `loadSession`.
  Without it the model keeps its context while AO has no structured history to
  render.
- The warm reattach path — a daemon restart, where the persistent host survives
  and `Resume` reattaches instead of reconnecting — is not covered by a live
  test. `SessionMeta` is not re-sent on that path at all.

## Live Test Gates

Grok's live tests are gated because they spend real Grok account usage. Without
the gate they skip, with the missing prerequisite as the reason.

**Driver-level** (`backend/internal/adapters/chatdriver/grokacp/live_test.go`) —
session smoke with on-disk proof, the four-mode permission matrix, and
terminate/resume:

```bash
AO_LIVE_GROK_ACP=1 go test ./internal/adapters/chatdriver/grokacp/...
```

**API E2E** (`backend/e2e/chat_grok_test.go`) — spawn, model override, reasoning
effort, per-turn approval override, staged attachments, and combined server
state through the daemon's HTTP routes:

```bash
AO_CHAT_E2E=1 AO_LIVE_GROK_ACP=1 go test ./e2e/ -run ChatGrok
```

**UI E2E** (`frontend/e2e/chat-grok-e2e.spec.ts`) — harness selection in the
new-task dialog, the composer's model and reasoning-effort menus, an attachment
uploaded through the file picker, the timeline's tool rows, and the workspace
panel, each asserted against server state and the worktree rather than the UI
label alone:

```bash
AO_LIVE_GROK_ACP=1 AO_E2E_LIVE_PROJECT=<projectId> \
  npx playwright test chat-grok-e2e
```

The UI suite needs a running daemon and a project already registered in it; it
never creates or deletes projects, and the sessions it spawns are terminated,
not force-deleted. Two prerequisites are easy to miss:

- The project named by `AO_E2E_LIVE_PROJECT` needs a resolvable default branch.
  One still reporting `auto` fails at session spawn, before any UI assertion
  runs; set its `defaultBranch` to `main`.
- The renderer must be served **without** `VITE_NO_ELECTRON`
  (`npm run dev:web:live`, which `playwright.config.ts` starts itself when a
  live gate is set). The ordinary `dev:web` server compiles in preview mode and
  reads a mocked workspace snapshot, so no real session is ever visible.

The bootstrap is documented in full in `frontend/e2e/support/live-daemon.ts`.

## Parity Reference

Grok is new to Chat, so the same UI and API scenarios run against Claude Code as
a reference: that separates "Grok is broken" from "this surface never worked for
any ACP harness", which a failing Grok run alone cannot answer.

```bash
AO_LIVE_CLAUDE_ACP=1 CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1 \
  AO_E2E_LIVE_PROJECT=<projectId> \
  npx playwright test chat-reference-e2e
```

The reference harness is Claude Code driven through AO's `claudeacp` binding.
Which models answer behind it is the operator's own Claude Code configuration:
on this stack that is the Anthropic-compatible gateway at
`https://ai.metrica.pro/v1` with the GLM 5.3 family as the primary model
mapping. See [the parity note](../research/grok-claude-parity-p4b.md) for the
environment mapping and for what the runs do and do not establish.

## Review Mode

Grok is also available as an interactive reviewer pane, classified experimental
and user-approved: it keeps its native approval prompts instead of receiving
broad unattended flags, because AO cannot enforce a read-only sandbox for it.

## Not Supported

- Headless `-p`/`--single` mode as a session driver: it runs one turn and exits.
- TUI-to-Chat or Chat-to-TUI handoff of an existing Grok session.
- Updating standing instructions on an existing provider session (see
  [Resume](#resume)).

## Status

Current shipped state, including the live evidence behind each gate, is tracked
in [docs/STATUS.md](../STATUS.md).
