// The Agent panel's state, for the whole window: its chats live here, not
// in a page, so a run goes on while the person looks at another phone or
// at Settings. A message to several phones is one run per phone, side by
// side, in one stream (see deskapi's chat).

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { api, streamChat, type ChatStep } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useI18n } from '@/i18n';
import {
  emptySession,
  historyOf,
  isBusy,
  load,
  newID,
  save,
  titleFrom,
  type ChatSession,
  type Mode,
  type PhoneRef,
  type Run,
  type Turn,
} from './model';

export interface Chat {
  sessions: ChatSession[];
  active: ChatSession;
  busy: boolean;
  /** Sends to the phones given, or queues while the chat runs. */
  send: (text: string, phones: PhoneRef[], mode: Mode) => void;
  stop: () => void;
  steer: (i: number) => void;
  removeQueued: (i: number) => void;
  newChat: () => void;
  select: (id: string) => void;
  close: (id: string) => void;
  setPhones: (phones: PhoneRef[], noCurrent?: boolean) => void;
  setMode: (mode: Mode) => void;
}

export interface Panel {
  open: boolean;
  width: number;
  setOpen: (v: boolean) => void;
  toggle: () => void;
  setWidth: (w: number) => void;
  /** Focuses the composer, opening the panel. */
  focus: () => void;
  /** The composer registers its input here. */
  inputRef: React.MutableRefObject<HTMLTextAreaElement | null>;
}

const ChatCtx = createContext<{ chat: Chat; panel: Panel } | null>(null);

const panelKey = 'pinky-agent-panel';
export const panelMin = 320;
export const panelMax = 640;

function loadPanel(): { open: boolean; width: number } {
  try {
    const v = JSON.parse(localStorage.getItem(panelKey) ?? '');
    return { open: v.open !== false, width: Math.min(panelMax, Math.max(panelMin, Number(v.width) || 400)) };
  } catch {
    return { open: true, width: 400 };
  }
}

