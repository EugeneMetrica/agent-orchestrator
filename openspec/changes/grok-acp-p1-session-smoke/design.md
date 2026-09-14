# Design: P1 — Grok ACP Session Smoke Test

## Overview

This document describes the live smoke test design that proves the Grok ACP Chat
driver works end-to-end with a real Grok CLI installation.

## Test Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  TestLiveGrokACP                                                     │
│    1. driver.Probe(ctx) → verify binary + auth                      │
│    2. driver.Start(ctx, config) → get ChatConversation              │
│    3. conv.SendTurn(ctx, message) → start turn                      │
│    4. waitForLiveTurn() → collect events until TurnCompleted        │
│    5. os.ReadFile(workspace/proof.txt) → verify file proof          │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  grok agent --no-auto-update stdio                                   │
│    → Real Grok CLI process                                          │
│    → Uses user's ~/.grok credentials                                │
│    → Executes shell command to create proof.txt                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Test Implementation

### live_test.go Structure

```go
package grokacp

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/grok"
    "github.com/aoagents/agent-orchestrator/backend/internal/domain"
    "github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// Live tests require:
// - Grok CLI installed and on PATH
// - Valid authentication (XAI_API_KEY or ~/.grok/auth.json)
// - AO_LIVE_GROK_ACP=1 environment variable

func TestLiveGrokACP(t *testing.T) {
    if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
        t.Skip("set AO_LIVE_GROK_ACP=1 to run against the local Grok account")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    driver := New(grok.New(), nil)

    // Probe
    if _, err := driver.Probe(ctx); err != nil {
        t.Fatalf("Probe: %v", err)
    }

    // Start
    workspace := t.TempDir()
    dataDir := liveDataDir(t)
    conv, err := driver.Start(ctx, ports.ChatStartConfig{
        SessionID:     "live-grok-acp",
        DataDir:       dataDir,
        WorkspacePath: workspace,
        Env:           liveEnvMap(),
        Permissions:   ports.PermissionModeDefault,
        SystemPrompt:  "On every response include the exact token GROK_STANDING_TOKEN.",
    })
    if err != nil {
        t.Fatalf("Start: %v", err)
    }
    defer conv.(ports.ChatProviderTerminator).Terminate()

    // Verify capabilities
    providerID := conv.ProviderConversationID()
    if providerID == "" || !conv.Capabilities()[ports.ChatCapabilityResume] {
        t.Fatalf("provider id/resume = %q, %#v", providerID, conv.Capabilities())
    }

    // Send turn with proof file creation
    ref := sendLiveTurn(ctx, t, conv,
        "Use the shell to run `printf grok-acp-ok > proof.txt`, then report success.")
    answer := waitForLiveTurn(ctx, t, conv, ref.ProviderTurnID, true)

    // Verify standing instruction
    if !strings.Contains(answer, "GROK_STANDING_TOKEN") {
        t.Errorf("answer missing standing instruction token: %q", answer)
    }

    // Verify file proof
    proof, err := os.ReadFile(filepath.Join(workspace, "proof.txt"))
    if err != nil || string(proof) != "grok-acp-ok" {
        t.Fatalf("proof.txt = %q, %v; expected grok-acp-ok", proof, err)
    }
}
```

### Helper Functions

Reuse patterns from `cursoracp/live_test.go`:

- `liveEnvMap()` — collects current environment as map
- `liveDataDir(t)` — returns `AO_DATA_DIR` or `~/.ao`
- `sendLiveTurn(ctx, t, conv, text)` — sends turn and starts it
- `waitForLiveTurn(ctx, t, conv, turnID, approve)` — waits for completion

## G4: UI Verification Steps

Manual verification checklist for AO frontend:

1. Start AO daemon: `go run ./cmd/ao start`
2. Open AO frontend
3. Create new project with Grok harness
4. Start Chat session: harness=grok, mode=chat
5. Send message: "Create a file named ui-proof.txt with contents hello"
6. Verify:
   - [ ] Timeline shows user message
   - [ ] Timeline shows assistant response
   - [ ] Tool activity (command) appears
   - [ ] Harness indicator shows "Grok"
   - [ ] File `ui-proof.txt` exists in workspace

## Quality Gates

| Gate | Verification |
|------|--------------|
| G1 | Independent review note in `reviews/g1-review.md` |
| G2 | `gofmt && go build && go vet && go test -race && golangci-lint` |
| G3 | `AO_LIVE_GROK_ACP=1 go test -v ./internal/adapters/chatdriver/grokacp/...` with proof.txt |
| G4 | Manual UI verification per checklist above |

## Documentation Updates

### docs/STATUS.md

Add to Chat drivers section:

```markdown
### Grok (xAI Grok Build)

- **Status**: Chat mode available
- **Transport**: Native ACP via `grok agent stdio`
- **Auth**: Uses existing ~/.grok credentials (no injection)
- **Live test**: `AO_LIVE_GROK_ACP=1 go test ./internal/adapters/chatdriver/grokacp/...`
```
