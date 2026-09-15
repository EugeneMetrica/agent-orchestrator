# Claude Code with GLM models (Z.ai / metrica gateway)

Claude Code can be pointed at an Anthropic-compatible gateway that serves GLM
models instead of Anthropic ones. The canonical operator reference is Z.ai's
[Claude Code + GLM Coding Plan guide](https://docs.z.ai/devpack/tool/claude):
it configures the gateway through the `env` block of `~/.claude/settings.json`,
so each Claude alias routes to a GLM model.

## Operator setup (Claude Code's own settings)

Per the Z.ai guide, `~/.claude/settings.json`:

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "your_api_key",
    "ANTHROPIC_BASE_URL": "https://api.z.ai/api/anthropic",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "glm-5.3",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.3-flash",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-4.6v-flash",
    "API_TIMEOUT_MS": "3000000"
  }
}
```

On this stack the base URL is the metrica gateway (`https://ai.metrica.pro`)
instead of `https://api.z.ai/api/anthropic`; everything else is the same
pattern. The standing metrica alias map is:

| Claude Code alias | Environment variable             | Model            |
| ----------------- | -------------------------------- | ---------------- |
| Opus              | `ANTHROPIC_DEFAULT_OPUS_MODEL`   | `glm-5.3`        |
| Sonnet            | `ANTHROPIC_DEFAULT_SONNET_MODEL` | `glm-5.3-flash`  |
| Haiku             | `ANTHROPIC_DEFAULT_HAIKU_MODEL`  | `glm-4.6v-flash` |

Z.ai's own defaults point every alias at GLM-5.3-Flash, and its manual-config
example uses the long-context ids `glm-5.3[1m]` / `glm-5.3-flash[1m]`. Whatever
the gateway serves, the ids AO offers follow these variables (see below), and
any other id can still be typed into the picker.

Claude Code reads `settings.json` at startup, so a changed mapping needs a fresh
Claude Code process — for AO that means a new session, not a reload of an
existing one.

## How this shows up in the AO picker

Claude Code has no machine-readable model-list command, so AO's pre-session
picker is built from a static snapshot of the models an ACP `session/new`
advertises (`backend/internal/adapters/agent/modelcatalog/catalog.go`). That
snapshot lists aliases (`opus`, `sonnet`, `haiku`, …), which is what Claude Code
sends upstream — the gateway, not AO, decides what an alias resolves to.

AO additionally lists the concrete ids each alias routes to, so a GLM model is
selectable in the picker before spawn and is stored on the session like any
other model id. `GET /api/v1/agents/claude-code/models` therefore returns, for
example:

```json
{
  "agentId": "claude-code",
  "selectionMode": "catalog",
  "models": [
    { "id": "fable", "label": "Fable 5.1" },
    { "id": "glm-4.6v-flash", "label": "glm-4.6v-flash (Haiku alias)" },
    { "id": "glm-5.3", "label": "glm-5.3 (Opus alias)", "isDefault": true },
    { "id": "glm-5.3-flash", "label": "glm-5.3-flash (Sonnet alias)" },
    { "id": "haiku", "label": "Haiku" },
    { "id": "opus", "label": "Opus" },
    { "id": "opus[1m]", "label": "Opus (1M context)" },
    { "id": "sonnet", "label": "Sonnet" }
  ],
  "customModelEntry": "direct",
  "allowCustom": true,
  "source": "catalog"
}
```

Alias ids are resolved from, in precedence order: the session environment, the
daemon process environment, `<project>/.claude/settings.local.json`,
`<project>/.claude/settings.json`, and `~/.claude/settings.json` — reading the
`env` block of each settings file, which is where the Z.ai pattern puts the
variables. When an alias has no override, `glm-5.3` and `glm-5.3-flash` stay
listed as the documented gateway defaults; Haiku is listed only when its
override is set. AO surfaces only the ids the variables name, so an operator who
follows Z.ai's long-context example gets `glm-5.3[1m]` and `glm-5.3-flash[1m]`
rows instead; AO does not guess variants the gateway may not serve.

The selected id is forwarded verbatim as the ACP `model` session option, so a
Chat session started on `glm-5.3` runs on `glm-5.3`. Free-text entry
(`customModelEntry: "direct"`) still accepts any other id the gateway serves.

## Boundaries

- AO never stores or hardcodes a gateway URL or credential. `ANTHROPIC_BASE_URL`
  and `ANTHROPIC_AUTH_TOKEN` stay in Claude Code's own settings/environment;
  AO reads only model ids.
- Discovery stays side-effect free: it reads local configuration and launches
  no Agent SDK, interactive client, or provider turn.
- The alias overrides are part of the catalog fingerprint, so editing the
  mapping in settings or the environment invalidates the cached catalog and the
  next picker read (or manual Refresh) shows the new ids.
