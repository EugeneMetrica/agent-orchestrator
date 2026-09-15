# Design: P2 — Grok ACP Permission Mode Matrix

## Overview

This document describes the live permission mode matrix test design for validating
all four AO permission modes work correctly with Grok ACP.

## Permission Mode Mapping

| AO Permission Mode | Grok CLI Flag | ACP Session Mode | Behavior |
|--------------------|---------------|------------------|----------|
| default | (none) | `""` | Agent prompts for tool approvals |
| accept-edits | (none) | `"acceptEdits"` | Auto-approve file edits only |
| auto | (none) | `"auto"` | Auto-approve all tools |
| bypass-permissions | `--always-approve` | `"bypassPermissions"` | Skip all approval UI |

## Test Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  TestLiveGrokACPPermissionModes                                      │
│    for each mode in [default, accept-edits, auto, bypass]:          │
│      1. Start session with mode                                      │
│      2. Send turn: create mode-<name>.txt                           │
│      3. Auto-approve if needed (default mode)                       │
│      4. Wait for completion                                          │
│      5. Verify mode-<name>.txt exists with correct content          │
└─────────────────────────────────────────────────────────────────────┘
```

## Test Implementation

### TestLiveGrokACPPermissionModes

```go
func TestLiveGrokACPPermissionModes(t *testing.T) {
    if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
        t.Skip("set AO_LIVE_GROK_ACP=1 to run against the local Grok account")
    }
    dataDir := liveDataDir(t)
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
            ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
            defer cancel()
            workspace := t.TempDir()
            driver := New(grok.New(), nil)
            conv, err := driver.Start(ctx, ports.ChatStartConfig{
                SessionID:     domain.SessionID("live-grok-mode-" + test.name),
                DataDir:       dataDir,
                WorkspacePath: workspace,
                Env:           liveEnvMap(),
                Permissions:   test.mode,
            })
            if err != nil {
                t.Fatalf("Start(%s): %v", test.mode, err)
            }
            defer conv.(ports.ChatProviderTerminator).Terminate()

            // Use file editing tool (not shell) to test edit-specific modes
            ref := sendLiveTurnWithSettings(ctx, t, conv,
                "Use the file editing tool (not the shell) to create mode.txt containing ok, then say done.",
                ports.ChatTurnSettings{Approval: test.mode})

            // default mode may need approval; others should auto-approve
            waitForLiveTurn(ctx, t, conv, ref.ProviderTurnID, true, false)

            content, err := os.ReadFile(filepath.Join(workspace, "mode.txt"))
            if err != nil || strings.TrimSpace(string(content)) != "ok" {
                t.Fatalf("mode %s proof = %q, %v", test.mode, content, err)
            }
        })
    }
}
```

## Quality Gates

| Gate | Verification |
|------|--------------|
| G1 | Independent review note in `reviews/g1-review.md` |
| G2 | `gofmt && go build && go vet && go test -race && golangci-lint` |
| G3 | `AO_LIVE_GROK_ACP=1 go test -v -run PermissionModes ./internal/adapters/chatdriver/grokacp/...` |
| G4 | Manual UI verification: mode selector affects session behavior |

## G4 UI Verification Steps

1. Start AO daemon and frontend
2. Create Chat session with Grok harness
3. Select each permission mode from UI dropdown
4. Verify mode is reflected in session config
5. Send task that requires tool approval
6. Verify approval behavior matches mode:
   - default: approval prompt appears
   - accept-edits: file edits auto-approve, others prompt
   - auto: all tools auto-approve
   - bypass-permissions: no prompts at all
