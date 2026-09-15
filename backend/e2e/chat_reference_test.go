//go:build !windows

package e2e

import (
	"testing"
)

// Claude Code reference runs for the Grok API scenarios.
//
// A live Grok result on its own cannot say whether a behaviour is Grok's or AO's:
// if an override never reaches the provider, the failure looks the same either
// way. These run the identical scenario bodies against a harness that was already
// shipping, so a Grok failure can be read as a Grok failure and an AO regression
// shows up on both.
//
// Separately gated because they spend a second account's quota:
//
//	AO_LIVE_CLAUDE_ACP=1 go test ./e2e/ -v -run ChatClaudeCode -timeout 30m

const claudeGateEnv = "AO_LIVE_CLAUDE_ACP"

// requireClaudeReferenceE2E skips unless the Claude gate is set and the binary is
// installed. The harness id is `claude-code`; the executable is `claude`.
func requireClaudeReferenceE2E(t *testing.T) {
	t.Helper()
	requireLiveHarness(t, claudeGateEnv, "claude")
}

func TestChatClaudeCodeSpawn(t *testing.T) {
	requireClaudeReferenceE2E(t)
	runChatSpawnScenario(t, "claude-code")
}

func TestChatClaudeCodeModelOverride(t *testing.T) {
	requireClaudeReferenceE2E(t)
	runChatModelOverrideScenario(t, "claude-code")
}

func TestChatClaudeCodeReasoningEffort(t *testing.T) {
	requireClaudeReferenceE2E(t)
	runChatReasoningEffortScenario(t, "claude-code")
}

func TestChatClaudeCodeApprovalModeOverride(t *testing.T) {
	requireClaudeReferenceE2E(t)
	runChatApprovalModeOverrideScenario(t, "claude-code")
}

func TestChatClaudeCodeAttachments(t *testing.T) {
	requireClaudeReferenceE2E(t)
	runChatAttachmentsScenario(t, "claude-code")
}

func TestChatClaudeCodeServerStateConsistency(t *testing.T) {
	requireClaudeReferenceE2E(t)
	runChatServerStateScenario(t, "claude-code")
}
