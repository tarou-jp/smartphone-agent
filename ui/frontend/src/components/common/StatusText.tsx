import type { ReactNode } from 'react';
import { Box, useTheme } from '@mui/material';

export type Tone = 'good' | 'warn' | 'bad' | 'agent' | 'muted' | 'none';

/**
 * A state or a count as quiet text, with a small dot when the state has a
 * meaning (good, needs a look, failed, the agent at work). Used instead of
 * outlined chips, which made every label look like a button.
 */
export function StatusText({ tone = 'none', dot, children }: { tone?: Tone; dot?: ReactNode; children: ReactNode }) {
  const p = useTheme().pinky;
  const color: Record<Tone, string> = {
    good: p.green,
    warn: p.amberDot,
    bad: p.red,
    agent: p.overlay,
    muted: p.ink3,
    none: 'transparent',
  };
  return (
    <Box
      component="span"
      sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.75, fontSize: 13, lineHeight: '18px', color: 'text.secondary', whiteSpace: 'nowrap' }}
    >
      {dot ??
        (tone !== 'none' && (
          <Box component="span" aria-hidden sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: color[tone], flexShrink: 0 }} />
        ))}
      {children}
    </Box>
  );
}
