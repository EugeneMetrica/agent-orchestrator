# chat-drivers Specification

## Purpose

How AO binds a user-installed coding agent to Chat mode, and what a binding owes
its callers. A Chat driver is thin: AO resolves and launches the user's own CLI,
speaks the agent's protocol (ACP for the native bindings), and maps that protocol
onto AO's vocabulary for turns, tools, approvals, models, attachments, and
resume. It does not bundle, download, or substitute a provider CLI, and it does
not own login, settings, or project instructions.

The requirements below are the accumulated deltas of the Grok ACP phases
(P0–P4b), so they are stated in Grok's terms while describing the binding
contract every native ACP harness shares: registration and identity, spawn
command construction, standing instructions through session metadata, permission
mode mapping, model-override forwarding, resume, and the API and UI E2E
guarantees a binding has to satisfy. The archived changes under
`openspec/changes/archive/` carry the reasoning and the live evidence behind
each.

## Requirements

### Requirement: Grok Chat Driver Registration

The `grokacp` Chat driver SHALL be registered in `chatdriver/registry.Build`
so that the daemon can resolve it for Chat-mode sessions with `harness=grok`.

#### Scenario: Registry supports Grok chat

**GIVEN** the daemon registry is built via `registry.Build(log)`
**WHEN** `registry.SupportsChat(domain.HarnessGrok)` is called
**THEN** it returns `true`

#### Scenario: Driver resolution succeeds

**GIVEN** the daemon registry is built via `registry.Build(log)`
**WHEN** `registry.Driver(domain.HarnessGrok)` is called
**THEN** it returns a non-nil `ports.ChatDriver` without error

---

### Requirement: Harness Identity

The driver SHALL return `domain.HarnessGrok` as its harness identifier.

#### Scenario: Harness method returns correct value

**GIVEN** a `grokacp` driver instance
**WHEN** `driver.Harness()` is called
**THEN** it returns `domain.HarnessGrok` (`"grok"`)

---

### Requirement: Grok ACP Spawn Command Construction

The driver SHALL construct the ACP spawn command with the global
`--no-auto-update` flag (in the same position the TUI adapter uses), the `grok
agent` subcommand, appropriate flags, and the `stdio` transport selector.

#### Scenario: Default permissions spawn command

**GIVEN** a valid `acpdriver.LaunchConfig` with default permissions
**WHEN** the driver's `configure()` callback is invoked
**THEN** the returned args include `["--no-auto-update", "agent", "stdio"]`
**AND** no `--always-approve` flag is present

#### Scenario: Bypass permissions spawn command

**GIVEN** a `LaunchConfig` with `Permissions = ports.PermissionModeBypassPermissions`
**WHEN** the driver's `configure()` callback is invoked
**THEN** the args include `--always-approve` before `stdio`

#### Scenario: Accept-edits permissions spawn command

**GIVEN** a `LaunchConfig` with `Permissions = ports.PermissionModeAcceptEdits`
**WHEN** the driver's `configure()` callback is invoked
**THEN** no `--always-approve` flag is present (Grok ACP handles mode via session)

#### Scenario: Auto permissions spawn command

**GIVEN** a `LaunchConfig` with `Permissions = ports.PermissionModeAuto`
**WHEN** the driver's `configure()` callback is invoked
**THEN** no `--always-approve` flag is present (Grok ACP handles mode via session)

#### Scenario: Model override in spawn command

**GIVEN** a `LaunchConfig` with `Model = "grok-code-fast"`
**WHEN** the driver's `configure()` callback is invoked
**THEN** the args include `["--model", "grok-code-fast"]` before `stdio`

---

### Requirement: Standing Instructions Delivery

The driver SHALL deliver AO's standing instructions through ACP session
metadata, under the key `rules`, so Grok's agent mode folds them into the
`<human_rules>` section of the system prompt the user's own installation
configures — adding to it and never replacing it.

The driver SHALL NOT place standing instructions on the argv. `--rules` is read
only by Grok's TUI and `-p` paths; `grok … agent stdio` accepts the flag and
ignores it, so an argv-borne prompt is dropped with no error or warning.

#### Scenario: System prompt is delivered as session metadata

