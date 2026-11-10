import { Badge, Box, Card, CardActionArea, Stack, Typography, useTheme } from '@mui/material';
import type { Device } from '@/api/types';
import { useT } from '@/i18n';
import { href } from '@/lib/router';
import { StatusDot } from '@/components/common';
import { Mirror } from './Mirror';
import { markedStep, usePhone } from './usePhone';
import { useGw } from '@/lib/gwtext';

/** A phone on All phones: a small live picture, its name and what it does. */
export function PhoneTile({ device }: { device: Device }) {
  const t = useT();
  const gw = useGw();
  const p = useTheme().pinky;
  const { task, needs } = usePhone(device.id);
  const mine = needs.filter((n) => n.deviceId === device.id).length;
  const step = markedStep(task);
  const live = device.status !== 'offline' && device.status !== 'needs_setup' && device.status !== 'locked';
  return (
    <Card sx={{ borderColor: mine ? p.amberLine : undefined, bgcolor: mine ? p.amberTint : undefined }}>
      <CardActionArea
        href={href({ page: 'phones', deviceId: device.id })}
        aria-label={t('sidebarPhoneRow', { name: device.name, status: gw.statusLine(device) })}
        sx={{ p: 1.5, display: 'flex', flexDirection: 'column', alignItems: 'stretch' }}
      >
        <Box sx={{ height: 300, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', color: p.ink3 }}>
          {live ? (
            <Mirror
              deviceId={device.id}
              interactive={false}
              fps={10}
              label={t('mirrorScreenLabel', { phone: device.name })}
              target={step?.target}
              point={step?.point}
              generation={device.state}
            />
          ) : (
            <Typography variant="body2">
              {device.status === 'locked' ? t('mirrorLockedTile') : device.status === 'offline' ? t('phoneNotConnected') : device.detail}
            </Typography>
          )}
        </Box>
        <Stack direction="row" alignItems="center" spacing={1} sx={{ mt: 1.5 }}>
          <StatusDot status={device.status} />
          <Typography sx={{ fontWeight: 650, flex: 1, minWidth: 0 }} noWrap>
            {device.name}
          </Typography>
          {mine > 0 && <Badge badgeContent={mine} color="warning" sx={{ mr: 1 }} />}
        </Stack>
        <Typography variant="body2" color="text.secondary" noWrap sx={{ pl: 2 }}>
          {gw.statusLine(device)}
        </Typography>
      </CardActionArea>
    </Card>
  );
}
