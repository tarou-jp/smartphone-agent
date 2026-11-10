// The HTTP client for internal/deskapi. The page arrives with ?token=…,
// which the server trades for a cookie; we keep it for headers too and take
// it out of the address bar.

import type {
  AgentSetupView,
  AgentView,
  AppState,
  Bootstrap,
  Checkup,
  Connections,
  Decision,
  DiskImage,
  Exemption,
  FileProblem,
  HistoryPage,
  HostCheck,
  MirrorSize,
  ModelView,
  PINState,
  RecordingInfo,
  Rule,
  RulesView,
  ScopeView,
  Settings,
  Device,
} from './types';

const params = new URLSearchParams(window.location.search);
let token = params.get('token') ?? '';
try {
  if (token) sessionStorage.setItem('pinky-token', token);
  else token = sessionStorage.getItem('pinky-token') ?? '';
} catch {
  // Storage can be off; the cookie still works.
}
export const forcedTheme = params.get('theme') as 'light' | 'dark' | null;
if (params.has('token')) {
  params.delete('token');
  const q = params.toString();
  window.history.replaceState(null, '', window.location.pathname + (q ? '?' + q : '') + window.location.hash);
}

/** An error the API answered with. */
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

function headers(json: boolean): HeadersInit {
  const h: Record<string, string> = {};
  if (token) h['X-Pinky-Token'] = token;
  if (json) h['Content-Type'] = 'application/json';
  return h;
}

async function request<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: headers(body !== undefined),
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: 'same-origin',
    signal,
  });
  const text = await res.text();
  let data: unknown = undefined;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }
  if (!res.ok) {
    const msg =
      data && typeof data === 'object' && 'error' in data ? String((data as { error: unknown }).error) : res.statusText;
    throw new ApiError(res.status, msg);
  }
  return data as T;
}

const get = <T>(p: string, signal?: AbortSignal) => request<T>('GET', p, undefined, signal);
const post = <T>(p: string, b: unknown = {}) => request<T>('POST', p, b);
const put = <T>(p: string, b: unknown) => request<T>('PUT', p, b);
const del = <T>(p: string) => request<T>('DELETE', p);
const enc = encodeURIComponent;

/** URL with the token, for <img>, <video> and EventSource, which cannot send headers. */
export function authURL(path: string): string {
  if (!token) return path;
  return path + (path.includes('?') ? '&' : '?') + 'token=' + enc(token);
}

