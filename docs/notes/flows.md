# Flows: replay without a model

Flows detail the M4 rows of [DESIGN.md](../DESIGN.md) "Turn a successful task into a flow that replays without a model and hands back to the agent on a mismatch" and "CLI and SDK for code mode", with automated tests on real phones as the first use. This note has the design, what is built, and what is not yet.

## Status

| Part | State |
| --- | --- |
| Format, selectors, checkpoints (`internal/flow`) | Built |
| Runner in the gateway, `phone_flow_run`, `mismatch` and `from_step` | Built |
| Recording a task's actions as flow steps, `phone_flow_save`, `phone_flow_list` | Built, for the running task |
| Flows by themselves: kept when a task ends with `done`, offered at `phone_connect`, repaired by a task that fixed a mismatch by hand | Built (`internal/gateway/flowauto.go`) |
| Parts: every step that worked, kept per app with the screen it leads to; offered at `phone_connect` and `phone_launch`; plans (`phone_flow_run { parts }` or `{ steps }`) and `part:` steps | Built (`internal/flow/parts.go`, `internal/gateway/flowparts.go`) |
| Kept flows as lists of parts by name; a part repaired in place repairs every flow that names it | Built |
| Test runs (`phone_connect { test }`, `pinkyctl flow run`): report, learn nothing, repair nothing | Built |
| Learning from the person: taps, keys and the fields they type into on the mirror, while a task holds the phone | Built (`internal/gateway/person.go`), on the fake phones only |
| A known state to start a test from: closing an app before opening it | Not yet: no driver can close an app |
| `pinkyctl flow run / list / show / check / parts`, JUnit XML | Built |
| Proposed changes from an agent's fix, reviewed by the person | Not yet: the repaired flow is kept without review |
| Screen map: recognizing the screen the phone is on and finding the path to another (`phone_goto`) | Not yet. Today the agent picks the parts and their order |
| Notes on apps: free text per app, shown to agents with the app's parts, added to by `phone_note` | Built (`internal/flow/notes.go`, `internal/gateway/notes.go`), on the fake phones only |
| Client: Flows page (web client): each app's notes, flows step by step and parts; edit the notes, remove a flow or a part | Built (`internal/deskapi/library.go`, `ui/frontend/src/pages/FlowsPage.tsx`) |
| Client: "Save as flow" on a finished task | Not yet |

Checked against the fake phones (`internal/flow`, `internal/gateway/flow_test.go`, `internal/gateway/flowparts_test.go`, `internal/mcp`, `internal/inapp`, `cmd/pinky/flow_e2e_test.go`). On a real phone, one thing was tried, once: the in-app agent checked the five tabs of one app on an iPhone, by hand in 58 s (36 s of them the model thinking), and the same request again replayed the kept flow in 24 s (6 s of model, 15 s of the phone: 1.4 to 3.8 s per step until its checkpoint showed). Later the same day that flow stopped at its first step, because it had been kept expecting the app's splash screen and the app was already running: the agent went on by hand, the flow repaired itself, and the next replay ran whole. That is why a launch's checkpoint is the app alone now. **A plan made of parts has not run on a real phone yet**: parts were learned there (six for that app). Then plans ran: the in-app agent, given the same request on two iPhones at once, ran the six parts in one call (13 s on one phone, 17 s on the other; 22 to 24 s for the task), a part one phone had learned by hand was used by the other a minute later, and the plan was kept as a flow of parts. A second test, posting and deleting a review, did not finish: the app's rating stars all have the same label and its slider no value an agent can read, a tap on a song's title had been kept as a part and matched two rows, and a feed's checkpoint was made of what somebody had posted. The last two are fixed in what a part and a checkpoint may be. How often real apps give elements ids, and how checkpoints behave during real animations, is still to measure (open questions 1 and 2).

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
3. **Wait for the checkpoint, where it is needed.** With `expect`, the pipeline polls the screen until the condition holds instead of waiting for the screen to stop changing (`Call.until` in `execute`): the replay moves on as soon as the expected screen shows. If it does not within the timeout, the step ran but the replay stops with `mismatch`. Without `expect`, the step settles as usual. The whole checkpoint is checked only where nothing else would notice a wrong screen: after the last step, and before a step that acts without looking first (a place on the screen, a key, an optional step). Before a step that looks for its target, an assert or a launch, finding the target is the check and the wait, and of the checkpoint only the app is checked (`checkpoint` in `flow.go`).

Actions are at least `Options.FlowPace` apart (300 ms), so a replay does not drive a phone faster than a quick person would.

