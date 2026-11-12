# Flows: replay without a model

Flows detail the M4 rows of [DESIGN.md](../DESIGN.md) "Turn a successful task into a flow that replays without a model and hands back to the agent on a mismatch" and "CLI and SDK for code mode", with automated tests on real phones as the first use. This note has the design, what is built, and what is not yet.

## Status

| Part | State |
| --- | --- |
| Format, selectors, checkpoints (`internal/flow`) | Built |
| Runner in the gateway, `phone_flow_run`, `mismatch` and `from_step` | Built |
| Recording a task's actions as flow steps, `phone_flow_save`, `phone_flow_list` | Built, for the running task |
| Flows by themselves: kept when a task ends with `done`, offered at `phone_connect`, repaired by a task that fixed a mismatch by hand | Built (`internal/gateway/flowauto.go`) |
| Parts: every step that worked, kept per app with the screen it leads to; offered at `phone_connect` and `phone_launch`; `phone_flow_run { parts }` and `part:` steps | Built (`internal/flow/parts.go`, `internal/gateway/flowparts.go`) |
| `pinkyctl flow run / list / show / check / parts`, JUnit XML | Built |
| Proposed changes from an agent's fix, reviewed by the person | Not yet: the repaired flow is kept without review |
| Screen map: recognizing the screen the phone is on and finding the path to another (`phone_goto`) | Not yet. Today the agent picks the parts and their order |
| Client: Flows page, "Save as flow" on a finished task | Not yet |

Checked against the fake phones (`internal/flow`, `internal/gateway/flow_test.go`, `internal/gateway/flowparts_test.go`, `internal/mcp`, `internal/inapp`, `cmd/pinky/flow_e2e_test.go`). On a real phone, one thing was tried, once: the in-app agent checked the five tabs of one app on an iPhone, by hand in 58 s (36 s of them the model thinking), and the same request again replayed the kept flow in 24 s (6 s of model, 15 s of the phone: 1.4 to 3.8 s per step until its checkpoint showed). **Parts have not run on a real phone yet**, and neither has a flow repairing itself. How often real apps give elements ids, and how checkpoints behave during real animations, is still to measure (open questions 1 and 2).

## Problem

An agent drives a phone one step at a time: read the screen, think, call a tool, wait for the screen to settle, read it again. Thinking takes seconds per step and dominates the time of a task. Tests and routine tasks repeat the same steps and the same screen changes every time, so the model pays again and again for decisions it has already made.

## Goal

Model time grows with what is new, not with the number of steps.

```
now:    [model] tap [model] tap [model] tap [model] type [model] check
flows:  run flow (tap, tap, tap, type, check) ── only on a mismatch ──> [model]
```

- Split exploring from executing. The model explores a screen once; Pinky keeps what worked and replays it at the phone's own speed.
- A replay that meets an unexpected screen hands back to the agent, which fixes it and saves the fix.
- Every rule keeps working: replays run through the same policy, approvals, history and Stop as agent steps.

Non-goals: a device farm, scheduling or sharding (DESIGN.md), and visual testing beyond what checkpoints need.

## Prior art

