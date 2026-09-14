// Package grokacp binds the user's own Grok installation to AO's reusable ACP
// Chat transport.
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

// New launches `grok agent stdio` from the exact binary resolved by the existing
// Grok agent plugin. AO adds only per-session permission mode and model override.
func New(plugin nativeacp.Plugin, log *slog.Logger) ports.ChatDriver {
	return nativeacp.New(plugin, nativeacp.Config{
		Harness:              domain.HarnessGrok,
		Configure:            configure,
		SessionMode:          sessionMode,
		SessionOptions:       sessionOptions,
		ValidateTurnSettings: validateTurnSettings,
	}, log)
}

// configure constructs spawn args for `grok agent [--always-approve] [--model M] [--no-auto-update] stdio`.
func configure(_ context.Context, cfg acpdriver.LaunchConfig) ([]string, map[string]string, error) {
	args := []string{"agent", "--no-auto-update"}

	if ports.NormalizePermissionMode(cfg.Permissions) == ports.PermissionModeBypassPermissions {
		args = append(args, "--always-approve")
	}

	if model := strings.TrimSpace(cfg.Model); model != "" {
		args = append(args, "--model", model)
	}

	args = append(args, "stdio")
	return args, nil, nil
}

// sessionMode maps AO permission modes to Grok ACP session mode strings.
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

// sessionOptions maps AO's per-turn choices onto ACP config option ids.
func sessionOptions(settings ports.ChatTurnSettings) []acpdriver.SessionOption {
	if settings.Model == "" {
		return nil
	}
	return []acpdriver.SessionOption{{ID: "model", Value: settings.Model}}
}

// validateTurnSettings validates model overrides to ensure they use provider/model format.
func validateTurnSettings(_ ports.PermissionMode, settings ports.ChatTurnSettings) error {
	if settings.Model == "" {
		return nil
	}
	provider, model, found := strings.Cut(settings.Model, "/")
	if !found || strings.TrimSpace(provider) == "" || strings.TrimSpace(model) == "" {
		return fmt.Errorf("%w: Grok model %q must use provider/model format "+
			"(for example, xai/grok-3); select a full model ID from `grok models`, "+
			"or clear the model override to use Agent default",
			ports.ErrChatConfigOptionInvalid, settings.Model)
	}
	return nil
}
