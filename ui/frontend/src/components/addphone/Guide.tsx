// The pieces the Add phone steps are drawn with, after ui/guide.go: a
// numbered step that is done, now or later; text, notes, a waiting line,
// a problem box, and menu paths drawn as chips.

import type { ReactNode } from 'react';
import { Alert, AlertTitle, Box, Chip, CircularProgress, Stack, Typography, useTheme } from '@mui/material';
import CheckIcon from '@mui/icons-material/CheckRounded';
import ChevronIcon from '@mui/icons-material/ChevronRightRounded';

export type GuideState = 'done' | 'now' | 'later';

export function GuideStep({ n, title, state, children }: { n: number; title: ReactNode; state: GuideState; children?: ReactNode }) {
  const p = useTheme().pinky;
  return (
    <Stack direction="row" spacing={1.5} role="listitem" sx={{ opacity: state === 'later' ? 0.55 : 1 }}>
      <Box
        sx={{
          width: 24,
          height: 24,
          borderRadius: '50%',
          flexShrink: 0,
          display: 'grid',
          placeItems: 'center',
          fontSize: 12,
          fontWeight: 600,
          bgcolor: state === 'done' ? p.green : state === 'now' ? 'primary.main' : p.bg3,
          color: state === 'later' ? p.ink2 : '#fff',
        }}
      >
        {state === 'done' ? <CheckIcon sx={{ fontSize: 16 }} /> : n}
      </Box>
      <Box sx={{ flex: 1, minWidth: 0, pt: 0.25 }}>
        <Typography variant="subtitle1" sx={{ fontSize: 15 }}>
          {title}
        </Typography>
        {state !== 'done' && children && (
          <Stack spacing={1} sx={{ mt: 1 }}>
            {children}
          </Stack>
        )}
      </Box>
    </Stack>
  );
}

export function GuideText({ children }: { children: ReactNode }) {
  return (
    <Typography variant="body1" color="text.secondary" sx={{ lineHeight: 1.55 }}>
      {children}
    </Typography>
  );
}

export function GuideNote({ children }: { children: ReactNode }) {
  const p = useTheme().pinky;
  return (
    <Typography variant="body2" sx={{ color: p.ink3, lineHeight: 1.5 }}>
      {children}
    </Typography>
  );
}

export function GuideWaiting({ children }: { children: ReactNode }) {
  return (
    <Stack direction="row" spacing={1} alignItems="center" role="status">
      <CircularProgress size={14} thickness={5} />
      <Typography variant="body2" color="text.secondary">
        {children}
      </Typography>
    </Stack>
  );
}

export function GuideProblem({ title, detail, children }: { title: ReactNode; detail?: ReactNode; children?: ReactNode }) {
  return (
    <Alert severity="warning" sx={{ fontSize: 13 }}>
      <AlertTitle sx={{ fontSize: 13, fontWeight: 500, mb: detail ? 0.5 : 0 }}>{title}</AlertTitle>
      {detail}
      {children}
    </Alert>
  );
}

/** "Settings › About phone" as chips, the last one the item to find. */
export function MenuPath({ path, item }: { path: string; item?: string }) {
  const p = useTheme().pinky;
  const parts = path
    .split('›')
    .map((s) => s.trim())
    .filter(Boolean);
  if (item) parts.push(item);
  return (
    <Stack direction="row" alignItems="center" flexWrap="wrap" useFlexGap spacing={0.25}>
      {parts.map((part, i) => (
        <Stack key={i} direction="row" alignItems="center">
          {i > 0 && <ChevronIcon sx={{ fontSize: 16, color: p.ink3 }} />}
          <Chip
            label={part}
            variant={i === parts.length - 1 ? 'filled' : 'outlined'}
            color={i === parts.length - 1 ? 'primary' : 'default'}
            sx={{ fontWeight: 500 }}
          />
        </Stack>
      ))}
    </Stack>
  );
}

/** The heading of a page of steps. */
export function GuideHeading({ title, detail }: { title: ReactNode; detail?: ReactNode }) {
  return (
    <Box sx={{ mb: 2.5 }}>
      <Typography variant="h1">{title}</Typography>
      {detail && (
        <Typography variant="body1" color="text.secondary" sx={{ mt: 1 }}>
          {detail}
        </Typography>
      )}
    </Box>
  );
}