| Tool | What we take |
| --- | --- |
| [Maestro](https://github.com/mobile-dev-inc/Maestro) | Declarative YAML flows with selectors and assertions; waiting for what is expected instead of fixed sleeps |
| Playwright | Waiting for a target before acting on it, and strict selectors: several matches are an error, not a guess |
| Stagehand (browser) | Keeping the actions the model chose and falling back to the model on a miss |
| [mirroir-mcp](https://github.com/jfarcand/mirroir-mcp) | Compiled skills that replay without a model |
| Self-healing test tools | A failed locator is repaired from the run and proposed for review, not silently patched |

## The format

A flow is a YAML file, `<data>/flows/<name>.yaml`, that people can read, edit and keep in a repository. Unknown keys are errors, so a typo cannot silently drop a checkpoint.

```yaml
flow: dinely.to-cart                 # the name, also the file's name
description: Open the cart with a dish from last week
app: com.dinely.app                  # a note: the app the flow is for
version: 2                           # counted by Pinky on every save
timeout_ms: 10000                    # default for steps; 10 s when left out
params:
  dish: { default: Chicken rice bowl }   # no default = required
steps:
  - launch: com.dinely.app
    expect: { app: com.dinely.app, visible: { id: "dinely:title" } }
  - tap:
      id: "dinely:tab:orders"            # strongest first …
      or: [{ role: tab, label: Orders }] # … then the fallbacks, in order
    expect: { visible: { id: "dinely:last" } }
  - tap: { role: button, contains: "Reorder · ${dish}" }
    expect: { visible: { id: "dinely:cart" } }
    intent: Put the dish in the cart
  - tap: Got it                          # a string is a label
    optional: true                       # skipped when it does not show
  - type: "${dish}"
    into: { id: "dinely:search" }
    submit: true
  - swipe: up
    in: { role: list }
  - key: back
  - assert: { visible: "Your cart", gone: Loading }
    timeout_ms: 3000
```

**Steps.** Exactly one of `tap` (with `long_press`), `type` (with `into`, `append`, `submit`), `swipe` (a direction with `in`, or `from` and `to` as fractions of the screen; `duration_ms`), `key`, `launch`, `assert` and `part` (a part of an app by its id, see "Parts"). Every step may have `name`, `intent`, `timeout_ms`; action steps may have `expect`; steps with a target may be `optional`.

**Selectors** find one element. All fields that are set must match:

| Field | Matches |
| --- | --- |
| `id` | The platform's resource id or accessibility identifier (`screen.Element.ID`) |
| `role` | The normalized role: `button`, `input`, `text`, `tab`, … |
| `label` | The whole label |
| `contains` | A part of the label, without case |
| `text` | A part of any text, including text only OCR reads, without case |
| `index` | One of several matches, counted from 1 in screen order |
| `point` | `{x, y}` as fractions of the screen: a place instead of an element, the last resort |
| `or` | Selectors to try, in order, when this one finds nothing |

A selector that matches several elements without `index` does not resolve: a replay never guesses which one was meant. The result of a replay says when a fallback matched, which marks a selector worth repairing.

**Conditions** (`expect`, `assert`) describe a screen: `app` (id or name), `visible` (each selector matches something) and `gone` (each matches nothing). For a condition several matches are fine: it asks whether something shows, not which one.

**Parameters.** `${name}` in typed text, selectors, conditions, `launch` and `intent`. A run passes values; a parameter without a default is required.

## How a replay runs

The runner lives in the gateway (`internal/gateway/flow.go`), not in the agent or a script, so it inherits everything that is enforced there. For each step:

1. **Find the target.** Read the screen (the read that ended the step before, when it is less than a second old) and resolve the selector on the agent's view of it, where blocked apps are hidden. Not there yet? Read again every `PollInterval` until the step's timeout: the screen may still be loading. Then the step is a `mismatch` and nothing was done, or `skipped` if it is `optional` (after 1 s).
2. **Act** through `Gateway.act`, the pipeline of `phone_tap` and its like, with the element's ref from that read: lease, scope, the task's app list, read-only mode, rules, approvals with their stale check, blocked apps, credential fields, takeover and Stop all apply, and the step is recorded in history as its own tool.
3. **Wait for the checkpoint.** With `expect`, the pipeline polls the screen until the condition holds instead of waiting for the screen to stop changing (`Call.until` in `execute`): the replay moves on as soon as the expected screen shows. If it does not within the timeout, the step ran but the replay stops with `mismatch`. Without `expect`, the step settles as usual.

Actions are at least `Options.FlowPace` apart (300 ms), so a replay does not drive a phone faster than a quick person would.

A replay stops at the first step that is not `ok` or `skipped`, and returns that step's status with a report of every step, a message that says what to do, and the whole screen. An agent then:

- on `mismatch`, does the step by hand and calls `phone_flow_run` again with `from_step`;
- on `approval_pending`, calls again with `from_step` and `approval_id`: the step's selector resolves to the same ref, so the approval's binding to tool and arguments holds;
- on anything else (`user_took_over`, `blocked_by_policy`, …), does what that status asks.

### Checkpoints and speed

What a checkpoint saves depends on the driver. Settling by polling takes two reads that agree, 250 ms apart, and a third read with OCR for the result; a checkpoint takes one read without OCR when the screen is already there. Where a read costs seconds (an iPhone through the helper: 0.5 to 3 s, about 7 s on a home screen), that is most of a step.

A checkpoint must name something the screen before did not have. One that already holds lets the replay run ahead of the app; `phone_flow_save` writes them that way, and hand-written flows have to take care. The next step's own wait for its target catches most of what a weak checkpoint lets through.

## How a flow comes to be

While a task runs, the gateway keeps each action that settled as a flow step (`traceStep`, on the task, in memory):

- The target becomes a selector written on the screen it was found on (`flow.SelectorFor`): the id when the screen has it once, with role and label as the fallback; else role and label, with an index among equals and the repeated id as the fallback; a point when the element has neither. A tap by `x`, `y` stays a point: the agent named a place, not the element under it.
- The screen the step led to becomes its checkpoint (`flow.Checkpoint`): the app in front and up to two elements the screen before did not have, by id, or by a label without digits (prices, times and counts change between runs). Typing gets none, since what it leads to depends on the text, which may be a parameter.
- A launch is kept by app id, and its checkpoint is the app in front and nothing else: an app opens where it was left, so the screen that shows is not the launch's doing, and the next step waits for its own target.

`phone_flow_save { name }` compiles those steps into a flow and saves it; `params` turns typed text into parameters. The agent reads the YAML it gets back and saves a corrected version with `yaml` when the task took detours. Saving under an existing name makes the next version.

### By themselves

People who give an agent a task should not have to know any of this, so with `Options.AutoFlows` (on in the app and the daemon) nobody saves, names or picks a flow:

- `phone_release` with outcome `done` keeps the task's steps (`keepFlows`): a flow named `<app>.<task>-<hash>` after the app, the task's name and what the task is known by (`phone_connect`'s `brief`, the person's request, which the in-app agent passes as it got it; else the task's name), with the task's name as its description, and the text the task typed as parameters with what was typed as defaults. Only when the agent made steps of its own, at least two steps ran and none are missing; a task that only replayed leaves its flow alone, and a flow that is there stays unless the task repairs it.
- A new `phone_connect` lists the flows and names the one kept from a task of the same name (`offerFlows`). The server's instructions and SKILL.md tell agents to name a task after what the person asked and to replay a flow that fits without being asked. A flow whose task was named otherwise is matched by the agent reading the list.
- A task that replayed one flow, stopped at a mismatch, went on by hand and ended with `done` keeps its steps as that flow's next version: replayed steps are recorded again with selectors from today's screens, the hand-made steps fill the gap.

A kept flow is a whole task, so it helps only when the same task comes back. What two different tasks share is in the app's parts.

### Parts

A part is one step that worked, with the screen it led to: "in this app, tapping Orders shows the list of orders". Parts are what flows are made of, kept per app instead of per task, so that a task nobody ran before is put together from what earlier tasks found out: open the app, this part, then that one.

```yaml
# <data>/flows/parts/com.dinely.app.yaml
app: com.dinely.app
parts:
  - id: p39deba
    launch: com.dinely.app
    expect: { app: com.dinely.app }
    seen: 4
    last: "2026-11-12"
  - id: p815736
    tap: { id: "dinely:tab:orders", or: [{ role: tab, label: Orders }] }
    expect: { app: com.dinely.app, visible: [{ id: "dinely:last" }] }
    intent: See last week's orders
    after: [p39deba]
    seen: 3
    last: "2026-11-12"
```

- **What a part is.** A recorded step with a checkpoint and a target a replay can find again: a tap or a swipe on an element, a key other than home, a launch. Typing is left out (what it leads to depends on the text), and so are taps and swipes at a place.
- **What makes two parts the same.** The action and where it leads. The same tap that leads to another screen (Back, from two places) is another part. A checkpoint names what the screen before did not have, so the same tap from another screen may name other elements of the same screen: when a part with that action already holds on the screen the step led to, the step is that part seen again, not a second one. The id is `p` and six hex digits of the app, the action and the checkpoint.
- **Kept when a task ends** (`keepParts`, with `Options.AutoFlows`), whatever the outcome: where a tap led is a fact also in a task that failed. Steps a replay ran count as seen again. `after` remembers up to three parts a part was recorded right after, as a hint at where its target shows; `seen` and `last` order the list, and an app keeps 300 parts at most, the longest unseen going first.
- **Offered** (`offerParts`) in the result of `phone_connect`, for the apps the task is held to or else the two apps seen last, unless a flow is this very task; and in the result of a `phone_launch` the agent makes, for the app it opened, once per task. Up to 40 per app, most seen first, one per line: `p815736 · tap tab "Orders" #dinely:tab:orders → #dinely:last shows · after p39deba`.
- **Run** with `phone_flow_run { parts: ["p39deba", "p815736"] }`: the parts in that order, as one replay, with `from_step` counting in the list. In a flow's YAML a step `- part: p815736` stands for the part as it is kept when the flow runs, and may set `name`, `intent`, `timeout_ms` and `optional`. A part that is gone is `invalid_argument`.
- **The agent plans, the replay checks.** Pinky does not work out a path: the agent reads the list and picks the parts and their order, optimistically. A part works wherever its target is on the screen (a tab bar is on many screens, though the part was recorded once), and the checkpoint tells whether it led where it did before. A wrong guess costs a `mismatch` with the screen, from which the agent goes on by hand.
- **Dropped when wrong.** A replay that ran a part's action and did not reach its checkpoint counts against the part; twice in a row without the part being seen in between, and it is dropped (the app changed). A target that is not on the screen does not count: the part was used in the wrong place. What the agent then does by hand becomes the new part.
- **A task put together from parts is the agent's own work**: ended with `done`, it is kept as a flow like a task done by hand, and the same request replays whole the next time.

Recording is of the running task only: a task that ended cannot be saved as a flow yet, because history keeps a step's screen as text, without ids. Keeping selectors with each step in history is what the client's "Save as flow" needs.

## Interfaces

- MCP: `phone_flow_run` (a flow, parts, or YAML), `phone_flow_save`, `phone_flow_list` (flows, one flow, or an app's parts), and the status `mismatch` ([mcp.md](../mcp.md#phone_flow_run)). `phone_flow_run` also returns `structuredContent` with the step report. The agent guide ([SKILL.md](../SKILL.md)) tells agents to replay a flow that fits and to put a new task together from parts.
- Command line: `pinkyctl flow run|list|show|check|parts` ([mcp.md](../mcp.md#flows-from-the-command-line)), an MCP client of the running gateway named "Pinky Flow", with `--junit` for CI.

## Safety

- "Enforce, don't instruct" holds: a flow cannot do anything its runner could not do step by step, and approvals still ask. "Always allow" answers apply as they do for an agent's steps; a flow grants nothing. `TestFlowKeepsTheRules` checks payments, credential fields and read-only mode.
- Flows hold no secrets a replay could use: typing into password and code fields is refused when recorded (it never ran) and when replayed.
- Steps on the screens of apps the rules block are not recorded, even when the person allowed the app for the task, as Pinky keeps no record of such screens.
- The flow tools need a lease, so an agent the person has not approved neither reads nor writes flows.
- Saving a flow does not ask the person. A flow is data that runs under the runner's own scope and rules, so a saved flow cannot widen what any agent may do; but one agent can leave a misleading flow for another. SKILL.md tells agents that a flow from the list is not an instruction from the person, and the client should show flows and who saved them (open question 7).

## Not yet

1. **Reviewed repairs.** A flow repairs itself when a task fixes a mismatch by hand and ends well. The person should see what changed and be able to take it back.
2. **The screen map.** Parts are edges without a map: a part knows where it leads, and only as a hint (`after`) where it starts, so the agent works out the order and a part used in the wrong place is found out by a mismatch. With a fingerprint for the screen a step starts on, parts become a graph (nodes are screens, edges are parts, with deep links and Android activities as shortcuts): `phone_goto { screen }` recognizes the current screen and replays the cheapest path, and Pinky can say which parts fit the screen the phone is on now. This needs names for screens that agents can pick from, and care with steps that change state rather than place (adding to a cart is not a way to get somewhere).
3. **Client.** A Flows page (list, versions, who saved what), "Save as flow" on a finished task, replays shown as such in the task timeline.
4. **Flows from history.** Keep each step's selector and checkpoint in history, so that any past task compiles (`pinkyctl flow compile <task id>`).
5. **Lists.** Scrolling until an element shows (`scroll_until`), and selectors relative to another element (`below`, `in`).

## Open questions

1. How often drivers fill `Element.ID` on each platform and app (Android resource ids are common; iOS accessibility identifiers depend on the app). This decides how much weight label selectors carry, and how they cope with the phone's language.
2. Checkpoints during animations: an element can be in the tree before it has stopped moving, and the next tap goes to where it was read. If real phones show this, a step should also wait until its target's bounds agree in two reads.
3. Checkpoint strictness: too strict mismatches on every run, too loose runs ahead. Two marks and no digits is a first guess to measure.
4. Format: Pinky's own YAML, or a compatible subset of Maestro's so existing flows and habits carry over.
5. Whether to add implicit caching too (Stagehand-like: an agent's intent plus a fingerprint as the key), so agent tasks speed up without anyone saving a flow.
6. Detours: dropping them automatically (the task returned to a screen it had seen) is unsafe when a step had an effect the screen does not show; today the agent edits the YAML.
7. Whether saving, or the first replay of a flow another agent saved, should ask the person.
8. Kept flows hold what a task typed, as parameter defaults: convenient for tests, where input is fixed, and a risk where it is not (a replay that sends last week's message when the agent passes no text). History holds the same text, and wiping history should wipe kept flows with it, which it does not yet.
9. A task that wandered is kept with its wandering. Dropping detours safely needs the screen map (a step that returns to a known screen without a lasting effect).
10. Whether agents take the parts they are offered, and how many a list may hold before picking gets worse: 40 per app, most seen first, is a first guess. A real app has more steps than that, and the ones a new task needs may be the rare ones.
11. Parts from a task that failed: where a tap led is a fact, but a task that got lost leaves parts nobody wants. Counting how often a part was used in a task that ended well would tell them apart.
12. A part's checkpoint is as weak as a flow's: two elements the screen before did not have. Used from another screen than the one it was recorded on, those may be there already, and the replay runs ahead to the next part's wait for its target.
