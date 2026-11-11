import { useState } from 'react';
import { Box, Button, ButtonBase, Collapse, Typography, useTheme } from '@mui/material';
import ExpandIcon from '@mui/icons-material/ExpandMoreRounded';
import AddIcon from '@mui/icons-material/AddRounded';
import { PhoneGlyph, Section } from '@/components/common';
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

function statusText(d: Device, t: ReturnType<typeof useT>): string {
  if (d.status === 'offline') return t('settingsPhonesOffline');
  if (d.status === 'needs_setup') return t('settingsPhonesNeedsSetup');
  return t('settingsPhonesConnected');
}

/**
 * One phone: a card whose header names it and says where it stands, and
 * which opens to everything about that phone (unlock, background,
 * connection, checkup, remove), so several phones read as a short list.
 */
function PhoneCard({ d, startOpen }: { d: Device; startOpen: boolean }) {
  const t = useT();
  const p = useTheme().pinky;
  const { boot } = useApp();
  const hasConnections = useHas('connections');
  const hasCheckup = useHas('checkup');
  const hasRemove = useHas('removePhone');
  const [open, setOpen] = useState(startOpen);
  const [removing, setRemoving] = useState(false);
  const facts = [d.model !== d.name && d.model, d.osVersion && `${d.platform === 'ios' ? 'iOS' : 'Android'} ${d.osVersion}`, d.connection]
    .filter(Boolean)
    .join(' · ');
  const panel = `phone-${d.id}-settings`;
  return (
    <Box sx={{ mb: 1.5, border: 1, borderColor: 'divider', borderRadius: '12px', bgcolor: 'background.paper', overflow: 'hidden' }}>
      <ButtonBase
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        aria-controls={panel}
        sx={{ width: '100%', textAlign: 'left', px: 2, py: 1.75, gap: 1.5, justifyContent: 'flex-start', '&:hover': { bgcolor: 'action.hover' } }}
      >
        <PhoneGlyph status={d.status} size={24} />
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="subtitle1" noWrap>
            {d.name}
          </Typography>
          <Typography variant="body2" noWrap sx={{ color: p.ink3 }}>
            {[statusText(d, t), facts].filter(Boolean).join(' · ')}
          </Typography>
        </Box>
        <ExpandIcon sx={{ color: p.ink3, transition: 'transform .2s cubic-bezier(0.22, 1, 0.36, 1)', transform: open ? 'rotate(180deg)' : 'none' }} />
      </ButtonBase>
      {/* The rows are there from the start, closed: each reads its state
          when the page opens, so opening the card is only the animation and
          the card does not grow again as their answers come in. */}
      <Collapse in={open} timeout={200} easing="cubic-bezier(0.22, 1, 0.36, 1)">
        <Box id={panel} sx={{ borderTop: 1, borderColor: 'divider', '& > *:not(:last-child)': { borderBottom: 1, borderColor: 'divider' } }}>
          {d.detail && d.status !== 'idle' && d.status !== 'working' && (
            <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1.5 }}>
              {d.detail}
            </Typography>
          )}
          {boot?.keychain && d.features.unlock && <PinRow d={d} />}
          {d.features.background && <BackgroundRow d={d} />}
          {hasConnections && d.platform === 'ios' && <ConnectionRow d={d} />}
          {hasCheckup && <CheckupPanel d={d} />}
          {hasRemove && (
            <Box sx={{ px: 1, py: 1 }}>
              <Button color="error" onClick={() => setRemoving(true)} aria-label={t('removePhoneButtonLabel', { name: d.name })}>
                {t('removePhoneLink')}
              </Button>
              <RemovePhoneDialog d={d} open={removing} onClose={() => setRemoving(false)} />
            </Box>
          )}
        </Box>
      </Collapse>
    </Box>
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
        <PhoneCard key={d.id} d={d} startOpen={devices.length === 1} />
      ))}
    </Section>
  );
}
