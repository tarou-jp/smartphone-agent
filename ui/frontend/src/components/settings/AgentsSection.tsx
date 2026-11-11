// Settings › Agents: one list of the agents on this computer and the
// agents that have connected. Each agent has one row, with its mark, where
// it stands, what it may do, and the one thing to do next. (There were two
// lists, and a connected agent showed in both.)

import { useEffect, useState } from 'react';
import { Alert, Box, Button, CircularProgress, Link, Stack, Typography } from '@mui/material';
import { CardRows, Section, StatusText } from '@/components/common';
import { api } from '@/api/client';
import { useApp, useHas } from '@/state/AppState';
import { useT, type T } from '@/i18n';
import type { AgentSetupView, AgentView } from '@/api/types';
import { AgentMark, agentKey } from './agents/AgentMark';
import { ScopeForm, scopeLine } from './agents/AgentScopeEditor';
import { RemoteAgents } from './agents/RemoteAgents';
import { errorText } from './controls';

/**
 * Pinky's own chat records its tasks as an agent named Pinky. It is part of
 * the app, not an agent the person connects or limits, so it is not listed.
 */
const isPinky = (name: string) => agentKey(name) === 'pinky';

/** The agents seen in tasks, for gateways that do not list them. */
function seenAgents(names: string[]): AgentView[] {
  return [...new Set(names.filter(Boolean))].map((name) => ({ name, approved: true, scope: { mode: 'everything', apps: [] } }));
}

function doneText(t: T, a: AgentSetupView, r: { restart: string; opened: string }, removed: boolean): string {
  if (removed) return r.restart === 'app' ? t('connectRemovedRestart', { agent: a.name }) : t('connectRemoved');
  if (r.opened) return t('connectDoneOpened', { agent: a.name });
  if (r.restart === 'app') return t('connectDoneRestart', { agent: a.name });
  if (r.restart === 'session') return t('connectDoneSession', { agent: a.name });
  return t('connectDoneNow', { agent: a.name });
}

/** An agent: how this computer's copy is set up, and what the gateway knows of it. */
interface Item {
  name: string;
  setup?: AgentSetupView;
  agent?: AgentView;
}

// Needs a look first, then the connected ones, then the rest.
function rank(i: Item): number {
  if (i.setup?.state === 'outdated' || (i.agent && !i.agent.approved)) return 0;
  if (i.agent || i.setup?.state === 'connected') return 1;
  return 2;
}

function AgentItem({ item, onChanged }: { item: Item; onChanged: () => void }) {
  const t = useT();
  const { toast } = useApp();
  const canScope = useHas('agentScope');
  const { setup: s, agent: a, name } = item;
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState(false);
  // What a change did, and the one thing left to do (restart the agent,
  // open a new session), goes by as a toast; the row itself already shows
  // the new state. Only a failure stays under the row, until the next try.
  const [failure, setFailure] = useState('');
  const act = async (op: 'connect' | 'remove') => {
    if (!s) return;
    setBusy(true);
    setFailure('');
    try {
      const r = await api.agentSetupDo(s.id, op);
      toast(doneText(t, s, r, op === 'remove'), 'success');
      onChanged();
    } catch (e) {
      setFailure(t('connectFailed', { error: errorText(e) }));
    } finally {
      setBusy(false);
    }
  };

  // One line says where the agent stands.
  let tone: 'good' | 'warn' | 'muted' = 'muted';
  let line = t('connectStateNotConnected');
  if (s?.state === 'outdated') {
    tone = 'warn';
    line = t('connectCardBroken', { agent: name });
  } else if (a && !a.approved) {
    tone = 'warn';
    line = t('settingsAgentsNotAllowed');
  } else if (a) {
    tone = 'good';
    line = scopeLine(t, a.scope);
  } else if (s?.state === 'connected') {
    tone = 'good';
    line = s.extension ? t('connectStateExtension', { agent: name }) : t('connectStateConnected');
  }
  const canConnect = !!s && !s.manual && s.state !== 'connected';
  const canRemove = !!s && !s.manual && !s.extension && s.state !== 'not_connected';
  const dim = !a && s?.state === 'not_connected';
  return (
    <Box sx={{ px: 2, py: 1.75 }}>
      <Stack direction="row" alignItems="center" spacing={1.75}>
        <Box sx={{ opacity: dim ? 0.7 : 1, display: 'flex' }}>
          <AgentMark id={s?.id} name={name} />
        </Box>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="subtitle1" noWrap>
            {name}
          </Typography>
          <Box sx={{ mt: 0.25, minWidth: 0, overflow: 'hidden' }}>
            <StatusText tone={tone}>{line}</StatusText>
          </Box>
        </Box>
        {busy && <CircularProgress size={16} aria-label={t('connectWorking', { agent: name })} />}
        {canRemove && (
          <Button color="inherit" disabled={busy} onClick={() => act('remove')} aria-label={t('connectRemoveLabel', { agent: name })}>
            {t('connectRemove')}
          </Button>
        )}
        {canScope && a?.approved && !editing && (
          <Button variant="outlined" color="inherit" aria-label={t('settingsAgentsChangeLabel', { agent: name })} onClick={() => setEditing(true)}>
            {t('settingsChange')}
          </Button>
        )}
        {canConnect && (
          <Button
            variant={s.state === 'outdated' ? 'contained' : 'outlined'}
            color={s.state === 'outdated' ? 'primary' : 'inherit'}
            disabled={busy}
            onClick={() => act('connect')}
            aria-label={s.state === 'outdated' ? t('connectFixLabel', { agent: name }) : t('connectConnectLabel', { agent: name })}
            sx={{ minWidth: 72 }}
          >
            {s.state === 'outdated' ? t('connectFix') : t('connectConnect')}
          </Button>
        )}
      </Stack>
      {/* Notes line up under the name, past the mark. */}
      <Box sx={{ pl: '50px' }}>
        {s?.problem && (
          <Typography variant="body2" color="error" sx={{ mt: 1 }}>
            {t('connectProblem', { problem: s.problem })}
          </Typography>
        )}
        {s?.manual && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
            {t('connectManual')}
          </Typography>
        )}
        {failure && (
          <Typography variant="body2" role="status" sx={{ mt: 1, color: 'error.main' }}>
            {failure}
          </Typography>
        )}
        {editing && a && (
          <ScopeForm
            a={a}
            onDone={(changed) => {
              setEditing(false);
              if (changed) onChanged();
            }}
          />
        )}
      </Box>
    </Box>
  );
}

