import { Alert, Button } from '@mui/material';
import { api } from '@/api/client';
import type { Device, Need, Task } from '@/api/types';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';

/** The line above the mirror while the person has the phone, or the agent waits for them. */
export function StatusBanner({ device, task, needs }: { device: Device; task?: Task; needs: Need[] }) {
  const t = useT();
  const { run } = useApp();
  const agent = task?.agent || t('agentGeneric');
  if (device.status === 'in_control') {
    return (
      <Alert
        severity="info"
        icon={false}
        action={
          <Button variant="contained" onClick={() => run(t('callHandBack'), () => api.handBack(device.id))}>
            {t('phoneHandBack')}
          </Button>
        }
        sx={{ borderRadius: 2, alignItems: 'center' }}
      >
        <b>{t('phoneBannerInControl')}</b>
        {task?.state === 'paused' ? t('phoneBannerPaused', { agent }) : t('phoneBannerHandBackHint')}
      </Alert>
    );
  }
  const handoff = needs.find((n) => n.kind === 'handoff' && n.deviceId === device.id);
  if (handoff) {
    const reason = handoff.title.charAt(0).toLowerCase() + handoff.title.slice(1);
    return (
      <Alert severity="warning" icon={false} sx={{ borderRadius: 2 }}>
        <b>{t('phoneBannerYourTurn')}</b>
        {t('phoneBannerWaits', { agent: handoff.agent || agent, reason })}
      </Alert>
    );
  }
  return null;
}
