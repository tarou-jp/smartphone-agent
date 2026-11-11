import { useState } from 'react';
import { Box, ButtonBase, Dialog, IconButton, Tooltip, useTheme } from '@mui/material';
import CloseIcon from '@mui/icons-material/CloseRounded';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { Step, Task } from '@/api/types';

/**
 * The target's box on a screenshot, as fractions of it. The gateway scales
 * screenshots down by a whole factor, which the phone's size gives.
 */
function overlayBox(step: Step, img: { w: number; h: number }, dev?: { w: number; h: number }) {
  if (!step.target || !dev || dev.h === 0 || img.h === 0) return null;
  const k = Math.max(1, Math.round(dev.h / img.h));
  const sw = img.w * k;
  const sh = img.h * k;
  return {
    left: (step.target.x / sw) * 100,
    top: (step.target.y / sh) * 100,
    width: (step.target.w / sw) * 100,
    height: (step.target.h / sh) * 100,
  };
}

function Shot({ task, step, height, overlay }: { task: Task; step: Step; height: number; overlay: boolean }) {
  const p = useTheme().pinky;
  const { sizes } = useApp();
  const t = useT();
  const id = step.after || step.before;
  const [img, setImg] = useState<{ w: number; h: number } | null>(null);
  const [failed, setFailed] = useState(false);
  if (!id) return null;
  const box = overlay && img ? overlayBox(step, img, sizes[task.deviceId]) : null;
  return (
    <Box
      sx={{
        position: 'relative',
        height,
        width: img ? (height * img.w) / img.h : height * 0.46,
        borderRadius: '8px',
        overflow: 'hidden',
        // The screen as a small phone: a dark bezel around it.
        boxShadow: `0 0 0 3px ${p.phone}, 0 0 0 4px ${p.line2}`,
        bgcolor: failed ? p.bg3 : '#000',
        flexShrink: 0,
      }}
    >
      {!failed && (
        <Box
          component="img"
          src={api.mediaURL(id)}
          alt={t('activityStepShotLabel', { step: step.text.split('\n')[0] })}
          onLoad={(e: React.SyntheticEvent<HTMLImageElement>) =>
            setImg({ w: e.currentTarget.naturalWidth, h: e.currentTarget.naturalHeight })
          }
          onError={() => setFailed(true)}
          sx={{ height: '100%', width: '100%', objectFit: 'contain', display: 'block' }}
        />
      )}
      {box && (
        <Box
          sx={{
            position: 'absolute',
            left: `${box.left}%`,
            top: `${box.top}%`,
            width: `${box.width}%`,
            height: `${box.height}%`,
            border: `2px solid ${p.overlay}`,
            borderRadius: 1,
            boxShadow: `0 0 0 9999px rgba(0,0,0,0.12)`,
          }}
        />
      )}
    </Box>
  );
}

/** A step's screenshot: a thumbnail that opens larger. */
export function StepShot({ task, step }: { task: Task; step: Step }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  if (!step.after && !step.before) return null;
  return (
    <>
      <Tooltip title={t('shotEnlarge')}>
        <ButtonBase onClick={() => setOpen(true)} sx={{ borderRadius: '8px', m: '4px' }} aria-label={t('activityShowShot')}>
          <Shot task={task} step={step} height={88} overlay={false} />
        </ButtonBase>
      </Tooltip>
      <Dialog open={open} onClose={() => setOpen(false)} maxWidth={false} PaperProps={{ sx: { bgcolor: 'transparent', boxShadow: 'none' } }}>
        <Box sx={{ position: 'relative' }}>
          <Shot task={task} step={step} height={Math.min(760, window.innerHeight - 96)} overlay />
          <IconButton
            onClick={() => setOpen(false)}
            aria-label={t('shotClose')}
            sx={{ position: 'absolute', top: 8, right: 8, bgcolor: 'rgba(0,0,0,0.55)', color: '#fff', '&:hover': { bgcolor: 'rgba(0,0,0,0.7)' } }}
          >
            <CloseIcon fontSize="small" />
          </IconButton>
        </Box>
      </Dialog>
    </>
  );
}
