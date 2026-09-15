//go:build !windows

package e2e

import (
	"encoding/base64"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Scenarios for "the frontend contract holds for a Grok Chat session".
//
// The renderer never talks to the grokacp driver; it talks to these routes. So
// what the driver unit and live tests prove about Grok is not yet a promise to a
// client: spawn has to return a Grok chat session, PATCH settings has to be
// readable back from the snapshot the composer renders from, an attachment has to
// be a file the agent can open, and the timeline has to account for what ran.
// Only the daemon boundary can show that, which is why these are HTTP scenarios.
//
// Separately gated from the codex suite because they spend the developer's own
// Grok account, binary, and quota:
//
//	AO_LIVE_GROK_ACP=1 go test ./e2e/ -v -run ChatGrok -timeout 30m
//
// The scenario bodies take a harness so chat_reference_test.go can run the same
// assertions against Claude Code. Parity is the point of the reference run: a
// Grok result is only meaningful if the same API operations behave the same way
// on a harness that was already shipping.

const grokGateEnv = "AO_LIVE_GROK_ACP"

// requireGrokE2E skips unless the Grok gate is set and the binary is installed.
func requireGrokE2E(t *testing.T) {
	t.Helper()
	requireLiveHarness(t, grokGateEnv, "grok")
}

// requireLiveHarness gates a live harness scenario on its own env var and binary
// rather than on the codex suite's gate: these tests do not use codex at all, and
// borrowing its gate would make a Grok run depend on an unrelated install.
func requireLiveHarness(t *testing.T, gate, binary string) {
	t.Helper()
	if os.Getenv(gate) != "1" {
		t.Skipf("set %s=1 to run the live chat API end-to-end suite for %s", gate, binary)
	}
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("%s is not on PATH: %v", binary, err)
	}
}

/* ---- shared fixtures --------------------------------------------------- */

// harnessChatSession spawns a chat session on the named harness and waits for its
// first turn to settle, so a scenario starts from a known state.
func harnessChatSession(t *testing.T, d *daemon, project, harness, prompt string) string {
	t.Helper()
	spawned := spawn(t, d, map[string]any{
		"projectId": project, "kind": "worker", "harness": harness, "mode": "chat",
		"prompt": prompt,
	})
	snap := d.awaitConversation(spawned.Session.ID, 5*time.Minute,
		"the "+harness+" session to finish its first turn",
		func(s snapshot) bool {
			return len(s.Turns) >= 1 && terminal(s.Turns[0].State)
		})
	if snap.Turns[0].State != "completed" {
		t.Fatalf("%s is installed but its first credentialed turn ended as %q (err=%q):\n%s",
			harness, snap.Turns[0].State, snap.Turns[0].ErrorMessage, describe(snap))
	}
	return spawned.Session.ID
}

// sessionWorkspace is the worktree the session's agent runs in. Attachment and
// tool-output proofs are read from it: a path the API reports is only a promise
// until the bytes are there.
func sessionWorkspace(t *testing.T, d *daemon, session string) string {
	t.Helper()
	var read struct {
		Session struct {
			WorkspacePath string `json:"workspacePath"`
		} `json:"session"`
	}
	d.mustCall("GET", "/sessions/"+session, http.StatusOK, nil, &read)
	if read.Session.WorkspacePath == "" {
		t.Fatalf("session %s reports no workspace path", session)
	}
	return read.Session.WorkspacePath
}

// modelCatalog is what the composer's model picker renders from.
type modelCatalog struct {
	Models []struct {
		ID          string   `json:"id"`
		DisplayName string   `json:"displayName"`
		Default     bool     `json:"default"`
		Efforts     []string `json:"efforts"`
	} `json:"models"`
	Selected turnSettings `json:"selected"`
}

func conversationModels(t *testing.T, d *daemon, session string) modelCatalog {
	t.Helper()
	var catalog modelCatalog
	d.mustCall("GET", "/sessions/"+session+"/conversation/models", http.StatusOK, nil, &catalog)
	return catalog
}

