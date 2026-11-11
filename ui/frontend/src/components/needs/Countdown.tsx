import { useEffect, useState } from 'react';
import { Typography, useTheme } from '@mui/material';
import { useT } from '@/i18n';
import { duration } from '@/lib/format';

/** "Denies in 42 s": how long until no answer counts as Deny. */
export function Countdown({ deadline, words = 'panelDeniesIn' }: { deadline: string; words?: string }) {
  const t = useT();
  const p = useTheme().pinky;
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);
  const left = new Date(deadline).getTime() - now;
  if (left <= 0) return null;
  return (
    <Typography variant="caption" sx={{ fontFamily: p.mono, fontSize: 12, whiteSpace: 'nowrap' }}>
      {t(words, { duration: duration(t, left) })}
    </Typography>
  );
}
