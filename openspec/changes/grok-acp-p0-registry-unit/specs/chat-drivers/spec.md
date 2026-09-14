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

### Requirement: Model Override Forwarding

The driver SHALL forward a model override verbatim and SHALL NOT impose an id
format. Grok model ids in the AO catalog and the TUI adapter are bare
(`grok-code-fast`, `grok-4.5`), so availability is decided by the models the ACP
session advertises, not by AO.

#### Scenario: Bare model id is accepted

**GIVEN** a model override `"grok-code-fast"`
**WHEN** the driver starts or resumes a conversation
**THEN** AO does not reject it with `ports.ErrChatConfigOptionInvalid`
**AND** the id reaches the launch as `["--model", "grok-code-fast"]` and the
`model` session option

#### Scenario: Qualified model id is accepted

**GIVEN** a model override `"xai/grok-code-fast-1"`
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
- `backend/internal/adapters/chatdriver/registry/registry_test.go` — registration test update
