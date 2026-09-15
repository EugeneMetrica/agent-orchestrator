# Specification: Grok ACP Resume & Persistent Host (P3)

This specification defines the Resume capability and persistent-host requirements
for the Grok ACP Chat driver.

## ADDED Requirements

### Requirement: Resume Capability Advertised

The driver SHALL advertise Resume capability.

#### Scenario: Capabilities include resume

**GIVEN** a started Grok ACP conversation
**WHEN** `conv.Capabilities()` is called
**THEN** `capabilities[ports.ChatCapabilityResume] == true`

---

### Requirement: Provider Conversation ID Captured

The driver SHALL capture and expose the provider conversation ID for resume.

#### Scenario: Provider ID available after start

**GIVEN** a successfully started conversation
**WHEN** `conv.ProviderConversationID()` is called
**THEN** it returns a non-empty string
**AND** this ID can be used for Resume

---

### Requirement: Resume Restores Session

The driver SHALL support resuming a conversation by provider ID.

#### Scenario: Resume with provider ID

**GIVEN** a previous conversation with `providerID = conv.ProviderConversationID()`
**AND** the conversation was terminated
**WHEN** `driver.Resume(ctx, ChatResumeConfig{ProviderConversationID: providerID})` is called
**THEN** it returns a non-nil ChatConversation
**AND** the conversation has access to previous history

---

### Requirement: Files Persist Across Resume

The driver SHALL ensure workspace files are visible after resume.

#### Scenario: File proof across terminate/resume

**GIVEN** a conversation that created `before.txt` with content "before-value"
**AND** the conversation was terminated
**WHEN** the conversation is resumed
**AND** a turn is sent asking to read `before.txt`
**THEN** the agent response references or confirms the file exists
**AND** the file content is still "before-value"

---

### Requirement: History Visible After Resume

The driver SHALL make conversation history available after resume.

#### Scenario: Agent sees previous context

**GIVEN** a conversation where the agent was told "Remember the codeword ALPHA"
**AND** the conversation was terminated and resumed
**WHEN** a turn is sent asking "What was the codeword?"
**THEN** the agent response includes "ALPHA"

---

### Requirement: Standing Instructions Survive Resume

The driver SHALL keep AO's standing instructions in force on a resumed
conversation, and SHALL keep sending them as ACP session metadata on
`session/load`.

Grok reads `_meta.rules` when it creates a session and keeps the rules that
session was created with when it reloads one, so resume preserves standing
instructions but does not update them. AO re-sends them for protocol
correctness and forward compatibility, and MUST NOT treat `session/load` as a
way to rewrite Grok's standing rules.

#### Scenario: Standing instruction token present after resume

**GIVEN** a conversation started with `SystemPrompt = "Include START_TOKEN in responses"`
**AND** the conversation was terminated and resumed
**WHEN** a turn is sent and completes
**THEN** the agent response contains a standing instruction token
**AND** the token MAY be the one from the original start, because Grok does not
replace standing rules on `session/load`

#### Scenario: Metadata still delivered on load

**GIVEN** a resumed conversation with a non-empty `SystemPrompt`
**WHEN** AO issues `session/load`
**THEN** the request carries `_meta.rules` with that prompt
**AND** no assertion is made about Grok applying the newer copy

---

### Requirement: SessionMeta Extension (If Needed)

If Grok's ACP requires `_meta.rules` or `_meta.yoloMode` for standing instructions
or permission mode, the driver SHALL document the implementation decision.

#### Scenario: SessionMeta documented (conditional)

**GIVEN** Grok ACP requires SessionMeta for rules/yoloMode
**WHEN** the implementation decision is made
**THEN** design.md documents:
  - Whether canonical `nativeacp` SessionMeta extension is used
  - OR why piacp-style `acpdriver.New` is preferred
  - Rationale for the choice

---

## Test File

- `backend/internal/adapters/chatdriver/grokacp/live_test.go`

## Test Function

```go
func TestLiveGrokACPResume(t *testing.T) {
    // 1. Start session, create before.txt
    // 2. Terminate
    // 3. Resume with same provider ID
    // 4. Send turn referencing before.txt
    // 5. Verify history/file visible
}
```
