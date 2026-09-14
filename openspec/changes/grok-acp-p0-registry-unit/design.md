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
        Harness:        domain.HarnessGrok,
        Configure:      configure,
        SessionMode:    sessionMode,
        SessionOptions: sessionOptions,
    }, log)
}
```

### configure Function

Constructs spawn args for `grok --no-auto-update [--rules TEXT] agent [--always-approve] [--model M] stdio`:

```go
func configure(ctx context.Context, cfg acpdriver.LaunchConfig) ([]string, map[string]string, error) {
    // --no-auto-update and --rules are global flags, so they precede the
    // subcommand exactly as the TUI adapter passes them.
    args := []string{"--no-auto-update"}

    // AO's standing instructions, appended to Grok's own system prompt
    if prompt := strings.TrimSpace(cfg.SystemPrompt); prompt != "" {
        args = append(args, "--rules", prompt)
    }
    args = append(args, "agent")

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

### Model IDs: no format validation

The binding installs no `ValidateTurnSettings` model gate and keeps no Grok model
list. AO already learns Grok's models from the installation: `modelcatalog` runs
`grok models` and `parseGrokModels` yields bare ids (`grok-code-fast`,
`grok-4.5`), which is also what the TUI adapter passes to `--model`. A
`provider/model` requirement would reject every one of them. The id is forwarded
verbatim through `sessionOptions`, and the shared `acp` transport rejects ids the
ACP session does not advertise:

```go
func sessionOptions(settings ports.ChatTurnSettings) []acpdriver.SessionOption {
    if model := strings.TrimSpace(settings.Model); model != "" {
        return []acpdriver.SessionOption{{ID: "model", Value: model}}
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
| `TestConfigure_DefaultPermissions` | Returns `--no-auto-update agent stdio` |
| `TestConfigure_BypassPermissions` | Includes `--always-approve` |
| `TestConfigure_ModelOverride` | Includes `--model grok-code-fast` |
| `TestConfigureAppendsStandingInstructionsAsRules` | `--rules <prompt>` precedes `agent` |
| `TestConfigureOmitsRulesWithoutStandingInstructions` | No `--rules` when the prompt is empty |
| `TestSessionMode_Mapping` | Each permission mode maps correctly |
| `TestSessionOptions_ModelForwarded` | Bare and qualified ids forward verbatim |
| `TestSessionOptions_EmptyModel` | Empty/blank model yields no option |
| `TestBareModelReachesLaunch` | `grok-code-fast` is not rejected by AO |

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
grok --no-auto-update [--rules <text>] agent [--always-approve] [--model <model>] stdio
```

Key differences:
- `agent` subcommand enables ACP mode
- `stdio` subcommand selects JSON-RPC transport
- `--always-approve` replaces TUI's `--permission-mode bypassPermissions`
- `--rules` carries AO's standing instructions in both modes

## System prompt: `--rules`, not ACP session metadata

P0 delivers the system prompt exactly the way the TUI adapter does: as
`--rules <text>`, appended to the argv when `acpdriver.LaunchConfig.SystemPrompt`
is non-empty. Grok treats `--rules` as an addition to the system prompt its own
installation configures, so AO's standing instructions never replace the user's.

`--rules` is a top-level flag, so it keeps `--no-auto-update`'s position ahead of
the `agent` subcommand:

```go
args := []string{"--no-auto-update"}
if prompt := strings.TrimSpace(cfg.SystemPrompt); prompt != "" {
    args = append(args, "--rules", prompt)
}
args = append(args, "agent")
```

There is **no** system prompt delivery through ACP `session/new` metadata in P0.
The shared `acp` transport does have a `SessionMeta` hook, but `nativeacp.Config`
— the layer this binding is built on — exposes none, so nothing this driver does
reaches the `Meta` field of the `session/new` request. Any future metadata-based
channel would need a `SessionMeta` hook on `nativeacp.Config` and a Grok-side
shape confirmed against a real `grok agent stdio` session; that is deferred to a
later phase and is not part of P0.
