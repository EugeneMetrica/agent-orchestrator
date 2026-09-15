# Specification: API Frontend E2E Contract (P4a)

This specification defines the API-level E2E requirements for proving the
frontend↔engine contract works for Grok Chat sessions.

## ADDED Requirements

### Requirement: Spawn Chat Session with Grok Harness

The daemon SHALL spawn Chat sessions with harness=grok via HTTP API.

#### Scenario: Spawn via POST /sessions

**GIVEN** a project with Grok configured as agent
**WHEN** `POST /api/v1/sessions` is called with:
```json
{
  "projectId": "test-project",
  "kind": "worker",
  "harness": "grok",
  "mode": "chat",
  "prompt": "Initial task"
}
```
**THEN** the response contains `session.id` and `session.mode == "chat"`
**AND** the session's harness is `"grok"`

---

### Requirement: Model Override Applied to Engine

The daemon SHALL apply model overrides from turn settings to the Grok engine.

#### Scenario: Model set via turn settings API

**GIVEN** an active Grok Chat session
**WHEN** `PATCH /api/v1/sessions/{id}/conversation/settings` is called with:
```json
{"model": "grok-4.5"}
```
**THEN** the response confirms model is set
**AND** subsequent turns use the specified model
**AND** `GET /api/v1/sessions/{id}/conversation` shows `settings.model == "grok-4.5"`

#### Scenario: Model switch not ignored

**GIVEN** an active Grok Chat session with model set to `"xai/grok-2"`
**WHEN** model is changed to `"grok-4.5"` via API
**AND** a turn is sent
**THEN** the engine processes the turn with model `"grok-4.5"`
**AND** the turn's model is recorded in conversation state

---

### Requirement: Reasoning Effort Propagation

The daemon SHALL propagate reasoning effort settings to the engine.

#### Scenario: Reasoning effort set via API

**GIVEN** an active Grok Chat session
**WHEN** `PATCH /api/v1/sessions/{id}/conversation/settings` is called with:
```json
{"reasoningEffort": "high"}
```
**THEN** the response confirms reasoningEffort is set
**AND** `GET /api/v1/sessions/{id}/conversation` shows `settings.reasoningEffort == "high"`

#### Scenario: Reasoning effort affects engine behavior

**GIVEN** reasoningEffort set to `"high"`
**WHEN** a turn is sent with a complex reasoning task
**THEN** the engine uses the configured reasoning effort level
**AND** (if observable) the provider's reasoning_effort parameter matches

---

### Requirement: Approval Mode Override

The daemon SHALL apply per-turn approval mode changes.

#### Scenario: Turn-level approval override via API

**GIVEN** an active session with default permissions
**WHEN** `POST /api/v1/sessions/{id}/conversation/messages` is called with:
```json
{
  "text": "Create a file",
  "settings": {"approvalMode": "bypass-permissions"}
}
```
**THEN** the turn uses bypass-permissions mode
**AND** no approval prompts are generated for that turn

---

### Requirement: File Attachments Delivered to Worktree

The daemon SHALL deliver uploaded attachments to the session worktree.

#### Scenario: Upload and reference attachment

**GIVEN** an active Grok Chat session with workspace at `/tmp/workspace`
**WHEN** `POST /api/v1/sessions/{id}/conversation/attachments` uploads `test.png`
**AND** the response contains `paths: [".ao/attachments/test.png"]`
**AND** a message is sent referencing the attachment
**THEN** the file exists at `<workspace>/.ao/attachments/test.png`
**AND** the agent can read/reference the file

#### Scenario: Multiple attachments in single turn

**GIVEN** an active Grok Chat session
**WHEN** multiple files are uploaded and referenced in one message
**THEN** all files exist in the worktree `.ao/attachments/` directory
**AND** the message content references all paths

---

### Requirement: Server State Consistency

The daemon SHALL maintain consistent server state across API operations.

#### Scenario: Conversation state reflects operations

**GIVEN** a series of API operations (spawn, set model, send turn)
**WHEN** `GET /api/v1/sessions/{id}/conversation` is called
**THEN** the response includes:
  - Correct `harness: "grok"`
  - Current `settings.model`
  - Current `settings.reasoningEffort`
  - All turns with correct states
  - Activities (tool calls, commands) in timeline

#### Scenario: Turn state progression

**GIVEN** a sent message
**WHEN** polling conversation state
**THEN** turn progresses through: `queued → running → completed`
**AND** final state is `completed` (or `failed` with error)

---

### Requirement: Parity with Claude Code Reference

Tests SHALL verify Grok behavior matches Claude Code for the same API surface.

The reference is the Claude Code harness driven through AO's `claudeacp`
binding. Which models answer behind it is the operator's own Claude Code
configuration, not AO's: on this stack that is the Anthropic-compatible gateway
at `https://ai.metrica.pro/v1` with the GLM 5.3 family as its primary model
mapping. The Anthropic wire protocol is the contract both sides speak, so the
parity claim is about the harness contract and holds for any model the gateway
serves.

#### Scenario: Same API operations produce equivalent outcomes

**GIVEN** identical API operation sequences for Grok and Claude Code
**WHEN** both complete
**THEN** server state structures are equivalent (modulo harness-specific fields)
**AND** file/worktree outcomes match (same attachment paths, same file creation)

---

## Test Files

- `backend/e2e/chat_grok_test.go` — Grok-specific API E2E tests
- `backend/e2e/chat_reference_test.go` — Claude Code reference tests for parity

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `AO_LIVE_GROK_ACP` | Enable live Grok tests |
| `AO_LIVE_CLAUDE_ACP` | Enable Claude Code reference tests |
| `AO_CHAT_E2E` | Enable the Chat e2e suite at all |

## Test Functions

```go
func TestChatGrokSpawn(t *testing.T)
func TestChatGrokModelOverride(t *testing.T)
func TestChatGrokReasoningEffort(t *testing.T)
func TestChatGrokApprovalModeOverride(t *testing.T)
func TestChatGrokAttachments(t *testing.T)
func TestChatGrokServerStateConsistency(t *testing.T)

// Reference tests for parity
func TestChatClaudeCodeModelOverride(t *testing.T) // Reference implementation
func TestChatClaudeCodeAttachments(t *testing.T)   // Reference implementation
```
