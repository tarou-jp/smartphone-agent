import {
  Badge,
  Box,
  Button,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  ListSubheader,
  Stack,
  Tooltip,
  Typography,
  useTheme,
} from '@mui/material';
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
import { StatusDot } from '@/components/common';
import type { Device } from '@/api/types';
import { useGw } from '@/lib/gwtext';

export const sidebarWidth = 248;

function PhoneRow({ d, selected, needs }: { d: Device; selected: boolean; needs: number }) {
  const t = useT();
  const status = useGw().statusLine(d);
  return (
    <ListItemButton
      component="a"
      href={href({ page: 'phones', deviceId: d.id })}
      selected={selected}
      aria-label={t('sidebarPhoneRow', { name: d.name, status })}
      sx={{ py: 0.75, alignItems: 'flex-start' }}
    >
      <ListItemIcon sx={{ minWidth: 20, mt: '7px' }}>
        <StatusDot status={d.status} />
      </ListItemIcon>
      <ListItemText
        primary={d.name}
        secondary={status}
        primaryTypographyProps={{ noWrap: true, fontWeight: 500, fontSize: 13 }}
        secondaryTypographyProps={{ noWrap: true, fontSize: 12 }}
        sx={{ my: 0, opacity: d.status === 'offline' ? 0.6 : 1 }}
      />
      {needs > 0 && <Badge badgeContent={needs} color="warning" sx={{ mt: '12px', mr: 1 }} />}
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
  const nav = (r: Route, icon: React.ReactNode, label: string, selected: boolean, badge = 0) => (
    <ListItemButton component="a" href={href(r)} selected={selected} sx={{ py: 0.75 }}>
      <ListItemIcon sx={{ minWidth: 30, color: 'text.secondary' }}>{icon}</ListItemIcon>
      <ListItemText primary={label} primaryTypographyProps={{ fontSize: 13, fontWeight: 500 }} />
      {badge > 0 && <Badge badgeContent={badge} color="warning" sx={{ mr: 1 }} />}
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
      <Stack direction="row" alignItems="center" spacing={1} sx={{ px: 2, height: 52, flexShrink: 0 }}>
        <Box component="img" src={logo} alt="" sx={{ width: 22, height: 22 }} />
        <Typography sx={{ fontWeight: 600, fontSize: 15 }}>Pinky</Typography>
      </Stack>
      <Box sx={{ flex: 1, overflowY: 'auto', px: 1 }}>
        <List
          dense
          subheader={
            <ListSubheader disableSticky sx={{ bgcolor: 'transparent', lineHeight: '28px', fontSize: 12, px: 1 }}>
              {t('sidebarPhones')}
            </ListSubheader>
          }
        >
          {devices.length > 1 &&
            nav(
              { page: 'phones' },
              <GridViewIcon fontSize="small" />,
              t('sidebarAllPhones'),
              route.page === 'phones' && !route.deviceId,
            )}
          {devices.map((d) => (
            <PhoneRow
              key={d.id}
              d={d}
              needs={needsBy(d.id)}
              selected={route.page === 'phones' && route.deviceId === d.id}
            />
          ))}
          {nav({ page: 'add-phone' }, <AddIcon fontSize="small" />, t('sidebarAddPhone'), route.page === 'add-phone')}
        </List>
      </Box>
      <Box sx={{ px: 1, pb: 1 }}>
        <List dense>
          {nav({ page: 'activity' }, <HistoryIcon fontSize="small" />, t('sidebarActivity'), route.page === 'activity')}
          {nav({ page: 'settings' }, <SettingsIcon fontSize="small" />, t('sidebarSettings'), route.page === 'settings')}
        </List>
        <Tooltip title={navigator.platform.startsWith('Mac') ? '⌥⌘.' : 'Ctrl+Alt+.'}>
          <span>
            <Button
              fullWidth
              variant="outlined"
              color="error"
              startIcon={<StopIcon />}
              disabled={!anyRunning}
              onClick={() => run(t('callStopAll'), () => import('@/api/client').then((m) => m.api.stopAll()))}
              sx={{ mt: 0.5, borderColor: p.redLine, bgcolor: anyRunning ? p.redTint : 'transparent' }}
            >
              {t('sidebarStopAll')}
            </Button>
          </span>
        </Tooltip>
      </Box>
    </Box>
  );
}