**GIVEN** a `LaunchConfig` with a non-empty `SystemPrompt`
**WHEN** the driver's `sessionMeta()` callback is invoked
**THEN** it returns metadata whose `rules` key carries the trimmed system prompt

#### Scenario: Standing instructions never reach the argv

**GIVEN** a `LaunchConfig` with any `SystemPrompt`, empty or not
**WHEN** the driver's `configure()` callback is invoked
**THEN** no `--rules` flag is present
**AND** the system prompt text does not appear in the args

#### Scenario: Empty system prompt sends no rules key

**GIVEN** a `LaunchConfig` with an empty or blank `SystemPrompt`
**WHEN** the driver's `sessionMeta()` callback is invoked
**THEN** it returns `nil`, so the ACP request carries no `rules` key

---

### Requirement: Native ACP Session Metadata Pass-Through

`nativeacp.Config` SHALL expose a `SessionMeta` hook and forward it verbatim to
`acpdriver.Config.SessionMeta`, so a native binding can reach the ACP `_meta`
channel the shared transport already sends on `session/new`, `session/load`, and
`session/resume`. Without the forward, a binding can configure a metadata-based
prompt channel that is silently never sent.

The hook SHALL be a pass-through only: `nativeacp` SHALL NOT synthesize,
inspect, or default the metadata.

#### Scenario: Binding hook reaches the transport

**GIVEN** a `nativeacp.Config` with a `SessionMeta` function
**WHEN** the native ACP binding is built
**THEN** `acpdriver.Config.SessionMeta` is that function's value unchanged

#### Scenario: Bindings without a hook stay metadata-free

**GIVEN** a `nativeacp.Config` with no `SessionMeta` function
**WHEN** the native ACP binding is built
**THEN** `acpdriver.Config.SessionMeta` is nil and no metadata is sent

---

### Requirement: Model Override Forwarding

The driver SHALL forward a model override verbatim and SHALL NOT impose an id
format or keep a Grok model list of its own. AO's Grok model ids come from the
installation itself — `grok models`, parsed by `modelcatalog.parseGrokModels`,
the same ids the TUI adapter passes to `--model` — and are bare
(`grok-code-fast`, `grok-4.5`). Availability is decided by the models the ACP
session advertises, not by AO.

#### Scenario: Bare model id is accepted

**GIVEN** a model override `"grok-code-fast"`
**WHEN** the driver starts or resumes a conversation
**THEN** AO does not reject it with `ports.ErrChatConfigOptionInvalid`
**AND** the id reaches the launch as `["--model", "grok-code-fast"]` and the
`model` session option

#### Scenario: Advertised model id is forwarded unchanged

**GIVEN** a model override `"grok-4.5"` as reported by `grok models`
**WHEN** `sessionOptions()` is called
**THEN** it returns one `model` option carrying the id unchanged

#### Scenario: Empty model adds no option

**GIVEN** an empty or blank model override
**WHEN** `sessionOptions()` is called
**THEN** it returns no options (uses agent default)

---

### Requirement: Session Mode Mapping

The driver SHALL map AO permission modes to Grok ACP session mode strings.

#### Scenario: Default permission mode

**GIVEN** `ports.PermissionModeDefault`
**WHEN** `sessionMode()` is called
**THEN** it returns `""` (no explicit mode; Grok uses config default)

#### Scenario: Accept-edits permission mode

**GIVEN** `ports.PermissionModeAcceptEdits`
**WHEN** `sessionMode()` is called
**THEN** it returns `"acceptEdits"`

#### Scenario: Auto permission mode

**GIVEN** `ports.PermissionModeAuto`
**WHEN** `sessionMode()` is called
**THEN** it returns `"auto"`

#### Scenario: Bypass-permissions mode

**GIVEN** `ports.PermissionModeBypassPermissions`
**WHEN** `sessionMode()` is called
**THEN** it returns `"bypassPermissions"`

---

### Requirement: Binary Resolution Delegation

The driver SHALL delegate binary resolution to the existing `grok.Plugin`.

#### Scenario: Plugin ResolveBinary used

**GIVEN** a probe or launch request
**WHEN** the driver needs the Grok binary path
**THEN** it calls `plugin.ResolveBinary(ctx)` from `grok.Plugin`
**AND** does NOT implement its own binary search