export const api = {
  bootstrap: () => get<Bootstrap>('/api/bootstrap'),
  state: () => get<AppState>('/api/state'),
  quit: () => post('/api/quit'),
  open: (path: string) => post('/api/open', { path }),

  stopAll: () => post('/api/stop-all'),
  stop: (id: string) => post(`/api/devices/${enc(id)}/stop`),
  unlock: (id: string) => post(`/api/devices/${enc(id)}/unlock`),
  takeOver: (id: string) => post(`/api/devices/${enc(id)}/takeover`),
  handBack: (id: string, note = '') => post(`/api/devices/${enc(id)}/handback`, { note }),
  touch: (id: string, action: 'down' | 'move' | 'up', nx: number, ny: number) =>
    post(`/api/devices/${enc(id)}/touch`, { action, nx, ny }),
  key: (id: string, key: string) => post(`/api/devices/${enc(id)}/key`, { key }),
  text: (id: string, text: string) => post(`/api/devices/${enc(id)}/text`, { text }),
  mirrorURL: (id: string, o: { w: number; h: number; agent?: boolean; fps?: number }) =>
    authURL(
      `/api/devices/${enc(id)}/mirror?w=${Math.round(o.w)}&h=${Math.round(o.h)}` +
        (o.agent ? '&agent=1' : '') +
        (o.fps ? `&fps=${o.fps}` : '') +
        `&n=${Date.now()}`,
    ),
  size: (id: string) => get<MirrorSize>(`/api/devices/${enc(id)}/size`),
  pin: (id: string) => get<PINState>(`/api/devices/${enc(id)}/pin`),
  savePIN: (id: string, pin: string) => put<PINState>(`/api/devices/${enc(id)}/pin`, { pin }),
  removePIN: (id: string) => del<PINState>(`/api/devices/${enc(id)}/pin`),

  decide: (needId: string, decision: Decision, note = '') =>
    post(`/api/needs/${enc(needId)}/decide`, { decision, note }),
  needShotURL: (needId: string) => authURL(`/api/needs/${enc(needId)}/screenshot`),

  history: (q: { search?: string; before?: string; limit?: number }, signal?: AbortSignal) => {
    const p = new URLSearchParams();
    if (q.search) p.set('search', q.search);
    if (q.before) p.set('before', q.before);
    if (q.limit) p.set('limit', String(q.limit));
    return get<HistoryPage>('/api/history?' + p.toString(), signal);
  },
  mediaURL: (id: string) => authURL(`/api/media/${enc(id)}`),
  wipeHistory: () => del('/api/history'),
  exportHistory: (taskIds: string[]) => post<{ path: string }>('/api/history/export', { taskIds }),
  recording: (taskId: string) => get<RecordingInfo>(`/api/recordings/${enc(taskId)}`),
  recordingURL: (taskId: string) => authURL(`/api/recordings/${enc(taskId)}/file`),

  settings: () => get<Settings>('/api/settings'),
  patchSettings: (patch: Partial<Settings>) => put<Settings>('/api/settings', patch),

  agents: () => get<AgentView[]>('/api/agents'),
  setAgentScope: (name: string, scope: ScopeView) => put(`/api/agents/${enc(name)}/scope`, scope),
  agentSetup: () => get<AgentSetupView[]>('/api/agent-setup'),
  agentSetupDo: (id: string, op: 'connect' | 'remove') =>
    post<{ backups: string[] | null; restart: string; opened: string }>(`/api/agent-setup/${enc(id)}/${op}`),
  exemptions: () => get<Exemption[]>('/api/exemptions'),
  removeExemption: (x: Exemption) => post('/api/exemptions/remove', x),
  rules: () => get<RulesView>('/api/rules'),
  setRules: (base: Rule[], rules: Rule[]) => put<RulesView>('/api/rules', { base, rules }),
  fileProblems: () => get<FileProblem[]>('/api/file-problems'),

  hostChecks: () => get<Record<string, HostCheck>>('/api/host-checks'),
  companion: (id: string) => get<{ state: string }>(`/api/devices/${enc(id)}/companion`),
  installCompanion: (id: string) => post(`/api/devices/${enc(id)}/companion/install`),
  updateCompanion: (id: string) =>
    post<{ from: string; to: string; updated: boolean }>(`/api/devices/${enc(id)}/companion/update`),
  background: (id: string) => get<{ on: boolean }>(`/api/devices/${enc(id)}/background`),
  setBackground: (id: string, on: boolean) => put<{ on: boolean }>(`/api/devices/${enc(id)}/background`, { on }),
  connections: (id: string) => get<Connections>(`/api/devices/${enc(id)}/connections`),
  setConnection: (id: string, way: string) => put(`/api/devices/${enc(id)}/connection`, { way }),
  checkup: (id: string) => post<Checkup>(`/api/devices/${enc(id)}/checkup`),
  removePhone: (id: string, history: boolean) => del(`/api/devices/${enc(id)}${history ? '?history=1' : ''}`),

  startPairing: () => post<{ id: string; qr: string }>('/api/pairing'),
  waitPairing: (id: string) => get<{ paired: boolean; device?: Device }>(`/api/pairing/${enc(id)}/wait`),
  cancelPairing: (id: string) => del(`/api/pairing/${enc(id)}`),
  pairWithCode: (addr: string, code: string) => post<Device>('/api/pairing/code', { addr, code }),
  connectWiFi: (addr: string) => post<Device>('/api/pairing/wifi', { addr }),
  diskImage: () => get<DiskImage>('/api/disk-image'),
  downloadDiskImage: () => post('/api/disk-image/download'),

  model: () => get<ModelView>('/api/model'),
  addModelKey: (provider: string, label: string, key: string) =>
    post<ModelView>('/api/model/keys', { provider, label, key }),
  editModelKey: (id: string, change: { label?: string; key?: string }) =>
    put<ModelView>(`/api/model/keys/${enc(id)}`, change),
  removeModelKey: (id: string) => del<ModelView>(`/api/model/keys/${enc(id)}`),
  setActiveKey: (id: string) => put<ModelView>('/api/model/active', { id }),
  refreshOllama: () => post<ModelView>('/api/model/ollama/refresh'),
  cancelChat: (runId: string) => post('/api/chat/cancel', { runId }),
};

