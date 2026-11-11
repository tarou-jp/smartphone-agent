// The Agent panel at the window's right edge, as Cursor's: chat tabs, the
// thread and the composer, the same on every page. Wide windows give it a
// column, which its left edge resizes; narrow ones lay it over the page so
// the page keeps its width. ⌘L shows it and puts the cursor in it.

import { useEffect, useRef } from 'react';
import { Box, Stack, Typography, useMediaQuery, useTheme } from '@mui/material';
import { PhoneGlyph } from '@/components/common/PhoneGlyph';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { Route } from '@/lib/router';
import { ChatHeader } from './ChatHeader';
import { MessageList } from './MessageList';
import { Composer } from './Composer';
import { useChat, usePanel } from './store';

const mac = typeof navigator !== 'undefined' && navigator.platform.startsWith('Mac');

/** Under a new chat's composer: what this mode does and, for Agent, the
 * phones the message will go to, so the empty column says something. */
function EmptyChat({ agentMode, targets }: { agentMode: boolean; targets: { id: string; name: string }[] }) {
  const t = useT();
  const { live } = useApp();
  const p = useTheme().pinky;
  return (
    <Box sx={{ px: 2, pt: 2 }}>
      <Typography sx={{ fontSize: 12, color: p.ink3, lineHeight: 1.6 }}>
        {agentMode ? t('webChatEmptyAgent') : t('webChatEmptyAsk')}
      </Typography>
      {agentMode && targets.length > 0 && (
        <Box sx={{ mt: 2 }}>
          <Typography sx={{ fontSize: 12, fontWeight: 500, color: p.ink2, mb: 1 }}>{t('webChatEmptyTargets')}</Typography>
          <Stack spacing={1}>
            {targets.map((d) => {
              const dev = live.devices.find((x) => x.id === d.id);
              return (
                <Stack key={d.id} direction="row" spacing={1.25} alignItems="center">
                  {dev && <PhoneGlyph status={dev.status} size={18} />}
                  <Typography sx={{ fontSize: 13 }}>{d.name}</Typography>
                </Stack>
              );
            })}
          </Stack>
        </Box>
      )}
    </Box>
  );
}

export function AgentPanel({ route }: { route: Route }) {
  const t = useT();
  const chat = useChat();
  const panel = usePanel();
  const { live, boot } = useApp();
  const docked = useMediaQuery('(min-width:1240px)');
  const drag = useRef<{ x: number; w: number } | null>(null);

  // ⌘L (Ctrl+L): open and focus; again while focused: close.
  useEffect(() => {
    const on = (e: KeyboardEvent) => {
      if (e.key.toLowerCase() !== 'l' || !(mac ? e.metaKey : e.ctrlKey) || e.altKey || e.shiftKey) return;
      e.preventDefault();
      if (panel.open && document.activeElement === panel.inputRef.current) panel.setOpen(false);
      else panel.focus();
    };
    window.addEventListener('keydown', on);
    return () => window.removeEventListener('keydown', on);
  }, [panel]);

  // The phones a message goes to follow the sidebar: the phone chosen
  // there, or every connected one on All phones. Elsewhere (Activity,
  // Settings) the last phone chosen stays.
  const online = live.devices.filter((d) => d.status !== 'offline').map((d) => ({ id: d.id, name: d.name }));
  const last = useRef<string | undefined>(undefined);
  const onPhones = route.page === 'phones';
  if (onPhones && route.deviceId) last.current = route.deviceId;
  const all = onPhones && !route.deviceId;
  const chosen = online.find((d) => d.id === (onPhones ? route.deviceId : last.current));
  const targets = all ? online : chosen ? [chosen] : [];

  const agentAvailable = !!boot?.inAppAgent && online.length > 0;

  // Settings has the window to itself: no chat beside it.
  if (!panel.open || route.page === 'settings') return null;

  const startDrag = (e: React.PointerEvent) => {
    drag.current = { x: e.clientX, w: panel.width };
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
  };
  const moveDrag = (e: React.PointerEvent) => {
    if (drag.current) panel.setWidth(drag.current.w + drag.current.x - e.clientX);
  };
  const endDrag = () => {
    drag.current = null;
  };

  return (
    <Box
      component="aside"
      aria-label={t('webAgentPanel')}
      sx={{
        width: docked ? panel.width : Math.min(panel.width, 420),
        flexShrink: 0,
        display: 'flex',
        flexDirection: 'column',
        minHeight: 0,
        bgcolor: 'background.default',
        borderLeft: 1,
        borderColor: 'divider',
        position: docked ? 'relative' : 'absolute',
        top: 0,
        right: 0,
        bottom: 0,
        height: '100vh',
        zIndex: docked ? 'auto' : 10,
        boxShadow: docked ? 'none' : 12,
      }}
    >
      {docked && (
        <Box
          role="separator"
          aria-orientation="vertical"
          aria-label={t('webAgentResize')}
          onPointerDown={startDrag}
          onPointerMove={moveDrag}
          onPointerUp={endDrag}
          onDoubleClick={() => panel.setWidth(400)}
          sx={{
            position: 'absolute',
            left: -3,
            top: 0,
            bottom: 0,
            width: 6,
            cursor: 'col-resize',
            zIndex: 3,
            '&:hover, &:active': { bgcolor: 'primary.main', opacity: 0.35 },
          }}
        />
      )}
      <ChatHeader chat={chat} onCollapse={() => panel.setOpen(false)} />
      {chat.active.turns.length === 0 ? (
        // A new chat, as Cursor draws one: the composer at the top.
        <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', pt: 0.75 }}>
          <Composer targets={targets} all={all} agentAvailable={agentAvailable} tall />
          <EmptyChat agentMode={chat.active.mode === 'agent' && agentAvailable} targets={targets} />
        </Box>
      ) : (
        <>
          <MessageList session={chat.active} />
          <Composer targets={targets} all={all} agentAvailable={agentAvailable} />
        </>
      )}
    </Box>
  );
}

