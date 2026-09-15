# Design: P4b — Playwright UI E2E with Claude Code Reference

## Overview

This document describes the Playwright E2E test design for validating the
frontend↔engine contract through real UI interaction, with Claude Code as the
reference implementation for parity verification.

## As built

The code sketches below are the original design sketch. Three things resolved
differently once the scenarios ran against a real daemon; the spec delta carries
the corrected text, and the sketches are kept as written for the record.

1. **A provider model catalog, not `settings.model`.** Grok and Claude Code both
   advertise a `model` config option, so the composer writes
   `PATCH /conversation/config-options/{id}` and the engine records the choice as
   that option's `currentValue`. `settings.model` is the native-catalog path and
   is never written for these two harnesses, so the sketch's
   `expect(state.settings.model)` would fail on a model switch that did apply.
   Every scenario resolves the live catalog from the daemon first and asserts
   against the field that catalog writes. Effort follows the same split
   (`thought_level` option versus `settings.reasoningEffort`).
2. **The daemon names staged attachments.** The path is
   `.ao/attachments/attachment-*.<ext>`, not the uploaded file name, so the
   assertion reads the staged path AO recorded in the user message rather than
   `.ao/attachments/test-upload.txt`.
3. **One shared scenario body, not a harness matrix.** The scenarios live in
   `frontend/e2e/support/live-chat-scenarios.ts` and both specs call them, which
   is the same split `backend/e2e/chat_reference_test.go` uses. A `for` loop over
   a harness list, as sketched, would put both providers' gates in one file and
   make a Grok failure indistinguishable from a surface that never worked.

## Test Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  Playwright Test Runner                                              │
│    - chat-grok-e2e.spec.ts (Grok tests)                             │
│    - chat-reference-e2e.spec.ts (Claude Code reference)             │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  AO Desktop App (Electron) or Web UI                                 │
│    - New task dialog → harness selection                            │
│    - Model dropdown → settings change                               │
│    - Chat input → file attachments                                  │
│    - Timeline → tool activities                                     │
│    - Workspace panel → file listing                                 │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  AO Daemon → Chat Service → Driver → Provider CLI                   │
│    Grok path: grokacp → grok agent stdio                            │
│    Claude path: claudeacp → claude-agent-acp                        │
│                 (via ai.metrica.pro router)                         │
└─────────────────────────────────────────────────────────────────────┘
```

## Claude Code via ai.metrica.pro Router

For reference tests, Claude Code runs against the Anthropic-compatible gateway
at ai.metrica.pro, with the GLM 5.3 family as its primary model mapping. This is
the canonical Claude Code configuration on this stack, so a reference run should
go through it.

```
┌─────────────────────────────────────────────────────────────────────┐
│  Claude Code on the ai.metrica.pro gateway                           │
│                                                                      │
│  Claude Code's own environment (AO configures none of this):         │
│    ANTHROPIC_BASE_URL=https://ai.metrica.pro/v1                     │
│    ANTHROPIC_AUTH_TOKEN=<gateway key>                               │
│    ANTHROPIC_DEFAULT_OPUS_MODEL=glm-5.3                             │
│    ANTHROPIC_DEFAULT_SONNET_MODEL=glm-5.3-flash                     │
│                                                                      │
│  The suite's own variable:                                           │
│    CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1                      │
│      the operator's statement that the above was configured, so an   │
│      unconfigured machine skips instead of passing as G4             │
│                                                                      │
│  Purpose:                                                            │
│    - Reference implementation for Grok parity tests                 │
│    - Proves AO surfaces work for the Claude setup actually in use   │
└─────────────────────────────────────────────────────────────────────┘
```

The gateway speaks the Anthropic wire protocol, which is what Claude Code and
AO's `claudeacp` binding both talk, so nothing in AO needs to know which models
sit behind it. Z.ai documents the operator procedure for this mapping
(https://docs.z.ai/devpack/tool/claude), whose direct Anthropic-protocol
endpoint is `https://api.z.ai/api/anthropic`; the `ai.metrica.pro` gateway
fronts the same protocol. AO does already name this class of route on its own:
the Claude hook route hint maps `api.z.ai` to the `zai` billing provider
(`backend/internal/cli/hooks.go`), and `pricing/catalog/v1/providers/zai` prices
`glm-5.3` and `glm-5.3-flash`.

## Test Implementation

### chat-grok-e2e.spec.ts

