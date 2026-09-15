// Package grokacp binds the user's own Grok Build installation to AO's reusable
// ACP Chat transport.
package grokacp

import (
	"context"
	"log/slog"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

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
		Harness:          domain.HarnessGrok,
		Configure:        configure,
		SessionMeta:      sessionMeta,
		SessionMode:      sessionMode,
		SessionOptions:   sessionOptions,
		PermissionPolicy: permissionPolicy,
	}, log)
}

// configure builds `grok --no-auto-update agent [--always-approve]
// [--model <model>] stdio`. The `agent` subcommand selects ACP mode and `stdio`
// selects the JSON-RPC transport. `--no-auto-update` is a global flag and keeps
// the TUI adapter's position ahead of the subcommand, so an AO-managed session
// never self-updates mid-run.
//
// Standing instructions are deliberately absent from the argv: `--rules` is
// consumed by Grok's TUI and `-p` paths only, and `agent` mode ignores it. They
// travel through sessionMeta instead.
func configure(_ context.Context, cfg acpdriver.LaunchConfig) ([]string, map[string]string, error) {
	args := []string{"--no-auto-update", "agent"}
	if ports.NormalizePermissionMode(cfg.Permissions) == ports.PermissionModeBypassPermissions {
		args = append(args, "--always-approve")
	}
	if model := strings.TrimSpace(cfg.Model); model != "" {
		args = append(args, "--model", model)
	}
	return append(args, "stdio"), nil, nil
}

// sessionMeta delivers AO's standing instructions through the ACP session
// metadata Grok's agent mode reads. Grok folds `_meta.rules` into the
// `<human_rules>` section of its own system prompt, so AO's instructions are
// appended to — never a replacement for — what the user's installation
// configures.
//
// The shared transport also sends this metadata on session/load and
// session/resume, and AO keeps doing so for protocol correctness. Grok, however,
// documents `_meta.rules` as a session/new input: a reloaded session keeps the
// rules it was created with, and a session/load carrying different rules is
// observably ignored (proved on the wire by TestLiveGrokACPResume, where the
// resumed answer still bears the token from the original start). So resume
// preserves standing instructions but does not update them; AO must not rely on
// session/load to rewrite them, and a session whose standing instructions
// changed needs a new provider session. Re-sending stays worthwhile because it
// costs nothing today and a future Grok that honours it needs no AO change.
//
// Resume therefore needs no extension to nativeacp or a Grok-specific ACP
// transport: `rules` is the only `_meta` field AO sends. Grok's other documented
// extension, `_meta.yoloMode`, stays unused because AO already expresses
// bypass-permissions through the `bypassPermissions` session mode plus the
// launch-time `--always-approve` flag, and duplicating it in metadata would give
// the session a second, silent way to widen the user's approvals.
func sessionMeta(cfg acpdriver.LaunchConfig) map[string]any {
	prompt := strings.TrimSpace(cfg.SystemPrompt)
	if prompt == "" {
		return nil
	}
	return map[string]any{"rules": prompt}
}

// sessionMode maps AO's approval vocabulary onto Grok's own mode ids, which are
// the same strings the TUI adapter passes to `--permission-mode`. Empty leaves
// the user's ~/.grok config default in place. bypass-permissions is the one mode
// that is expressed twice: this session mode plus the launch-time
// `--always-approve` from configure, because the flag is what suppresses Grok's
// approval UI for tools the session mode alone would still route through it.
//
//	default            -> ""                   (Grok prompts for approvals)
//	accept-edits       -> "acceptEdits"        (file edits auto-approve)
//	auto               -> "auto"               (all tools auto-approve)
//	bypass-permissions -> "bypassPermissions"  (+ --always-approve)
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

// permissionPolicy resolves mid-session approvalMode changes that Grok still
// surfaces as session/request_permission. Launch-time bypass uses
// --always-approve so Grok never asks; a PATCH to bypass-permissions after an
// accept-edits spawn only gets SetSessionMode, and Grok keeps asking the client.
// Without a policy AO parks those requests forever, which is exactly what the
// Chat API approval-mode override must not do.
func permissionPolicy(
	mode ports.PermissionMode,
	params acpsdk.RequestPermissionRequest,
) (acpsdk.PermissionOptionId, bool) {
	mode = ports.NormalizePermissionMode(mode)
	if mode == ports.PermissionModeAcceptEdits {
		kind := acpsdk.ToolKind("")
		if params.ToolCall.Kind != nil {
			kind = *params.ToolCall.Kind
		}
		if kind != acpsdk.ToolKindEdit && kind != acpsdk.ToolKindDelete && kind != acpsdk.ToolKindMove {
			return "", false
		}
		return permissionOption(params.Options, acpsdk.PermissionOptionKindAllowOnce)
	}
	if mode == ports.PermissionModeAuto || mode == ports.PermissionModeBypassPermissions {
		if id, ok := permissionOption(params.Options, acpsdk.PermissionOptionKindAllowAlways); ok {
			return id, true
		}
		return permissionOption(params.Options, acpsdk.PermissionOptionKindAllowOnce)
	}
	return "", false
}

func permissionOption(
	options []acpsdk.PermissionOption,
	kind acpsdk.PermissionOptionKind,
) (acpsdk.PermissionOptionId, bool) {
	for _, option := range options {
		if option.Kind == kind {
			return option.OptionId, true
		}
	}
	return "", false
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
