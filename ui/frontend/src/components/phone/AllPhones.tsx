import { Box, Stack, Typography } from '@mui/material';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import { PageHeader } from '@/components/common';
import { NeedsList } from '@/components/needs/NeedsList';
import { statusRank } from '@/lib/format';
import { PhoneTile } from './PhoneTile';

/** All phones: a tile per phone, and every Needs you item beside them. */
export function AllPhones() {
  const t = useT();
  const { live } = useApp();
  const devices = [...live.devices].sort((a, b) => statusRank[a.status] - statusRank[b.status] || a.name.localeCompare(b.name));
  const online = devices.filter((d) => d.status !== 'offline').length;
  const offline = devices.length - online;
  return (
    <>
      <PageHeader title={t('phoneAllPhones')}>
        <Typography variant="body2" color="text.secondary">
          {[t('phoneConnected', { count: online }), offline ? t('phoneOfflineCount', { count: offline }) : '']
            .filter(Boolean)
            .join(' · ')}
        </Typography>
      </PageHeader>
      <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <Box
          sx={{
            flex: 1,
            minWidth: 0,
            overflowY: 'auto',
            p: 2.5,
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(210px, 1fr))',
            gridAutoRows: 'max-content',
            gap: 2,
            alignContent: 'start',
          }}
        >
          {devices.map((d) => (
            <PhoneTile key={d.id} device={d} />
          ))}
        </Box>
        {live.needs.length > 0 && (
          <Stack component="aside" sx={{ width: 'clamp(280px, 34%, 360px)', flexShrink: 0, borderLeft: 1, borderColor: 'divider', overflowY: 'auto', p: 2 }}>
            <NeedsList needs={live.needs} />
          </Stack>
        )}
      </Box>
    </>
  );
}