```typescript
import { expect, test } from "@playwright/test";

const SKIP_WITHOUT_LIVE = !process.env.AO_LIVE_GROK_ACP;

test.describe("Grok Chat E2E", () => {
    test.skip(SKIP_WITHOUT_LIVE, "set AO_LIVE_GROK_ACP=1 for live tests");

    test("select Grok harness in new task dialog", async ({ page }) => {
        await page.goto("/#/projects/e2e-grok");
        await page.getByRole("button", { name: "New task" }).click();

        const dialog = page.getByRole("dialog", { name: "New task" });
        await expect(dialog).toBeVisible();

        // Select Grok harness
        await dialog.getByRole("button", { name: "Agent" }).click();
        await page.getByRole("option", { name: "Grok" }).click();

        // Enter task and start
        await dialog.getByRole("textbox").fill("Reply with SELECTED");
        await dialog.getByRole("button", { name: "Start" }).click();

        // Verify session created with Grok
        await expect(page.getByText("Grok")).toBeVisible();
        await expect(page.getByText("SELECTED")).toBeVisible({ timeout: 60000 });
    });

    test("model switch from UI applies to engine", async ({ page }) => {
        // ... setup session ...

        // Switch model via dropdown
        await page.getByRole("button", { name: "Model" }).click();
        await page.getByRole("option", { name: "grok-4.5" }).click();

        // Send message
        await page.getByRole("combobox", { name: "Message" }).fill("Confirm model");
        await page.getByRole("button", { name: "Send" }).click();

        // Verify in API state (via test helper)
        const state = await getConversationState(page);
        expect(state.settings.model).toBe("grok-4.5");
    });

    test("file attachment delivered to worktree", async ({ page }) => {
        // ... setup session ...

        // Upload file via input
        const fileInput = page.locator('input[type="file"]');
        await fileInput.setInputFiles({
            name: "test-upload.txt",
            mimeType: "text/plain",
            buffer: Buffer.from("upload content"),
        });

        // Send message referencing file
        await page.getByRole("button", { name: "Send" }).click();

        // Wait for turn completion
        await expect(page.getByText("upload content")).toBeVisible({ timeout: 60000 });

        // Verify file in worktree (via test helper)
        const workspace = await getSessionWorkspace(page);
        const content = await readWorkspaceFile(workspace, ".ao/attachments/test-upload.txt");
        expect(content).toBe("upload content");
    });

    test("timeline shows tool activities", async ({ page }) => {
        // ... setup session ...

        // Send task that requires tools
        await sendMessage(page, "Create a file named timeline-test.txt with content hello");

        // Wait for completion
        await expect(page.getByText("created")).toBeVisible({ timeout: 60000 });

        // Verify timeline shows activity
        await expect(page.getByTestId("activity-command")).toBeVisible();
        // or
        await expect(page.getByTestId("activity-file-edit")).toBeVisible();
    });
});
```

### chat-reference-e2e.spec.ts

```typescript
import { expect, test } from "@playwright/test";

const SKIP_WITHOUT_CLAUDE = !process.env.AO_LIVE_CLAUDE_ACP;

test.describe("Claude Code Reference E2E", () => {
    test.skip(SKIP_WITHOUT_CLAUDE, "set AO_LIVE_CLAUDE_ACP=1 for reference tests");

    // Same tests as Grok, but with harness=claude-code
    // Uses ai.metrica.pro router for model resolution

    test.beforeAll(async () => {
        // Verify router configured
        expect(process.env.CLAUDE_ROUTER_URL).toBe("https://ai.metrica.pro/v1");
    });

    test("reference: model switch from UI applies to engine", async ({ page }) => {
        // Same test as Grok version
        // Establishes reference behavior
    });

    test("reference: file attachment delivered to worktree", async ({ page }) => {
        // Same test as Grok version
        // Establishes reference behavior
    });
});
```

## Harness Matrix Pattern

For running same tests across harnesses:

```typescript
const harnesses = [
    { name: "grok", gate: "AO_LIVE_GROK_ACP" },
    { name: "claude-code", gate: "AO_LIVE_CLAUDE_ACP", router: "ai.metrica.pro" },
];

for (const harness of harnesses) {
    test.describe(`${harness.name} Chat E2E`, () => {
        test.skip(!process.env[harness.gate], `set ${harness.gate}=1`);

        test("model switch from UI applies to engine", async ({ page }) => {
            await createSession(page, harness.name);
            // ... common test logic ...
        });
    });
}
```

## Quality Gates

| Gate | Verification |
|------|--------------|
| G1 | Independent review note in `reviews/g1-review.md` |
| G2 | `mise run lint && npm run frontend:typecheck` |
| G3 | `AO_LIVE_GROK_ACP=1 npx playwright test chat-grok-e2e` |
| G4 | `AO_LIVE_CLAUDE_ACP=1 CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1 npx playwright test chat-reference-e2e` |

## G3/G4 Environment Setup

### For Grok Tests (G3)

```bash
export AO_LIVE_GROK_ACP=1
export AO_E2E_LIVE_PROJECT=<projectId>   # project the sessions spawn in
# Requires: grok CLI installed, XAI credentials configured, a running daemon

cd frontend
npx playwright test chat-grok-e2e.spec.ts
```

### For Claude Code Reference (G4)

```bash
export AO_LIVE_CLAUDE_ACP=1
export CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1
export AO_E2E_LIVE_PROJECT=<projectId>
# Requires: claude CLI configured against the gateway (ANTHROPIC_BASE_URL plus
# the ANTHROPIC_DEFAULT_*_MODEL mapping above), and a running daemon

cd frontend
npx playwright test chat-reference-e2e.spec.ts
```

The project named by `AO_E2E_LIVE_PROJECT` needs a resolvable default branch.
A project still reporting `auto` fails at spawn, before any UI assertion runs;
setting its `defaultBranch` to `main` is what the G3/G4 runs needed.

## Parity Verification

To verify Grok matches Claude Code reference:

1. Run both test suites
2. Compare outcomes:
   - Model applied: both show model in settings
   - Files delivered: both have files in worktree
   - Timeline activities: both show tool calls
3. Document any intentional differences (harness-specific)
