import { Box, Button, ListItemButton, Stack, Typography, useTheme } from '@mui/material';
import GridViewIcon from '@mui/icons-material/GridViewRounded';
import HistoryIcon from '@mui/icons-material/HistoryRounded';
import SettingsIcon from '@mui/icons-material/SettingsOutlined';
import AddIcon from '@mui/icons-material/AddRounded';
import StopIcon from '@mui/icons-material/StopCircleOutlined';
import logo from '@/assets/pinky.svg';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import { href, type Route } from '@/lib/router';
import { statusRank, isRunning } from '@/lib/format';
import { Keys, PhoneGlyph } from '@/components/common';
import type { Device } from '@/api/types';
import { useGw } from '@/lib/gwtext';
import { inMacWindow, topRow } from '@/lib/window';

export const sidebarWidth = 248;

function PhoneRow({ d, selected, needs }: { d: Device; selected: boolean; needs: number }) {
  const t = useT();
  const p = useTheme().pinky;
  const status = useGw().statusLine(d);
  return (
    <ListItemButton
      component="a"
      href={href({ page: 'phones', deviceId: d.id })}
      selected={selected}
      aria-label={t('sidebarPhoneRow', { name: d.name, status })}
      sx={{ py: 1, px: 1.25, gap: 1.5, alignItems: 'center', mb: 0.5 }}
    >
      <PhoneGlyph status={d.status} size={24} />
      <Box sx={{ flex: 1, minWidth: 0, opacity: d.status === 'offline' ? 0.6 : 1 }}>
        <Typography noWrap sx={{ fontSize: 14, fontWeight: 500, lineHeight: '20px' }}>
          {d.name}
        </Typography>
        <Typography noWrap sx={{ fontSize: 13, lineHeight: '18px', color: d.status === 'needs_you' ? p.amber : p.ink2 }}>
          {status}
        </Typography>
      </Box>
      {needs > 0 && (
        <Box
          component="span"
          sx={{
            fontFamily: p.mono,
            fontSize: 11,
            fontWeight: 500,
            lineHeight: '18px',
            minWidth: 18,
            px: '5px',
            textAlign: 'center',
            borderRadius: '9px',
            bgcolor: p.amberDot,
            color: '#1a1206',
          }}
        >
          {needs}
        </Box>
      )}
    </ListItemButton>
  );
}

export function Sidebar({ route }: { route: Route }) {
  const t = useT();
  const p = useTheme().pinky;
  const { live, run } = useApp();
  const devices = [...live.devices].sort(
    (a, b) => statusRank[a.status] - statusRank[b.status] || a.name.localeCompare(b.name),
  );
  const needsBy = (id: string) => live.needs.filter((n) => n.deviceId === id).length;
  const anyRunning = live.tasks.some(isRunning);
  const mac = navigator.platform.startsWith('Mac');
  const nav = (r: Route, icon: React.ReactNode, label: string, selected: boolean) => (
    <ListItemButton component="a" href={href(r)} selected={selected} sx={{ minHeight: 36, py: 0, px: 1.25, gap: 1.5, mb: 0.25 }}>
      <Box sx={{ width: 15, display: 'flex', justifyContent: 'center', color: selected ? 'text.primary' : p.ink3, '& svg': { fontSize: 18 } }}>
        {icon}
      </Box>
      <Typography sx={{ fontSize: 14, fontWeight: selected ? 500 : 400, color: selected ? 'text.primary' : p.ink2 }}>{label}</Typography>
    </ListItemButton>
  );
  return (
    <Box
      component="nav"
      sx={{
        width: sidebarWidth,
        flexShrink: 0,
        bgcolor: p.bg2,
        borderRight: 1,
        borderColor: 'divider',
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
      }}
    >
      {/* As tall as the page's and the chat's top rows. In Pinky's own
          window it holds only the traffic lights; in a browser, the name. */}
      <Stack direction="row" alignItems="center" spacing={1} sx={{ px: 2, height: topRow, flexShrink: 0, cursor: 'default' }}>
        {!inMacWindow && (
          <>
            <Box component="img" src={logo} alt="" sx={{ width: 18, height: 18, borderRadius: '5px' }} />
            <Typography sx={{ fontWeight: 600, fontSize: 14 }}>Pinky</Typography>
          </>
        )}
      </Stack>
      <Box sx={{ flex: 1, overflowY: 'auto', px: 1.25 }}>
        <Typography sx={{ fontSize: 12, color: p.ink3, px: 1.25, mt: 1, mb: 1 }}>{t('sidebarPhones')}</Typography>
        <Box>
          {devices.length > 1 &&
            nav({ page: 'phones' }, <GridViewIcon />, t('sidebarAllPhones'), route.page === 'phones' && !route.deviceId)}
          {devices.map((d) => (
            <PhoneRow key={d.id} d={d} needs={needsBy(d.id)} selected={route.page === 'phones' && route.deviceId === d.id} />
          ))}
          {nav({ page: 'add-phone' }, <AddIcon />, t('sidebarAddPhone'), route.page === 'add-phone')}
        </Box>
      </Box>
      <Box sx={{ px: 1.25, pb: 1.5, pt: 1 }}>
        {nav({ page: 'activity' }, <HistoryIcon />, t('sidebarActivity'), route.page === 'activity')}
        {nav({ page: 'settings' }, <SettingsIcon />, t('sidebarSettings'), route.page === 'settings')}
        <Button
          fullWidth
          variant="outlined"
          color="error"
          disabled={!anyRunning}
          onClick={() => run(t('callStopAll'), () => import('@/api/client').then((m) => m.api.stopAll()))}
          sx={{
            mt: 1.5,
            minHeight: 36,
            justifyContent: 'space-between',
            borderColor: anyRunning ? p.redLine : p.line2,
            bgcolor: anyRunning ? p.redTint : 'transparent',
            boxShadow: 'none',
            '&.Mui-disabled': { borderColor: p.line, color: p.ink3 },
          }}
        >
          <Stack direction="row" spacing={1} alignItems="center">
            <StopIcon sx={{ fontSize: 18 }} />
            <span>{t('sidebarStopAll')}</span>
          </Stack>
          <Keys keys={mac ? ['⌥', '⌘', '.'] : ['Ctrl', 'Alt', '.']} />
        </Button>
      </Box>
    </Box>
  );
}