// setSettings PATCHes the composer's per-turn choices and returns what the daemon
// echoed back.
func setSettings(t *testing.T, d *daemon, session string, body map[string]any) turnSettings {
	t.Helper()
	var applied turnSettings
	d.mustCall("PATCH", "/sessions/"+session+"/conversation/settings", http.StatusOK, body, &applied)
	return applied
}

// stageTextAttachment uploads one text file and returns the worktree-relative path
// the caller should name in the message it sends next.
func stageTextAttachment(t *testing.T, d *daemon, session, content string) string {
	t.Helper()
	var staged struct {
		SessionID string   `json:"sessionId"`
		Paths     []string `json:"paths"`
	}
	d.mustCall("POST", "/sessions/"+session+"/attachments", http.StatusCreated,
		map[string]any{"attachments": []map[string]any{{
			"mimeType": "text/plain",
			"data":     base64.StdEncoding.EncodeToString([]byte(content)),
		}}}, &staged)
	if len(staged.Paths) != 1 {
		t.Fatalf("staged %d paths, want 1: %+v", len(staged.Paths), staged.Paths)
	}
	return staged.Paths[0]
}

func approvalActivities(s snapshot) []activity {
	var found []activity
	for _, a := range s.Activities {
		if a.Kind == "approval" {
			found = append(found, a)
		}
	}
	return found
}

/* ---- Grok scenarios ---------------------------------------------------- */

func TestChatGrokSpawn(t *testing.T) {
	requireGrokE2E(t)
	runChatSpawnScenario(t, "grok")
}

func TestChatGrokModelOverride(t *testing.T) {
	requireGrokE2E(t)
	runChatModelOverrideScenario(t, "grok")
}

func TestChatGrokReasoningEffort(t *testing.T) {
	requireGrokE2E(t)
	runChatReasoningEffortScenario(t, "grok")
}

func TestChatGrokApprovalModeOverride(t *testing.T) {
	requireGrokE2E(t)
	runChatApprovalModeOverrideScenario(t, "grok")
}

func TestChatGrokAttachments(t *testing.T) {
	requireGrokE2E(t)
	runChatAttachmentsScenario(t, "grok")
}

func TestChatGrokServerStateConsistency(t *testing.T) {
	requireGrokE2E(t)
	runChatServerStateScenario(t, "grok")
}

/* ---- scenario bodies --------------------------------------------------- */

// Spawn is the first thing the frontend does. A session that comes back on some
// other harness, or in TUI mode, is a session whose whole chat surface is wrong —
// and the spawn response is the only place the client learns which it got.
func runChatSpawnScenario(t *testing.T, harness string) {
	t.Helper()
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, harness+"-spawn")

	spawned := spawn(t, d, map[string]any{
		"projectId": project, "kind": "worker", "harness": harness, "mode": "chat",
		"prompt": "Reply with exactly: SPAWNED",
	})
	if spawned.Session.Harness != harness {
		t.Errorf("harness = %q, want %q", spawned.Session.Harness, harness)
	}
	if spawned.Session.Mode != "chat" {
		t.Errorf("mode = %q, want chat", spawned.Session.Mode)
	}

	snap := d.awaitConversation(spawned.Session.ID, 5*time.Minute, "the first turn to complete",
		func(s snapshot) bool {
			return len(s.Turns) >= 1 && terminal(s.Turns[0].State)
		})
	if snap.Mode != "chat" {
		t.Errorf("conversation mode = %q, want chat", snap.Mode)
	}
	if snap.Turns[0].State != "completed" {
		t.Fatalf("first turn ended as %q (err=%q):\n%s",
			snap.Turns[0].State, snap.Turns[0].ErrorMessage, describe(snap))
	}
	if !contains(snap.assistantText(), "SPAWNED") {
		t.Errorf("the spawn prompt was never answered:\n%s", describe(snap))
	}
}

