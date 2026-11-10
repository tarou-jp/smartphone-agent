import { useEffect, useRef, useState } from 'react';
import { Alert, Box, List, ListItemButton, ListItemText, Typography } from '@mui/material';
import { api, ApiError } from '@/api/client';
import { useT } from '@/i18n';
import type { RecordingInfo, Task } from '@/api/types';

const at = (ms: number) => {
  const s = Math.floor(ms / 1000);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
};

/** A task's screen recording, with its steps to jump to. */
export function RecordingPlayer({ task }: { task: Task }) {
  const t = useT();
  const video = useRef<HTMLVideoElement>(null);
  const [rec, setRec] = useState<RecordingInfo | null>(null);
  const [err, setErr] = useState('');
  useEffect(() => {
    setRec(null);
    setErr('');
    api.recording(task.id).then(setRec, (e) => setErr(e instanceof ApiError && e.status === 404 ? 'gone' : String(e)));
  }, [task.id]);
  if (err === 'gone') return null;
  if (err) return <Alert severity="error">{t('recordingPlayFailed', { detail: err })}</Alert>;
  if (!rec) return null;
  if (!rec.hasFile)
    return (
      <Typography variant="body2" color="text.secondary">
        {t('recordingUnsupported')}
      </Typography>
    );
  return (
    <Box aria-label={t('recordingPanelLabel')}>
      <Box
        component="video"
        ref={video}
        controls
        preload="metadata"
        src={api.recordingURL(task.id)}
        aria-label={t('recordingPlayerLabel', { title: task.title })}
        onError={() => setErr(t('recordingGone'))}
        sx={{ width: '100%', maxHeight: 460, bgcolor: '#000', borderRadius: 2.5, display: 'block' }}
      />
      <Typography variant="subtitle2" sx={{ mt: 1.5 }}>
        {t('recordingJumpHeading')}
      </Typography>
      {rec.marks.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          {t('recordingNoMarks')}
        </Typography>
      ) : (
        <List dense aria-label={t('recordingStepsLabel')}>
          {rec.marks.map((m) => (
            <ListItemButton
              key={`${m.step}-${m.atMs}`}
              aria-label={t('recordingJumpTo', { at: at(m.atMs), step: m.text })}
              onClick={() => {
                if (video.current) {
                  video.current.currentTime = m.atMs / 1000;
                  void video.current.play();
                }
              }}
            >
              <ListItemText
                primary={m.text}
                secondary={at(m.atMs)}
                primaryTypographyProps={{ fontSize: 13 }}
                secondaryTypographyProps={{ fontSize: 11 }}
              />
            </ListItemButton>
          ))}
        </List>
      )}
    </Box>
  );
}
