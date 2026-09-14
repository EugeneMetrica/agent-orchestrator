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
		{name: "defaults", want: []string{"--no-auto-update", "agent", "stdio"}},
		{
			name: "accept edits keeps the provider default flagless",
			cfg:  acpdriver.LaunchConfig{Permissions: ports.PermissionModeAcceptEdits},
			want: []string{"--no-auto-update", "agent", "stdio"},
		},
		{
			name: "auto keeps the provider default flagless",
			cfg:  acpdriver.LaunchConfig{Permissions: ports.PermissionModeAuto},
			want: []string{"--no-auto-update", "agent", "stdio"},
		},
		{
			name: "bypass permissions always approves",
			cfg:  acpdriver.LaunchConfig{Permissions: ports.PermissionModeBypassPermissions},
			want: []string{"--no-auto-update", "agent", "--always-approve", "stdio"},
		},
		{
			name: "model override forwards the id Grok advertises",
			cfg:  acpdriver.LaunchConfig{Model: "grok-code-fast"},
			want: []string{"--no-auto-update", "agent", "--model", "grok-code-fast", "stdio"},
		},
		{
			name: "model override and bypass permissions",
			cfg:  acpdriver.LaunchConfig{Model: "grok-4.5", Permissions: ports.PermissionModeBypassPermissions},
			want: []string{"--no-auto-update", "agent", "--always-approve", "--model", "grok-4.5", "stdio"},
		},
		{
			name: "blank model is not forwarded",
			cfg:  acpdriver.LaunchConfig{Model: "   "},
			want: []string{"--no-auto-update", "agent", "stdio"},
		},
		{
			name: "standing instructions ride the global --rules flag",
			cfg:  acpdriver.LaunchConfig{SystemPrompt: "ao standing instructions"},
			want: []string{"--no-auto-update", "--rules", "ao standing instructions", "agent", "stdio"},
		},
		{
			name: "standing instructions combine with permissions and model",
			cfg: acpdriver.LaunchConfig{
				SystemPrompt: "ao standing instructions",
				Model:        "grok-code-fast",
				Permissions:  ports.PermissionModeBypassPermissions,
			},
			want: []string{
				"--no-auto-update", "--rules", "ao standing instructions",
				"agent", "--always-approve", "--model", "grok-code-fast", "stdio",
			},
		},
		{
			name: "blank standing instructions add no rules flag",
			cfg:  acpdriver.LaunchConfig{SystemPrompt: "  \n "},
			want: []string{"--no-auto-update", "agent", "stdio"},
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

// Chat delivers AO's standing instructions exactly like the TUI adapter does:
// `--rules` appends to whatever system prompt the user's own Grok installation
// configures, so AO never overrides it.
func TestConfigureAppendsStandingInstructionsAsRules(t *testing.T) {
	args, _, err := configure(context.Background(), acpdriver.LaunchConfig{
		SystemPrompt: "ao standing instructions",
		Permissions:  ports.PermissionModeBypassPermissions,
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	rules := indexOf(args, "--rules")
	if rules < 0 || rules+1 >= len(args) || args[rules+1] != "ao standing instructions" {
		t.Fatalf("args = %#v, want --rules followed by the standing instructions", args)
	}
	if agent := indexOf(args, "agent"); agent < rules {
		t.Fatalf("args = %#v, want --rules before the agent subcommand", args)
	}
	if strings.Contains(strings.Join(args, " "), "system-prompt-override") {
		t.Fatalf("args = %#v must append rules, not override Grok's system prompt", args)
	}
}

func TestConfigureOmitsRulesWithoutStandingInstructions(t *testing.T) {
	args, _, err := configure(context.Background(), acpdriver.LaunchConfig{Permissions: ports.PermissionModeAcceptEdits})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	if indexOf(args, "--rules") >= 0 {
		t.Fatalf("args = %#v, want no --rules flag for an empty system prompt", args)
	}
}

func indexOf(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
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
	// Ids in the shape `grok models` reports and modelcatalog parses; AO keeps no
	// Grok model list of its own.
	for _, model := range []string{"grok-code-fast", "grok-4.5"} {
		got := sessionOptions(ports.ChatTurnSettings{Model: model})
		want := []acpdriver.SessionOption{{ID: "model", Value: model}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("settings for %q = %#v, want %#v", model, got, want)
		}
	}
}

// AO does not gate Grok model ids on a provider/model shape: the AO catalog and
// the TUI adapter both use bare ids such as `grok-code-fast`. The id travels to
// the launch, and the ACP session's advertised models decide availability.
func TestBareModelNameReachesGrokLaunch(t *testing.T) {
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
					SessionID: "sess-grok", DataDir: t.TempDir(), WorkspacePath: workspace,
					Model: "grok-code-fast", ProviderConversationID: "existing",
				})
			} else {
				_, err = driver.Start(context.Background(), ports.ChatStartConfig{
					SessionID: "sess-grok", DataDir: t.TempDir(), WorkspacePath: workspace,
					Model: "grok-code-fast",
				})
			}
			if errors.Is(err, ports.ErrChatConfigOptionInvalid) {
				t.Fatalf("error = %v, want no model format gate", err)
			}
			if !plugin.resolved {
				t.Fatal("bare model id never reached Grok binary resolution")
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
