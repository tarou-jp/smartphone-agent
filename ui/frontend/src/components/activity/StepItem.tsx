import { useState } from 'react';
import { Box, Collapse, Link, Stack, Typography, useTheme } from '@mui/material';
import { useI18n } from '@/i18n';
import { clock, duration, splitStep } from '@/lib/format';
import type { Step, Task } from '@/api/types';
import type { PinkyTokens } from '@/theme/theme';
import { answered } from './outcome';
import { StepShot } from './StepShot';
import { useGw } from '@/lib/gwtext';

function stepColor(p: PinkyTokens, s: Step): string {
  if (s.status === 'blocked_by_policy' || s.status === 'failed' || s.status === 'approval_denied') return p.red;
  if (s.status === 'approval_pending' || s.text.startsWith('Waiting for you')) return p.amberDot;
  if (s.status === 'uncertain') return p.amberDot;
  return p.line2;
}

/** One step of a task: its words, the time, its screenshot, and in Developer mode its tool call. */
export function StepItem({ task, step, developer }: { task: Task; step: Step; developer: boolean }) {
  const { t, locale } = useI18n();
  const p = useTheme().pinky;
  const [tech, setTech] = useState(false);
  const gw = useGw();
  const split = splitStep(step.text);
  const text = gw.stepText(split.text);
  const sub = gw.stepSub(split.sub);
  const prev = task.steps[step.index - 1];
  const answer = answered(step) && prev ? new Date(step.at).getTime() - new Date(prev.at).getTime() : 0;
  return (
    <Stack direction="row" spacing={1.5} sx={{ py: 1.25, borderBottom: 1, borderColor: 'divider' }}>
      <Box sx={{ width: 3, borderRadius: 2, bgcolor: stepColor(p, step), flexShrink: 0 }} />
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Stack direction="row" spacing={1} alignItems="baseline">
          <Typography variant="body1" sx={{ flex: 1, fontWeight: answered(step) ? 600 : 400 }} className="selectable">
            {text}
          </Typography>
          <Typography variant="caption" sx={{ flexShrink: 0 }}>
            {clock(locale, step.at)}
          </Typography>
        </Stack>
        {sub && (
          <Typography variant="body2" color="text.secondary" className="selectable">
            {sub}
          </Typography>
        )}
        {answer > 0 && (
          <Typography variant="caption" component="div">
            {t('activityAnsweredIn', { duration: duration(t, answer) })}
          </Typography>
        )}
        {developer && step.tool && (
          <>
            <Link component="button" variant="caption" onClick={() => setTech(!tech)} sx={{ mt: 0.5 }}>
              {step.tool}
              {step.elapsedMs > 0 && ` · ${step.elapsedMs} ms`}
            </Link>
            <Collapse in={tech}>
              <Box
                component="pre"
                className="selectable"
                sx={{
                  m: 0,
                  mt: 0.5,
                  p: 1,
                  borderRadius: 1.5,
                  bgcolor: p.bg2,
                  border: 1,
                  borderColor: 'divider',
                  fontSize: 12,
                  fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {prettyArgs(step.args)}
              </Box>
            </Collapse>
          </>
        )}
      </Box>
      <StepShot task={task} step={step} />
    </Stack>
  );
}

function prettyArgs(a?: string): string {
  if (!a) return '';
  try {
    return JSON.stringify(JSON.parse(a), null, 2);
  } catch {
    return a;
  }
}
