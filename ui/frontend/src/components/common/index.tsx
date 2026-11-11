// Small pieces every page uses.

import { useState, type ReactNode } from 'react';
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  Paper,
  Snackbar,
  Stack,
  Tooltip,
  Typography,
  useTheme,
  type SxProps,
  type Theme,
} from '@mui/material';
import AndroidIcon from '@mui/icons-material/Android';
import PhoneIphoneIcon from '@mui/icons-material/PhoneIphone';
import AgentIcon from '@mui/icons-material/ViewSidebarOutlined';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import { usePanel } from '@/components/chat/store';

export { StatusDot } from './StatusDot';

const mac = typeof navigator !== 'undefined' && navigator.platform.startsWith('Mac');

/** Shows the Agent panel when it is hidden, at the end of every title bar. */
function AgentToggle() {
  const t = useT();
  const panel = usePanel();
  if (panel.open) return null;
  return (
    <Tooltip title={`${t('chatExpand')}  ${mac ? '⌘' : 'Ctrl+'}L`}>
      <IconButton aria-label={t('chatExpand')} onClick={panel.focus}>
        <AgentIcon fontSize="small" />
      </IconButton>
    </Tooltip>
  );
}

/** A page's title bar: title on the left, actions on the right. */
export function PageHeader({
  title,
  actions,
  children,
  leading,
}: {
  title: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
  /** Before the title, such as a back button. */
  leading?: ReactNode;
}) {
  return (
    <Box
      sx={{
        height: 52,
        px: 2.5,
        display: 'flex',
        alignItems: 'center',
        gap: 1.5,
        borderBottom: 1,
        borderColor: 'divider',
        flexShrink: 0,
        WebkitAppRegion: 'drag',
      }}
    >
      {leading && <Box sx={{ display: 'flex', ml: -1, mr: -0.5, WebkitAppRegion: 'no-drag' }}>{leading}</Box>}
      <Typography variant="h2" noWrap sx={{ flexShrink: 0, maxWidth: '45%', minWidth: 0 }}>
        {title}
      </Typography>
      {/* The status line gives way first: it ellipsizes, the title and the buttons never wrap. */}
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', overflow: 'hidden' }}>{children}</Box>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ flexShrink: 0, WebkitAppRegion: 'no-drag' }}>
        {actions}
        <AgentToggle />
      </Stack>
    </Box>
  );
}

/** A titled group of rows, as Settings and the phone page use. */
export function Section({
  title,
  description,
  action,
  children,
  sx,
}: {
  title?: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  children?: ReactNode;
  sx?: SxProps<Theme>;
}) {
  return (
    <Box sx={{ mb: 3, ...sx }}>
      {(title || action) && (
        <Stack direction="row" alignItems="flex-end" spacing={1} sx={{ mb: 1 }}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            {title && <Typography variant="h3">{title}</Typography>}
            {description && (
              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                {description}
              </Typography>
            )}
          </Box>
          {action}
        </Stack>
      )}
      {children}
    </Box>
  );
}

/** A bordered card of rows separated by lines. */
export function RowCard({ children, sx }: { children: ReactNode; sx?: SxProps<Theme> }) {
  return (
    <Paper
      variant="outlined"
      sx={{ borderRadius: 3, overflow: 'hidden', '& > *:not(:last-child)': { borderBottom: 1, borderColor: 'divider' }, ...sx }}
    >
      {children}
    </Paper>
  );
}

/** One row of a RowCard: a label and description on the left, a control on the right. */
export function Row({
  label,
  description,
  control,
  children,
  onClick,
}: {
  label: ReactNode;
  description?: ReactNode;
  control?: ReactNode;
  children?: ReactNode;
  onClick?: () => void;
}) {
  return (
    <Box
      onClick={onClick}
      sx={{ px: 2, py: 1.5, cursor: onClick ? 'pointer' : undefined, '&:hover': onClick ? { bgcolor: 'action.hover' } : {} }}
    >
      <Stack direction="row" alignItems="center" spacing={2}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="subtitle1">{label}</Typography>
          {description && (
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.25 }}>
              {description}
            </Typography>
          )}
        </Box>
        {control}
      </Stack>
      {children}
    </Box>
  );
}

export function EmptyState({ icon, title, children }: { icon?: ReactNode; title: ReactNode; children?: ReactNode }) {
  const p = useTheme().pinky;
  return (
    <Stack alignItems="center" justifyContent="center" spacing={1.5} sx={{ flex: 1, p: 4, textAlign: 'center', color: p.ink3 }}>
      {icon}
      <Typography variant="h3" color="text.primary">
        {title}
      </Typography>
      {children && (
        <Typography variant="body1" color="text.secondary" sx={{ maxWidth: 420 }}>
          {children}
        </Typography>
      )}
    </Stack>
  );
}

export function PlatformIcon({ platform, fontSize = 16 }: { platform: string; fontSize?: number }) {
  return platform === 'ios' ? <PhoneIphoneIcon sx={{ fontSize }} /> : <AndroidIcon sx={{ fontSize }} />;
}

/** Asks before something that cannot be undone. */
export function useConfirm(): [
  (o: { title: ReactNode; body?: ReactNode; confirm: string; cancel: string; danger?: boolean }) => Promise<boolean>,
  ReactNode,
] {
  const [state, setState] = useState<{
    title: ReactNode;
    body?: ReactNode;
    confirm: string;
    cancel: string;
    danger?: boolean;
    resolve: (v: boolean) => void;
  } | null>(null);
  const ask = (o: { title: ReactNode; body?: ReactNode; confirm: string; cancel: string; danger?: boolean }) =>
    new Promise<boolean>((resolve) => setState({ ...o, resolve }));
  const close = (v: boolean) => {
    state?.resolve(v);
    setState(null);
  };
  const dialog = (
    <Dialog open={!!state} onClose={() => close(false)} maxWidth="xs" fullWidth>
      {state && (
        <>
          <DialogTitle sx={{ fontSize: 15, fontWeight: 600 }}>{state.title}</DialogTitle>
          {state.body && (
            <DialogContent>
              <DialogContentText component="div" sx={{ fontSize: 13 }}>
                {state.body}
              </DialogContentText>
            </DialogContent>
          )}
          <DialogActions sx={{ px: 3, pb: 2 }}>
            <Button onClick={() => close(false)} color="inherit">
              {state.cancel}
            </Button>
            <Button onClick={() => close(true)} variant="contained" color={state.danger ? 'error' : 'primary'} autoFocus>
              {state.confirm}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
  return [ask, dialog];
}

/** The toasts, bottom centre, newest last. */
export function Toasts() {
  const { toasts, dismissToast } = useApp();
  const top = toasts[toasts.length - 1];
  return (
    <Snackbar
      key={top?.id}
      open={!!top}
      autoHideDuration={top?.severity === 'error' ? 8000 : 4000}
      onClose={(_, reason) => reason !== 'clickaway' && top && dismissToast(top.id)}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
    >
      {top ? (
        <Alert
          severity={top.severity}
          variant="filled"
          onClose={() => dismissToast(top.id)}
          sx={{ maxWidth: 560, fontSize: 13, alignItems: 'center' }}
        >
          {top.text}
        </Alert>
      ) : undefined}
    </Snackbar>
  );
}
