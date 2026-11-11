import { useState } from 'react';
import { Box, Button, ButtonBase, Collapse, IconButton, Stack, Tooltip, Typography, useTheme } from '@mui/material';
import ChevronRightIcon from '@mui/icons-material/ChevronRightRounded';
import CloseIcon from '@mui/icons-material/CloseRounded';
import { useT } from '@/i18n';
import type { Queued } from './model';

/** "Queued message" preview: one line. */
const preview = (s: string) => s.split(/\s+/).join(' ');

/** Messages typed while a run goes, above the composer, as Cursor's queue. */
export function QueueList({
  queued,
  onSteer,
  onRemove,
}: {
  queued: Queued[];
  onSteer: (i: number) => void;
  onRemove: (i: number) => void;
}) {
  const t = useT();
  const p = useTheme().pinky;
  const [open, setOpen] = useState(true);
  if (queued.length === 0) return null;
  return (
    <Box sx={{ border: 1, borderColor: p.line2, borderRadius: 2, bgcolor: 'background.paper', mb: 1, overflow: 'hidden' }}>
      <ButtonBase
        onClick={() => setOpen((o) => !o)}
        aria-label={t('chatQueueToggle')}
        aria-expanded={open}
        sx={{ width: '100%', justifyContent: 'flex-start', px: 1, py: 0.5, gap: 0.5, color: p.ink2 }}
      >
        <ChevronRightIcon sx={{ fontSize: 16, transition: 'transform .15s', transform: open ? 'rotate(90deg)' : 'none' }} />
        <Typography variant="caption" sx={{ fontWeight: 500, color: p.ink2 }}>
          {t('chatQueueHeading', { count: queued.length })}
        </Typography>
      </ButtonBase>
      <Collapse in={open}>
        <Stack sx={{ pb: 0.5 }}>
          {queued.map((q, i) => (
            <Stack key={i} direction="row" alignItems="center" spacing={0.5} sx={{ px: 1, py: 0.25, '&:hover .q-actions': { opacity: 1 } }}>
              <Box sx={{ width: 10, height: 10, borderRadius: '50%', border: 1.5, borderColor: p.ink3, flexShrink: 0, mx: 0.5 }} />
              <Typography variant="body2" noWrap sx={{ flex: 1, minWidth: 0 }}>
                {preview(q.text)}
              </Typography>
              <Stack direction="row" className="q-actions" sx={{ opacity: 0.7 }}>
                <Tooltip title={t('webChatSteerHint')}>
                  <Button size="small" onClick={() => onSteer(i)} sx={{ minHeight: 22, py: 0, px: 1, fontSize: 12 }}>
                    {t('chatQueueSteer')}
                  </Button>
                </Tooltip>
                <Tooltip title={t('chatQueueRemove')}>
                  <IconButton size="small" aria-label={t('chatQueueRemove')} onClick={() => onRemove(i)} sx={{ p: 0.25 }}>
                    <CloseIcon sx={{ fontSize: 14 }} />
                  </IconButton>
                </Tooltip>
              </Stack>
            </Stack>
          ))}
        </Stack>
      </Collapse>
    </Box>
  );
}
