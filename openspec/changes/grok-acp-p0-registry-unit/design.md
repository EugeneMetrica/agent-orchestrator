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
        SessionMeta:    sessionMeta,
        SessionMode:    sessionMode,
        SessionOptions: sessionOptions,
    }, log)
}
```

### configure Function

Constructs spawn args for `grok --no-auto-update agent [--always-approve] [--model M] stdio`:

```go
func configure(ctx context.Context, cfg acpdriver.LaunchConfig) ([]string, map[string]string, error) {
    // --no-auto-update is a global flag, so it precedes the subcommand exactly
    // as the TUI adapter passes it. Standing instructions are not on the argv:
    // see "System prompt: ACP _meta.rules, not argv --rules" below.
    args := []string{"--no-auto-update", "agent"}

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

### sessionMeta Function

Carries AO's standing instructions in the ACP session metadata Grok's agent mode
reads. Returns `nil` for an empty or whitespace-only prompt so the request has no
`rules` key at all:

```go
func sessionMeta(cfg acpdriver.LaunchConfig) map[string]any {
    prompt := strings.TrimSpace(cfg.SystemPrompt)
    if prompt == "" {
        return nil
    }
    return map[string]any{"rules": prompt}
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
| `TestConfigureNeverPutsRulesOnArgv` | No `--rules` on the argv for any prompt |
| `TestSessionMetaDeliversStandingInstructionsAsRules` | `_meta` carries `rules: <prompt>` |
| `TestSessionMetaTrimsAndOmitsBlankStandingInstructions` | Prompt trimmed; blank yields `nil` meta |
| `TestBindingForwardsSessionMetaToTransport` (`nativeacp`) | `nativeacp.Config.SessionMeta` reaches `acpdriver.Config` |
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
grok --no-auto-update agent [--always-approve] [--model <model>] stdio
```

Key differences:
- `agent` subcommand enables ACP mode
- `stdio` subcommand selects JSON-RPC transport
- `--always-approve` replaces TUI's `--permission-mode bypassPermissions`
- TUI takes AO's standing instructions on the argv as `--rules`; agent mode takes
  them in ACP session metadata instead (see below)

## System prompt: ACP `_meta.rules`, not argv `--rules`

`--rules` is **not** a global flag that agent mode forwards. In grok-build only
the TUI and `-p` paths read it; `grok … agent stdio` accepts it on the command
line and then ignores it. An argv-borne `--rules` therefore looks like delivery
while dropping AO's standing instructions silently — there is no error and no
warning, which is exactly what makes it worth calling out here.

Agent mode takes appended standing instructions through the ACP session's
`_meta`, under the key `rules` (the key name is Grok's, not AO's). Grok folds
that value into the `<human_rules>` section of the system prompt its own
installation configures, so AO's instructions are appended and never replace the
user's.

The shared `acp` transport already sends `Config.SessionMeta` on every request
that creates the provider-side session object — `session/new`, `session/load`,
and `session/resume` — so a recovered conversation receives the standing context
again rather than relying on replayed transcript history.

`nativeacp.Config` did not expose that hook, which is the gap this phase closes.
The addition is a field and one assignment in `buildConfig`; the shared transport
and every other native binding are unchanged, and a binding that sets no
`SessionMeta` still sends no metadata:

```go
// nativeacp.Config
SessionMeta func(acpdriver.LaunchConfig) map[string]any

// nativeacp.buildConfig → acpdriver.Config
SessionMeta: cfg.SessionMeta,
```

The Grok binding supplies it, and `configure` stays free of the prompt:

```go
args := []string{"--no-auto-update", "agent"}
// … --always-approve / --model / stdio only

func sessionMeta(cfg acpdriver.LaunchConfig) map[string]any {
    prompt := strings.TrimSpace(cfg.SystemPrompt)
    if prompt == "" {
        return nil
    }
    return map[string]any{"rules": prompt}
}
```

Returning `nil` for a blank prompt is deliberate: the request then carries no
`rules` key, rather than an empty one Grok would have to interpret.
