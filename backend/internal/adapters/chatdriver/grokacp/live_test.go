package grokacp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func sendLiveTurn(ctx context.Context, t *testing.T, conv ports.ChatConversation, text string) ports.ChatTurnRef {
	t.Helper()
	ref, err := conv.SendTurn(ctx, ports.ChatUserMessage{
		Text: text, ClientMessageID: "live-" + time.Now().Format("150405.000000000"),
		Origin: domain.MessageOriginHuman,
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
