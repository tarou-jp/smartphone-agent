import { useState } from 'react';
import { Alert, Box, Button, Chip, Stack, TextField } from '@mui/material';
import { Row, RowCard } from '@/components/common';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { ModelView } from '@/api/types';
import { CommitTextField, errorText } from '../controls';

/** Local models through Ollama: the server and the model names. */
export function OllamaSettings({ m, onModel }: { m: ModelView; onModel: (v: ModelView) => void }) {
  const t = useT();
  const { patchSettings } = useApp();
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');
  const setModels = async (list: string[]) => {
    await patchSettings({ ollamaModels: list });
    onModel(await api.model());
  };
  const add = () => {
    const n = draft.trim();
    if (!n || m.ollamaModels.includes(n)) return;
    setDraft('');
    void setModels([...m.ollamaModels, n]);
  };
  return (
    <RowCard>
      <Row label={t('settingsOllamaURL')}>
        <CommitTextField
          value={m.ollamaBaseURL}
          placeholder={t('settingsOllamaURLPlaceholder')}
          onCommit={async (v) => {
            await patchSettings({ ollamaBaseURL: v.trim() });
            onModel(await api.model());
          }}
          sx={{ mt: 1, width: 320 }}
        />
      </Row>
      <Row
        label={t('settingsOllamaModels')}
        control={
          <Button
            variant="outlined"
            color="inherit"
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              setErr('');
              try {
                onModel(await api.refreshOllama());
              } catch (e) {
                setErr(t('settingsOllamaRefreshFail', { detail: errorText(e) }));
              } finally {
                setBusy(false);
              }
            }}
          >
            {t('settingsOllamaRefresh')}
          </Button>
        }
      >
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.75, mt: 1 }}>
          {m.ollamaModels.map((n) => (
            <Chip
              key={n}
              label={n}
              variant="outlined"
              onDelete={() => void setModels(m.ollamaModels.filter((x) => x !== n))}
              title={t('settingsOllamaModelRemove')}
            />
          ))}
        </Box>
        <Stack direction="row" spacing={1} sx={{ mt: 1 }}>
          <TextField
            placeholder={t('settingsOllamaModelPlaceholder')}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && add()}
            sx={{ width: 240 }}
          />
          <Button variant="outlined" color="inherit" disabled={!draft.trim()} onClick={add}>
            {t('settingsOllamaModelAdd')}
          </Button>
        </Stack>
        {err && (
          <Alert severity="error" sx={{ mt: 1 }}>
            {err}
          </Alert>
        )}
      </Row>
    </RowCard>
  );
}
