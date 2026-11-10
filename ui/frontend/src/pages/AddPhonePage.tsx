// Add phone, after ui/addphone.go: choose Android or iPhone, then follow
// the steps; Android can also pair over Wi-Fi.

import { useState } from 'react';
import { Box, ButtonBase } from '@mui/material';
import ChevronIcon from '@mui/icons-material/ChevronRightRounded';
import { PageHeader } from '@/components/common';
import { useT } from '@/i18n';
import { navigate } from '@/lib/router';
import { ChoosePlatform } from '@/components/addphone/ChoosePlatform';
import { AndroidSteps } from '@/components/addphone/AndroidSteps';
import { WirelessPairing } from '@/components/addphone/WirelessPairing';
import { IPhoneSteps } from '@/components/addphone/IPhoneSteps';

export function AddPhonePage({ platform }: { platform?: string }) {
  const t = useT();
  const [wireless, setWireless] = useState(false);
  // A breadcrumb in the title: each step before the last goes back to it.
  const toChoose = () => {
    setWireless(false);
    navigate({ page: 'add-phone' });
  };
  const crumbs: { label: string; go?: () => void }[] = [{ label: t('addPhoneLabel'), go: platform ? toChoose : undefined }];
  if (platform === 'android') crumbs.push({ label: 'Android', go: wireless ? () => setWireless(false) : undefined });
  else if (platform === 'iphone' || platform === 'ios') crumbs.push({ label: 'iPhone' });
  if (platform === 'android' && wireless) crumbs.push({ label: 'Wi-Fi' });

  let body;
  if (platform === 'android') {
    body = wireless ? <WirelessPairing onUSB={() => setWireless(false)} /> : <AndroidSteps onWireless={() => setWireless(true)} />;
  } else if (platform === 'iphone' || platform === 'ios') {
    body = <IPhoneSteps />;
  } else {
    body = <ChoosePlatform />;
  }

  return (
    <>
      <PageHeader title={<Crumbs items={crumbs} />} />
      <Box sx={{ flex: 1, overflowY: 'auto', scrollbarGutter: 'stable', bgcolor: (th) => th.pinky.bg2 }}>
        <Box sx={{ maxWidth: 720, mx: 'auto', px: 4, py: 4 }}>{body}</Box>
      </Box>
    </>
  );
}

function Crumbs({ items }: { items: { label: string; go?: () => void }[] }) {
  return (
    <Box component="span" sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5, minWidth: 0 }}>
      {items.map((c, i) => (
        <Box component="span" key={i} sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5, minWidth: 0 }}>
          {i > 0 && <ChevronIcon sx={{ fontSize: 18, color: 'text.disabled' }} />}
          {c.go ? (
            <ButtonBase
              onClick={c.go}
              sx={{
                font: 'inherit',
                color: 'text.secondary',
                borderRadius: 1,
                px: 0.5,
                mx: -0.5,
                '&:hover': { color: 'text.primary', bgcolor: 'action.hover' },
                WebkitAppRegion: 'no-drag',
              }}
            >
              {c.label}
            </ButtonBase>
          ) : (
            <span>{c.label}</span>
          )}
        </Box>
      ))}
    </Box>
  );
}
