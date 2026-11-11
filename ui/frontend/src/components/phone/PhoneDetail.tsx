// One phone's page: header, the mirror with its keys, and Needs you and
// the task on the right. The Agent panel is the window's, beside it.

import { useEffect, useState } from 'react';
import { Box, Stack, Typography, useTheme } from '@mui/material';
import { api } from '@/api/client';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import { PageHeader, PhoneGlyph } from '@/components/common';
import { NeedsList } from '@/components/needs/NeedsList';
import { Mirror } from './Mirror';
import { ControlBar } from './ControlBar';
import { StatusBanner } from './StatusBanner';
import { TaskCard } from './TaskCard';
import { CompanionCard } from './CompanionCard';
import { PhoneStatePanel } from './PhoneStatePanel';
import { PhoneActions } from './PhoneActions';
import { markedStep, usePhone } from './usePhone';
import { useGw } from '@/lib/gwtext';
import { useWidth } from '@/hooks/useWidth';

// Needs you narrows when the page is narrow, so the mirror keeps room.
const asideWidth = 380;
const asideNarrow = 300;
const mirrorMin = 280;

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
  const p = useTheme().pinky;
  const { device, task, running, needs } = usePhone(id);
  const hasBackground = useHas('background');
  const hasAgentStream = useHas('agentStream');
  const [background, setBackground] = useBackground(id, !!device?.features.background && hasBackground);
  const [agentScreen, setAgentScreen] = useState(false);
  const [rootRef, width] = useWidth<HTMLDivElement>();
  useEffect(() => setAgentScreen(false), [id]);

  if (!device) return null;
  const agentName = task?.agent || t('agentGeneric');
  const offerAgentScreen = !!background && hasAgentStream && device.status !== 'offline';
  const showingAgent = offerAgentScreen && agentScreen;
  const step = markedStep(task);
  const noMirror = device.status === 'offline' || device.status === 'needs_setup' || device.status === 'locked';
  const aside = width === 0 || width >= asideWidth + mirrorMin ? asideWidth : asideNarrow;
  return (
    <Box ref={rootRef} sx={{ display: 'flex', flex: 1, minHeight: 0, minWidth: 0, position: 'relative' }}>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <PageHeader
          title={
            <Stack direction="row" alignItems="center" spacing={1.25} component="span">
              <PhoneGlyph status={device.status} size={20} />
              <span>{device.name}</span>
            </Stack>
          }
          actions={<PhoneActions device={device} task={task} running={running} background={background} onBackground={setBackground} />}
        >
          <Typography variant="body2" noWrap sx={{ color: device.status === 'needs_you' ? p.amber : p.ink3 }}>
            {gw.statusLine(device)}
          </Typography>
        </PageHeader>
        <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
          <Box sx={{ flex: 1, minWidth: 220, display: 'flex', flexDirection: 'column', px: 3, pt: 2, pb: 1 }}>
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
            sx={{ width: aside, flexShrink: 0, borderLeft: 1, borderColor: 'divider', overflowY: 'auto', p: 2.5 }}
          >
            <Stack spacing={3}>
              <NeedsList needs={needs} shotFor={(n) => n.deviceId !== id || noMirror} />
              <Box>
                <Typography sx={{ fontSize: 12, color: p.ink3, mb: 1.25 }}>{t('tasksSection')}</Typography>
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
    </Box>
  );
}
