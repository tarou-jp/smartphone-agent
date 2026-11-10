import { useEffect, useState } from 'react';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { Device } from '@/api/types';
import { SettingSwitchRow, errorText } from '../controls';

/** Work in the background: agents use a hidden second display. */
export function BackgroundRow({ d }: { d: Device }) {
  const t = useT();
  const { toast } = useApp();
  const [on, setOn] = useState<boolean | null>(null);
  useEffect(() => {
    api.background(d.id).then((r) => setOn(r.on), () => setOn(null));
  }, [d.id]);
  if (on === null) return null;
  return (
    <SettingSwitchRow
      label={t('backgroundTitle')}
      description={t('backgroundDesc')}
      checked={on}
      disabled={d.status === 'offline'}
      onChange={async (v) => {
        setOn(v);
        try {
          setOn((await api.setBackground(d.id, v)).on);
        } catch (e) {
          setOn(!v);
          toast(t('backgroundChangeFailed', { phone: d.name, reason: errorText(e) }), 'error');
        }
      }}
    />
  );
}