// The model the user picks has to reach the provider, not just be accepted by AO.
// The catalog is read from the provider rather than hardcoded because the offered
// models change per account and per release.
func runChatModelOverrideScenario(t *testing.T, harness string) {
	t.Helper()
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, harness+"-model")
	session := harnessChatSession(t, d, project, harness, "Reply with exactly: READY")

	catalog := conversationModels(t, d, session)
	if len(catalog.Models) == 0 {
		t.Skipf("%s offered no models; there is no choice to override", harness)
	}
	if catalog.Selected.Model != "" {
		t.Errorf("a fresh conversation already has model %q selected", catalog.Selected.Model)
	}
	// Pick something that is not the default, so the assertion cannot pass by
	// accident when the choice is dropped on the floor.
	var chosen string
	for _, model := range catalog.Models {
		if model.ID == "" || model.DisplayName == "" {
			t.Errorf("model %+v is missing an id or a label, so it cannot be rendered", model)
		}
		if !model.Default && chosen == "" {
			chosen = model.ID
		}
	}
	if chosen == "" {
		t.Skipf("%s offers only one model; nothing to switch to", harness)
	}

	if applied := setSettings(t, d, session, map[string]any{"model": chosen}); applied.Model != chosen {
		t.Fatalf("settings echoed model %q, want %q", applied.Model, chosen)
	}
	// The snapshot is what the composer labels itself from, so the choice has to be
	// visible there and not only in the PATCH response.
	if got := d.conversation(session).Settings.Model; got != chosen {
		t.Errorf("snapshot reports model %q, want %q", got, chosen)
	}

	send(t, d, session, "Reply with exactly: MODEL_APPLIED", harness+"-model")
	snap := d.awaitConversation(session, 5*time.Minute, "a turn on the chosen model",
		func(s snapshot) bool { return terminal(s.Turns[len(s.Turns)-1].State) })
	last := snap.Turns[len(snap.Turns)-1]
	if last.State != "completed" {
		t.Fatalf("turn on model %q ended as %q (err=%q):\n%s",
			chosen, last.State, last.ErrorMessage, describe(snap))
	}
	if !contains(snap.assistantText(), "MODEL_APPLIED") {
		t.Errorf("the agent did not answer on the chosen model:\n%s", describe(snap))
	}
	// A model choice is sticky per conversation: the next turn must not silently
	// fall back to the provider default.
	if snap.Settings.Model != chosen {
		t.Errorf("recorded model = %q, want %q", snap.Settings.Model, chosen)
	}
}

// Reasoning effort is a second axis on the same PATCH. It is only meaningful when
// it survives into the snapshot the composer reads, because that is where the
// selected effort is labelled.
func runChatReasoningEffortScenario(t *testing.T, harness string) {
	t.Helper()
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, harness+"-effort")
	session := harnessChatSession(t, d, project, harness, "Reply with exactly: READY")

	catalog := conversationModels(t, d, session)
	var model, effort string
	for _, candidate := range catalog.Models {
		if len(candidate.Efforts) > 0 {
			model, effort = candidate.ID, candidate.Efforts[0]
			break
		}
	}
	if effort == "" {
		t.Skipf("%s offers no reasoning efforts; there is nothing to propagate", harness)
	}

	applied := setSettings(t, d, session, map[string]any{"model": model, "reasoningEffort": effort})
	if applied.Model != model || applied.ReasoningEffort != effort {
		t.Fatalf("settings echoed %+v, want model %q effort %q", applied, model, effort)
	}
	snap := d.conversation(session)
	if snap.Settings.ReasoningEffort != effort {
		t.Errorf("snapshot reports reasoning effort %q, want %q", snap.Settings.ReasoningEffort, effort)
	}

	// An effort the provider rejects fails the turn, which is what makes this more
	// than an echo test.
	send(t, d, session, "Reply with exactly: EFFORT_APPLIED", harness+"-effort")
	answered := d.awaitConversation(session, 5*time.Minute, "a turn on the chosen reasoning effort",
		func(s snapshot) bool { return terminal(s.Turns[len(s.Turns)-1].State) })
	last := answered.Turns[len(answered.Turns)-1]
	if last.State != "completed" {
		t.Fatalf("turn at effort %q ended as %q (err=%q):\n%s",
			effort, last.State, last.ErrorMessage, describe(answered))
	}
	if answered.Settings.ReasoningEffort != effort {
		t.Errorf("recorded reasoning effort = %q, want %q", answered.Settings.ReasoningEffort, effort)
	}
}

