import { Box, useTheme } from '@mui/material';
import type { DeviceStatus } from '@/api/types';

/**
 * Pinky's signature: a small phone whose screen lights up in the color of
 * what is happening on it. Indigo while an agent works (breathing, since
 * it is live), amber when it needs you, a dim screen when idle, an empty
 * dashed outline when it is not connected. Used wherever a phone is named:
 * the sidebar, the page header and the chat's "goes to" list.
 */
export function PhoneGlyph({ status, size = 22 }: { status: DeviceStatus; size?: number }) {
  const p = useTheme().pinky;
  const w = Math.round(size * 0.64);
  const screen: Record<DeviceStatus, string> = {
    needs_you: p.amberDot,
    needs_setup: p.amberDot,
    locked: p.line2,
    in_control: p.overlay,
    working: p.overlay,
    idle: p.line2,
    offline: 'transparent',
  };
  const live = status === 'working';
  return (
    <Box
      component="span"
      aria-hidden
      sx={{
        width: w,
        height: size,
        flexShrink: 0,
        display: 'inline-flex',
        boxSizing: 'border-box',
        borderRadius: `${Math.max(3, Math.round(size / 6))}px`,
        border: `1.5px ${status === 'offline' ? 'dashed' : 'solid'} ${status === 'offline' ? p.ink3 : p.ink2}`,
        p: '2px',
      }}
    >
      <Box
        component="span"
        sx={{
          flex: 1,
          borderRadius: `${Math.max(1, Math.round(size / 10))}px`,
          bgcolor: screen[status],
          animation: live ? 'pinky-breathe 1.8s cubic-bezier(0.22, 1, 0.36, 1) infinite' : 'none',
          '@keyframes pinky-breathe': { '50%': { opacity: 0.45 } },
        }}
      />
    </Box>
  );
}

/** A keyboard shortcut as small keycaps, in the mono face. */
export function Keys({ keys }: { keys: string[] }) {
  const p = useTheme().pinky;
  return (
    <Box component="span" sx={{ display: 'inline-flex', gap: '3px' }}>
      {keys.map((k) => (
        <Box
          key={k}
          component="kbd"
          sx={{
            fontFamily: p.mono,
            fontSize: 11,
            lineHeight: '16px',
            minWidth: 18,
            px: '4px',
            textAlign: 'center',
            borderRadius: '4px',
            border: `1px solid ${p.line2}`,
            borderBottomWidth: 2,
            color: 'inherit',
            opacity: 0.85,
          }}
        >
          {k}
        </Box>
      ))}
    </Box>
  );
}
