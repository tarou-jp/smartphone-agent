// How a finished task ended, after ui/activity.go.

import type { Step, Task } from '@/api/types';
import type { T } from '@/i18n';
import type { PinkyTokens } from '@/theme/theme';

/** Whether a step is the person's answer to an approval. */
export function answered(s: Step): boolean {
  return (
    s.text.startsWith('You allowed') ||
    s.text.startsWith('You denied') ||
    (s.status === 'approval_denied' && s.text.startsWith('Waiting for you: '))
  );
}

function denied(t: Task): boolean {
  return t.steps.some((s) => s.text.startsWith('You denied') || s.status === 'approval_denied');
}

function allowedLater(t: Task): boolean {
  let seen = false;
  for (const s of t.steps) {
    if (s.text.startsWith('You denied') || s.status === 'approval_denied') seen = true;
    if (seen && s.text.startsWith('You allowed')) return true;
  }
  return false;
}

export type OutcomeKind = 'done' | 'stopped' | 'failed' | 'denied' | 'running';

export function outcomeKind(task: Task): OutcomeKind {
  if (task.state === 'working' || task.state === 'waiting' || task.state === 'paused') return 'running';
  if (task.state === 'stopped') return 'stopped';
  if (task.state === 'failed') return 'failed';
  const last = task.steps[task.steps.length - 1]?.text ?? '';
  if (denied(task) && !last.startsWith('You allowed') && !allowedLater(task)) return 'denied';
  return 'done';
}

export function outcomeLabel(t: T, k: OutcomeKind): string {
  switch (k) {
    case 'stopped':
      return t('activityOutcomeStopped');
    case 'failed':
      return t('activityOutcomeFailed');
    case 'denied':
      return t('activityOutcomeDenied');
    case 'running':
      return t('tasksStatusWorking');
  }
  return t('activityOutcomeDone');
}

export function outcomeColor(p: PinkyTokens, k: OutcomeKind): string {
  switch (k) {
    case 'stopped':
      return p.ink3;
    case 'failed':
      return p.red;
    case 'denied':
      return 'transparent';
    case 'running':
      return p.overlay;
  }
  return p.green;
}

export const approvals = (t: Task) => t.steps.filter(answered).length;
