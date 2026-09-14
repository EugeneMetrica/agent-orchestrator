# Design: Grok ACP Chat Driver

## Overview

This document describes the technical design for adding a native ACP Chat driver
for the Grok (xAI Grok Build) harness. The design follows the established pattern
used by `opencodeacp`, `droidacp`, `ompacp`, and other native ACP drivers.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         AO Daemon                                    │
├─────────────────────────────────────────────────────────────────────┤
│  registry.Build()                                                    │
│    ├── codexappserver.New(...)   → HarnessCodex                     │
│    ├── claudeacp.New(...)        → HarnessClaudeCode                │
│    ├── cursoracp.New(...)        → HarnessCursor                    │
│    ├── opencodeacp.New(...)      → HarnessOpenCode                  │
│    ├── droidacp.New(...)         → HarnessDroid                     │
│    ├── ...                                                          │
│    └── grokacp.New(...)          → HarnessGrok  ← NEW               │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  nativeacp.New(plugin, config, log)                                  │
│    → Wraps grok.Plugin (existing agent adapter)                     │
│    → Builds acpdriver.Config with:                                  │
│        - Probe (binary + auth check)                                │
│        - Launch (spawn command construction)                        │
│        - SessionMode (permission mapping)                           │
│        - SessionOptions (model override)                            │
│        - ValidateTurnSettings (model format check)                  │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  acpdriver.Config                                                    │
│    → Common ACP transport layer                                     │
│    → JSON-RPC over stdio                                            │
│    → Persistent host for daemon replacement                         │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  grok agent --no-auto-update [--always-approve] [--model M] stdio   │
│    → User's installed Grok CLI                                      │
│    → ACP JSON-RPC protocol                                          │
│    → Uses user's ~/.grok credentials                                │
└─────────────────────────────────────────────────────────────────────┘
```

## Package Structure

```
backend/internal/adapters/chatdriver/grokacp/
├── driver.go           # Main driver implementation
├── driver_test.go      # Unit tests
└── live_test.go        # Optional live integration tests (gated)
```

## Component Design

### driver.go

The driver is a thin wrapper around `nativeacp.New`, following the exact pattern
of `opencodeacp`.

```go
// Package grokacp binds the user's own Grok installation to AO's
// reusable ACP Chat transport.
package grokacp

