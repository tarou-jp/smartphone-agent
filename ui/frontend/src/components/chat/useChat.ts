// The in-app chat's sessions and runs, after ui/chat.go: one run at a
// time; Enter while a run goes queues the message; Steer stops the run and
// sends a queued message next; a run's progress lines go into one tail.
// Sessions are kept in this browser's storage, a convenience per viewer.

import { useCallback, useEffect, useRef, useState } from 'react';
import { api, streamChat } from '@/api/client';

export type ChatRole = 'user' | 'assistant' | 'error' | 'progress';

export interface ChatMsg {
  role: ChatRole;
  text: string;
  at: number;
  /** Progress: the agent's lines; error: no_key links to Settings › Model. */
  lines?: string[];
  streaming?: boolean;
  code?: string;
}

export interface ChatSession {
  id: string;
  title: string;
  msgs: ChatMsg[];
  queued: string[];
  updated: number;
  /** Shown as a tab; closed sessions stay in Past chats. */
  open: boolean;
}

export interface ChatScope {
  deviceId?: string;
  deviceName?: string;
  phones: string[];
  locale: string;
}

const storeKey = 'pinky-chat-sessions';
const keep = 40;

function load(): ChatSession[] {
  try {
    const raw = localStorage.getItem(storeKey);
    if (!raw) return [];
    const list = JSON.parse(raw) as ChatSession[];
    if (!Array.isArray(list)) return [];
    // A run never survives a reload: drop tails that were still going.
    return list.map((s) => ({
      ...s,
      queued: s.queued ?? [],
      msgs: (s.msgs ?? []).map((m) => (m.streaming ? { ...m, streaming: false } : m)),
    }));
  } catch {
    return [];
  }
}

function save(list: ChatSession[]) {
  try {
    localStorage.setItem(storeKey, JSON.stringify(list.slice(0, keep)));
  } catch {
    // Storage may be full or off; the chat keeps working.
  }
}

let seq = 0;
const newID = () => `c${Date.now().toString(36)}${(++seq).toString(36)}`;

/** A title from the first prompt: its first 27 characters and an ellipsis. */
export function titleFrom(prompt: string): string {
  const p = prompt.split(/\s+/).filter(Boolean).join(' ');
  const r = Array.from(p);
  if (r.length > 28) return r.slice(0, 27).join('') + '…';
  return p || 'Chat';
}

function emptySession(): ChatSession {
  return { id: newID(), title: '', msgs: [], queued: [], updated: Date.now(), open: true };
}

export interface Chat {
  sessions: ChatSession[];
  active: ChatSession;
  busy: boolean;
  send: (text: string) => void;
  stop: () => void;
  steer: (i: number) => void;
  removeQueued: (i: number) => void;
  newChat: () => void;
  select: (id: string) => void;
  close: (id: string) => void;
}

