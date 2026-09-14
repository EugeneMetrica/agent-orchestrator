// Package grokacp binds the user's own Grok Build installation to AO's reusable
// ACP Chat transport.
package grokacp

import (
	"context"
	"log/slog"
	"strings"

	acpdriver "github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/acp"
	"github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/nativeacp"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// New launches `grok --no-auto-update agent … stdio` from the exact binary
// resolved by the existing Grok agent plugin. Login, models, settings, and
// updates stay owned by the user's Grok installation; AO adds only the
// launch-time permission mode and model override.
func New(plugin nativeacp.Plugin, log *slog.Logger) ports.ChatDriver {
	return nativeacp.New(plugin, nativeacp.Config{
		Harness:        domain.HarnessGrok,
		Configure:      configure,
		SessionMode:    sessionMode,
		SessionOptions: sessionOptions,
	}, log)
}

// configure builds `grok --no-auto-update [--rules <text>] agent
// [--always-approve] [--model <model>] stdio`. The `agent` subcommand selects
// ACP mode and `stdio` selects the JSON-RPC transport. `--no-auto-update` and
// `--rules` are global flags and keep the TUI adapter's position ahead of the
// subcommand, so an AO-managed session never self-updates mid-run and receives
// AO's standing instructions the same way a TUI session does.
func configure(_ context.Context, cfg acpdriver.LaunchConfig) ([]string, map[string]string, error) {
	args := []string{"--no-auto-update"}
	// Grok appends --rules to its own system prompt rather than replacing it,
	// which is why AO passes standing instructions through this flag.
	if prompt := strings.TrimSpace(cfg.SystemPrompt); prompt != "" {
		args = append(args, "--rules", prompt)
	}
	args = append(args, "agent")
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

// sessionOptions forwards the durable model choice verbatim, exactly as the TUI
// adapter passes it to `grok --model`. Grok accepts bare ids such as
// `grok-code-fast` alongside qualified ones, so AO does not impose a format;
// availability is answered by the models the ACP session advertises.
func sessionOptions(settings ports.ChatTurnSettings) []acpdriver.SessionOption {
	if model := strings.TrimSpace(settings.Model); model != "" {
		return []acpdriver.SessionOption{{ID: "model", Value: model}}
	}
	return nil
}
