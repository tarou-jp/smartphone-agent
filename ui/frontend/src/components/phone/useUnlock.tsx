import { useState, type ReactNode } from 'react';
import { api, ApiError } from '@/api/client';
import type { Device } from '@/api/types';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import { PinDialog } from './PinDialog';

/** "Unlock with Touch ID", "Unlock with Windows Hello" or "Unlock from this computer". */
export function useUnlockLabel(): string {
  const t = useT();
  const { boot } = useApp();
  switch (boot?.platform) {
    case 'darwin':
      return t('platformUnlockMac');
    case 'windows':
      return t('platformUnlockWindows');
    default:
      return t('platformUnlockOther');
  }
}

/**
 * Unlocks a phone with its saved PIN. When none is saved it offers to save
 * one, then tries again; other failures toast.
 */
export function useUnlock(): [(d: Device) => void, ReactNode] {
  const t = useT();
  const { toast, refresh } = useApp();
  const [asking, setAsking] = useState<Device | null>(null);
  const unlock = async (d: Device) => {
    try {
      await api.unlock(d.id);
      refresh();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if ((e instanceof ApiError && e.status === 404) || /no PIN is saved/i.test(msg)) {
        setAsking(d);
        return;
      }
      if (e instanceof ApiError && e.status === 409) return; // the person cancelled Touch ID
      toast(t('unlockErrOther', { name: d.name, detail: msg }), 'error');
      refresh();
    }
  };
  const dialog = asking ? (
    <PinDialog
      device={asking}
      open
      onClose={() => setAsking(null)}
      onSaved={() => {
        const d = asking;
        setAsking(null);
        void unlock(d);
      }}
    />
  ) : null;
  return [(d) => void unlock(d), dialog];
}
