// Shared ways of putting things into words, after ui/format.go.

import type { T } from '@/i18n';
import type { DeviceStatus, Task, TaskState } from '@/api/types';

/** "14 s", "1 min 02 s", "2 h 05 min". */
export function duration(t: T, ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return t('timeSeconds', { count: s, seconds: s });
  const m = Math.floor(s / 60);
  if (m < 60) return t('timeMinutes', { minutes: m, seconds: String(s % 60).padStart(2, '0'), count: m });
  return t('timeHours', { hours: Math.floor(m / 60), minutes: String(m % 60).padStart(2, '0'), count: Math.floor(m / 60) });
}

/** The day a date falls on: Today, Yesterday, Monday, January 2, … */
export function day(t: T, iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const start = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((start(now) - start(d)) / 86400000);
  if (diff === 0) return t('dayToday');
  if (diff === 1) return t('dayYesterday');
  if (d.getFullYear() === now.getFullYear()) return t('dayThisYear', { day: d });
  return t('dayOtherYear', { day: d });
}

export function clock(locale: string, iso: string): string {
  return new Date(iso).toLocaleTimeString(locale, { hour: 'numeric', minute: '2-digit' });
}

/** Status rank for sorting and the colour of the dot. */
export const statusRank: Record<DeviceStatus, number> = {
  needs_you: 0,
  needs_setup: 1,
  locked: 2,
  in_control: 3,
  working: 4,
  idle: 5,
  offline: 6,
};

export function taskStatus(t: T, state: TaskState): string {
  switch (state) {
    case 'working':
      return t('tasksStatusWorking');
    case 'waiting':
      return t('tasksStatusWaiting');
    case 'paused':
      return t('tasksStatusPaused');
    case 'done':
      return t('tasksStatusDone');
    case 'stopped':
      return t('tasksStatusStopped');
    case 'failed':
      return t('tasksStatusFailed');
  }
}

export function isRunning(task: Task): boolean {
  return task.state === 'working' || task.state === 'waiting' || task.state === 'paused';
}

/** "Opened Dinely\nsub line" → text and the smaller second line. */
export function splitStep(text: string): { text: string; sub: string } {
  const i = text.indexOf('\n');
  return i < 0 ? { text, sub: '' } : { text: text.slice(0, i), sub: text.slice(i + 1) };
}
