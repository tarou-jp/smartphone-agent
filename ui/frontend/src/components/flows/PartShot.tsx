import { useState } from 'react';
import { Box, ButtonBase, Dialog, Tooltip, useTheme } from '@mui/material';
import ImageIcon from '@mui/icons-material/ImageOutlined';
import { api } from '@/api/client';
import { useT } from '@/i18n';

/**
 * The picture of the screen a part leads to, at width; a click shows it
 * large. Without a picture, an empty frame of the same shape says one
 * comes with the next task that does the step.
 */
export function PartShot({ part, shot, width = 96, caption }: { part: string; shot?: boolean; width?: number; caption?: string }) {
  const t = useT();
  const p = useTheme().pinky;
  const [open, setOpen] = useState(false);
  const [broken, setBroken] = useState(false);
  const frame = {
    width,
    aspectRatio: '9 / 19.5',
    flexShrink: 0,
    borderRadius: '10px',
    overflow: 'hidden',
    border: 1,
    borderColor: 'divider',
    bgcolor: p.bg2,
  };
  if (!shot || broken) {
    return (
      <Tooltip title={t('flowsNoShot')}>
        <Box sx={{ ...frame, display: 'grid', placeItems: 'center', color: p.ink3, borderStyle: 'dashed' }}>
          <ImageIcon sx={{ fontSize: Math.min(28, width * 0.3), opacity: 0.6 }} />
        </Box>
      </Tooltip>
    );
  }
  const src = api.partShotURL(part);
  return (
    <>
      <ButtonBase
        onClick={() => setOpen(true)}
        aria-label={t('flowsShotAlt')}
        sx={{ ...frame, display: 'block', transition: 'transform 120ms, box-shadow 120ms', '&:hover': { transform: 'translateY(-1px)', boxShadow: p.cardShadow } }}
      >
        <Box
          component="img"
          src={src}
          alt={t('flowsShotAlt')}
          loading="lazy"
          draggable={false}
          onError={() => setBroken(true)}
          sx={{ width: '100%', height: '100%', objectFit: 'cover', objectPosition: 'top', display: 'block' }}
        />
      </ButtonBase>
      <Dialog open={open} onClose={() => setOpen(false)} PaperProps={{ sx: { bgcolor: 'transparent', boxShadow: 'none', m: 2, overflow: 'visible' } }}>
        <Box onClick={() => setOpen(false)} sx={{ cursor: 'zoom-out', textAlign: 'center' }}>
          <Box
            component="img"
            src={src}
            alt={t('flowsShotAlt')}
            sx={{
              height: caption ? 'min(calc(100vh - 170px), 860px)' : 'min(calc(100vh - 120px), 900px)',
              width: 'auto',
              maxWidth: 'calc(100vw - 64px)',
              objectFit: 'contain',
              borderRadius: '18px',
              display: 'block',
              mx: 'auto',
              boxShadow: p.floatShadow,
            }}
          />
          {caption && (
            <Box className="selectable" sx={{ mt: 1.5, color: '#fff', fontFamily: p.mono, fontSize: 12.5, maxWidth: 420, mx: 'auto', textShadow: '0 1px 2px rgba(0,0,0,.6)' }}>
              {caption}
            </Box>
          )}
        </Box>
      </Dialog>
    </>
  );
}
