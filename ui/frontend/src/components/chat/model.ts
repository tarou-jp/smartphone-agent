// The Agent panel's data: chats, their turns, and each turn's runs, one a
// phone (or one that only talks). Kept in this browser's storage, a
// convenience per viewer, as the MyGo client kept its own.

export type Mode = 'agent' | 'ask';

export interface PhoneRef {
  id: string;
  name: string;
}

export interface ToolStep {
  id: string;
  tool: string;
  intent?: string;
  args?: Record<string, unknown>;
  /** The result's status ("settled", "approval_denied", "error: …"); unset while it runs. */
  status?: string;
  detail?: string;
  running: boolean;
  /** What a running call waits for, such as the person's approval. */
  progress?: string;
  /** The phone, when one agent works on several. */
  device?: string;
}

export type RunItem =
  | { kind: 'tool'; step: ToolStep }
  | { kind: 'text'; text: string; streaming?: boolean }
  | { kind: 'note'; text: string };

export interface PlanStep {
  step: string;
  status: 'pending' | 'in_progress' | 'completed';
}

export type RunState = 'running' | 'done' | 'error' | 'stopped';

export interface Run {
  /** Unset when the run only talks. */
  phone?: PhoneRef;
  items: RunItem[];
  state: RunState;
  reply?: string;
  code?: string;
  error?: string;
  started: number;
  ended?: number;
  /** The agent's plan, as update_plan last left it. */
  plan?: PlanStep[];
  /** A model call failed and is tried again: what the person sees meanwhile. */
  retry?: string;
  usage?: { input: number; output: number; cacheRead: number; cacheWrite: number };
}

export interface Turn {
  id: string;
  prompt: string;
  at: number;
  mode: Mode;
  runs: Run[];
  /** The phones the message went to. */
  phones?: PhoneRef[];
  /** One agent worked on all of them. */
  together?: boolean;
}

export interface Queued {
  text: string;
  phones: PhoneRef[];
  mode: Mode;
  together?: boolean;
}

export interface ChatSession {
  id: string;
  title: string;
  turns: Turn[];
  queued: Queued[];
  updated: number;
  /** Shown as a tab; closed chats stay in Past chats. */
  open: boolean;
  /** The phones the next message goes to, as the composer's pills show them. */
  phones: PhoneRef[];
  /** Whether the person removed the phone on screen from the pills, so it is not offered again. */
  noCurrent?: boolean;
  mode: Mode;
  /** Several phones go to one agent that works on them together. */
  together?: boolean;
}

const storeKey = 'pinky-chat-v2';
const oldKey = 'pinky-chat-sessions';
const keep = 40;

let seq = 0;
export const newID = (p = 'c') => `${p}${Date.now().toString(36)}${(++seq).toString(36)}`;

export function emptySession(): ChatSession {
  return { id: newID(), title: '', turns: [], queued: [], updated: Date.now(), open: true, phones: [], mode: 'agent' };
}

/** A title from the first prompt: its first 27 characters and an ellipsis. */
export function titleFrom(prompt: string): string {
  const p = prompt.split(/\s+/).filter(Boolean).join(' ');
  const r = Array.from(p);
  if (r.length > 28) return r.slice(0, 27).join('') + '…';
  return p || 'Chat';
}

export const isBusy = (s: ChatSession) => s.turns.some((t) => t.runs.some((r) => r.state === 'running'));

// The first web client kept a flat list of messages; each user message
// starts a turn, and what followed it becomes the turn's one run.
interface OldMsg {
  role: 'user' | 'assistant' | 'error' | 'progress';
  text: string;
  at: number;
  lines?: string[];
  code?: string;
}
interface OldSession {
  id: string;
  title: string;
  msgs?: OldMsg[];
  updated: number;
  open: boolean;
}

function migrate(old: OldSession[]): ChatSession[] {
  return old.map((o) => {
    const turns: Turn[] = [];
    for (const m of o.msgs ?? []) {
      if (m.role === 'user') {
        turns.push({ id: newID('t'), prompt: m.text, at: m.at, mode: 'agent', runs: [{ items: [], state: 'done', started: m.at }] });
        continue;
      }
      const run = turns[turns.length - 1]?.runs[0];
      if (!run) continue;
      if (m.role === 'progress') {
        for (const l of m.lines ?? []) run.items.push({ kind: 'text', text: l });
      } else if (m.role === 'assistant') run.reply = m.text;
      else if (m.role === 'error') Object.assign(run, { state: 'error', error: m.text, code: m.code });
    }
    return { ...emptySession(), id: o.id, title: o.title, turns, updated: o.updated, open: o.open };
  });
}

export function load(): ChatSession[] {
  try {
    const raw = localStorage.getItem(storeKey);
    let list: ChatSession[];
    if (raw) list = JSON.parse(raw) as ChatSession[];
    else {
      const old = localStorage.getItem(oldKey);
      list = old ? migrate(JSON.parse(old) as OldSession[]) : [];
    }
    if (!Array.isArray(list)) return [];
    // A run never survives a reload.
    return list.map((s) => ({
      ...emptySession(),
      ...s,
      queued: s.queued ?? [],
      phones: s.phones ?? [],
      turns: (s.turns ?? []).map((t) => ({
        ...t,
        runs: t.runs.map((r) =>
          r.state === 'running'
            ? {
                ...r,
                state: 'stopped' as const,
                items: r.items.map((i) =>
                  i.kind === 'tool' && i.step.running
                    ? { ...i, step: { ...i.step, running: false, progress: undefined } }
                    : i.kind === 'text' && i.streaming
                      ? { ...i, streaming: false }
                      : i,
                ),
              }
            : r,
        ),
      })),
    }));
  } catch {
    return [];
  }
}

export function save(list: ChatSession[]) {
  try {
    localStorage.setItem(storeKey, JSON.stringify(list.slice(0, keep)));
  } catch {
    // Storage may be full or off; the chat keeps working.
  }
}

/** The conversation so far, as the server's agent reads it: each turn's prompt and what came back. */
export function historyOf(s: ChatSession, phone?: string): { role: string; text: string }[] {
  const out: { role: string; text: string }[] = [];
  for (const t of s.turns) {
    out.push({ role: 'user', text: t.prompt });
    const mine = phone ? t.runs.filter((r) => r.phone?.id === phone) : [];
    const runs = mine.length ? mine : t.runs;
    const replies = runs
      .filter((r) => r.reply)
      .map((r) => (runs.length > 1 && r.phone ? `[${r.phone.name}] ${r.reply}` : (r.reply as string)));
    if (replies.length) out.push({ role: 'assistant', text: replies.join('\n\n') });
  }
  return out;
}