export function useChat(scope: ChatScope): Chat {
  const [sessions, setSessions] = useState<ChatSession[]>(() => {
    const l = load();
    return l.length ? l : [emptySession()];
  });
  const [activeID, setActiveID] = useState<string>(() => sessions.find((s) => s.open)?.id ?? sessions[0].id);
  const [busy, setBusy] = useState(false);

  const ref = useRef(sessions);
  const scopeRef = useRef(scope);
  scopeRef.current = scope;
  const run = useRef<{ id: string; abort: AbortController; session: string } | null>(null);
  const interrupt = useRef<{ session: string; text: string } | null>(null);

  const mutate = useCallback((fn: (l: ChatSession[]) => ChatSession[]) => {
    const next = fn(ref.current);
    ref.current = next;
    setSessions(next);
    save(next);
  }, []);

  const edit = useCallback(
    (id: string, fn: (s: ChatSession) => ChatSession) =>
      mutate((l) => l.map((s) => (s.id === id ? { ...fn(s), updated: Date.now() } : s))),
    [mutate],
  );

  // start is defined through a ref so that a run's end can start the next.
  const startRef = useRef<(session: string, text: string) => void>(() => undefined);

  const afterRun = useCallback(
    (session: string) => {
      const next = interrupt.current;
      if (next) {
        interrupt.current = null;
        startRef.current(next.session, next.text);
        return;
      }
      const s = ref.current.find((x) => x.id === session);
      if (s && s.queued.length) {
        const [head, ...rest] = s.queued;
        edit(session, (x) => ({ ...x, queued: rest }));
        startRef.current(session, head);
      }
    },
    [edit],
  );

  startRef.current = (session: string, text: string) => {
    const s = ref.current.find((x) => x.id === session);
    if (!s) return;
    const history = s.msgs
      .filter((m) => m.role === 'user' || m.role === 'assistant')
      .map((m) => ({ role: m.role, text: m.text }));
    const title = s.title || titleFrom(text);
    const now = Date.now();
    edit(session, (x) => ({
      ...x,
      title,
      msgs: [...x.msgs, { role: 'user', text, at: now }, { role: 'progress', text: '', lines: [], streaming: true, at: now }],
    }));
    const abort = new AbortController();
    const runID = newID();
    run.current = { id: runID, abort, session };
    setBusy(true);
    const sc = scopeRef.current;

    // Edits the tail, the last progress message of the session.
    const tail = (fn: (m: ChatMsg) => ChatMsg | null) =>
      edit(session, (x) => {
        const i = x.msgs.map((m) => m.role).lastIndexOf('progress');
        if (i < 0) return x;
        const m = fn(x.msgs[i]);
        const msgs = [...x.msgs];
        if (m) msgs[i] = m;
        else msgs.splice(i, 1);
        return { ...x, msgs };
      });
    // Ends the tail: kept, folded, when it says what the agent did.
    const endTail = () => tail((m) => (m.lines && m.lines.length ? { ...m, streaming: false } : null));
    const append = (m: ChatMsg) => edit(session, (x) => ({ ...x, msgs: [...x.msgs, m] }));

    void streamChat(
      {
        runId: runID,
        history,
        prompt: text,
        title,
        deviceId: sc.deviceId,
        deviceName: sc.deviceName,
        phones: sc.phones,
        locale: sc.locale,
      },
      {
        onProgress: (line) => tail((m) => ({ ...m, lines: [...(m.lines ?? []), line] })),
        onDone: (reply) => {
          endTail();
          append({ role: 'assistant', text: reply, at: Date.now() });
        },
        onError: (code, message) => {
          endTail();
          if (code !== 'cancelled') append({ role: 'error', text: message, code, at: Date.now() });
        },
      },
      abort.signal,
    ).finally(() => {
      if (run.current?.id === runID) run.current = null;
      setBusy(run.current !== null);
      afterRun(session);
    });
  };

  const stop = useCallback(() => {
    const r = run.current;
    if (!r) return;
    r.abort.abort();
    void api.cancelChat(r.id).catch(() => undefined);
  }, []);

  const send = useCallback(
    (raw: string) => {
      const text = raw.trim();
      if (!text) return;
      if (run.current) {
        edit(activeID, (x) => ({ ...x, queued: [...x.queued, text] }));
        return;
      }
      startRef.current(activeID, text);
    },
    [activeID, edit],
  );

  const steer = useCallback(
    (i: number) => {
      const s = ref.current.find((x) => x.id === activeID);
      if (!s || i < 0 || i >= s.queued.length) return;
      const text = s.queued[i];
      edit(activeID, (x) => ({ ...x, queued: x.queued.filter((_, j) => j !== i) }));
      if (run.current) {
        interrupt.current = { session: activeID, text };
        stop();
      } else {
        startRef.current(activeID, text);
      }
    },
    [activeID, edit, stop],
  );

  const removeQueued = useCallback(
    (i: number) => edit(activeID, (x) => ({ ...x, queued: x.queued.filter((_, j) => j !== i) })),
    [activeID, edit],
  );

  const newChat = useCallback(() => {
    const cur = ref.current.find((x) => x.id === activeID);
    // An empty chat is already new.
    if (cur && cur.msgs.length === 0) return;
    const s = emptySession();
    mutate((l) => [s, ...l]);
    setActiveID(s.id);
  }, [activeID, mutate]);

  const select = useCallback(
    (id: string) => {
      mutate((l) => l.map((s) => (s.id === id ? { ...s, open: true } : s)));
      setActiveID(id);
    },
    [mutate],
  );

  const close = useCallback(
    (id: string) => {
      if (run.current?.session === id) stop();
      const rest = ref.current.filter((s) => s.id !== id && s.open);
      mutate((l) => l.filter((s) => s.msgs.length > 0 || s.id !== id).map((s) => (s.id === id ? { ...s, open: false } : s)));
      if (id === activeID) {
        if (rest.length) setActiveID(rest[0].id);
        else {
          const s = emptySession();
          mutate((l) => [s, ...l]);
          setActiveID(s.id);
        }
      }
    },
    [activeID, mutate, stop],
  );

  // Stop a run when the column goes away.
  useEffect(() => () => run.current?.abort.abort(), []);

  const active = sessions.find((s) => s.id === activeID) ?? sessions[0];
  return { sessions, active, busy, send, stop, steer, removeQueued, newChat, select, close };
}