A step that does not find its target, or whose checkpoint does not show, within 1.5 s of quick reads goes on looking the way a step by hand reads the screen: settled, and with OCR. Right after an input an iPhone answers with the tree it pushed, which is fast and can lack what sits over the page: a bar of tabs above the tab bar was missing from it for ten seconds, in a plan that tapped it right after the tab that shows it, while every read a moment later, by hand, had it. A replay acts at once, so it is the one that meets that tree. When a replay ends, the screen it returns is read that way too: the agent plans its next steps from it.

A step waits 10 s for its target and its checkpoint unless it or the flow says otherwise, and 30 s when it follows a launch: an app that starts cold shows a splash screen and loads for longer than a screen change takes. On a real iPhone a plan's second step once gave up after 10 s on an app that was still starting.

A replay stops at the first step that is not `ok` or `skipped`, and returns that step's status with a report of every step, a message that says what to do, and the whole screen. An agent then:

- on `mismatch`, does the step by hand and calls `phone_flow_run` again with `from_step`;
- on `approval_pending`, calls again with `from_step` and `approval_id`: the step's selector resolves to the same ref, so the approval's binding to tool and arguments holds;
- on anything else (`user_took_over`, `blocked_by_policy`, …), does what that status asks.

### Checkpoints and speed

What a checkpoint saves depends on the driver. Settling by polling takes two reads that agree, 250 ms apart, and a third read with OCR for the result; a checkpoint takes one read without OCR when the screen is already there. Where a read costs seconds (an iPhone through the helper: 0.5 to 3 s, about 7 s on a home screen), that is most of a step.

A checkpoint must name something the screen before did not have. One that already holds lets the replay run ahead of the app; `phone_flow_save` writes them that way, and hand-written flows have to take care. The next step's own wait for its target catches most of what a weak checkpoint lets through.

That wait is also why most checkpoints are not checked during a replay. On an iPhone a part's checkpoint named the button of a trending person ("原菜乃華") on the tab it opened; a few hours later the ranking had changed, and the replay waited out its timeout and stopped, while the tab it opened was there and the next step's tab button with it. Checking the screen a step leads to twice, once by its checkpoint and once by the next step's target, costs a read where both hold and a stop where only the checkpoint is stale. The checkpoints are still kept whole: they are what the Flows page shows a part leads to, what a plan's last step is checked against, and what tells two parts apart.

## How a flow comes to be

While a task runs, the gateway keeps each action that settled as a flow step (`traceStep`, on the task, in memory):

- The target becomes a selector written on the screen it was found on (`flow.SelectorFor`): the id when the screen has it once, with role and label as the fallback; else role and label, with an index among equals and the repeated id as the fallback; a point when the element has neither. A tap by `x`, `y` stays a point: the agent named a place, not the element under it.
- The screen the step led to becomes its checkpoint (`flow.Checkpoint`): the app in front and up to two elements the screen before did not have. It names what belongs to the screen rather than what the screen happens to show: ids first, then the labels of controls (buttons, tabs, switches, checkboxes, links), and plain text only when the screen has neither, and then only text of up to 30 characters that is no sentence. Labels with digits are left out (prices, times and counts change between runs). The first run on a real app showed why: a feed's checkpoint had been the name of whoever posted last and the first line of their post. Typing gets no checkpoint, since what it leads to depends on the text, which may be a parameter.
- A launch is kept by app id, and its checkpoint is the app in front and nothing else: an app opens where it was left, so the screen that shows is not the launch's doing, and the next step waits for its own target.

`phone_flow_save { name }` compiles those steps into a flow and saves it; `params` turns typed text into parameters. The agent reads the YAML it gets back and saves a corrected version with `yaml` when the task took detours. Saving under an existing name makes the next version.

### By themselves

People who give an agent a task should not have to know any of this, so with `Options.AutoFlows` (on in the app and the daemon) nobody saves, names or picks a flow:

