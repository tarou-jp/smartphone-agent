import { useEffect, useState } from 'react';
import { Alert, Box, Button, Chip, CircularProgress, Link, Stack, Typography } from '@mui/material';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT, type T } from '@/i18n';
import type { AgentSetupView } from '@/api/types';
import { RowCard, Section } from '@/components/common';
import { errorText } from '../controls';
import { joinAnd } from '../labels';

function doneText(t: T, a: AgentSetupView, r: { restart: string; opened: string }, removed: boolean): string {
  if (removed) return r.restart === 'app' ? t('connectRemovedRestart', { agent: a.name }) : t('connectRemoved');
  if (r.opened) return t('connectDoneOpened', { agent: a.name });
  if (r.restart === 'app') return t('connectDoneRestart', { agent: a.name });
  if (r.restart === 'session') return t('connectDoneSession', { agent: a.name });
  return t('connectDoneNow', { agent: a.name });
}

function SetupRow({ a, onChanged }: { a: AgentSetupView; onChanged: () => void }) {
  const t = useT();
  const { toast } = useApp();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ text: string; error?: boolean; kept?: boolean } | null>(null);
  const act = async (op: 'connect' | 'remove') => {
    setBusy(true);
    setMsg(null);
    try {
      const r = await api.agentSetupDo(a.id, op);
      setMsg({ text: doneText(t, a, r, op === 'remove'), kept: !!r.backups?.length });
      if (op === 'connect') toast(r.opened ? t('connectToastOpened', { agent: a.name }) : t('connectToastConnected', { agent: a.name }), 'success');
      onChanged();
    } catch (e) {
      setMsg({ text: t('connectFailed', { error: errorText(e) }), error: true });
    } finally {
      setBusy(false);
    }
  };
  const state =
    a.state === 'connected'
      ? a.extension
        ? t('connectStateExtension', { agent: a.name })
        : t('connectStateConnected')
      : a.state === 'outdated'
        ? t('connectCardBroken', { agent: a.name })
        : t('connectStateNotConnected');
  return (
    <Box sx={{ px: 2, py: 1.5 }}>
      <Stack direction="row" alignItems="center" spacing={1.5}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="subtitle1">{a.name}</Typography>
          <Typography variant="body2" color="text.secondary">
            {state}
          </Typography>
        </Box>
        {busy && <CircularProgress size={16} aria-label={t('connectWorking', { agent: a.name })} />}
        {a.state === 'connected' && <Chip label={t('connectStateConnected')} color="success" variant="outlined" />}
        {!a.manual && a.state !== 'connected' && (
          <Button
            variant="contained"
            disabled={busy}
            onClick={() => act('connect')}
            aria-label={a.state === 'outdated' ? t('connectFixLabel', { agent: a.name }) : t('connectConnectLabel', { agent: a.name })}
          >
            {a.state === 'outdated' ? t('connectFix') : t('connectConnect')}
          </Button>
        )}
        {!a.manual && !a.extension && a.state !== 'not_connected' && (
          <Button color="inherit" disabled={busy} onClick={() => act('remove')} aria-label={t('connectRemoveLabel', { agent: a.name })}>
            {t('connectRemove')}
          </Button>
        )}
      </Stack>
      {a.problem && (
        <Typography variant="body2" color="error" sx={{ mt: 0.5 }}>
          {t('connectProblem', { problem: a.problem })}
        </Typography>
      )}
      {a.manual && (
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          {t('connectManual')}
        </Typography>
      )}
      {msg && (
        <Alert severity={msg.error ? 'error' : 'success'} sx={{ mt: 1 }}>
          {msg.text} {msg.kept && t('connectKeptCopy')}
        </Alert>
      )}
    </Box>
  );
}

/** Agents on this computer, to add Pinky to. */
export function AgentSetupList() {
  const t = useT();
  const [list, setList] = useState<AgentSetupView[] | null>(null);
  const [err, setErr] = useState('');
  const load = () =>
    api.agentSetup().then(
      (l) => {
        setList(l);
        setErr('');
      },
      (e) => setErr(errorText(e)),
    );
  useEffect(() => {
    void load();
  }, []);
  const found = list?.filter((a) => a.found) ?? [];
  const missing = list?.filter((a) => !a.found) ?? [];
  return (
    <Section title={t('connectTitle')} description={t('connectIntro')}>
      {!list && !err && (
        <Typography variant="body2" color="text.secondary">
          {t('connectLooking')}
        </Typography>
      )}
      {err && <Alert severity="error">{t('connectFailed', { error: err })}</Alert>}
      {list && found.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          {t('connectNoneFound')}
        </Typography>
      )}
      {found.length > 0 && (
        <RowCard>
          {found.map((a) => (
            <SetupRow key={a.id} a={a} onChanged={load} />
          ))}
        </RowCard>
      )}
      {missing.length > 0 && (
        <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
          {t('connectNotHere', { names: joinAnd(t, missing.map((a) => a.name)) })}{' '}
          {missing
            .filter((a) => a.url)
            .map((a) => (
              <Link key={a.id} href={a.url} target="_blank" rel="noreferrer" sx={{ mr: 1 }}>
                {t('connectGet', { agent: a.name })}
              </Link>
            ))}
        </Typography>
      )}
    </Section>
  );
}
