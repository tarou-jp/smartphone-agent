import { useState } from 'react';
import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import AddIcon from '@mui/icons-material/AddRounded';
import { PlatformIcon, Row, RowCard, Section, StatusDot } from '@/components/common';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import { href } from '@/lib/router';
import { statusRank } from '@/lib/format';
import type { Device } from '@/api/types';
import { PinRow } from './phones/PinRow';
import { BackgroundRow } from './phones/BackgroundRow';
import { ConnectionRow } from './phones/ConnectionRow';
import { CheckupPanel } from './phones/CheckupPanel';
import { RemovePhoneDialog } from './phones/RemovePhoneDialog';

function statusChip(d: Device, t: ReturnType<typeof useT>) {
  if (d.status === 'offline') return <Chip label={t('settingsPhonesOffline')} variant="outlined" />;
  if (d.status === 'needs_setup') return <Chip label={t('settingsPhonesNeedsSetup')} color="warning" variant="outlined" />;
  return <Chip label={t('settingsPhonesConnected')} color="success" variant="outlined" />;
}

function PhoneCard({ d }: { d: Device }) {
  const t = useT();
  const { boot } = useApp();
  const hasConnections = useHas('connections');
  const hasCheckup = useHas('checkup');
  const hasRemove = useHas('removePhone');
  const [removing, setRemoving] = useState(false);
  const facts = [d.model, d.osVersion && `${d.platform === 'ios' ? 'iOS' : 'Android'} ${d.osVersion}`, d.connection, d.backend]
    .filter(Boolean)
    .join(' · ');
  return (
    <RowCard sx={{ mb: 2 }}>
      <Box sx={{ px: 2, py: 1.5 }}>
        <Stack direction="row" alignItems="center" spacing={1.25}>
          <StatusDot status={d.status} />
          <PlatformIcon platform={d.platform} />
          <Typography variant="h3" sx={{ flex: 1, minWidth: 0 }} noWrap>
            {d.name}
          </Typography>
          {statusChip(d, t)}
        </Stack>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          {facts}
        </Typography>
        {d.detail && d.status !== 'idle' && d.status !== 'working' && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {d.detail}
          </Typography>
        )}
      </Box>
      {boot?.keychain && d.features.unlock && <PinRow d={d} />}
      {d.features.background && <BackgroundRow d={d} />}
      {hasConnections && d.platform === 'ios' && <ConnectionRow d={d} />}
      {hasCheckup && (
        <Row label={t('phoneCheckup')}>
          <CheckupPanel d={d} />
        </Row>
      )}
      {hasRemove && (
        <Box sx={{ px: 2, py: 1 }}>
          <Button color="error" onClick={() => setRemoving(true)} aria-label={t('removePhoneButtonLabel', { name: d.name })}>
            {t('removePhoneLink')}
          </Button>
          <RemovePhoneDialog d={d} open={removing} onClose={() => setRemoving(false)} />
        </Box>
      )}
    </RowCard>
  );
}

export function PhonesSection() {
  const t = useT();
  const { live } = useApp();
  const devices = [...live.devices].sort(
    (a, b) => statusRank[a.status] - statusRank[b.status] || a.name.localeCompare(b.name),
  );
  return (
    <Section
      title={t('settingsSectionPhones')}
      description={t('settingsPhonesIntro')}
      action={
        <Button variant="outlined" color="inherit" startIcon={<AddIcon />} href={href({ page: 'add-phone' })}>
          {t('settingsPhonesAdd')}
        </Button>
      }
    >
      {devices.length === 0 && (
        <Typography variant="body1" color="text.secondary">
          {t('settingsPhonesNone')}
        </Typography>
      )}
      {devices.map((d) => (
        <PhoneCard key={d.id} d={d} />
      ))}
    </Section>
  );
}
