import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test, type Locator, type Page } from "@playwright/test";
import {
	assistantText,
	awaitTurnCompleted,
	describeConversation,
	failOnStartupGate,
	gotoLiveSession,
	prepareLivePage,
	type ConversationActivity,
	type ConversationConfigOption,
	type ConversationSnapshot,
	type LiveContext,
} from "./live-daemon";

// The UI scenarios both live specs run. One body per guarantee, called from
// chat-grok-e2e.spec.ts and (for the critical ones) chat-reference-e2e.spec.ts,
// so "Grok is broken" and "this surface never worked for any ACP harness" are
// answered by the same code rather than by two drifting copies — the same split
// backend/e2e uses for runChatModelOverride / runChatAttachments.
//
// MODEL AND EFFORT COME FROM TWO PLACES. The composer has two model paths and
// which one renders is the provider's choice, not the test's: SessionChatSurface
// suppresses the native control for every dimension a provider catalog replaces.
//
//   - provider catalog — the ACP agent advertises a `model` config option, so
//     the UI writes PATCH /conversation/config-options/{id} and the engine
//     records the choice as that option's `currentValue`. Both Grok and Claude
//     Code land here: acp.ListModels projects the provider's `model` config
//     option into GET /conversation/models, so the option exists whenever the
//     catalog does.
//   - native catalog — no provider model option, so the UI writes PATCH
//     /conversation/settings and the engine records `settings.model`.
//
// Every scenario resolves which path is live from the daemon before touching the
// UI and asserts against the field that path actually writes. Asserting
// `settings.model` unconditionally, as the P4b design sketch does, would fail on
// every ACP harness for a model switch that did apply.

/** One value to select in the composer's turn-settings menu. */
type TurnSettingPick = {
	/** Submenu trigger label, which is the provider option name or "Model"/"Effort". */
	submenu: string;
	/** Choice label as the menu renders it. */
	choice: string;
	/**
	 * Accessible name of the standalone trigger a provider catalog with exactly
	 * one control renders instead of the grouped menu (ConfigOptionPicker passes
	 * `option.description || option.name` as both title and aria-label).
	 */
	standaloneName?: string;
};

/** What a model switch should select, plus how the engine records it. */
type ModelTarget =
	| { path: "provider"; optionId: string; value: string; pick: TurnSettingPick }
	| { path: "native"; modelId: string; pick: TurnSettingPick };

/** What a reasoning-effort change should select. */
type EffortTarget =
	| { path: "provider"; optionId: string; value: string; pick: TurnSettingPick }
	| { path: "native"; modelId: string; effort: string; modelPick: TurnSettingPick; effortPick: TurnSettingPick };

const CONVERSATION_LOG = { role: "log" as const, name: "Conversation" };
const UI_TIMEOUT_MS = 60_000;

/* ---- session setup ------------------------------------------------------- */

/**
 * Spawn a chat session on the harness under test and wait for its first turn.
 *
 * Used by the scenarios whose subject is not the new-task dialog, so a model or
 * attachment failure cannot be confused with a dialog failure.
 *
 * @param {LiveContext} live Resolved live context.
 * @returns {Promise<string>} The new session id.
 */
export async function startLiveChatSession(live: LiveContext): Promise<string> {
	const sessionId = await live.daemon.spawnChatSession({
		projectId: live.projectId,
		harness: live.harness.id,
		prompt: "Reply with exactly: READY",
	});
	await awaitTurnCompleted(live.daemon, sessionId, "the spawn turn to settle");
	return sessionId;
}

/**
 * Run one scenario against a fresh session and stop that session afterwards.
 *
 * Teardown terminates; it never deletes. These specs run against the operator's
 * own ~/.ao, and AO does not force-delete dirty registered worktrees.
 *
 * @param {LiveContext} live Resolved live context.
 * @param {(sessionId: string) => Promise<void>} body The scenario.
 */
export async function withLiveChatSession(
	live: LiveContext,
	body: (sessionId: string) => Promise<void>,
): Promise<void> {
	const sessionId = await startLiveChatSession(live);
	try {
		await body(sessionId);
	} finally {
		await live.daemon.terminateQuietly(sessionId);
	}
}

/* ---- scenario: harness selection ---------------------------------------- */

