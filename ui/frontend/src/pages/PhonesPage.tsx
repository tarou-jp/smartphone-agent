import { useEffect } from 'react';
import { Button } from '@mui/material';
import PhoneIcon from '@mui/icons-material/SmartphoneOutlined';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import { href, navigate } from '@/lib/router';
import { EmptyState, PageHeader } from '@/components/common';
import { AllPhones } from '@/components/phone/AllPhones';
import { PhoneDetail } from '@/components/phone/PhoneDetail';

/** Phones: one phone's page, or all of them. */
export function PhonesPage({ deviceId }: { deviceId?: string }) {
  const t = useT();
  const { live, loaded } = useApp();
  const count = live.devices.length;
  const known = !!deviceId && live.devices.some((d) => d.id === deviceId);
  // With one phone there is nothing to choose; a phone that went away
  // leaves its page.
  useEffect(() => {
    if (!loaded) return;
    if (!deviceId && count === 1) navigate({ page: 'phones', deviceId: live.devices[0].id }, true);
    else if (deviceId && !known) navigate({ page: 'phones' }, true);
  }, [loaded, deviceId, count, known, live.devices]);

  if (deviceId && known) return <PhoneDetail id={deviceId} />;
  if (!loaded) return <PageHeader title={t('phonePageTitle')} />;
  if (count === 0) {
    return (
      <>
        <PageHeader title={t('phonePageTitle')} />
        <EmptyState icon={<PhoneIcon sx={{ fontSize: 48 }} />} title={t('phoneNoPhones')}>
          <Button variant="contained" href={href({ page: 'add-phone' })} sx={{ mt: 1 }}>
            {t('phoneAddPhone')}
          </Button>
        </EmptyState>
      </>
    );
  }
  return <AllPhones />;
}
