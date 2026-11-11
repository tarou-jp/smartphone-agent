// Small pieces every page uses.

import { useState, type ReactNode } from 'react';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
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
import { topRow } from '@/lib/window';
import DoneIcon from '@mui/icons-material/CheckCircleOutlineRounded';
import InfoIcon from '@mui/icons-material/InfoOutlined';
import WarnIcon from '@mui/icons-material/WarningAmberRounded';
import ErrorIcon from '@mui/icons-material/ErrorOutlineRounded';
import CloseIcon from '@mui/icons-material/CloseRounded';

export { StatusDot } from './StatusDot';
export { StatusText } from './StatusText';
export { PhoneGlyph, Keys } from './PhoneGlyph';

const mac = typeof navigator !== 'undefined' && navigator.platform.startsWith('Mac');

/** Shows the Agent panel when it is hidden, at the end of every page's top row. */
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

/**
 * A page's top row: its title on the left, its actions on the right, and a
 * line under it that runs on into the chat panel's. The window has no
 * title bar of its own, so this row is also what the window is dragged by
 * (lib/window.ts).
 */
export function PageHeader({
  title,
  actions,
  children,
  noAgent,
}: {
  title: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
  /** A page without the Agent panel, such as Settings: no button to show it. */
  noAgent?: boolean;
}) {
  return (
    <Box
      sx={{
        height: topRow,
        px: 2.5,
        display: 'flex',
        alignItems: 'center',
        gap: 1.5,
        flexShrink: 0,
        cursor: 'default',
        borderBottom: 1,
        borderColor: 'divider',
      }}
    >
      <Typography noWrap sx={{ flexShrink: 0, maxWidth: '45%', minWidth: 0, fontSize: 14, fontWeight: 600 }}>
        {title}
      </Typography>
      {/* The status line gives way first: it ellipsizes, the title and the buttons never wrap. */}
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', overflow: 'hidden' }}>{children}</Box>
      <Stack
        direction="row"
        spacing={1}
        alignItems="center"
        sx={{ flexShrink: 0, '& .MuiButton-root': { minHeight: 26, py: 0, fontSize: 12.5 }, '& .MuiIconButton-root': { p: '3px' } }}
      >
        {actions}
        {!noAgent && <AgentToggle />}
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
    <Box sx={{ mb: 6, ...sx }}>
      {(title || action) && (
        <Stack direction="row" alignItems="flex-end" spacing={1} sx={{ mb: 2 }}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            {title && <Typography variant="h2">{title}</Typography>}
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

/**
 * The rows of a settings group, in a card: a rounded, bordered surface that
 * holds them apart from the page, with a hairline between rows. Every
 * settings page uses it, so groups look alike everywhere.
 */
export function RowCard({ children, sx }: { children: ReactNode; sx?: SxProps<Theme> }) {
  return <CardRows sx={sx}>{children}</CardRows>;
}

/**
 * Rows in a card: a rounded, bordered surface that holds a group of rows
 * apart from the page, as the phones in Settings › Phones are.
 */
export function CardRows({ children, sx }: { children: ReactNode; sx?: SxProps<Theme> }) {
  return (
    <Box
      sx={{
        border: 1,
        borderColor: 'divider',
        borderRadius: '12px',
        bgcolor: 'background.paper',
        overflow: 'hidden',
        '& > *:not(:last-child)': { borderBottom: 1, borderColor: 'divider' },
        ...sx,
      }}
    >
      {children}
    </Box>
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
      sx={{ px: 2, py: 2, cursor: onClick ? 'pointer' : undefined, '&:hover': onClick ? { bgcolor: 'action.hover' } : {} }}
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
    <Stack alignItems="center" justifyContent="center" spacing={1.5} sx={{ flex: 1, p: 4, textAlign: 'center', color: p.ink3, '& > svg': { fontSize: 40, mb: 0.5 } }}>
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

/**
 * The toasts, at the top right under the window's top row, newest on top:
 * a quiet card with the colour only in its icon, gone after a few seconds
 * (AppState's toast keeps the time).
 */
export function Toasts() {
  const { toasts, dismissToast } = useApp();
  const p = useTheme().pinky;
  const tone = { success: p.green, info: p.ink2, warning: p.amberDot, error: p.red };
  const Icon = { success: DoneIcon, info: InfoIcon, warning: WarnIcon, error: ErrorIcon };
  return (
    <Stack
      spacing={1}
      aria-live="polite"
      sx={{ position: 'fixed', top: topRow + 12, right: 12, zIndex: (th) => th.zIndex.snackbar, width: 360, maxWidth: 'calc(100vw - 24px)', pointerEvents: 'none' }}
    >
      {[...toasts].reverse().map((x) => {
        const I = Icon[x.severity];
        return (
          <Stack
            key={x.id}
            role={x.severity === 'error' ? 'alert' : 'status'}
            direction="row"
            spacing={1.25}
            alignItems="flex-start"
            sx={{
              pointerEvents: 'auto',
              pl: 1.5,
              pr: 0.5,
              py: 1.25,
              borderRadius: '10px',
              bgcolor: 'background.paper',
              border: 1,
              borderColor: 'divider',
              boxShadow: p.floatShadow,
              animation: 'pinky-toast-in .2s cubic-bezier(0.22, 1, 0.36, 1)',
              '@keyframes pinky-toast-in': { from: { opacity: 0, transform: 'translateY(-6px)' } },
            }}
          >
            <I sx={{ fontSize: 18, color: tone[x.severity], mt: '1px', flexShrink: 0 }} />
            <Typography sx={{ flex: 1, minWidth: 0, fontSize: 13, lineHeight: 1.5 }} className="selectable">
              {x.text}
            </Typography>
            <IconButton aria-label="close" onClick={() => dismissToast(x.id)} sx={{ p: '2px', mt: '-1px' }}>
              <CloseIcon sx={{ fontSize: 16 }} />
            </IconButton>
          </Stack>
        );
      })}
    </Stack>
  );
}
