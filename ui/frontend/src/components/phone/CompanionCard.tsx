// Pinky Companion on an Android phone: install, update or turn it on,
// after ui/companion.go. On and ready need nothing, so the card hides.

import { useCallback, useEffect, useState } from 'react';
import { Button, LinearProgress, Paper, Stack, Typography, useTheme } from '@mui/material';
import { api } from '@/api/client';
import type { Device } from '@/api/types';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';

const later = new Set<string>();

export function CompanionCard({ device }: { device: Device }) {
  const t = useT();
  const p = useTheme().pinky;
  const { toast } = useApp();
  const [state, setState] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');
  const [hidden, setHidden] = useState(later.has(device.id));
  const load = useCallback(() => {
    api.companion(device.id).then(
      (r) => setState(r.state),
      () => setState(null),
    );
  }, [device.id]);
  // Asked again whenever the phone changes (an install, a reconnect), not
  // only when another phone is shown.
  useEffect(() => {
    if (device.status !== 'offline') load();
    else setState(null);
  }, [load, device.status, device.state]);

  if (hidden || !state || state === 'on' || state === 'ready' || state === 'no_apk') return null;
  let title: string, action: string, body: string;
  switch (state) {
    case 'missing':
      [title, action, body] = [t('companionInstallTitle'), t('companionInstall'), t('companionInstallBody', { name: device.name })];
      break;
    case 'outdated':
    case 'public_key':
      [title, action, body] = [
        t('companionUpdateTitle'),
        t('companionUpdate'),
        state === 'public_key' ? t('companionPublicKeyBody') : t('companionUpdateBody'),
      ];
      break;
    case 'off':
      [title, action, body] = [t('companionTurnOnTitle'), t('companionTurnOn'), t('companionTurnOnBody')];
      break;
    default:
      return null;
  }
  const install = async () => {
    setBusy(true);
    setErr('');
    try {
      await api.installCompanion(device.id);
      toast(t('companionOnToast', { name: device.name }), 'success');
      load();
    } catch (e) {
      setErr(t('companionErrOther', { detail: e instanceof Error ? e.message : String(e) }));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Paper variant="outlined" sx={{ p: 1.75, borderRadius: 3 }} aria-label={t('companionCardLabel', { name: device.name })}>
      <Typography sx={{ fontSize: 14, fontWeight: 650 }}>{busy ? t('companionInstallingTitle') : title}</Typography>
      <Typography variant="body1" color="text.secondary" sx={{ mt: 0.5 }}>
        {body}
      </Typography>
      {busy && <LinearProgress sx={{ mt: 1.5 }} aria-label={t('companionInstalling')} />}
      {err && (
        <Typography variant="body2" sx={{ mt: 1, color: p.red }}>
          {err}
        </Typography>
      )}
      {!busy && (
        <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
          <Button
            color="inherit"
            onClick={() => {
              later.add(device.id);
              setHidden(true);
            }}
          >
            {t('companionNotNow')}
          </Button>
          <Button variant="contained" onClick={install}>
            {action}
          </Button>
        </Stack>
      )}
    </Paper>
  );
}
