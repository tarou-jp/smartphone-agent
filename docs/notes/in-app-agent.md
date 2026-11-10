# In-app agent and multi-device same task (fork experiment)

Branch: `feat/in-app-agent`. Lives on the fork only (`tarou-jp/smartphone-agent`). Do not push this to `DIYgod/pinky` until the product choice is deliberate: upstream [DESIGN.md](../DESIGN.md) still says Pinky ships no model and lists a built-in agent as a non-goal. The closest upstream idea is a later “quick task” box that calls an agent the person already has, headlessly.

## What we want

Pinky’s window becomes the place to:

1. Save an LLM API key (OpenAI, Anthropic, …) in the OS keychain, same pattern as phone PINs.
2. Type one instruction in Pinky and have Pinky call that model.
3. Run **the same instruction on several connected phones** (parallel fan-out), with the existing per-phone mirror, approvals, stop and takeover.

For app developers: walk the same test script on several real devices without opening Cursor/Claude for each phone.

## How it differs from today

| | Today | This experiment |
| --- | --- | --- |
| Who holds the model | External MCP client (Cursor, Claude, …) | Pinky (key in Settings, call from the UI) |
| Who plans the steps | That external agent | An in-process agent loop in Pinky |
| Several phones | The external agent picks devices itself | Pinky fans one prompt out to N device leases |

Safety stays in the gateway: the in-app loop must use the **agent-facing** path (the same surface MCP uses), never the desktop `gateway.API` that can approve or take over. The person still sees approvals and can Stop / take over per phone.

## MVP (this branch)

1. **Done:** Notes + package skeleton (`internal/inapp`): provider id, keychain accounts, fan-out types, `Complete` HTTP client.
2. **Done:** Settings › Model — choose Anthropic/OpenAI, save/change/remove the API key in the OS keychain.
3. **Done:** Phone page layout is mirror → Tasks/Needs → Chat. Chat follows the sidebar’s phone selection (no separate chips), with a Cursor-like New chat header and flat composer. Chat calls the model; it does not drive phones yet.
4. **Done (web client):** Agent loop: tools mapped to gateway agent methods → one run per phone picked with @, side by side, in the window-wide Agent panel ([web-ui-migration.md](web-ui-migration.md)).
5. **Later:** shared step templates, “stop all fan-out”, per-device outcome summary for QA.

## Keychain accounts

Service stays `app.pinky.desktop`. Accounts:

- `llm:anthropic` — Anthropic API key
- `llm:openai` — OpenAI API key

Reveal reason examples: “use the Anthropic API key for an in-app task”.

## Out of scope for MVP

- Shipping a model binary or local weights
- Cloud device farm / CI sharding (still a DESIGN non-goal)
- Bypassing approvals or read-only rules
- Changing upstream DESIGN without an explicit product decision

## How to try (once UI lands)

1. Settings › (Model) › choose provider → paste key → Save (keychain).
2. Select two or more connected phones → enter a prompt → Run.
3. Watch each phone’s task card; approve when asked; Stop one phone without stopping the others.

## The harness (`internal/inapp`)

The loop follows the design of OpenAI's open-source Codex harness (Apache-2.0, `openai/codex`), rebuilt in Go for phones and for every provider Pinky offers (Anthropic, OpenAI, Ollama). No Codex code is used.

- **Thread of items** (`thread.go`): the person's messages, the model's turns (text and tool calls) and the tools' answers. Providers turn items into their own messages (`anthropic.go`, `openai.go`).
- **Context** (`view`, `compact`): the model sees the last three tool answers whole and older ones cut to their first lines (a screen is long and goes stale); any answer is cut to 24k characters; a thread past about 60k tokens is summed up by the model and goes on from the summary, keeping its latest items.
- **Streaming and the prompt cache**: replies stream (`text_delta`). On Anthropic the system prompt, the tools and the thread's end carry `cache_control`, so a turn pays full price only for what is new; OpenAI caches the unchanged start by itself.
- **Retries** (`transport.go`, `streamTurn`): 408, 409, 429, 5xx, 529 and dropped connections are tried again up to five times, waiting 1, 2, 4, 8 s (with jitter, at most 30 s) or what Retry-After says. A stream that broke before the model said anything is tried again; after, it is an error, since its words are on screen. `PINKY_ANTHROPIC_BASE_URL` and `PINKY_OPENAI_BASE_URL` point the clients elsewhere (a proxy, the tests).
- **Tool calls side by side** (`runCalls`): one reply's calls for different phones run at the same time; calls for one phone run in order.
- **Approvals** (`PhoneTools.callPending`): a call that opened an approval is repeated with its id, each time blocking in the gateway, until the person decides or the approval expires; the gateway's progress ("Waiting for the person: …") goes to the UI as `tool_progress`. Nothing gives up after a fixed time any more.
- **Plan** (`update_plan`): the agent's own tool, as Codex's; the UI shows it as a checklist.
- **One agent on several phones** (`AgentConfig.Devices`, the composer's 並行/連携 toggle, `together` in `/api/chat`): one loop connects every phone and names the device on each call; without it, each phone gets its own loop.
- **Ask** (`Talk`): the chat without tools streams too, with the same retries.

Events (`AgentEvent.Kind`): `tool_start`, `tool_progress`, `tool_end`, `text_delta`, `text`, `plan`, `retry`, `compact`, `usage`; `/api/chat` sends them as `step` events.

Tests: `harness_test.go` (streaming, retries, context, the loop with a scripted model) and `gateway_test.go` (a scripted model reorders lunch in Dinely on two demo phones through a real gateway, payments approved while the loop waits); `internal/deskapi/chat_test.go` drives `/api/chat` against an Ollama-like server.
