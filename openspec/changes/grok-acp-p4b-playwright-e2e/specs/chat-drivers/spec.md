# Specification: Playwright UI E2E Contract (P4b)

This specification defines the UI-level E2E requirements for proving the
frontend↔engine contract works for Grok Chat sessions, with Claude Code as
the reference implementation.

## ADDED Requirements

### Requirement: Harness Selection from UI

The UI SHALL allow selecting Grok harness when creating a new task.

#### Scenario: Select Grok in new task dialog

**GIVEN** the AO desktop/web UI is open
**WHEN** the user clicks "New task"
**AND** selects "Grok" from the agent/harness dropdown
**AND** enters a task prompt
**AND** clicks "Start"
**THEN** a Chat session is created with harness=grok
**AND** the session header shows "Grok" as the agent

---

### Requirement: Model Switch from UI Applied to Engine

The UI model selector SHALL apply model changes to the engine session.

The engine records the choice in whichever catalog the live session advertises,
and an assertion SHALL read the field that catalog writes:

- **provider catalog** — the ACP agent advertises a `model` config option, so
  the UI writes `PATCH /conversation/config-options/{id}` and the engine records
  the choice as that option's `currentValue` in
  `GET /conversation/config-options`. Grok and Claude Code both take this path.
- **native catalog** — no provider model option, so the UI writes
  `PATCH /conversation/settings` and the engine records `settings.model`, which
  `GET /conversation` reports.

Asserting `settings.model` unconditionally would fail on every ACP harness for a
model switch that did apply.

#### Scenario: Model dropdown changes engine model

**GIVEN** an active Grok Chat session in the UI
**WHEN** the user clicks the model dropdown
**AND** selects a second model the live session advertises
**THEN** the UI shows the selected model
**AND** the engine records it in the catalog that owns the choice
**AND** the next turn completes on the selected model

#### Scenario: Model switch not ignored (regression prevention)

**GIVEN** an active session on its default model
**WHEN** the user switches to a different advertised model via UI
**AND** sends a new message
**THEN** the engine processes the turn on the newly selected model
**AND** the recorded state still reports it after the turn settles
**AND** the change is NOT silently ignored

---

### Requirement: Reasoning Effort from UI Propagates

The UI reasoning effort selector SHALL propagate to the engine.

Effort follows the same two catalogs as the model: a provider `thought_level`
(or `effort`) config option records the choice as that option's `currentValue`,
while a native effort list belongs to a model and records
`settings.reasoningEffort`. When nothing advertises efforts the composer renders
no effort control, and the scenario SHALL skip rather than assert an invented
one.

#### Scenario: Effort selector changes engine setting

**GIVEN** an active Grok Chat session in the UI
**WHEN** the user opens the composer's turn-settings menu
**AND** sets reasoning effort to a non-default advertised value
**THEN** the UI shows the selected effort
**AND** the engine records it in the catalog that owns the choice

---

### Requirement: File Attachments from UI Delivered to Worktree

Files attached via UI SHALL be delivered to the session worktree.

The daemon names staged files itself, so the path is
`<workspace>/.ao/attachments/attachment-*.<ext>` rather than the uploaded file
name. An assertion SHALL read the staged path AO recorded in the user message
instead of reconstructing one from the upload.

#### Scenario: File picker upload appears in worktree

**GIVEN** an active Grok Chat session
**WHEN** the user attaches "test-upload.txt" through the composer's file input
**AND** sends a message asking the agent to read it
**THEN** the recorded user message carries a staged path under `.ao/attachments/`
**AND** the bytes at that path in the worktree are the uploaded bytes
**AND** the agent reports the file's contents in its answer
**AND** the staged path is visible in the timeline

---

### Requirement: Timeline Shows Tool Activities

The UI timeline SHALL display tool activities from the agent.

#### Scenario: Command execution visible

**GIVEN** an active Grok Chat session
**WHEN** a turn completes that executed shell commands
**THEN** the timeline shows command activities
**AND** command output is viewable

#### Scenario: File edit visible

**GIVEN** an active Grok Chat session
**WHEN** a turn completes that edited files
**THEN** the timeline shows file edit activities
**AND** diff is viewable

