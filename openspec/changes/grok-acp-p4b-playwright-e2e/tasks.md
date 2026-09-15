# Tasks: P4b — Playwright UI E2E with Claude Code Reference

## Overview

Playwright E2E tests validating the frontend↔engine contract through real UI
interaction, with Claude Code via ai.metrica.pro as reference implementation.

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
- [ ] Model dropdown opens
- [ ] Selection changes UI display
- [ ] API state reflects new model
- [ ] Next turn uses selected model

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "model switch"`

---

## Task 4b.4: Implement Reasoning Effort Test

Test reasoning effort selector propagates.

**Assertions**:
- [ ] Effort selector accessible
- [ ] Change reflects in UI
- [ ] API state shows effort

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "reasoning effort"`

---

## Task 4b.5: Implement File Attachment Test

Test file upload via UI.

**Assertions**:
- [ ] File input accepts file
- [ ] Upload completes
- [ ] File exists in worktree
- [ ] Agent can read file

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "file attachment"`

---

## Task 4b.6: Implement Timeline Activities Test

Test timeline shows tool activities.

**Assertions**:
- [ ] Command activities visible
- [ ] File edit activities visible
- [ ] Can expand/view details

**Verification**: `AO_LIVE_GROK_ACP=1 npx playwright test -g "timeline"`

---

## Task 4b.7: Implement Workspace Panel Test

Test workspace panel matches worktree.

**Assertions**:
- [ ] Created files appear in panel
- [ ] Panel content matches `os.ReadFile`

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

- [ ] `reference: model switch from UI applies to engine`
- [ ] `reference: reasoning effort propagates`
- [ ] `reference: file attachment delivered to worktree`
- [ ] `reference: timeline shows tool activities`

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
- [ ] `npm run frontend:typecheck` passes
- [ ] Backend tests pass

### G3: Live Playwright Grok Tests
- [ ] `AO_LIVE_GROK_ACP=1` set
- [ ] Grok harness selection works
- [ ] Model switch applies to engine
- [ ] File attachments delivered to worktree
- [ ] Timeline shows activities

### G4: Claude Code Reference Parity
- [ ] `AO_LIVE_CLAUDE_ACP=1` set
- [ ] `CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1` set
- [ ] Same tests pass for Claude Code
- [ ] Grok behavior matches Claude Code for:
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
