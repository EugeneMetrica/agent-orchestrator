package grokacp

import (
	"context"
	"errors"
	"reflect"
	"testing"

	acpdriver "github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/acp"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestHarness(t *testing.T) {
	driver := New(&fakePlugin{binary: "/usr/bin/grok", status: ports.AgentAuthStatusAuthorized}, nil)
	if got := driver.Harness(); got != domain.HarnessGrok {
		t.Fatalf("Harness() = %q, want %q", got, domain.HarnessGrok)
	}
}

func TestConfigureDefaultPermissions(t *testing.T) {
	args, env, err := configure(context.Background(), acpdriver.LaunchConfig{})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	want := []string{"agent", "--no-auto-update", "stdio"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	if env != nil {
		t.Fatalf("env = %#v, want nil", env)
	}
}

func TestConfigureBypassPermissions(t *testing.T) {
	args, env, err := configure(context.Background(), acpdriver.LaunchConfig{
		Permissions: ports.PermissionModeBypassPermissions,
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	want := []string{"agent", "--no-auto-update", "--always-approve", "stdio"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	if env != nil {
		t.Fatalf("env = %#v, want nil", env)
	}
}

func TestConfigureAcceptEditsDoesNotIncludeAlwaysApprove(t *testing.T) {
	args, _, err := configure(context.Background(), acpdriver.LaunchConfig{
		Permissions: ports.PermissionModeAcceptEdits,
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	for _, arg := range args {
		if arg == "--always-approve" {
			t.Fatalf("args = %#v should not include --always-approve for acceptEdits", args)
		}
	}
}

func TestConfigureAutoDoesNotIncludeAlwaysApprove(t *testing.T) {
	args, _, err := configure(context.Background(), acpdriver.LaunchConfig{
		Permissions: ports.PermissionModeAuto,
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	for _, arg := range args {
		if arg == "--always-approve" {
			t.Fatalf("args = %#v should not include --always-approve for auto", args)
		}
	}
}

func TestConfigureModelOverride(t *testing.T) {
	args, _, err := configure(context.Background(), acpdriver.LaunchConfig{
		Model: "xai/grok-3",
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	want := []string{"agent", "--no-auto-update", "--model", "xai/grok-3", "stdio"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestConfigureModelAndBypassPermissions(t *testing.T) {
	args, _, err := configure(context.Background(), acpdriver.LaunchConfig{
		Model:       "xai/grok-3",
		Permissions: ports.PermissionModeBypassPermissions,
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	want := []string{"agent", "--no-auto-update", "--always-approve", "--model", "xai/grok-3", "stdio"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestSessionModeMapping(t *testing.T) {
	tests := []struct {
		permission ports.PermissionMode
		want       string
	}{
		{ports.PermissionModeDefault, ""},
		{ports.PermissionModeAcceptEdits, "acceptEdits"},
		{ports.PermissionModeAuto, "auto"},
		{ports.PermissionModeBypassPermissions, "bypassPermissions"},
	}
	for _, tt := range tests {
		t.Run(string(tt.permission), func(t *testing.T) {
			if got := sessionMode(tt.permission); got != tt.want {
				t.Fatalf("sessionMode(%q) = %q, want %q", tt.permission, got, tt.want)
			}
		})
	}
}

func TestSessionOptionsModel(t *testing.T) {
	if got := sessionOptions(ports.ChatTurnSettings{}); got != nil {
		t.Fatalf("empty settings = %#v, want nil", got)
	}
	got := sessionOptions(ports.ChatTurnSettings{Model: "xai/grok-3"})
	want := []acpdriver.SessionOption{{ID: "model", Value: "xai/grok-3"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sessionOptions = %#v, want %#v", got, want)
	}
}

func TestValidateTurnSettingsValidModel(t *testing.T) {
	for _, model := range []string{"xai/grok-3", "openrouter/vendor/model", "custom-provider/private-model:latest"} {
		t.Run(model, func(t *testing.T) {
			err := validateTurnSettings(ports.PermissionModeDefault, ports.ChatTurnSettings{Model: model})
			if err != nil {
				t.Fatalf("validateTurnSettings(%q) = %v, want nil", model, err)
			}
		})
	}
}

func TestValidateTurnSettingsInvalidModel(t *testing.T) {
	for _, model := range []string{"grok-3", "GrokModel", " ", "/model", "provider/", " /model", "provider/ "} {
		t.Run(model, func(t *testing.T) {
			err := validateTurnSettings(ports.PermissionModeDefault, ports.ChatTurnSettings{Model: model})
			if !errors.Is(err, ports.ErrChatConfigOptionInvalid) {
				t.Fatalf("validateTurnSettings(%q) = %v, want ErrChatConfigOptionInvalid", model, err)
			}
		})
	}
}

func TestValidateTurnSettingsEmptyModel(t *testing.T) {
	err := validateTurnSettings(ports.PermissionModeDefault, ports.ChatTurnSettings{Model: ""})
	if err != nil {
		t.Fatalf("validateTurnSettings(\"\") = %v, want nil", err)
	}
}

func TestDriverReusesGrokPluginForProbe(t *testing.T) {
	plugin := &fakePlugin{status: ports.AgentAuthStatusAuthorized, binary: "/usr/bin/grok"}
	driver := New(plugin, nil)

	caps, err := driver.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if driver.Harness() != domain.HarnessGrok {
		t.Fatalf("harness = %q, want grok", driver.Harness())
	}
	for _, capability := range []ports.ChatCapability{
		ports.ChatCapabilityStreaming,
		ports.ChatCapabilityTools,
		ports.ChatCapabilityApprovals,
		ports.ChatCapabilityInterrupt,
		ports.ChatCapabilityResume,
	} {
		if !caps.Has(capability) {
			t.Errorf("missing capability %q", capability)
		}
	}
	if plugin.resolveCalls != 1 || plugin.authCalls != 1 {
		t.Fatalf("plugin calls = resolve %d, auth %d; want one each",
			plugin.resolveCalls, plugin.authCalls)
	}
}

func TestDriverRejectsUnauthenticatedGrok(t *testing.T) {
	driver := New(
		&fakePlugin{status: ports.AgentAuthStatusUnauthorized, binary: "/usr/bin/grok"},
		nil,
	)
	if _, err := driver.Probe(context.Background()); !errors.Is(err, ports.ErrChatAuthRequired) {
		t.Fatalf("Probe error = %v, want ErrChatAuthRequired", err)
	}
}

func TestDriverNoSecretEnvInjection(t *testing.T) {
	args, env, err := configure(context.Background(), acpdriver.LaunchConfig{
		Model:       "xai/grok-3",
		Permissions: ports.PermissionModeBypassPermissions,
		Env:         map[string]string{"PATH": "/usr/bin"},
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	if env != nil {
		t.Fatalf("env = %#v, want nil (no credential injection)", env)
	}
	for _, arg := range args {
		if arg == "GROK_HOME" || arg == "XAI_API_KEY" {
			t.Fatalf("args should not include credential-related flags: %#v", args)
		}
	}
}

type fakePlugin struct {
	binary       string
	status       ports.AgentAuthStatus
	resolveCalls int
	authCalls    int
}

func (p *fakePlugin) ResolveBinary(context.Context) (string, error) {
	p.resolveCalls++
	return p.binary, nil
}

func (p *fakePlugin) AuthStatus(context.Context) (ports.AgentAuthStatus, error) {
	p.authCalls++
	return p.status, nil
}
