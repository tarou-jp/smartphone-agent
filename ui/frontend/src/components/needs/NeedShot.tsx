import { useState } from 'react';
import { Box, Dialog, useTheme } from '@mui/material';
import { api } from '@/api/client';
import type { Need } from '@/api/types';
import { useT } from '@/i18n';
import { TargetOverlay } from '@/components/phone/TargetOverlay';

/** The approval's screenshot with the target marked; click to enlarge. */
export function NeedShot({ need, height = 220 }: { need: Need; height?: number }) {
  const t = useT();
  const p = useTheme().pinky;
  const [open, setOpen] = useState(false);
  if (!need.hasScreenshot || !need.shotW || !need.shotH) return null;
  const w = (height * need.shotW) / need.shotH;
  const label = t('panelShotLabel', { phone: need.deviceName });
  const figure = (h: number) => (
    <Box sx={{ position: 'relative', width: (h * need.shotW!) / need.shotH!, height: h, borderRadius: 2, overflow: 'hidden', bgcolor: p.phone }}>
      <img src={api.needShotURL(need.id)} alt={label} style={{ width: '100%', height: '100%', display: 'block' }} />
      <TargetOverlay w={need.shotW!} h={need.shotH!} target={need.target} point={need.point} to={need.to} />
    </Box>
  );
  return (
    <>
      <Box
        component="button"
        onClick={() => setOpen(true)}
        aria-label={t('shotEnlarge')}
        sx={{ p: 0, border: `1px solid ${p.line2}`, borderRadius: 2, cursor: 'zoom-in', bgcolor: 'transparent', width: w, alignSelf: 'center' }}
      >
        {figure(height)}
      </Box>
      <Dialog open={open} onClose={() => setOpen(false)} maxWidth={false}>
        <Box onClick={() => setOpen(false)} sx={{ p: 1, cursor: 'zoom-out' }}>
          {figure(Math.min(window.innerHeight * 0.85, need.shotH))}
        </Box>
      </Dialog>
    </>
  );
}
