// A checkup's items, as Settings › Phones and Add phone show them: each
// item's name from the catalog (checkXTitle), its status, and the driver's
// detail for what did not pass.

import { Box, Stack, Typography, useTheme } from '@mui/material';
import CheckCircleIcon from '@mui/icons-material/CheckCircleRounded';
import ErrorIcon from '@mui/icons-material/ErrorRounded';
import WarningIcon from '@mui/icons-material/WarningAmberRounded';
import RemoveCircleIcon from '@mui/icons-material/RemoveCircleOutlineRounded';
import { useI18n } from '@/i18n';
import type { Check } from '@/api/types';

// Check ids (internal/driver: Check*) to the catalog's names for them.
const titleIds: Record<string, string> = {
  host: 'Host',
  adb: 'ADB',
  connection: 'Connection',
  authorized: 'Authorized',
  companion: 'Companion',
  input: 'Input',
  screen: 'Screen',
  background: 'Background',
  wifi: 'WiFi',
  local_network: 'LocalNetwork',
  developer_mode: 'DeveloperMode',
  disk_image: 'DiskImage',
  helper: 'Helper',
};

export function CheckRow({ c }: { c: Check }) {
  const { t, has } = useI18n();
  const p = useTheme().pinky;
  const key = `check${titleIds[c.id] ?? c.id}Title`;
  const title = has(key) ? t(key) : c.detail || c.id;
  const icon = {
    pass: <CheckCircleIcon sx={{ color: p.green }} fontSize="small" aria-label={t('checkupPassLabel')} />,
    fail: <ErrorIcon sx={{ color: p.red }} fontSize="small" aria-label={t('checkupFailLabel')} />,
    warn: <WarningIcon sx={{ color: p.amberDot }} fontSize="small" aria-label={t('checkupWarnLabel')} />,
    skip: <RemoveCircleIcon sx={{ color: p.ink3 }} fontSize="small" aria-label={t('checkupSkipLabel')} />,
  }[c.status] ?? null;
  return (
    <Stack direction="row" spacing={1.25} alignItems="flex-start" sx={{ py: 0.75 }}>
      <Box sx={{ pt: 0.25 }}>{icon}</Box>
      <Box sx={{ minWidth: 0 }}>
        <Typography variant="subtitle1">{title}</Typography>
        {c.detail && c.status !== 'pass' && (
          <Typography variant="body2" color="text.secondary" className="selectable">
            {c.detail}
          </Typography>
        )}
      </Box>
    </Stack>
  );
}

export function CheckList({ checks }: { checks: Check[] }) {
  return (
    <Box>
      {checks.map((c) => (
        <CheckRow key={c.id} c={c} />
      ))}
    </Box>
  );
}
