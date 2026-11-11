import { useState } from 'react';
import { Stack, Typography, useTheme } from '@mui/material';
import type { Need } from '@/api/types';
import { useT } from '@/i18n';
import { NeedCard } from './NeedCard';

// Unlock cards the person put off with Not now, until the window reloads.
const later = new Set<string>();

/** The Needs you section: oldest first, as the gateway asked. */
export function NeedsList({ needs, shotFor }: { needs: Need[]; shotFor?: (n: Need) => boolean }) {
  const t = useT();
  const p = useTheme().pinky;
  const [, setTick] = useState(0);
  const shown = needs
    .filter((n) => !(n.kind === 'unlock' && later.has(n.id)))
    .sort((a, b) => a.created.localeCompare(b.created));
  if (shown.length === 0) return null;
  return (
    <Stack spacing={1.25}>
      <Typography sx={{ fontSize: 12, color: p.ink3 }}>{t('needsSection')}</Typography>
      {shown.map((n) => (
        <NeedCard
          key={n.id}
          need={n}
          showShot={shotFor ? shotFor(n) : true}
          onLater={() => {
            later.add(n.id);
            setTick((x) => x + 1);
          }}
        />
      ))}
    </Stack>
  );
}
