package grokacp

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	acpdriver "github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/acp"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestConfigureSpawnsGrokACPStdio(t *testing.T) {
	tests := []struct {
		name string
		cfg  acpdriver.LaunchConfig
		want []string
	}{
		{name: "defaults", want: []string{"agent", "--no-auto-update", "stdio"}},
		{
			name: "accept edits keeps the provider default flagless",
			cfg:  acpdriver.LaunchConfig{Permissions: ports.PermissionModeAcceptEdits},
			want: []string{"agent", "--no-auto-update", "stdio"},
		},
		{
			name: "bypass permissions always approves",
			cfg:  acpdriver.LaunchConfig{Permissions: ports.PermissionModeBypassPermissions},
			want: []string{"agent", "--no-auto-update", "--always-approve", "stdio"},
		},
		{
			name: "model override",
			cfg:  acpdriver.LaunchConfig{Model: "xai/grok-code-fast-1"},
			want: []string{"agent", "--no-auto-update", "--model", "xai/grok-code-fast-1", "stdio"},
		},
		{
			name: "model override and bypass permissions",
			cfg:  acpdriver.LaunchConfig{Model: "xai/grok-code-fast-1", Permissions: ports.PermissionModeBypassPermissions},
			want: []string{"agent", "--no-auto-update", "--always-approve", "--model", "xai/grok-code-fast-1", "stdio"},
		},
		{
			name: "blank model is not forwarded",
			cfg:  acpdriver.LaunchConfig{Model: "   "},
			want: []string{"agent", "--no-auto-update", "stdio"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, env, err := configure(context.Background(), tt.cfg)
			if err != nil {
				t.Fatalf("configure: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) || env != nil {
				t.Fatalf("args/env = %#v, %#v; want %#v, nil", got, env, tt.want)
			}
		})
	}
}

// AO never injects credentials or rewrites GROK_HOME: the launch carries no
// environment overlay, so Chat and TUI sessions share one provider profile.
func TestConfigureAddsNoEnvironmentOverlay(t *testing.T) {
	_, env, err := configure(context.Background(), acpdriver.LaunchConfig{
		Env:          map[string]string{"GROK_HOME": "/home/user/.grok"},
		SystemPrompt: "Follow AO worker rules.",
		Permissions:  ports.PermissionModeBypassPermissions,
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	if env != nil {
		t.Fatalf("env overlay = %#v, want nil", env)
	}
}

func TestSessionModeUsesGrokPermissionModeIDs(t *testing.T) {
	tests := map[ports.PermissionMode]string{
		ports.PermissionModeDefault:           "",
		"":                                    "",
		ports.PermissionModeAcceptEdits:       "acceptEdits",
		ports.PermissionModeAuto:              "auto",
		ports.PermissionModeBypassPermissions: "bypassPermissions",
	}
	for permission, want := range tests {
		if got := sessionMode(permission); got != want {
			t.Errorf("sessionMode(%q) = %q, want %q", permission, got, want)
		}
	}
}

func TestSessionOptionsUseAdvertisedModelOption(t *testing.T) {
	if got := sessionOptions(ports.ChatTurnSettings{}); got != nil {
		t.Fatalf("empty settings = %#v", got)
	}
	got := sessionOptions(ports.ChatTurnSettings{Model: "xai/grok-code-fast-1"})
	want := []acpdriver.SessionOption{{ID: "model", Value: "xai/grok-code-fast-1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("settings = %#v, want %#v", got, want)
	}
}

func TestValidateTurnSettingsModelFormat(t *testing.T) {
	for _, model := range []string{"", "xai/grok-code-fast-1", "openrouter/x-ai/grok-4", "custom/private-model:latest"} {
		t.Run("valid/"+model, func(t *testing.T) {
			if err := validateTurnSettings(ports.PermissionModeDefault, ports.ChatTurnSettings{Model: model}); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, model := range []string{"grok-4", " ", "/model", "provider/", " /model", "provider/ "} {
		t.Run("invalid/"+model, func(t *testing.T) {
			err := validateTurnSettings(ports.PermissionModeDefault, ports.ChatTurnSettings{Model: model})
			if !errors.Is(err, ports.ErrChatConfigOptionInvalid) {
				t.Fatalf("error = %v, want ErrChatConfigOptionInvalid", err)
			}
		})
	}
}

// An invalid model must be refused before AO resolves or spawns the user's Grok
// binary, so a typo cannot start a process that immediately fails.
func TestRejectsBareModelNameBeforeResolvingBinary(t *testing.T) {
	for _, resume := range []bool{false, true} {
		name := "start"
		if resume {
			name = "resume"
		}
		t.Run(name, func(t *testing.T) {
			plugin := &unresolvedPlugin{}
			driver := New(plugin, nil)
			workspace := t.TempDir()
			var err error
			if resume {
				_, err = driver.Resume(context.Background(), ports.ChatResumeConfig{
					WorkspacePath: workspace, Model: "grok-4", ProviderConversationID: "existing",
				})
			} else {
				_, err = driver.Start(context.Background(), ports.ChatStartConfig{WorkspacePath: workspace, Model: "grok-4"})
			}
			if !errors.Is(err, ports.ErrChatConfigOptionInvalid) || !strings.Contains(err.Error(), "provider/model") {
				t.Fatalf("error = %v, want model format validation with recovery guidance", err)
			}
			if plugin.resolved {
				t.Fatal("invalid model reached Grok binary resolution")
			}
		})
	}
}

func TestDriverReusesGrokPluginForProbe(t *testing.T) {
	plugin := &fakePlugin{status: ports.AgentAuthStatusAuthorized, binary: "/usr/local/bin/grok"}
	driver := New(plugin, nil)

	caps, err := driver.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if driver.Harness() != domain.HarnessGrok {
		t.Fatalf("harness = %q, want %q", driver.Harness(), domain.HarnessGrok)
	}
	for _, capability := range []ports.ChatCapability{
		ports.ChatCapabilityStreaming, ports.ChatCapabilityTools, ports.ChatCapabilityApprovals,
		ports.ChatCapabilityInterrupt, ports.ChatCapabilityResume,
	} {
		if !caps.Has(capability) {
			t.Errorf("missing capability %q", capability)
		}
	}
	if plugin.resolveCalls != 1 || plugin.authCalls != 1 {
		t.Fatalf("plugin calls = resolve %d, auth %d; want one each", plugin.resolveCalls, plugin.authCalls)
	}
}

func TestDriverRejectsUnauthenticatedGrok(t *testing.T) {
	driver := New(&fakePlugin{status: ports.AgentAuthStatusUnauthorized, binary: "/usr/local/bin/grok"}, nil)
	if _, err := driver.Probe(context.Background()); !errors.Is(err, ports.ErrChatAuthRequired) {
		t.Fatalf("Probe error = %v, want ErrChatAuthRequired", err)
	}
}

type unresolvedPlugin struct{ resolved bool }

func (p *unresolvedPlugin) ResolveBinary(context.Context) (string, error) {
	p.resolved = true
	return "", errors.New("unexpected binary resolution")
}

func (*unresolvedPlugin) AuthStatus(context.Context) (ports.AgentAuthStatus, error) {
	return ports.AgentAuthStatusUnknown, nil
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
