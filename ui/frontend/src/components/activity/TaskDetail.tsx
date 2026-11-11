import { useState } from 'react';
import { Box, Button, IconButton, Stack, Tab, Tabs, Tooltip, Typography } from '@mui/material';
import CloseIcon from '@mui/icons-material/CloseRounded';
import { EmptyState, StatusText } from '@/components/common';
import { useApp, useHas } from '@/state/AppState';
import { useI18n } from '@/i18n';
import { navigate } from '@/lib/router';
import { day, clock, duration } from '@/lib/format';
import type { Task } from '@/api/types';
import { approvals, outcomeKind, outcomeLabel } from './outcome';
import { OutcomeDot } from './TaskRow';
import { StepItem } from './StepItem';
import { RecordingPlayer } from './RecordingPlayer';

/** Activity's right column: a task's steps and recording. */
export function TaskDetail({ task, onExport }: { task?: Task; onExport: (ids: string[]) => void }) {
  const { t, locale } = useI18n();
  const { settings } = useApp();
  const hasRec = useHas('recordings');
  const hasExport = useHas('export');
  const [tab, setTab] = useState<'steps' | 'recording'>('steps');
  if (!task) return <EmptyState title={t('activityChooseTask')} />;
  const k = outcomeKind(task);
  const n = approvals(task);
  const end = task.ended ? new Date(task.ended).getTime() : Date.now();
  const took = end - new Date(task.started).getTime();
  return (
    <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }} aria-label={t('activityDetailLabel')}>
      <Box sx={{ px: 3, pt: 2.5, pb: 1.5, borderBottom: 1, borderColor: 'divider' }}>
        <Stack direction="row" alignItems="flex-start" spacing={1}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="h2" className="selectable">
              {task.title}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
              {[task.agent, task.deviceName, `${day(t, task.started)} ${clock(locale, task.started)}`, duration(t, took)]
                .filter(Boolean)
                .join(' · ')}
            </Typography>
          </Box>
          {hasExport && task.ended && (
            <Button variant="outlined" color="inherit" onClick={() => onExport([task.id])}>
              {t('exportTask')}
            </Button>
          )}
          <Tooltip title={t('activityPanelClose')}>
            <IconButton aria-label={t('activityPanelClose')} onClick={() => navigate({ page: 'activity' })}>
              <CloseIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Stack>
        <Stack direction="row" spacing={2} alignItems="center" sx={{ mt: 1 }}>
          <StatusText dot={<OutcomeDot task={task} />}>{outcomeLabel(t, k)}</StatusText>
          {n > 0 && <StatusText>{t('activityApprovals', { count: n })}</StatusText>}
          {task.readOnly && <StatusText>{t('tasksReadOnly')}</StatusText>}
        </Stack>
        {hasRec && task.ended && (
          <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mt: 1 }}>
            <Tab value="steps" label={t('activitySteps')} />
            <Tab value="recording" label={t('recordingWatch')} />
          </Tabs>
        )}
      </Box>
      <Box sx={{ flex: 1, overflowY: 'auto', px: 3, py: 1 }}>
        {tab === 'recording' && hasRec && task.ended ? (
          <Box sx={{ py: 1.5 }}>
            <RecordingPlayer task={task} />
          </Box>
        ) : (
          task.steps.map((s) => <StepItem key={s.index} task={task} step={s} developer={!!settings?.developerMode} />)
        )}
      </Box>
    </Box>
  );
}
