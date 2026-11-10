// Add phone, after ui/addphone.go: choose Android or iPhone, then follow
// the steps; Android can also pair over Wi-Fi.

import { useState } from 'react';
import { Box, Button } from '@mui/material';
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
  const back =
    platform === 'android' && wireless
      ? () => setWireless(false)
      : platform
        ? () => {
            setWireless(false);
            navigate({ page: 'add-phone' });
          }
        : undefined;

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
      <PageHeader
        title={t('addPhoneLabel')}
        actions={
          back && (
            <Button color="inherit" onClick={back}>
              {t('addPhoneBack')}
            </Button>
          )
        }
      />
      <Box sx={{ flex: 1, overflowY: 'auto', bgcolor: (th) => th.pinky.bg2 }}>
        <Box sx={{ maxWidth: 720, mx: 'auto', px: 4, py: 4 }}>{body}</Box>
      </Box>
    </>
  );
}