export function ChatProvider({ children }: { children: ReactNode }) {
  const { boot } = useApp();
  const { locale } = useI18n();
  const agent = !!boot?.inAppAgent;

  // Chats.
  const [sessions, setSessions] = useState<ChatSession[]>(() => {
    const l = load();
    return l.length ? l : [emptySession()];
  });
  const [activeID, setActiveID] = useState<string>(() => sessions.find((s) => s.open)?.id ?? sessions[0].id);
  const ref = useRef(sessions);
  // One stream per chat at a time; several chats may run at once.
  const runs = useRef(new Map<string, { id: string; abort: AbortController }>());
  const interrupt = useRef(new Map<string, { text: string; phones: PhoneRef[]; mode: Mode }>());

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

  const startRef = useRef<(session: string, text: string, phones: PhoneRef[], mode: Mode) => void>(() => undefined);

  const afterRun = useCallback(
    (session: string) => {
      const next = interrupt.current.get(session);
      if (next) {
        interrupt.current.delete(session);
        startRef.current(session, next.text, next.phones, next.mode);
        return;
      }
      const s = ref.current.find((x) => x.id === session);
      if (s && s.queued.length) {
        const [head, ...rest] = s.queued;
        edit(session, (x) => ({ ...x, queued: rest }));
        startRef.current(session, head.text, head.phones, head.mode);
      }
    },
    [edit],
  );

  startRef.current = (session, text, phones, mode) => {
    const s = ref.current.find((x) => x.id === session);
    if (!s) return;
    const drive = mode === 'agent' && agent && phones.length > 0;
    const now = Date.now();
    const turnID = newID('t');
    const turn: Turn = {
      id: turnID,
      prompt: text,
      at: now,
      mode,
      runs: drive ? phones.map((p) => ({ phone: p, items: [], state: 'running', started: now })) : [{ items: [], state: 'running', started: now }],
    };
    const title = s.title || titleFrom(text);
    const history = historyOf(s, drive && phones.length === 1 ? phones[0].id : undefined);
    edit(session, (x) => ({ ...x, title, turns: [...x.turns, turn] }));

    // Edits one run of this turn: the phone's, or the only one.
    const editRun = (phone: string | undefined, fn: (r: Run) => Run) =>
      edit(session, (x) => ({
        ...x,
        turns: x.turns.map((t) =>
          t.id !== turnID
            ? t
            : { ...t, runs: t.runs.map((r, i) => ((phone ? r.phone?.id === phone : i === 0) || (t.runs.length === 1) ? fn(r) : r)) },
        ),
      }));
    const endAll = (fn: (r: Run) => Run) =>
      edit(session, (x) => ({
        ...x,
        turns: x.turns.map((t) => (t.id !== turnID ? t : { ...t, runs: t.runs.map((r) => (r.state === 'running' ? fn(r) : r)) })),
      }));
    const settle = (r: Run): Run => ({
      ...r,
      ended: Date.now(),
      items: r.items.map((i) => (i.kind === 'tool' && i.step.running ? { ...i, step: { ...i.step, running: false } } : i)),
    });

    const onStep = (st: ChatStep) =>
      editRun(st.deviceId, (r) => {
        if (st.kind === 'text') return { ...r, items: [...r.items, { kind: 'text', text: st.text }] };
        if (st.kind === 'tool_start') {
          return {
            ...r,
            items: [...r.items, { kind: 'tool', step: { id: st.id ?? newID('s'), tool: st.tool ?? '', intent: st.intent, args: st.args, running: true } }],
          };
        }
        return {
          ...r,
          items: r.items.map((i) =>
            i.kind === 'tool' && i.step.id === st.id ? { ...i, step: { ...i.step, running: false, status: st.status, detail: st.detail } } : i,
          ),
        };
      });

    const abort = new AbortController();
    const runID = newID('r');
    runs.current.set(session, { id: runID, abort });
    void streamChat(
      {
        runId: runID,
        history,
        prompt: text,
        title,
        devices: drive ? phones : undefined,
        phones: phones.map((p) => p.name),
        locale,
      },
      {
        onStep,
        onRunEnd: (device, reply, code, message) =>
          editRun(device, (r) =>
            settle(
              code
                ? { ...r, state: code === 'cancelled' ? 'stopped' : 'error', code, error: message }
                : { ...r, state: 'done', reply: reply || undefined },
            ),
          ),
        onDone: (reply) => endAll((r) => settle({ ...r, state: 'done', reply: reply || r.reply })),
        onError: (code, message) => endAll((r) => settle({ ...r, state: code === 'cancelled' ? 'stopped' : 'error', code, error: message })),
      },
      abort.signal,
    ).finally(() => {
      if (runs.current.get(session)?.id === runID) runs.current.delete(session);
      afterRun(session);
    });
  };

  const stopSession = useCallback((session: string) => {
    const r = runs.current.get(session);
    if (!r) return;
    r.abort.abort();
    void api.cancelChat(r.id).catch(() => undefined);
  }, []);

  const stop = useCallback(() => stopSession(activeID), [activeID, stopSession]);

  const send = useCallback(
    (raw: string, phones: PhoneRef[], mode: Mode) => {
      const text = raw.trim();
      if (!text) return;
      if (runs.current.has(activeID)) {
        edit(activeID, (x) => ({ ...x, queued: [...x.queued, { text, phones, mode }] }));
        return;
      }
      startRef.current(activeID, text, phones, mode);
    },
    [activeID, edit],
  );

  const steer = useCallback(
    (i: number) => {
      const s = ref.current.find((x) => x.id === activeID);
      if (!s || i < 0 || i >= s.queued.length) return;
      const q = s.queued[i];
      edit(activeID, (x) => ({ ...x, queued: x.queued.filter((_, j) => j !== i) }));
      if (runs.current.has(activeID)) {
        interrupt.current.set(activeID, q);
        stopSession(activeID);
      } else startRef.current(activeID, q.text, q.phones, q.mode);
    },
    [activeID, edit, stopSession],
  );

  const removeQueued = useCallback(
    (i: number) => edit(activeID, (x) => ({ ...x, queued: x.queued.filter((_, j) => j !== i) })),
    [activeID, edit],
  );

  const newChat = useCallback(() => {
    const cur = ref.current.find((x) => x.id === activeID);
    if (cur && cur.turns.length === 0) return; // an empty chat is already new
    const s = { ...emptySession(), mode: cur?.mode ?? 'agent' };
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
      stopSession(id);
      const rest = ref.current.filter((s) => s.id !== id && s.open);
      mutate((l) => l.filter((s) => s.turns.length > 0 || s.id !== id).map((s) => (s.id === id ? { ...s, open: false } : s)));
      if (id === activeID) {
        if (rest.length) setActiveID(rest[0].id);
        else {
          const s = emptySession();
          mutate((l) => [s, ...l]);
          setActiveID(s.id);
        }
      }
    },
    [activeID, mutate, stopSession],
  );

  const setPhones = useCallback(
    (phones: PhoneRef[], noCurrent?: boolean) => edit(activeID, (x) => ({ ...x, phones, noCurrent: noCurrent ?? x.noCurrent })),
    [activeID, edit],
  );
  const setMode = useCallback((mode: Mode) => edit(activeID, (x) => ({ ...x, mode })), [activeID, edit]);

  // Stop every run when the window goes.
  useEffect(() => {
    const all = runs.current;
    return () => all.forEach((r) => r.abort.abort());
  }, []);

  const active = sessions.find((s) => s.id === activeID) ?? sessions[0];
  const chat: Chat = {
    sessions,
    active,
    busy: isBusy(active),
    send,
    stop,
    steer,
    removeQueued,
    newChat,
    select,
    close,
    setPhones,
    setMode,
  };

  // The panel.
  const [panelState, setPanelState] = useState(loadPanel);
  const inputRef = useRef<HTMLTextAreaElement | null>(null);
  const setPanel = useCallback((fn: (p: { open: boolean; width: number }) => { open: boolean; width: number }) => {
    setPanelState((prev) => {
      const next = fn(prev);
      try {
        localStorage.setItem(panelKey, JSON.stringify(next));
      } catch {
        // Remembered until the window closes.
      }
      return next;
    });
  }, []);
  const panel: Panel = useMemo(
    () => ({
      open: panelState.open,
      width: panelState.width,
      setOpen: (open) => setPanel((p) => ({ ...p, open })),
      toggle: () => setPanel((p) => ({ ...p, open: !p.open })),
      setWidth: (w) => setPanel((p) => ({ ...p, width: Math.min(panelMax, Math.max(panelMin, Math.round(w))) })),
      focus: () => {
        setPanel((p) => ({ ...p, open: true }));
        requestAnimationFrame(() => inputRef.current?.focus());
      },
      inputRef,
    }),
    [panelState, setPanel],
  );

  return <ChatCtx.Provider value={{ chat, panel }}>{children}</ChatCtx.Provider>;
}

export function useChat(): Chat {
  const c = useContext(ChatCtx);
  if (!c) throw new Error('useChat outside ChatProvider');
  return c.chat;
}

export function usePanel(): Panel {
  const c = useContext(ChatCtx);
  if (!c) throw new Error('usePanel outside ChatProvider');
  return c.panel;
}
