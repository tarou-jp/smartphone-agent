// The live mirror of a phone's screen: MJPEG from deskapi in an <img>,
// with the person's touches, scrolls and typing sent back to the phone,
// and the running step's target drawn over it.

import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent, type PointerEvent, type WheelEvent } from 'react';
import { Box, CircularProgress, Stack, Typography, useTheme } from '@mui/material';
import { api } from '@/api/client';
import type { Point, Rect } from '@/api/types';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import { TargetOverlay } from './TargetOverlay';

const defaultAspect = 9 / 19.5;

/** The box an element has, kept up to date. */
function useBoxSize<E extends HTMLElement>(): [React.RefObject<E | null>, { w: number; h: number }] {
  const ref = useRef<E>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => {
      const r = e.contentRect;
      setSize((s) => (Math.abs(s.w - r.width) < 1 && Math.abs(s.h - r.height) < 1 ? s : { w: r.width, h: r.height }));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return [ref, size];
}

/** The size the stream was asked for: it changes only when the box changes a lot, after it settles. */
function useStreamSize(fit: { w: number; h: number }): { w: number; h: number } {
  const [asked, setAsked] = useState(fit);
  useEffect(() => {
    if (!fit.w || !fit.h) return;
    const far = !asked.w || Math.abs(fit.h - asked.h) / asked.h > 0.25;
    if (!far) return;
    const id = window.setTimeout(() => setAsked(fit), asked.w ? 400 : 0);
    return () => window.clearTimeout(id);
  }, [fit, asked]);
  return asked;
}

const keyNames: Record<string, string> = {
  Enter: 'enter',
  Backspace: 'delete',
  Delete: 'forward_delete',
  Escape: 'escape',
  Tab: 'tab',
  ArrowUp: 'dpad_up',
  ArrowDown: 'dpad_down',
  ArrowLeft: 'dpad_left',
  ArrowRight: 'dpad_right',
  PageUp: 'page_up',
  PageDown: 'page_down',
};

export interface MirrorProps {
  deviceId: string;
  /** Show the background display agents work on instead of the phone's screen. */
  agent?: boolean;
  /** Touches, scrolls and typing go to the phone; false for tiles. */
  interactive?: boolean;
  /** At most this many pictures a second. */
  fps?: number;
  /** Marks drawn over the picture, in device pixels. */
  target?: Rect;
  point?: Point;
  label: string;
  /** Bumped to reconnect, as when the phone comes back online. */
  generation?: string;
}

export function Mirror({ deviceId, agent, interactive = true, fps, target, point, label, generation }: MirrorProps) {
  const t = useT();
  const p = useTheme().pinky;
  const { sizes, toast } = useApp();
  const size = sizes[deviceId];
  const aspect = size && size.w && size.h ? size.w / size.h : defaultAspect;
  const [boxRef, box] = useBoxSize<HTMLDivElement>();
  const fit = useMemo(() => {
    if (!box.w || !box.h) return { w: 0, h: 0 };
    const w = Math.min(box.w, box.h * aspect);
    return { w: Math.floor(w), h: Math.floor(w / aspect) };
  }, [box, aspect]);
  const asked = useStreamSize(fit);

  // Each attempt gets a new URL; errors retry with a growing pause.
  const [attempt, setAttempt] = useState(0);
  const [phase, setPhase] = useState<'connecting' | 'playing' | 'ended'>('connecting');
  const backoff = useRef(1000);
  const imgRef = useRef<HTMLImageElement>(null);
  const dpr = window.devicePixelRatio || 1;
  const src = useMemo(
    () =>
      asked.w && asked.h
        ? api.mirrorURL(deviceId, { w: asked.w * dpr, h: asked.h * dpr, agent, fps })
        : '',
    // attempt and generation make a new connection.
    [deviceId, agent, fps, asked, dpr, attempt, generation],
  );
  useEffect(() => {
    setPhase('connecting');
  }, [src]);
  useEffect(() => {
    if (phase !== 'ended') return;
    const id = window.setTimeout(() => setAttempt((a) => a + 1), backoff.current);
    backoff.current = Math.min(backoff.current * 2, 10000);
    return () => window.clearTimeout(id);
  }, [phase]);
  // Close the connection when the mirror goes: an <img> keeps streaming
  // otherwise. lastImg outlives the ref, which React clears first.
  const lastImg = useRef<HTMLImageElement | null>(null);
  useEffect(
    () => () => {
      if (lastImg.current) lastImg.current.src = '';
    },
    [],
  );
  // Not every engine fires load for MJPEG: a picture with a size is playing.
  useEffect(() => {
    if (phase !== 'connecting') return;
    const id = window.setInterval(() => {
      if ((imgRef.current?.naturalWidth ?? 0) > 0) {
        setPhase('playing');
        backoff.current = 1000;
      }
    }, 300);
    return () => window.clearInterval(id);
  }, [phase, src]);

  // Input. Touches go out in order; moves at most about 60 a second.
  const chain = useRef<Promise<unknown>>(Promise.resolve());
  const failed = useRef(0);
  const send = useCallback(
    (what: string, fn: () => Promise<unknown>) => {
      chain.current = chain.current.then(fn).catch((e: unknown) => {
        // One toast per burst of failures, not one per move.
        const now = Date.now();
        if (now - failed.current > 3000) {
          failed.current = now;
          toast(t('toastCouldNot', { action: what, error: e instanceof Error ? e.message : String(e) }), 'error');
        }
      });
    },
    [toast, t],
  );
  const pos = (e: { clientX: number; clientY: number }) => {
    const r = imgRef.current?.getBoundingClientRect();
    if (!r || !r.width || !r.height) return null;
    return {
      nx: Math.min(Math.max((e.clientX - r.left) / r.width, 0), 0.9999),
      ny: Math.min(Math.max((e.clientY - r.top) / r.height, 0), 0.9999),
    };
  };
  const down = useRef(false);
  const lastMove = useRef(0);
  const pendingMove = useRef<{ nx: number; ny: number } | null>(null);
  const touchWhat = t('mirrorCallTouch');
  const onPointerDown = (e: PointerEvent) => {
    if (!interactive || e.button !== 0) return;
    const q = pos(e);
    if (!q) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    (e.currentTarget as HTMLElement).focus();
    down.current = true;
    lastMove.current = performance.now();
    send(touchWhat, () => api.touch(deviceId, 'down', q.nx, q.ny));
  };
  const onPointerMove = (e: PointerEvent) => {
    if (!down.current) return;
    const q = pos(e);
    if (!q) return;
    const now = performance.now();
    if (now - lastMove.current < 16) {
      pendingMove.current = q;
      return;
    }
    lastMove.current = now;
    pendingMove.current = null;
    send(touchWhat, () => api.touch(deviceId, 'move', q.nx, q.ny));
  };
  const onPointerUp = (e: PointerEvent) => {
    if (!down.current) return;
    down.current = false;
    const q = pos(e) ?? pendingMove.current;
    pendingMove.current = null;
    if (q) send(touchWhat, () => api.touch(deviceId, 'up', q.nx, q.ny));
  };
  const lastWheel = useRef(0);
  const onWheel = (e: WheelEvent) => {
    if (!interactive) return;
    const now = performance.now();
    if (now - lastWheel.current < 250) return;
    lastWheel.current = now;
    const q = pos(e);
    if (!q) return;
    const shift = Math.max(-0.3, Math.min(0.3, e.deltaY / 800));
    if (Math.abs(shift) < 0.01) return;
    const to = Math.max(0, Math.min(0.9999, q.ny - shift));
    const mid = (q.ny + to) / 2;
    send(touchWhat, async () => {
      await api.touch(deviceId, 'down', q.nx, q.ny);
      await api.touch(deviceId, 'move', q.nx, mid);
      await api.touch(deviceId, 'move', q.nx, to);
      await api.touch(deviceId, 'up', q.nx, to);
    });
  };
  const onKeyDown = (e: KeyboardEvent) => {
    if (!interactive) return;
    if (e.metaKey || e.ctrlKey) return; // shortcuts, and paste below
    const k = keyNames[e.key];
    if (k) {
      e.preventDefault();
      send(t('mirrorCallKey', { key: e.key }), () => api.key(deviceId, k));
    } else if (e.key.length === 1 && !e.altKey) {
      e.preventDefault();
      send(t('mirrorCallType'), () => api.text(deviceId, e.key));
    }
  };
  const onPaste = (e: React.ClipboardEvent) => {
    if (!interactive) return;
    const text = e.clipboardData.getData('text/plain');
    if (!text) return;
    e.preventDefault();
    send(t('mirrorCallPaste'), () => api.text(deviceId, text));
  };

  return (
    <Box
      ref={boxRef}
      sx={{ position: 'relative', width: '100%', height: '100%', minHeight: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}
    >
      <Box
        role={interactive ? 'application' : 'img'}
        aria-label={label}
        tabIndex={interactive ? 0 : -1}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onWheel={onWheel}
        onKeyDown={onKeyDown}
        onPaste={onPaste}
        sx={{
          position: 'relative',
          width: fit.w,
          height: fit.h,
          borderRadius: `${Math.max(8, fit.w / 14)}px`,
          overflow: 'hidden',
          bgcolor: p.phone,
          boxShadow: interactive ? `0 0 0 6px ${p.phone}, 0 10px 40px rgba(0,0,0,.25)` : 'none',
          cursor: interactive ? 'pointer' : 'inherit',
          touchAction: 'none',
          outline: 'none',
          '&:focus-visible': { boxShadow: `0 0 0 6px ${p.phone}, 0 0 0 9px ${p.overlay}` },
        }}
      >
        {src && (
          <img
            ref={(el) => {
              imgRef.current = el;
              if (el) lastImg.current = el;
            }}
            src={src}
            alt=""
            draggable={false}
            onLoad={() => {
              setPhase('playing');
              backoff.current = 1000;
            }}
            onError={() => setPhase('ended')}
            style={{ width: '100%', height: '100%', display: 'block', objectFit: 'contain', pointerEvents: 'none' }}
          />
        )}
        {size && <TargetOverlay w={size.w} h={size.h} target={target} point={point} />}
        {phase !== 'playing' && (
          <Stack
            alignItems="center"
            justifyContent="center"
            spacing={1}
            sx={{ position: 'absolute', inset: 0, color: '#c9c9d1', px: 2, textAlign: 'center' }}
          >
            <CircularProgress size={fit.w > 160 ? 22 : 14} sx={{ color: 'inherit' }} />
            {fit.w > 120 && (
              <Typography variant="body2" sx={{ color: 'inherit' }}>
                {phase === 'ended' ? t('mirrorStreamEnded') : t('mirrorConnecting')}
              </Typography>
            )}
          </Stack>
        )}
      </Box>
    </Box>
  );
}
