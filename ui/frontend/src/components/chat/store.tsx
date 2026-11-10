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
  type RunItem,
  type ToolStep,
  type Turn,
} from './model';

export interface Chat {
  sessions: ChatSession[];
  active: ChatSession;
  busy: boolean;
  /** Sends to the phones given, or queues while the chat runs. */
  send: (text: string, phones: PhoneRef[], mode: Mode, together?: boolean) => void;
  stop: () => void;
  steer: (i: number) => void;
  removeQueued: (i: number) => void;
  newChat: () => void;
  select: (id: string) => void;
  close: (id: string) => void;
  setPhones: (phones: PhoneRef[], noCurrent?: boolean) => void;
  setMode: (mode: Mode) => void;
  setTogether: (together: boolean) => void;
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
  const interrupt = useRef(new Map<string, { text: string; phones: PhoneRef[]; mode: Mode; together?: boolean }>());

  // Streaming edits many times a second: storage is written at most twice
  // a second, and when the window goes.
  const saveTimer = useRef<number | null>(null);
  const mutate = useCallback((fn: (l: ChatSession[]) => ChatSession[]) => {
    const next = fn(ref.current);
    ref.current = next;
    setSessions(next);
    if (saveTimer.current === null) {
      saveTimer.current = window.setTimeout(() => {
        saveTimer.current = null;
        save(ref.current);
      }, 500);
    }
  }, []);
  useEffect(() => {
    const flush = () => save(ref.current);
    window.addEventListener('beforeunload', flush);
    return () => window.removeEventListener('beforeunload', flush);
  }, []);
  const edit = useCallback(
    (id: string, fn: (s: ChatSession) => ChatSession) =>
      mutate((l) => l.map((s) => (s.id === id ? { ...fn(s), updated: Date.now() } : s))),
    [mutate],
  );

  const startRef = useRef<(session: string, text: string, phones: PhoneRef[], mode: Mode, together?: boolean) => void>(() => undefined);

  const afterRun = useCallback(
    (session: string) => {
      const next = interrupt.current.get(session);
      if (next) {
        interrupt.current.delete(session);
        startRef.current(session, next.text, next.phones, next.mode, next.together);
        return;
      }
      const s = ref.current.find((x) => x.id === session);
      if (s && s.queued.length) {
        const [head, ...rest] = s.queued;
        edit(session, (x) => ({ ...x, queued: rest }));
        startRef.current(session, head.text, head.phones, head.mode, head.together);
      }
    },
    [edit],
  );

  startRef.current = (session, text, phones, mode, together) => {
    const s = ref.current.find((x) => x.id === session);
    if (!s) return;
    const drive = mode === 'agent' && agent && phones.length > 0;
    // One agent on several phones is one run; side by side, a run each.
    const one = drive && !!together && phones.length > 1;
    const now = Date.now();
    const turnID = newID('t');
    const turn: Turn = {
      id: turnID,
      prompt: text,
      at: now,
      mode,
      phones: drive ? phones : undefined,
      together: one || undefined,
      runs:
        drive && !one
          ? phones.map((p) => ({ phone: p, items: [], state: 'running', started: now }))
          : [{ items: [], state: 'running', started: now }],
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
    // A streamed text ends when a tool call starts or the run ends.
    const closeText = (items: RunItem[]): RunItem[] =>
      items.map((i) => (i.kind === 'text' && i.streaming ? { ...i, streaming: false } : i));
    const settle = (r: Run): Run => ({
      ...r,
      ended: Date.now(),
      retry: undefined,
      items: closeText(r.items).map((i) =>
        i.kind === 'tool' && i.step.running ? { ...i, step: { ...i.step, running: false, progress: undefined } } : i,
      ),
    });
    const setTool = (r: Run, id: string | undefined, fn: (t: ToolStep) => ToolStep): Run => ({
      ...r,
      items: r.items.map((i) => (i.kind === 'tool' && i.step.id === id ? { ...i, step: fn(i.step) } : i)),
    });

    const onStep = (st: ChatStep) =>
      editRun(one ? undefined : st.deviceId, (r) => {
        switch (st.kind) {
          case 'text_delta': {
            const last = r.items[r.items.length - 1];
            if (last?.kind === 'text' && last.streaming) {
              return { ...r, retry: undefined, items: [...r.items.slice(0, -1), { ...last, text: last.text + (st.delta ?? '') }] };
            }
            return { ...r, retry: undefined, items: [...r.items, { kind: 'text', text: st.delta ?? '', streaming: true }] };
          }
          case 'text': {
            // The words a turn streamed, now whole.
            const last = r.items[r.items.length - 1];
            if (last?.kind === 'text' && last.streaming) return { ...r, items: [...r.items.slice(0, -1), { kind: 'text', text: st.text }] };
            return { ...r, items: [...r.items, { kind: 'text', text: st.text }] };
          }
          case 'tool_start':
            return {
              ...r,
              retry: undefined,
              items: [
                ...closeText(r.items),
                {
                  kind: 'tool',
                  step: { id: st.id ?? newID('s'), tool: st.tool ?? '', intent: st.intent, args: st.args, running: true, device: one ? st.device : undefined },
                },
              ],
            };
          case 'tool_progress':
            return setTool(r, st.id, (t) => ({ ...t, progress: st.text }));
          case 'tool_end':
            return setTool(r, st.id, (t) => ({ ...t, running: false, progress: undefined, status: st.status, detail: st.detail }));
          case 'plan':
            return { ...r, plan: st.plan };
          case 'retry':
            return { ...r, retry: st.text };
          case 'compact':
            return { ...r, items: [...closeText(r.items), { kind: 'note', text: st.text }] };
          case 'usage':
            return { ...r, usage: st.usage };
          default:
            return r;
        }
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
        together: one || undefined,
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
    (raw: string, phones: PhoneRef[], mode: Mode, together?: boolean) => {
      const text = raw.trim();
      if (!text) return;
      if (runs.current.has(activeID)) {
        edit(activeID, (x) => ({ ...x, queued: [...x.queued, { text, phones, mode, together }] }));
        return;
      }
      startRef.current(activeID, text, phones, mode, together);
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
      } else startRef.current(activeID, q.text, q.phones, q.mode, q.together);
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
  const setTogether = useCallback((together: boolean) => edit(activeID, (x) => ({ ...x, together })), [activeID, edit]);

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
    setTogether,
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
