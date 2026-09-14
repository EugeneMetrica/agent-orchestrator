# Design: P0 — Grok ACP Registry & Unit Tests

## Overview

This document describes the technical design for the Grok ACP Chat driver binding
and unit test coverage. P0 establishes the driver scaffolding without live
integration; subsequent phases add live tests.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  registry.Build()                                                    │
│    └── grokacp.New(grok.New(), log)  → HarnessGrok                  │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  nativeacp.New(plugin, config, log)                                  │
│    → Wraps grok.Plugin for binary + auth                            │
│    → Delegates to acpdriver with Grok-specific config               │
└─────────────────────────────────────────────────────────────────────┘
```

## Package Structure

```
backend/internal/adapters/chatdriver/grokacp/
├── driver.go           # nativeacp binding with configure/sessionMode/validate
└── driver_test.go      # Unit tests for binding logic
```

## Component Design

### driver.go

```go
// Package grokacp binds the user's own Grok installation to AO's
// reusable ACP Chat transport.
package grokacp

import (
    "context"
    "fmt"
    "log/slog"
    "strings"

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

Constructs spawn args for `grok agent [--always-approve] [--model M] [--no-auto-update] stdio`:

```go
func configure(ctx context.Context, cfg acpdriver.LaunchConfig) ([]string, map[string]string, error) {
    args := []string{"agent", "--no-auto-update"}

    // Bypass-permissions mode → --always-approve CLI flag
    if ports.NormalizePermissionMode(cfg.Permissions) == ports.PermissionModeBypassPermissions {
        args = append(args, "--always-approve")
    }

    // Model override
    if model := cfg.TurnSettings.Model; model != "" {
        args = append(args, "--model", model)
    }

    args = append(args, "stdio")
    return args, nil, nil
}
```

### sessionMode Function

Maps AO permission modes to Grok ACP session mode strings:

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

Add `domain.HarnessGrok` to `TestShippedChatDrivers`:

```go
for _, harness := range []domain.AgentHarness{
    // ... existing ...
    domain.HarnessGrok,  // ← Add
} {
```

## Test Design

### driver_test.go

Unit tests covering the Grok-specific binding only (not shared nativeacp logic):

| Test | Coverage |
|------|----------|
| `TestHarness` | `driver.Harness() == domain.HarnessGrok` |
| `TestConfigure_DefaultPermissions` | Returns `agent --no-auto-update stdio` |
| `TestConfigure_BypassPermissions` | Includes `--always-approve` |
| `TestConfigure_ModelOverride` | Includes `--model xai/grok-3` |
| `TestSessionMode_Mapping` | Each permission mode maps correctly |
| `TestValidateTurnSettings_ValidModel` | `xai/grok-3` passes |
| `TestValidateTurnSettings_InvalidModel` | `grok-3` fails with correct error |
| `TestValidateTurnSettings_EmptyModel` | Empty string passes |

## Quality Gates

| Gate | Verification |
|------|--------------|
| G1 | Independent review note in `reviews/g1-review.md` |
| G2 | `gofmt -l . && go build ./... && go vet ./... && go test -race ./... && golangci-lint run` |
| G3 | **WAIVED** for P0 (unit tests only) |
| G4 | **WAIVED** for P0 (unit tests only) |

## Dependencies

Uses existing packages only:
- `github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/grok`
- `github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/nativeacp`
- `github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/acp`
- `github.com/aoagents/agent-orchestrator/backend/internal/domain`
- `github.com/aoagents/agent-orchestrator/backend/internal/ports`

## Spawn Command Reference

TUI mode (existing):
```
grok --no-auto-update [--permission-mode <mode>] [--model <model>] [--rules <text>]
```

Chat mode (new ACP):
```
grok agent --no-auto-update [--always-approve] [--model <model>] stdio
```

Key differences:
- `agent` subcommand enables ACP mode
- `stdio` subcommand selects JSON-RPC transport
- `--always-approve` replaces TUI's `--permission-mode bypassPermissions`
- `--rules` not used (system prompt via ACP `session/new` metadata)
