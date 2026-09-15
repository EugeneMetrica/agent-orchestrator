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

// API-level scenarios for a Grok Chat session.
//
// The driver's own live tests prove Probe→Start→SendTurn against the provider.
// What they cannot prove is that AO's HTTP surface — the only surface a client
// has — reaches that driver: a spawn that picks the harness, a settings PATCH
// that the next turn actually runs under, a staged attachment that is really on
// disk where the returned path says it is. So these drive the whole chain
// (routes → service → driver → grok) and assert against server state and the
// worktree rather than against a fake.
//
// Gated twice over: AO_CHAT_E2E because they boot a real daemon, and
// AO_LIVE_GROK_ACP because they spend turns on the user's own Grok account.

const (
	grokGateEnv = "AO_LIVE_GROK_ACP"
	grokHarness = "grok"
)

// requireGrokE2E skips unless both gates are set and grok is installed.
//
// It deliberately does not call requireE2E: that helper also insists on the
// codex binary, which has nothing to do with a Grok session and would turn a
// perfectly runnable Grok machine into a silent skip.
func requireGrokE2E(t *testing.T) {
	t.Helper()
	requireLiveHarnessE2E(t, grokGateEnv, grokHarness)
}

// requireLiveHarnessE2E is the gate shared by the Grok tests and the Claude Code
// reference runs of the same scenarios.
func requireLiveHarnessE2E(t *testing.T, liveGateEnv, binary string) {
	t.Helper()
	if os.Getenv(gateEnv) != "1" {
		t.Skipf("set %s=1 to run the chat end-to-end suite (real daemon, real model calls)", gateEnv)
	}
	if os.Getenv(liveGateEnv) != "1" {
		t.Skipf("set %s=1 to run against the local %s account", liveGateEnv, binary)
	}
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("agent binary %q is not on PATH: %v", binary, err)
	}
}

// chatSessionWithHarness spawns a chat session on a named harness and waits for
// its spawn turn to settle, so a scenario starts from a known state. The
// package's chatSession helper is codex-only by design; this is the same shape
// with the harness left to the caller.
func chatSessionWithHarness(t *testing.T, d *daemon, project, harness, prompt string) string {
	t.Helper()
	id := spawn(t, d, map[string]any{
		"projectId": project, "kind": "worker", "harness": harness, "mode": "chat",
		"prompt": prompt,
	}).Session.ID
	d.awaitConversation(id, 3*time.Minute, "the session to finish its first turn",
		func(s snapshot) bool {
			return len(s.Turns) >= 1 && terminal(s.Turns[0].State)
		})
	return id
}

type modelChoice struct {
	ID            string   `json:"id"`
	DisplayName   string   `json:"displayName"`
	Default       bool     `json:"default"`
	Efforts       []string `json:"efforts"`
	DefaultEffort string   `json:"defaultEffort"`
}

type modelCatalog struct {
	Models   []modelChoice `json:"models"`
	Selected turnSettings  `json:"selected"`
}

func (d *daemon) models(session string) modelCatalog {
	d.t.Helper()
	var out modelCatalog
	d.mustCall("GET", "/sessions/"+session+"/conversation/models", http.StatusOK, nil, &out)
	return out
}

// workspacePath is the absolute worktree the session's agent runs in. It is the
// only way to check a claim about the filesystem from outside the daemon, which
// is what an attachment path is.
func (d *daemon) workspacePath(session string) string {
	d.t.Helper()
	var out struct {
		WorkspacePath string `json:"workspacePath"`
	}
	d.mustCall("GET", "/desktop/sessions/"+session+"/workspace", http.StatusOK, nil, &out)
	if out.WorkspacePath == "" {
		d.t.Fatalf("session %s reported no workspace path", session)
	}
	return out.WorkspacePath
}

// setTurnSettings records the provider choices for the next turn and returns
// what the daemon says is now stored.
func setTurnSettings(t *testing.T, d *daemon, session string, body map[string]any) turnSettings {
	t.Helper()
	var applied turnSettings
	d.mustCall("PATCH", "/sessions/"+session+"/conversation/settings", http.StatusOK, body, &applied)
	return applied
}