- `phone_release` with outcome `done` keeps the task's steps (`keepFlows`): a flow named `<app>.<task>-<hash>` after the app, the task's name and what the task is known by (`phone_connect`'s `brief`, the person's request, which the in-app agent passes as it got it; else the task's name), with the task's name as its description, and the text the task typed as parameters with what was typed as defaults. Only when the agent made steps of its own, at least two steps ran and none are missing; a task that only replayed leaves its flow alone, and a flow that is there stays unless the task repairs it.
- A new `phone_connect` lists the flows and names the one kept from a task of the same name (`offerFlows`). The server's instructions and SKILL.md tell agents to name a task after what the person asked and to replay a flow that fits without being asked. A flow whose task was named otherwise is matched by the agent reading the list.
- A task that replayed one flow, stopped at a mismatch, went on by hand and ended with `done` keeps its steps as that flow's next version: replayed steps are recorded again with selectors from today's screens, the hand-made steps fill the gap.

A kept flow is a whole task, so it helps only when the same task comes back. What two different tasks share is in the app's parts, and those are what everything is made of.

### Parts

A part is one step that worked, with the screen it led to: "in this app, tapping Orders shows the list of orders". Parts are kept per app instead of per task, so that a task nobody ran before is put together from what earlier tasks found out: open the app, this part, then that one. The three layers:

| Layer | What it is | Who makes it |
| --- | --- | --- |
| Part | One action and the screen it leads to | Pinky, from every step that runs |
| Plan | Parts in the order a task needs them, with steps of the agent's own where no part fits, run in one call | The agent, per task |
| Flow | A plan kept under a name: the test case | Pinky, when a task ends well; people may edit it |

The point is what the agent no longer does: act, read the screen, think, act. It plans once from the list, and Pinky checks the screen after every step against the part's checkpoint. The agent looks at the screen where the plan stops and at the end.

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

- **What a part is.** A recorded step with a checkpoint and a target that belongs to the app rather than to what the app shows today: a tap on a control or on an element with an id, a key other than home, a launch. Typing into a field is a part without a checkpoint and without its text (`type: "${text}"`): what it leads to depends on the text, which the step that names the part gives.
- **What is no part.** A tap on text or an image without an id is a tap on content: a title in a list, a post in a feed. Tomorrow another title is there, and a part that taps "the newest song" by its name of today would tap the wrong one, so such a step stays the agent's to make each time (a kept flow holds it as a plain step, which is as fragile; see "Not yet", 5). Swipes are no parts either: scrolling shows more of the same screen, and what comes into view is content. Nor are taps at a place. A parts file that holds such parts from before still reads, without them.
- **What makes two parts the same.** The action and where it leads. The same tap that leads to another screen (Back, from two places) is another part. A checkpoint names what the screen before did not have, so the same tap from another screen may name other elements of the same screen: when a part with that action already holds on the screen the step led to, the step is that part seen again, not a second one.
- **An id that stays.** `p` and six hex digits of the app, the action and the checkpoint the part had when it was first kept. It does not change when the part is repaired, because flows name parts by it.
- **Kept when a task ends** (`keepParts`, with `Options.AutoFlows`), whatever the outcome: where a tap led is a fact also in a task that failed. Steps a replay ran count as seen again. `after` remembers up to three parts a part was recorded right after, as a hint at where its target shows; `seen` and `last` order the list, and an app keeps 300 parts at most, the longest unseen going first.
- **Offered** (`offerParts`) in the result of `phone_connect`, for the apps the task is held to or else the two apps seen last, unless a flow is this very task; and in the result of a `phone_launch` the agent makes, for the app it opened, once per task. Up to 40 per app, most seen first, one per line: `p815736 · tap tab "Orders" #dinely:tab:orders → #dinely:last shows · after p39deba`.
- **Run as a plan** with `phone_flow_run { parts: ["p39deba", "p815736"] }`: the parts in that order, as one replay, with `from_step` counting in the list. `steps` is the long form, which mixes parts with steps of the agent's own in the flow format, as JSON: `[{"part": "p39deba"}, {"tap": "Orders"}, {"part": "p7c01d2", "type": "ramen"}, {"assert": {"visible": "Your orders"}}]`. In a flow's YAML a step `- part: p815736` stands for the part as it is kept when the flow runs, and may set `name`, `intent`, `timeout_ms`, `optional`, and `type` for a part that types. A part that is not there is `invalid_argument`.
- **The agent plans, the replay checks.** Pinky does not work out a path: the agent reads the list and picks the parts and their order, optimistically. A part works wherever its target is on the screen (a tab bar is on many screens, though the part was recorded once), and the checkpoint tells whether it led where it did before. A wrong guess costs a `mismatch` with the screen, from which the agent goes on by hand.
- **Wrong parts.** A replay that ran a part's action and did not reach its checkpoint counts against the part; twice in a row without the part being seen in between, and it is no longer offered (the app changed). It stays in the file, for the flows that name it. A target that is not on the screen does not count: the part was used in the wrong place.
- **Repaired in place.** When a saved flow runs a part, the action leads to another screen than the part says, and the task goes on and ends with `done`, the part's checkpoint becomes what the step led to, under the same id: every flow that names the part follows. A plan the agent made does not repair parts this way, since the agent may simply have used a part in the wrong place; there the step is learned as another part.

### Learning from the person

An agent that cannot do a step asks the person (`phone_handoff`), or the person takes over. What the person then does on the mirror in Pinky's window is kept like a step the agent made (`person.go`): it joins the task's trace, becomes a part of its app when the task ends, and the next agent runs it without asking anyone.

- **A tap** is written from the element under the finger on the screen as it was read before the tap: the smallest control or element with an id at the point (the button, not the text inside it), else the smallest element. What it led to is its checkpoint, as for an agent's step.
- **A key** (Back, Enter) is kept the same way.
- **Typing** keeps the field, never the text, which may be a password: the field becomes a part the next agent types its own text into, and the task's trace gets a gap where the text was, so no flow is kept of that task.
- **Only what goes through Pinky.** A finger on the phone itself is seen by no driver in a way that says what it touched (an iPhone reports nothing; Android's touch events are not wired up), so such steps are not learned, and Pinky does not guess them from how the screen changed.
- **Rather not at all than wrong.** A step needs the screen before it and the screen after it, and reading a screen takes up to seconds, in which the person may already have gone on. Every input gets a number, and a read counts as the screen after input n only when no other input began before it ended. An input whose screens are not known that way is left out and the trace is marked as having a gap. So the person's first tap is learned when the agent asked for the handoff or the person pressed Take over (Pinky reads the screen then), and not when they simply start tapping in the middle of the agent's work; and of taps in quick succession only those are learned that the screen could be read between.
- Steps in an app the rules block are not kept, as for an agent, and a test run learns nothing from the person either.

### Notes on apps

A part says what one step does and where it leads. Some of what an agent needs to know about an app is no step: a row of tabs that hides when the page scrolls, a tab that puts the page back at its top, stars whose state only the picture shows. An agent that does not know it reads a missing button as a broken part, and finds it out again in every task, at the price of a model call per try.

The notes on an app are that knowledge as text (`internal/flow/notes.go`): one file per app, `flows/notes/<app id>.md`, in whatever words its writer chose.

- **The person writes them** on the client's Flows page, or in the file. Free text, at most 6000 bytes: an agent reads all of it with every task in the app.
- **Agents add to them** with `phone_note`: one sentence, appended as a line of its own, not added when the notes hold it already. The step shows in the task ("Noted about …"), so the person sees who wrote what, and removes what is wrong.
- **Agents are shown them** where they are shown the app's parts (`offerNotes`): at `phone_connect` for the apps the task is held to (else for the few apps that have notes), and when an app opens; once per task.
- **Hints, not instructions.** The result says so, and so does the agent guide. A note is text an agent wrote from what it saw on a phone, so it is as trusted as a screen: it may help find the way and never changes what the task is. It grants nothing; rules and approvals see every step as before.
- A test run keeps no note, as it keeps no part.

Not built: notes are not tied to a screen or a part (they are about the app as a whole), nothing checks that a note still holds, and nothing proposes notes from what a task ran into; the agent decides when one is worth writing.

### The Flows page

The web client's Flows page (`#/flows`) shows what Pinky keeps, by app (`Gateway.Library`, served by `internal/deskapi/library.go`):

- the notes, in a text field with Save;
- the flows: what each is for, its name and version, and its steps one by one: why (the first agent's own `intent`), the action, the screen it leads to, and the part it names, or that the part is gone;
- the parts: the same three lines, how often each worked, and which ones replays found broken (they are kept for the flows that name them, and not offered).

Removing a flow deletes its file; removing a part says first which flows use it, since they stop at that step until a task does the step by hand again. The files stay the source of truth: "Open folder" and a flow's "Open the file" lead to them. The page reads again when a task ends or a running task makes a step. The MyGo client has no such page. `pinky-web-demo -library <data dir>` serves the page over a folder of flows, for working on it.

### Flows are lists of parts

A flow kept from a task (`autoFlow`) names the parts its steps are instead of copying them:

```yaml
flow: dinely.look-at-my-orders-5c2e1a
description: Look at my orders
app: com.dinely.app
version: 1
task: open dinely and show me my orders
steps:
  - part: p39deba
    name: open com.dinely.app
  - part: p815736
    name: tap tab "Orders" #dinely:tab:orders
```

This is what page objects are in test automation: when the app changes, one part is fixed and every test that uses it works again, instead of each test breaking by itself. `name` is for people who read the file; steps that are no part (typing into the focused field, a tap at a place) are kept as they are, and flows people write may mix both. A plan the agent ran and ended with `done` is kept the same way, so the request that was new today is a flow tomorrow.

`phone_connect` names the flow kept from the same request, and lists the app's parts whether or not there is one: the flow is last time's plan, to run again when the task is the same, and the parts are there for when it differs.

### Test runs

A regression test is there to find out that the app changed, so a test run must not quietly adapt to it. `phone_connect { test: true }`, which `pinkyctl flow run` passes, marks the task as one: a replay that stops is reported, no flow is kept or repaired, no part is learned, counted against or repaired. Fixing what a test found is a task of its own, by an agent or a person.

Recording is of the running task only: a task that ended cannot be saved as a flow yet, because history keeps a step's screen as text, without ids. Keeping selectors with each step in history is what the client's "Save as flow" needs.

## Interfaces

- MCP: `phone_flow_run` (a flow, parts, or YAML), `phone_flow_save`, `phone_flow_list` (flows, one flow, or an app's parts), `phone_note` (a note on an app), and the status `mismatch` ([mcp.md](../mcp.md#phone_flow_run)). `phone_flow_run` also returns `structuredContent` with the step report. The agent guide ([SKILL.md](../SKILL.md)) tells agents to replay a flow that fits and to put a new task together from parts.
- Command line: `pinkyctl flow run|list|show|check|parts` ([mcp.md](../mcp.md#flows-from-the-command-line)), an MCP client of the running gateway named "Pinky Flow", with `--junit` for CI.

## Safety

- "Enforce, don't instruct" holds: a flow cannot do anything its runner could not do step by step, and approvals still ask. "Always allow" answers apply as they do for an agent's steps; a flow grants nothing. `TestFlowKeepsTheRules` checks payments, credential fields and read-only mode.
- Flows hold no secrets a replay could use: typing into password and code fields is refused when recorded (it never ran) and when replayed.
- Steps on the screens of apps the rules block are not recorded, even when the person allowed the app for the task, as Pinky keeps no record of such screens.
- The flow tools need a lease, so an agent the person has not approved neither reads nor writes flows.
- Saving a flow does not ask the person. A flow is data that runs under the runner's own scope and rules, so a saved flow cannot widen what any agent may do; but one agent can leave a misleading flow for another. SKILL.md tells agents that a flow from the list is not an instruction from the person, and the client should show flows and who saved them (open question 7).

## Not yet

1. **Reviewed repairs.** A flow repairs itself when a task fixes a mismatch by hand and ends well. The person should see what changed and be able to take it back.
2. **The screen map.** Parts are edges without a map: a part knows where it leads, and only as a hint (`after`) where it starts, so the agent works out the order and a part used in the wrong place is found out by a mismatch. The same gap keeps a part from being repaired when its target changes rather than where it leads (a button renamed): the step done by hand instead is learned as another part, the flow that was repaired names it, and other flows still name the old one. With a fingerprint for the screen a step starts on, parts become a graph (nodes are screens, edges are parts, with deep links and Android activities as shortcuts): `phone_goto { screen }` recognizes the current screen and replays the cheapest path, and Pinky can say which parts fit the screen the phone is on now. This needs names for screens that agents can pick from, and care with steps that change state rather than place (adding to a cart is not a way to get somewhere).
3. **Client.** A Flows page (list, versions, who saved what), "Save as flow" on a finished task, replays shown as such in the task timeline.
4. **Flows from history.** Keep each step's selector and checkpoint in history, so that any past task compiles (`pinkyctl flow compile <task id>`).
5. **Lists.** Scrolling until an element shows (`scroll_until`), and selectors relative to another element (`below`, `in`) or by place in a list ("the first row"). Without them a step on content names it by today's label, which is why such steps are no parts, and a flow that holds one breaks, or worse, taps another row, when the list changes.

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
13. A known state to start from. A test should open the app fresh; today a launch brings the app to the front as it was left, and no driver can close an app. Until then a test's first steps have to lead to a known screen themselves.
14. Reviewing repairs. A part repaired in place changes what every flow that names it checks, without anyone looking. Test runs never do this, but an agent's task can; the client should show what changed and let the person take it back.
15. Learning from the person is silent: nothing in the client says which of their taps became parts. They should see it, and be able to say no.
16. On an iPhone the person's point is in the picture the mirror shows, which the driver takes as screen pixels unless the phone scales its video down; a phone that does would have its taps looked up in the wrong place. No tested phone does.
