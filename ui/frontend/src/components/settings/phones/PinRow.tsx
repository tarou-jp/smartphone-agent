import { useEffect, useState } from 'react';
import { Button, Stack, TextField } from '@mui/material';
import { Row, StatusText } from '@/components/common';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { Device, PINState } from '@/api/types';
import { errorText } from '../controls';

/** Saves, changes and removes the PIN Pinky unlocks the phone with. */
export function PinRow({ d }: { d: Device }) {
  const t = useT();
  const { toast, boot } = useApp();
  const [st, setSt] = useState<PINState | null>(null);
  const [editing, setEditing] = useState(false);
  const [pin, setPin] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.pin(d.id).then(setSt, () => setSt(null));
  }, [d.id]);

  const pitch =
    boot?.platform === 'darwin' ? 'unlockPitchMac' : boot?.platform === 'windows' ? 'unlockPitchWindows' : 'unlockPitchOther';

  const save = async () => {
    if (pin.trim().length < 4) {
      setErr(t('pinErrShort', { name: d.name }));
      return;
    }
    setBusy(true);
    try {
      setSt(await api.savePIN(d.id, pin.trim()));
      setEditing(false);
      setPin('');
      setErr('');
      toast(t('pinSavedToast', { name: d.name }), 'success');
    } catch (e) {
      setErr(t('pinErrSave', { detail: errorText(e) }));
    } finally {
      setBusy(false);
    }
  };
  const remove = async () => {
    setBusy(true);
    try {
      setSt(await api.removePIN(d.id));
      toast(t('pinRemovedToast', { name: d.name }));
    } catch (e) {
      toast(t('pinErrRemove', { detail: errorText(e) }), 'error');
    } finally {
      setBusy(false);
    }
  };

  if (!st?.supported) return null;
  return (
    <Row
      label={t('settingsUnlockFromComputer')}
      description={st.saved ? undefined : t(pitch)}
      control={
        !editing && (
          <Stack direction="row" spacing={1} alignItems="center">
            {st.saved && <StatusText tone="good">{t('pinSaved')}</StatusText>}
            <Button
              variant="outlined"
              color="inherit"
              aria-label={st.saved ? t('pinChangeLabel', { name: d.name }) : t('pinSaveLabel', { name: d.name })}
              onClick={() => setEditing(true)}
            >
              {st.saved ? t('pinChange') : t('pinSaveOpen')}
            </Button>
            {st.saved && (
              <Button color="error" disabled={busy} aria-label={t('pinRemoveLabel', { name: d.name })} onClick={remove}>
                {t('pinRemove')}
              </Button>
            )}
          </Stack>
        )
      }
    >
      {editing && (
        <Stack direction="row" spacing={1} alignItems="flex-start" sx={{ mt: 1.5 }}>
          <TextField
            autoFocus
            type="password"
            autoComplete="off"
            label={t('pinFieldLabel', { name: d.name })}
            placeholder={t('pinPlaceholder')}
            value={pin}
            error={!!err}
            helperText={err || undefined}
            onChange={(e) => setPin(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && void save()}
            sx={{ width: 260 }}
          />
          <Button variant="contained" disabled={busy} onClick={save} sx={{ mt: 0.25 }}>
            {t('pinSave')}
          </Button>
          <Button
            color="inherit"
            onClick={() => {
              setEditing(false);
              setPin('');
              setErr('');
            }}
            sx={{ mt: 0.25 }}
          >
            {t('pinCancel')}
          </Button>
        </Stack>
      )}
    </Row>
  );
}
