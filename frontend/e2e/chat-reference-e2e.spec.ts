import { test } from "@playwright/test";
import {
	runAttachmentDelivery,
	runModelSwitch,
	runReasoningEffort,
	runTimelineActivities,
	withLiveChatSession,
} from "./support/live-chat-scenarios";
import {
	CLAUDE_REFERENCE_HARNESS,
	LIVE_TEST_TIMEOUT_MS,
	liveGate,
	requireLiveContext,
} from "./support/live-daemon";

// Claude Code reference runs of the Grok UI scenarios (P4b, gate G4).
//
// Grok is new to Chat; Claude Code has been driven through these surfaces for
// long enough that its behaviour is the yardstick. Running the identical
// scenario bodies against it is what separates "Grok is broken" from "this
// surface never worked for any ACP harness", which is the question a failing
// Grok run otherwise cannot answer. Same reasoning, and the same shared-body
// split, as backend/e2e/chat_reference_test.go.
//
// AI.METRICA.PRO ROUTER. The reference environment points Claude Code at
// https://ai.metrica.pro/v1 as a custom router, which performs model alias
// substitution (for example claude-sonnet → a cached upstream alias) for
// cost/routing reasons. That is the production Claude setup the parity claim is
// about, so a reference run should go through it. AO itself never reads
// CLAUDE_ROUTER_URL — the daemon's own Claude Code installation does — so this
// spec cannot verify the routing from the inside. It requires the variable as
// the operator's explicit statement that the router was configured, and skips
// (never silently passes as G4) when it is absent.
//
// HOW TO RUN
//
//   1. Start a real daemon with a Claude Code installation pointed at the
//      router, and register a project to spawn sessions in.
//   2. Run the gated spec (playwright.config.ts starts the live renderer):
//        AO_LIVE_CLAUDE_ACP=1 CLAUDE_ROUTER_URL=https://ai.metrica.pro/v1 \
//          AO_E2E_LIVE_PROJECT=<projectId> \
//          npx playwright test chat-reference-e2e
//
// ENV
//   AO_LIVE_CLAUDE_ACP=1   required; spends real Claude account usage
//   CLAUDE_ROUTER_URL      required; the reference router (see above)
//   AO_E2E_LIVE_PROJECT    required; daemon project id the sessions run in
//   AO_E2E_DAEMON_URL      optional; default is read from the daemon run file
//   AO_E2E_LIVE_PORT       optional; live renderer port (default AO_E2E_PORT + 1)
//
// The full bootstrap (live renderer server, bridge injection, daemon discovery)
// is documented in frontend/e2e/support/live-daemon.ts and mirrored in the
// header of chat-grok-e2e.spec.ts.

const REFERENCE_ROUTER_URL = "https://ai.metrica.pro/v1";

const gate = liveGate(CLAUDE_REFERENCE_HARNESS);
const router = process.env.CLAUDE_ROUTER_URL?.trim() ?? "";

test.skip(
	!gate.ready || router === "",
	gate.ready
		? router === ""
			? `set CLAUDE_ROUTER_URL=${REFERENCE_ROUTER_URL} to run the Claude Code reference suite through the router G4 specifies`
			: ""
		: gate.reason,
);
test.use(gate.ready ? { baseURL: gate.live.rendererBaseUrl } : {});
test.describe.configure({ mode: "serial" });

test.describe("Claude Code Reference E2E", () => {
	test("reference: model switch from UI applies to engine", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		const live = requireLiveContext(gate);
		await withLiveChatSession(live, (sessionId) => runModelSwitch(page, live, sessionId));
	});

	test("reference: reasoning effort propagates from UI", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		const live = requireLiveContext(gate);
		await withLiveChatSession(live, (sessionId) => runReasoningEffort(page, live, sessionId));
	});

	test("reference: file attachment delivered to worktree", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		const live = requireLiveContext(gate);
		await withLiveChatSession(live, (sessionId) => runAttachmentDelivery(page, live, sessionId));
	});

	test("reference: timeline shows tool activities", async ({ page }) => {
		test.setTimeout(LIVE_TEST_TIMEOUT_MS);
		const live = requireLiveContext(gate);
		await withLiveChatSession(live, async (sessionId) => {
			await runTimelineActivities(page, live, sessionId);
		});
	});
});
