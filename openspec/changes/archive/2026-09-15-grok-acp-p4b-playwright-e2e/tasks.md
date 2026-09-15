# Tasks: P4b — Playwright UI E2E with Claude Code Reference

## Overview

Playwright E2E tests validating the frontend↔engine contract through real UI
interaction, with Claude Code via ai.metrica.pro as reference implementation.

## Status

Complete. Merged as [#8](https://github.com/EugeneMetrica/agent-orchestrator/pull/8);
G3 and G4 ran live afterwards on a box with both provider CLIs installed
(2026-09-15).

- Written and statically verified: both spec files, the shared live bootstrap
  (`frontend/e2e/support/live-daemon.ts`) and scenario bodies
  (`frontend/e2e/support/live-chat-scenarios.ts`), the `dev:web:live` renderer
  server, and the router documentation.
- Ran green at merge: `npm run typecheck`, `npm run typecheck:e2e`,
  `npx vitest run`, `CI=true npm run test:e2e:renderer` (60 passed), the ungated
  `npx playwright test chat-grok-e2e chat-reference-e2e` (10 skipped, 0 failed),
  `mise run lint`, and `go test ./...`.
- **G3 — live Grok, 6/6 passed.**

  ```bash
  AO_LIVE_GROK_ACP=1 AO_E2E_LIVE_PROJECT=<projectId> \
    npx playwright test chat-grok-e2e
  ```

  All six scenarios executed against a real daemon and a real Grok account:
  harness selection, model switch, reasoning effort, attachment delivery,
  timeline activities, and the workspace panel. No scenario reported the
  "provider offers no second model" or "nothing advertises efforts" skip, so the
  model and effort controls were both exercised on live catalogs.
- **G4 — Claude Code reference, 4/4 passed.**

  ```bash
  AO_LIVE_CLAUDE_ACP=1 CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1 \
    AO_E2E_LIVE_PROJECT=<projectId> \
    npx playwright test chat-reference-e2e
  ```

  Claude Code ran on its canonical configuration for this stack: the
  Anthropic-compatible gateway at `https://ai.metrica.pro/v1` with the GLM 5.3
  family as the primary model mapping (`ANTHROPIC_DEFAULT_OPUS_MODEL=glm-5.3`,
  `ANTHROPIC_DEFAULT_SONNET_MODEL=glm-5.3-flash`), following the operator
  procedure Z.ai documents at https://docs.z.ai/devpack/tool/claude. The parity
  claim is about the harness contract — the Claude Code harness driven through
  AO's `claudeacp` binding over the Anthropic wire protocol — and that is what
  both runs exercised. See `docs/research/grok-claude-parity-p4b.md` for what
  the two runs do and do not establish.
- **Fixture prerequisite found by the run.** The project named by
  `AO_E2E_LIVE_PROJECT` needs a resolvable default branch. A project still
  reporting `auto` fails at session spawn, before any UI assertion; setting its
  `defaultBranch` to `main` is what made the run reach the UI. Recorded in
  `design.md` and `docs/harnesses/grok.md`.
- **G1 for this phase is not recorded.** No independent review note exists at
  `reviews/g1-review.md` for P4b. The two corrections the live runs did force —
  the provider `config-options` model path and the daemon-named attachment path —
  are carried in the spec delta and in `design.md`'s "As built" section.

---

## Task 4b.1: Create chat-grok-e2e.spec.ts

**File**: `frontend/e2e/chat-grok-e2e.spec.ts`

Create Playwright test file with gate:

```typescript
import { expect, test } from "@playwright/test";

test.describe("Grok Chat E2E", () => {
    test.skip(!process.env.AO_LIVE_GROK_ACP, "set AO_LIVE_GROK_ACP=1");

    // Tests go here
});
```

**Verification**: File compiles with `npm run frontend:typecheck`

---

## Task 4b.2: Implement Harness Selection Test

Test selecting Grok in new task dialog.

```typescript
test("select Grok harness in new task dialog", async ({ page }) => {
    // Click New task
    // Select Grok from dropdown
    // Enter prompt, start
    // Verify session shows Grok
});
```

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "select Grok"`

---

## Task 4b.3: Implement Model Switch Test

Test model dropdown changes engine model.

**Assertions**:
- [x] Model dropdown opens
- [x] Selection changes UI display
- [x] API state reflects new model
- [x] Next turn uses selected model

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "model switch"`

---

## Task 4b.4: Implement Reasoning Effort Test

Test reasoning effort selector propagates.

**Assertions**:
- [x] Effort selector accessible
- [x] Change reflects in UI
- [x] API state shows effort

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "reasoning effort"`

---

## Task 4b.5: Implement File Attachment Test

Test file upload via UI.

**Assertions**:
- [x] File input accepts file
- [x] Upload completes
- [x] File exists in worktree
- [x] Agent can read file

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "file attachment"`

---

## Task 4b.6: Implement Timeline Activities Test

Test timeline shows tool activities.

**Assertions**:
- [x] Command activities visible
- [x] File edit activities visible
- [x] Can expand/view details

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "timeline"`

---

## Task 4b.7: Implement Workspace Panel Test

Test workspace panel matches worktree.

**Assertions**:
- [x] Created files appear in panel
- [x] Panel content matches `os.ReadFile`

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "workspace panel"`

---

## Task 4b.8: Create chat-reference-e2e.spec.ts

**File**: `frontend/e2e/chat-reference-e2e.spec.ts`

Create Claude Code reference test file:

```typescript
import { expect, test } from "@playwright/test";

test.describe("Claude Code Reference E2E", () => {
    test.skip(!process.env.AO_LIVE_CLAUDE_ACP, "set AO_LIVE_CLAUDE_ACP=1");

    test.beforeAll(() => {
        // Verify ai.metrica.pro router configured
        expect(process.env.CLAUDE_ROUTER_URL).toBeDefined();
    });

    // Same tests as Grok, establishing reference behavior
});
```

**Verification**: File compiles with `npm run frontend:typecheck`

---

## Task 4b.9: Implement Claude Code Reference Tests

Mirror all Grok tests for Claude Code:

- [x] `reference: model switch from UI applies to engine`
- [x] `reference: reasoning effort propagates`
- [x] `reference: file attachment delivered to worktree`
- [x] `reference: timeline shows tool activities`

**Verification**: `AO_LIVE_CLAUDE_ACP=1 CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1 npx playwright test chat-reference-e2e`

---

## Task 4b.10: Document ai.metrica.pro Router Setup

**File**: `frontend/e2e/README.md` or inline in test file

Document:
- What ai.metrica.pro router does (model alias substitution)
- Required environment variables
- How to obtain router access
- Why it's used as reference (production Claude setup)

**Verification**: Documentation clear enough for new contributor.

---

## Task 4b.11: Run Full CI Suite

```bash
mise run lint
npm run frontend:typecheck
cd backend && go test ./... && go test -race ./...
```

**Verification**: All checks pass.

---

## Exit Criteria Checklist

### G1: Independent Deep Review
- [ ] Review conducted by non-author
- [ ] Review note created at `reviews/g1-review.md`
- [ ] Review verdict: PASS or FAIL with rationale

### G2: Full CI Suite
- [x] `npm run frontend:typecheck` passes
- [x] Backend tests pass

### G3: Live Playwright Grok Tests — 6/6 passed
- [x] `AO_LIVE_GROK_ACP=1` set
- [x] Grok harness selection works
- [x] Model switch applies to engine
- [x] File attachments delivered to worktree
- [x] Timeline shows activities

### G4: Claude Code Reference Parity — 4/4 passed
- [x] `AO_LIVE_CLAUDE_ACP=1` set
- [x] `CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1` set
- [x] Same tests pass for Claude Code
- [x] Grok behavior matches Claude Code for:
  - Model switch
  - Reasoning effort
  - File attachments
  - Timeline activities

---

## Guarantees Verification Matrix

| Guarantee | Grok Test | Claude Ref | Parity |
|-----------|-----------|------------|--------|
| G-MODEL: Model switch applied | Task 4b.3 | Task 4b.9 | Compare outcomes |
| G-EFFORT: Effort propagates | Task 4b.4 | Task 4b.9 | Compare outcomes |
| G-FILES: UI files delivered | Task 4b.5 | Task 4b.9 | Compare outcomes |
| G-PARITY: Matches reference | — | — | All above match |

---

## Reference Files

Existing Playwright tests:
- `frontend/e2e/chat-agent-switch.spec.ts`
- `frontend/e2e/switch-agent-dialog.spec.ts`
- `frontend/e2e/model-menu-scroll.spec.ts`
- `frontend/e2e/chat-queued-attachments.spec.ts`

Playwright config:
- `frontend/playwright.config.ts`
