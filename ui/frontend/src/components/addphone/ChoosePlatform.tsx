import { Card, CardActionArea, Stack, Typography, useTheme } from '@mui/material';
import AndroidIcon from '@mui/icons-material/Android';
import PhoneIphoneIcon from '@mui/icons-material/PhoneIphone';
import ChevronIcon from '@mui/icons-material/ChevronRightRounded';
import { useT } from '@/i18n';
import { href } from '@/lib/router';
import { GuideHeading } from './Guide';

// A platform to pick: its icon in the text color, its name and what it
// takes, and a chevron that says the card leads on.
function PlatformCard({ to, icon, title, detail }: { to: string; icon: React.ReactNode; title: string; detail: string }) {
  const p = useTheme().pinky;
  return (
    <Card sx={{ flex: 1, minWidth: 240 }}>
      <CardActionArea href={to} sx={{ px: 2.5, py: 2, height: '100%' }}>
        <Stack direction="row" spacing={2} alignItems="center">
          <Stack sx={{ color: p.ink2, flexShrink: 0, '& svg': { fontSize: 24 } }}>{icon}</Stack>
          <Stack spacing={0.5} sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="h3">{title}</Typography>
            <Typography variant="body2" color="text.secondary">
              {detail}
            </Typography>
          </Stack>
          <ChevronIcon sx={{ fontSize: 18, color: p.ink3, flexShrink: 0 }} />
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
