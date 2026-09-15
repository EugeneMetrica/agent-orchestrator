//go:build !windows

package e2e

import "testing"

func TestPickNonDefaultChatModelPrefersSibling(t *testing.T) {
	models := []modelChoice{
		{ID: "hy3", DisplayName: "hy3"},
		{ID: "grok-imagine-video", DisplayName: "Imagine Video"},
		{ID: "grok-4.20-0309-non-reasoning", DisplayName: "Grok 4.20"},
		{ID: "grok-4.5", DisplayName: "Grok 4.5"},
		{ID: "grok-4.6", DisplayName: "Grok 4.6", Default: true},
		{ID: "grok-4.3", DisplayName: "Grok 4.3"},
	}
	got := pickNonDefaultChatModel(models, "grok-4.6")
	if got != "grok-4.5" {
		t.Fatalf("picked %q, want grok-4.5 (closest short sibling of default)", got)
	}
}

func TestPickNonDefaultChatModelFallsBackWhenNoSibling(t *testing.T) {
	models := []modelChoice{
		{ID: "hy3", DisplayName: "hy3"},
		{ID: "only-default", DisplayName: "Only", Default: true},
	}
	got := pickNonDefaultChatModel(models, "only-default")
	if got != "hy3" {
		t.Fatalf("picked %q, want hy3 fallback", got)
	}
}
