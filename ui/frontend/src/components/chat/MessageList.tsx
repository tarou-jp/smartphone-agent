import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Box, Fab, Stack, useTheme } from '@mui/material';
import ArrowDownIcon from '@mui/icons-material/ArrowDownwardRounded';
import { useT } from '@/i18n';
import { TurnView } from './TurnView';
import type { ChatSession } from './model';

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
  }, [session.turns]);

  // A new chat is empty, as Cursor's: only the composer below.
  if (session.turns.length === 0) return <Box sx={{ flex: 1 }} />;
  return (
    <Box sx={{ flex: 1, minHeight: 0, position: 'relative' }}>
      <Box ref={box} onScroll={onScroll} sx={{ height: '100%', overflowY: 'auto', px: 1.5, pb: 2 }}>
        <Stack spacing={2}>
          {session.turns.map((turn) => (
            <TurnView key={turn.id} turn={turn} />
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
