# Grok ↔ Claude Code UI parity (P4b)

Updated: 2026-09-15. Subject: the Grok ACP Chat driver's UI-level E2E gates and
what the Claude Code reference run beside them does and does not establish.

Specs: `frontend/e2e/chat-grok-e2e.spec.ts`,
`frontend/e2e/chat-reference-e2e.spec.ts`, shared scenario bodies in
`frontend/e2e/support/live-chat-scenarios.ts`. Archived OpenSpec change:
`openspec/changes/archive/2026-09-15-grok-acp-p4b-playwright-e2e`.

## Why a reference run at all

Grok is new to Chat. A failing Grok scenario cannot, on its own, distinguish
"the Grok binding is broken" from "this AO surface never worked for any ACP
harness". Claude Code has been driven through these same surfaces long enough to
be the yardstick, so the reference specs call the identical scenario bodies —
the same split `backend/e2e/chat_reference_test.go` uses at the API level.

## What the reference harness is

The reference is **the Claude Code harness**, driven through AO's `claudeacp`
binding over the Anthropic wire protocol.

Which models answer behind that protocol is the operator's own Claude Code
configuration; AO configures none of it and reads none of these variables. On
this stack the canonical Claude Code mapping is the Anthropic-compatible gateway
at `https://ai.metrica.pro/v1` with the GLM 5.3 family as the primary models:

| Variable (Claude Code's own environment) | Value |
| --- | --- |
| `ANTHROPIC_BASE_URL` | `https://ai.metrica.pro/v1` |
| `ANTHROPIC_AUTH_TOKEN` | the gateway key |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | `glm-5.3` |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | `glm-5.3-flash` |

The suite's own variable is separate: `CLAUDE_ROUTER_URL` is the operator's
statement that the above was configured. AO never reads it, so the spec cannot
verify the routing from the inside; requiring it means an unconfigured machine
skips instead of quietly passing as G4.

This is a first-class configuration, not a stand-in. GLM 5.3 is multimodal and
speaks the Anthropic protocol, which is the contract Claude Code and AO's ACP
binding both talk, so neither side needs to know which models sit behind the
gateway. AO already names this class of route on its own: the Claude hook route
hint maps `api.z.ai` to the `zai` billing provider
(`backend/internal/cli/hooks.go`), and `pricing/catalog/v1/providers/zai` prices
both `glm-5.3` and `glm-5.3-flash`.

One accounting detail is worth stating because it is easy to trip over:
`ai.metrica.pro` is not `api.z.ai`, so `claudeHookProviderHint` records it as
`unidentified` rather than `zai`. That affects billing attribution for such a
session and nothing in the harness contract these runs assert.

## Scenario results

Run on 2026-09-15 against a real daemon, a real Grok account, and Claude Code on
the gateway above.

```bash
AO_LIVE_GROK_ACP=1 AO_E2E_LIVE_PROJECT=<projectId> \
  npx playwright test chat-grok-e2e                         # 6/6 passed

AO_LIVE_CLAUDE_ACP=1 CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1 \
  AO_E2E_LIVE_PROJECT=<projectId> \
  npx playwright test chat-reference-e2e                    # 4/4 passed
```

| Scenario | Grok | Claude Code reference | What the shared body asserts |
| --- | --- | --- | --- |
| Harness selection in the new-task dialog | PASS | not run (Grok-specific) | The dialog's agent menu creates a chat session on the chosen harness, and its first turn answers on screen. |
| Model switch from UI applies to engine | PASS | PASS | The composer's model menu writes to the catalog the live session advertises, the engine reports the new value, and a following turn completes on it. |
| Reasoning effort propagates from UI | PASS | PASS | The effort control writes to the catalog that owns it and the engine reports the chosen value. |
| File attachment delivered to worktree | PASS | PASS | The staged path AO recorded in the message exists under `.ao/attachments/`, holds the uploaded bytes, is readable by the agent, and appears in the timeline. |
| Timeline shows tool activities | PASS | PASS | A turn that had to write a file produces tool activities with a kind and status, a matching row renders, and the file is on disk. |
| Workspace panel matches worktree | PASS | not run (extends the timeline body) | The panel lists the file the agent created, and its contents on disk match. |

Neither suite reported a scenario skip, so the model and effort controls were
exercised on live catalogs rather than skipped for want of a second advertised
choice.

## What was compared

- **The same code, not two descriptions of it.** Both specs call one scenario
  body per guarantee, so a claim of parity is a claim about one implementation
  running twice.
- **Server state and disk, not UI labels.** Every scenario asserts the outcome
  through the daemon (`GET /conversation`, `/conversation/config-options`) and
  the worktree. A dropdown that changes only its own label fails.
- **The catalog each harness actually uses.** Grok and Claude Code both
  advertise a provider `model` config option, so both took the
  `PATCH /conversation/config-options/{id}` path and both were asserted on that
  option's `currentValue`. `settings.model` is the native-catalog path and is
  never written for either harness; asserting it unconditionally — as the
  original P4b design sketch did — would have failed a model switch that did
  apply.

## What was not compared

- **No side-by-side transcript diff.** The runs are independent and their model
  output is not compared token for token, turn for turn, or timing for timing.
  Parity here means both harnesses satisfy the same assertions about AO's
  surfaces, not that they answer alike.
- **No identical model behind both harnesses.** Grok answers on xAI models from
  `grok models`; Claude Code answers on the gateway's GLM 5.3 mapping. Model
  identity was never the subject — the harness contract was.
- **Two scenarios are Grok-only by design.** Harness selection and the
  workspace panel have no reference run: the first is about picking Grok
  specifically, and the second extends the timeline body rather than standing
  alone.
- **No approval-mode coverage at the UI level.** These scenarios never change
  approval mode. Grok's four-mode matrix lives in the driver-level live test
  (`grokacp/live_test.go`) and the per-turn override in the API suite
  (`backend/e2e/chat_grok_test.go`).
- **No warm-reattach resume.** Resume coverage is the cold path
  (`Terminate()` then `Resume`). A daemon restart takes the reattach branch,
  where `SessionMeta` is not re-sent at all; that path has no live test in
  either suite.

## Related

- [Grok harness doc](../harnesses/grok.md) — install, modes, permission
  mapping, resume caveats, and the full gate list.
- [docs/STATUS.md](../STATUS.md) — shipped state and the live evidence summary.
- `openspec/changes/archive/2026-09-15-grok-acp-p4b-playwright-e2e` — the
  archived change, including the "As built" corrections the live runs forced.
