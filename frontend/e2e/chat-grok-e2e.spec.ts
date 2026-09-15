import { test } from "@playwright/test";
import {
	expectWorkspacePanelMatchesWorktree,
	runAttachmentDelivery,
	runHarnessSelection,
	runModelSwitch,
	runReasoningEffort,
	runTimelineActivities,
	withLiveChatSession,
} from "./support/live-chat-scenarios";
import { GROK_HARNESS, LIVE_TEST_TIMEOUT_MS, liveGate, requireLiveContext } from "./support/live-daemon";

// Live UI e2e for a Grok Chat session (P4b). The API suite
// (backend/e2e/chat_grok_test.go) proves the routes reach the driver; this
// proves the surfaces a user actually operates reach those routes — a harness
// the new-task dialog drops, a model dropdown that changes only its own label,
// an attachment that never lands in the worktree, and a timeline that hides the
// tools a turn ran are all invisible to an API test.
//
// Claude Code runs the critical scenarios as the parity reference in
// chat-reference-e2e.spec.ts, from the same scenario bodies.
//
// HOW TO RUN
//
//   1. Start a real daemon and register a project to spawn sessions in:
//        ao start                      # or leave the desktop app running
//   2. Serve the renderer in live mode. playwright.config.ts starts this itself
//      when a live gate is set; the underlying command is:
//        npm --prefix frontend run dev:web:live
//      It is `vite` WITHOUT VITE_NO_ELECTRON, unlike the `dev:web` server the
//      rest of the suite uses: with VITE_NO_ELECTRON the renderer compiles in
//      preview mode and reads a mocked workspace snapshot, so no real session is
//      ever visible. See frontend/e2e/support/live-daemon.ts for the whole
//      bootstrap (bridge injection, daemon discovery, CORS).
//   3. Run the gated spec:
//        AO_LIVE_GROK_ACP=1 AO_E2E_LIVE_PROJECT=<projectId> \
//          npx playwright test chat-grok-e2e
//
// ENV
//   AO_LIVE_GROK_ACP=1     required; spends real Grok account usage
//   AO_E2E_LIVE_PROJECT    required; daemon project id the sessions run in
//   AO_E2E_DAEMON_URL      optional; default is read from the daemon run file
//   AO_E2E_LIVE_PORT       optional; live renderer port (default AO_E2E_PORT + 1)
//
// Without the gate — the ordinary `npx playwright test` and the CI renderer job —
// every test here is skipped with the missing prerequisite as the reason.

const gate = liveGate(GROK_HARNESS);

test.skip(!gate.ready, gate.ready ? "" : gate.reason);
test.use(gate.ready ? { baseURL: gate.live.rendererBaseUrl } : {});
// One live provider account, one session at a time: parallel turns only add
// rate-limit flakes to a suite whose subject is the UI.
test.describe.configure({ mode: "serial" });

test.describe("Grok Chat E2E", () => {
	test("select Grok harness in new task dialog", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		await runHarnessSelection(page, requireLiveContext(gate));
	});

	test("model switch from UI applies to engine", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		const live = requireLiveContext(gate);
		await withLiveChatSession(live, (sessionId) => runModelSwitch(page, live, sessionId));
	});

	test("reasoning effort propagates from UI", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		const live = requireLiveContext(gate);
		await withLiveChatSession(live, (sessionId) => runReasoningEffort(page, live, sessionId));
	});

	test("file attachment delivered to worktree", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		const live = requireLiveContext(gate);
		await withLiveChatSession(live, (sessionId) => runAttachmentDelivery(page, live, sessionId));
	});

	test("timeline shows tool activities", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		const live = requireLiveContext(gate);
		await withLiveChatSession(live, async (sessionId) => {
			await runTimelineActivities(page, live, sessionId);
		});
	});

	test("workspace panel matches worktree", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		const live = requireLiveContext(gate);
		await withLiveChatSession(live, async (sessionId) => {
			const fileName = await runTimelineActivities(page, live, sessionId);
			await expectWorkspacePanelMatchesWorktree(page, live, sessionId, fileName);
		});
	});
});