---

### Requirement: Auth Status Delegation

The driver SHALL delegate authentication checking to the existing `grok.Plugin`.

#### Scenario: Plugin AuthStatus used

**GIVEN** a probe request
**WHEN** the driver checks authentication
**THEN** it calls `plugin.AuthStatus(ctx)` from `grok.Plugin`
**AND** returns `ports.ErrChatAuthRequired` when status is `Unauthorized`

---

### Requirement: No Credential Injection

The driver SHALL NOT inject, modify, or isolate Grok credentials.

#### Scenario: Environment not modified for credentials

**GIVEN** any Chat session configuration
**WHEN** the driver constructs the launch environment
**THEN** it does NOT set or override `GROK_HOME`
**AND** it does NOT set or override `XAI_API_KEY`

---

### Requirement: Native ACP Capabilities

The driver SHALL expose standard native ACP capabilities.

#### Scenario: Standard capabilities enabled

**GIVEN** a `grokacp` driver instance built via `nativeacp.New`
**WHEN** capabilities are queried
**THEN** the following are `true`:
  - `ChatCapabilityStreaming`
  - `ChatCapabilityTools`
  - `ChatCapabilityApprovals`
  - `ChatCapabilityInterrupt`
  - `ChatCapabilityResume`

---

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

### Requirement: Default Permission Mode

The driver SHALL support default permission mode where agent prompts for approvals.

#### Scenario: Default mode file creation

**GIVEN** a Chat session with `Permissions = ports.PermissionModeDefault`
**AND** `AO_LIVE_GROK_ACP=1` is set
**WHEN** a turn is sent asking to create `mode-default.txt`
**AND** tool approvals are granted
**THEN** the file `<workspace>/mode-default.txt` exists with expected content

---

### Requirement: Accept-Edits Permission Mode

The driver SHALL support accept-edits mode where file edits are auto-approved.

#### Scenario: Accept-edits mode file creation

**GIVEN** a Chat session with `Permissions = ports.PermissionModeAcceptEdits`
**AND** `AO_LIVE_GROK_ACP=1` is set
**WHEN** a turn is sent asking to create `mode-accept-edits.txt` using file edit
**THEN** the file is created without manual approval prompt
**AND** `<workspace>/mode-accept-edits.txt` exists

---

### Requirement: Auto Permission Mode

The driver SHALL support auto mode where all tools are auto-approved.

#### Scenario: Auto mode file creation

**GIVEN** a Chat session with `Permissions = ports.PermissionModeAuto`
**AND** `AO_LIVE_GROK_ACP=1` is set
**WHEN** a turn is sent asking to create `mode-auto.txt`
**THEN** the file is created without manual approval prompt
**AND** `<workspace>/mode-auto.txt` exists

---

### Requirement: Bypass-Permissions Mode

The driver SHALL support bypass-permissions mode with `--always-approve` flag.

#### Scenario: Bypass mode file creation

**GIVEN** a Chat session with `Permissions = ports.PermissionModeBypassPermissions`
**AND** `AO_LIVE_GROK_ACP=1` is set
**WHEN** a turn is sent asking to create `mode-bypass.txt`
**THEN** the file is created without any approval prompt
**AND** `<workspace>/mode-bypass.txt` exists
**AND** the spawn command included `--always-approve`

---

### Requirement: Session Mode String Mapping

The driver SHALL pass correct mode strings to ACP session/new.

#### Scenario: Mode strings in ACP session

**GIVEN** permission mode is set at session start
**WHEN** the ACP session/new request is sent
**THEN** the `mode` field contains:
  - `""` (empty) for default
  - `"acceptEdits"` for accept-edits
  - `"auto"` for auto
  - `"bypassPermissions"` for bypass-permissions

---

### Requirement: Turn Settings Override

The driver SHALL support per-turn permission mode changes via TurnSettings.

#### Scenario: Turn-level approval mode

**GIVEN** a session started with `PermissionModeDefault`
**WHEN** a turn is sent with `Settings.Approval = ports.PermissionModeAuto`
**THEN** that turn uses auto-approve behavior
**AND** subsequent turns revert to session default

---

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
