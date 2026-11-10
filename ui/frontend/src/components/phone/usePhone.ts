import { useApp } from '@/state/AppState';
import type { Device, Need, Task } from '@/api/types';
import { isRunning } from '@/lib/format';

/** A phone, its running task (else its latest) and its Needs you items. */
export function usePhone(id: string): { device?: Device; task?: Task; running: boolean; needs: Need[] } {
  const { live, device } = useApp();
  const d = device(id);
  const mine = live.tasks.filter((t) => t.deviceId === id);
  const task = mine.find((t) => t.id === d?.taskId) ?? mine.find(isRunning) ?? mine[mine.length - 1];
  return {
    device: d,
    task,
    running: !!task && isRunning(task),
    needs: live.needs.filter((n) => n.deviceId === id || !n.deviceId),
  };
}

/** The step whose target the mirror marks: the running task's last with a position. */
export function markedStep(task: Task | undefined) {
  if (!task || !isRunning(task)) return undefined;
  for (let i = task.steps.length - 1; i >= Math.max(0, task.steps.length - 2); i--) {
    const s = task.steps[i];
    if (s.target || s.point) return s;
  }
  return undefined;
}
