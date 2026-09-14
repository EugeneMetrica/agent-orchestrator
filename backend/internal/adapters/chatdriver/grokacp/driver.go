// Package grokacp binds the user's own Grok Build installation to AO's reusable
// ACP Chat transport.
package grokacp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	acpdriver "github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/acp"
	"github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/nativeacp"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// New launches `grok agent … stdio` from the exact binary resolved by the
// existing Grok agent plugin. Login, models, settings, and updates stay owned by
// the user's Grok installation; AO adds only the launch-time permission mode and
// model override.
func New(plugin nativeacp.Plugin, log *slog.Logger) ports.ChatDriver {
	return nativeacp.New(plugin, nativeacp.Config{
		Harness:              domain.HarnessGrok,
		Configure:            configure,
		SessionMode:          sessionMode,
		SessionOptions:       sessionOptions,
		ValidateTurnSettings: validateTurnSettings,
	}, log)
}

// configure builds `grok agent --no-auto-update [--always-approve]
// [--model <model>] stdio`. The `agent` subcommand selects ACP mode and `stdio`
// selects the JSON-RPC transport. `--no-auto-update` matches the TUI adapter so
// an AO-managed session never self-updates mid-run.
func configure(_ context.Context, cfg acpdriver.LaunchConfig) ([]string, map[string]string, error) {
	args := []string{"agent", "--no-auto-update"}
	if ports.NormalizePermissionMode(cfg.Permissions) == ports.PermissionModeBypassPermissions {
		args = append(args, "--always-approve")
	}
	if model := strings.TrimSpace(cfg.Model); model != "" {
		args = append(args, "--model", model)
	}
	return append(args, "stdio"), nil, nil
}

// sessionMode maps AO's approval vocabulary onto Grok's own mode ids, which are
// the same strings the TUI adapter passes to `--permission-mode`. Empty leaves
// the user's ~/.grok config default in place.
func sessionMode(permissions ports.PermissionMode) string {
	switch ports.NormalizePermissionMode(permissions) {
	case ports.PermissionModeAcceptEdits:
		return "acceptEdits"
	case ports.PermissionModeAuto:
		return "auto"
	case ports.PermissionModeBypassPermissions:
		return "bypassPermissions"
	default:
		return ""
	}
}

func sessionOptions(settings ports.ChatTurnSettings) []acpdriver.SessionOption {
	if model := strings.TrimSpace(settings.Model); model != "" {
		return []acpdriver.SessionOption{{ID: "model", Value: model}}
	}
	return nil
}

// Grok parses model overrides as provider/model. Reject a bare model name before
// spawning so the user gets a recoverable message instead of a provider-side
// launch failure. Model availability stays the user's Grok installation's answer.
func validateTurnSettings(_ ports.PermissionMode, settings ports.ChatTurnSettings) error {
	if settings.Model == "" {
		return nil
	}
	provider, model, found := strings.Cut(settings.Model, "/")
	if !found || strings.TrimSpace(provider) == "" || strings.TrimSpace(model) == "" {
		return fmt.Errorf("%w: Grok model %q must use provider/model format (for example, xai/grok-code-fast-1); select a full model ID from your Grok installation, or clear the model override to use Agent default", ports.ErrChatConfigOptionInvalid, settings.Model)
	}
	return nil
}
