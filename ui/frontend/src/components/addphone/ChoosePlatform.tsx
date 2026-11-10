import { Card, CardActionArea, Stack, Typography, useTheme } from '@mui/material';
import AndroidIcon from '@mui/icons-material/Android';
import PhoneIphoneIcon from '@mui/icons-material/PhoneIphone';
import { useT } from '@/i18n';
import { href } from '@/lib/router';
import { GuideHeading } from './Guide';

function PlatformCard({ to, icon, title, detail }: { to: string; icon: React.ReactNode; title: string; detail: string }) {
  const p = useTheme().pinky;
  return (
    <Card sx={{ flex: 1, minWidth: 220 }}>
      <CardActionArea href={to} sx={{ p: 3, height: '100%' }}>
        <Stack spacing={1.5}>
          <Stack sx={{ width: 44, height: 44, borderRadius: 2.5, bgcolor: p.accentTint, color: 'primary.main', alignItems: 'center', justifyContent: 'center' }}>
            {icon}
          </Stack>
          <Typography variant="h3">{title}</Typography>
          <Typography variant="body2" color="text.secondary">
            {detail}
          </Typography>
        </Stack>
      </CardActionArea>
    </Card>
  );
}

export function ChoosePlatform() {
  const t = useT();
  return (
    <>
      <GuideHeading title={t('addPhoneChooseTitle')} detail={t('addPhoneChooseDetail')} />
      <Stack direction="row" spacing={2} useFlexGap flexWrap="wrap">
        <PlatformCard to={href({ page: 'add-phone', platform: 'android' })} icon={<AndroidIcon />} title="Android" detail={t('addPhoneChooseAndroid')} />
        <PlatformCard to={href({ page: 'add-phone', platform: 'iphone' })} icon={<PhoneIphoneIcon />} title="iPhone" detail={t('addPhoneChooseIPhone')} />
      </Stack>
    </>
  );
}
