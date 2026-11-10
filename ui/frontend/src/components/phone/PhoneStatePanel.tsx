import { Button, Stack, Typography, useTheme } from '@mui/material';
import PhoneOffIcon from '@mui/icons-material/MobileOffOutlined';
import LockIcon from '@mui/icons-material/LockOutlined';
import BuildIcon from '@mui/icons-material/BuildCircleOutlined';
import type { Device } from '@/api/types';
import { useT } from '@/i18n';
import { href } from '@/lib/router';
import { useUnlock, useUnlockLabel } from './useUnlock';

/** What shows in place of the mirror when the phone cannot show its screen. */
export function PhoneStatePanel({ device }: { device: Device }) {
  const t = useT();
  const p = useTheme().pinky;
  const [unlock, dialog] = useUnlock();
  const unlockLabel = useUnlockLabel();
  let icon, title: string, detail: string | undefined, action;
  switch (device.status) {
    case 'offline':
      icon = <PhoneOffIcon sx={{ fontSize: 48 }} />;
      title = t('phoneOffline', { phone: device.name });
      detail = device.detail || t('phoneNotConnected');
      break;
    case 'needs_setup':
      icon = <BuildIcon sx={{ fontSize: 48 }} />;
      title = t('phoneNeedsSetup', { phone: device.name });
      detail = device.detail;
      action = (
        <Button variant="outlined" href={href({ page: 'add-phone', platform: device.platform === 'ios' ? 'iphone' : 'android' })}>
          {t('phoneSetUpIPhone')}
        </Button>
      );
      break;
    case 'locked':
      icon = <LockIcon sx={{ fontSize: 48 }} />;
      title = t('unlockLocked', { name: device.name });
      detail = device.features.unlock ? t('unlockFromHere') : t('unlockOnPhone', { end: t('unlockEnd') });
      action = device.features.unlock ? (
        <Button variant="contained" onClick={() => unlock(device)}>
          {unlockLabel}
        </Button>
      ) : undefined;
      break;
    default:
      return null;
  }
  return (
    <Stack alignItems="center" justifyContent="center" spacing={1.5} sx={{ flex: 1, p: 4, textAlign: 'center', color: p.ink3 }}>
      {icon}
      <Typography variant="h3" color="text.primary">
        {title}
      </Typography>
      {detail && (
        <Typography variant="body1" color="text.secondary" sx={{ maxWidth: 420 }} className="selectable">
          {detail}
        </Typography>
      )}
      {action}
      {dialog}
    </Stack>
  );
}