// The composer can override the project's permission posture for the turns that
// follow. Bypass is the one that must be honoured exactly: a project configured to
// ask, with the composer set to bypass, has to run without ever blocking on a
// person — and the file it was asked to write has to be on disk afterwards.
func runChatApprovalModeOverrideScenario(t *testing.T, harness string) {
	t.Helper()
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, harness+"-approval")
	// Without an asking posture underneath it, "did not prompt" would be true of
	// every mode and the override would prove nothing.
	setPermissions(t, d, project, "accept-edits")
	session := harnessChatSession(t, d, project, harness, "Reply with exactly: READY")

	if applied := setSettings(t, d, session,
		map[string]any{"approvalMode": "bypass-permissions"}); applied.ApprovalMode != "bypass-permissions" {
		t.Fatalf("settings echoed approval mode %q, want bypass-permissions", applied.ApprovalMode)
	}
	if got := d.conversation(session).Settings.ApprovalMode; got != "bypass-permissions" {
		t.Errorf("snapshot reports approval mode %q, want bypass-permissions", got)
	}

	const proof = "approval-override.txt"
	send(t, d, session,
		"Use the file editing tool (not the shell) to create "+proof+" containing ok, then reply with exactly: WROTE",
		harness+"-approval")
	snap := d.awaitConversation(session, 5*time.Minute, "the bypassed turn to settle",
		func(s snapshot) bool { return terminal(s.Turns[len(s.Turns)-1].State) })

	if pending, ok := snap.pendingApproval(); ok {
		t.Fatalf("a bypassed turn still blocked on approval %s (%q)", pending.RequestID, pending.Summary)
	}
	if asked := approvalActivities(snap); len(asked) != 0 {
		t.Errorf("a bypassed turn recorded %d approval request(s); the override was not applied", len(asked))
	}
	last := snap.Turns[len(snap.Turns)-1]
	if last.State != "completed" {
		t.Fatalf("bypassed turn ended as %q (err=%q):\n%s",
			last.State, last.ErrorMessage, describe(snap))
	}
	// The mode is only really bypassed if the work it was gating actually happened.
	content, err := os.ReadFile(filepath.Join(sessionWorkspace(t, d, session), proof))
	if err != nil || !strings.Contains(string(content), "ok") {
		t.Fatalf("%s = %q, %v; the unattended edit never landed:\n%s", proof, content, err, describe(snap))
	}
}

// An attachment chip claims the file is somewhere the agent can open. Staging has
// to put the bytes in the worktree, the reported path has to be the one they are
// at, and a second attachment must not overwrite the first — a chat session
// attaches repeatedly and an earlier message keeps pointing at its own file.
func runChatAttachmentsScenario(t *testing.T, harness string) {
	t.Helper()
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, harness+"-attach")
	session := harnessChatSession(t, d, project, harness, "Reply with exactly: READY")

	const (
		firstContent  = "attachment-content-ALPHA"
		secondContent = "attachment-content-BETA"
	)
	first := stageTextAttachment(t, d, session, firstContent)
	second := stageTextAttachment(t, d, session, secondContent)
	if first == second {
		t.Fatalf("the second attachment reused the path %q, overwriting the first", first)
	}

	workspace := sessionWorkspace(t, d, session)
	for path, want := range map[string]string{first: firstContent, second: secondContent} {
		if !strings.HasPrefix(path, ".ao/attachments/") || !strings.HasSuffix(path, ".txt") {
			t.Errorf("staged path = %q, want a .txt under .ao/attachments/", path)
		}
		got, err := os.ReadFile(filepath.Join(workspace, path))
		if err != nil || string(got) != want {
			t.Fatalf("attachment %s on disk = %q, %v; want %q", path, got, err, want)
		}
	}

	// The claim the chip makes, tested against the agent rather than the filesystem.
	send(t, d, session,
		"Read the files "+first+" and "+second+
			" and reply with their exact contents, one per line. Do not modify any files.",
		harness+"-attach")
	snap := d.awaitConversation(session, 5*time.Minute, "the agent to read both attachments",
		func(s snapshot) bool { return terminal(s.Turns[len(s.Turns)-1].State) })
	answer := snap.assistantText()
	if !contains(answer, firstContent) || !contains(answer, secondContent) {
		t.Fatalf("the agent could not read the staged attachments %s and %s:\n%s",
			first, second, describe(snap))
	}
}

