import { useState } from 'react';
import { Alert, Box, Button, CircularProgress, Typography } from '@mui/material';
import { api } from '@/api/client';
import { useI18n } from '@/i18n';
import type { Checkup, Device } from '@/api/types';
import { CheckRow } from '@/components/common/CheckList';
import { Row, StatusText } from '@/components/common';
import { errorText } from '../controls';

/** A phone's checkup: what works, and what to fix. */
export function CheckupPanel({ d }: { d: Device }) {
  const { t } = useI18n();
  const [res, setRes] = useState<Checkup | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');
  const run = async () => {
    setBusy(true);
    setErr('');
    try {
      setRes(await api.checkup(d.id));
    } catch (e) {
      setErr(t('checkupFailed', { error: errorText(e) }));
    } finally {
      setBusy(false);
    }
  };
  const toFix = res?.checks.filter((c) => c.status === 'fail').length ?? 0;
  // What passed needs no line of its own: the result is one line, and only
  // what failed or wants a look stays listed, until it is fixed.
  const problems = res?.checks.filter((c) => c.status === 'fail' || c.status === 'warn') ?? [];
  // One row: what the checkup is on the left, the button that runs it on
  // the right, and what it found below.
  return (
    <Row
      label={t('phoneCheckup')}
      description={
        res && problems.length === 0 ? (
          <StatusText tone="good">{t('checkupAllGood').replace(/[。.]$/, '')}</StatusText>
        ) : (
          t('checkupIntro')
        )
      }
      control={
        <Button
          variant="outlined"
          color="inherit"
          onClick={run}
          disabled={busy}
          aria-label={t('settingsPhonesCheckupLabel', { name: d.name })}
          startIcon={busy ? <CircularProgress size={14} /> : undefined}
        >
          {busy ? t('checkupRunning') : res ? t('checkupAgain') : t('webCheckupRun')}
        </Button>
      }
    >
      {err && (
        <Alert severity="error" sx={{ mt: 1 }}>
          {err}
        </Alert>
      )}
      {res && problems.length > 0 && (
        <Box sx={{ mt: 1.5 }}>
          <Typography variant="subtitle1" sx={{ mb: 0.5 }}>
            {toFix > 0 ? t('checkupToFix', { count: toFix }) : t('checkupAllGood')}
          </Typography>
          {res.system && (
            <Typography variant="body2" color="text.secondary">
              {t('checkupSystem', { system: [res.family, res.system].filter(Boolean).join(' ') })}
            </Typography>
          )}
          {problems.map((c) => (
            <CheckRow key={c.id} c={c} />
          ))}
          {d.status === 'offline' && (
            <Typography variant="body2" color="text.secondary">
              {t('checkupWaitOnline')}
            </Typography>
          )}
        </Box>
      )}
    </Row>
  );
}
