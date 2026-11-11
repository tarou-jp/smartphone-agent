// The buttons of each kind of Needs you card, after ui/rightpanel.go.

import { useState } from 'react';
import {
  Box,
  Button,
  ButtonGroup,
  Link,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import ArrowDropDownIcon from '@mui/icons-material/ArrowDropDown';
import { api } from '@/api/client';
import type { Decision, Need, ScopeView } from '@/api/types';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import { agentOf, ruleAction, ruleName } from './words';
import { useUnlock, useUnlockLabel } from '@/components/phone/useUnlock';

function useDecide() {
  const t = useT();
  const { run } = useApp();
  return (n: Need, d: Decision, note = '') => run(t('callAnswer'), () => api.decide(n.id, d, note.trim()));
}

const Hint = ({ children }: { children: React.ReactNode }) => (
  <Typography variant="body1" color="text.secondary" sx={{ mt: 1 }}>
    {children}
  </Typography>
);

function NoteField({ label, placeholder, value, onChange }: { label: string; placeholder: string; value: string; onChange: (v: string) => void }) {
  return (
    <TextField
      label={label}
      placeholder={placeholder}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      multiline
      minRows={2}
      maxRows={4}
      autoFocus
      fullWidth
      sx={{ mt: 1.5, bgcolor: 'background.paper' }}
    />
  );
}

/** Deny, Allow and More options, or the note of Deny with a note. */
export function ApprovalControls({ need }: { need: Need }) {
  const t = useT();
  const decide = useDecide();
  const [noting, setNoting] = useState(false);
  const [note, setNote] = useState('');
  const [menu, setMenu] = useState<HTMLElement | null>(null);
  const agent = agentOf(t, need);
  const kind = ruleAction(t, need.rule);
  return (
    <>
      {noting && (
        <NoteField label={t('needsNoteFor', { agent })} placeholder={t('needsDenyNotePlaceholder')} value={note} onChange={setNote} />
      )}
      <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
        <Button fullWidth variant="outlined" color="inherit" onClick={() => decide(need, 'deny', note)}>
          {noting ? t('panelDenyWithNote') : t('needsDeny')}
        </Button>
        {noting ? (
          <Button fullWidth color="inherit" onClick={() => setNoting(false)}>
            {t('panelCancel')}
          </Button>
        ) : (
          <ButtonGroup variant="contained" fullWidth>
            <Button onClick={() => decide(need, 'allow_once')} sx={{ flex: 1 }}>
              {t('needsAllow')}
            </Button>
            <Button aria-label={t('needsMoreOptions')} onClick={(e) => setMenu(e.currentTarget)} sx={{ flex: 0, px: 0.5, minWidth: 0 }}>
              <ArrowDropDownIcon fontSize="small" />
            </Button>
          </ButtonGroup>
        )}
      </Stack>
      <Menu anchorEl={menu} open={!!menu} onClose={() => setMenu(null)}>
        {need.allowTask && (
          <MenuItem onClick={() => (setMenu(null), decide(need, 'allow_task'))}>{t('needsAllowTask')}</MenuItem>
        )}
        {need.alwaysAllow && (
          <MenuItem onClick={() => (setMenu(null), decide(need, 'allow_always'))}>
            {need.app ? t('needsAlwaysAllowIn', { kind, app: need.app }) : t('needsAlwaysAllow', { kind })}
          </MenuItem>
        )}
        <MenuItem onClick={() => (setMenu(null), decide(need, 'take_over'))}>
          <ListItemText primary={t('phoneTakeOver')} secondary={t('needsTakeOverTooltip')} />
        </MenuItem>
        <MenuItem onClick={() => (setMenu(null), setNoting(true))}>{t('needsDenyWithNote')}</MenuItem>
      </Menu>
    </>
  );
}

/** An agent's wish to open a blocked app: Deny, or allow it for the task. */
export function UnblockControls({ need }: { need: Need }) {
  const t = useT();
  const decide = useDecide();
  return (
    <>
      <Hint>
        {t('needsUnblockHow', {
          app: need.app || t('panelThisApp'),
          rule: ruleName(t, need.rule ?? ''),
          agent: agentOf(t, need),
        })}
      </Hint>
      <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
        <Button fullWidth variant="outlined" color="inherit" onClick={() => decide(need, 'deny')}>
          {t('needsDeny')}
        </Button>
        <Button fullWidth variant="contained" onClick={() => decide(need, 'allow_task')}>
          {t('needsAllowTask')}
        </Button>
      </Stack>
    </>
  );
}

const scopeModes = [
  { mode: 'everything', label: 'scopeEverything', hint: 'scopeHintEverything' },
  { mode: 'read_only', label: 'scopeReadOnly', hint: 'scopeHintReadOnly' },
  { mode: 'watch_only', label: 'scopeWatchOnly', hint: 'scopeHintWatchOnly' },
  { mode: 'apps', label: 'scopeApps', hint: 'scopeHintApps' },
] as const;

/** An agent's first connection: what it may do, then Deny or Approve. */
export function AgentRequestControls({ need }: { need: Need }) {
  const t = useT();
  const { run } = useApp();
  const canScope = useHas('agentScope');
  const decide = useDecide();
  const agent = agentOf(t, need);
  const [mode, setMode] = useState<ScopeView['mode']>(need.readOnly ? 'read_only' : 'everything');
  const [apps, setApps] = useState('');
  const appList = apps
    .split(/[,、，]/)
    .map((s) => s.trim())
    .filter(Boolean);
  const noApps = mode === 'apps' && appList.length === 0;
  const approve = () =>
    run(t('settingsAgentsApprove'), async () => {
      if (canScope && need.agent) await api.setAgentScope(need.agent, { mode, apps: mode === 'apps' ? appList : [] });
      await api.decide(need.id, 'allow_once');
    });
  const hint = scopeModes.find((s) => s.mode === mode)?.hint;
  return (
    <>
      {need.readOnly && <Hint>{t('needsAgentAskedReadOnly', { agent })}</Hint>}
      {canScope && (
        <Box sx={{ mt: 1.5 }}>
          <TextField
            select
            fullWidth
            label={t('needsLetAgent', { agent })}
            value={mode}
            onChange={(e) => setMode(e.target.value)}
            sx={{ bgcolor: 'background.paper' }}
          >
            {scopeModes.map((s) => (
              <MenuItem key={s.mode} value={s.mode}>
                {t(s.label)}
              </MenuItem>
            ))}
          </TextField>
          {mode === 'apps' && (
            <TextField
              fullWidth
              value={apps}
              onChange={(e) => setApps(e.target.value)}
              placeholder={t('settingsAgentsAppsPlaceholder')}
              sx={{ mt: 1, bgcolor: 'background.paper' }}
            />
          )}
          {hint && (
            <Typography variant="caption" component="p" sx={{ mt: 1 }}>
              {t(hint)}
            </Typography>
          )}
        </Box>
      )}
      <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
        <Button fullWidth variant="outlined" color="inherit" onClick={() => decide(need, 'deny')}>
          {t('needsDeny')}
        </Button>
        <Tooltip title={noApps ? t('needsApproveNoApps') : ''}>
          <span style={{ flex: 1, display: 'flex' }}>
            <Button fullWidth variant="contained" disabled={noApps} onClick={approve}>
              {t('needsApprove')}
            </Button>
          </span>
        </Tooltip>
      </Stack>
    </>
  );
}

/** A handoff: the agent asked the person to do something on the phone. */
export function HandoffControls({ need }: { need: Need }) {
  const t = useT();
  const { live, device } = useApp();
  const decide = useDecide();
  const [unlock, unlockDialog] = useUnlock();
  const unlockLabel = useUnlockLabel();
  const [noting, setNoting] = useState(false);
  const [note, setNote] = useState('');
  const agent = agentOf(t, need);
  const d = device(need.deviceId);
  const locked = d?.status === 'locked' || live.needs.some((n) => n.kind === 'unlock' && n.deviceId === need.deviceId);
  const holding = live.tasks.some((x) => x.id === need.taskId && x.state === 'paused');
  const setup = d?.status === 'needs_setup';
  const canUnlock = locked && !!d?.features.unlock;
  let how: string;
  if (setup) how = t('needsHandoffSetup', { agent });
  else if (holding && !locked) how = t('needsHandoffInControl', { agent });
  else if (canUnlock) how = need.endsOnUnlock ? t('needsHandoffLockedHere', { agent }) : t('needsHandoffLockedHereThenDone', { agent });
  else if (locked) how = t('needsHandoffLocked', { how: t('unlockOnPhone', { end: t('needsThenClickDone') }) });
  else how = t('needsHandoffHowTo');
  const declineButton = locked || holding || setup;
  return (
    <>
      {canUnlock && d && (
        <Button fullWidth variant="contained" sx={{ mt: 1.5 }} onClick={() => unlock(d)}>
          {unlockLabel}
        </Button>
      )}
      <Hint>{how}</Hint>
      {noting && (
        <NoteField label={t('needsNoteFor', { agent })} placeholder={t('needsHandoffNotePlaceholder')} value={note} onChange={setNote} />
      )}
      <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
        {declineButton ? (
          <Tooltip title={t('needsDeclineTooltip', { agent })}>
            <Button fullWidth variant="outlined" color="inherit" onClick={() => decide(need, 'deny', note)}>
              {t('needsDecline')}
            </Button>
          </Tooltip>
        ) : (
          <Tooltip title={t('needsTakeOverTooltip')}>
            <Button fullWidth variant="outlined" color="inherit" onClick={() => decide(need, 'take_over')}>
              {t('phoneTakeOver')}
            </Button>
          </Tooltip>
        )}
        <Tooltip title={t('needsDoneTooltip', { agent })}>
          <Button fullWidth variant={locked ? 'outlined' : 'contained'} onClick={() => decide(need, 'allow_once', note)}>
            {t('needsDone')}
          </Button>
        </Tooltip>
      </Stack>
      <Stack direction="row" spacing={2} sx={{ mt: 1 }}>
        {!noting && (
          <Link component="button" variant="body2" onClick={() => setNoting(true)}>
            {t('needsAddNote')}
          </Link>
        )}
        {!declineButton && (
          <Link component="button" variant="body2" color="text.secondary" onClick={() => decide(need, 'deny', note)}>
            {t('needsDecline')}
          </Link>
        )}
      </Stack>
      {unlockDialog}
    </>
  );
}

/** An unlock card: Unlock from here, or Not now. */
export function UnlockControls({ need, onLater }: { need: Need; onLater: () => void }) {
  const t = useT();
  const { device } = useApp();
  const [unlock, unlockDialog] = useUnlock();
  const unlockLabel = useUnlockLabel();
  const d = device(need.deviceId);
  const can = !!d?.features.unlock;
  return (
    <>
      <Hint>{can ? t('unlockFromHere') : t('unlockOnPhone', { end: t('unlockEnd') })}</Hint>
      <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
        <Button
          fullWidth
          variant="outlined"
          color="inherit"
          onClick={onLater}
          aria-label={t('unlockNotNowLabel', { name: need.deviceName })}
        >
          {t('unlockNotNow')}
        </Button>
        {can && d && (
          <Button fullWidth variant="contained" onClick={() => unlock(d)}>
            {unlockLabel}
          </Button>
        )}
      </Stack>
      {unlockDialog}
    </>
  );
}
