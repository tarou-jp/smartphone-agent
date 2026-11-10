import { useState } from 'react';
import { Box, Button, Chip, FormControlLabel, Radio, RadioGroup, Stack, TextField, Typography } from '@mui/material';
import { api } from '@/api/client';
import { useApp, useHas } from '@/state/AppState';
import { useT, type T } from '@/i18n';
import type { AgentView, ScopeView } from '@/api/types';
import { joinAnd, splitList } from '../labels';

export function scopeLine(t: T, s: ScopeView): string {
  switch (s.mode) {
    case 'read_only':
      return t('scopeLineReadOnly');
    case 'watch_only':
      return t('scopeLineWatchOnly');
    case 'apps':
      return s.apps.length ? t('scopeLineApps', { apps: joinAnd(t, s.apps) }) : t('scopeLineNoApp');
  }
  return t('scopeLineEverything');
}

const modes: { mode: ScopeView['mode']; label: string; hint: string }[] = [
  { mode: 'everything', label: 'scopeEverything', hint: 'scopeHintEverything' },
  { mode: 'read_only', label: 'scopeReadOnly', hint: 'scopeHintReadOnly' },
  { mode: 'watch_only', label: 'scopeWatchOnly', hint: 'scopeHintWatchOnly' },
  { mode: 'apps', label: 'scopeApps', hint: 'scopeHintApps' },
];

/** One agent: what it may do, and a way to change it. */
export function AgentRow({ a, onChanged }: { a: AgentView; onChanged: () => void }) {
  const t = useT();
  const { run } = useApp();
  const canScope = useHas('agentScope');
  const [editing, setEditing] = useState(false);
  const [mode, setMode] = useState(a.scope.mode);
  const [apps, setApps] = useState(a.scope.apps.join(', '));
  const appList = splitList(apps);
  const invalid = mode === 'apps' && appList.length === 0;
  const save = async () => {
    await run(t('settingsAgentsChangeLabel', { agent: a.name }).toLowerCase(), () =>
      api.setAgentScope(a.name, { mode, apps: mode === 'apps' ? appList : [] }),
    );
    setEditing(false);
    onChanged();
  };
  return (
    <Box sx={{ px: 2, py: 1.5 }}>
      <Stack direction="row" alignItems="center" spacing={1.5}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="subtitle1">{a.name}</Typography>
          <Typography variant="body2" color="text.secondary">
            {a.approved ? scopeLine(t, a.scope) : t('settingsAgentsNotAllowed')}
          </Typography>
        </Box>
        <Chip
          label={a.approved ? t('settingsAgentsAllowed') : t('settingsAgentsWaiting')}
          color={a.approved ? 'success' : 'warning'}
          variant="outlined"
        />
        {canScope && a.approved && !editing && (
          <Button
            variant="outlined"
            color="inherit"
            aria-label={t('settingsAgentsChangeLabel', { agent: a.name })}
            onClick={() => setEditing(true)}
          >
            {t('settingsChange')}
          </Button>
        )}
      </Stack>
      {editing && (
        <Box sx={{ mt: 1.5 }}>
          <Typography variant="subtitle2" sx={{ mb: 0.5 }}>
            {t('settingsAgentsScopeLabel', { agent: a.name })}
          </Typography>
          <RadioGroup value={mode} onChange={(e) => setMode(e.target.value)}>
            {modes.map((m) => (
              <FormControlLabel
                key={m.mode}
                value={m.mode}
                control={<Radio size="small" />}
                sx={{ alignItems: 'flex-start', mb: 0.5, '& .MuiRadio-root': { pt: 0.25 } }}
                label={
                  <Box>
                    <Typography variant="subtitle1">{t(m.label)}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      {t(m.hint)}
                    </Typography>
                  </Box>
                }
              />
            ))}
          </RadioGroup>
          {mode === 'apps' && (
            <TextField
              fullWidth
              label={t('settingsAgentsApps')}
              placeholder={t('settingsAgentsAppsPlaceholder')}
              value={apps}
              error={invalid}
              helperText={invalid ? t('needsApproveNoApps') : undefined}
              onChange={(e) => setApps(e.target.value)}
              sx={{ mt: 1 }}
            />
          )}
          <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
            <Button variant="contained" disabled={invalid} onClick={save}>
              {t('settingsSave')}
            </Button>
            <Button
              color="inherit"
              onClick={() => {
                setEditing(false);
                setMode(a.scope.mode);
                setApps(a.scope.apps.join(', '));
              }}
            >
              {t('settingsCancel')}
            </Button>
          </Stack>
        </Box>
      )}
    </Box>
  );
}
