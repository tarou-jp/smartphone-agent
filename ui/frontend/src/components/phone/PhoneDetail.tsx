// One phone's page: header, the mirror with its keys, Needs you and the
// task on the right, and the chat column at the far right.

import { useEffect, useState } from 'react';
import { Box, IconButton, Stack, Tooltip, Typography } from '@mui/material';
import ChatIcon from '@mui/icons-material/ForumOutlined';
import { api } from '@/api/client';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import { PageHeader, PlatformIcon, StatusDot } from '@/components/common';
import { NeedsList } from '@/components/needs/NeedsList';
import { ChatPanel } from '@/components/chat/ChatPanel';
import { Mirror } from './Mirror';
import { ControlBar } from './ControlBar';
import { StatusBanner } from './StatusBanner';
import { TaskCard } from './TaskCard';
import { CompanionCard } from './CompanionCard';
import { PhoneStatePanel } from './PhoneStatePanel';
import { PhoneActions } from './PhoneActions';
import { markedStep, usePhone } from './usePhone';
import { useGw } from '@/lib/gwtext';

const chatKey = 'pinky-chat-collapsed';

function useChatCollapsed(): [boolean, (v: boolean) => void] {
  const [v, setV] = useState(() => {
    try {
      return localStorage.getItem(chatKey) === '1';
    } catch {
      return false;
    }
  });
  const set = (x: boolean) => {
    setV(x);
    try {
      localStorage.setItem(chatKey, x ? '1' : '0');
    } catch {
      // Remembered until the window closes.
    }
  };
  return [v, set];
}

/** Whether agents work on the phone's background display, when it can. */
function useBackground(id: string, can: boolean): [boolean | null, (on: boolean) => void] {
  const t = useT();
  const { toast } = useApp();
  const [on, setOn] = useState<boolean | null>(null);
  useEffect(() => {
    setOn(null);
    if (!can) return;
    api.background(id).then((r) => setOn(r.on), () => setOn(null));
  }, [id, can]);
  const set = (v: boolean) => {
    setOn(v);
    api.setBackground(id, v).then(
      (r) => setOn(r.on),
      (e: unknown) => {
        setOn(!v);
        toast(t('backgroundChangeFailed', { phone: id, reason: e instanceof Error ? e.message : String(e) }), 'error');
      },
    );
  };
  return [on, set];
}

export function PhoneDetail({ id }: { id: string }) {
  const t = useT();
  const gw = useGw();
  const { device, task, running, needs } = usePhone(id);
  const hasBackground = useHas('background');
  const hasAgentStream = useHas('agentStream');
  const [background, setBackground] = useBackground(id, !!device?.features.background && hasBackground);
  const [agentScreen, setAgentScreen] = useState(false);
  const [chatCollapsed, setChatCollapsed] = useChatCollapsed();
  useEffect(() => setAgentScreen(false), [id]);

  if (!device) return null;
  const agentName = task?.agent || t('agentGeneric');
  const offerAgentScreen = !!background && hasAgentStream && device.status !== 'offline';
  const showingAgent = offerAgentScreen && agentScreen;
  const step = markedStep(task);
  const noMirror = device.status === 'offline' || device.status === 'needs_setup' || device.status === 'locked';
  const chatToggle = (
    <Tooltip title={chatCollapsed ? t('chatExpand') : t('chatCollapse')}>
      <IconButton aria-label={chatCollapsed ? t('chatExpand') : t('chatCollapse')} onClick={() => setChatCollapsed(!chatCollapsed)}>
        <ChatIcon fontSize="small" />
      </IconButton>
    </Tooltip>
  );
  return (
    <Box sx={{ display: 'flex', flex: 1, minHeight: 0 }}>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <PageHeader
          title={
            <Stack direction="row" alignItems="center" spacing={1} component="span">
              <PlatformIcon platform={device.platform} />
              <span>{device.name}</span>
            </Stack>
          }
          actions={
            <>
              <PhoneActions device={device} task={task} running={running} background={background} onBackground={setBackground} />
              {chatToggle}
            </>
          }
        >
          <Stack direction="row" alignItems="center" spacing={0.75} sx={{ minWidth: 0 }}>
            <StatusDot status={device.status} />
            <Typography variant="body2" color="text.secondary" noWrap>
              {gw.statusLine(device)}
            </Typography>
          </Stack>
        </PageHeader>
        <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
          <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', p: 2, pb: 0 }}>
            <StatusBanner device={device} task={task} needs={needs} />
            {noMirror ? (
              <PhoneStatePanel device={device} />
            ) : (
              <>
                <Box sx={{ flex: 1, minHeight: 0, pt: 2 }}>
                  <Mirror
                    deviceId={id}
                    agent={showingAgent}
                    label={
                      showingAgent
                        ? t('mirrorAgentScreenLabel', { agent: agentName, phone: device.name })
                        : t('mirrorScreenLabel', { phone: device.name })
                    }
                    target={showingAgent === !!background ? step?.target : undefined}
                    point={showingAgent === !!background ? step?.point : undefined}
                    generation={device.state}
                  />
                </Box>
                <ControlBar
                  device={device}
                  agent={showingAgent}
                  agentName={agentName}
                  showAgentScreen={offerAgentScreen}
                  onAgentScreen={setAgentScreen}
                />
              </>
            )}
          </Box>
          <Box
            component="aside"
            aria-label={t('needsAndTasksLabel')}
            sx={{ width: 360, flexShrink: 0, borderLeft: 1, borderColor: 'divider', overflowY: 'auto', p: 2 }}
          >
            <Stack spacing={2}>
              <NeedsList needs={needs} shotFor={(n) => n.deviceId !== id || noMirror} />
              <Box>
                <Typography variant="subtitle2" sx={{ mb: 1 }}>
                  {t('tasksSection')}
                </Typography>
                {task ? (
                  <TaskCard task={task} />
                ) : (
                  <Typography variant="body1" color="text.secondary">
                    {t('tasksNone')}
                  </Typography>
                )}
              </Box>
              {device.platform === 'android' && device.features.companion && <CompanionCard device={device} />}
            </Stack>
          </Box>
        </Box>
      </Box>
      {!chatCollapsed && (
        <Box sx={{ width: 380, flexShrink: 0, borderLeft: 1, borderColor: 'divider', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
          <ChatPanel deviceId={id} onCollapse={() => setChatCollapsed(true)} />
        </Box>
      )}
    </Box>
  );
}
