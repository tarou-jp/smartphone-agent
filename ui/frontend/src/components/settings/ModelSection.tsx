import { useEffect, useState } from 'react';
import { Alert, Button, CircularProgress, Stack, Typography } from '@mui/material';
import { RowCard, Section } from '@/components/common';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { ModelView } from '@/api/types';
import { SettingSelectRow, errorText } from './controls';
import { KeyProfiles } from './model/KeyProfiles';
import { OllamaSettings } from './model/OllamaSettings';

export function ModelSection() {
  const t = useT();
  const { patchSettings } = useApp();
  const [m, setM] = useState<ModelView | null>(null);
  const [err, setErr] = useState('');
  const [slow, setSlow] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let live = true;
    setErr('');
    setSlow(false);
    // Never a blank page: a spinner, and after a while a way to try again.
    const id = window.setTimeout(() => live && setSlow(true), 6000);
    api.model().then(
      (v) => live && setM(v),
      (e) => live && setErr(errorText(e)),
    ).finally(() => window.clearTimeout(id));
    return () => {
      live = false;
      window.clearTimeout(id);
    };
  }, [attempt]);
  if (!m) {
    if (err || slow) {
      return (
        <Alert
          severity={err ? 'error' : 'warning'}
          action={
            <Button color="inherit" onClick={() => setAttempt((a) => a + 1)}>
              {t('webRetry')}
            </Button>
          }
        >
          {err || t('webModelSlow')}
        </Alert>
      );
    }
    return (
      <Stack alignItems="center" sx={{ py: 6 }}>
        <CircularProgress size={22} />
      </Stack>
    );
  }
  const label = (id: string) => {
    const x = m.models.find((y) => y.id === id);
    return x ? x.label : id;
  };
  const cloud = m.models.filter((x) => x.provider !== 'ollama');
  const choose = async (key: 'chatModel' | 'chatAgentModel', id: string) => {
    await patchSettings({ [key]: id });
    setM(await api.model());
  };
  return (
    <>
      <Section title={t('settingsSectionModel')} description={t('settingsModelIntro')}>
        <Typography variant="caption" sx={{ display: 'block', mb: 1 }}>
          {t('settingsModelExperiment')}
        </Typography>
        <KeyProfiles m={m} onModel={setM} />
      </Section>
      <Section title={t('webChatModel')}>
        <RowCard>
          <SettingSelectRow
            label={t('webChatModel')}
            description={t('webChatModelDesc')}
            value={m.chatModel}
            minWidth={220}
            options={m.models.map((x) => ({
              value: x.id,
              label: x.ready ? x.label : `${x.label} · ${t('chatModelNone')}`,
              disabled: !x.ready,
            }))}
            onChange={(v) => void choose('chatModel', v)}
          />
        </RowCard>
      </Section>
      <Section title={t('settingsChatAgentTitle')} description={t('settingsChatAgentIntro')}>
        <RowCard>
          <SettingSelectRow
            label={t('settingsChatAgentTitle')}
            value={m.chatAgentModel || ''}
            minWidth={220}
            options={[
              ...(m.chatAgentModel ? [] : [{ value: '', label: cloud.length ? t('webChatAgentNone') : t('chatModelNone'), disabled: true }]),
              ...cloud.map((x) => ({
                value: x.id,
                label: x.ready ? label(x.id) : `${x.label} · ${t('chatModelNone')}`,
                disabled: !x.ready,
              })),
            ]}
            onChange={(v) => v && void choose('chatAgentModel', v)}
          />
        </RowCard>
      </Section>
      <Section title={t('settingsOllamaTitle')} description={t('settingsOllamaIntro')}>
        <OllamaSettings m={m} onModel={setM} />
      </Section>
    </>
  );
}
