import { useState } from 'react';
import {
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Typography,
} from '@mui/material';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { Device } from '@/api/types';
import { errorText } from '../controls';

/** Asks before removing a phone from Pinky, and says what goes with it. */
export function RemovePhoneDialog({ d, open, onClose }: { d: Device; open: boolean; onClose: () => void }) {
  const t = useT();
  const { live, toast, refresh } = useApp();
  const [history, setHistory] = useState(false);
  const [busy, setBusy] = useState(false);
  const task = live.tasks.find((x) => x.id === d.taskId);
  const ios = d.platform === 'ios';
  const wifi = d.connection === 'wifi';
  const lines = [
    t('removePhoneGoesList'),
    task ? t('removePhoneGoesTask', { agent: task.agent, task: task.title }) : '',
    t('removePhoneGoesSettings'),
    wifi ? (ios ? t('removePhoneWiFiIPhone') : t('removePhoneWiFi')) : '',
    !wifi && d.status !== 'offline' ? t('removePhoneCable') : '',
    !wifi ? t('removePhoneTrust') : '',
    ios ? t('removePhoneAddBackIPhone') : t('removePhoneAddBackAndroid'),
  ].filter(Boolean);
  const remove = async () => {
    setBusy(true);
    try {
      await api.removePhone(d.id, history);
      toast(t('removePhoneDone', { name: d.name }), 'success');
      onClose();
    } catch (e) {
      toast(t('removePhoneFailed', { name: d.name, error: errorText(e) }), 'error');
    } finally {
      setBusy(false);
      refresh();
    }
  };
  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ fontSize: 15, fontWeight: 600 }}>{t('removePhoneTitle', { name: d.name })}</DialogTitle>
      <DialogContent>
        {lines.map((l) => (
          <Typography key={l} variant="body1" sx={{ mb: 1 }}>
            {l}
          </Typography>
        ))}
        <FormControlLabel
          control={<Checkbox size="small" checked={history} onChange={(e) => setHistory(e.target.checked)} />}
          label={<Typography variant="body1">{t('removePhoneHistory')}</Typography>}
        />
        {!history && (
          <Typography variant="body2" color="text.secondary">
            {t('removePhoneHistoryStays')}
          </Typography>
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button color="inherit" onClick={onClose}>
          {t('removePhoneCancel')}
        </Button>
        <Button variant="contained" color="error" disabled={busy} onClick={remove}>
          {busy ? t('removePhoneBusy') : t('removePhoneConfirm')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