// A Chat spawn has to reach the Grok driver, not merely be accepted. The proof
// is the session's own harness plus an answer that only the provider could have
// produced.
func TestChatGrokSpawn(t *testing.T) {
	requireGrokE2E(t)
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, "grokspawn")

	session := spawn(t, d, map[string]any{
		"projectId": project, "kind": "worker", "harness": grokHarness, "mode": "chat",
		"prompt": "Reply with exactly: SPAWNED",
	})
	if session.Session.Harness != grokHarness {
		t.Errorf("harness = %q, want %q", session.Session.Harness, grokHarness)
	}
	if session.Session.Mode != "chat" {
		t.Errorf("mode = %q, want chat", session.Session.Mode)
	}

	snap := d.awaitConversation(session.Session.ID, 3*time.Minute, "the spawn turn to settle",
		func(s snapshot) bool {
			return len(s.Turns) >= 1 && terminal(s.Turns[0].State)
		})
	if got := snap.Turns[0].State; got != "completed" {
		t.Fatalf("spawn turn state = %q, want completed (err=%q)\n%s", got, snap.Turns[0].ErrorMessage, describe(snap))
	}
	if !contains(snap.assistantText(), "SPAWNED") {
		t.Errorf("the agent's answer is missing from the timeline:\n%s", describe(snap))
	}
}

func TestChatGrokModelOverride(t *testing.T) {
	requireGrokE2E(t)
	runChatModelOverride(t, grokHarness, "grokmodel")
}

// A model chosen through the API has to be the model the next turn runs on. The
// catalog is read from the provider rather than hardcoded: Grok's model ids are
// added, renamed, and gated per account, and a test naming one by hand stops
// testing anything the day it changes.
func runChatModelOverride(t *testing.T, harness, fixture string) {
	t.Helper()
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, fixture)
	session := chatSessionWithHarness(t, d, project, harness, "Reply with exactly: READY")

	catalog := d.models(session)
	if len(catalog.Models) == 0 {
		t.Skip("provider offered no models; there is no choice to apply")
	}
	// Nothing is chosen until the user chooses, so the provider's default applies.
	if catalog.Selected.Model != "" {
		t.Errorf("a fresh conversation already has model %q selected", catalog.Selected.Model)
	}
	// Pick something that is NOT the default, so the assertion cannot pass by
	// accident when the choice is ignored.
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
		t.Skip("provider offers only one model; nothing to switch to")
	}

	if applied := setTurnSettings(t, d, session, map[string]any{"model": chosen}); applied.Model != chosen {
		t.Fatalf("settings echoed model %q, want %q", applied.Model, chosen)
	}
	// The snapshot is what the composer labels itself from, so the choice has to
	// be visible there rather than only in the PATCH response.
	if got := d.conversation(session).Settings.Model; got != chosen {
		t.Errorf("snapshot reports model %q, want %q", got, chosen)
	}

	// And the next turn actually runs. A model the provider rejects fails the
	// turn, which is what makes this more than an echo test.
	send(t, d, session, "Reply with exactly: MODEL_APPLIED", "grok-model-switch")
	answered := d.awaitConversation(session, 3*time.Minute, "a turn on the chosen model",
		func(s snapshot) bool { return terminal(s.Turns[len(s.Turns)-1].State) })
	last := answered.Turns[len(answered.Turns)-1]
	if last.State != "completed" {
		t.Fatalf("turn on model %q ended as %q (err=%q)", chosen, last.State, last.ErrorMessage)
	}
	if !contains(answered.assistantText(), "MODEL_APPLIED") {
		t.Errorf("the agent did not answer on the chosen model:\n%s", describe(answered))
	}
	if answered.Settings.Model != chosen {
		t.Errorf("recorded model = %q, want %q", answered.Settings.Model, chosen)
	}
	// The provider is allowed to answer on a different model, but then it must say
	// so rather than letting the composer keep advertising the one that is not
	// replying.
	if answered.ModelReroute != nil {
		t.Logf("provider rerouted %s -> %s (%s)",
			answered.ModelReroute.FromModel, answered.ModelReroute.ToModel, answered.ModelReroute.Reason)
	}
}

