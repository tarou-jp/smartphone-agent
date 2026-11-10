import { IconButton, Stack, ToggleButton, ToggleButtonGroup, Tooltip, Typography } from '@mui/material';
import ArrowBackIcon from '@mui/icons-material/ArrowBackIosNewRounded';
import HomeIcon from '@mui/icons-material/CircleOutlined';
import RecentsIcon from '@mui/icons-material/CropSquareRounded';
import { api } from '@/api/client';
import type { Device } from '@/api/types';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';

/** The keys under the mirror, and which screen it shows. */
export function ControlBar({
  device,
  agent,
  agentName,
  showAgentScreen,
  onAgentScreen,
}: {
  device: Device;
  agent: boolean;
  agentName: string;
  /** Offer the agent's background screen. */
  showAgentScreen: boolean;
  onAgentScreen: (on: boolean) => void;
}) {
  const t = useT();
  const { run } = useApp();
  const press = (key: string, label: string) => run(t('phonePressKey', { key: label }), () => api.key(device.id, key));
  const keys = [
    { key: 'back', label: t('phoneKeyBack'), icon: <ArrowBackIcon fontSize="small" />, android: true },
    { key: 'home', label: t('phoneKeyHome'), icon: <HomeIcon fontSize="small" />, android: false },
    { key: 'recents', label: t('phoneKeyRecents'), icon: <RecentsIcon fontSize="small" />, android: true },
  ].filter((k) => !k.android || device.platform === 'android');
  const offline = device.status === 'offline' || device.status === 'needs_setup';
  return (
    <Stack direction="row" alignItems="center" justifyContent="center" spacing={1} sx={{ py: 1, flexShrink: 0 }}>
      {!agent &&
        keys.map((k) => (
          <Tooltip key={k.key} title={k.label}>
            <span>
              <IconButton aria-label={k.label} disabled={offline} onClick={() => press(k.key, k.label)}>
                {k.icon}
              </IconButton>
            </span>
          </Tooltip>
        ))}
      {agent && (
        <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 360, textAlign: 'center' }}>
          {t('phoneKeysAgentScreen', { agent: agentName })}
        </Typography>
      )}
      {showAgentScreen && (
        <ToggleButtonGroup
          size="small"
          exclusive
          value={agent ? 'agent' : 'phone'}
          onChange={(_, v) => v && onAgentScreen(v === 'agent')}
          sx={{ ml: 1 }}
        >
          <ToggleButton value="phone" sx={{ px: 1.5, py: 0.25, textTransform: 'none' }}>
            {t('phonePhoneScreen')}
          </ToggleButton>
          <ToggleButton value="agent" sx={{ px: 1.5, py: 0.25, textTransform: 'none' }}>
            {t('phoneAgentScreen', { agent: agentName })}
          </ToggleButton>
        </ToggleButtonGroup>
      )}
    </Stack>
  );
}
