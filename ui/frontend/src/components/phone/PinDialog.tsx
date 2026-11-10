import { useState } from 'react';
import { Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField, Typography } from '@mui/material';
import { api } from '@/api/client';
import type { Device } from '@/api/types';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';

/**
 * Saves the phone's PIN in the system keychain, for unlocking it from
 * this computer. The PIN goes to Pinky and is never shown again.
 */
export function PinDialog({
  device,
  open,
  onClose,
  onSaved,
}: {
  device: Device;
  open: boolean;
  onClose: () => void;
  onSaved?: () => void;
}) {
  const t = useT();
  const { boot, toast } = useApp();
  const [pin, setPin] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);
  const pitch =
    boot?.platform === 'darwin' ? t('unlockPitchMac') : boot?.platform === 'windows' ? t('unlockPitchWindows') : t('unlockPitchOther');
  const close = () => {
    setPin('');
    setErr('');
    onClose();
  };
  const save = async () => {
    if (pin.trim().length < 4) {
      setErr(t('pinErrShort', { name: device.name }));
      return;
    }
    setBusy(true);
    try {
      await api.savePIN(device.id, pin.trim());
      toast(t('pinSavedToast', { name: device.name }), 'success');
      setPin('');
      setErr('');
      onSaved?.();
      onClose();
    } catch (e) {
      setErr(t('pinErrSave', { detail: e instanceof Error ? e.message : String(e) }));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog open={open} onClose={close} maxWidth="xs" fullWidth>
      <DialogTitle sx={{ fontSize: 15, fontWeight: 650 }}>{t('pinSaveLabel', { name: device.name })}</DialogTitle>
      <DialogContent>
        <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
          {boot?.keychain === false ? t('unlockNotAvailable') : pitch}
        </Typography>
        <TextField
          type="password"
          autoFocus
          fullWidth
          autoComplete="off"
          label={t('pinFieldLabel', { name: device.name })}
          placeholder={t('pinPlaceholder')}
          value={pin}
          onChange={(e) => setPin(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && void save()}
          error={!!err}
          helperText={err || ' '}
          disabled={boot?.keychain === false}
        />
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button color="inherit" onClick={close}>
          {t('pinCancel')}
        </Button>
        <Button variant="contained" onClick={save} disabled={busy || boot?.keychain === false}>
          {t('pinSave')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
