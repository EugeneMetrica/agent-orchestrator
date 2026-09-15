//go:build !windows

package e2e

import "testing"

// Claude Code reference runs of the Grok API scenarios.
//
// Grok is new to Chat; Claude Code has been driven through the same routes for
// long enough that its behaviour is the yardstick. Running the identical
// scenario bodies against it is what separates "Grok is broken" from "this API
// never worked for any ACP harness", which is the question a failing Grok run
// otherwise cannot answer.
//
// Gated separately (AO_LIVE_CLAUDE_ACP) because they spend turns on the user's
// Claude account; a machine with only one of the two agents still runs that one.

const (
	claudeReferenceGateEnv = "AO_LIVE_CLAUDE_ACP"
	claudeReferenceHarness = "claude-code"
	claudeReferenceBinary  = "claude"
)

func requireClaudeReferenceE2E(t *testing.T) {
	t.Helper()
	requireLiveHarnessE2E(t, claudeReferenceGateEnv, claudeReferenceBinary)
}

func TestChatClaudeCodeModelOverride(t *testing.T) {
	requireClaudeReferenceE2E(t)
	runChatModelOverride(t, claudeReferenceHarness, "refmodel")
}

func TestChatClaudeCodeAttachments(t *testing.T) {
	requireClaudeReferenceE2E(t)
	runChatAttachments(t, claudeReferenceHarness, "refattach")
}
