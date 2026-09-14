# Design: P4a — API Frontend E2E (Backend HTTP Routes)

## Overview

This document describes the API E2E test design for validating the
frontend↔engine contract through AO's HTTP API.

## Test Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  backend/e2e/chat_grok_test.go                                       │
│                                                                      │
│  Uses daemon test harness:                                          │
│    d := startDaemon(t, dataDir)                                     │
│    d.mustCall("POST", "/sessions", ...)                             │
│    d.awaitConversation(sessionID, ...)                              │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  AO Daemon HTTP API                                                  │
│    POST /api/v1/sessions         → Spawn chat session               │
│    PATCH .../conversation/settings → Set model/effort               │
│    POST .../conversation/messages → Send turn                       │
│    POST .../conversation/attachments → Upload files                 │
│    GET .../conversation          → Poll state                       │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Chat Service → grokacp Driver → grok agent stdio                   │
└─────────────────────────────────────────────────────────────────────┘
```

## Test Implementation

### TestChatGrokSpawn

```go
func TestChatGrokSpawn(t *testing.T) {
    requireE2E(t)
    if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
        t.Skip("set AO_LIVE_GROK_ACP=1 for live Grok tests")
    }

    d := startDaemon(t, t.TempDir())
    project := seedProject(t, d, "grok-spawn", withAgent("grok"))

    session := spawn(t, d, map[string]any{
        "projectId": project,
        "kind":      "worker",
        "harness":   "grok",
        "mode":      "chat",
        "prompt":    "Reply with exactly: SPAWNED",
    })

    if session.Session.Harness != "grok" {
        t.Errorf("harness = %q, want grok", session.Session.Harness)
    }
    if session.Session.Mode != "chat" {
        t.Errorf("mode = %q, want chat", session.Session.Mode)
    }

    snap := d.awaitConversation(session.Session.ID, 3*time.Minute,
        "first turn to complete",
        func(s snapshot) bool {
            return len(s.Turns) >= 1 && terminal(s.Turns[0].State)
        })

    if !contains(snap.assistantText(), "SPAWNED") {
        t.Errorf("agent response missing SPAWNED: %s", describe(snap))
    }
}
```

### TestChatGrokModelOverride

```go
func TestChatGrokModelOverride(t *testing.T) {
    requireE2E(t)
    if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
        t.Skip("set AO_LIVE_GROK_ACP=1 for live Grok tests")
    }

    d := startDaemon(t, t.TempDir())
    project := seedProject(t, d, "grok-model")
    session := chatSession(t, d, project, "grok", "Reply with exactly: READY")

    // Set model via API
    var settings ConversationTurnSettingsPayload
    d.mustCall("PATCH", "/sessions/"+session+"/conversation/settings",
        http.StatusOK,
        map[string]any{"model": "xai/grok-3"},
        &settings)

    if settings.Model != "xai/grok-3" {
        t.Errorf("settings.model = %q, want xai/grok-3", settings.Model)
    }

    // Verify GET returns same
    conv := d.conversation(session)
    if conv.Settings.Model != "xai/grok-3" {
        t.Errorf("conversation settings.model = %q, want xai/grok-3",
            conv.Settings.Model)
    }

    // Send turn with model
    send(t, d, session, "Reply with exactly: MODEL_APPLIED", "model-test")
    snap := d.awaitConversation(session, 3*time.Minute,
        "turn to complete",
        func(s snapshot) bool {
            return contains(s.assistantText(), "MODEL_APPLIED")
        })

    // Model should be recorded
    if snap.Settings.Model != "xai/grok-3" {
        t.Errorf("recorded model = %q", snap.Settings.Model)
    }
}
```

### TestChatGrokAttachments

```go
func TestChatGrokAttachments(t *testing.T) {
    requireE2E(t)
    if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
        t.Skip("set AO_LIVE_GROK_ACP=1 for live Grok tests")
    }

    d := startDaemon(t, t.TempDir())
    project := seedProject(t, d, "grok-attach")
    session := chatSession(t, d, project, "grok", "Reply with exactly: READY")

    // Upload attachment
    var uploadResp struct {
        Paths []string `json:"paths"`
    }
    d.mustCallMultipart("POST", "/sessions/"+session+"/conversation/attachments",
        http.StatusOK,
        []multipartFile{{name: "test.txt", content: "attachment content"}},
        &uploadResp)

    if len(uploadResp.Paths) != 1 {
        t.Fatalf("upload paths = %v, want 1", uploadResp.Paths)
    }
    attachPath := uploadResp.Paths[0]

    // Verify file in worktree
    workspace := d.sessionWorkspace(session)
    content, err := os.ReadFile(filepath.Join(workspace, attachPath))
    if err != nil || string(content) != "attachment content" {
        t.Fatalf("attachment file = %q, err=%v", content, err)
    }

    // Send message referencing attachment
    send(t, d, session,
        "Read the file at "+attachPath+" and tell me its contents",
        "attach-test")

    snap := d.awaitConversation(session, 3*time.Minute,
        "turn referencing attachment",
        func(s snapshot) bool {
            return contains(s.assistantText(), "attachment content")
        })

    if !contains(snap.assistantText(), "attachment content") {
        t.Errorf("agent didn't read attachment: %s", describe(snap))
    }
}
```

## Claude Code Reference Tests

For parity verification, same tests run against Claude Code:

```go
func TestChatClaudeCodeModelOverride(t *testing.T) {
    if os.Getenv("AO_LIVE_CLAUDE_ACP") != "1" {
        t.Skip("set AO_LIVE_CLAUDE_ACP=1 for Claude Code reference")
    }
    // Same test logic as TestChatGrokModelOverride but with harness=claude-code
    // Used as reference implementation to verify Grok parity
}
```

## Quality Gates

| Gate | Verification |
|------|--------------|
| G1 | Independent review note in `reviews/g1-review.md` |
| G2 | `gofmt && go build && go vet && go test -race && golangci-lint` |
| G3 | `AO_LIVE_GROK_ACP=1 go test -v ./e2e/... -run ChatGrok` |
| G4 | `AO_LIVE_CLAUDE_ACP=1` reference tests pass for parity |

## Test Matrix

| Test | Grok | Claude Code (ref) |
|------|------|-------------------|
| Spawn | Required | Reference |
| Model override | Required | Reference |
| Reasoning effort | Required | Reference |
| Approval mode | Required | Reference |
| Attachments | Required | Reference |
| State consistency | Required | Reference |