/** Server-Sent Events the server pushes; see deskapi's events. */
export type ServerEvent =
  | { type: 'change'; data: { kind: string; deviceId?: string; taskId?: string; needId?: string } }
  | { type: 'settings'; data: Settings }
  | { type: 'mirrorSize'; data: MirrorSize }
  | { type: 'diskImage'; data: DiskImage['progress'] }
  | { type: 'hello'; data: unknown };

/** Subscribes to the server's events, reconnecting as EventSource does. */
export function subscribe(onEvent: (e: ServerEvent) => void, onOpen?: () => void): () => void {
  const es = new EventSource(authURL('/api/events'), { withCredentials: true });
  const names: ServerEvent['type'][] = ['change', 'settings', 'mirrorSize', 'diskImage', 'hello'];
  for (const n of names) {
    es.addEventListener(n, (ev) => {
      try {
        onEvent({ type: n, data: JSON.parse((ev as MessageEvent).data) } as ServerEvent);
      } catch {
        // A malformed event changes nothing.
      }
    });
  }
  if (onOpen) es.addEventListener('open', onOpen);
  return () => es.close();
}

export interface ChatStreamHandlers {
  onProgress: (line: string) => void;
  onDone: (reply: string) => void;
  onError: (code: string, message: string) => void;
}

/** One run of the in-app chat, streamed; abort the signal to stop it. */
export async function streamChat(
  body: {
    runId: string;
    history: { role: string; text: string }[];
    prompt: string;
    title: string;
    deviceId?: string;
    deviceName?: string;
    phones?: string[];
    locale: string;
  },
  h: ChatStreamHandlers,
  signal: AbortSignal,
): Promise<void> {
  let res: Response;
  try {
    res = await fetch('/api/chat', {
      method: 'POST',
      headers: headers(true),
      body: JSON.stringify(body),
      credentials: 'same-origin',
      signal,
    });
  } catch (e) {
    if (signal.aborted) h.onError('cancelled', 'stopped');
    else h.onError('failed', String(e));
    return;
  }
  if (!res.ok || !res.body) {
    let msg = res.statusText;
    try {
      msg = (await res.json()).error ?? msg;
    } catch {
      // keep the status text
    }
    h.onError('failed', msg);
    return;
  }
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = '';
  let ended = false;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      let i: number;
      while ((i = buf.indexOf('\n\n')) >= 0) {
        const chunk = buf.slice(0, i);
        buf = buf.slice(i + 2);
        let name = 'message';
        let data = '';
        for (const line of chunk.split('\n')) {
          if (line.startsWith('event: ')) name = line.slice(7);
          else if (line.startsWith('data: ')) data += line.slice(6);
        }
        if (!data) continue;
        const d = JSON.parse(data);
        if (name === 'progress') h.onProgress(d.text);
        else if (name === 'done') {
          ended = true;
          h.onDone(d.reply);
        } else if (name === 'error') {
          ended = true;
          h.onError(d.code, d.message);
        }
      }
    }
  } catch (e) {
    if (!ended) {
      if (signal.aborted) h.onError('cancelled', 'stopped');
      else h.onError('failed', String(e));
    }
    return;
  }
  if (!ended) h.onError(signal.aborted ? 'cancelled' : 'failed', 'the connection ended');
}

/** The pairing QR code as a PNG. */
export const pairingQRURL = (id: string) => authURL(`/api/pairing/${encodeURIComponent(id)}/qr.png`);
