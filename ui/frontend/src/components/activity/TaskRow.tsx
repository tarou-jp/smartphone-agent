import { Box, ListItemButton, Stack, Typography, useTheme } from '@mui/material';
import { useI18n } from '@/i18n';
import { href } from '@/lib/router';
import { clock } from '@/lib/format';
import type { Task } from '@/api/types';
import { approvals, outcomeColor, outcomeKind, outcomeLabel } from './outcome';

export function OutcomeDot({ task }: { task: Task }) {
  const p = useTheme().pinky;
  const k = outcomeKind(task);
  return (
    <Box
      component="span"
      aria-hidden
      sx={{
        width: 8,
        height: 8,
        borderRadius: '50%',
        flexShrink: 0,
        display: 'inline-block',
        bgcolor: outcomeColor(p, k),
        border: k === 'denied' ? `1.5px solid ${p.ink3}` : 'none',
      }}
    />
  );
}

/** A task in Activity's list. */
export function TaskRow({ task, selected }: { task: Task; selected: boolean }) {
  const { t, locale } = useI18n();
  const n = approvals(task);
  return (
    <ListItemButton component="a" href={href({ page: 'activity', taskId: task.id })} selected={selected} sx={{ py: 1, px: 1.5, alignItems: 'flex-start' }}>
      <Box sx={{ pt: '6px', pr: 1.25 }}>
        <OutcomeDot task={task} />
      </Box>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Stack direction="row" spacing={1} alignItems="baseline">
          <Typography variant="subtitle1" noWrap sx={{ flex: 1, minWidth: 0 }}>
            {task.title}
          </Typography>
          <Typography variant="caption" sx={{ flexShrink: 0 }}>
            {clock(locale, task.ended ?? task.started)}
          </Typography>
        </Stack>
        <Typography variant="body2" color="text.secondary" noWrap>
          {[task.agent, task.deviceName].filter(Boolean).join(' · ')}
        </Typography>
        <Typography variant="caption" noWrap component="div">
          {outcomeLabel(t, outcomeKind(task))}
          {n > 0 && ` · ${t('activityApprovals', { count: n })}`}
        </Typography>
      </Box>
    </ListItemButton>
  );
}
