// Cursor's composer: a context pill above the text (here, the phones the
// message goes to: the one chosen in the sidebar, or every connected one
// on All phones), the mode and model below it, and one round button that
// sends, or stops while a run goes.
//
// Enter sends, or queues while the chat runs; Shift+Enter breaks the line;
// Ctrl+C with nothing selected stops; ⌘. switches Agent and Ask; Escape
// leaves the box.

import { useRef, useState, type KeyboardEvent } from 'react';
import {
  Box,
  ButtonBase,
  IconButton,
  InputBase,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  Tooltip,
  useTheme,
} from '@mui/material';
import ArrowUpIcon from '@mui/icons-material/ArrowUpwardRounded';
import StopIcon from '@mui/icons-material/StopRounded';
import PhoneIcon from '@mui/icons-material/SmartphoneRounded';
import PhonesIcon from '@mui/icons-material/DevicesOtherRounded';
import InfinityIcon from '@mui/icons-material/AllInclusiveRounded';
import AskIcon from '@mui/icons-material/ChatBubbleOutlineRounded';
import ExpandIcon from '@mui/icons-material/ExpandMoreRounded';
import CheckIcon from '@mui/icons-material/CheckRounded';
import { useT } from '@/i18n';
import { ModelPicker } from './ModelPicker';
import { QueueList } from './QueueList';
import { useChat, usePanel } from './store';
import type { Mode, PhoneRef } from './model';

const mac = typeof navigator !== 'undefined' && navigator.platform.startsWith('Mac');

