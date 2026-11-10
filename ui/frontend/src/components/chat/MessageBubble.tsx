import { Fragment, useState, type ReactNode } from 'react';
import { Box, ButtonBase, Collapse, Link, Stack, Typography, useTheme } from '@mui/material';
import ChevronRightIcon from '@mui/icons-material/ChevronRightRounded';
import { useT } from '@/i18n';
import { href } from '@/lib/router';
import type { ChatMsg } from './useChat';

/** Inline `code` and **bold**; everything else as typed, line breaks kept. */
function lite(text: string): ReactNode[] {
  const out: ReactNode[] = [];
  const re = /(`[^`\n]+`|\*\*[^*\n]+\*\*)/g;
  let last = 0;
  let m: RegExpExecArray | null;
  let k = 0;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(<Fragment key={k++}>{text.slice(last, m.index)}</Fragment>);
    const tok = m[0];
    if (tok.startsWith('`')) {
      out.push(
        <Box
          key={k++}
          component="code"
          sx={{ fontFamily: 'ui-monospace, Menlo, monospace', fontSize: '0.92em', px: 0.5, borderRadius: 0.75, bgcolor: 'action.hover' }}
        >
          {tok.slice(1, -1)}
        </Box>,
      );
    } else {
      out.push(<strong key={k++}>{tok.slice(2, -2)}</strong>);
    }
    last = m.index + tok.length;
  }
  if (last < text.length) out.push(<Fragment key={k++}>{text.slice(last)}</Fragment>);
  return out;
}

const bodySx = { whiteSpace: 'pre-wrap', wordBreak: 'break-word', userSelect: 'text', lineHeight: 1.55 } as const;

function Progress({ m }: { m: ChatMsg }) {
  const t = useT();
  const p = useTheme().pinky;
  const [open, setOpen] = useState(false);
  const lines = m.lines ?? [];
  const last = lines[lines.length - 1];
  if (m.streaming) {
    return (
      <Box sx={{ pl: 0.5, color: p.ink3 }}>
        <Typography
          variant="body2"
          sx={{
            color: p.ink2,
            animation: 'pinky-shimmer 1.6s ease-in-out infinite',
            '@keyframes pinky-shimmer': { '50%': { opacity: 0.5 } },
          }}
        >
          {last ?? t('chatThinking')}
        </Typography>
        {lines.length > 1 && (
          <Stack spacing={0.25} sx={{ mt: 0.5 }}>
            {lines.slice(-6, -1).map((l, i) => (
              <Typography key={i} variant="caption" noWrap>
                {l}
              </Typography>
            ))}
          </Stack>
        )}
      </Box>
    );
  }
  return (
    <Box sx={{ pl: 0.5 }}>
      <ButtonBase onClick={() => setOpen((o) => !o)} sx={{ borderRadius: 1, color: p.ink3, gap: 0.25, pr: 0.5 }}>
        <ChevronRightIcon sx={{ fontSize: 16, transition: 'transform .15s', transform: open ? 'rotate(90deg)' : 'none' }} />
        <Typography variant="caption">{t('webChatSteps', { count: lines.length })}</Typography>
      </ButtonBase>
      <Collapse in={open}>
        <Stack spacing={0.25} sx={{ pl: 2.5, pt: 0.5 }}>
          {lines.map((l, i) => (
            <Typography key={i} variant="caption" sx={bodySx}>
              {l}
            </Typography>
          ))}
        </Stack>
      </Collapse>
    </Box>
  );
}

export function MessageBubble({ m }: { m: ChatMsg }) {
  const t = useT();
  const p = useTheme().pinky;
  switch (m.role) {
    case 'progress':
      return <Progress m={m} />;
    case 'user':
      return (
        <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
          <Box sx={{ maxWidth: '88%', px: 1.5, py: 1, borderRadius: 2.5, bgcolor: p.accentTint, border: 1, borderColor: p.line }}>
            <Typography variant="body1" sx={bodySx}>
              {m.text}
            </Typography>
          </Box>
        </Box>
      );
    case 'assistant':
      return (
        <Typography variant="body1" sx={{ ...bodySx, px: 0.5 }}>
          {lite(m.text)}
        </Typography>
      );
    case 'error':
      return (
        <Box sx={{ px: 1.5, py: 1, borderRadius: 2, bgcolor: p.redTint, border: 1, borderColor: p.redLine, color: p.red }}>
          <Typography variant="body2" sx={bodySx}>
            {m.code === 'no_key'
              ? t('chatErrNoKey')
              : m.code === 'no_fallback'
                ? t('chatAgentNoFallback')
                : t('chatErrReply', { detail: m.text })}
          </Typography>
          {(m.code === 'no_key' || m.code === 'no_fallback') && (
            <Link href={href({ page: 'settings', section: 'model' })} variant="body2" sx={{ fontWeight: 600, mt: 0.5, display: 'inline-block' }}>
              {t('chatNeedKeyLink')}
            </Link>
          )}
        </Box>
      );
  }
}
