import { Box, useTheme } from '@mui/material';
import type { DeviceStatus } from '@/api/types';

/** The sidebar's dot: amber for Needs you, indigo while an agent works. */
export function StatusDot({ status, size = 8 }: { status: DeviceStatus; size?: number }) {
  const p = useTheme().pinky;
  const color: Record<DeviceStatus, string> = {
    needs_you: p.amberDot,
    needs_setup: p.amberDot,
    locked: p.ink3,
    in_control: p.overlay,
    working: p.overlay,
    idle: p.green,
    offline: 'transparent',
  };
  return (
    <Box
      component="span"
      aria-hidden
      sx={{
        width: size,
        height: size,
        borderRadius: '50%',
        flexShrink: 0,
        display: 'inline-block',
        background: color[status],
        border: status === 'offline' ? `1.5px solid ${p.ink3}` : 'none',
        animation: status === 'working' ? 'pinky-pulse 1.6s ease-in-out infinite' : 'none',
        '@keyframes pinky-pulse': { '50%': { opacity: 0.45 } },
      }}
    />
  );
}
