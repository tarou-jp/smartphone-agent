import { useState } from 'react';
import { Box, useTheme } from '@mui/material';
import FlowsIcon from '@mui/icons-material/AccountTreeOutlined';
import { api } from '@/api/client';
import { useT } from '@/i18n';

// Tile colors for apps without an icon, picked by the app's id so an app
// keeps its color.
const tiles = ['#5450d6', '#c2410c', '#0f766e', '#a21caf', '#1d4ed8', '#b45309', '#15803d', '#be123c'];

function hash(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
  return Math.abs(h);
}

/** The letter an app's tile shows: the first letter of its id's last word. */
function letter(app: string): string {
  const words = app.split('.').filter((w) => w && !/^(com|app|net|org|io|jp|cn|co|android|ios|www)$/i.test(w));
  const w = words[0] ?? app;
  return (w[0] ?? '?').toUpperCase();
}

/**
 * An app's icon, from a phone that has the app or the App Store; a letter
 * tile while it loads or when nobody has it. app "" is the flows of no app.
 */
export function AppIcon({ app, size = 32 }: { app: string; size?: number }) {
  const t = useT();
  const p = useTheme().pinky;
  const [failed, setFailed] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const radius = Math.round(size * 0.24);
  const box = {
    width: size,
    height: size,
    flexShrink: 0,
    borderRadius: `${radius}px`,
    overflow: 'hidden',
    display: 'grid',
    placeItems: 'center',
    position: 'relative' as const,
  };
  if (!app) {
    return (
      <Box sx={{ ...box, bgcolor: p.bg3, color: p.ink3 }} aria-hidden>
        <FlowsIcon sx={{ fontSize: size * 0.56 }} />
      </Box>
    );
  }
  return (
    <Box
      sx={{
        ...box,
        bgcolor: loaded ? 'transparent' : tiles[hash(app) % tiles.length],
        boxShadow: loaded ? `inset 0 0 0 1px ${p.line}` : 'none',
        color: '#fff',
        fontSize: size * 0.46,
        fontWeight: 600,
        lineHeight: 1,
      }}
    >
      {!loaded && <span aria-hidden>{letter(app)}</span>}
      {!failed && (
        <Box
          component="img"
          src={api.appIconURL(app)}
          alt={t('flowsAppIcon', { app })}
          loading="lazy"
          draggable={false}
          onLoad={() => setLoaded(true)}
          onError={() => setFailed(true)}
          sx={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover', opacity: loaded ? 1 : 0 }}
        />
      )}
    </Box>
  );
}
