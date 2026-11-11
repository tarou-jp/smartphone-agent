// The Agent panel's title bar, as Cursor's: the open chats as tabs (a
// chat icon, the title, a cross), then a running mark, New chat, Past
// chats, More, and the panel button that hides the panel.

import { useState } from 'react';
import { Box, ButtonBase, CircularProgress, IconButton, ListItemText, Menu, MenuItem, Stack, Tooltip, Typography, useTheme } from '@mui/material';
import AddIcon from '@mui/icons-material/AddRounded';
import HistoryIcon from '@mui/icons-material/HistoryRounded';
import MoreIcon from '@mui/icons-material/MoreHorizRounded';
import CloseIcon from '@mui/icons-material/CloseRounded';
import ChatIcon from '@mui/icons-material/ChatBubbleOutlineRounded';
import PanelIcon from '@mui/icons-material/ViewSidebarOutlined';
import { useI18n } from '@/i18n';
import type { Chat } from './store';
import { isBusy } from './model';
import { topRow } from '@/lib/window';

const mod = typeof navigator !== 'undefined' && navigator.platform.startsWith('Mac') ? '⌘' : 'Ctrl+';

/** When a past chat was last used: the time today, else the day. */
function when(locale: string, at: number): string {
  const d = new Date(at);
  const today = new Date();
  if (d.toDateString() === today.toDateString()) return d.toLocaleTimeString(locale, { hour: 'numeric', minute: '2-digit' });
  return d.toLocaleDateString(locale, { month: 'short', day: 'numeric' });
}

export function ChatHeader({ chat, onCollapse }: { chat: Chat; onCollapse?: () => void }) {
  const { t, locale } = useI18n();
  const p = useTheme().pinky;
  const [history, setHistory] = useState<HTMLElement | null>(null);
  const [more, setMore] = useState<HTMLElement | null>(null);
  const open = chat.sessions.filter((s) => s.open);
  const past = [...chat.sessions].filter((s) => s.turns.length > 0).sort((a, b) => b.updated - a.updated);
  const title = (s: { title: string }) => s.title || t('chatUntitled');
  const icon = { width: 26, height: 26, color: p.ink3, '&:hover': { color: 'text.primary', bgcolor: 'action.hover' } };

  return (
    <Stack
      direction="row"
      alignItems="stretch"
      sx={{ height: topRow, pr: 0.75, flexShrink: 0, borderBottom: 1, borderColor: 'divider', '& > .MuiIconButton-root, & > .MuiCircularProgress-root': { alignSelf: 'center' } }}
    >
      <Stack
        direction="row"
        alignItems="stretch"
        role="tablist"
        sx={{ flex: 1, minWidth: 0, overflowX: 'auto', scrollbarWidth: 'none', '&::-webkit-scrollbar': { display: 'none' } }}
      >
        {open.map((s) => {
          const on = s.id === chat.active.id;
          return (
            <ButtonBase
              key={s.id}
              role="tab"
              aria-selected={on}
              onClick={() => chat.select(s.id)}
              // An editor tab: the open one sits on the panel's own colour,
              // over the bar's bottom line.
              sx={{
                flexShrink: on ? 0 : 1,
                minWidth: 0,
                maxWidth: 200,
                pl: 1.5,
                pr: 0.75,
                gap: 1,
                mb: '-1px',
                borderRight: 1,
                borderColor: 'divider',
                color: on ? 'text.primary' : p.ink3,
                bgcolor: on ? p.bg2 : 'transparent',
                '&:hover': { color: 'text.primary', '& .tab-x': { opacity: 1 } },
              }}
            >
              {isBusy(s) ? (
                <CircularProgress size={12} thickness={5} sx={{ color: p.ink3, flexShrink: 0 }} />
              ) : (
                <ChatIcon sx={{ fontSize: 14, flexShrink: 0 }} />
              )}
              <Typography noWrap sx={{ fontSize: 13, fontWeight: 500 }}>
                {title(s)}
              </Typography>
              <Box
                component="span"
                role="button"
                aria-label={t('chatCloseTab')}
                className="tab-x"
                onClick={(e) => {
                  e.stopPropagation();
                  chat.close(s.id);
                }}
                sx={{ display: 'inline-flex', p: 0.25, borderRadius: 0.75, opacity: on ? 1 : 0, '&:hover': { bgcolor: 'action.hover' } }}
              >
                <CloseIcon sx={{ fontSize: 14 }} />
              </Box>
            </ButtonBase>
          );
        })}
      </Stack>
      <Tooltip title={t('chatNew')}>
        <IconButton aria-label={t('chatNew')} onClick={chat.newChat} sx={icon}>
          <AddIcon sx={{ fontSize: 18 }} />
        </IconButton>
      </Tooltip>
      <Tooltip title={t('chatHistory')}>
        <IconButton aria-label={t('chatHistory')} onClick={(e) => setHistory(e.currentTarget)} sx={icon}>
          <HistoryIcon sx={{ fontSize: 16 }} />
        </IconButton>
      </Tooltip>
      <IconButton aria-label={t('webChatMore')} onClick={(e) => setMore(e.currentTarget)} sx={icon}>
        <MoreIcon sx={{ fontSize: 18 }} />
      </IconButton>
      {onCollapse && (
        <Tooltip title={`${t('chatCollapse')}  ${mod}L`}>
          <IconButton aria-label={t('chatCollapse')} onClick={onCollapse} sx={icon}>
            <PanelIcon sx={{ fontSize: 16 }} />
          </IconButton>
        </Tooltip>
      )}

      <Menu anchorEl={history} open={!!history} onClose={() => setHistory(null)} slotProps={{ paper: { sx: { width: 300, maxHeight: 420 } } }}>
        {past.length === 0 && (
          <MenuItem disabled dense>
            {t('chatHistoryEmpty')}
          </MenuItem>
        )}
        {past.map((s) => (
          <MenuItem
            key={s.id}
            dense
            selected={s.id === chat.active.id}
            onClick={() => {
              setHistory(null);
              chat.select(s.id);
            }}
          >
            <ListItemText primary={title(s)} primaryTypographyProps={{ noWrap: true, fontSize: 13 }} />
            <Typography variant="caption" sx={{ ml: 1, color: p.ink3, flexShrink: 0 }}>
              {when(locale, s.updated)}
            </Typography>
          </MenuItem>
        ))}
      </Menu>
      <Menu
        anchorEl={more}
        open={!!more}
        onClose={() => setMore(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        slotProps={{ paper: { sx: { minWidth: 220 } } }}
      >
        <MenuItem
          dense
          onClick={() => {
            setMore(null);
            chat.close(chat.active.id);
          }}
        >
          <ListItemText primary={t('chatCloseTab')} primaryTypographyProps={{ fontSize: 13 }} />
        </MenuItem>
        <MenuItem
          dense
          disabled={open.length < 2}
          onClick={() => {
            setMore(null);
            open.filter((s) => s.id !== chat.active.id).forEach((s) => chat.close(s.id));
          }}
        >
          <ListItemText primary={t('webChatCloseOthers')} primaryTypographyProps={{ fontSize: 13 }} />
        </MenuItem>
      </Menu>
    </Stack>
  );
}
