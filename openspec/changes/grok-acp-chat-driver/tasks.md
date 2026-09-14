# Tasks: Grok ACP Chat Driver

## Phase 0: OpenSpec Accepted ✓

This phase — current PR contains only spec work.

- [x] Initialize OpenSpec in repository (`openspec init`)
- [x] Create change scaffolding at `openspec/changes/grok-acp-chat-driver/`
- [x] Write `proposal.md` with problem statement and solution
- [x] Write `specs/requirements.md` with WHEN/THEN acceptance criteria
- [x] Write `specs/scenarios.md` with integration and error scenarios
- [x] Write `design.md` with architecture and code patterns
- [x] Write `tasks.md` with phased implementation plan
- [x] Run `openspec validate` and fix until green
- [x] Open PR with spec-only content

**Exit criteria**: Human reviews and accepts the spec; implementation begins
after acceptance.

---

## Phase 1: Red Tests

Write failing tests that define the expected behavior. Tests fail because
implementation does not exist yet.

### Task 1.1: Create grokacp Package Scaffolding

**File**: `backend/internal/adapters/chatdriver/grokacp/driver.go`

```go
package grokacp

// Stub: implementation in Phase 2
```

**Verification**: Package compiles with `go build ./...`

### Task 1.2: Write driver_test.go Unit Tests

**File**: `backend/internal/adapters/chatdriver/grokacp/driver_test.go`

Tests to write:

1. `TestHarness` — `driver.Harness() == domain.HarnessGrok`
2. `TestConfigure_DefaultPermissions` — returns `agent --no-auto-update stdio`
3. `TestConfigure_BypassPermissions` — includes `--always-approve`
4. `TestConfigure_ModelOverride` — includes `--model xai/grok-3`
5. `TestSessionMode_Mapping` — each permission mode maps correctly
6. `TestValidateTurnSettings_ValidModel` — `xai/grok-3` passes
7. `TestValidateTurnSettings_InvalidModel` — `grok-3` fails with correct error
8. `TestValidateTurnSettings_EmptyModel` — passes

**Verification**: Tests fail with "not implemented" or similar; `go test -v ./...`

### Task 1.3: Update Registry Test

**File**: `backend/internal/adapters/chatdriver/registry/registry_test.go`

Add `domain.HarnessGrok` to the expected Chat drivers list in `TestShippedChatDrivers`.

**Verification**: Test fails because Grok is not yet registered.

### Task 1.4: Run Full Test Suite

```bash
cd backend
go test -v ./internal/adapters/chatdriver/grokacp/...
go test -v ./internal/adapters/chatdriver/registry/...
```

**Verification**: Tests fail as expected (red).

---

## Phase 2: Green Implementation

Implement the driver to make all tests pass.

### Task 2.1: Implement driver.go

**File**: `backend/internal/adapters/chatdriver/grokacp/driver.go`

Implement per design.md:

1. `New()` function returning `ports.ChatDriver`
2. `configure()` function for spawn args
3. `sessionMode()` function for permission mapping
4. `sessionOptions()` function for model override
5. `validateTurnSettings()` function for model validation

**Verification**: `go build ./internal/adapters/chatdriver/grokacp/...`

### Task 2.2: Register Driver in Registry

**File**: `backend/internal/adapters/chatdriver/registry/registry.go`

1. Add import for `grokacp` package
2. Add import for `grok` agent package
3. Add `grokacp.New(grok.New(), log)` to `Build()` function

**Verification**: `go build ./internal/adapters/chatdriver/registry/...`

### Task 2.3: Run Unit Tests

```bash
cd backend
go test -v ./internal/adapters/chatdriver/grokacp/...
go test -v ./internal/adapters/chatdriver/registry/...
```

**Verification**: All tests pass (green).

### Task 2.4: Run Full CI Suite Locally

```bash
# From repo root
npm run lint

# Backend specific
cd backend
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

**Verification**: All checks pass.

---

## Phase 3: Optional Live Smoke Test

Only if Grok CLI is available on the development machine.

### Task 3.1: Create live_test.go

**File**: `backend/internal/adapters/chatdriver/grokacp/live_test.go`

Gated behind `AO_LIVE_GROK_ACP=1`:

1. `TestLive_Probe` — verify binary/auth check succeeds
2. `TestLive_SpawnAndMessage` — full spawn, message, response cycle
3. `TestLive_Interrupt` — verify interrupt handling

**Verification**: `AO_LIVE_GROK_ACP=1 go test -v ./internal/adapters/chatdriver/grokacp/...`

### Task 3.2: Document Live Test Requirements

Add comment at top of `live_test.go`:

```go
// Live tests require:
// - Grok CLI installed and on PATH
// - Valid authentication (XAI_API_KEY or ~/.grok/auth.json)
// - AO_LIVE_GROK_ACP=1 environment variable
```

---

## Phase 4: Documentation (If Needed)

Only if other harnesses have harness-specific docs.

### Task 4.1: Check for Existing Harness Docs

Search for:
- `docs/harnesses/`
- `docs/agents/`
- References to harness-specific setup in README

### Task 4.2: Create Grok Chat Docs (If Pattern Exists)

If other harnesses have dedicated docs, create equivalent for Grok:
- Prerequisites (Grok CLI installation)
- Authentication methods
- Chat-specific configuration

---

## Quality Checklist

Before marking implementation complete:

- [ ] `gofmt -l .` returns no files
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` passes (v2.12.2)
- [ ] `npm run lint` passes
- [ ] No drive-by cleanups or unrelated changes
- [ ] PR description follows `.agents/skills/pr-description/SKILL.md`
- [ ] Conventional commit messages used

---

## Notes

### Why This Order?

1. **Phase 0 (Specs)** ensures human alignment before code investment
2. **Phase 1 (Red)** defines expected behavior before implementation
3. **Phase 2 (Green)** makes tests pass with minimal code
4. **Phase 3 (Live)** optional validation against real provider
5. **Phase 4 (Docs)** only if pattern exists for other harnesses

### Reference Files

Existing native ACP drivers to reference:
- `backend/internal/adapters/chatdriver/opencodeacp/driver.go`
- `backend/internal/adapters/chatdriver/droidacp/driver.go`
- `backend/internal/adapters/chatdriver/ompacp/driver.go`

Existing Grok agent adapter:
- `backend/internal/adapters/agent/grok/grok.go`
- `backend/internal/adapters/agent/grok/auth.go`

### Spawn Command Reference

TUI mode (existing):
```
grok --no-auto-update [--permission-mode <mode>] [--model <model>] [--rules <text>]
```

Chat mode (new ACP):
```
grok agent --no-auto-update [--always-approve] [--model <model>] stdio
```

Key differences:
- `agent` subcommand enables ACP mode
- `stdio` subcommand selects JSON-RPC transport
- `--always-approve` replaces `--permission-mode bypassPermissions`
- `--rules` is not used (system prompt via ACP `session/new` metadata)
