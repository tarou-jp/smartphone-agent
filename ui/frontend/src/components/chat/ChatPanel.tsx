// The Agent column of the phone page, after ui/chat.go: session tabs, the
// thread and Cursor's composer. On a phone the agent works on that phone;
// on All phones, on the first one connected; without the in-app agent it
// only talks.

import { Box, useTheme } from '@mui/material';
import { useApp } from '@/state/AppState';
import { useI18n } from '@/i18n';
import { useChat } from './useChat';
import { ChatHeader } from './ChatHeader';
import { MessageList } from './MessageList';
import { Composer } from './Composer';

export function ChatPanel({ deviceId, onCollapse }: { deviceId?: string; onCollapse?: () => void }) {
  const { locale } = useI18n();
  const p = useTheme().pinky;
  const { live, boot } = useApp();
  const online = live.devices.filter((d) => d.status !== 'offline');
  const target = deviceId ? online.find((d) => d.id === deviceId) : online[0];
  const phones = deviceId ? live.devices.filter((d) => d.id === deviceId).map((d) => d.name) : online.map((d) => d.name);
  const agent = !!boot?.inAppAgent && !!target;

  const chat = useChat({
    deviceId: agent ? target?.id : undefined,
    deviceName: agent ? target?.name : undefined,
    phones,
    locale,
  });

  return (
    <Box
      component="aside"
      sx={{ width: '100%', height: '100%', display: 'flex', flexDirection: 'column', bgcolor: p.bg2, borderLeft: 1, borderColor: 'divider', minWidth: 0 }}
    >
      <ChatHeader chat={chat} onCollapse={onCollapse} />
      <MessageList session={chat.active} />
      <Composer chat={chat} scope={target?.name ?? null} />
    </Box>
  );
}
