# Chat Drivers Specification

This specification defines the requirements for AO Chat drivers that bind agent
harnesses to the daemon's native Chat protocol.

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

### Requirement: Grok ACP Spawn Command

The driver SHALL spawn Grok using the ACP JSON-RPC stdio subcommand with
appropriate flags for auto-update suppression and permission mode.

#### Scenario: Default permissions spawn command

**GIVEN** a valid `acpdriver.LaunchConfig` with default permissions
**WHEN** the driver's `Launch` callback is invoked
**THEN** the returned command is `<resolved-grok-binary> agent --no-auto-update stdio`

#### Scenario: Bypass permissions spawn command

**GIVEN** a `LaunchConfig` with `Permissions = ports.PermissionModeBypassPermissions`
**WHEN** the driver's `Launch` callback is invoked
**THEN** the command includes `--always-approve` before `stdio`

#### Scenario: Model override in spawn command

**GIVEN** a `LaunchConfig` with model override `xai/grok-3`
**WHEN** the driver's `Launch` callback is invoked
**THEN** the command includes `--model xai/grok-3`

---

### Requirement: Grok Binary Resolution Reuse

The driver SHALL use the existing `grok.Plugin.ResolveBinary` for locating
the installed Grok CLI binary. No fallback download or bundled binary.

#### Scenario: Binary found in search path

**GIVEN** `grok` binary is installed at `/usr/local/bin/grok`
**WHEN** the driver probes availability
**THEN** it uses `/usr/local/bin/grok` as the spawn command

#### Scenario: Binary not found

**GIVEN** `grok` binary is not found in any search path
**WHEN** the driver probes availability
**THEN** it returns `ports.ErrChatDriverUnavailable`

---

### Requirement: Grok Authentication Status Reuse

The driver SHALL use the existing `grok.Plugin.AuthStatus` for checking
user authentication (XAI_API_KEY, ~/.grok/auth.json, ~/.grok/config.toml).

#### Scenario: Auth via environment variable

**GIVEN** `XAI_API_KEY` environment variable is set
**WHEN** the driver probes auth status
**THEN** probe succeeds (no `ErrChatAuthRequired`)

#### Scenario: Auth via config file

**GIVEN** valid API key exists in `~/.grok/config.toml` under `[models.*]`
**WHEN** the driver probes auth status
**THEN** probe succeeds

#### Scenario: Auth via auth.json

**GIVEN** valid tokens exist in `~/.grok/auth.json`
**WHEN** the driver probes auth status
**THEN** probe succeeds

#### Scenario: No authentication found

**GIVEN** no authentication credentials are found
**AND** auth status returns `ports.AgentAuthStatusUnauthorized`
**WHEN** the driver probes availability
**THEN** it returns `ports.ErrChatAuthRequired`

---

### Requirement: No Credential Injection

The driver SHALL NOT inject, rewrite, or isolate the user's Grok credentials.
It operates with the user's existing `~/.grok` / `GROK_HOME` configuration.

#### Scenario: Environment not modified

**GIVEN** a Chat session launch for Grok
**WHEN** the driver constructs environment variables
**THEN** it does NOT set or override `GROK_HOME`
**AND** it does NOT set or override `XAI_API_KEY`
**AND** it does NOT modify any files under `~/.grok`

---

### Requirement: Native ACP Capabilities

The driver SHALL expose the standard native ACP capabilities for Chat mode.

#### Scenario: All capabilities enabled

**GIVEN** a `grokacp` driver instance
**WHEN** capabilities are queried
**THEN** the following capabilities are `true`:
  - `ChatCapabilityStreaming`
  - `ChatCapabilityTools`
  - `ChatCapabilityApprovals`
  - `ChatCapabilityInterrupt`
  - `ChatCapabilityResume`

---

### Requirement: Permission Mode Mapping

The driver SHALL map AO's `PermissionMode` to Grok's ACP session mode.

#### Scenario: Default mode

**GIVEN** `PermissionModeDefault`
**WHEN** session mode is computed
**THEN** no explicit mode is set (Grok uses its config default)

#### Scenario: Accept edits mode

**GIVEN** `PermissionModeAcceptEdits`
**WHEN** session mode is computed
**THEN** mode `"acceptEdits"` is passed to ACP session/new

#### Scenario: Auto mode

**GIVEN** `PermissionModeAuto`
**WHEN** session mode is computed
**THEN** mode `"auto"` is passed to ACP session/new

#### Scenario: Bypass permissions mode

**GIVEN** `PermissionModeBypassPermissions`
**WHEN** session mode is computed
**THEN** mode `"bypassPermissions"` is passed to ACP session/new

---

### Requirement: Model Override Validation

The driver SHALL validate model overrides to ensure they use the correct format.

#### Scenario: Valid model format

**GIVEN** a model override `"xai/grok-3"`
**WHEN** turn settings are validated
**THEN** validation succeeds

#### Scenario: Invalid model format

**GIVEN** a model override `"grok-3"` (missing provider)
**WHEN** turn settings are validated
**THEN** validation fails with `ports.ErrChatConfigOptionInvalid`

#### Scenario: Empty model allowed

**GIVEN** an empty model override
**WHEN** turn settings are validated
**THEN** validation succeeds (uses agent default)

---

### Requirement: Harness Identity

The driver SHALL return `domain.HarnessGrok` as its harness identifier.

#### Scenario: Harness method returns correct value

**GIVEN** a `grokacp` driver instance
**WHEN** `driver.Harness()` is called
**THEN** it returns `domain.HarnessGrok` (`"grok"`)

---

### Requirement: TUI Mode Unchanged

The implementation SHALL NOT alter existing TUI-mode behavior for Grok sessions.

#### Scenario: TUI sessions unaffected

**GIVEN** a TUI-mode session with `harness=grok`
**WHEN** the session is spawned
**THEN** it uses the existing `adapters/agent/grok` launch command
**AND** hooks are installed via Claude Code compatibility layer
**AND** terminal prompt delivery works as before
