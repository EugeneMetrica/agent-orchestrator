package acp

import (
	"context"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func selectOption(id, name, current string, values ...string) acpsdk.SessionConfigOption {
	if len(values) == 0 {
		values = []string{current}
	}
	ungrouped := make(acpsdk.SessionConfigSelectOptionsUngrouped, 0, len(values))
	for _, value := range values {
		ungrouped = append(ungrouped, acpsdk.SessionConfigSelectOption{
			Value: acpsdk.SessionConfigValueId(value),
			Name:  value,
		})
	}
	return acpsdk.SessionConfigOption{
		Select: &acpsdk.SessionConfigOptionSelect{
			Id:           acpsdk.SessionConfigId(id),
			Name:         name,
			CurrentValue: acpsdk.SessionConfigValueId(current),
			Options:      acpsdk.SessionConfigSelectOptions{Ungrouped: &ungrouped},
		},
	}
}

func boolOption(id, name string, current bool) acpsdk.SessionConfigOption {
	return acpsdk.SessionConfigOption{
		Boolean: &acpsdk.SessionConfigOptionBoolean{
			Id:           acpsdk.SessionConfigId(id),
			Name:         name,
			CurrentValue: current,
		},
	}
}

// The session/update notification documents itself as a complete replacement
// "including removing an option", so an empty catalog from that channel is a
// real statement about the session and must apply verbatim. Swallowing it would
// leave a picker offering options the agent has withdrawn.
func TestReplaceConfigOptionsAppliesEmptyCatalogVerbatim(t *testing.T) {
	c := &conversation{capabilities: make(ports.ChatCapabilities)}
	c.replaceConfigOptions([]acpsdk.SessionConfigOption{selectOption("model", "Model", "sonnet")})
	if got := len(c.configOptions); got != 1 {
		t.Fatalf("seed catalog: got %d options, want 1", got)
	}

	c.replaceConfigOptions(nil)

	if got := len(c.configOptions); got != 0 {
		t.Fatalf("authoritative empty replacement was ignored: got %d options, want 0", got)
	}
}

// A non-empty replacement is authoritative too: switching models can add,
// change, or remove the other controls, so the new catalog replaces the old one
// wholesale rather than merging into it.
func TestReplaceConfigOptionsReplacesWholesale(t *testing.T) {
	c := &conversation{capabilities: make(ports.ChatCapabilities)}
	c.replaceConfigOptions([]acpsdk.SessionConfigOption{
		selectOption("model", "Model", "sonnet"),
		selectOption("effort", "Effort", "high"),
	})

	c.replaceConfigOptions([]acpsdk.SessionConfigOption{selectOption("model", "Model", "opus")})

	if got := len(c.configOptions); got != 1 {
		t.Fatalf("got %d options, want 1 — a non-empty update is a full replacement", got)
	}
	if got := c.configOptions[0].Current.Select; got != "opus" {
		t.Fatalf("current value not updated: got %q, want %q", got, "opus")
	}
	if !c.capabilities[ports.ChatCapabilityConfigOptions] {
		t.Fatal("config-options capability should be set by a non-empty catalog")
	}
}

// The bug this guards: an agent accepts session/set_config_option but answers
// without the rebuilt catalog. Wiping made the picker vanish; returning the
// pre-change catalog would show the old value for a change the agent already
// applied. Neither is acceptable — record the accepted value and keep the rest.
func TestApplyAcceptedConfigOptionRecordsSelectWithoutLosingCatalog(t *testing.T) {
	c := &conversation{capabilities: make(ports.ChatCapabilities)}
	c.replaceConfigOptions([]acpsdk.SessionConfigOption{
		selectOption("model", "Model", "sonnet", "sonnet", "opus"),
		selectOption("effort", "Effort", "high", "high", "low"),
	})

	c.applyAcceptedConfigOption("model", ports.ChatConfigOptionValue{Select: "opus"})

	if got := len(c.configOptions); got != 2 {
		t.Fatalf("catalog lost entries: got %d options, want 2", got)
	}
	if got := c.configOptions[0].Current.Select; got != "opus" {
		t.Fatalf("accepted value not recorded: got %q, want %q", got, "opus")
	}
	if got := c.configOptions[1].Current.Select; got != "high" {
		t.Fatalf("unrelated option was disturbed: got %q, want %q", got, "high")
	}
	if got := len(c.configOptions[0].Choices); got != 2 {
		t.Fatalf("choices dropped: got %d, want 2", got)
	}
}

func TestApplyAcceptedConfigOptionRecordsBoolean(t *testing.T) {
	c := &conversation{capabilities: make(ports.ChatCapabilities)}
	c.replaceConfigOptions([]acpsdk.SessionConfigOption{boolOption("fast", "Fast mode", false)})

	c.applyAcceptedConfigOption("fast", ports.ChatConfigOptionValue{Boolean: boolPtr(true)})

	current := c.configOptions[0].Current.Boolean
	if current == nil || !*current {
		t.Fatalf("accepted boolean not recorded: got %v, want true", current)
	}
}

// An id with no matching entry must leave the catalog untouched rather than
// inventing a row for an option the session never advertised.
func TestApplyAcceptedConfigOptionIgnoresUnknownID(t *testing.T) {
	c := &conversation{capabilities: make(ports.ChatCapabilities)}
	c.replaceConfigOptions([]acpsdk.SessionConfigOption{selectOption("model", "Model", "sonnet")})

	c.applyAcceptedConfigOption("nope", ports.ChatConfigOptionValue{Select: "whatever"})

	if got := len(c.configOptions); got != 1 {
		t.Fatalf("got %d options, want 1", got)
	}
	if got := c.configOptions[0].Current.Select; got != "sonnet" {
		t.Fatalf("catalog mutated by unknown id: got %q, want %q", got, "sonnet")
	}
}

func boolPtr(v bool) *bool { return &v }

// GET .../conversation/models reads ChatModelLister, not config-options. ACP
// already holds the provider catalog as the "model" select; projecting it is
// what lets Chat pickers (and the Grok e2e override test) see the same list
// session/new advertised.
func TestListModelsProjectsAdvertisedModelOption(t *testing.T) {
	c := &conversation{capabilities: make(ports.ChatCapabilities)}
	c.replaceConfigOptions([]acpsdk.SessionConfigOption{
		selectOption("model", "Model", "grok-4.6", "grok-4.5", "grok-4.6"),
		selectOption("effort", "Effort", "high", "high", "low"),
	})

	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2: %#v", len(models), models)
	}
	byID := map[string]ports.ChatModel{}
	for _, model := range models {
		byID[model.ID] = model
	}
	if got := byID["grok-4.6"]; !got.Default || got.DisplayName != "grok-4.6" {
		t.Fatalf("current model = %#v, want default with matching display name", got)
	}
	if got := byID["grok-4.5"]; got.Default || got.DisplayName != "grok-4.5" {
		t.Fatalf("other model = %#v, want non-default", got)
	}
}

