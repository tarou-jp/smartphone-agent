import { useRef, useState, type KeyboardEvent } from 'react';
import { Box, Chip, IconButton, InputBase, Stack, Tooltip, useTheme } from '@mui/material';
import ArrowUpIcon from '@mui/icons-material/ArrowUpwardRounded';
import StopIcon from '@mui/icons-material/StopRounded';
import PhoneIcon from '@mui/icons-material/SmartphoneRounded';
import { useT } from '@/i18n';
import { ModelPicker } from './ModelPicker';
import { QueueList } from './QueueList';
import type { Chat } from './useChat';

/**
 * Cursor's composer: Enter sends, or queues while a run goes; Shift+Enter
 * breaks the line; Ctrl+C with nothing selected stops the run.
 */
export function Composer({ chat, scope }: { chat: Chat; scope: string | null }) {
  const t = useT();
  const p = useTheme().pinky;
  const [draft, setDraft] = useState('');
  const input = useRef<HTMLTextAreaElement>(null);
  const s = chat.active;

  const submit = () => {
    if (!draft.trim()) return;
    chat.send(draft);
    setDraft('');
  };

  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      submit();
    } else if (e.key === 'c' && e.ctrlKey && !e.metaKey && chat.busy) {
      const el = e.currentTarget;
      if (el.selectionStart === el.selectionEnd) {
        e.preventDefault();
        chat.stop();
      }
    } else if (e.key === 'Escape') {
      e.currentTarget.blur();
    }
  };

  const canSend = draft.trim() !== '';
  return (
    <Box sx={{ p: 1.25, pt: 0.5 }}>
      <QueueList queued={s.queued} onSteer={chat.steer} onRemove={chat.removeQueued} />
      <Box
        onClick={() => input.current?.focus()}
        sx={{
          border: 1,
          borderColor: p.line2,
          borderRadius: 2.5,
          bgcolor: 'background.paper',
          px: 1.25,
          pt: 1,
          pb: 0.5,
          cursor: 'text',
          '&:focus-within': { borderColor: 'primary.main' },
        }}
      >
        <InputBase
          inputRef={input}
          multiline
          minRows={2}
          maxRows={12}
          fullWidth
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKey}
          placeholder={s.msgs.length ? t('chatPlaceholderFollowUp') : t('chatPlaceholder')}
          inputProps={{ 'aria-label': t('chatSection') }}
          sx={{ fontSize: 13, lineHeight: 1.5 }}
        />
        <Stack direction="row" alignItems="center" spacing={0.5} sx={{ mt: 0.5 }}>
          <ModelPicker />
          <Chip
            icon={<PhoneIcon sx={{ fontSize: '14px !important' }} />}
            label={scope ? t('chatScopeOne', { name: scope }) : t('chatScopeNone')}
            variant="outlined"
            sx={{ height: 22, fontSize: 11.5, fontWeight: 500, maxWidth: 130, borderColor: p.line2, color: p.ink2 }}
          />
          <Box sx={{ flex: 1 }} />
          {chat.busy && !canSend ? (
            <Tooltip title={`${t('chatStop')}  ${t('chatStopKeyHint')}`}>
              <IconButton
                aria-label={t('chatStop')}
                onClick={(e) => {
                  e.stopPropagation();
                  chat.stop();
                }}
                sx={{ width: 28, height: 28, bgcolor: p.ink2, color: 'background.paper', '&:hover': { bgcolor: p.ink3 } }}
              >
                <StopIcon sx={{ fontSize: 16 }} />
              </IconButton>
            </Tooltip>
          ) : (
            <Tooltip title={t('chatSend')}>
              <span>
                <IconButton
                  aria-label={t('chatSend')}
                  disabled={!canSend}
                  onClick={(e) => {
                    e.stopPropagation();
                    submit();
                  }}
                  sx={{
                    width: 28,
                    height: 28,
                    bgcolor: canSend ? 'primary.main' : p.bg3,
                    color: canSend ? 'primary.contrastText' : p.ink3,
                    '&:hover': { bgcolor: 'primary.dark' },
                    '&.Mui-disabled': { bgcolor: p.bg3, color: p.ink3 },
                  }}
                >
                  <ArrowUpIcon sx={{ fontSize: 16 }} />
                </IconButton>
              </span>
            </Tooltip>
          )}
        </Stack>
      </Box>
    </Box>
  );
}