// Reasoning effort is a per-turn provider choice like the model is. AO must
// store it, report it back on the snapshot, and hand it to a turn the provider
// still accepts.
func TestChatGrokReasoningEffort(t *testing.T) {
	requireGrokE2E(t)
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, "grokeffort")
	session := chatSessionWithHarness(t, d, project, grokHarness, "Reply with exactly: READY")

	catalog := d.models(session)
	var model modelChoice
	for _, candidate := range catalog.Models {
		if len(candidate.Efforts) > 0 {
			model = candidate
			break
		}
	}
	if model.ID == "" {
		t.Skip("no model advertises reasoning efforts; there is nothing to propagate")
	}
	// Prefer an effort that is not the model's default, so a dropped setting
	// cannot look like a pass.
	effort := model.Efforts[0]
	for _, candidate := range model.Efforts {
		if candidate != model.DefaultEffort {
			effort = candidate
			break
		}
	}

	applied := setTurnSettings(t, d, session, map[string]any{"model": model.ID, "reasoningEffort": effort})
	if applied.ReasoningEffort != effort {
		t.Fatalf("settings echoed reasoning effort %q, want %q", applied.ReasoningEffort, effort)
	}
	snap := d.conversation(session)
	if snap.Settings.ReasoningEffort != effort {
		t.Errorf("snapshot reports reasoning effort %q, want %q", snap.Settings.ReasoningEffort, effort)
	}
	if snap.Settings.Model != model.ID {
		t.Errorf("snapshot reports model %q, want %q", snap.Settings.Model, model.ID)
	}

	// An effort the provider will not accept fails the turn, so a completed turn
	// is the observable half of propagation.
	send(t, d, session,
		"Think about whether 91 is prime, then reply with exactly: EFFORT_APPLIED",
		"grok-effort")
	answered := d.awaitConversation(session, 5*time.Minute, "a turn at the chosen reasoning effort",
		func(s snapshot) bool { return terminal(s.Turns[len(s.Turns)-1].State) })
	last := answered.Turns[len(answered.Turns)-1]
	if last.State != "completed" {
		t.Fatalf("turn at effort %q ended as %q (err=%q)\n%s", effort, last.State, last.ErrorMessage, describe(answered))
	}
	if !contains(answered.assistantText(), "EFFORT_APPLIED") {
		t.Errorf("the agent did not answer at the chosen effort:\n%s", describe(answered))
	}
	if answered.Settings.ReasoningEffort != effort {
		t.Errorf("recorded reasoning effort = %q, want %q", answered.Settings.ReasoningEffort, effort)
	}
}

// Approval mode is the one setting where being ignored is dangerous in both
// directions. This asserts the permissive direction end to end: a session whose
// project posture would otherwise ask must stop asking for the turn the user
// widened, and the work must actually happen.
func TestChatGrokApprovalModeOverride(t *testing.T) {
	requireGrokE2E(t)
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, "grokapproval")
	// The default posture never asks, so an override to bypass-permissions would
	// be indistinguishable from doing nothing. accept-edits is a posture that
	// still routes shell commands through approval.
	setPermissions(t, d, project, "accept-edits")
	session := chatSessionWithHarness(t, d, project, grokHarness, "Reply with exactly: READY")

	applied := setTurnSettings(t, d, session, map[string]any{"approvalMode": "bypass-permissions"})
	if applied.ApprovalMode != "bypass-permissions" {
		t.Fatalf("settings echoed approval mode %q, want bypass-permissions", applied.ApprovalMode)
	}

	send(t, d, session,
		"Use the shell to run `printf bypassed > approval-proof.txt`, then reply with exactly: BYPASSED",
		"grok-approval-bypass")
	answered := d.awaitConversation(session, 3*time.Minute, "the bypassed turn to settle",
		func(s snapshot) bool { return terminal(s.Turns[len(s.Turns)-1].State) })

	if last := answered.Turns[len(answered.Turns)-1]; last.State != "completed" {
		t.Fatalf("bypassed turn ended as %q (err=%q)\n%s", last.State, last.ErrorMessage, describe(answered))
	}
	for _, a := range answered.Activities {
		if a.Kind == "approval" {
			t.Errorf("an approval was requested under bypass-permissions: %s (%s)", a.Summary, a.Status)
		}
	}
	proof := filepath.Join(d.workspacePath(session), "approval-proof.txt")
	content, err := os.ReadFile(proof)
	if err != nil || strings.TrimSpace(string(content)) != "bypassed" {
		t.Fatalf("file created without approval = %q, %v\n%s", content, err, describe(answered))
	}
}

