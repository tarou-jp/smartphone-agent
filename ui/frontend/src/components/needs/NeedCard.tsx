import { useEffect, useState } from 'react';
import { Box, Stack, Typography, useTheme } from '@mui/material';
import { StatusText } from '@/components/common';
import type { Need } from '@/api/types';
import { useT } from '@/i18n';
import { clock } from '@/lib/format';
import { useI18n } from '@/i18n';
import { Countdown } from './Countdown';
import { NeedShot } from './NeedShot';
import { AgentRequestControls, ApprovalControls, HandoffControls, UnblockControls, UnlockControls } from './controls';
import { isNewAgent, ruleName } from './words';
import { useGw } from '@/lib/gwtext';

/** How much of the time to answer is left, as a bar along the card's top
 * edge that drains toward the automatic answer. */
function DeadlineBar({ created, deadline }: { created: string; deadline: string }) {
  const p = useTheme().pinky;
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);
  const start = new Date(created).getTime();
  const end = new Date(deadline).getTime();
  const left = end > start ? Math.min(1, Math.max(0, (end - now) / (end - start))) : 0;
  return (
    <Box aria-hidden sx={{ position: 'absolute', top: 0, left: 0, right: 0, height: 3, bgcolor: p.amberLine }}>
      <Box
        sx={{
          height: '100%',
          width: '100%',
          bgcolor: p.amberDot,
          transformOrigin: 'left',
          transform: `scaleX(${left})`,
          transition: 'transform 1s linear',
        }}
      />
    </Box>
  );
}

/**
 * A Needs you card: what the agent asks, and the buttons that answer it.
 * showShot is false where the mirror next to it already shows the target.
 */
export function NeedCard({ need, showShot = true, onLater }: { need: Need; showShot?: boolean; onLater?: () => void }) {
  const t = useT();
  const { locale } = useI18n();
  const p = useTheme().pinky;
  const gw = useGw();
  const newAgent = isNewAgent(need);
  const eyebrow = need.deviceName ? need.deviceName + (need.app ? ' · ' + need.app : '') : t('needsFirstConnection');
  let detail = gw.needDetail(need);
  if (newAgent && need.created) {
    detail = [detail, t('agentRequestAskedAt', { time: clock(locale, need.created) })].filter(Boolean).join(' ');
  }
  let controls;
  if (need.kind === 'unlock') controls = <UnlockControls need={need} onLater={onLater ?? (() => undefined)} />;
  else if (need.kind === 'handoff') controls = <HandoffControls need={need} />;
  else if (newAgent) controls = <AgentRequestControls need={need} />;
  else if (need.unblock) controls = <UnblockControls need={need} />;
  else
    controls = (
      <>
        {showShot && need.hasScreenshot && (
          <Box sx={{ mt: 1.5, display: 'flex', justifyContent: 'center' }}>
            <NeedShot need={need} />
          </Box>
        )}
        <ApprovalControls need={need} />
      </>
    );
  return (
    <Box
      role="group"
      aria-label={gw.needTitle(need)}
      sx={{
        position: 'relative',
        overflow: 'hidden',
        p: 2.5,
        pt: need.deadline ? 3 : 2.5,
        borderRadius: '16px',
        bgcolor: p.amberTint,
        border: `1px solid ${p.amberLine}`,
        boxShadow: p.cardShadow,
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {need.deadline && <DeadlineBar created={need.created} deadline={need.deadline} />}
      <Stack direction="row" alignItems="center" spacing={1}>
        <Box sx={{ width: 7, height: 7, borderRadius: '50%', bgcolor: p.amberDot, flexShrink: 0 }} />
        <Typography noWrap sx={{ fontSize: 12, fontWeight: 500, color: p.amber, flex: 1, minWidth: 0 }}>
          {eyebrow}
        </Typography>
        {need.deadline && <Countdown deadline={need.deadline} words={need.kind === 'handoff' ? 'panelEndsIn' : 'panelDeniesIn'} />}
      </Stack>
      <Typography sx={{ fontSize: 20, fontWeight: 600, lineHeight: 1.25, mt: 1.25, mb: 0.5 }} className="selectable">
        {gw.needTitle(need)}
      </Typography>
      {detail && (
        <Typography variant="body1" color="text.secondary" className="selectable">
          {detail}
        </Typography>
      )}
      {need.rule && !newAgent && need.kind === 'approval' && (
        <Box sx={{ mt: 1 }}>
          <StatusText>{ruleName(t, need.rule)}</StatusText>
        </Box>
      )}
      {controls}
    </Box>
  );
}
