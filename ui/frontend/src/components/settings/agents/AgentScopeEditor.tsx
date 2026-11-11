import { useState } from 'react';
import { Box, Button, FormControlLabel, Radio, RadioGroup, Stack, TextField, Typography } from '@mui/material';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
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

/** The form that changes what an agent may do; it shows under the agent's row. */
export function ScopeForm({ a, onDone }: { a: AgentView; onDone: (changed: boolean) => void }) {
  const t = useT();
  const { run } = useApp();
  const [mode, setMode] = useState(a.scope.mode);
  const [apps, setApps] = useState(a.scope.apps.join(', '));
  const appList = splitList(apps);
  const invalid = mode === 'apps' && appList.length === 0;
  const save = async () => {
    await run(t('settingsAgentsChangeLabel', { agent: a.name }).toLowerCase(), () =>
      api.setAgentScope(a.name, { mode, apps: mode === 'apps' ? appList : [] }),
    );
    onDone(true);
  };
  return (
    <Box sx={{ mt: 2 }}>
      <Typography variant="subtitle2" sx={{ mb: 1 }}>
        {t('settingsAgentsScopeLabel', { agent: a.name })}
      </Typography>
      <RadioGroup value={mode} onChange={(e) => setMode(e.target.value)}>
        {modes.map((m) => (
          <FormControlLabel
            key={m.mode}
            value={m.mode}
            control={<Radio size="small" />}
            sx={{ alignItems: 'flex-start', mb: 1, '& .MuiRadio-root': { pt: 0.25 } }}
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
      <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
        <Button variant="contained" disabled={invalid} onClick={save}>
          {t('settingsSave')}
        </Button>
        <Button color="inherit" onClick={() => onDone(false)}>
          {t('settingsCancel')}
        </Button>
      </Stack>
    </Box>
  );
}