export function AgentsSection() {
  const t = useT();
  const { live } = useApp();
  const canList = useHas('agents');
  const canSetup = useHas('agentSetup');
  const [agents, setAgents] = useState<AgentView[] | null>(null);
  const [setups, setSetups] = useState<AgentSetupView[] | null>(null);
  const [err, setErr] = useState('');
  const load = () => {
    const fail = (e: unknown) => setErr(errorText(e));
    if (canList) api.agents().then((l) => (setAgents(l), setErr('')), fail);
    if (canSetup) api.agentSetup().then((l) => (setSetups(l), setErr('')), fail);
  };
  // A new agent's request shows up as a Needs you item; read the lists again.
  useEffect(load, [live.needs.length, canList, canSetup]);

  const known = (canList ? agents : seenAgents(live.tasks.map((x) => x.agent)))?.filter((a) => !isPinky(a.name));
  // The agents the gateway knows show at once; the ones found on this
  // computer join them when the search is done.
  const searching = canSetup && !setups;
  const left = new Map((known ?? []).map((a) => [agentKey(a.name), a]));
  const items: Item[] = [];
  for (const s of setups ?? []) {
    if (!s.found) continue;
    const a = left.get(agentKey(s.name));
    if (a) left.delete(agentKey(s.name));
    items.push({ name: s.name, setup: s, agent: a });
  }
  for (const a of left.values()) items.push({ name: a.name, agent: a });
  items.sort((x, y) => rank(x) - rank(y));
  const missing = (setups ?? []).filter((s) => !s.found);

  return (
    <>
      <Section title={t('settingsSectionAgents')} description={t('settingsAgentsIntro')}>
        {!canList && (
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
            {t('settingsAgentsUnlisted')}
          </Typography>
        )}
        {err && (
          <Alert severity="error" sx={{ mb: 1.5 }}>
            {err}
          </Alert>
        )}
        {searching && !err && items.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {t('connectLooking')}
          </Typography>
        )}
        {!searching && !(canList && !agents) && items.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {canSetup ? t('connectNoneFound') : t('settingsAgentsNoneYet')}
          </Typography>
        )}
        {[
          { label: t('settingsAgentsList'), hint: '', rows: items.filter((i) => rank(i) < 2) },
          { label: t('connectTitle'), hint: t('connectIntro'), rows: items.filter((i) => rank(i) === 2) },
        ]
          .filter((g) => g.rows.length > 0)
          .map((g, n) => (
            <Box key={g.label} sx={{ mt: n ? 4 : 0 }}>
              <Typography variant="subtitle2" sx={{ mb: g.hint ? 0.5 : 1.25 }}>
                {g.label}
              </Typography>
              {g.hint && (
                <Typography variant="caption" component="p" sx={{ mb: 1.25 }}>
                  {g.hint}
                </Typography>
              )}
              <CardRows>
                {g.rows.map((i) => (
                  <AgentItem key={i.setup?.id ?? i.name} item={i} onChanged={load} />
                ))}
              </CardRows>
            </Box>
          ))}
        {searching && !err && items.length > 0 && (
          <Typography variant="caption" component="p" sx={{ mt: 1.5 }}>
            {t('connectLooking')}
          </Typography>
        )}
        {missing.length > 0 && (
          <Box sx={{ mt: 4 }}>
            <Typography variant="subtitle2" sx={{ mb: 1.25 }}>
              {t('webAgentsNotHere')}
            </Typography>
            <Stack direction="row" flexWrap="wrap" useFlexGap spacing={1}>
              {missing.map((s) => (
                <Stack
                  key={s.id}
                  direction="row"
                  alignItems="center"
                  spacing={1}
                  component={s.url ? Link : 'span'}
                  {...(s.url ? { href: s.url, target: '_blank', rel: 'noreferrer', underline: 'none', 'aria-label': t('connectGet', { agent: s.name }) } : {})}
                  sx={{
                    pl: 0.5,
                    pr: 1.5,
                    py: 0.5,
                    borderRadius: '10px',
                    border: 1,
                    borderColor: 'divider',
                    fontSize: 13,
                    color: 'text.secondary',
                    '&:hover': s.url ? { color: 'text.primary', bgcolor: 'action.hover' } : {},
                  }}
                >
                  <AgentMark id={s.id} name={s.name} size={24} />
                  <span>{s.name}</span>
                </Stack>
              ))}
            </Stack>
          </Box>
        )}
      </Section>
      <RemoteAgents />
    </>
  );
}