/**
 * Pick the harness in the new-task dialog and prove the session it created runs
 * on it. The dialog is the only place a user chooses a harness, and it posts to
 * /orchestrators/delegate rather than spawning directly, so an agent the daemon
 * offers but the dialog drops is invisible to an API test.
 *
 * @param {Page} page Playwright page.
 * @param {LiveContext} live Resolved live context.
 */
export async function runHarnessSelection(page: Page, live: LiveContext): Promise<void> {
	const agentLabel = await live.daemon.agentLabel(live.harness.id);
	await prepareLivePage(page, live);
	await page.goto(`/#/projects/${live.projectId}`);

	const newTask = page.getByRole("button", { name: "New task" }).first();
	await expect(newTask).toBeVisible({ timeout: UI_TIMEOUT_MS });
	await failOnStartupGate(page);
	await newTask.click();
	const dialog = page.getByRole("dialog", { name: "Create a new task" });
	await expect(dialog).toBeVisible();

	await dialog.getByRole("button", { name: "Agent" }).click();
	// Prefix, not exact: an item appends its auth state ("…Auth unknown") when the
	// catalog cannot confirm the account. The readiness catalog is also fetched
	// when the dialog opens, so the harness's own label may arrive after the menu
	// does; the locator waits for it.
	await page
		.getByRole("menuitem", { name: startsWith(agentLabel) })
		.first()
		.click({ timeout: UI_TIMEOUT_MS });
	await expect(dialog.getByRole("button", { name: "Agent" })).toContainText(agentLabel);

	await dialog.getByLabel("Task").fill("Reply with exactly: SELECTED");
	await dialog.getByRole("button", { name: "Start task" }).click();

	// The dialog navigates to the session it created; that URL is how a user gets
	// there and the only place the new id is exposed.
	await page.waitForURL(/#\/projects\/[^/]+\/sessions\/[^/]+$/, { timeout: 5 * 60_000 });
	const sessionId = sessionIdFromUrl(page.url());

	try {
		const session = await live.daemon.session(sessionId);
		expect(session.harness, "the dialog created a session on the wrong harness").toBe(live.harness.id);
		expect(session.mode, "the dialog created a terminal session, not a chat one").toBe("chat");

		await expect(page.getByRole("region", { name: "Chat" })).toBeVisible({ timeout: UI_TIMEOUT_MS });
		const settled = await awaitTurnCompleted(live.daemon, sessionId, "the dialog's first turn");
		expect(
			assistantText(settled),
			`the agent's answer never arrived:\n${describeConversation(settled)}`,
		).toContain("SELECTED");
		// And it is on screen, not merely in the API: the timeline is the only
		// place the user sees it.
		await expect(timelineOf(page)).toContainText("SELECTED", { timeout: UI_TIMEOUT_MS });
	} finally {
		await live.daemon.terminateQuietly(sessionId);
	}
}

/* ---- scenario: model switch --------------------------------------------- */

/**
 * Switch the model from the composer and prove the engine session took it — then
 * that it actually ran a turn on it, which is what separates an applied choice
 * from a stored one.
 *
 * @param {Page} page Playwright page.
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session already spawned on the harness under test.
 */
export async function runModelSwitch(page: Page, live: LiveContext, sessionId: string): Promise<void> {
	const target = await resolveModelTarget(live, sessionId);
	if (!target) {
		test.skip(true, "the provider offers no second model, so there is nothing to switch to");
		return;
	}

	await gotoLiveSession(page, live, sessionId);
	await chooseTurnSetting(page, target.pick);
	await expectEngineModel(live, sessionId, target);

	// A model the provider rejects fails the turn, so a completed turn on the
	// chosen model is the observable half of "applied".
	await sendFromComposer(page, "Reply with exactly: MODEL_APPLIED");
	const answered = await awaitTurnCompleted(live.daemon, sessionId, "a turn on the chosen model", 2);
	expect(
		assistantText(answered),
		`the agent did not answer on the chosen model:\n${describeConversation(answered)}`,
	).toContain("MODEL_APPLIED");
	await expectEngineModel(live, sessionId, target);
}

/* ---- scenario: reasoning effort ----------------------------------------- */

/**
 * Set reasoning effort from the composer and prove the engine recorded it.
 *
 * Skips when nothing advertises efforts: the composer renders no effort control
 * in that case, and inventing one would test the test.
 *
 * @param {Page} page Playwright page.
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session already spawned on the harness under test.
 */
export async function runReasoningEffort(page: Page, live: LiveContext, sessionId: string): Promise<void> {
	const target = await resolveEffortTarget(live, sessionId);
	if (!target) {
		test.skip(
			true,
			"neither the provider config catalog nor any model advertises reasoning efforts, so the composer renders no effort control",
		);
		return;
	}

	await gotoLiveSession(page, live, sessionId);
	if (target.path === "provider") {
		await chooseTurnSetting(page, target.pick);
		await expectConfigOption(live, sessionId, target.optionId, target.value);
		return;
	}
	// The native effort submenu exists only once a model that advertises efforts
	// is the selected one.
	await chooseTurnSetting(page, target.modelPick);
	await expectSettings(live, sessionId, (s) => s.model === target.modelId, `model ${target.modelId}`);
	await chooseTurnSetting(page, target.effortPick);
	await expectSettings(
		live,
		sessionId,
		(s) => s.reasoningEffort === target.effort,
		`reasoningEffort ${target.effort}`,
	);
}

/* ---- scenario: attachments ---------------------------------------------- */

/**
 * Attach a file through the composer's picker and prove the bytes reached the
 * worktree at the path the message claims, and that the agent can open them.
 *
 * The daemon names staged files itself (`.ao/attachments/attachment-*.ext`), so
 * the assertion is on the path AO recorded in the message rather than on the
 * uploaded file name.
 *
 * @param {Page} page Playwright page.
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session already spawned on the harness under test.
 */
export async function runAttachmentDelivery(page: Page, live: LiveContext, sessionId: string): Promise<void> {
	const content = `upload-content-${Date.now()}`;
	await gotoLiveSession(page, live, sessionId);

	await page.locator('input[type="file"]').setInputFiles({
		name: "test-upload.txt",
		mimeType: "text/plain",
		buffer: Buffer.from(content),
	});
	// The chip appears only after the daemon staged the bytes, so this is also the
	// wait for POST /attachments to have succeeded.
	await expect(page.getByLabel("Remove test-upload.txt")).toBeVisible({ timeout: UI_TIMEOUT_MS });

	await sendFromComposer(page, "Read the attached file with the shell and reply with its exact contents.");
	const answered = await awaitTurnCompleted(live.daemon, sessionId, "the agent to read the attachment", 2);

	const staged = stagedAttachmentPaths(answered);
	expect(
		staged,
		`no staged attachment path in the recorded user message:\n${describeConversation(answered)}`,
	).not.toHaveLength(0);

	const workspace = await live.daemon.workspacePath(sessionId);
	for (const path of staged) {
		expect(path, "staged path escaped .ao/attachments/").toMatch(/^\.ao\/attachments\/[^/]+$/);
		const onDisk = await readFile(join(workspace, ...path.split("/")), "utf8");
		expect(onDisk, `attachment ${path} on disk does not hold the uploaded bytes`).toBe(content);
	}
	expect(
		assistantText(answered),
		`the agent did not report the attachment's contents:\n${describeConversation(answered)}`,
	).toContain(content);
	// The chip's promise as the user reads it: the path is in the transcript.
	await expect(timelineOf(page)).toContainText(staged[0], { timeout: UI_TIMEOUT_MS });
}

/* ---- scenario: timeline + workspace panel ------------------------------- */

/**
 * Run a turn that must use tools and prove the timeline accounts for it.
 *
 * Tolerates command / mcp_tool / file_change the same way backend/e2e does:
 * Grok's ACP agent reports shell tools without ToolKind execute, so the shared
 * mapper records them as mcp_tool rather than command. Either kind proves the
 * turn reached a tool.
 *
 * @param {Page} page Playwright page.
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session already spawned on the harness under test.
 * @returns {Promise<string>} Name of the file the agent was asked to create.
 */
export async function runTimelineActivities(
	page: Page,
	live: LiveContext,
	sessionId: string,
): Promise<string> {
	const fileName = `timeline-${Date.now()}.txt`;
	await gotoLiveSession(page, live, sessionId);
	await sendFromComposer(
		page,
		`Use the shell to create a file named ${fileName} containing exactly hello, then reply with exactly: CREATED`,
	);
	const answered = await awaitTurnCompleted(live.daemon, sessionId, "the tool-using turn", 2);

	const tools = answered.activities.filter((activity) =>
		["command", "mcp_tool", "file_change"].includes(activity.activityKind),
	);
	expect(
		tools,
		`no tool activity recorded for a turn that had to write a file:\n${describeConversation(answered)}`,
	).not.toHaveLength(0);
	for (const activity of tools) {
		expect(activity.activityKind, "an activity with no kind cannot be rendered").not.toBe("");
		expect(activity.status, "an activity with no status cannot be rendered").not.toBe("");
	}
	await expectToolActivityRendered(page, tools);

	const workspace = await live.daemon.workspacePath(sessionId);
	const onDisk = await readFile(join(workspace, fileName), "utf8");
	expect(onDisk.trim(), `${fileName} on disk does not hold what the agent was asked to write`).toBe("hello");
	return fileName;
}

/**
 * Prove the workspace panel lists a file the agent really created.
 *
 * @param {Page} page Playwright page already on the session.
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session under test.
 * @param {string} fileName Worktree-root file the agent created.
 */
export async function expectWorkspacePanelMatchesWorktree(
	page: Page,
	live: LiveContext,
	sessionId: string,
	fileName: string,
): Promise<void> {
	const inspector = page.locator("#inspector");
	if (!(await inspector.isVisible())) {
		await page.getByRole("button", { name: "Open inspector panel" }).click();
	}
	const files = page.getByRole("region", { name: "Session files" });
	if (!(await files.isVisible())) {
		// The pane the tab opens has a "Files" tab of its own, so scope to the
		// inspector's own tab bar, which precedes the pane it switches.
		await inspector.getByRole("tab", { name: "Files" }).first().click();
	}
	await expect(files).toBeVisible({ timeout: UI_TIMEOUT_MS });
	// The pane opens on Changes (GET /workspace/files, changed files only) and
	// offers an all-files tree beside it. A new untracked file should appear in
	// both, but which one is asked first is the pane's choice, not this test's, so
	// accept either rather than pinning the assertion to a default view.
	if (!(await appearsIn(files, fileName))) {
		await files.getByRole("tab", { name: "Files" }).click();
		await expect(files, `${fileName} is in the worktree but in neither files view`).toContainText(
			fileName,
			{ timeout: UI_TIMEOUT_MS },
		);
	}

	const workspace = await live.daemon.workspacePath(sessionId);
	const onDisk = await readFile(join(workspace, fileName), "utf8");
	expect(onDisk.trim(), "the panel lists a file whose contents differ on disk").toBe("hello");
}

/* ---- UI steps ----------------------------------------------------------- */

/**
 * Whether text shows up in a region, without failing when it does not.
 *
 * @param {Locator} region Region to read.
 * @param {string} text Text to look for.
 * @returns {Promise<boolean>} True when the text arrived in time.
 */
async function appearsIn(region: Locator, text: string): Promise<boolean> {
	try {
		await expect(region).toContainText(text, { timeout: 30_000 });
		return true;
	} catch {
		return false;
	}
}

function timelineOf(page: Page) {
	return page.getByRole(CONVERSATION_LOG.role, { name: CONVERSATION_LOG.name });
}

/**
 * Type into the composer and send, the way a user does.
 *
 * @param {Page} page Playwright page on a chat session.
 * @param {string} text Message to send.
 */
async function sendFromComposer(page: Page, text: string): Promise<void> {
	const field = page.getByRole("combobox", { name: "Message the agent" });
	await expect(field).toBeVisible();
	await field.fill(text);
	await page.getByRole("button", { name: "Send message", exact: true }).click();
}

/**
 * Open the composer's model/effort menu and pick one value.
 *
 * The trigger is `Model and reasoning effort for the next turn` whenever the
 * menu groups more than one control; a provider advertising exactly one control
 * gets a standalone picker named after that option instead.
 *
 * @param {Page} page Playwright page on a chat session.
 * @param {TurnSettingPick} pick Submenu label, choice label, standalone fallback.
 */
async function chooseTurnSetting(page: Page, pick: TurnSettingPick): Promise<void> {
	const settings = page.getByRole("group", { name: "Turn settings" });
	await expect(settings).toBeVisible({ timeout: UI_TIMEOUT_MS });
	const grouped = page.getByRole("button", { name: "Model and reasoning effort for the next turn" });
	let trigger = grouped;
	if (await grouped.count()) {
		await grouped.click();
		// partitionConfigOptions keeps model and effort at the top level of this
		// menu (only unclassified options are nested under "More"), so a missing
		// submenu is a real change in the composer, not a layout the test should
		// paper over by clicking whatever radio happens to be on screen.
		const submenu = page.getByRole("menuitem", { name: startsWith(pick.submenu) });
		await expect(
			submenu.first(),
			`the turn-settings menu has no "${pick.submenu}" submenu`,
		).toBeVisible({ timeout: UI_TIMEOUT_MS });
		await submenu.first().click();
	} else {
		trigger = settings
			.getByRole("button", { name: startsWith(pick.standaloneName ?? pick.submenu) })
			.first();
		await trigger.click();
	}
	await page.getByRole("menuitemradio", { name: pick.choice }).first().click();

	// The trigger relabels itself from the value now in force, so this is the
	// user-visible half of the change. A provider that answered an earlier turn on
	// a substituted model labels itself with the substitution instead and flags it,
	// which is the composer working as designed, not a dropped selection — the
	// engine-state assertion each caller makes next is what settles that.
	const substituted = settings.getByLabel(/^Substituted for /);
	await expect
		.poll(
			async () =>
				(await substituted.count()) > 0 || ((await trigger.first().innerText()).includes(pick.choice)),
			{ message: `the turn-settings trigger never showed "${pick.choice}"`, timeout: UI_TIMEOUT_MS },
		)
		.toBe(true);
}

/**
 * Expand any batched activity runs and assert one of the turn's tool rows is on
 * screen. Rows carry no test id, so the expectation is built from what the
 * daemon reported for this very turn plus the collapsed labels the renderer
 * derives for command and file_change rows.
 *
 * @param {Page} page Playwright page on a chat session.
 * @param {ConversationActivity[]} activities Tool activities from the settled turn.
 */
async function expectToolActivityRendered(page: Page, activities: ConversationActivity[]): Promise<void> {
	const timeline = timelineOf(page);
	const runToggles = timeline.getByRole("button", { name: /(Ran|Explored) \d+ tool calls?/ });
	const runCount = await runToggles.count();
	for (let index = 0; index < runCount; index += 1) {
		const toggle = runToggles.nth(index);
		if ((await toggle.getAttribute("aria-expanded")) === "false") await toggle.click();
	}
	const providerNames = activities
		.map(toolRowText)
		.filter((text): text is string => Boolean(text))
		.map(escapeRegExp);
	const pattern = new RegExp(
		[
			// Collapsed labels ChatTimelineItems derives for command and file_change
			// rows, which never repeat the provider's own summary.
			"Ran command",
			"Read files?",
			"Search",
			"Checked repository",
			"Edited",
			"Created",
			"Deleted",
			"Renamed",
			...providerNames,
		].join("|"),
	);
	await expect(timeline.getByText(pattern).first()).toBeVisible({ timeout: UI_TIMEOUT_MS });
}

/**
 * The text an mcp_tool row shows: its tool name, falling back to the summary.
 *
 * @param {ConversationActivity} activity One conversation activity.
 * @returns {string | undefined} Row text, when the row shows provider text.
 */
function toolRowText(activity: ConversationActivity): string | undefined {
	if (activity.activityKind !== "mcp_tool") return undefined;
	const toolName = activity.detail?.toolName;
	if (typeof toolName === "string" && toolName !== "") return toolName;
	return activity.summary || undefined;
}

/* ---- engine-state assertions -------------------------------------------- */

async function expectEngineModel(live: LiveContext, sessionId: string, target: ModelTarget): Promise<void> {
	if (target.path === "provider") {
		await expectConfigOption(live, sessionId, target.optionId, target.value);
		return;
	}
	await expectSettings(live, sessionId, (s) => s.model === target.modelId, `model ${target.modelId}`);
}

/**
 * Poll the conversation snapshot — what the composer labels itself from — until
 * it reports the expected turn settings.
 *
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session under test.
 * @param {(settings: ConversationSnapshot["settings"]) => boolean} holds Predicate.
 * @param {string} what Expected state, used in the failure message.
 */
async function expectSettings(
	live: LiveContext,
	sessionId: string,
	holds: (settings: ConversationSnapshot["settings"]) => boolean,
	what: string,
): Promise<void> {
	await expect
		.poll(
			async () => {
				const snapshot = await live.daemon.conversation(sessionId);
				return holds(snapshot.settings) ? what : JSON.stringify(snapshot.settings);
			},
			{ message: `the conversation snapshot never reported ${what}`, timeout: UI_TIMEOUT_MS },
		)
		.toBe(what);
}

/**
 * Poll the provider config catalog until the option reports the chosen value.
 *
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session under test.
 * @param {string} optionId Provider option id.
 * @param {string} value Value the UI selected.
 */
async function expectConfigOption(
	live: LiveContext,
	sessionId: string,
	optionId: string,
	value: string,
): Promise<void> {
	await expect
		.poll(
			async () => {
				const { options } = await live.daemon.configOptions(sessionId);
				return options.find((option) => option.id === optionId)?.currentValue ?? "";
			},
			{ message: `provider option ${optionId} never reported ${value}`, timeout: UI_TIMEOUT_MS },
		)
		.toBe(value);
}

/* ---- catalog resolution -------------------------------------------------- */

/**
 * Pick a model the UI can switch TO, from whichever catalog the live provider
 * session advertises.
 *
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session under test.
 * @returns {Promise<ModelTarget | null>} Target, or null when there is no choice.
 */
async function resolveModelTarget(live: LiveContext, sessionId: string): Promise<ModelTarget | null> {
	const option = await providerOption(live, sessionId, isModelOption);
	if (option) {
		const choice = pickChoice(option);
		if (!choice) return null;
		return {
			path: "provider",
			optionId: option.id,
			value: choice.value,
			pick: providerPick(option, choice.name),
		};
	}
	const { models } = await live.daemon.models(sessionId);
	if (models.length === 0) return null;
	const defaultId = models.find((model) => model.default)?.id ?? "";
	const chosen = pickNonDefaultChatModel(models, defaultId);
	if (!chosen) return null;
	return { path: "native", modelId: chosen.id, pick: { submenu: "Model", choice: chosen.displayName } };
}

/**
 * Pick a reasoning effort the UI can set, from whichever catalog is live.
 *
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session under test.
 * @returns {Promise<EffortTarget | null>} Target, or null when nothing advertises efforts.
 */
async function resolveEffortTarget(live: LiveContext, sessionId: string): Promise<EffortTarget | null> {
	const option = await providerOption(live, sessionId, isEffortOption);
	if (option) {
		const choice = pickChoice(option);
		if (!choice) return null;
		return {
			path: "provider",
			optionId: option.id,
			value: choice.value,
			pick: providerPick(option, choice.name),
		};
	}
	// A native effort list belongs to a model, so that model has to be selected
	// first; prefer an effort that is not its default so a dropped setting cannot
	// look like a pass.
	const { models } = await live.daemon.models(sessionId);
	const model = models.find((candidate) => (candidate.efforts ?? []).length > 0);
	if (!model) return null;
	const efforts = model.efforts ?? [];
	const effort = efforts.find((candidate) => candidate !== model.defaultEffort) ?? efforts[0];
	return {
		path: "native",
		modelId: model.id,
		effort,
		modelPick: { submenu: "Model", choice: model.displayName },
		effortPick: { submenu: "Effort", choice: capitalize(effort) },
	};
}

/**
 * One provider config select, when the live session advertises the catalog.
 *
 * @param {LiveContext} live Resolved live context.
 * @param {string} sessionId Session under test.
 * @param {(option: ConversationConfigOption) => boolean} matches Option predicate.
 * @returns {Promise<ConversationConfigOption | undefined>} The option, if any.
 */
async function providerOption(
	live: LiveContext,
	sessionId: string,
	matches: (option: ConversationConfigOption) => boolean,
): Promise<ConversationConfigOption | undefined> {
	const snapshot = await live.daemon.conversation(sessionId);
	if (!(snapshot.capabilities ?? []).includes("config_options")) return undefined;
	const { options } = await live.daemon.configOptions(sessionId);
	return options.find((option) => option.type === "select" && matches(option));
}

// Same classification the composer uses (TurnSettingsBar isModelOption and the
// thought_level grouping), so the test drives the control the UI actually shows.
const isModelOption = (option: ConversationConfigOption): boolean =>
	option.category === "model" || option.id === "model";

const isEffortOption = (option: ConversationConfigOption): boolean =>
	option.category === "thought_level" || option.id === "effort";

function providerPick(option: ConversationConfigOption, choice: string): TurnSettingPick {
	return { submenu: option.name, choice, standaloneName: option.description || option.name };
}

/**
 * A choice that is not the one already in force, so a dropped write cannot pass.
 *
 * @param {ConversationConfigOption} option Provider select.
 * @returns {{ value: string; name: string } | undefined} A different choice, if one exists.
 */
function pickChoice(option: ConversationConfigOption): { value: string; name: string } | undefined {
	const current = option.currentValue ?? "";
	const choices = (option.choices ?? []).filter(
		(choice) => choice.value !== "" && choice.value !== current,
	);
	if (choices.length === 0) return undefined;
	if (!isModelOption(option)) return choices[0];
	// Model ids: prefer the closest sibling of the value in force (grok-4.6 →
	// grok-4.5) and skip the modality / free-tier entries a session catalog still
	// lists, which are advertised but unsuitable for an agent turn. Same heuristic
	// as pickNonDefaultChatModel in backend/e2e/chat_grok_test.go.
	const agentish = choices.filter((choice) => likelyAgentChatModel(choice.value));
	const pool = agentish.length > 0 ? agentish : choices;
	return [...pool].sort((a, b) => score(b.value, current) - score(a.value, current))[0];
}

function pickNonDefaultChatModel(
	models: { id: string; displayName: string; default: boolean }[],
	defaultId: string,
): { id: string; displayName: string } | null {
	const candidates = models.filter((model) => !model.default && model.id !== "");
	if (candidates.length === 0) return null;
	const agentish = candidates.filter((model) => likelyAgentChatModel(model.id));
	const pool = agentish.length > 0 ? agentish : candidates;
	return [...pool].sort((a, b) => score(b.id, defaultId) - score(a.id, defaultId))[0];
}

/** Longer shared prefix wins; among ties the shorter id wins. */
function score(id: string, reference: string): number {
	return sharedPrefixLength(id, reference) * 1000 - id.length;
}

function sharedPrefixLength(a: string, b: string): number {
	const limit = Math.min(a.length, b.length);
	let index = 0;
	while (index < limit && a[index] === b[index]) index += 1;
	return index;
}

function likelyAgentChatModel(id: string): boolean {
	const lower = id.toLowerCase();
	return !["imagine", "voice", "tts", "stt", "video", "image", ":free"].some((marker) =>
		lower.includes(marker),
	);
}

/* ---- small helpers ------------------------------------------------------- */

/** Worktree-relative attachment paths AO recorded in the sent user message. */
function stagedAttachmentPaths(snapshot: ConversationSnapshot): string[] {
	const message = [...snapshot.messages]
		.reverse()
		.find((entry) => entry.role === "user" && entry.text.includes(".ao/attachments/"));
	if (!message) return [];
	return [...message.text.matchAll(/\.ao\/attachments\/[\w.-]+/g)].map((match) => match[0]);
}

function sessionIdFromUrl(url: string): string {
	const match = /#\/projects\/[^/]+\/sessions\/([^/?]+)/.exec(url);
	if (!match) throw new Error(`no session id in ${url}`);
	return decodeURIComponent(match[1]);
}

function capitalize(value: string): string {
	return value.charAt(0).toUpperCase() + value.slice(1);
}

function escapeRegExp(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** Menu triggers append the value in force to their label, so match the head. */
function startsWith(label: string): RegExp {
	return new RegExp(`^${escapeRegExp(label)}`);
}
