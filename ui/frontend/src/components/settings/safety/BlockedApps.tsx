import { useState } from 'react';
import { Box, Button, Chip, Stack, TextField } from '@mui/material';
import { Row } from '@/components/common';
import { useT } from '@/i18n';
import type { BlockedApp } from '@/api/types';
import { kindLabel } from '../labels';

/** Apps agents may not open: chips, and a field to add one. */
export function BlockedApps({ apps, onChange }: { apps: BlockedApp[]; onChange: (apps: BlockedApp[]) => void }) {
  const t = useT();
  const [draft, setDraft] = useState('');
  const add = () => {
    const name = draft.trim();
    if (!name || apps.some((a) => a.name === name)) return;
    onChange([...apps, { name }]);
    setDraft('');
  };
  return (
    <Row label={t('settingsSafetyBlockedApps')} description={t('settingsSafetyBlockedIntro')}>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.75, mt: 1.25 }}>
        {apps.map((a) => (
          <Chip
            key={a.name}
            label={a.kind ? `${a.name} · ${kindLabel(t, a.kind)}` : a.name}
            onDelete={() => onChange(apps.filter((x) => x.name !== a.name))}
            title={t('settingsSafetyUnblockApp', { app: a.name })}
            variant="outlined"
          />
        ))}
      </Box>
      <Stack direction="row" spacing={1} sx={{ mt: 1.25 }}>
        <TextField
          label={t('settingsSafetyAppLabel')}
          placeholder={t('settingsSafetyAppPlaceholder')}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && add()}
          sx={{ width: 260 }}
        />
        <Button variant="outlined" color="inherit" disabled={!draft.trim()} onClick={add}>
          {t('settingsSafetyBlock')}
        </Button>
      </Stack>
    </Row>
  );
}
