# Flows: replay without a model

Status: draft design. Nothing here is built yet. It details the M4 rows of [DESIGN.md](../DESIGN.md) "Turn a successful task into a flow that replays without a model and hands back to the agent on a mismatch" and "CLI and SDK for code mode", with automated tests on real phones as the first use.

## Problem

An agent drives a phone one step at a time: read the screen, think, call a tool, wait for the screen to settle, read it again. Thinking takes seconds per step and dominates the time of a task. Tests and routine tasks repeat the same steps and the same screen changes every time, so the model pays again and again for decisions it has already made.

## Goal

Model time grows with what is new, not with the number of steps.

```
now:    [model] tap [model] tap [model] tap [model] type [model] check
flows:  run flow (tap, tap, tap, type, check) ── only on a mismatch ──> [model]
```

- Split exploring from executing. The model explores a screen once; Pinky keeps what worked and replays it at the phone's own speed.
- A replay that meets an unexpected screen hands back to the agent, which fixes it; the fix becomes a proposed change to the flow.
- Every rule keeps working: replays run through the same policy, approvals, history and Stop as agent steps.

Non-goals: a device farm, scheduling or sharding (DESIGN.md), and visual testing beyond what checkpoints need.

## Prior art

| Tool | What we take |
| --- | --- |
| [Maestro](https://github.com/mobile-dev-inc/Maestro) | Declarative YAML flows with selectors and assertions; waiting for what is expected instead of fixed sleeps |
| Stagehand (browser) | Caching the actions the model chose, keyed by the instruction and the page, and falling back to the model on a miss |
| [mirroir-mcp](https://github.com/jfarcand/mirroir-mcp) | Compiled skills that replay without a model |
| Self-healing test tools | A failed locator is repaired from the run and proposed for review, not silently patched |

## Building blocks

### 1. Flow

A flow is a named, versioned, parameterized list of steps for one app, stored as YAML in Pinky's data folder (`<data>/flows/<app id>/<name>.yaml`) and exportable to a project's repository.

```yaml
flow: settings.open-display
app: com.android.settings
version: 3
params:
  query: { default: "Brightness" }
start: { screen: settings.home }        # precondition, a screen-map node (below)
steps:
  - tap: { id: "com.android.settings:id/search_action_bar" }
    expect: { visible: { role: textfield } }
  - type: { text: "${query}", submit: true }
    expect: { visible: { label: "Brightness level" } }
  - tap: { label: "Brightness level", role: button }
    expect: { screen: settings.display }   # checkpoint
end: { screen: settings.display }
```

- Steps are the tools agents already have (`tap`, `type`, `swipe`, `key`, `launch`) plus `wait` and `assert`.
- No refs and, where avoidable, no coordinates: targets are selectors.
- Every step may carry `expect`, its checkpoint. The compiler adds one after each step that changes the screen.
- Tests are flows whose last steps are `assert`s.

### 2. Selectors

`screen.Element` already carries what selectors need: `ID` (resource id or accessibility identifier), `Role`, `Label`, `Path` (ancestry signature built for stable identity), `Bounds` and `States`.

A selector lists what it knows, strongest first; the runner takes the first that matches exactly one element:

1. `id`, when the app sets ids
2. `role` + `label`, optionally narrowed by `path`
3. `label` alone, or `contains`
4. relative: `below: {label: …}`, `in: {id: …}`, `index` among matches
5. OCR text, then a point as a last resort, relative to the screen size

The compiler writes several of these per target so that a replay survives a renamed label or a moved button; the runner records which one matched, and a flow whose strong selector keeps failing is flagged for review.

### 3. Screen fingerprints and checkpoints

A fingerprint names a screen: the app, plus a few elements that identify it (ids first, then role and label), minus anything that varies (times, counts, list contents). The compiler derives it from the screens recorded in history and the person or agent can edit it.

A checkpoint passes when the fingerprint, or the `visible` / `gone` conditions, hold. Checkpoints do double duty:

- they verify the replay is where it should be, and
- they replace settling: the runner moves on as soon as the expected screen shows, instead of waiting for the screen to stop changing (up to `SettleMax`, 3 s). Driver-level change events (`driver.Settler`) make the check cheap.

### 4. Compiler: history to flow

History already stores every step's tool, arguments, target rectangle and point, and the screen text after it (`history.Step`). The compiler turns a successful task into a flow draft:

1. Replace each ref with selectors for the element at the step's target, from the screen before the step.
2. Turn typed text into parameters (the person or agent names them).
3. Drop detours: steps whose screen the task later returned to without using, observes, failed calls.
4. Add a checkpoint after each step from the screen after it.
5. Split the task at stable screens into reusable segments, which become screen-map edges.

The draft is shown as a diff-like review in the client (steps, selectors, checkpoints) and saved on approval. Agents can also ask for it (`phone_flow_save`), which still needs the person's yes.

### 5. Runner

The runner lives in the gateway, not in the agent or a script, so it inherits everything that is enforced there: lease, scope, task app list, read-only mode, policy rules, approvals, blocked apps, credential fields, takeover and Stop. A replay is a task step sequence in history like any other, marked as a replay of `flow@version`.

- Each step: resolve the selector on the current screen, act, wait for the checkpoint (timeout per step, default from the flow).
- Pacing keeps a floor between actions (about 300 ms, as people pace input; see WORKING.md).
- An action the rules ask about opens the usual approval; the replay waits, then continues or stops.
- Result: `settled` with the end screen, or `mismatch` with the step, the expected checkpoint, the actual screen and the remaining steps.

### 6. Healing: back to the agent on a mismatch

On a mismatch the runner stops and returns control with everything the agent needs to continue: which flow and step, what was expected, what is there, and what is left. The agent resolves it with the normal tools (close a dialog, scroll, pick the new button) and then either resumes the flow from a step (`phone_flow_run { resume: … }`) or finishes by hand.

Pinky records what the agent did between the mismatch and the resume and proposes it as a change to the flow: a new selector, an inserted step for a dialog that now appears, a new checkpoint. The person accepts or rejects it; accepted changes bump the version. Flows therefore follow the app as it changes, and the model is paid only for the change.

Without an agent (CI), a mismatch is a test failure with the expected and actual screens attached. An optional heal mode hands the failure to an agent to propose the fix.

### 7. Screen map

A graph per app (and one for system screens): nodes are fingerprints, edges are flow segments that go from one to another, learned from compiled tasks and replays. Edge cost is the measured replay time; entry shortcuts are edges too (`launch`, an Android activity or a deep link where the app allows it).

`phone_goto { screen: "settings.display" }` recognizes the current screen, finds the cheapest path and replays it, handing back on a mismatch like a flow. Tests become "go to the screen, check what is specific to it", and navigation is written once for all of them.

## Interfaces

MCP (additive; listed in docs/mcp.md once built):

| Tool | Does |
| --- | --- |
| `phone_flow_run` | `flow`, `params`, optional `resume` step; returns `settled` or `mismatch` |
| `phone_goto` | `screen`; navigates by the screen map |
| `phone_flow_save` | Proposes a flow from the current task or a range of its steps |

Flows and the screen map are also an MCP resource, and SKILL.md tells agents to look for a flow or a `goto` target before stepping by hand.

CLI for code mode and CI: `pinky flow run <file|name> [--param k=v] [--device …] [--junit out.xml]`, `pinky flow compile <task id>`, `pinky flow list`. It talks to the running gateway like `pinkyctl mcp`, so the same rules apply, with the agent identity of the CLI.

Client: a Flows page (list, versions, proposed changes to review), a "Save as flow" action on a finished task, and replays shown in the task timeline with their checkpoints.

## Safety

- "Enforce, don't instruct" holds: a flow cannot do anything its agent could not do step by step, and approvals still ask. "Always allow" answers apply as they do now; a flow grants nothing.
- Flows never hold secrets. A `type` into a password or code field is refused at compile time and at run time; flows hand off for sign-in like agents do.
- Flows store selectors and fingerprints, not screenshots or screen text beyond what identifies a screen. Compiling from a task in a blocked app the person allowed for the task is refused, since Pinky keeps no screen text of it.
- Flows run only against apps in the agent's scope and the task's app list.

## Ownership

`internal/gateway/`, `internal/mcp/`, `internal/history/` and `cmd/pinky/` belong to the main session (WORKING.md); the runner, tools and compiler go there, likely as a new `internal/flow/` package for the format, selectors, fingerprints and compiler, with the gateway calling it. Selector resolution needs nothing new from `internal/screen` (ids and paths are there). The client pages belong to the desktop client session.

## Phases

1. **Format and runner.** Flow YAML, selectors, checkpoints, the runner in the gateway, `pinky flow run`, hand-written flows. Proves replay without a model and the speed of checkpoint-driven waiting.
2. **Compiler.** History to flow draft, review in the client, `phone_flow_save`. The model explores once, then replays.
3. **Healing.** `mismatch` result, resume, proposed changes from the agent's fix, versions.
4. **Screen map.** Fingerprint graph from compiled segments, `phone_goto`, deep-link and activity shortcuts.

Each phase is usable on its own; 1 and 2 together already deliver "explore once, replay fast".

## Open questions

1. How often drivers fill `Element.ID` on each platform and app (Android resource ids are common; iOS accessibility identifiers depend on the app). This decides how much weight label selectors carry, and how they cope with the phone's language.
2. Fingerprint strictness: too strict mismatches on every run, too loose acts on the wrong screen. Needs measurement on real apps.
3. Format: Pinky's own YAML, or a compatible subset of Maestro's so existing flows and habits carry over.
4. Lists and dynamic content: scrolling to find an element (`scrollUntilVisible`), and selectors for the nth item of a list.
5. Whether to add implicit caching too (Stagehand-like: an agent's intent plus a fingerprint as the key), so agent tasks speed up without anyone saving a flow.
6. How much of the speed after the model is gone comes from checkpoints alone, and where reads (iPhone: 0.5–3 s, about 7 s on a home screen) become the limit.
