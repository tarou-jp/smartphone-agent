# Pinky MCP reference

Pinky's gateway serves one MCP server, `pinky`, with 17 tools. Agents launch `pinkyctl mcp` (Pinky's executable with the argument `mcp`), a stdio shim that forwards to the running gateway over its user-only Unix socket (and starts the gateway first when none runs: the Pinky app on macOS, `pinky daemon` elsewhere). Agents that cannot start a command can use Streamable HTTP instead (below). The agent-facing guide is [SKILL.md](SKILL.md); the design is in [DESIGN.md](DESIGN.md), section "Agent interface".

## Connecting an agent

### With a click

Settings › Agents › Connect an agent lists the agents Pinky finds on this computer, each with Connect, and connects one with a click. Until some agent can reach Pinky (its settings have Pinky, or one has connected), a card in the sidebar offers the same: Connect when Pinky found one agent, Choose… for several, and where to get one (Claude Code, Claude Desktop) when there is none; the Tasks list says no agent is connected.

Connect adds an MCP server named `pinky` to the agent's own settings, which starts this Pinky by its full path with the argument `mcp` (as `/Applications/Pinky.app/Contents/MacOS/Pinky mcp`): no other program can stand in for a path, and agents started from the Dock or the Start menu find it without the terminal's `PATH`.

- Pinky changes nothing until the person clicks Connect (or Update, Fix or Remove) for that agent.
- Where the agent has a command for its MCP servers, Pinky runs it, found on the `PATH` of the person's login shell and where installers put it: `claude mcp add --scope user pinky -- <path> mcp`, `codex mcp add pinky -- <path> mcp`, `copilot mcp add pinky -- <path> mcp`, `gemini mcp add --scope user pinky <path> mcp`, and `… mcp remove` for Remove. Otherwise Pinky changes the settings file itself, in Pinky's entry only: the rest of the file stays as it was, comments, order and formatting included. Pinky reads the file again after the change and refuses one that would alter anything else, and it leaves a file with a mistake in it alone: Settings says what is wrong where, with Show the file.
- Before any change Pinky keeps a copy of the file as it was in its data folder, `agents/backups/<agent>/<time>-<file>` (the newest ten for each agent), and Settings offers to show it.
- It installs Pinky's skill, [SKILL.md](SKILL.md), as `pinky/SKILL.md` in the folder of skills the agent reads: `~/.claude/skills` for Claude Code, `~/.agents/skills` for the agents that share it ([agentskills.io](https://agentskills.io)). Remove takes it out, from `~/.agents/skills` once no agent that reads it has Pinky. A `pinky` skill that is not Pinky's (its front matter names another) stays as it is.
- An agent is Connected when its entry starts this Pinky (or `pinkyctl`, when that is on the `PATH`) and its skill is this Pinky's. Settings marks an entry that starts a Pinky that is no longer there (the app moved), `pinky` (coreutils' command on many computers, below), another copy of Pinky or something else under the name `pinky`, and an old skill; Update sets it right. The sidebar's card offers Fix for the first two, which leave the agent without Pinky.
- Settings reads the agents again every 10 s while it shows them, and the card every 30 s, so an agent installed meanwhile shows up.

