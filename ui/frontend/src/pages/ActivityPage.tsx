import { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, Box, Button, InputAdornment, Snackbar, TextField, Tooltip } from '@mui/material';
import SearchIcon from '@mui/icons-material/SearchRounded';
import { PageHeader } from '@/components/common';
import { TaskList } from '@/components/activity/TaskList';
import { TaskDetail } from '@/components/activity/TaskDetail';
import { api } from '@/api/client';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import { isRunning } from '@/lib/format';
import type { Task } from '@/api/types';

const pageSize = 50;

function base(p: string): string {
  const parts = p.split(/[/\\]/).filter(Boolean);
  return parts[parts.length - 1] ?? p;
}

function useDebounced<V>(v: V, ms: number): V {
  const [d, setD] = useState(v);
  useEffect(() => {
    const id = window.setTimeout(() => setD(v), ms);
    return () => window.clearTimeout(id);
  }, [v, ms]);
  return d;
}

export function ActivityPage({ taskId }: { taskId?: string }) {
  const t = useT();
  const { live } = useApp();
  const hasExport = useHas('export');
  const [search, setSearch] = useState('');
  const query = useDebounced(search.trim(), 250);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [more, setMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState('');
  const [exported, setExported] = useState<{ text: string; path?: string; error?: boolean } | null>(null);
  const abort = useRef<AbortController | null>(null);

  const load = useCallback(
    async (before?: string) => {
      abort.current?.abort();
      const ac = new AbortController();
      abort.current = ac;
      setLoading(true);
      try {
        const page = await api.history({ search: query, before, limit: pageSize }, ac.signal);
        setTasks((cur) => (before ? [...cur, ...page.tasks.filter((x) => !cur.some((c) => c.id === x.id))] : page.tasks));
        setMore(page.more);
        setErr('');
      } catch (e) {
        if (!ac.signal.aborted) setErr(t('activityReadFailed', { detail: e instanceof Error ? e.message : String(e) }));
      } finally {
        if (!ac.signal.aborted) setLoading(false);
      }
    },
    [query, t],
  );

  useEffect(() => {
    void load();
  }, [load]);

  // A task that finishes moves from the running ones into the list.
  const finished = live.tasks.filter((x) => !isRunning(x)).length;
  useEffect(() => {
    if (!query) void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [finished]);

  const running = query ? [] : live.tasks.filter(isRunning);
  const selected = [...running, ...live.tasks, ...tasks].find((x) => x.id === taskId);

  const doExport = async (ids: string[]) => {
    try {
      const r = await api.exportHistory(ids);
      const parts = r.path.split(/[/\\]/).filter(Boolean);
      setExported({ text: t('exportDone', { folder: base(r.path), parent: parts[parts.length - 2] ?? '' }), path: r.path });
    } catch (e) {
      setExported({ text: t('exportFailed', { detail: e instanceof Error ? e.message : String(e) }), error: true });
    }
  };

  return (
    <>
      <PageHeader
        title={t('activityTitle')}
        actions={
          <>
            <TextField
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t('activitySearchPlaceholder')}
              inputProps={{ 'aria-label': t('activitySearchLabel') }}
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" />
                  </InputAdornment>
                ),
              }}
              sx={{ width: 260 }}
            />
            {hasExport && (
              <Tooltip title={t('exportAllTooltip')}>
                <Button variant="outlined" color="inherit" onClick={() => doExport([])}>
                  {t('exportAll')}
                </Button>
              </Tooltip>
            )}
          </>
        }
      />
      <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <Box sx={{ width: 360, flexShrink: 0, borderRight: 1, borderColor: 'divider', display: 'flex', flexDirection: 'column' }}>
          {err && (
            <Alert severity="error" sx={{ m: 1 }}>
              {err}
            </Alert>
          )}
          <TaskList
            running={running}
            tasks={tasks}
            selected={taskId}
            more={more}
            loading={loading}
            onMore={() => void load(tasks[tasks.length - 1]?.started)}
            empty={query ? t('activityNoMatch', { query }) : t('activityEmpty')}
          />
        </Box>
        <TaskDetail key={selected?.id} task={selected} onExport={doExport} />
      </Box>
      <Snackbar
        open={!!exported}
        autoHideDuration={10000}
        onClose={() => setExported(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert
          severity={exported?.error ? 'error' : 'success'}
          variant="filled"
          onClose={() => setExported(null)}
          action={
            exported?.path ? (
              <Button color="inherit" size="small" onClick={() => exported.path && void api.open(exported.path)}>
                {t('webOpen')}
              </Button>
            ) : undefined
          }
        >
          {exported?.text}
        </Alert>
      </Snackbar>
    </>
  );
}
