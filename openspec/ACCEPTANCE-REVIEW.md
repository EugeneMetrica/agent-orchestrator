# Acceptance Review: OPSX Grok ACP Chat Driver (desired-state only)

**PR**: https://github.com/EugeneMetrica/agent-orchestrator/pull/1
**Branch**: `cursor/grok-acp-chat-driver-spec-3879`
**Reviewer**: Independent acceptance reviewer (not author)
**Date**: 2026-09-14

---

## Verdict: PASS

---

## Checklist

- [x] A. OPSX hygiene
- [x] B. G1–G4 gates encoded
- [x] C. Canonical AO fit
- [x] D. E2E/parity P4a/P4b
- [x] E. Quality/non-goals

---

## Findings

### A. OPSX Hygiene — PASS

| # | Severity | Finding |
|---|----------|---------|
| A1 | nit | `openspec validate --changes` could not be run (CLI not installed), but manual inspection confirms valid structure |
| A2 | PASS | All 6 changes have correct structure: `proposal.md`, `design.md`, `specs/chat-drivers/spec.md`, `tasks.md`, `reviews/.gitkeep` |
| A3 | PASS | All specs use clear **GIVEN/WHEN/THEN** or **Requirement/Scenario** form |
| A4 | PASS | No implementation code (no `.go`, `.ts`, `.tsx` files in diff) — verified via `git diff main...HEAD --name-only` |

### B. Phase Gates G1–G4 Encoded — PASS

| # | Severity | Finding |
|---|----------|---------|
| B1 | PASS | **P0** explicitly waives G3/G4 with documented waiver: "unit tests only, no live Grok required" |
| B2 | PASS | **P1–P4b** all require G1–G4 without waiver |
| B3 | PASS | G1 (independent deep review) required in all phases, with `reviews/` placeholder present |
| B4 | PASS | G2 specifies full suite: `gofmt`, `go build`, `go vet`, `go test -race ./...`, `golangci-lint v2.12.2` / `npm run lint` — NOT package-only |
| B5 | PASS | G3 requires **on-disk file proof**: `proof.txt`, `mode.txt`, `before.txt` — not "process started" |
| B6 | PASS | G4 requires AO frontend/CLI UI verification with explicit checklists |

### C. Canonical AO Fit — PASS

| # | Severity | Finding |
|---|----------|---------|
| C1 | PASS | Thin `chatdriver/grokacp` via `nativeacp.New(plugin, config, log)` — mirrors `opencodeacp`, `ompacp`, `kimchiacp` pattern |
| C2 | PASS | Registration via `registry.Build()` with existing `grok.Plugin` — does not duplicate shared ACP tests |
| C3 | PASS | Auth: reuses existing `grok.Plugin.AuthStatus()` / `ResolveBinary()` — **no credential injection**, no `GROK_HOME` rewrite |
| C4 | PASS | Spawn shape: `grok agent [--always-approve] [--model M] [--no-auto-update] stdio` — correct ACP spawn command |
| C5 | PASS | SessionMeta gap acknowledged in P3 design.md with decision matrix: canonical `nativeacp` extension OR piacp-style path |

### D. E2E / Parity (P4a + P4b) — PASS

| # | Severity | Finding |
|---|----------|---------|
| D1 | PASS | **API frontend E2E (P4a)**: tests for spawn, model override, reasoning effort, approval mode, attachments → worktree |
| D2 | PASS | **Playwright UI E2E (P4b)**: tests harness selection, model dropdown, file upload, timeline activities |
| D3 | PASS | **Claude Code + ai.metrica.pro router as REFERENCE**: explicitly encoded with `AO_LIVE_CLAUDE_ACP=1` + `CLAUDE_ROUTER_URL` |
| D4 | PASS | **G-MODEL** guarantee: "Model switch from UI applies to engine" — WHEN/THEN encoded in P4b spec |
| D5 | PASS | **G-EFFORT** guarantee: "Reasoning effort propagates from UI" — WHEN/THEN encoded in P4b spec |
| D6 | PASS | **G-FILES** guarantee: "UI files delivered to worktree" — file proof in both P4a and P4b |
| D7 | PASS | **G-PARITY** guarantee: "Same UI operations produce equivalent outcomes" — Claude Code reference tests parallel Grok tests |
| D8 | PASS | Anti-pattern addressed: tests require on-disk file verification, not fake-bridge |

### E. Ruthless Quality — PASS

| # | Severity | Finding |
|---|----------|---------|
| E1 | PASS | Non-goals explicitly listed in each proposal: no Telegram, no TUI Chat, no vendoring grok-build |
| E2 | PASS | Tasks phased with clear red→green progression |
| E3 | PASS | Honest capabilities: `--always-approve` flag documented for bypass-permissions, not claiming approval enforcement beyond what Grok ACP provides |
| E4 | nit | P4a/P4b reference tests require `AO_LIVE_CLAUDE_ACP=1` which may not always be available; acceptable for desired-state |

---

## Required Fixes Before Merge

**None.** All acceptance criteria pass.

---

## Recommendation: MERGE

This PR delivers a comprehensive, well-structured OpenSpec (OPSX) desired-state specification for the Grok ACP Chat driver. The spec:

1. **Correctly phases** the implementation from unit tests (P0) through full E2E parity (P4b)
2. **Encodes all required gates** (G1–G4) with explicit waiver only for P0
3. **Follows canonical AO patterns** — thin nativeacp binding, reuses existing grok.Plugin, no credential injection
4. **Specifies rigorous E2E coverage** with on-disk file proof (not fake-bridge)
5. **Includes Claude Code reference parity** via ai.metrica.pro router

The OPSX is ready to guide implementation. No blockers identified.

---

## Notes for Implementation

1. `openspec validate` CLI was not available; recommend installing for future CI gating
2. Existing `grok.Plugin` (TUI mode) and `domain.HarnessGrok` already exist — implementation adds only the chat driver binding
3. Pattern for live tests (`cursoracp/live_test.go`) is well-established and should be followed closely