| Agent | Found by | Settings | Changed with | Skills |
| --- | --- | --- | --- | --- |
| Claude Code | `claude`, or `~/.claude.json` | `~/.claude.json` (or `$CLAUDE_CONFIG_DIR/.claude.json`): `mcpServers`, user scope | `claude mcp add` and `remove`, else the file | `~/.claude/skills` |
| Claude Desktop | Claude.app, or its folder | Pinky's extension (below); an entry in `claude_desktop_config.json` (`mcpServers`, in `~/Library/Application Support/Claude`, `%APPDATA%\Claude` or MSIX's folder, `~/.config/Claude`) is set right there | opens the extension in Claude Desktop, which asks to install it | — |
| Codex (CLI, IDE extension, ChatGPT app) | `codex` (also inside ChatGPT.app), or `~/.codex` | `~/.codex/config.toml` (or `$CODEX_HOME`): `[mcp_servers.pinky]` | `codex mcp add` and `remove`, else the file | `~/.agents/skills` |
| Cursor | Cursor.app, or `~/.cursor/mcp.json` | `~/.cursor/mcp.json`: `mcpServers` | the file | `~/.agents/skills` |
| VS Code | the app, or its User folder | Copilot's global `~/.copilot/mcp-config.json` (or `$COPILOT_HOME`): `mcpServers`, `"type": "local"`; an entry in VS Code's `User/mcp.json` (`servers`) is set right there | the file | `~/.agents/skills` |
| Copilot CLI | `copilot` | the same as VS Code | `copilot mcp add` and `remove`, else the file | `~/.agents/skills` |
| Windsurf, now Devin Desktop | the app, `~/.config/devin`, `devin` or `~/.codeium/windsurf` | Devin's `~/.config/devin/mcp_config.json` (`%APPDATA%\devin`), or Windsurf's `~/.codeium/windsurf/mcp_config.json`: `mcpServers` | the file | `~/.agents/skills`; Windsurf's own `~/.codeium/windsurf/skills` |
| Gemini CLI | `gemini` (`~/.gemini` is Antigravity's too) | `~/.gemini/settings.json` (or `$GEMINI_CLI_HOME/.gemini`): `mcpServers` | `gemini mcp add` and `remove`, else the file | `~/.agents/skills` |

Claude Code, Codex, Copilot CLI and Gemini CLI read their MCP servers when a session starts, so Settings says to start a new session; Cursor, Windsurf and Claude Desktop's settings file when they open (quit and open them again). VS Code starts a new server at the next message.

Claude Desktop keeps its settings in memory while it runs and writes them back whole, so a change to `claude_desktop_config.json` made while it runs may be lost. Connect therefore installs Pinky as an extension: it writes `Pinky.mcpb` into `<data>/agents/` (the extension below, made to start this Pinky) and opens it in Claude Desktop (`open -b com.anthropic.claudefordesktop` on macOS), which asks the person to install it. Settings then finds it among Claude Desktop's extensions (`Claude Extensions/*/manifest.json` named `pinky`) and shows it connected, or turned off; only Claude Desktop removes it, in Settings › Extensions. An entry that the person added to `claude_desktop_config.json` comes first in Claude Desktop, so Pinky changes that one instead, and says to quit and open Claude Desktop again.

`PINKY_AGENTS_PATH`, for tests of the app, is the whole search path for agents' commands, and leaves out the login shell, the system's application folders and every command outside that path.

Not yet: Antigravity (`~/.gemini/config/mcp_config.json`), opencode, Zed, Kiro, Cline, Factory Droid and Qwen Code; add them by hand. How it is built and what was tried: [notes/agents.md](notes/agents.md).

### Claude Desktop extension

Each release has `pinky-<version>.mcpb`, an [MCP bundle](https://github.com/modelcontextprotocol/mcpb): opened with a double-click, or dragged into Claude Desktop's Settings › Extensions, it installs as the extension Pinky, with no settings file to change. The bundle does not carry Pinky: it starts the Pinky installed on the computer, `/Applications/Pinky.app` or `~/Applications/Pinky.app` on a Mac (then the cask's `pinkyctl`), `%LOCALAPPDATA%\Programs\Pinky\Pinky.exe` on Windows, so install Pinky first. Its setting Pinky takes another path to Pinky's program on a Mac. `go run ./scripts/release mcpb -version <version> -out <file>` makes it; the release workflow adds it to every release. The one Settings › Agents opens names this Pinky's path instead.

### By hand

Settings › Agents › Add an agent by hand shows both commands below for the copy of Pinky that is open, by its own path, with buttons that copy them.

Claude Code:

```sh
claude mcp add --scope user pinky -- pinkyctl mcp
mkdir -p ~/.claude/skills/pinky && cp docs/SKILL.md ~/.claude/skills/pinky/SKILL.md
```

`--scope user` makes Pinky available in every folder; without it, Claude Code adds it to the folder it runs in only.

`pinkyctl` is the command that Homebrew's cask (`brew install --cask diygod/tap/pinky`) and the Linux packages put on the `PATH`. Without them, or for agents started from the Dock or the Start menu, which may not see the terminal's `PATH`, give the app's own executable instead: `/Applications/Pinky.app/Contents/MacOS/Pinky` on a Mac, `Pinky.exe` in `%LOCALAPPDATA%\Programs\Pinky` on Windows, `/opt/pinky/pinky` (the .deb) or `~/.local/pinky.app/pinky` (the .tar.gz's `install.sh`) on Linux.

The command used to be named `pinky`, which is also a coreutils command (on many Macs through Homebrew's coreutils, and on every Linux): an agent set up with `pinky mcp` that reports "Connection closed" runs coreutils' `pinky` (`pinky --version` says GNU coreutils). Set it up again with `pinkyctl mcp` or the app's path; Settings › Agents marks such an agent and sets it right with Update.

Any client that takes a JSON config:

```json
{ "mcpServers": { "pinky": { "command": "pinkyctl", "args": ["mcp"] } } }
```

To try Pinky without a phone, use `pinkyctl mcp --fake` (it starts `pinkyctl daemon --fake` when no gateway runs) or run `pinkyctl daemon --fake` yourself first.

### Over HTTP

For agents on another machine, in a container, or clients that speak only HTTP, Pinky serves the same tools over Streamable HTTP. It is off by default: turn on Settings › Remote agents in the app, or run `pinky daemon --http 127.0.0.1:7465`. The endpoint is `http://<address>/mcp`, on this computer only (`127.0.0.1:7465`) unless the person picks another address. Every request needs the agent token as a bearer token; `pinky token` prints it.

Claude Code:

```sh
claude mcp add --transport http pinky http://127.0.0.1:7465/mcp --header "Authorization: Bearer $(pinkyctl token)"
```

A JSON config:

```json
{ "mcpServers": { "pinky": { "type": "http", "url": "http://127.0.0.1:7465/mcp", "headers": { "Authorization": "Bearer <token>" } } } }
```

- Without the token, or with another one, requests get `401`. Browser pages from other origins get `403`, and so do requests to `127.0.0.1` that name another host (DNS rebinding).
- Each MCP session over HTTP is one Pinky session, with the same first-connection approval and scopes as over stdio. The agent's name gets " (remote)" after it ("Claude Code (remote)"), so a remote agent is asked about on its own first connection and never gets the scope the person gave the agent of that name on this computer. Closing it (`DELETE`), an hour without requests, or turning Remote agents off ends it, with its leases and open approvals.

- Protocol: the official Go SDK (`github.com/modelcontextprotocol/go-sdk` v1.8.0), newline-delimited JSON-RPC over stdio; the SDK negotiates the protocol version.
- Server info: name `pinky`, title `Pinky`; the server sends short usage instructions at initialization.
- Identity: the agent's name in Pinky comes from the client's `clientInfo` (`claude-code` shows as "Claude Code"). The shim presents the agent token from `agent-token` in the config directory (mode 0600).
- Sessions: one MCP session is one Pinky session. Closing it (the agent exits, or closes stdin) ends its leases and withdraws its open approval panels without acting.

## Results

Every tool returns one text block (plus a JPEG image block for `phone_observe { screenshot: true }`). Results are text for models, not JSON:

```text
<status> · <device name> · approval_id: <id> · rule: <rule> · settle: timeout
note: "<the person's note>"
<one or two sentences>
<the screen, or the lines that changed>
```

- The first line is the status, or `error: <code>` for errors (then the MCP result also has `isError: true`). Details follow as `key: value`, separated by ` · `: `approval_id`, `rule`, `decision`, `settle`, `matched`, `holder`, `setup`, and `no change` when the screen did not change.
- The screen is the format in DESIGN.md: a header line, then events (`toast:`, `notification:`, `dialog:`), then one element per line `[ref] role "label" @x,y state`, then `ocr` lines.
- After an action, only the changes are sent: `+` added, `-` removed, `~` changed (including `~ screen …` for a changed header). A new app, size, orientation or lock state, or a diff longer than the screen, sends the whole screen.
- `phone_connect`, `phone_observe` and `phone_handoff` always send the whole screen.
- A blocked app shows as `app: blocked app` with no elements; its toasts and dialogs are dropped, and no screenshot is returned. An app the person allowed for the task (see `phone_launch`) shows as usual, but still without screenshots.

### Statuses

| Status | Meaning |
| --- | --- |
| `settled` | The call ran and the screen stopped changing, or settling hit its 3 s limit (`settle: timeout`) |
| `uncertain` | The action may have happened (the driver failed mid-action, the phone dropped). Observe before anything else |
| `blocked_by_policy` | A rule denied the action; `rule` names it (`credentials` for password and code fields, `Read-only`, `Finance apps`, …) |
| `approval_denied` | The person said no, or nobody answered in time; `note` may say why |
| `approval_pending` | The call blocked for 45 s; repeat it with `approval_id` |
| `stale` | The screen changed while the approval waited, so nothing was done |
| `user_took_over` | The person took over; returned at once with the current screen. Later calls return `paused` until they hand back |
| `paused` | The person is in control. Wait with `phone_handoff` |
| `stopped` | The person pressed Stop; the lease is gone, and this session may not use that phone again |
| `device_locked` | The phone is locked. Call `phone_handoff` |
| `mismatch` | A replay (`phone_flow_run`) met a screen the flow does not expect and stopped: a step's target is not there (nothing was done), or the step ran and its checkpoint did not hold. The message names the step and the whole screen follows |
| `device_needs_setup` | The phone is connected, but Pinky cannot use it until the person does a step on it or on the computer, and nothing was done. `setup` names the step and the message says what to do: on an iPhone `trust`, `developer_mode`, `unlock`, `passcode` (iOS asks for it before the WebDriverAgent helper may control the phone), `no_disk_image`, `disk_image`, `install_helper`, `connection` (the connection chosen in Settings › Phones does not work with the phone: the person chooses another); on Android `authorize` (allow USB debugging), `usb_permission`, `usb_driver`; `preparing` (Pinky prepares the phone: wait a few seconds, then observe) and `update` (the phone's iOS is too old: end the task) are no steps to hand off. Otherwise call `phone_handoff` asking for the step: it ends by itself once the phone is ready, with the whole screen. After `usb_driver` the phone comes back under another id: call `phone_connect` again. The lease waits with the phone instead of ending after 60 s |

### Errors

| Error | Meaning |
| --- | --- |
| `device_offline` | The phone is not connected. A lease survives 60 s of disconnection |
| `lease_held` | Another session holds the phone; `holder` names its agent and task |
| `scope_denied` | The app is outside the agent's scope or the task's app list |
| `not_found` | The ref is gone, or no installed app matches |
| `unsupported` | The phone or backend cannot do this, such as `recents` on iPhone, or `phone_files` on iPhone |
| `invalid_argument` | Bad or missing arguments, a misused `approval_id`, or no phone connected in this session |
| `internal` | The call was cancelled, or the gateway failed |

## Common arguments

Every action tool (`phone_tap`, `phone_type`, `phone_swipe`, `phone_key`, `phone_launch`) takes:

| Argument | Type | Meaning |
| --- | --- | --- |
| `device` | string | The phone, by id, name or a unique id prefix. Required when the session holds more than one phone |
| `intent` | string | One sentence on why, shown on the approval panel and in history |
| `approval_id` | string | From `approval_pending`: repeat the same call with it to keep waiting |

`phone_observe`, `phone_apps`, `phone_wait_for`, `phone_release`, `phone_flow_save` and `phone_flow_list` take `device`; `phone_flow_run` takes all three; `phone_files`, `phone_request_approval` and `phone_handoff` take `device` and `approval_id` (and `phone_files` takes `intent`). Unknown arguments are rejected.

## Tools

### phone_connect

Leases a phone for a task and returns its first screen.

| Argument | Type | Meaning |
| --- | --- | --- |
| `task` | string, required | One line naming the task, shown to the person |
| `device` | string | Optional when exactly one phone is online, or the session already holds one, or none is online and exactly one is connected but needs setup |
| `apps` | string[] | Optional app allow-list for the task (names or ids). Launching another app returns `scope_denied`; typing into one asks the person. Launchers and system UI are always allowed |
| `brief` | string | What the person asked for, in their words. Pinky knows a task that ran before by it (case and spacing aside; by `task` when it is not passed) and then names the flow that replays it |
| `read_only` | boolean | Run the task read-only, whatever the agent's scope: anything the person's rules would ask about is refused (`blocked_by_policy`, rule `Read-only`), and typing works only in search fields. Connecting again for the same task never widens it. A new agent's approval card then offers read-only access first |

The first connection of an agent the person has not approved yet returns `approval_pending` (rule `New agent`); repeat with `approval_id`. Another session's phone returns `lease_held`. Calling again with the same task keeps the lease; another task name ends the previous task (outcome `replaced`) and starts a new one. After the person pressed Stop, the session cannot connect to that phone again. While the person is in control of the phone (they took over and have not handed back, whichever task was running), the lease starts paused and the result is `paused`: call `phone_handoff`. A phone that is connected but waits for a step (Trust, Allow USB debugging, Developer Mode, …) is leased all the same and returns `device_needs_setup`, without a screen, so `phone_handoff` can ask the person for the step; other calls return `device_needs_setup` until the phone is ready.

### phone_release

Ends the task and frees the phone. With outcome `done`, Pinky keeps the task's steps as a flow by itself, and the message names it (see "Flows by themselves" under `phone_flow_run`).

| Argument | Type | Meaning |
| --- | --- | --- |
| `outcome` | `done` \| `failed` \| `partial` \| `cancelled` | Default `done` |
| `summary` | string | For the person's history |
| `device` | string | Default: every phone the session holds |

### phone_observe

Returns the whole current screen.

| Argument | Type | Meaning |
| --- | --- | --- |
| `wait_ms` | 0–30000 | Wait until the screen differs from what the agent last saw (then let it settle), or until this passes |
| `screenshot` | boolean | Add a JPEG screenshot, in the screen's pixel space. None while the phone is locked or a blocked app is in front |

Works while the person is in control and while an approval is open.

### phone_tap

| Argument | Type | Meaning |
| --- | --- | --- |
| `ref` | integer | The element to tap |
| `x`, `y` | integers | A point instead of a ref (both or neither) |
| `long_press` | boolean | Press and hold |

The policy checks every element under the finger, not only the smallest: tapping a price label inside a Pay button counts as tapping Pay.

### phone_type

| Argument | Type | Meaning |
| --- | --- | --- |
| `text` | string, ≤ 16384 chars | The text; may be empty with `submit` |
| `ref` | integer | The field (tapped first to focus it); default the focused field. Must be a text field |
| `append` | boolean | Keep the field's text |
| `submit` | boolean | Press Enter afterwards |

Typing into password and one-time-code fields is always refused (`blocked_by_policy`, rule `credentials`). So is typing without `ref` when no field reports focus and the screen has such a field.

### phone_swipe

| Argument | Type | Meaning |
| --- | --- | --- |
| `direction` | `up` \| `down` \| `left` \| `right` | Where the finger moves; `up` shows the content below |
| `ref` | integer | Swipe inside this element (with `direction`) |
| `from`, `to` | `{x, y}` | Exact points instead of a direction |
| `duration_ms` | 80–1200 | Default 300 |

### phone_key

| Argument | Type | Meaning |
| --- | --- | --- |
| `key` | `back` `home` `recents` `enter` `tab` `delete` `forward_delete` `escape` `dpad_up` `dpad_down` `dpad_left` `dpad_right` `page_up` `page_down` | Keys the phone lacks return `unsupported` |

`home` always works, even in a blocked app, except on an iPhone below iOS 27 without the WebDriverAgent helper (see Limits).

### phone_launch

| Argument | Type | Meaning |
| --- | --- | --- |
| `app` | string, required | Package name, bundle id, or name (exact, then a unique part of it) |

Opening a blocked banking or wallet app, or one the person blocked, asks the person like an Ask action (rule `Finance apps` or `Blocked by you`; up to 45 s, then `approval_pending`). If they allow it, it stays allowed for the rest of the task: the agent may open it and read its screen, and everything else the rules ask about there still asks; Pinky keeps no screenshots, recording or screen text of it, and Settings do not change. If they deny it, later launches of it in the task return `blocked_by_policy` at once. Password managers return `blocked_by_policy` without asking. Apps outside the agent's scope or the task's list return `scope_denied`; unknown apps return `not_found`. A URL (`https://…`) counts as opening a link (rule `Installs and links`), which asks the person when they chose to approve more than payments.

### phone_apps

| Argument | Type | Meaning |
| --- | --- | --- |
| `query` | string | Only apps whose name or id contains it |

Lists installed apps inside the agent's scope, user apps first, one per line: `Name (id)`, with `· system` for system apps. Blocked apps are never listed.

### phone_batch

| Argument | Type | Meaning |
| --- | --- | --- |
| `steps` | array, ≤ 50 | Each step has `tool` (`tap`, `type`, `swipe`, `key`, `launch`, with or without the `phone_` prefix) and that tool's arguments, plus its own `intent` and `approval_id` |
| `intent` | string | For steps without their own |

Steps run in order, each settling first, and the batch stops at the first step that does not settle. The result's first line is the last step's status; then `step N:` blocks follow with each step's status and changes. To finish a batch that stopped at `approval_pending`, repeat that step alone as its own tool call with the `approval_id`.

### phone_wait_for

| Argument | Type | Meaning |
| --- | --- | --- |
| `text` | string, required | Matched without case in element labels and new events. In languages that put spaces between words it matches whole words only (`Stop` does not match `Stopwatch`); Chinese, Japanese, Korean and Thai text matches anywhere in a label |
| `gone` | boolean | Wait for it to disappear |
| `timeout_ms` | 0–45000 | Default 10000 |

Returns `settled` with `matched: true` or `matched: false` and the screen changes.

### phone_files

Copies files between the phone's Pinky folder (`/sdcard/Pinky` on Android, the only place on the phone agents may reach) and a folder on the computer, `<data>/files/<phone id>/`, which the result names. Agents read and write that folder with their own file tools. Needs a `phone_connect` lease like the other tools. iPhones return `unsupported`.

| Argument | Type | Meaning |
| --- | --- | --- |
| `action` | `list` \| `pull` \| `push` | `list` (default) shows both sides; `pull` copies from the phone; `push` copies to the phone |
| `paths` | string[] | Files or folders, relative to the Pinky folder. Default: everything; for `push`, the files that are new or changed on the computer |
| `intent` | string | For `push`: one sentence on why, shown on the approval panel |
| `approval_id` | string | From `approval_pending`: repeat the same call with it |

The result lists the folder, then one line per file: `path · size · modified · where`, where `where` is `both` (same size and time), `phone`, `computer` or `differs`. `pull` and `push` list the files they copied and skip files that are the same on both sides; copied files keep their modification time.

- `push` asks the person first (rule `Files`), like an Ask action: up to 45 s, then `approval_pending`. "Allow for this task" covers later pushes in the task. Read-only mode refuses it (`blocked_by_policy`, rule `Read-only`), and read-only and watch-only agents get `scope_denied`: they may only list and pull.
- Paths that leave the folder (`..`, absolute paths) are `invalid_argument`; files that are not there are `not_found`. Symbolic links in the computer's folder are not copied, and a pull replaces a link in its way instead of writing through it.
- Each call is a step in the task's history ("Copied 2 files from the phone: a.txt, b.png").

### phone_request_approval

| Argument | Type | Meaning |
| --- | --- | --- |
| `summary` | string, required | The question as the person reads it |
| `details` | string | More context |

Returns `settled` with `decision: allow`, or `approval_denied`, with the person's `note`. It grants nothing: later steps the person's rules ask about, such as payments, still ask, and so does your next question. Like action approvals it blocks up to 45 s and holds the phone while open.

### phone_handoff

| Argument | Type | Meaning |
| --- | --- | --- |
| `reason` | string, required | What the person should do |

Shows a Needs you card and waits until the person hands control back; returns `settled` with their `note` and the whole screen. If they already handed back before the call, it returns at once. A handoff opened while the phone waits for a step (`device_needs_setup`) ends by itself once Pinky has opened the phone, with `note: "The phone is ready."`; if the person hands back before that, the result is `device_needs_setup` again, with their note. A handoff opened while the phone is locked ends by itself when the person unlocks from the computer, if unlocking is all its `reason` asks for ("Please unlock the phone"); one that asks for more ("Unlock the phone, then sign in to Wise") stays open until the person hands back, and so does any handoff after an unlock on the phone itself. The person may decline (`approval_denied`); with no answer it ends after 15 minutes. Blocks up to 45 s at a time, then `approval_pending`.

### phone_flow_run

Replays a flow: steps that worked before, run again in one call without a model choosing refs. The format, and how flows come to be, is in [notes/flows.md](notes/flows.md).

| Argument | Type | Meaning |
| --- | --- | --- |
| `flow` | string | The name of a saved flow |
| `parts` | array of strings | Instead of `flow`: the ids of parts to run, in order (see "Parts") |
| `yaml` | string | Instead of `flow`: a flow to run without saving it. A step may name a part: `- part: p815736` |
| `params` | object of strings | Values for the flow's parameters. One without a default is required; one the flow does not have is `invalid_argument` |
| `from_step` | 1–200 | Start at this step: after a mismatch the agent fixed by hand, or with `approval_id` |
| `intent` | string | For steps without their own; default "Flow “name”: what the step does" |
| `approval_id` | string | From `approval_pending`: repeat with `from_step` set to the step that waits |

Each step finds its target by selector on the current screen, waiting up to the step's timeout (10 s unless the flow says otherwise) for it to show; acts through the same pipeline as `phone_tap` and its like; then waits for its checkpoint (`expect`), or for the screen to settle when it has none. Steps are at least 300 ms apart.

```text
settled · Pixel 9
The flow “dinely.to-cart” (version 2) ran: 4 steps in 1.9 s.
step 1: ok · open com.dinely.app · 420 ms
step 2: ok · tap #dinely:tab:orders · 380 ms
step 3: ok · tap #dinely:reorder:1 · 510 ms · found by fallback selector 1 (button "Reorder · Chicken rice bowl")
step 4: skipped · close the tip, if it shows · 1004 ms
<the whole screen at the end, with every toast and dialog the steps met>
```

- The first line is `settled` when every step ran, else the status or error of the step that stopped the replay: `mismatch`, `approval_pending`, `approval_denied`, `blocked_by_policy`, `scope_denied`, `user_took_over`, … The message says at which step and what to do next.
- A step is `ok`, `skipped` (an `optional` step whose target did not show within 1 s), or what stopped the replay.
- A selector that matches several elements without an `index` is a mismatch, never a guess.
- A replay is nothing an agent could not do step by step: lease, scope, the task's app list, read-only mode, rules, approvals, blocked apps, credential fields, takeover and Stop apply to every step, and each step is in the task's history as its own tool (`phone_tap`, …), followed by one `phone_flow_run` step for the replay.
- The result also carries `structuredContent` for scripts: `status`, `message`, `approval_id`, `flow`, `version`, `total`, `elapsed_ms` and `steps[]` with `index`, `text`, `status`, `detail`, `elapsed_ms`.

#### Flows by themselves

Nobody has to save or name a flow for a task to replay the next time:

- **Kept at the end.** `phone_release` with outcome `done` keeps the task's steps as a flow when the agent made steps of its own in the task, at least two steps ran, and none are missing (steps in an app the rules block). The flow is named `<app>.<task>-<hash>` after the app, the task's name and a hash of what the task is known by (`brief`, else the task's name), has the task's name as its description, and takes the text the task typed as parameters `text`, `text2`, … with the typed text as defaults. A flow that is already there for the task stays as it is, unless the task repairs it (below): the same task on two phones at once, or done by hand once more, does not replace steps that work. A task that only replayed a flow leaves it as it is; other outcomes keep nothing.
- **Offered at the start.** A new `phone_connect` lists the saved flows (up to 20) in the format of `phone_flow_list`, and when a flow was kept from the same request (`brief`, else the task's name) the message says so and names it. Matching a task to another flow is the agent's part.
- **Repaired by use.** A task that replayed one saved flow, stopped at a mismatch, went on by hand and ended with `done` keeps its steps as that flow's next version, with its name and description.

This is on in the app and the daemon (`gateway.Options.AutoFlows`).

#### Parts

A kept flow is a whole task. What different tasks share is kept per app as parts: single steps that worked, each with the screen it led to, which an agent puts together into a task nobody ran before ([notes/flows.md](notes/flows.md#parts)).

- **Kept when a task ends**, whatever the outcome: each tap or swipe on an element, key other than home, and launch that ran and has a checkpoint. The same action leading to the same screen is one part, counted each time it is seen; leading elsewhere, it is another. Parts are files, `<data>/flows/parts/<app id>.yaml`, 300 per app at most.
- **Listed** in the result of a new `phone_connect`, for the apps in `apps` or else the two apps seen last, unless a flow was kept from the same request; and in the result of `phone_launch` for the app it opened, once per task. Up to 40 per app, most seen first:

  ```text
  parts of com.dinely.app:
  p39deba · open com.dinely.app → app com.dinely.app
  p815736 · tap tab "Orders" #dinely:tab:orders → #dinely:last shows · after p39deba
  ```

  `after` names parts this one was recorded right after: a hint at where its target shows, not a condition.
- **Run** with `phone_flow_run { parts: ["p39deba", "p815736"] }`. The parts run in that order like the steps of a flow, `from_step` counts in the list, and the message reads "The 2 parts ran: …". An id no part has is `invalid_argument`.
- A part's action that ran and did not lead to its checkpoint (`mismatch` after the step ran) counts against the part: twice in a row, and the part is dropped. A part whose target is not on the screen stays.
- A task put together from parts and ended with `done` is kept as a flow, like a task done by hand.

### phone_flow_save

Saves a flow under its name, as the next version of a flow that has it, and returns it as YAML.

| Argument | Type | Meaning |
| --- | --- | --- |
| `name` | string | Lower-case letters, digits, dots, dashes and underscores, up to 64 characters: `settings.open-display` |
| `description` | string | One line on what the flow does, for `phone_flow_list` |
| `params` | object of strings | Turns text the task typed into parameters: name to the text as typed. The text becomes the parameter's default |
| `yaml` | string | Saves this flow as it is (its `flow:` is the name) instead of the task's steps |

Without `yaml`, the flow is made of the actions the task ran so far that settled, including those a replay ran: each target becomes a selector written on the screen it was found on (its id when the screen has it once, with role and label as the fallback; else role and label, with an index among equals; a point for taps by `x`, `y`), and each step except typing gets a checkpoint: the app in front and up to two elements that the screen before did not have, by id or by a label without digits. Steps on the screens of an app that the rules block are left out even when the person allowed the app for the task, and the result says so. Saving asks nobody: a flow grants nothing, since its steps ask when replayed.

### phone_flow_list

| Argument | Type | Meaning |
| --- | --- | --- |
| `name` | string | Return this flow's YAML instead of the list |
| `app` | string | List the parts kept for this app instead, by its id or a part of it |

Lists the saved flows, one per line: `name · version N · N steps · app · params: a (default "x"), b (required) · description`. Flows are files, `<data>/flows/<name>.yaml` (`<data>/fake/flows` with fake phones), which people may edit and keep under version control; a file that is not a valid flow is named as not readable.

All three need a `phone_connect` lease, so an agent the person has not approved neither reads nor writes flows.

## Flows from the command line

`pinkyctl flow` replays flows without an agent, for scripts and CI. It connects to the running Pinky like an agent named "Pinky Flow" (first connection approval, scope, rules and approvals as for any agent), starting the gateway when none runs.

```sh
pinkyctl flow run [--device phone] [--param name=value]… [--junit report.xml] [--task title] <name or file.yaml>…
pinkyctl flow list
pinkyctl flow show <name>
pinkyctl flow check <file.yaml>…
pinkyctl flow parts [app]
```

- `run` replays the flows in order in one task, prints a line per step, and exits with 1 at the first flow that does not run to its end; later flows are not started. A step that waits for the person is repeated until they answer (`--approval-wait`, 10 minutes in all by default).
- `--junit` writes JUnit XML: one suite per flow, one case per step, a failure on the step that stopped the replay, and the steps after it as skipped.
- `check` validates files without a phone. `list` and `show` read the flows folder; `--fake` is the fake phones' folder.
- `parts` lists the apps Pinky keeps parts of, or one app's parts with how often and when each was last seen.

## Approvals

- Which actions ask is the person's choice in Settings › Safety: payments only (the default); payments, messages, deletions and other commits, calls, installs, links that leave the app and settings changes; or every action. Their own rules may ask about more, and blocked apps and credential fields are refused whatever they chose.
- An Ask action opens an approval panel in the client and the call blocks for up to 45 s, sending progress notifications if the request carried a `progressToken` (at the start, then every 5 s, with the time left to answer).
- The person has 60 s from when the panel opens (configurable with `pinky daemon --approval-timeout`). No answer means Deny (`note: "No answer in 60 s."`).
- After 45 s the call returns `approval_pending` with the id. The panel stays open. A decision made while no call waits is kept for the next call with that id, for as long as the timeout again; later the id is gone and a call with it asks anew.
- The id is single-use and bound to the session, device, lease, tool and arguments (`intent` excluded). Another call with it returns `invalid_argument`.
- While an approval is open on a phone, other actions on it return `approval_pending` with that approval's id; reads still work.
- Before acting on Allow, Pinky reads the screen again and compares the target's role, label, states and position (a few pixels of tolerance) and the screen's text and states (status bar excluded; OCR text only when the screen has no element text). Any change returns `stale`.
- "Allow for this task" covers the rules that asked, in that app, for the rest of the task. An action that another rule also asks about still asks. `phone_request_approval` has no such answer: each question asks the person.
- "Always allow" keeps the answer in the rules file: in that app, the rule that asked, and the rules that asked with it, no longer ask about the steps that rule asks about, in later tasks too, until the person takes it back in Settings › Safety. The approved call's message says `from now on "Payments" no longer asks in Dinely`; a later such action returns `settled` at once, its message starting `No approval needed: the person always allows "Payments" in Dinely.` It never lifts a deny, read-only mode, the agent's scope or the task's `apps`, and it does not apply while every action asks. The panel offers it for actions only: not for `phone_request_approval`, `phone_files` or `phone_launch`.
- An MCP cancellation of the blocked call, a closed session, Stop and Take over all withdraw the panel without acting.
- Only the client's `Decide` answers approvals. No tool can.

## Progress notifications

Sent only when the request carries `_meta.progressToken`: while waiting for an approval or handoff (at the start and every 5 s), for `phone_observe` with `wait_ms`, and for `phone_wait_for` (every 5 s). `progress` and `total` are seconds.

## Limits

- iPhones below iOS 27 without the WebDriverAgent helper (iOS 17.4 to 26, nothing installed) are read through the accessibility inspector and OCR and can launch apps, nothing more: `phone_tap`, `phone_swipe`, `phone_type` and `phone_key` return `unsupported`, with a sentence naming the helper. With the helper, everything works; reading the screen takes 0.5 to 3 s, and about 7 s on a home screen.

| Limit | Value |
| --- | --- |
| Longest block of one call | 45 s |
| Approval timeout | 60 s |
| Handoff timeout | 15 min |
| Settle limit | 3 s |
| Idle lease | 10 min |
| Reconnect grace | 60 s |
| `text` | 16384 characters |
| `phone_batch` steps | 50 |
| Flow steps | 200 |
| Parts kept per app | 300 |
| Parts listed per app in a result | 40 |
| Flow step timeout | 10 s by default, 45 s at most |
| Least time between a replay's actions | 300 ms |
