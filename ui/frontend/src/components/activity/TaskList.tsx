import { Box, Button, CircularProgress, List, ListSubheader, Typography } from '@mui/material';
import { useT } from '@/i18n';
import { day } from '@/lib/format';
import type { Task } from '@/api/types';
import { TaskRow } from './TaskRow';

/** Tasks grouped by the day they ended, newest first. */
export function TaskList({
  running,
  tasks,
  selected,
  more,
  loading,
  onMore,
  empty,
}: {
  running: Task[];
  tasks: Task[];
  selected?: string;
  more: boolean;
  loading: boolean;
  onMore: () => void;
  empty: string;
}) {
  const t = useT();
  const groups: { day: string; tasks: Task[] }[] = [];
  for (const task of tasks) {
    const d = day(t, task.ended ?? task.started);
    const last = groups[groups.length - 1];
    if (last && last.day === d) last.tasks.push(task);
    else groups.push({ day: d, tasks: [task] });
  }
  const header = (label: string) => (
    <ListSubheader sx={{ lineHeight: '30px', fontSize: 12, fontWeight: 500, bgcolor: 'background.default', px: 1.5 }}>
      {label}
    </ListSubheader>
  );
  return (
    <Box sx={{ flex: 1, overflowY: 'auto', px: 1, pb: 2 }} aria-label={t('activityTasksLabel')}>
      {running.length > 0 && (
        <List dense subheader={header(t('tasksStatusWorking'))}>
          {running.map((x) => (
            <TaskRow key={x.id} task={x} selected={x.id === selected} />
          ))}
        </List>
      )}
      {groups.map((g) => (
        <List dense key={g.day} subheader={header(g.day)}>
          {g.tasks.map((x) => (
            <TaskRow key={x.id} task={x} selected={x.id === selected} />
          ))}
        </List>
      ))}
      {!loading && tasks.length === 0 && running.length === 0 && (
        <Typography variant="body1" color="text.secondary" sx={{ p: 2, textAlign: 'center' }}>
          {empty}
        </Typography>
      )}
      {loading && (
        <Box sx={{ display: 'flex', justifyContent: 'center', p: 2 }}>
          <CircularProgress size={20} />
        </Box>
      )}
      {more && !loading && (
        <Box sx={{ display: 'flex', justifyContent: 'center', p: 1 }}>
          <Button color="inherit" onClick={onMore}>
            {t('webLoadMore')}
          </Button>
        </Box>
      )}
    </Box>
  );
}