// What the frontend renders after a run of operations has to be one consistent
// account: the harness it asked for, the settings it set, a terminal state for
// every turn it sent, and a timeline that explains how the work was done. A
// snapshot that disagrees with any of that is a UI showing something that did not
// happen.
func runChatServerStateScenario(t *testing.T, harness string) {
	t.Helper()
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, harness+"-state")
	session := harnessChatSession(t, d, project, harness, "Reply with exactly: READY")

	setSettings(t, d, session, map[string]any{"approvalMode": "bypass-permissions"})
	send(t, d, session,
		"Use the shell to run `printf state-proof > state.txt`, then reply with exactly: STATE_DONE",
		harness+"-state")
	snap := d.awaitConversation(session, 5*time.Minute, "every turn to settle",
		func(s snapshot) bool {
			return len(s.Turns) >= 2 && terminal(s.Turns[len(s.Turns)-1].State)
		})

	var read struct {
		Session struct {
			Harness string `json:"harness"`
			Mode    string `json:"mode"`
		} `json:"session"`
	}
	d.mustCall("GET", "/sessions/"+session, http.StatusOK, nil, &read)
	if read.Session.Harness != harness || read.Session.Mode != "chat" {
		t.Errorf("session reads back as harness %q mode %q, want %q chat",
			read.Session.Harness, read.Session.Mode, harness)
	}
	if snap.Settings.ApprovalMode != "bypass-permissions" {
		t.Errorf("snapshot approval mode = %q, want bypass-permissions", snap.Settings.ApprovalMode)
	}
	for _, tn := range snap.Turns {
		if tn.State != "completed" {
			t.Errorf("turn %s ended as %q (err=%q)", short(tn.ID), tn.State, tn.ErrorMessage)
		}
		if tn.ProviderTurnID == "" {
			t.Errorf("turn %s carries no provider turn id, so it cannot be correlated", short(tn.ID))
		}
	}
	if !contains(snap.assistantText(), "STATE_DONE") {
		t.Errorf("the last prompt is unanswered in the timeline:\n%s", describe(snap))
	}

	// One writer owns ordering: a duplicate sequence means two of them raced for a
	// position, which would make the timeline's order unreproducible.
	seen := map[int64]string{}
	for _, m := range snap.Messages {
		if where, dup := seen[m.Sequence]; dup {
			t.Fatalf("sequence %d handed out twice (%s and message %s)", m.Sequence, where, m.ID)
		}
		seen[m.Sequence] = "message " + m.ID
		if m.TurnID == "" {
			t.Errorf("message %s belongs to no turn", m.ID)
		}
	}
	// The tool work has to be accounted for, not just its result: an answer with no
	// activity behind it is a timeline that cannot explain itself.
	var tools int
	for _, a := range snap.Activities {
		if where, dup := seen[a.Sequence]; dup {
			t.Fatalf("sequence %d handed out twice (%s and activity %s)", a.Sequence, where, a.ID)
		}
		seen[a.Sequence] = "activity " + a.ID
		if a.Kind == "command" || a.Kind == "file_change" {
			tools++
		}
	}
	if tools == 0 {
		t.Errorf("no tool activity recorded for a turn that had to write a file:\n%s", describe(snap))
	}
	proof, err := os.ReadFile(filepath.Join(sessionWorkspace(t, d, session), "state.txt"))
	if err != nil || string(proof) != "state-proof" {
		t.Fatalf("state.txt = %q, %v; the timeline claims work that did not land", proof, err)
	}
}