func TestChatGrokAttachments(t *testing.T) {
	requireGrokE2E(t)
	runChatAttachments(t, grokHarness, "grokattach")
}

// An attachment path returned by the API is a promise about the worktree: the
// bytes are there, under that exact relative path, and the agent can open them.
// Each half is checked separately because either can be true without the other.
func runChatAttachments(t *testing.T, harness, fixture string) {
	t.Helper()
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, fixture)
	session := chatSessionWithHarness(t, d, project, harness, "Reply with exactly: READY")

	// Two files in one request: names must not collide, and a message has to be
	// able to reference every one of them.
	contents := []string{"attachment-one-content", "attachment-two-content"}
	payload := make([]map[string]any, 0, len(contents))
	for _, content := range contents {
		payload = append(payload, map[string]any{
			"mimeType": "text/plain",
			"data":     base64.StdEncoding.EncodeToString([]byte(content)),
		})
	}
	var staged struct {
		SessionID string   `json:"sessionId"`
		Paths     []string `json:"paths"`
	}
	d.mustCall("POST", "/sessions/"+session+"/attachments", http.StatusCreated,
		map[string]any{"attachments": payload}, &staged)

	if len(staged.Paths) != len(contents) {
		t.Fatalf("staged %d paths, want %d: %+v", len(staged.Paths), len(contents), staged.Paths)
	}
	workspace := d.workspacePath(session)
	seen := map[string]bool{}
	for i, path := range staged.Paths {
		if !strings.HasPrefix(path, ".ao/attachments/") || !strings.HasSuffix(path, ".txt") {
			t.Errorf("staged path = %q, want a .txt under .ao/attachments/", path)
		}
		if seen[path] {
			t.Fatalf("path %q was handed out twice, so one attachment overwrote the other", path)
		}
		seen[path] = true
		// Paths are returned in submission order, so the bytes at each one have to
		// be the bytes that were sent for it.
		onDisk, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(path)))
		if err != nil || string(onDisk) != contents[i] {
			t.Fatalf("attachment %s on disk = %q, %v; want %q", path, onDisk, err, contents[i])
		}
	}

	// The claim the chip makes, tested against the agent rather than the
	// filesystem: what the API returned is what the agent can open.
	prompt := "Read " + strings.Join(staged.Paths, " and ") +
		" using the shell, then reply with the exact contents of both files."
	send(t, d, session, prompt, "grok-attach-read")
	answered := d.awaitConversation(session, 3*time.Minute, "the agent to read both attachments",
		func(s snapshot) bool { return terminal(s.Turns[len(s.Turns)-1].State) })

	answer := answered.assistantText()
	for _, content := range contents {
		if !contains(answer, content) {
			t.Errorf("the agent did not report %q from the staged attachments:\n%s", content, describe(answered))
		}
	}
	// The message AO recorded must name every path, or the transcript no longer
	// explains what the agent was asked to open.
	var sent string
	for _, m := range answered.Messages {
		if m.Role == "user" && strings.Contains(m.Text, ".ao/attachments/") {
			sent = m.Text
		}
	}
	for _, path := range staged.Paths {
		if !strings.Contains(sent, path) {
			t.Errorf("recorded user message does not reference %q: %q", path, sent)
		}
	}
}

