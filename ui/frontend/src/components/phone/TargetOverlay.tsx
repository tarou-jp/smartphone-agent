import { useTheme } from '@mui/material';
import type { Point, Rect } from '@/api/types';

/**
 * Marks where an action goes on a picture of the screen: the element's
 * outline, the finger, and for a swipe the line to where it lifts. It
 * covers its positioned parent; w and h are the picture's size in the
 * coordinates the marks are in.
 */
export function TargetOverlay({
  w,
  h,
  target,
  point,
  to,
  color,
}: {
  w: number;
  h: number;
  target?: Rect;
  point?: Point;
  to?: Point;
  color?: string;
}) {
  const theme = useTheme();
  const c = color ?? theme.pinky.overlay;
  if (!w || !h || (!target && !point)) return null;
  const r = Math.max(w, h) / 60;
  const stroke = Math.max(w, h) / 300;
  return (
    <svg
      viewBox={`0 0 ${w} ${h}`}
      preserveAspectRatio="none"
      aria-hidden
      style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', pointerEvents: 'none' }}
    >
      {target && (
        <rect
          x={target.x}
          y={target.y}
          width={target.w}
          height={target.h}
          rx={r / 2}
          fill={c}
          fillOpacity={0.12}
          stroke={c}
          strokeWidth={stroke * 1.5}
        />
      )}
      {point && to && (
        <line x1={point.x} y1={point.y} x2={to.x} y2={to.y} stroke={c} strokeWidth={stroke * 2} strokeLinecap="round" />
      )}
      {point && <circle cx={point.x} cy={point.y} r={r} fill={c} fillOpacity={0.35} stroke="#fff" strokeWidth={stroke} />}
      {to && <circle cx={to.x} cy={to.y} r={r * 0.7} fill="none" stroke={c} strokeWidth={stroke * 1.5} />}
    </svg>
  );
}
