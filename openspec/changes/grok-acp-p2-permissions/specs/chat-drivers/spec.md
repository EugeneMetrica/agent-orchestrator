# Specification: Grok ACP Permission Mode Matrix (P2)

This specification defines the live permission mode requirements for the Grok
ACP Chat driver.

## ADDED Requirements

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

## Test File

- `backend/internal/adapters/chatdriver/grokacp/live_test.go`

## Test Function

```go
func TestLiveGrokACPPermissionModes(t *testing.T) {
    if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
        t.Skip("set AO_LIVE_GROK_ACP=1 to run against the local Grok account")
    }
    for _, test := range []struct {
        name string
        mode ports.PermissionMode
    }{
        {name: "default", mode: ports.PermissionModeDefault},
        {name: "accept-edits", mode: ports.PermissionModeAcceptEdits},
        {name: "auto", mode: ports.PermissionModeAuto},
        {name: "bypass-permissions", mode: ports.PermissionModeBypassPermissions},
    } {
        t.Run(test.name, func(t *testing.T) {
            // ... test implementation per design.md
        })
    }
}
```
