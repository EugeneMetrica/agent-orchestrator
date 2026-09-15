import { existsSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import { delimiter, join } from "node:path";
import { expect, type Page } from "@playwright/test";
import { installFakeBridge } from "./fake-bridge";

// Live-daemon bootstrap for the gated UI e2e specs (chat-grok-e2e.spec.ts,
// chat-reference-e2e.spec.ts). Everything else in this suite is renderer-only.
//
// WHY THIS EXISTS. The rest of the suite runs the renderer under `dev:web`
// (VITE_NO_ELECTRON=1), where `usesPreviewWorkspaceData` is compiled in and
// `fetchWorkspaces` reads a mocked snapshot instead of `/api/v1/projects` (see
// src/renderer/hooks/useWorkspaceQuery.ts). A spec on that server can never see
// a real session, so "live UI against a real daemon" needs a different server
// and a different bootstrap:
//
//   1. A real `ao` daemon, discovered from AO_E2E_DAEMON_URL or from the run
//      file (AO_RUN_FILE, else $AO_DATA_DIR/running.json, else
//      ~/.ao/running.json). The daemon is never started or stopped here.
//   2. The renderer served WITHOUT VITE_NO_ELECTRON so preview mode is off and
//      every query goes to that daemon: `npm run dev:web:live`, started by
//      playwright.config.ts on its own port whenever a live gate is set.
//   3. `installFakeBridge(page, { daemonPort })`. Only the *bridge* — the
//      Electron preload IPC the browser has no access to — is faked, and it
//      reports the daemon ready on its real port so applyDaemonStatus points
//      the API client at it. No `page.route` interception: the renderer really
//      talks to the daemon over loopback, an origin corsMiddleware accepts
//      (isLoopbackOrigin in backend/internal/httpd/cors.go).
//   4. A project already registered in that daemon, named by
//      AO_E2E_LIVE_PROJECT. These specs run against the operator's own ~/.ao,
//      so they never create or delete projects and never remove a worktree they
//      did not create; sessions they spawn are terminated, not force-deleted.
//
// One expected console error in live mode: the renderer's Codex account SSE
// stream (/api/v1/agents/codex/accounts/events) is refused. Account routes sit
// behind codexAccountOriginMiddleware, which requires one of the exact
// configured renderer origins and deliberately does NOT accept a loopback dev
// origin the way the rest of the API does. Nothing these specs touch depends on
// it.
//
// ENV.
//   AO_LIVE_GROK_ACP=1     run the Grok UI suite (spends real Grok usage)
//   AO_LIVE_CLAUDE_ACP=1   run the Claude Code reference suite
//   AO_E2E_LIVE_PROJECT    daemon project id to spawn sessions in (required)
//   AO_E2E_DAEMON_URL      daemon base URL (default: from the run file)
//   AO_E2E_LIVE_PORT       live renderer port (default: AO_E2E_PORT + 1)
//   AO_E2E_LIVE_URL        live renderer base URL (default: 127.0.0.1:<port>)
//
// Missing gate, missing agent binary, missing project id, or no discoverable
// daemon all resolve to a skip with the reason spelled out, so an ungated
// `npx playwright test` reports these specs as skipped and never fails. A gate
// that IS set but points at a project or daemon that does not answer fails
// loudly: that is a misconfiguration, not an absence.

/** Provider whose UI suite a live gate opts in. */
export type LiveHarness = {
	/** Daemon harness id, as sent on spawn and reported on the session. */
	id: string;
	/** Provider CLI the daemon spawns; a machine without it skips. */
	binary: string;
	/** Env var that must be "1" for the suite to run. */
	gateEnv: string;
};

export const GROK_HARNESS: LiveHarness = { id: "grok", binary: "grok", gateEnv: "AO_LIVE_GROK_ACP" };

export const CLAUDE_REFERENCE_HARNESS: LiveHarness = {
	id: "claude-code",
	binary: "claude",
	gateEnv: "AO_LIVE_CLAUDE_ACP",
};

/** Everything a live spec needs once the gate is open. */
export type LiveContext = {
	harness: LiveHarness;
	daemon: LiveDaemon;
	projectId: string;
	/** Renderer origin the live specs point `baseURL` at. */
	rendererBaseUrl: string;
};

export type LiveGate = { ready: true; live: LiveContext } | { ready: false; reason: string };

/** Turn states the lifecycle treats as settled (mirrors backend/e2e). */
const TERMINAL_TURN_STATES = new Set(["completed", "interrupted", "failed"]);

export function isTerminalTurnState(state: string): boolean {
	return TERMINAL_TURN_STATES.has(state);
}

/** A live model turn is minutes, not seconds. */
export const LIVE_TURN_TIMEOUT_MS = 5 * 60_000;
/** Whole-test budget: session spawn plus one or two live turns. */
export const LIVE_TEST_TIMEOUT_MS = 15 * 60_000;

export function liveRendererPort(): number {
	const explicit = Number(process.env.AO_E2E_LIVE_PORT ?? Number.NaN);
	if (Number.isFinite(explicit) && explicit > 0) return explicit;
	return Number(process.env.AO_E2E_PORT ?? 5173) + 1;
}

/**
 * The resolved context, for use inside a test body that a file-level
 * `test.skip` already guarded.
 *
 * @param {LiveGate} gate Outcome of liveGate().
 * @returns {LiveContext} The live context.
 */
export function requireLiveContext(gate: LiveGate): LiveContext {
	if (!gate.ready) throw new Error(`live context unavailable: ${gate.reason}`);
	return gate.live;
}

/**
 * Resolve the live bootstrap for one harness, synchronously, so a spec can put
 * the outcome straight into `test.skip(condition, reason)`.
 *
 * @param {LiveHarness} harness Provider whose gate and CLI are checked.
 * @returns {LiveGate} Either the ready context or the reason for skipping.
 */
export function liveGate(harness: LiveHarness): LiveGate {
	if (process.env[harness.gateEnv] !== "1") {
		return {
			ready: false,
			reason: `set ${harness.gateEnv}=1 to run the live ${harness.id} UI suite (real daemon, real model calls)`,
		};
	}
	if (!onPath(harness.binary)) {
		return { ready: false, reason: `agent binary "${harness.binary}" is not on PATH` };
	}
	const projectId = process.env.AO_E2E_LIVE_PROJECT?.trim();
	if (!projectId) {
		return {
			ready: false,
			reason: "set AO_E2E_LIVE_PROJECT to a project id already registered in the running daemon",
		};
	}
	const daemonBaseUrl = resolveDaemonBaseUrl();
	if (!daemonBaseUrl) {
		return {
			ready: false,
			reason:
				"no running daemon found: start `ao start` or set AO_E2E_DAEMON_URL (the run file is read from AO_RUN_FILE, $AO_DATA_DIR/running.json, or ~/.ao/running.json)",
		};
	}
	const rendererBaseUrl =
		process.env.AO_E2E_LIVE_URL?.trim() || `http://127.0.0.1:${liveRendererPort()}`;
	return {
		ready: true,
		live: { harness, daemon: new LiveDaemon(daemonBaseUrl), projectId, rendererBaseUrl },
	};
}

/**
 * Daemon base URL, from the explicit override or the run-file handshake the
 * Electron main process itself reads (backend/internal/runfile).
 *
 * @returns {string | null} Base URL, or null when no daemon is recorded.
 */
function resolveDaemonBaseUrl(): string | null {
	const explicit = process.env.AO_E2E_DAEMON_URL?.trim();
	if (explicit) return explicit.replace(/\/+$/, "");
	const dataDir = process.env.AO_DATA_DIR?.trim() || join(homedir(), ".ao");
	const runFile = process.env.AO_RUN_FILE?.trim() || join(dataDir, "running.json");
	try {
		const { port } = JSON.parse(readFileSync(runFile, "utf8")) as { port?: number };
		return typeof port === "number" && port > 0 ? `http://127.0.0.1:${port}` : null;
	} catch {
		return null;
	}
}

/** Mirror of Go's exec.LookPath, so the skip reason matches backend/e2e. */
function onPath(binary: string): boolean {
	const entries = (process.env.PATH ?? "").split(delimiter).filter(Boolean);
	return entries.some((dir) => existsSync(join(dir, binary)));
}

/* ---- daemon API ---------------------------------------------------------- */

export type ConversationTurn = {
	id: string;
	state: string;
	errorMessage?: string;
	providerTurnId?: string;
};

export type ConversationMessage = { role: string; text: string };

export type ConversationActivity = {
	id: string;
	turnId?: string;
	activityKind: string;
	status: string;
	summary: string;
	detail?: Record<string, unknown>;
};

export type ConversationSettings = {
	model?: string;
	reasoningEffort?: string;
	approvalMode?: string;
};

export type ConversationSnapshot = {
	sessionId: string;
	harness: string;
	mode: string;
	controller: string;
	capabilities?: string[];
	turns: ConversationTurn[];
	messages: ConversationMessage[];
	activities: ConversationActivity[];
	settings: ConversationSettings;
};

export type ConversationModel = {
	id: string;
	displayName: string;
	default: boolean;
	efforts?: string[];
	defaultEffort?: string;
};

export type ConversationConfigChoice = { value: string; name: string };

export type ConversationConfigOption = {
	id: string;
	name: string;
	description?: string;
	category?: string;
	type: string;
	currentValue?: string;
	choices?: ConversationConfigChoice[];
};

/** Thin read/write client for the loopback daemon, from the Node side. */
export class LiveDaemon {
	constructor(readonly baseUrl: string) {}

	/**
	 * Call one daemon route and decode its JSON body.
	 *
	 * @param {string} method HTTP method.
	 * @param {string} path Path under /api/v1, e.g. "/sessions/foo/conversation".
	 * @param {unknown} [body] JSON request body.
	 * @returns {Promise<T>} Decoded response.
	 */
	async call<T>(method: string, path: string, body?: unknown): Promise<T> {
		const response = await fetch(`${this.baseUrl}/api/v1${path}`, {
			method,
			...(body === undefined
				? {}
				: { headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }),
		});
		const text = await response.text();
		if (!response.ok) {
			throw new Error(`${method} ${path} -> ${response.status}: ${text}`);
		}
		return (text ? JSON.parse(text) : {}) as T;
	}

	/** Label the agent menu renders for a harness (from the readiness catalog). */
	async agentLabel(harnessId: string): Promise<string> {
		const { agents } = await this.call<{ agents: { id: string; label: string }[] }>(
			"GET",
			"/agents/readiness",
		);
		const agent = agents.find((entry) => entry.id === harnessId);
		if (!agent) {
			throw new Error(
				`the daemon does not offer harness "${harnessId}"; installed: ${agents.map((a) => a.id).join(", ")}`,
			);
		}
		return agent.label || harnessId;
	}

	/** Spawn a chat session directly, for scenarios whose subject is not the dialog. */
	async spawnChatSession(input: {
		projectId: string;
		harness: string;
		prompt: string;
	}): Promise<string> {
		const { session } = await this.call<{ session: { id: string } }>("POST", "/sessions", {
			projectId: input.projectId,
			kind: "worker",
			harness: input.harness,
			mode: "chat",
			prompt: input.prompt,
		});
		return session.id;
	}

	async session(sessionId: string): Promise<{ harness: string; mode: string }> {
		const { session } = await this.call<{ session: { harness: string; mode: string } }>(
			"GET",
			`/sessions/${sessionId}`,
		);
		return session;
	}

	conversation(sessionId: string): Promise<ConversationSnapshot> {
		return this.call<ConversationSnapshot>("GET", `/sessions/${sessionId}/conversation`);
	}

	models(sessionId: string): Promise<{ models: ConversationModel[] }> {
		return this.call<{ models: ConversationModel[] }>(
			"GET",
			`/sessions/${sessionId}/conversation/models`,
		);
	}

	configOptions(sessionId: string): Promise<{ options: ConversationConfigOption[] }> {
		return this.call<{ options: ConversationConfigOption[] }>(
			"GET",
			`/sessions/${sessionId}/conversation/config-options`,
		);
	}

	/** Absolute worktree the session's agent runs in — the only way to check disk. */
	async workspacePath(sessionId: string): Promise<string> {
		const { workspacePath } = await this.call<{ workspacePath: string }>(
			"GET",
			`/desktop/sessions/${sessionId}/workspace`,
		);
		if (!workspacePath) throw new Error(`session ${sessionId} reported no workspace path`);
		return workspacePath;
	}

	/**
	 * Stop a session the suite spawned. Best effort: a live run must not fail on
	 * cleanup, and the worktree is deliberately left in place (AO does not
	 * force-delete dirty registered worktrees).
	 */
	async terminateQuietly(sessionId: string): Promise<void> {
		try {
			await this.call("POST", `/sessions/${sessionId}/kill`, {});
		} catch {
			/* the scenario's own assertions are the signal, not teardown */
		}
	}
}

/** Concatenated assistant text, the same view backend/e2e asserts on. */
export function assistantText(snapshot: ConversationSnapshot): string {
	return snapshot.messages
		.filter((message) => message.role === "assistant")
		.map((message) => message.text)
		.join("\n");
}

/** Short account of a conversation, for failure messages. */
export function describeConversation(snapshot: ConversationSnapshot): string {
	const turns = snapshot.turns
		.map((turn) => `${turn.state}${turn.errorMessage ? ` (${turn.errorMessage})` : ""}`)
		.join(", ");
	const activities = snapshot.activities
		.map((activity) => `${activity.activityKind}/${activity.status}: ${activity.summary}`)
		.join("\n  ");
	return [
		`controller=${snapshot.controller} settings=${JSON.stringify(snapshot.settings)}`,
		`turns: ${turns}`,
		`activities:\n  ${activities}`,
		`assistant: ${assistantText(snapshot)}`,
	].join("\n");
}

/**
 * Wait until a conversation satisfies a predicate, polling the daemon.
 *
 * @param {LiveDaemon} daemon Daemon client.
 * @param {string} sessionId Session under test.
 * @param {string} what What is being waited for, used in the failure message.
 * @param {(snapshot: ConversationSnapshot) => boolean} settled Predicate.
 * @param {number} [timeout] Budget in milliseconds.
 * @returns {Promise<ConversationSnapshot>} The snapshot that satisfied the predicate.
 */
export async function awaitConversation(
	daemon: LiveDaemon,
	sessionId: string,
	what: string,
	settled: (snapshot: ConversationSnapshot) => boolean,
	timeout = LIVE_TURN_TIMEOUT_MS,
): Promise<ConversationSnapshot> {
	let last: ConversationSnapshot | undefined;
	await expect
		.poll(
			async () => {
				last = await daemon.conversation(sessionId);
				return settled(last);
			},
			{ message: `waiting for ${what}`, timeout, intervals: [1_000] },
		)
		.toBe(true);
	if (!last) throw new Error(`no conversation snapshot while waiting for ${what}`);
	return last;
}

/** Wait for the session's last turn to settle, then require it completed. */
export async function awaitTurnCompleted(
	daemon: LiveDaemon,
	sessionId: string,
	what: string,
	minTurns = 1,
): Promise<ConversationSnapshot> {
	const snapshot = await awaitConversation(daemon, sessionId, what, (snap) => {
		if (snap.turns.length < minTurns) return false;
		return isTerminalTurnState(snap.turns[snap.turns.length - 1].state);
	});
	const last = snapshot.turns[snapshot.turns.length - 1];
	expect(
		last.state,
		`${what}: turn ended as ${last.state} (${last.errorMessage ?? "no error"})\n${describeConversation(snapshot)}`,
	).toBe("completed");
	return snapshot;
}

/* ---- page bootstrap ------------------------------------------------------ */

/**
 * Point a page at the live daemon and open a session's chat surface.
 *
 * @param {Page} page Playwright page.
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session to open.
 */
export async function gotoLiveSession(page: Page, live: LiveContext, sessionId: string): Promise<void> {
	await prepareLivePage(page, live);
	await page.goto(`/#/projects/${live.projectId}/sessions/${sessionId}`);
	await expect(page.getByRole("region", { name: "Chat" })).toBeVisible({ timeout: 60_000 });
	await failOnStartupGate(page);
}

/**
 * Fail with the reason on screen when the startup requirements dialog is up.
 *
 * That modal blocks every other control, so without this a machine missing a
 * system requirement reports "overlay intercepts pointer events" instead of
 * "install what AO says is missing". Call it after the surface behind the modal
 * has painted, so a gate that is going to appear already has.
 *
 * @param {Page} page Playwright page.
 */
export async function failOnStartupGate(page: Page): Promise<void> {
	const gate = page.getByRole("dialog", { name: /^(No coding agent found|Missing dependency)$/ });
	if ((await gate.count()) === 0) return;
	throw new Error(
		`AO is blocked on a system requirement, so no control is reachable:\n${await gate.innerText()}`,
	);
}

/**
 * Install the ready-daemon bridge before any page script runs. Must be called
 * before the first navigation; no API route is intercepted.
 *
 * @param {Page} page Playwright page.
 * @param {LiveContext} live Resolved live context.
 */
export async function prepareLivePage(page: Page, live: LiveContext): Promise<void> {
	await page.emulateMedia({ reducedMotion: "reduce" });
	await installFakeBridge(page, { daemonPort: daemonPortOf(live.daemon) });
}

function daemonPortOf(daemon: LiveDaemon): number {
	const port = Number(new URL(daemon.baseUrl).port);
	if (!Number.isFinite(port) || port <= 0) {
		throw new Error(`daemon base URL ${daemon.baseUrl} has no port to hand the renderer`);
	}
	return port;
}
