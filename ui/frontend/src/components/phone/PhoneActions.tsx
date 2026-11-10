// The phone page's header buttons: Take over or Hand back, Stop, Unlock,
// and the ⋯ menu with the PIN and background work.

import { useEffect, useState } from 'react';
import { Button, Divider, IconButton, ListItemIcon, ListItemText, Menu, MenuItem, Switch, Tooltip } from '@mui/material';
import MoreIcon from '@mui/icons-material/MoreHoriz';
import KeyIcon from '@mui/icons-material/PasswordRounded';
import LayersIcon from '@mui/icons-material/LayersOutlined';
import SettingsIcon from '@mui/icons-material/SettingsOutlined';
import { api } from '@/api/client';
import type { Device, PINState, Task } from '@/api/types';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import { href } from '@/lib/router';
import { PinDialog } from './PinDialog';
import { useUnlock, useUnlockLabel } from './useUnlock';

export function PhoneActions({
  device,
  task,
  running,
  background,
  onBackground,
}: {
  device: Device;
  task?: Task;
  running: boolean;
  background: boolean | null;
  onBackground: (on: boolean) => void;
}) {
  const t = useT();
  const { run, toast } = useApp();
  const hasBackground = useHas('background');
  const [unlock, unlockDialog] = useUnlock();
  const unlockLabel = useUnlockLabel();
  const [menu, setMenu] = useState<HTMLElement | null>(null);
  const [pin, setPin] = useState<PINState | null>(null);
  const [pinOpen, setPinOpen] = useState(false);
  useEffect(() => {
    if (!device.features.unlock) return;
    api.pin(device.id).then(setPin, () => setPin(null));
  }, [device.id, device.features.unlock, pinOpen]);

  const offline = device.status === 'offline';
  const inControl = device.status === 'in_control';
  const removePIN = () =>
    run(t('pinRemove'), async () => {
      setPin(await api.removePIN(device.id));
      toast(t('pinRemovedToast', { name: device.name }), 'success');
    });
  return (
    <>
      {device.status === 'locked' && device.features.unlock && (
        <Button variant="contained" onClick={() => unlock(device)}>
          {unlockLabel}
        </Button>
      )}
      {inControl ? (
        <Button variant="contained" onClick={() => run(t('callHandBack'), () => api.handBack(device.id))}>
          {t('phoneHandBack')}
        </Button>
      ) : (
        !offline &&
        running && (
          <Tooltip title={t('needsTakeOverTooltip')}>
            <Button variant="outlined" color="inherit" onClick={() => run(t('callTakeOver'), () => api.takeOver(device.id))}>
              {t('phoneTakeOver')}
            </Button>
          </Tooltip>
        )
      )}
      {running && task && (
        <Button variant="outlined" color="error" onClick={() => run(t('callStop'), () => api.stop(device.id))}>
          {t('tasksStop')}
        </Button>
      )}
      <Tooltip title={t('phoneMoreControls')}>
        <IconButton aria-label={t('phoneMoreControls')} onClick={(e) => setMenu(e.currentTarget)}>
          <MoreIcon />
        </IconButton>
      </Tooltip>
      <Menu anchorEl={menu} open={!!menu} onClose={() => setMenu(null)} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }} transformOrigin={{ vertical: 'top', horizontal: 'right' }}>
        {device.features.unlock && pin?.supported && [
          <MenuItem key="pin" onClick={() => (setMenu(null), setPinOpen(true))}>
            <ListItemIcon>
              <KeyIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText primary={pin.saved ? t('pinChangeLabel', { name: device.name }) : t('pinSaveOpen')} />
          </MenuItem>,
          pin.saved && (
            <MenuItem key="rm" onClick={() => (setMenu(null), void removePIN())}>
              <ListItemIcon />
              <ListItemText primary={t('pinRemoveLabel', { name: device.name })} />
            </MenuItem>
          ),
        ]}
        {device.features.background && hasBackground && background !== null && (
          <MenuItem onClick={() => onBackground(!background)}>
            <ListItemIcon>
              <LayersIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText primary={t('phoneWorkInBackground')} secondary={t('phoneBackgroundDesc')} sx={{ maxWidth: 280, whiteSpace: 'normal' }} />
            <Switch checked={background} tabIndex={-1} sx={{ ml: 1 }} />
          </MenuItem>
        )}
        <Divider />
        <MenuItem component="a" href={href({ page: 'settings', section: 'phones' })} onClick={() => setMenu(null)}>
          <ListItemIcon>
            <SettingsIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText primary={t('webPhoneSettings')} />
        </MenuItem>
      </Menu>
      <PinDialog device={device} open={pinOpen} onClose={() => setPinOpen(false)} />
      {unlockDialog}
    </>
  );
}
