import { useEffect, useState } from 'react';
import { Alert, Typography } from '@mui/material';
import { RowCard, Section } from '@/components/common';
import { api } from '@/api/client';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import type { AgentView } from '@/api/types';
import { AgentRow } from './agents/AgentScopeEditor';
import { AgentSetupList } from './agents/AgentSetupList';
import { RemoteAgents } from './agents/RemoteAgents';
import { errorText } from './controls';

/** The agents seen in tasks, for gateways that do not list them. */
function seenAgents(names: string[]): AgentView[] {
  return [...new Set(names.filter(Boolean))].map((name) => ({ name, approved: true, scope: { mode: 'everything', apps: [] } }));
}

export function AgentsSection() {
  const t = useT();
  const { live } = useApp();
  const canList = useHas('agents');
  const canSetup = useHas('agentSetup');
  const [agents, setAgents] = useState<AgentView[] | null>(null);
  const [err, setErr] = useState('');
  const load = () => {
    if (!canList) return;
    api.agents().then(
      (l) => {
        setAgents(l);
        setErr('');
      },
      (e) => setErr(errorText(e)),
    );
  };
  useEffect(load, [canList]);
  // A new agent's request shows up as a Needs you item; read the list again.
  useEffect(load, [live.needs.length, canList]);
  const list = canList ? agents : seenAgents(live.tasks.map((x) => x.agent));
  return (
    <>
      <Section title={t('settingsSectionAgents')} description={t('settingsAgentsIntro')}>
        <Typography variant="subtitle2" sx={{ mb: 1 }}>
          {t('settingsAgentsList')}
        </Typography>
        {!canList && (
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            {t('settingsAgentsUnlisted')}
          </Typography>
        )}
        {err && <Alert severity="error">{err}</Alert>}
        {list && list.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {canSetup ? t('settingsAgentsNone') : t('settingsAgentsNoneYet')}
          </Typography>
        )}
        {list && list.length > 0 && (
          <RowCard>
            {list.map((a) => (
              <AgentRow key={a.name} a={a} onChanged={load} />
            ))}
          </RowCard>
        )}
      </Section>
      {canSetup && <AgentSetupList />}
      <RemoteAgents />
    </>
  );
}
