package grokacp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/grok"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// Live tests require:
// - Grok CLI installed and on PATH
// - Valid authentication (XAI_API_KEY or ~/.grok/auth.json)
// - AO_LIVE_GROK_ACP=1 environment variable
//
// Run explicitly: AO_LIVE_GROK_ACP=1 go test -v ./internal/adapters/chatdriver/grokacp/...
//
// It uses the user's existing Grok executable, account, and configuration; CI
// never depends on them. The test proves the full Probe→Start→SendTurn cycle
// against the real provider: AO's standing instructions must reach Grok through
// the ACP session metadata, and the turn must leave a file on disk rather than
// only describing the work.
func TestLiveGrokACP(t *testing.T) {
	if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
		t.Skip("set AO_LIVE_GROK_ACP=1 to run against the local Grok account")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	driver := New(grok.New(), nil)
	if _, err := driver.Probe(ctx); err != nil {
		t.Fatalf("Probe: %v", err)
	}

	workspace := t.TempDir()
	conv, err := driver.Start(ctx, ports.ChatStartConfig{
		SessionID: "live-grok-acp", DataDir: liveDataDir(t), WorkspacePath: workspace,
		Env: liveEnvMap(), Permissions: ports.PermissionModeDefault,
		SystemPrompt: "On every response include the exact token GROK_STANDING_TOKEN.",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer conv.(ports.ChatProviderTerminator).Terminate()

	if providerID := conv.ProviderConversationID(); providerID == "" {
		t.Fatalf("provider conversation id = %q, want non-empty", providerID)
	}
	if !conv.Capabilities()[ports.ChatCapabilityResume] {
		t.Fatalf("capabilities = %#v, want resume", conv.Capabilities())
	}

	ref := sendLiveTurn(ctx, t, conv,
		"Use the shell to run `printf grok-acp-ok > proof.txt`, then report success.")
	answer := waitForLiveTurn(ctx, t, conv, ref.ProviderTurnID, true)
	if !strings.Contains(answer, "GROK_STANDING_TOKEN") {
		t.Fatalf("answer omitted the standing instruction token: %q", answer)
	}
	proof, err := os.ReadFile(filepath.Join(workspace, "proof.txt"))
	if err != nil || string(proof) != "grok-acp-ok" {
		t.Fatalf("tool-created proof = %q, %v", proof, err)
	}
}

// Resume must recover the provider's own conversation, not just reopen a
// process against the same workspace. The test proves both halves of that:
// Grok has to recall a codeword it was told only in the pre-terminate session
// (provider-side history), and the file that turn wrote has to still be on disk
// and visible to the resumed agent (workspace continuity). AO's standing
// instructions are changed between the two phases so the resumed answer also
// proves the shared ACP transport re-sent `_meta.rules` on session restore
// rather than leaving the resumed session with the original prompt.
func TestLiveGrokACPResume(t *testing.T) {
	if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
		t.Skip("set AO_LIVE_GROK_ACP=1 to run against the local Grok account")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	driver := New(grok.New(), nil)
	if _, err := driver.Probe(ctx); err != nil {
		t.Fatalf("Probe: %v", err)
	}

	workspace := t.TempDir()
	dataDir := liveDataDir(t)
	const sessionID = domain.SessionID("live-grok-resume")
	conv, err := driver.Start(ctx, ports.ChatStartConfig{
		SessionID: sessionID, DataDir: dataDir, WorkspacePath: workspace,
		Env: liveEnvMap(), Permissions: ports.PermissionModeDefault,
		SystemPrompt: "On every response include the exact token GROK_STANDING_START.",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	providerID := conv.ProviderConversationID()
	if providerID == "" || !conv.Capabilities()[ports.ChatCapabilityResume] {
		t.Fatalf("provider id/resume = %q, %#v", providerID, conv.Capabilities())
	}

	// The codeword carries a random suffix so a resumed answer can only contain
	// it by recalling this session; a fixed word like ALPHA could be guessed or
	// echoed from the model's own priors and would pass without real history.
	codeword := "ALPHA-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	startRef := sendLiveTurn(ctx, t, conv,
		"Remember the codeword "+codeword+". Use the shell to run "+
			"`printf before-value > before.txt`, then report success.")
	startAnswer := waitForLiveTurn(ctx, t, conv, startRef.ProviderTurnID, true)
	if !strings.Contains(startAnswer, "GROK_STANDING_START") {
		t.Fatalf("start answer omitted the standing instruction token: %q", startAnswer)
	}
	before, err := os.ReadFile(filepath.Join(workspace, "before.txt"))
	if err != nil || string(before) != "before-value" {
		t.Fatalf("pre-terminate proof = %q, %v", before, err)
	}

	if err := conv.(ports.ChatProviderTerminator).Terminate(); err != nil {
		t.Fatalf("Terminate: %v", err)
	}

	resumed, err := driver.Resume(ctx, ports.ChatResumeConfig{
		SessionID: sessionID, ProviderConversationID: providerID,
		DataDir: dataDir, WorkspacePath: workspace, Env: liveEnvMap(),
		Permissions:  ports.PermissionModeDefault,
		SystemPrompt: "On every response include the exact token GROK_STANDING_RESUME.",
	})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	defer resumed.(ports.ChatProviderTerminator).Terminate()
	if got := resumed.ProviderConversationID(); got != providerID {
		t.Fatalf("resumed provider id = %q, want %q", got, providerID)
	}

	resumeRef := sendLiveTurn(ctx, t, resumed,
		"Repeat the codeword I gave you earlier and read before.txt to confirm it "+
			"still exists. Do not modify any files.")
	resumeAnswer := waitForLiveTurn(ctx, t, resumed, resumeRef.ProviderTurnID, true)
	if !strings.Contains(resumeAnswer, codeword) {
		t.Fatalf("resumed answer lost pre-terminate history (codeword %q): %q", codeword, resumeAnswer)
	}
	if !strings.Contains(resumeAnswer, "GROK_STANDING_RESUME") {
		t.Fatalf("resumed answer omitted the resume standing instruction token: %q", resumeAnswer)
	}
	if !strings.Contains(resumeAnswer, "before.txt") {
		t.Fatalf("resumed answer never referenced the pre-terminate file: %q", resumeAnswer)
	}

	after, err := os.ReadFile(filepath.Join(workspace, "before.txt"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("proof after resume = %q, %v; want unchanged %q", after, err, before)
	}
}

// Every AO permission mode must at least open and complete a native Grok ACP
// turn. TestSessionModeUsesGrokPermissionModeIDs asserts the exact mode ids and
// launch flags; this live matrix catches provider-side drift in how Grok honours
// them. Run explicitly because it consumes account turns and may surface real
// permission prompts.
func TestLiveGrokACPPermissionModes(t *testing.T) {
	if os.Getenv("AO_LIVE_GROK_ACP") != "1" {
		t.Skip("set AO_LIVE_GROK_ACP=1 to run against the local Grok account")
	}
	dataDir := liveDataDir(t)
	for _, test := range []struct {
		name string
		mode ports.PermissionMode
	}{
		{name: "default", mode: ports.PermissionModeDefault},
		{name: "accept-edits", mode: ports.PermissionModeAcceptEdits},
		{name: "auto", mode: ports.PermissionModeAuto},
		{name: "bypass-permissions", mode: ports.PermissionModeBypassPermissions},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			workspace := t.TempDir()
			driver := New(grok.New(), nil)
			conv, err := driver.Start(ctx, ports.ChatStartConfig{
				SessionID: domain.SessionID("live-grok-mode-" + test.name),
				DataDir:   dataDir, WorkspacePath: workspace, Env: liveEnvMap(), Permissions: test.mode,
			})
			if err != nil {
				t.Fatalf("Start(%s): %v", test.mode, err)
			}
			defer conv.(ports.ChatProviderTerminator).Terminate()

			// The proof file is named per mode so a stray write from another
			// mode's workspace can never be mistaken for this mode's work, and
			// the edit tool (not the shell) is requested so acceptEdits is
			// exercised on the path it actually governs.
			proofName := "mode-" + test.name + ".txt"
			ref := sendLiveTurnWithSettings(ctx, t, conv,
				"Use the file editing tool (not the shell) to create "+proofName+" containing ok, then say done.",
				ports.ChatTurnSettings{Approval: test.mode})
			waitForLiveTurn(ctx, t, conv, ref.ProviderTurnID, true)

			content, err := os.ReadFile(filepath.Join(workspace, proofName))
			if err != nil || strings.TrimSpace(string(content)) != "ok" {
				t.Fatalf("mode %s proof = %q, %v", test.mode, content, err)
			}
		})
	}
}

func sendLiveTurn(ctx context.Context, t *testing.T, conv ports.ChatConversation, text string) ports.ChatTurnRef {
	return sendLiveTurnWithSettings(ctx, t, conv, text, ports.ChatTurnSettings{})
}

func sendLiveTurnWithSettings(
	ctx context.Context,
	t *testing.T,
	conv ports.ChatConversation,
	text string,
	settings ports.ChatTurnSettings,
) ports.ChatTurnRef {
	t.Helper()
	ref, err := conv.SendTurn(ctx, ports.ChatUserMessage{
		Text: text, ClientMessageID: "live-" + time.Now().Format("150405.000000000"),
		Origin: domain.MessageOriginHuman, Settings: settings,
	})
	if err != nil {
		t.Fatalf("SendTurn: %v", err)
	}
	if err := conv.(ports.ChatDeferredTurnStarter).StartDeferredTurn(ref.ProviderTurnID); err != nil {
		t.Fatalf("StartDeferredTurn: %v", err)
	}
	return ref
}

func waitForLiveTurn(
	ctx context.Context,
	t *testing.T,
	conv ports.ChatConversation,
	turnID string,
	approve bool,
) string {
	t.Helper()
	var answer strings.Builder
	for {
		select {
		case event, ok := <-conv.Events():
			if !ok {
				t.Fatalf("controller closed before turn completion; answer=%q", answer.String())
			}
			if event.ProviderTurnID != "" && event.ProviderTurnID != turnID {
				continue
			}
			switch event.Kind {
			case ports.ChatEventMessageDelta:
				answer.WriteString(event.Delta)
			case ports.ChatEventApprovalRequested:
				if !approve || len(event.Decisions) == 0 {
					t.Fatalf("unexpected/unanswerable approval: %#v", event)
				}
				if err := conv.ResolveRequest(ctx, event.RequestID, ports.ChatDecision{
					ID: allowOnceDecision(event.Decisions).ID,
				}); err != nil {
					t.Fatalf("ResolveRequest: %v", err)
				}
			case ports.ChatEventTurnCompleted:
				if event.TurnState != domain.TurnStateCompleted {
					t.Fatalf("turn state = %q; answer=%q", event.TurnState, answer.String())
				}
				if err := conv.(ports.ChatProviderEventAcknowledger).AcknowledgeProviderEvent(
					context.Background(), event.ProviderEventID); err != nil {
					t.Fatalf("acknowledge: %v", err)
				}
				return answer.String()
			}
		case <-ctx.Done():
			t.Fatalf("live turn timed out: %v; answer=%q", ctx.Err(), answer.String())
		}
	}
}

// allowOnceDecision prefers the narrowest offered approval so a live run never
// widens the user's standing Grok permissions.
func allowOnceDecision(offered []ports.ChatDecisionOption) ports.ChatDecisionOption {
	for _, decision := range offered {
		var raw struct {
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal(decision.Raw, &raw)
		if raw.Kind == "allow_once" {
			return decision
		}
	}
	return offered[0]
}

func liveEnvMap() map[string]string {
	out := make(map[string]string)
	for _, pair := range os.Environ() {
		name, value, ok := strings.Cut(pair, "=")
		if ok {
			out[name] = value
		}
	}
	return out
}

func liveDataDir(t *testing.T) string {
	t.Helper()
	if dataDir := strings.TrimSpace(os.Getenv("AO_DATA_DIR")); dataDir != "" {
		return dataDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home directory: %v", err)
	}
	return filepath.Join(home, ".ao")
}
