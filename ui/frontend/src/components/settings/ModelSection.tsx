import { useEffect, useState } from 'react';
import { Alert, Typography } from '@mui/material';
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
  useEffect(() => {
    api.model().then(setM, (e) => setErr(errorText(e)));
  }, []);
  if (!m) return err ? <Alert severity="error">{err}</Alert> : null;
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
              ...(m.chatAgentModel ? [] : [{ value: '', label: t('chatModelNone'), disabled: true }]),
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