func TestListModelsReturnsEmptyWhenNoModelOption(t *testing.T) {
	c := &conversation{capabilities: make(ports.ChatCapabilities)}
	c.replaceConfigOptions([]acpsdk.SessionConfigOption{
		selectOption("effort", "Effort", "high", "high", "low"),
	})

	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("got %d models, want 0 when no model option is advertised", len(models))
	}
}

func TestListModelsProjectsLegacySessionModels(t *testing.T) {
	c := &conversation{
		capabilities: make(ports.ChatCapabilities),
		configOptions: normalizeSessionOptions(nil, &legacySessionModelState{
			CurrentModelID: "grok-4.6",
			Available: []legacyModelInfo{
				{ModelID: "grok-4.5", Name: "Grok 4.5"},
				{ModelID: "grok-4.6", Name: "Grok 4.6"},
			},
		}, nil),
	}

	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models from legacy session/new models, want 2: %#v", len(models), models)
	}
	for _, model := range models {
		if model.ID == "grok-4.6" && (!model.Default || model.DisplayName != "Grok 4.6") {
			t.Fatalf("legacy current model = %#v", model)
		}
		if model.ID == "grok-4.5" && (model.Default || model.DisplayName != "Grok 4.5") {
			t.Fatalf("legacy other model = %#v", model)
		}
	}
}
