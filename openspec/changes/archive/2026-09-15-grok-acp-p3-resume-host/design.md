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
│    9. Verify before.txt still exists and is unchanged               │
│   10. Verify a standing token is still applied (START or RESUME)     │
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
    // Standing instructions must still be in force, but Grok keeps the rules the
    // session was created with, so RESUME_TOKEN is optional and START_TOKEN
    // surviving is the observed behaviour.
    if !strings.Contains(resumeAnswer, "START_TOKEN") &&
        !strings.Contains(resumeAnswer, "RESUME_TOKEN") {
        t.Errorf("resumed answer carries no standing token: %q", resumeAnswer)
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

Grok's ACP takes standing instructions through `_meta` on the session request:

| Field | Purpose | Grok Behavior |
|-------|---------|---------------|
| `_meta.rules` | Standing instructions | Folded into `<human_rules>` in Grok's own system prompt on `session/new` |
| `_meta.yoloMode` | Permission bypass | Maps to `--always-approve` — unused by AO |

**Decision**: the canonical `nativeacp.SessionMeta` hook is enough. `grokacp`
supplies `rules` from `cfg.SystemPrompt`; no `nativeacp` extension and no
piacp-style custom `acpdriver.New` are needed. `_meta.yoloMode` stays unused
because bypass-permissions is already expressed by the `bypassPermissions`
session mode plus the launch-time `--always-approve` flag, and a second silent
path to widen approvals is a liability.

### `_meta.rules` on `session/load`: documented limitation

The shared ACP transport sends `SessionMeta` on `session/new`, `session/load`,
and `session/resume`, so AO's `_meta.rules` is on the wire for resume too. Grok
documents the field as a `session/new` input, and the live run confirms the
provider behaviour: a reloaded session keeps the rules it was created with, and
the resumed answer still bears the token from the original start even though
`session/load` carried a different one.

Consequences, stated plainly so no downstream work assumes otherwise:

- Resume **preserves** standing instructions. It does **not** update them.
- AO must not treat `session/load` as a way to rewrite Grok's standing rules. A
  session whose standing instructions changed needs a new provider session.
- AO keeps re-sending `_meta.rules` on load anyway: it costs nothing, keeps the
  transport's contract uniform across bindings, and a future Grok that honours
  the update needs no AO change.
- The live test therefore asserts that *a* standing token is still applied after
  resume, not that the newer one is.

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