---

### Requirement: On-Disk Files Match UI

Files shown in workspace panel SHALL match actual worktree.

#### Scenario: Created file appears in workspace

**GIVEN** an active Grok Chat session
**WHEN** the agent creates a file "created.txt"
**THEN** the workspace panel shows "created.txt"
**AND** `os.ReadFile(<workspace>/created.txt)` returns expected content

---

### Requirement: Claude Code Reference Parity

Grok UI behavior SHALL match Claude Code reference implementation.

#### Scenario: Same UI operations produce equivalent outcomes

**GIVEN** identical UI operation sequences for Grok and Claude Code
**WHEN** both complete
**THEN** timeline structures are equivalent
**AND** worktree file outcomes match
**AND** model/settings UI reflects same state patterns

#### Scenario: Claude Code on its ai.metrica.pro primary-model mapping

The reference environment runs Claude Code against the Anthropic-compatible
gateway at `https://ai.metrica.pro/v1`, with the GLM 5.3 family as its primary
model mapping (`ANTHROPIC_DEFAULT_OPUS_MODEL=glm-5.3`,
`ANTHROPIC_DEFAULT_SONNET_MODEL=glm-5.3-flash`). That is the canonical Claude
Code configuration on this stack, not a fallback: the Anthropic wire protocol is
what Claude Code and AO's ACP binding speak, and AO already names this route
(`api.z.ai` → `zai` in `claudeHookProviderHint`) and prices `glm-5.3` and
`glm-5.3-flash` in `pricing/catalog/v1/providers/zai`.

**GIVEN** Claude Code configured against the gateway with that model mapping
**WHEN** the reference suite runs
**THEN** the reference scenarios complete on the gateway's models
**AND** the outcomes stand as the reference for the Grok scenarios

---

## Test Files

- `frontend/e2e/chat-grok-e2e.spec.ts` — Grok-specific UI E2E tests
- `frontend/e2e/chat-reference-e2e.spec.ts` — Claude Code reference tests

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `AO_LIVE_GROK_ACP` | Enable live Grok tests |
| `AO_LIVE_CLAUDE_ACP` | Enable Claude Code reference tests |
| `CLAUDE_ROUTER_URL` | The reference gateway the operator configured Claude Code against |
| `AO_E2E_LIVE_PROJECT` | Daemon project id the sessions spawn in |

The gateway itself is configured in the operator's own Claude Code installation,
not by AO: `ANTHROPIC_BASE_URL=https://ai.metrica.pro/v1` with
`ANTHROPIC_DEFAULT_OPUS_MODEL=glm-5.3` and
`ANTHROPIC_DEFAULT_SONNET_MODEL=glm-5.3-flash`. `CLAUDE_ROUTER_URL` is the
operator's statement that this was done, which the spec requires so an
unconfigured machine skips instead of passing as G4.

## Test Functions

```typescript
// Grok tests
test("select Grok harness in new task dialog", ...)
test("model switch from UI applies to engine", ...)
test("reasoning effort propagates from UI", ...)
test("file attachment delivered to worktree", ...)
test("timeline shows tool activities", ...)
test("workspace panel matches worktree", ...)

// Claude Code reference tests
test("reference: model switch from UI applies to engine", ...)
test("reference: reasoning effort propagates from UI", ...)
test("reference: file attachment delivered to worktree", ...)
test("reference: timeline shows tool activities", ...)
```

## Guarantees Encoded as WHEN/THEN

These are the critical guarantees Eugene requested:

### G-MODEL: Model Switch Applied
**WHEN** model is switched from UI dropdown
**THEN** the engine session uses the new model (not ignored)

### G-EFFORT: Reasoning Effort Propagates
**WHEN** reasoning effort is set in UI
**THEN** the engine receives the setting (visible in API state)

### G-FILES: UI Files Delivered
**WHEN** files are chosen in UI through the composer's file input
**THEN** the staged files exist in the engine cwd/worktree
**AND** are visible to the agent during task execution

### G-PARITY: Grok Matches Claude Reference
**WHEN** same UI operations are performed on Grok and Claude Code
**THEN** outcomes match for model, effort, files, and timeline
