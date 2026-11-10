// The Agent panel at the window's right edge, as Cursor's: chat tabs, the
// thread and the composer, the same on every page. Wide windows give it a
// column, which its left edge resizes; narrow ones lay it over the page so
// the page keeps its width. ⌘L shows it and puts the cursor in it.

import { useEffect, useRef } from 'react';
import { Box, useMediaQuery } from '@mui/material';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { Route } from '@/lib/router';
import { ChatHeader } from './ChatHeader';
import { MessageList } from './MessageList';
import { Composer } from './Composer';
import { useChat, usePanel } from './store';

const mac = typeof navigator !== 'undefined' && navigator.platform.startsWith('Mac');

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

  if (!panel.open) return null;

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

