import { defineConfig } from "@playwright/test";

// Overridable because 5173 is not ours alone: another worktree's
// `electron-forge start` also listens there, and with reuseExistingServer the
// suite silently runs against ITS renderer — which is built for Electron and
// fails in confusing, unrelated ways. Set AO_E2E_PORT to run alongside one.
const port = Number(process.env.AO_E2E_PORT ?? 5173);

// The live UI specs (chat-grok-e2e, chat-reference-e2e) cannot use the dev:web
// server: VITE_NO_ELECTRON compiles the renderer into preview mode, where
// useWorkspaceQuery reads a mocked snapshot instead of the daemon, so a real
// session is never visible. They get a second renderer on their own port,
// started only when a live gate opts them in — an ungated run pays nothing and
// still boots the ordinary server for every other spec. Keep in sync with
// liveRendererPort() in e2e/support/live-daemon.ts.
const liveGateEnabled =
	process.env.AO_LIVE_GROK_ACP === "1" || process.env.AO_LIVE_CLAUDE_ACP === "1";
const livePort = Number(process.env.AO_E2E_LIVE_PORT ?? port + 1);

const rendererServer = {
	// dev:web serves the renderer alone (VITE_NO_ELECTRON=1) — no Electron child to
	// launch, which is all the browser-based e2e suite needs.
	command: `npm run dev:web -- --port ${port} --host 127.0.0.1`,
	port,
	reuseExistingServer: !process.env.CI,
};

const liveRendererServer = {
	// Same renderer without VITE_NO_ELECTRON, so preview mode is off and every
	// query goes to the daemon the injected bridge reports.
	command: `npm run dev:web:live -- --port ${livePort} --host 127.0.0.1`,
	port: livePort,
	reuseExistingServer: !process.env.CI,
};

export default defineConfig({
	testDir: "e2e",
	use: {
		baseURL: `http://127.0.0.1:${port}`,
	},
	webServer: liveGateEnabled ? [rendererServer, liveRendererServer] : rendererServer,
});