import (
    "context"
    "fmt"
    "log/slog"
    "strings"

    "github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/grok"
    acpdriver "github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/acp"
    "github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/nativeacp"
    "github.com/aoagents/agent-orchestrator/backend/internal/domain"
    "github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// New launches `grok agent stdio` from the exact binary resolved by the existing
// Grok agent plugin. AO adds only per-session permission mode and model override.
func New(plugin nativeacp.Plugin, log *slog.Logger) ports.ChatDriver {
    return nativeacp.New(plugin, nativeacp.Config{
        Harness:              domain.HarnessGrok,
        Configure:            configure,
        SessionMode:          sessionMode,
        SessionOptions:       sessionOptions,
        ValidateTurnSettings: validateTurnSettings,
    }, log)
}
```

### configure Function

Constructs the spawn arguments for `grok agent stdio`:

```go
func configure(ctx context.Context, cfg acpdriver.LaunchConfig) ([]string, map[string]string, error) {
    args := []string{"agent", "--no-auto-update"}

    // Permission mode → --always-approve flag
    if ports.NormalizePermissionMode(cfg.Permissions) == ports.PermissionModeBypassPermissions {
        args = append(args, "--always-approve")
    }

    // Model override
    if model := cfg.TurnSettings.Model; model != "" {
        args = append(args, "--model", model)
    }

    // Final subcommand for ACP transport
    args = append(args, "stdio")

    return args, nil, nil
}
```

### sessionMode Function

Maps AO permission modes to Grok ACP session modes:

```go
func sessionMode(permissions ports.PermissionMode) string {
    switch ports.NormalizePermissionMode(permissions) {
    case ports.PermissionModeAcceptEdits:
        return "acceptEdits"
    case ports.PermissionModeAuto:
        return "auto"
    case ports.PermissionModeBypassPermissions:
        return "bypassPermissions"
    default:
        return "" // Grok uses its config default
    }
}
```

### sessionOptions Function

Passes model override to ACP session:

```go
func sessionOptions(settings ports.ChatTurnSettings) []acpdriver.SessionOption {
    if settings.Model == "" {
        return nil
    }
    return []acpdriver.SessionOption{{ID: "model", Value: settings.Model}}
}
```

### validateTurnSettings Function

Validates model format (must be `provider/model`):

```go
func validateTurnSettings(_ ports.PermissionMode, settings ports.ChatTurnSettings) error {
    if settings.Model == "" {
        return nil
    }
    provider, model, found := strings.Cut(settings.Model, "/")
    if !found || strings.TrimSpace(provider) == "" || strings.TrimSpace(model) == "" {
        return fmt.Errorf("%w: Grok model %q must use provider/model format "+
            "(for example, xai/grok-3); select a full model ID from `grok models`, "+
            "or clear the model override to use Agent default",
            ports.ErrChatConfigOptionInvalid, settings.Model)
    }
    return nil
}
```

## Registry Integration

### registry.go Changes

```go
import (
    // ... existing imports ...
    "github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/grok"
    "github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/grokacp"
)

func Build(log *slog.Logger) *Registry {
    return New(
        // ... existing drivers ...
        grokacp.New(grok.New(), log),
    )
}
```

### registry_test.go Changes

Add `domain.HarnessGrok` to the expected Chat drivers list:

```go
for _, harness := range []domain.AgentHarness{
    domain.HarnessCodex,
    domain.HarnessClaudeCode,
    domain.HarnessOpenCode,
    domain.HarnessDroid,
    domain.HarnessKimi,
    domain.HarnessKimchi,
    domain.HarnessPi,
    domain.HarnessCursor,
    domain.HarnessOMP,
    domain.HarnessGrok,  // ← Add
} {
```

## Test Design

### driver_test.go

Unit tests covering:

1. **Harness identity**: `driver.Harness() == domain.HarnessGrok`
2. **Configure function**:
   - Default permissions → `agent --no-auto-update stdio`
   - Bypass permissions → includes `--always-approve`
   - Model override → includes `--model xai/grok-3`
3. **Session mode mapping**:
   - Each permission mode maps to correct Grok mode string
4. **Turn settings validation**:
   - Valid `provider/model` passes
   - Missing provider fails with `ErrChatConfigOptionInvalid`
   - Empty model passes

### live_test.go (Optional)

Integration tests gated behind `AO_LIVE_GROK_ACP=1`:

1. Full spawn/message/response cycle
2. Interrupt handling
3. Resume from provider conversation ID

## Quality Gates (AO Canon)

Per AGENTS.md and docs/development.md:

1. **CI Checks**:
   - `gofmt` — code formatted
   - `go build ./...` — compiles
   - `go vet ./...` — passes vet
   - `go test -race ./...` — no race conditions
   - `golangci-lint run` — passes lint (v2.12.2)
   - `npm run lint` — full lint suite

2. **Package Discipline**:
   - `grokacp` imports only from:
     - `adapters/agent/grok` (existing plugin)
     - `adapters/chatdriver/acp` (shared ACP transport)
     - `adapters/chatdriver/nativeacp` (native binding wrapper)
     - `domain`, `ports` (shared vocabulary)
   - No direct SQLite, runtime, or workspace imports

3. **Error Types**:
   - `ports.ErrChatDriverUnavailable` for binary not found
   - `ports.ErrChatAuthRequired` for authentication failure
   - `ports.ErrChatConfigOptionInvalid` for invalid model format

4. **Context Usage**:
   - All I/O functions accept `context.Context` as first argument

## Dependencies

No new external dependencies. Uses existing:

- `github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/grok`
- `github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/nativeacp`
- `github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/acp`
- `github.com/aoagents/agent-orchestrator/backend/internal/domain`
- `github.com/aoagents/agent-orchestrator/backend/internal/ports`

## Security Considerations

1. **No credential handling**: The driver does not read, write, or transmit
   any credentials. Authentication is entirely delegated to the user's Grok
   installation and the existing `grok.Plugin.AuthStatus` check.

2. **No privilege escalation**: The Grok subprocess runs with the same
   privileges as the daemon.

3. **No network exposure**: All communication is over stdio to a local subprocess.

## Rollback Plan

If issues are discovered post-merge:

1. Remove `grokacp.New(grok.New(), log)` from `registry.Build`
2. Remove `domain.HarnessGrok` from `TestShippedChatDrivers`
3. Optionally delete `adapters/chatdriver/grokacp/` package

TUI mode for Grok remains unaffected.
