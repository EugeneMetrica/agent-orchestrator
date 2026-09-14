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

#### Scenario: Model dropdown changes engine model

**GIVEN** an active Grok Chat session in the UI
**WHEN** the user clicks the model dropdown
**AND** selects "xai/grok-3"
**THEN** the UI shows the selected model
**AND** the next turn uses model "xai/grok-3"
**AND** GET /conversation shows `settings.model == "xai/grok-3"`

#### Scenario: Model switch not ignored (regression prevention)

**GIVEN** an active session with model "xai/grok-2"
**WHEN** user switches model to "xai/grok-3" via UI
**AND** sends a new message
**THEN** the engine processes with "xai/grok-3"
**AND** the conversation state reflects the new model
**AND** the change is NOT silently ignored

---

### Requirement: Reasoning Effort from UI Propagates

The UI reasoning effort selector SHALL propagate to the engine.

#### Scenario: Effort selector changes engine setting

**GIVEN** an active Grok Chat session in the UI
**WHEN** the user opens settings/configuration panel
**AND** sets reasoning effort to "high"
**THEN** the UI shows effort as "high"
**AND** GET /conversation shows `settings.reasoningEffort == "high"`

---

### Requirement: File Attachments from UI Delivered to Worktree

Files attached via UI SHALL be delivered to the session worktree.

#### Scenario: Drag-drop file appears in worktree

**GIVEN** an active Grok Chat session
**WHEN** the user drags a file "test-upload.txt" to the chat input
**AND** sends a message referencing it
**THEN** the file exists at `<workspace>/.ao/attachments/test-upload.txt`
**AND** the agent can read the file
**AND** the agent response references the file content

#### Scenario: File picker upload

**GIVEN** an active Grok Chat session
**WHEN** the user clicks the attachment button
**AND** selects a file via the picker
**AND** sends the message
**THEN** the file is uploaded and visible in worktree
**AND** appears in the message content

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

#### Scenario: Claude Code via ai.metrica.pro router

**GIVEN** Claude Code configured with ai.metrica.pro as model router
**WHEN** running the reference test suite
**THEN** Claude Code uses the custom router for model resolution
**AND** model alias substitution works correctly
**AND** tests pass as reference implementation

---

## Test Files

- `frontend/e2e/chat-grok-e2e.spec.ts` — Grok-specific UI E2E tests
- `frontend/e2e/chat-reference-e2e.spec.ts` — Claude Code reference tests

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `AO_LIVE_GROK_ACP` | Enable live Grok tests |
| `AO_LIVE_CLAUDE_ACP` | Enable Claude Code reference tests |
| `CLAUDE_ROUTER_URL` | ai.metrica.pro router URL for Claude Code |

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
test("reference: file attachment delivered to worktree", ...)
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
**WHEN** files are chosen in UI (drag-drop or picker)
**THEN** files exist in engine cwd/worktree
**AND** are visible to the agent during task execution

### G-PARITY: Grok Matches Claude Reference
**WHEN** same UI operations are performed on Grok and Claude Code
**THEN** outcomes match for model, effort, files, and timeline