// One session, several API operations, one read: what the client sees afterwards
// has to be a faithful account of everything it asked for. A per-endpoint test
// can pass while the combined state is inconsistent, which is the failure this
// is looking for.
func TestChatGrokServerStateConsistency(t *testing.T) {
	requireGrokE2E(t)
	d := startDaemon(t, t.TempDir())
	project := seedProject(t, d, "grokstate")
	session := chatSessionWithHarness(t, d, project, grokHarness, "Reply with exactly: READY")

	settings := map[string]any{"approvalMode": "bypass-permissions"}
	catalog := d.models(session)
	var wantModel string
	for _, model := range catalog.Models {
		if !model.Default {
			wantModel = model.ID
			settings["model"] = model.ID
			break
		}
	}
	setTurnSettings(t, d, session, settings)

	// A turn that has to use a tool, so the timeline has something to account for
	// beyond an answer.
	turnID, state := send(t, d, session,
		"List the files in this repo using the shell, then reply with exactly: LISTED",
		"grok-state")
	if turnID == "" {
		t.Fatal("send returned no turn id")
	}
	// A turn starts life undispatched or running and ends terminal; anything else
	// means the client was told about a state the lifecycle does not have.
	switch state {
	case "queued", "running":
	default:
		t.Errorf("send reported state %q, want queued or running", state)
	}

	snap := d.awaitConversation(session, 3*time.Minute, "the turn to settle", func(s snapshot) bool {
		turn, ok := s.turnByID(turnID)
		return ok && terminal(turn.State)
	})

	var read struct {
		Session struct {
			Harness string `json:"harness"`
			Mode    string `json:"mode"`
		} `json:"session"`
	}
	d.mustCall("GET", "/sessions/"+session, http.StatusOK, nil, &read)
	if read.Session.Harness != grokHarness {
		t.Errorf("session harness = %q, want %q", read.Session.Harness, grokHarness)
	}
	if read.Session.Mode != "chat" {
		t.Errorf("session mode = %q, want chat", read.Session.Mode)
	}

	if snap.Settings.ApprovalMode != "bypass-permissions" {
		t.Errorf("settings.approvalMode = %q, want bypass-permissions", snap.Settings.ApprovalMode)
	}
	if wantModel != "" && snap.Settings.Model != wantModel {
		t.Errorf("settings.model = %q, want %q", snap.Settings.Model, wantModel)
	}

	final, _ := snap.turnByID(turnID)
	if final.State != "completed" {
		t.Fatalf("turn %s ended as %q (err=%q)\n%s", short(turnID), final.State, final.ErrorMessage, describe(snap))
	}
	if final.ProviderTurnID == "" {
		t.Errorf("completed turn %s carries no provider turn id", short(turnID))
	}
	for _, turn := range snap.Turns {
		if !terminal(turn.State) {
			t.Errorf("turn %s is still %q after the conversation settled", short(turn.ID), turn.State)
		}
	}
	if !contains(snap.assistantText(), "LISTED") {
		t.Errorf("the agent's answer is missing from the timeline:\n%s", describe(snap))
	}

	// Tool use has to be on the timeline, or the answer arrives with no account of
	// how it was reached. Grok's ACP agent reports shell tools without ToolKind
	// execute, so the shared ACP mapper records them as mcp_tool rather than
	// command; either kind proves the turn reached a tool.
	tools := 0
	for _, a := range snap.Activities {
		if a.TurnID == turnID && (a.Kind == "command" || a.Kind == "mcp_tool") {
			tools++
		}
		if a.Kind == "" || a.Status == "" {
			t.Errorf("activity %s has no kind or status, so it cannot be rendered: %+v", a.ID, a)
		}
	}
	if tools == 0 {
		t.Errorf("no tool activity recorded for a turn that had to read the repo:\n%s", describe(snap))
	}
}
