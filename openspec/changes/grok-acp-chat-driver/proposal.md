# Proposal: Grok ACP Chat Driver

## Why

Users who prefer Chat mode over TUI mode cannot use Grok as their agent because
the registry does not include a Grok Chat driver. This creates an inconsistent
experience where Claude Code, Cursor, OpenCode, and other agents support both
modes while Grok is limited to terminal-only interaction. Adding a native ACP
Chat driver for Grok enables Chat mode using the same proven pattern as other
harnesses, without modifying the existing TUI behavior.

## Problem Statement

The `grok` harness (xAI Grok Build) currently supports only TUI mode via terminal
scraping and hook-based activity tracking. Other comparable agents (Claude Code,
Cursor, OpenCode, Droid, Kimi, Kimchi, Pi, OMP) already have native ACP Chat
drivers that provide structured JSON-RPC communication, streaming responses,
tool approvals, and interrupt/resume capabilities.

Users who prefer Chat mode over TUI mode cannot use Grok as their agent because
`registry.SupportsChat(domain.HarnessGrok)` returns `false`.

## Proposed Solution

Add a new `grokacp` Chat driver package that:

1. Reuses the existing `adapters/agent/grok` plugin for binary resolution
   (`ResolveGrokBinary`) and authentication status (`AuthStatus`).
2. Spawns Grok via `grok agent [--always-approve] [--model …] [--no-auto-update] stdio`
   (ACP JSON-RPC over stdio transport).
3. Integrates with AO's shared ACP stack (`chatdriver/acp`, `nativeacp`,
   persistent host, permissions).
4. Registers in `chatdriver/registry.Build` alongside existing drivers.

## Key Constraints

- **No credential injection**: reuse the user's existing `~/.grok` / `GROK_HOME` /
  `XAI_API_KEY` configuration exactly as the existing TUI adapter does.
- **No private GROK_HOME rewrite**: unlike CAO-style harnesses that isolate
  config, Grok uses the user's native profile.
- **No `grok -p`**: the `-p`/`--single` flag runs one headless turn and exits,
  which is unsuitable for multi-turn Chat sessions.
- **No TUI/pty scraping for Chat**: the driver uses only the ACP stdio contract.
- **No Hermes-style adapter**: we do not vendor xai-org/grok-build source or run
  Grok as an ACP server; we run it as an ACP client subprocess.

## Success Criteria

1. `registry.SupportsChat(domain.HarnessGrok)` returns `true`.
2. Chat sessions with `harness=grok` spawn via `grok agent … stdio`.
3. Auth probe reuses existing `grok.Plugin.AuthStatus()`.
4. Tests pass: `TestShippedChatDrivers`, `driver_test.go` unit tests.
5. No changes to TUI-mode behavior or existing `adapters/agent/grok` semantics.

## Non-Goals

- Telegram or voice integration for Grok.
- Bundling or downloading the Grok CLI binary.
- Supporting `grok -p` as the Chat transport.
- Implementing Grok-specific MCP server extension features in this change.
- Modifying the `nativeacp` SessionMeta schema (if `_meta.rules` / `yoloMode` is
  needed, it will be a separate follow-up change).

## Impact

- **Scope**: `backend/internal/adapters/chatdriver/grokacp/` (new package),
  `backend/internal/adapters/chatdriver/registry/registry.go` (registration),
  `backend/internal/adapters/chatdriver/registry/registry_test.go` (test update).
- **Risk**: Low. The pattern exactly mirrors `opencodeacp`, `droidacp`, `ompacp`.
- **Dependencies**: No new external dependencies; uses existing `nativeacp` stack.
