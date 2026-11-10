import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Box, Fab, Stack, Typography, useTheme } from '@mui/material';
import ArrowDownIcon from '@mui/icons-material/ArrowDownwardRounded';
import { useT } from '@/i18n';
import { MessageBubble } from './MessageBubble';
import type { ChatSession } from './useChat';

/**
 * The thread. It follows new messages while the person is at the bottom,
 * and stays where they scrolled to otherwise, with a button back down.
 */
export function MessageList({ session }: { session: ChatSession }) {
  const t = useT();
  const p = useTheme().pinky;
  const box = useRef<HTMLDivElement>(null);
  const [stuck, setStuck] = useState(true);
  const stuckRef = useRef(true);

  const onScroll = () => {
    const el = box.current;
    if (!el) return;
    const at = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
    stuckRef.current = at;
    setStuck(at);
  };
  const toBottom = (smooth: boolean) => {
    const el = box.current;
    if (el) el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' });
  };

  // A new session starts at its end.
  useEffect(() => {
    stuckRef.current = true;
    setStuck(true);
    toBottom(false);
  }, [session.id]);

  // Follow what arrives, when following.
  useLayoutEffect(() => {
    if (stuckRef.current) toBottom(false);
  }, [session.msgs]);

  if (session.msgs.length === 0) {
    return (
      <Stack sx={{ flex: 1, p: 3, justifyContent: 'center', textAlign: 'center' }} spacing={1}>
        <Typography variant="h3">{t('chatEmptyTitle')}</Typography>
        <Typography variant="body2" sx={{ color: p.ink3 }}>
          {t('chatEmpty')}
        </Typography>
      </Stack>
    );
  }
  return (
    <Box sx={{ flex: 1, minHeight: 0, position: 'relative' }}>
      <Box ref={box} onScroll={onScroll} sx={{ height: '100%', overflowY: 'auto', px: 1.5, py: 2 }}>
        <Stack spacing={1.5}>
          {session.msgs.map((m, i) => (
            <MessageBubble key={i} m={m} />
          ))}
        </Stack>
      </Box>
      {!stuck && (
        <Fab
          size="small"
          aria-label={t('webChatJumpLatest')}
          onClick={() => toBottom(true)}
          sx={{ position: 'absolute', bottom: 12, left: '50%', transform: 'translateX(-50%)', width: 32, height: 32, minHeight: 32, bgcolor: 'background.paper', border: 1, borderColor: p.line2, boxShadow: 1 }}
        >
          <ArrowDownIcon fontSize="small" />
        </Fab>
      )}
    </Box>
  );
}
