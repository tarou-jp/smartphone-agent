// A turn of the thread: the person's message, pinned at the top while its
// turn scrolls, as Cursor pins it; then what came back. A message to
// several phones shows a tab per phone above their runs.

import { useState } from 'react';
import { Box, ButtonBase, CircularProgress, Link, Stack, Typography, useTheme } from '@mui/material';
import CheckIcon from '@mui/icons-material/CheckRounded';
import CloseIcon from '@mui/icons-material/CloseRounded';
import StopIcon from '@mui/icons-material/StopRounded';
import PhoneIcon from '@mui/icons-material/SmartphoneRounded';
import { useT } from '@/i18n';
import { href } from '@/lib/router';
import { Markdown } from './Markdown';
import { ToolRow } from './ToolRow';
import type { Run, Turn } from './model';

function StateIcon({ run }: { run: Run }) {
  const p = useTheme().pinky;
  switch (run.state) {
    case 'running':
      return <CircularProgress size={10} thickness={6} sx={{ color: 'inherit' }} />;
    case 'done':
      return <CheckIcon sx={{ fontSize: 13, color: p.green }} />;
    case 'error':
      return <CloseIcon sx={{ fontSize: 13, color: p.red }} />;
    default:
      return <StopIcon sx={{ fontSize: 13 }} />;
  }
}

function Shimmer({ children }: { children: React.ReactNode }) {
  const p = useTheme().pinky;
  return (
    <Typography
      sx={{
        fontSize: 12.5,
        px: 0.75,
        py: 0.375,
        background: `linear-gradient(90deg, ${p.ink3} 0%, ${p.ink3} 40%, ${p.line2} 50%, ${p.ink3} 60%, ${p.ink3} 100%)`,
        backgroundSize: '200% 100%',
        WebkitBackgroundClip: 'text',
        backgroundClip: 'text',
        color: 'transparent',
        animation: 'pinky-shimmer 1.8s linear infinite',
        '@keyframes pinky-shimmer': { from: { backgroundPosition: '100% 0' }, to: { backgroundPosition: '-100% 0' } },
      }}
    >
      {children}
    </Typography>
  );
}

function RunBody({ run }: { run: Run }) {
  const t = useT();
  const p = useTheme().pinky;
  const lastTool = [...run.items].reverse().find((i) => i.kind === 'tool');
  const toolRunning = lastTool?.kind === 'tool' && lastTool.step.running;
  return (
    <Stack spacing={0.5}>
      {run.items.map((it, i) =>
        it.kind === 'tool' ? (
          <ToolRow key={it.step.id + i} step={it.step} />
        ) : (
          <Box key={i} sx={{ px: 0.75, py: 0.25 }}>
            <Markdown text={it.text} />
          </Box>
        ),
      )}
      {run.state === 'running' && !toolRunning && <Shimmer>{run.items.length ? t('webChatWorking') : t('chatThinking')}</Shimmer>}
      {run.reply && (
        <Box sx={{ px: 0.75, pt: run.items.length ? 0.75 : 0 }}>
          <Markdown text={run.reply} />
        </Box>
      )}
      {run.state === 'stopped' && (
        <Typography sx={{ px: 0.75, fontSize: 12, color: p.ink3 }}>{t('webChatStopped')}</Typography>
      )}
      {run.state === 'error' && (
        <Box sx={{ mx: 0.75, px: 1.25, py: 1, borderRadius: 1.5, bgcolor: p.redTint, border: 1, borderColor: p.redLine, color: p.red }}>
          <Typography sx={{ fontSize: 12.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word', userSelect: 'text' }}>
            {run.code === 'no_key'
              ? t('chatErrNoKey')
              : run.code === 'no_fallback'
                ? t('chatAgentNoFallback')
                : t('chatErrReply', { detail: run.error ?? '' })}
          </Typography>
          {(run.code === 'no_key' || run.code === 'no_fallback') && (
            <Link href={href({ page: 'settings', section: 'model' })} sx={{ fontSize: 12.5, fontWeight: 600, mt: 0.5, display: 'inline-block' }}>
              {t('chatNeedKeyLink')}
            </Link>
          )}
        </Box>
      )}
    </Stack>
  );
}

/** The person's message: a box like the composer's, folded past six lines. */
function Prompt({ turn }: { turn: Turn }) {
  const t = useT();
  const p = useTheme().pinky;
  const [full, setFull] = useState(false);
  const phones = turn.runs.map((r) => r.phone).filter(Boolean);
  return (
    <Box sx={{ position: 'sticky', top: 0, zIndex: 2, bgcolor: 'background.default', pt: 1, pb: 1, mx: -0.5, px: 0.5 }}>
      <ButtonBase
        onClick={() => setFull((f) => !f)}
        sx={{
          display: 'block',
          width: '100%',
          textAlign: 'left',
          border: 1,
          borderColor: p.line2,
          borderRadius: 2,
          bgcolor: p.bg2,
          px: 1.25,
          py: 0.875,
        }}
      >
        <Typography
          sx={{
            fontSize: 13,
            lineHeight: 1.5,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            ...(full ? {} : { display: '-webkit-box', WebkitLineClamp: 6, WebkitBoxOrient: 'vertical', overflow: 'hidden' }),
          }}
        >
          {turn.prompt}
        </Typography>
        {(phones.length > 0 || turn.mode === 'ask') && (
          <Stack direction="row" spacing={0.5} sx={{ mt: 0.75, flexWrap: 'wrap', rowGap: 0.5 }}>
            {turn.mode === 'ask' && <Pill>{t('webChatModeAsk')}</Pill>}
            {phones.map((ph) => (
              <Pill key={ph!.id} icon>
                {ph!.name}
              </Pill>
            ))}
          </Stack>
        )}
      </ButtonBase>
    </Box>
  );
}

export function Pill({ children, icon }: { children: React.ReactNode; icon?: boolean }) {
  const p = useTheme().pinky;
  return (
    <Box
      component="span"
      sx={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 0.375,
        height: 20,
        px: 0.75,
        borderRadius: 1,
        bgcolor: p.bg3,
        color: p.ink2,
        fontSize: 11.5,
        fontWeight: 500,
        maxWidth: 160,
      }}
    >
      {icon && <PhoneIcon sx={{ fontSize: 12 }} />}
      <Box component="span" sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {children}
      </Box>
    </Box>
  );
}

export function TurnView({ turn }: { turn: Turn }) {
  const p = useTheme().pinky;
  const [sel, setSel] = useState(0);
  const multi = turn.runs.length > 1;
  const run = turn.runs[Math.min(sel, turn.runs.length - 1)];
  return (
    <Box>
      <Prompt turn={turn} />
      {multi && (
        <Stack direction="row" spacing={0.5} sx={{ mb: 1, flexWrap: 'wrap', rowGap: 0.5 }} role="tablist">
          {turn.runs.map((r, i) => (
            <ButtonBase
              key={r.phone?.id ?? i}
              role="tab"
              aria-selected={i === sel}
              onClick={() => setSel(i)}
              sx={{
                gap: 0.5,
                height: 24,
                px: 1,
                borderRadius: 1.5,
                fontSize: 12,
                fontWeight: 550,
                color: i === sel ? 'text.primary' : p.ink3,
                bgcolor: i === sel ? p.sel : 'transparent',
                border: 1,
                borderColor: i === sel ? p.line2 : 'transparent',
                '&:hover': { bgcolor: i === sel ? p.sel : 'action.hover' },
              }}
            >
              <StateIcon run={r} />
              {r.phone?.name}
            </ButtonBase>
          ))}
        </Stack>
      )}
      <RunBody run={run} />
    </Box>
  );
}
