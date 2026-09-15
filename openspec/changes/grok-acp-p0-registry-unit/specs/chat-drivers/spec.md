# Specification: Grok ACP Driver Binding (P0)

This specification defines the unit-testable requirements for the Grok ACP Chat
driver binding to `nativeacp`.

## ADDED Requirements

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

## Test Files

- `backend/internal/adapters/chatdriver/grokacp/driver_test.go` — unit tests
- `backend/internal/adapters/chatdriver/nativeacp/driver_test.go` — `SessionMeta` pass-through test
- `backend/internal/adapters/chatdriver/registry/registry_test.go` — registration test update
