# Claude Code

AO supports Claude Code in Terminal UI mode through the `claude` executable and
in Chat mode through the bundled `@agentclientprotocol/claude-agent-acp`
adapter. AO ships the protocol adapter and its Node runtime, not Claude Code
itself: the adapter receives `CLAUDE_CODE_EXECUTABLE` pointing at the same
user-installed binary the TUI adapter uses, so login, subscription, settings,
MCP configuration, hooks, and project instructions stay the user's own.

This doc covers install, auth, and **model routing** — in particular the
reference configuration AO's live Claude gates run on. Broader Chat behavior is
described in [docs/STATUS.md](../STATUS.md) and
[docs/architecture.md](../architecture.md).

## Install and resolution

AO resolves `claude` from `PATH` first, then the native installer's locations,
npm global, Homebrew, and Claude's own managed directory:

- `PATH`
- `/usr/local/bin/claude`, `/opt/homebrew/bin/claude`
- Node-managed global bin directories
- `~/.claude/local/claude`
- `%AppData%\npm\claude.exe`, `%AppData%\npm\claude.cmd`, and
  `%AppData%\npm\node_modules\@anthropic-ai\claude-code\bin\claude.exe` on
  Windows

Chat runs the ACP adapter on AO's own packaged Node runtime, not the user's
Node. The driver probes that runtime first and reports the Chat driver as
unavailable — rather than launching — when it is missing or older than Node 22.
`AO_ACP_RUNTIME_DIR` and `AO_CLAUDE_ACP_COMMAND` override where it comes from.

## Auth

AO checks Claude auth with local-only signals: `ANTHROPIC_API_KEY`,
`ANTHROPIC_AUTH_TOKEN`, or `CLAUDE_CODE_OAUTH_TOKEN` in the environment, then
Claude's own `~/.claude.json`. An inconclusive probe is not proof the session
cannot run, so AO continues and lets the handshake decide.

Any of these satisfies the probe, including a gateway key in
`ANTHROPIC_AUTH_TOKEN`. AO never reads the credential value and never writes to
Claude's config.

## Model routing

Claude Code talks the Anthropic Messages protocol, and that protocol — not a
model name — is the contract AO's binding depends on. Pointing Claude Code at an
Anthropic-compatible gateway is therefore a first-class configuration, and AO
treats it as one: the Claude hook route hint resolves `ANTHROPIC_BASE_URL` to one
of four named routes (`anthropic`, `zai`, `bedrock`, `vertex_ai`) and records
`unidentified` for a gateway host it cannot name, which keeps a real route from
being mistaken for "no hook ran"
(`backend/internal/cli/hooks.go`, `backend/internal/pricing/identity.go`).

Nothing in AO configures the route. It belongs to the operator's Claude Code
installation; AO forwards the project environment through unchanged and merges a
session overlay over the daemon environment.

### Standing reference configuration: GLM 5.3 over the Anthropic-compatible gateway

This is the supported configuration for AO's live Claude Code runs — the G4
parity reference today, and the pattern future live Claude runs should assume,
rather than a one-off substitution for a particular test session.

Claude Code's aliases are remapped to the GLM 5.3 family through the gateway:

| Variable | Value | What it controls |
| --- | --- | --- |
| `ANTHROPIC_BASE_URL` | `https://ai.metrica.pro/v1` | The Anthropic-compatible endpoint Claude Code talks to. |
| `ANTHROPIC_AUTH_TOKEN` | the gateway key | Credential for that endpoint. Satisfies AO's auth probe. |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | `glm-5.3` | What the `opus` alias, and `opusplan` in Plan Mode, resolve to — the Opus-class slot. |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | `glm-5.3-flash` | What the `sonnet` alias, and `opusplan` outside Plan Mode, resolve to — the Sonnet-class slot. |

Reproduce it the way Claude Code documents, in `~/.claude/settings.json`:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://ai.metrica.pro/v1",
    "ANTHROPIC_AUTH_TOKEN": "<gateway key>",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "glm-5.3",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.3-flash"
  }
}
```

Or per project, in AO's own project config `env` map, which AO forwards into
worker sessions — useful when one machine runs several routes.

Four things are worth knowing before reproducing it:

- **The `DEFAULT_*` variables pin alias resolution, not the starting model.**
  A session's initial model comes from `ANTHROPIC_MODEL`, the `model` settings
  key, or `--model`. Set the aliases and the model menu's `opus`/`sonnet` entries
  resolve to GLM; set `ANTHROPIC_MODEL` as well if a session should also *start*
  on a specific id.
- **Background work uses the Haiku slot.** `ANTHROPIC_DEFAULT_HAIKU_MODEL` is
  not part of the mapping above, so background functionality keeps resolving a
  Haiku-class id the gateway may not serve. Point it at a cheap gateway model if
  background calls start failing.
- **AO's agent-config model picker does not read the `env` block.** It resolves
  a Claude model from `ANTHROPIC_MODEL` (project env, then process env), then a
  top-level `"model"` key in the worktree's `.claude/settings.local.json` or
  `.claude/settings.json`, then `~/.claude/settings.json`. A mapping configured
  only inside `env` is invisible to it — which is harmless, because in Chat the
  model list comes from the live ACP session's own `model` config option, and
  that reflects whatever the gateway resolved.
- **Billing attribution follows the host, not the model.** `api.z.ai` is
  recorded as the `zai` provider and priced from
  `pricing/catalog/v1/providers/zai`, which already carries `glm-5.3` and
  `glm-5.3-flash`. A different gateway host — `ai.metrica.pro` among them — is
  recorded as `unidentified`, so its sessions are not priced. That affects cost
  reporting only; it has no bearing on the harness contract.

The same pattern is documented upstream by Z.ai for the GLM Coding Plan, whose
Anthropic-protocol endpoint is `https://api.z.ai/api/anthropic`
([Z.ai docs](https://docs.z.ai/devpack/tool/claude)); the variables themselves
are Claude Code's own
([model configuration](https://code.claude.com/docs/en/model-config)).

## Role in AO's live gates

Claude Code is AO's reference harness for new Chat bindings. A new binding's
scenarios run against it from the same shared bodies, which is what separates
"the new binding is broken" from "this AO surface never worked for any ACP
harness".

```bash
# API level
AO_CHAT_E2E=1 AO_LIVE_CLAUDE_ACP=1 go test ./e2e/ -run ChatReference

# UI level
AO_LIVE_CLAUDE_ACP=1 CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1 \
  AO_E2E_LIVE_PROJECT=<projectId> \
  npx playwright test chat-reference-e2e
```

`CLAUDE_ROUTER_URL` is not read by AO. The UI suite requires it as the
operator's statement that the gateway above was configured, so an unconfigured
machine skips instead of quietly passing as a reference run.

The most recent reference run and its scope are recorded in
[the Grok parity note](../research/grok-claude-parity-p4b.md).

## Related

- [Grok harness doc](grok.md) — the binding this reference currently backstops.
- [docs/STATUS.md](../STATUS.md) — shipped Chat driver state and live evidence.
