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
4. **Next:** Agent loop: tools mapped to gateway agent methods → one task per selected phone with the same prompt.
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
