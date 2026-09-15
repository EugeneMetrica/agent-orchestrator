# Specification: Grok ACP Live Smoke Test (P1)

This specification defines the live integration requirements for proving the
Grok ACP Chat driver works end-to-end with a real Grok CLI.

## ADDED Requirements

### Requirement: Live Probe Success

The driver SHALL successfully probe a locally installed Grok CLI with valid
authentication.

#### Scenario: Probe with installed Grok

**GIVEN** Grok CLI is installed and on PATH
**AND** valid authentication exists (XAI_API_KEY or ~/.grok/auth.json)
**AND** `AO_LIVE_GROK_ACP=1` environment variable is set
**WHEN** `driver.Probe(ctx)` is called
**THEN** it returns without error

#### Scenario: Probe reports auth required

**GIVEN** Grok CLI is installed but no authentication exists
**WHEN** `driver.Probe(ctx)` is called
**THEN** it returns `ports.ErrChatAuthRequired`

---

### Requirement: Live Session Start

The driver SHALL successfully start an ACP session with real Grok.

#### Scenario: Start conversation

**GIVEN** a successful probe
**AND** a valid `ChatStartConfig` with workspace, data dir, and permissions
**WHEN** `driver.Start(ctx, config)` is called
**THEN** it returns a non-nil `ChatConversation`
**AND** the conversation has a non-empty `ProviderConversationID()`
**AND** capabilities include Resume=true

---

### Requirement: Live Turn Execution with File Proof

The driver SHALL complete a turn that creates a verifiable file on disk.

#### Scenario: Turn creates proof file

**GIVEN** an active Grok ACP conversation
**WHEN** a turn is sent with prompt: "Use the shell to run `printf grok-acp-ok > proof.txt`, then report success."
**AND** the turn completes successfully
**THEN** the file `<workspace>/proof.txt` exists
**AND** the file contents equal exactly `grok-acp-ok`

---

### Requirement: Standing Instruction Application

The driver SHALL pass standing instructions to Grok via ACP session metadata.

#### Scenario: Standing instruction token in response

**GIVEN** a `ChatStartConfig` with `SystemPrompt = "On every response include the exact token GROK_STANDING_TOKEN."`
**WHEN** a turn is sent and completes
**THEN** the assistant's response text contains `GROK_STANDING_TOKEN`

---

### Requirement: Turn Completion State

The driver SHALL report correct turn completion states.

#### Scenario: Completed turn state

**GIVEN** a turn that finishes normally
**WHEN** the turn completion event is received
**THEN** `event.TurnState == domain.TurnStateCompleted`

---

### Requirement: Test Gating

Live tests SHALL only run when explicitly enabled.

#### Scenario: Tests skip without gate

**GIVEN** `AO_LIVE_GROK_ACP` environment variable is not set or empty
**WHEN** `go test ./internal/adapters/chatdriver/grokacp/...` is run
**THEN** live tests are skipped with message "set AO_LIVE_GROK_ACP=1 to run..."

---

### Requirement: UI Timeline Visibility (G4)

The AO frontend SHALL display Grok Chat sessions correctly.

#### Scenario: Session shows in UI

**GIVEN** a Chat session spawned via API with `harness=grok`, `mode=chat`
**WHEN** the session detail page is opened
**THEN** the timeline shows the conversation turns
**AND** the harness indicator shows "Grok"
**AND** tool activities (commands) appear in the timeline

---

## Test Files

- `backend/internal/adapters/chatdriver/grokacp/live_test.go` — live integration tests

## Test Header Comment

```go
// Live tests require:
// - Grok CLI installed and on PATH
// - Valid authentication (XAI_API_KEY or ~/.grok/auth.json)
// - AO_LIVE_GROK_ACP=1 environment variable
//
// Run explicitly: AO_LIVE_GROK_ACP=1 go test -v ./internal/adapters/chatdriver/grokacp/...
```
