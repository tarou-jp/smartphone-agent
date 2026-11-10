import { useState } from 'react';
import { Box, IconButton, ListItemText, Menu, MenuItem, Stack, Tab, Tabs, Tooltip, Typography, useTheme } from '@mui/material';
import AddIcon from '@mui/icons-material/AddRounded';
import HistoryIcon from '@mui/icons-material/HistoryRounded';
import CloseIcon from '@mui/icons-material/CloseRounded';
import CollapseIcon from '@mui/icons-material/KeyboardDoubleArrowRightRounded';
import { useI18n } from '@/i18n';
import type { Chat } from './useChat';

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
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const open = chat.sessions.filter((s) => s.open);
  const past = [...chat.sessions].filter((s) => s.msgs.length > 0).sort((a, b) => b.updated - a.updated);
  const title = (s: { title: string }) => s.title || t('chatUntitled');

  return (
    <Stack direction="row" alignItems="center" sx={{ height: 52, px: 0.5, borderBottom: 1, borderColor: 'divider', flexShrink: 0 }}>
      <Tabs
        value={open.some((s) => s.id === chat.active.id) ? chat.active.id : false}
        onChange={(_, id: string) => chat.select(id)}
        variant="scrollable"
        scrollButtons={false}
        sx={{ flex: 1, minWidth: 0, '& .MuiTabs-indicator': { height: 2 } }}
      >
        {open.map((s) => (
          <Tab
            key={s.id}
            value={s.id}
            sx={{ px: 1, minWidth: 0, maxWidth: 170, fontSize: 12.5, '&:hover .tab-close': { opacity: 1 } }}
            label={
              <Stack direction="row" alignItems="center" spacing={0.25} sx={{ maxWidth: '100%' }}>
                <Typography noWrap sx={{ fontSize: 12.5, fontWeight: 600 }}>
                  {title(s)}
                </Typography>
                {open.length > 1 && (
                  <Box
                    component="span"
                    role="button"
                    aria-label={t('chatCloseTab')}
                    className="tab-close"
                    onClick={(e) => {
                      e.stopPropagation();
                      chat.close(s.id);
                    }}
                    sx={{ display: 'inline-flex', opacity: s.id === chat.active.id ? 0.7 : 0, borderRadius: 1, '&:hover': { bgcolor: 'action.hover' } }}
                  >
                    <CloseIcon sx={{ fontSize: 13 }} />
                  </Box>
                )}
              </Stack>
            }
          />
        ))}
      </Tabs>
      <Tooltip title={t('chatNew')}>
        <IconButton aria-label={t('chatNew')} onClick={chat.newChat}>
          <AddIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title={t('chatHistory')}>
        <IconButton aria-label={t('chatHistory')} onClick={(e) => setAnchor(e.currentTarget)}>
          <HistoryIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      {onCollapse && (
        <Tooltip title={t('chatCollapse')}>
          <IconButton aria-label={t('chatCollapse')} onClick={onCollapse}>
            <CollapseIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      )}
      <Menu anchorEl={anchor} open={!!anchor} onClose={() => setAnchor(null)} slotProps={{ paper: { sx: { width: 300, maxHeight: 420 } } }}>
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
              setAnchor(null);
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
    </Stack>
  );
}
