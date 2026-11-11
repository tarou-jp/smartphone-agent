import { useState } from 'react';
import { Box, Collapse, Link, Stack, Typography, useTheme } from '@mui/material';
import { useI18n } from '@/i18n';
import { clock, duration, splitStep } from '@/lib/format';
import type { Step, Task } from '@/api/types';
import type { PinkyTokens } from '@/theme/theme';
import { answered } from './outcome';
import { StepShot } from './StepShot';
import { useGw } from '@/lib/gwtext';

function stepColor(p: PinkyTokens, s: Step): string | null {
  if (s.status === 'blocked_by_policy' || s.status === 'failed' || s.status === 'approval_denied') return p.red;
  if (s.status === 'approval_pending' || s.text.startsWith('Waiting for you')) return p.amberDot;
  if (s.status === 'uncertain') return p.amberDot;
  return null;
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
  const color = stepColor(p, step);
  const last = step.index === task.steps.length - 1;
  // A timeline: the time in mono on the left, a node on a line that runs
  // through the task, the words, and the screen as a small phone.
  return (
    <Stack direction="row" spacing={2} sx={{ position: 'relative' }}>
      <Typography sx={{ width: 44, flexShrink: 0, pt: 1.75, fontFamily: p.mono, fontSize: 12, color: p.ink3, textAlign: 'right' }}>
        {clock(locale, step.at)}
      </Typography>
      <Box sx={{ position: 'relative', width: 12, flexShrink: 0, display: 'flex', justifyContent: 'center' }}>
        <Box sx={{ position: 'absolute', top: 0, bottom: last ? 'calc(100% - 22px)' : 0, width: '1px', bgcolor: p.line2 }} />
        <Box
          sx={{
            position: 'relative',
            mt: '18px',
            width: 9,
            height: 9,
            borderRadius: '50%',
            bgcolor: color ?? 'background.default',
            border: color ? 'none' : `1.5px solid ${p.ink3}`,
            boxSizing: 'border-box',
          }}
        />
      </Box>
      <Box sx={{ flex: 1, minWidth: 0, py: 1.5 }}>
        <Typography variant="body1" sx={{ fontWeight: answered(step) ? 600 : 400 }} className="selectable">
          {text}
        </Typography>
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
                  fontFamily: p.mono,
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
      <Box sx={{ py: 1.5 }}>
        <StepShot task={task} step={step} />
      </Box>
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
