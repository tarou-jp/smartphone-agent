// One tool call of a run, as Cursor shows one: a quiet line with the verb
// and what it acted on, a spinner while it runs, and the result on click.

import { useState } from 'react';
import { Box, ButtonBase, CircularProgress, Collapse, Typography, useTheme } from '@mui/material';
import ChevronRightIcon from '@mui/icons-material/ChevronRightRounded';
import CloseIcon from '@mui/icons-material/CloseRounded';
import TouchIcon from '@mui/icons-material/TouchAppOutlined';
import EyeIcon from '@mui/icons-material/VisibilityOutlined';
import KeyboardIcon from '@mui/icons-material/KeyboardOutlined';
import SwipeIcon from '@mui/icons-material/SwipeOutlined';
import LaunchIcon from '@mui/icons-material/LaunchRounded';
import LinkIcon from '@mui/icons-material/LinkRounded';
import SearchIcon from '@mui/icons-material/SearchRounded';
import HandIcon from '@mui/icons-material/PanToolOutlined';
import TimerIcon from '@mui/icons-material/HourglassEmptyRounded';
import DoneIcon from '@mui/icons-material/DoneAllRounded';
import BuildIcon from '@mui/icons-material/BuildOutlined';
import type { T } from '@/i18n';
import { useT } from '@/i18n';
import type { ToolStep } from './model';

const tools: Record<string, { key: string; icon: typeof TouchIcon; target?: (a: Record<string, unknown>) => unknown }> = {
  phone_connect: { key: 'webToolConnect', icon: LinkIcon, target: (a) => a.task },
  phone_observe: { key: 'webToolObserve', icon: EyeIcon },
  phone_tap: { key: 'webToolTap', icon: TouchIcon, target: (a) => (a.ref ? `#${a.ref}` : a.x !== undefined ? `${a.x}, ${a.y}` : '') },
  phone_type: { key: 'webToolType', icon: KeyboardIcon, target: (a) => (a.text ? `“${a.text}”` : '') },
  phone_swipe: { key: 'webToolSwipe', icon: SwipeIcon, target: (a) => a.direction },
  phone_key: { key: 'webToolKey', icon: KeyboardIcon, target: (a) => a.key },
  phone_launch: { key: 'webToolLaunch', icon: LaunchIcon, target: (a) => a.app },
  phone_apps: { key: 'webToolApps', icon: SearchIcon, target: (a) => a.query },
  phone_handoff: { key: 'webToolHandoff', icon: HandIcon, target: (a) => a.reason },
  phone_wait_for: { key: 'webToolWaitFor', icon: TimerIcon, target: (a) => (a.text ? `“${a.text}”` : '') },
  phone_release: { key: 'webToolRelease', icon: DoneIcon, target: (a) => a.summary },
};

/** Whether a result's status is a failure, and which word says so. */
function failure(t: T, status?: string): string | null {
  if (!status) return null;
  if (status.startsWith('error')) return t('webToolFailed');
  if (status === 'approval_denied') return t('webToolDenied');
  if (status === 'blocked_by_policy') return t('webToolBlocked');
  return null;
}

/** The verb, what it acted on, and its icon. */
export function describeTool(t: T, s: ToolStep) {
  const d = tools[s.tool];
  const args = s.args ?? {};
  const target = d?.target?.(args);
  return {
    verb: d ? t(d.key) : s.tool.replace(/^phone_/, '').replace(/_/g, ' '),
    // The agent's own words for the step say the most; else what it acted on.
    target: s.intent || (target === undefined || target === null ? '' : String(target)),
    Icon: d?.icon ?? BuildIcon,
  };
}

export function ToolRow({ step }: { step: ToolStep }) {
  const t = useT();
  const p = useTheme().pinky;
  const [open, setOpen] = useState(false);
  const { verb, target, Icon } = describeTool(t, step);
  const failed = failure(t, step.status);
  const args = Object.entries(step.args ?? {}).filter(([, v]) => v !== '' && v !== false && v !== 0 && v !== null);
  const hasMore = !!step.status || args.length > 0;
  return (
    <Box>
      <ButtonBase
        disabled={!hasMore}
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        sx={{
          width: '100%',
          justifyContent: 'flex-start',
          gap: 0.75,
          px: 0.75,
          py: 0.375,
          borderRadius: 1,
          color: p.ink3,
          textAlign: 'left',
          '&:hover': { bgcolor: 'action.hover', '& .tool-chev': { opacity: 1 } },
          '&.Mui-disabled': { color: p.ink3 },
        }}
      >
        <Icon sx={{ fontSize: 14, flexShrink: 0, color: failed ? p.red : p.ink3 }} />
        <Typography noWrap sx={{ fontSize: 12.5, minWidth: 0, flex: '0 1 auto' }}>
          <Box component="span" sx={{ color: failed ? p.red : p.ink2, fontWeight: 550 }}>
            {verb}
          </Box>
          {target && (
            <Box component="span" sx={{ ml: 0.75 }}>
              {target}
            </Box>
          )}
        </Typography>
        {step.running && <CircularProgress size={10} thickness={6} sx={{ color: p.ink3, flexShrink: 0 }} />}
        {failed && (
          <Box component="span" sx={{ display: 'inline-flex', alignItems: 'center', color: p.red, fontSize: 11.5, flexShrink: 0 }}>
            <CloseIcon sx={{ fontSize: 13 }} />
            {failed}
          </Box>
        )}
        <Box sx={{ flex: 1 }} />
        {hasMore && (
          <ChevronRightIcon
            className="tool-chev"
            sx={{ fontSize: 15, opacity: open ? 1 : 0, flexShrink: 0, transition: 'transform .15s, opacity .15s', transform: open ? 'rotate(90deg)' : 'none' }}
          />
        )}
      </ButtonBase>
      <Collapse in={open} unmountOnExit>
        <Box
          sx={{
            ml: 3,
            mr: 0.75,
            mb: 0.5,
            px: 1,
            py: 0.75,
            borderRadius: 1,
            bgcolor: p.bg2,
            border: 1,
            borderColor: p.line,
            fontFamily: 'ui-monospace, Menlo, monospace',
            fontSize: 11.5,
            lineHeight: 1.5,
            color: p.ink2,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            userSelect: 'text',
          }}
        >
          {args.map(([k, v]) => (
            <div key={k}>
              <span style={{ opacity: 0.7 }}>{k}: </span>
              {typeof v === 'object' ? JSON.stringify(v) : String(v)}
            </div>
          ))}
          {step.status && <div style={{ marginTop: args.length ? 6 : 0 }}>→ {step.status}</div>}
          {step.detail && <div>{step.detail}</div>}
        </Box>
      </Collapse>
    </Box>
  );
}
