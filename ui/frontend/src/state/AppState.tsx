// The client's live state: what the server said at start, the phones,
// tasks and Needs you items (read again on every change event), the
// settings, the mirrors' sizes and the toasts.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { api, subscribe, ApiError, type ServerEvent } from '@/api/client';
import type { AppState as Live, Bootstrap, MirrorSize, Settings, Device, DiskImage } from '@/api/types';
import { useT } from '@/i18n';

export interface Toast {
  id: number;
  text: string;
  severity: 'info' | 'success' | 'error';
}

interface Ctx {
  boot: Bootstrap | null;
  live: Live;
  loaded: boolean;
  /** False while the event stream is down; the page shows it. */
  connected: boolean;
  settings: Settings | null;
  /** Patches settings; the server answers with all of them. */
  patchSettings: (p: Partial<Settings>) => Promise<void>;
  sizes: Record<string, MirrorSize>;
  diskImage: DiskImage['progress'] | null;
  refresh: () => void;
  toasts: Toast[];
  toast: (text: string, severity?: Toast['severity']) => void;
  dismissToast: (id: number) => void;
  /**
   * Runs an action, and says what failed in a toast: "Couldn't {what}:
   * {error}". what is a lowercase phrase, as the call* strings are.
   */
  run: <R>(what: string, fn: () => Promise<R>) => Promise<R | undefined>;
  device: (id: string | undefined) => Device | undefined;
}

const C = createContext<Ctx | null>(null);

const empty: Live = { devices: [], tasks: [], needs: [] };

export function AppStateProvider({ children }: { children: ReactNode }) {
  const t = useT();
  const [boot, setBoot] = useState<Bootstrap | null>(null);
  const [live, setLive] = useState<Live>(empty);
  const [loaded, setLoaded] = useState(false);
  const [connected, setConnected] = useState(true);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [sizes, setSizes] = useState<Record<string, MirrorSize>>({});
  const [diskImage, setDiskImage] = useState<DiskImage['progress'] | null>(null);
  const [toasts, setToasts] = useState<Toast[]>([]);
  const seq = useRef(0);
  const pending = useRef<number | null>(null);
  const inflight = useRef(false);
  const again = useRef(false);

  const load = useCallback(async () => {
    if (inflight.current) {
      again.current = true;
      return;
    }
    inflight.current = true;
    try {
      const s = await api.state();
      setLive(s);
      setLoaded(true);
      setConnected(true);
    } catch {
      setConnected(false);
    } finally {
      inflight.current = false;
      if (again.current) {
        again.current = false;
        void load();
      }
    }
  }, []);

  // Changes come in bursts: read the state once per burst.
  const refresh = useCallback(() => {
    if (pending.current !== null) return;
    pending.current = window.setTimeout(() => {
      pending.current = null;
      void load();
    }, 40);
  }, [load]);

  useEffect(() => {
    api.bootstrap().then(setBoot, () => setConnected(false));
    api.settings().then(setSettings, () => undefined);
    void load();
    const off = subscribe(
      (e: ServerEvent) => {
        switch (e.type) {
          case 'change':
            refresh();
            break;
          case 'settings':
            setSettings(e.data);
            break;
          case 'mirrorSize':
            setSizes((s) => ({ ...s, [e.data.deviceId]: e.data }));
            break;
          case 'diskImage':
            setDiskImage(e.data);
            break;
          case 'hello':
            setConnected(true);
            refresh();
            break;
        }
      },
      () => setConnected(true),
    );
    // Durations and deadlines move without events; so do phones that a
    // dropped event left stale.
    const tick = window.setInterval(() => void load(), 5000);
    return () => {
      off();
      window.clearInterval(tick);
    };
  }, [load, refresh]);

  const toast = useCallback((text: string, severity: Toast['severity'] = 'info') => {
    const id = ++seq.current;
    setToasts((l) => [...l.slice(-3), { id, text, severity }]);
  }, []);
  const dismissToast = useCallback((id: number) => setToasts((l) => l.filter((x) => x.id !== id)), []);

  const run = useCallback(
    async <R,>(what: string, fn: () => Promise<R>): Promise<R | undefined> => {
      try {
        const r = await fn();
        refresh();
        return r;
      } catch (e) {
        const msg = e instanceof ApiError || e instanceof Error ? e.message : String(e);
        toast(t('toastCouldNot', { action: what, error: msg }), 'error');
        refresh();
        return undefined;
      }
    },
    [refresh, toast, t],
  );

  const patchSettings = useCallback(
    async (p: Partial<Settings>) => {
      setSettings((s) => (s ? { ...s, ...p } : s));
      const next = await run('save the settings', () => api.patchSettings(p));
      if (next) setSettings(next);
    },
    [run],
  );

  const device = useCallback((id: string | undefined) => live.devices.find((d) => d.id === id), [live.devices]);

  const value = useMemo<Ctx>(
    () => ({
      boot,
      live,
      loaded,
      connected,
      settings,
      patchSettings,
      sizes,
      diskImage,
      refresh,
      toasts,
      toast,
      dismissToast,
      run,
      device,
    }),
    [boot, live, loaded, connected, settings, patchSettings, sizes, diskImage, refresh, toasts, toast, dismissToast, run, device],
  );
  return <C.Provider value={value}>{children}</C.Provider>;
}

export function useApp(): Ctx {
  const v = useContext(C);
  if (!v) throw new Error('useApp outside AppStateProvider');
  return v;
}

/** Whether the gateway offers an extra, by deskapi's capability name. */
export function useHas(name: string): boolean {
  return useApp().boot?.has[name] ?? false;
}
