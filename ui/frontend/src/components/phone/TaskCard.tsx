// A task's card on the phone page: its title, state and steps, after
// ui/rightpanel.go and ui/format.go.

import { useState } from 'react';
import { Box, Button, CircularProgress, Collapse, Link, Paper, Stack, Typography, useTheme } from '@mui/material';
import { StatusText } from '@/components/common';
import CheckIcon from '@mui/icons-material/CheckRounded';
import PersonIcon from '@mui/icons-material/PersonRounded';
import BlockIcon from '@mui/icons-material/BlockRounded';
import HelpIcon from '@mui/icons-material/HelpOutlineRounded';
import { api } from '@/api/client';
import type { Step, Task } from '@/api/types';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import { duration, isRunning, splitStep, taskStatus } from '@/lib/format';
import { useGw } from '@/lib/gwtext';

type StepKind = 'done' | 'now' | 'wait' | 'you' | 'blocked' | 'uncertain';

function kindOf(task: Task, i: number): StepKind {
  const s = task.steps[i];
  if (s.status === 'approval_pending') return 'wait';
  if (s.status === 'user' || (!s.tool && s.text.startsWith('You '))) return 'you';
  if ((s.status === '' || s.status === 'running') && i === task.steps.length - 1 && isRunning(task)) return 'now';
  if (s.status.startsWith('blocked') || s.status === 'denied' || s.status === 'failed') return 'blocked';
  if (s.status === 'uncertain') return 'uncertain';
  return 'done';
}

function StepMark({ kind }: { kind: StepKind }) {
  const p = useTheme().pinky;
  const box = { width: 18, height: 18, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, mt: '1px' };
  switch (kind) {
    case 'now':
      return (
        <Box sx={box}>
          <CircularProgress size={12} thickness={5} sx={{ color: p.overlay }} />
        </Box>
      );
    case 'wait':
      return (
        <Box sx={box}>
          <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: p.amberDot }} />
        </Box>
      );
    case 'you':
      return <PersonIcon sx={{ ...box, color: p.overlay }} />;
    case 'blocked':
      return <BlockIcon sx={{ ...box, fontSize: 16, color: p.red }} />;
    case 'uncertain':
      return <HelpIcon sx={{ ...box, fontSize: 16, color: p.ink3 }} />;
    default:
      return <CheckIcon sx={{ ...box, fontSize: 16, color: p.green }} />;
  }
}

function StepRow({ task, step, i, dev }: { task: Task; step: Step; i: number; dev: boolean }) {
  const t = useT();
  const p = useTheme().pinky;
  const [open, setOpen] = useState(false);
  const kind = kindOf(task, i);
  const gw = useGw();
  const split = splitStep(step.text);
  const text = gw.stepText(split.text);
  const sub = gw.stepSub(split.sub);
  return (
    <Stack direction="row" spacing={1} sx={{ py: 0.5 }}>
      <StepMark kind={kind} />
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography
          variant="body1"
          className="selectable"
          sx={{ fontWeight: kind === 'now' || kind === 'wait' ? 600 : 400, color: kind === 'wait' ? p.amber : 'text.primary' }}
        >
          {text}
        </Typography>
        {sub && (
          <Typography variant="body2" color="text.secondary" className="selectable">
            {sub}
          </Typography>
        )}
        {dev && step.tool && (
          <>
            <Link component="button" variant="caption" onClick={() => setOpen((o) => !o)} sx={{ fontFamily: 'ui-monospace, monospace' }}>
              {step.tool}
              {step.elapsedMs > 0 && ` · ${duration(t, step.elapsedMs)}`}
            </Link>
            <Collapse in={open}>
              <Box
                component="pre"
                className="selectable"
                sx={{ m: 0, mt: 0.5, p: 1, borderRadius: 1, bgcolor: p.bg3, fontSize: 11, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}
              >
                {step.args || t('tasksNoToolCalls')}
              </Box>
            </Collapse>
          </>
        )}
      </Box>
    </Stack>
  );
}

const shownSteps = 8;

export function TaskCard({ task }: { task: Task }) {
  const t = useT();
  const p = useTheme().pinky;
  const { run, settings } = useApp();
  const [all, setAll] = useState(false);
  const running = isRunning(task);
  const hidden = all ? 0 : Math.max(0, task.steps.length - shownSteps);
  const color = task.state === 'waiting' ? p.amber : task.state === 'failed' ? p.red : running ? p.overlay : p.ink2;
  return (
    <Paper variant="outlined" sx={{ p: 2, borderRadius: 3 }}>
      <Stack direction="row" alignItems="flex-start" spacing={1}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography sx={{ fontSize: 15, fontWeight: 600 }} className="selectable">
            {task.title}
          </Typography>
          <Typography variant="body2" color="text.secondary" noWrap>
            {task.agent || t('agentGeneric')} · <span style={{ color }}>{taskStatus(t, task.state)}</span>
          </Typography>
        </Box>
        {task.readOnly && <StatusText>{t('tasksReadOnly')}</StatusText>}
        {running && (
          <Button
            variant="outlined"
            color="error"
            onClick={() => run(t('callStop'), () => api.stop(task.deviceId))}
            sx={{ borderColor: p.redLine }}
          >
            {t('tasksStop')}
          </Button>
        )}
      </Stack>
      <Box sx={{ mt: 1 }}>
        {hidden > 0 && (
          <Link component="button" variant="body2" onClick={() => setAll(true)} sx={{ mb: 0.5 }}>
            {t('tasksEarlierSteps', { count: hidden })}
          </Link>
        )}
        {task.steps.slice(hidden).map((s, j) => (
          <StepRow key={s.index} task={task} step={s} i={hidden + j} dev={!!settings?.developerMode} />
        ))}
      </Box>
    </Paper>
  );
}
