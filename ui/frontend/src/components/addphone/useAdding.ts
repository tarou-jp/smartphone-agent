import { useEffect, useRef, useState } from 'react';
import { api } from '@/api/client';
import type { Device, HostCheck } from '@/api/types';
import { useApp } from '@/state/AppState';

/**
 * The phone of a platform the steps follow, as ui/addphone.go picks it:
 * one plugged in since the page opened, else one still waiting for a step,
 * else one that is ready already (already is then true).
 */
export function useAdding(platform: 'android' | 'ios'): { device?: Device; already: boolean } {
  const { live, loaded } = useApp();
  const known = useRef<Set<string> | null>(null);
  if (known.current === null && loaded) known.current = new Set(live.devices.map((d) => d.id));
  const mine = live.devices.filter((d) => d.platform === platform);
  const fresh = mine.find((d) => known.current && !known.current.has(d.id));
  if (fresh) return { device: fresh, already: false };
  const waiting = mine.find((d) => d.state === 'unauthorized' || d.state === 'needs_setup');
  if (waiting) return { device: waiting, already: false };
  const ready = mine.find((d) => d.state === 'online');
  return { device: ready, already: !!ready };
}

/** What the computer lacks for a platform's phones, asked every 2 s. */
export function useHostCheck(platform: 'android' | 'ios'): HostCheck {
  const [check, setCheck] = useState<HostCheck>({});
  useEffect(() => {
    let live = true;
    const ask = () =>
      api.hostChecks().then(
        (m) => live && setCheck(m[platform] ?? {}),
        () => undefined,
      );
    void ask();
    const t = window.setInterval(ask, 2000);
    return () => {
      live = false;
      window.clearInterval(t);
    };
  }, [platform]);
  return check;
}

/** How long a phone has been in a state, for "no prompt yet?" hints. */
export function useSince(key: string | undefined): number {
  const [since, setSince] = useState(() => Date.now());
  const [, tick] = useState(0);
  useEffect(() => setSince(Date.now()), [key]);
  useEffect(() => {
    const t = window.setInterval(() => tick((n) => n + 1), 1000);
    return () => window.clearInterval(t);
  }, []);
  return Date.now() - since;
}