export function Composer({
  targets,
  all,
  agentAvailable,
  tall,
}: {
  targets: PhoneRef[];
  all: boolean;
  agentAvailable: boolean;
  /** A new chat's composer, at the top: room for a longer first message. */
  tall?: boolean;
}) {
  const t = useT();
  const p = useTheme().pinky;
  const chat = useChat();
  const panel = usePanel();
  const s = chat.active;
  const [draft, setDraft] = useState('');
  const [modeAnchor, setModeAnchor] = useState<HTMLElement | null>(null);
  const input = useRef<HTMLTextAreaElement | null>(null);

  const agentMode = s.mode === 'agent' && agentAvailable;

  const submit = () => {
    if (!draft.trim()) return;
    chat.send(draft, agentMode ? targets : [], agentMode ? 'agent' : 'ask', agentMode && targets.length > 1 && !!s.together);
    setDraft('');
    // A new chat's first message moves the composer from the top to the
    // bottom, a new box: keep typing in it.
    window.setTimeout(() => panel.inputRef.current?.focus(), 0);
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
    } else if (e.key === '.' && (mac ? e.metaKey : e.ctrlKey) && !e.altKey) {
      e.preventDefault();
      chat.setMode(s.mode === 'agent' ? 'ask' : 'agent');
    } else if (e.key === 'Escape') {
      e.currentTarget.blur();
    }
  };

  const canSend = draft.trim() !== '';
  const modes: { mode: Mode; label: string; desc: string; Icon: typeof InfinityIcon }[] = [
    { mode: 'agent', label: t('webChatModeAgent'), desc: t('webChatModeAgentDesc'), Icon: InfinityIcon },
    { mode: 'ask', label: t('webChatModeAsk'), desc: t('webChatModeAskDesc'), Icon: AskIcon },
  ];
  const modeNow = modes.find((m) => m.mode === (agentMode ? 'agent' : 'ask'))!;

  const scope = all && targets.length > 1 ? t('webChatAllPhonesCount', { count: targets.length }) : targets[0]?.name;
  const round = {
    width: 24,
    height: 24,
    borderRadius: '50%',
    bgcolor: 'text.primary',
    color: 'background.paper',
    '&:hover': { bgcolor: p.ink2 },
    '&.Mui-disabled': { bgcolor: p.bg3, color: p.ink3 },
  } as const;

  return (
    <Box sx={{ px: 1.5, pb: 1.5, pt: 0.5 }}>
      <QueueList queued={s.queued} onSteer={chat.steer} onRemove={chat.removeQueued} />
      <Box
        onClick={() => input.current?.focus()}
        sx={{
          border: 1,
          borderColor: p.line2,
          borderRadius: 2.5,
          bgcolor: p.bg2,
          px: 1.25,
          pt: 1,
          pb: 1,
          cursor: 'text',
          transition: 'border-color .1s',
          '&:focus-within': { borderColor: p.ink3 },
        }}
      >
        {agentMode && (
          <Stack direction="row" alignItems="center" spacing={0.5} sx={{ mb: 0.5, minWidth: 0 }}>
          <Tooltip title={t('webChatScopeHint')} placement="top-start">
            <Box
              component="span"
              sx={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 0.5,
                height: 22,
                px: 0.75,
                maxWidth: '100%',
                borderRadius: 1,
                border: 1,
                borderColor: p.line2,
                color: scope ? p.ink2 : p.ink3,
                fontSize: 11.5,
                fontWeight: 500,
                whiteSpace: 'nowrap',
              }}
            >
              {all ? <PhonesIcon sx={{ fontSize: 13, flexShrink: 0 }} /> : <PhoneIcon sx={{ fontSize: 13, flexShrink: 0 }} />}
              <Box component="span" sx={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {scope ?? t('chatScopeNone')}
              </Box>
            </Box>
          </Tooltip>
            {targets.length > 1 && (
              <Tooltip title={s.together ? t('webChatTogetherDesc') : t('webChatSideBySideDesc')} placement="top">
                <ButtonBase
                  onClick={(e) => {
                    e.stopPropagation();
                    chat.setTogether(!s.together);
                  }}
                  sx={{
                    height: 22,
                    px: 0.75,
                    flexShrink: 0,
                    borderRadius: 1,
                    border: 1,
                    borderColor: s.together ? p.ink3 : p.line2,
                    color: s.together ? 'text.primary' : p.ink3,
                    fontSize: 11.5,
                    fontWeight: 500,
                    '&:hover': { bgcolor: 'action.hover' },
                  }}
                >
                  {s.together ? t('webChatTogether') : t('webChatSideBySide')}
                </ButtonBase>
              </Tooltip>
            )}
          </Stack>
        )}
        <InputBase
          inputRef={(el: HTMLTextAreaElement | null) => {
            input.current = el;
            panel.inputRef.current = el;
          }}
          multiline
          minRows={tall ? 4 : 2}
          maxRows={14}
          fullWidth
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKey}
          placeholder={
            s.turns.length ? t('chatPlaceholderFollowUp') : agentMode ? t('webChatPlaceholderAgent') : t('webChatPlaceholderAsk')
          }
          inputProps={{ 'aria-label': t('chatSection') }}
          sx={{ display: 'block', fontSize: 13, lineHeight: 1.5, px: 0.25, py: 0 }}
        />
        <Stack direction="row" alignItems="center" spacing={0.25} sx={{ mt: 0.75, height: 24 }}>
          <Tooltip title={`${t('webChatModeSwitch')}  ${mac ? '⌘.' : 'Ctrl+.'}`}>
          <ButtonBase
            onClick={(e) => {
              e.stopPropagation();
              setModeAnchor(e.currentTarget);
            }}
            sx={{
              height: 24,
              px: 0.75,
              gap: 0.5,
              borderRadius: 1.5,
              bgcolor: p.bg3,
              color: p.ink3,
              fontSize: 12,
              fontWeight: 500,
              '&:hover': { bgcolor: p.sel },
            }}
          >
            <modeNow.Icon sx={{ fontSize: 14 }} />
            {modeNow.label}
            <ExpandIcon sx={{ fontSize: 14, ml: -0.25 }} />
          </ButtonBase>
          </Tooltip>
          <ModelPicker />
          <Box sx={{ flex: 1 }} />
          {chat.busy && !canSend ? (
            <Tooltip title={`${t('chatStop')}  ${t('chatStopKeyHint')}`}>
              <IconButton
                aria-label={t('chatStop')}
                onClick={(e) => {
                  e.stopPropagation();
                  chat.stop();
                }}
                sx={round}
              >
                <StopIcon sx={{ fontSize: 14 }} />
              </IconButton>
            </Tooltip>
          ) : (
            <Tooltip title={t('chatSend')}>
              {/* A disabled button gets no tooltip events; the box keeps them, and must not add a line's height. */}
              <Box component="span" sx={{ display: 'inline-flex' }}>
                <IconButton
                  aria-label={t('chatSend')}
                  disabled={!canSend}
                  onClick={(e) => {
                    e.stopPropagation();
                    submit();
                  }}
                  sx={round}
                >
                  <ArrowUpIcon sx={{ fontSize: 15 }} />
                </IconButton>
              </Box>
            </Tooltip>
          )}
        </Stack>
      </Box>

      <Menu
        anchorEl={modeAnchor}
        open={!!modeAnchor}
        onClose={() => setModeAnchor(null)}
        anchorOrigin={{ vertical: 'top', horizontal: 'left' }}
        transformOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        slotProps={{ paper: { sx: { minWidth: 240 } } }}
      >
        {modes.map((m) => (
          <MenuItem
            key={m.mode}
            dense
            disabled={m.mode === 'agent' && !agentAvailable}
            selected={m.mode === modeNow.mode}
            onClick={() => {
              setModeAnchor(null);
              chat.setMode(m.mode);
              requestAnimationFrame(() => input.current?.focus());
            }}
          >
            <ListItemIcon sx={{ minWidth: 28 }}>
              <m.Icon sx={{ fontSize: 16 }} />
            </ListItemIcon>
            <ListItemText
              primary={m.label}
              secondary={m.desc}
              primaryTypographyProps={{ fontSize: 13, fontWeight: 600 }}
              secondaryTypographyProps={{ fontSize: 11.5 }}
            />
            {m.mode === modeNow.mode && <CheckIcon sx={{ fontSize: 16, ml: 1 }} />}
          </MenuItem>
        ))}
      </Menu>
    </Box>
  );
}
