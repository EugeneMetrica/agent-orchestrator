# Design: P3 — Grok ACP Resume & Persistent Host

## Overview

This document describes the Resume capability and optional persistent-host
integration design for Grok ACP sessions.

## Resume Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│  TestLiveGrokACPResume                                               │
│                                                                      │
│  PHASE 1: Initial Session                                           │
│    1. driver.Start() → conv1                                        │
│    2. providerID = conv1.ProviderConversationID()                   │
│    3. Send turn: create before.txt, remember ALPHA                  │
│    4. Verify before.txt exists                                      │
│    5. conv1.Terminate()                                              │
│                                                                      │
│  PHASE 2: Resumed Session                                           │
│    6. driver.Resume(providerID) → conv2                             │
│    7. Send turn: what was codeword? read before.txt                 │
│    8. Verify response contains ALPHA                                │
│    9. Verify before.txt still exists                                │
└─────────────────────────────────────────────────────────────────────┘
```

## Test Implementation

### TestLiveGrokACPResume

```go
func TestLiveGrokACPResume(t *testing.T) {
    if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
        t.Skip("set AO_LIVE_GROK_ACP=1 to run against the local Grok account")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    workspace := t.TempDir()
    dataDir := liveDataDir(t)
    driver := New(grok.New(), nil)

    // Phase 1: Initial session
    conv, err := driver.Start(ctx, ports.ChatStartConfig{
        SessionID:     "live-grok-resume",
        DataDir:       dataDir,
        WorkspacePath: workspace,
        Env:           liveEnvMap(),
        Permissions:   ports.PermissionModeDefault,
        SystemPrompt:  "Include START_TOKEN in responses.",
    })
    if err != nil {
        t.Fatalf("Start: %v", err)
    }
    providerID := conv.ProviderConversationID()
    if providerID == "" {
        t.Fatal("no provider conversation ID")
    }

    // Create file and establish memory
    ref := sendLiveTurn(ctx, t, conv,
        "Create before.txt with content 'before-value' and remember the codeword ALPHA.")
    answer := waitForLiveTurn(ctx, t, conv, ref.ProviderTurnID, true)
    if !strings.Contains(answer, "START_TOKEN") {
        t.Errorf("start answer missing token: %q", answer)
    }

    // Verify file created
    beforeContent, err := os.ReadFile(filepath.Join(workspace, "before.txt"))
    if err != nil {
        t.Fatalf("before.txt not created: %v", err)
    }

    // Terminate
    if err := conv.(ports.ChatProviderTerminator).Terminate(); err != nil {
        t.Fatalf("Terminate: %v", err)
    }

    // Phase 2: Resume
    resumed, err := driver.Resume(ctx, ports.ChatResumeConfig{
        SessionID:              "live-grok-resume",
        ProviderConversationID: providerID,
        DataDir:                dataDir,
        WorkspacePath:          workspace,
        Env:                    liveEnvMap(),
        Permissions:            ports.PermissionModeDefault,
        SystemPrompt:           "Include RESUME_TOKEN in responses.",
    })
    if err != nil {
        t.Fatalf("Resume: %v", err)
    }
    defer resumed.(ports.ChatProviderTerminator).Terminate()

    // Query history and file
    resumeRef := sendLiveTurn(ctx, t, resumed,
        "What was the codeword I told you? Also confirm before.txt exists.")
    resumeAnswer := waitForLiveTurn(ctx, t, resumed, resumeRef.ProviderTurnID, true)

    // Verify history persisted
    if !strings.Contains(resumeAnswer, "ALPHA") {
        t.Errorf("resumed answer missing codeword: %q", resumeAnswer)
    }
    if !strings.Contains(resumeAnswer, "RESUME_TOKEN") {
        t.Errorf("resumed answer missing new standing token: %q", resumeAnswer)
    }
    if !strings.Contains(resumeAnswer, "before.txt") {
        t.Errorf("resumed answer doesn't reference file: %q", resumeAnswer)
    }

    // File still exists
    afterContent, err := os.ReadFile(filepath.Join(workspace, "before.txt"))
    if err != nil || string(afterContent) != string(beforeContent) {
        t.Errorf("file changed after resume: before=%q, after=%q, err=%v",
            beforeContent, afterContent, err)
    }
}
```

## SessionMeta Decision

Grok's ACP may require `_meta` fields for standing instructions:

| Field | Purpose | Grok Behavior |
|-------|---------|---------------|
| `_meta.rules` | Standing instructions | Appended to system prompt |
| `_meta.yoloMode` | Permission bypass | Maps to `--always-approve` |

**Decision Matrix**:

| If | Then |
|----|------|
| Grok honors standard ACP `session/new` metadata | Use existing `nativeacp` config |
| Grok requires `_meta.rules` extension | Extend `nativeacp.SessionMeta` (canonical) |
| Extension too invasive for nativeacp | Use piacp-style custom `acpdriver.New` |

Document final decision in implementation PR.

## Quality Gates

| Gate | Verification |
|------|--------------|
| G1 | Independent review note in `reviews/g1-review.md` |
| G2 | `gofmt && go build && go vet && go test -race && golangci-lint` |
| G3 | `AO_LIVE_GROK_ACP=1 go test -v -run Resume ./internal/adapters/chatdriver/grokacp/...` |
| G4 | Manual UI verification: session survives restart with history |

## G4 UI Verification Steps

1. Start AO daemon and frontend
2. Create Grok Chat session, send message with codeword
3. Create a file via agent
4. Stop daemon (Ctrl+C)
5. Restart daemon
6. Reopen same session in frontend
7. Verify:
   - [ ] Conversation history visible
   - [ ] Can send new turn
   - [ ] Agent remembers previous context
   - [ ] File created earlier still in workspace
