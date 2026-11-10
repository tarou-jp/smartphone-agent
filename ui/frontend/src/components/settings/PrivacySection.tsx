import { Button, Typography } from '@mui/material';
import { Row, RowCard, Section, useConfirm } from '@/components/common';
import { api } from '@/api/client';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import { SettingSelectRow, SettingSwitchRow, errorText } from './controls';

/** The retention choices, with the current one when it is not among them. */
function keepChoices(cur: number): number[] {
  const out = [7, 30, 90, 365, 0];
  return out.includes(cur) ? out : [cur, ...out];
}

export function PrivacySection() {
  const t = useT();
  const { settings, patchSettings, boot, toast, refresh } = useApp();
  const canWipe = useHas('wipeHistory');
  const [confirm, dialog] = useConfirm();
  if (!settings) return null;
  const keepLabel = (d: number) =>
    d === 0 ? t('settingsPrivacyKeepForever') : d === 365 ? t('settingsPrivacyKeepYear') : t('settingsPrivacyKeepDays', { days: d });
  const wipe = async () => {
    const ok = await confirm({
      title: t('settingsPrivacyDeleteTitle'),
      body: t('settingsPrivacyDeleteMessage'),
      confirm: t('settingsPrivacyDeleteConfirm'),
      cancel: t('settingsCancel'),
      danger: true,
    });
    if (!ok) return;
    try {
      await api.wipeHistory();
      toast(t('settingsPrivacyDeleted'), 'success');
      refresh();
    } catch (e) {
      toast(t('settingsPrivacyDeleteFailed', { reason: errorText(e) }), 'error');
    }
  };
  return (
    <>
      <Section title={t('settingsSectionPrivacy')} description={t('settingsPrivacyIntro')}>
        <RowCard>
          <SettingSelectRow
            label={t('settingsPrivacyKeep')}
            description={t('settingsPrivacyKeepDesc')}
            value={settings.keepDays}
            options={keepChoices(settings.keepDays).map((d) => ({ value: d, label: keepLabel(d) }))}
            onChange={(v) => patchSettings({ keepDays: v })}
          />
          <SettingSwitchRow
            label={t('settingsPrivacyRecord')}
            description={t('settingsPrivacyRecordDesc')}
            checked={settings.record}
            onChange={(v) => patchSettings({ record: v })}
          />
          <Row
            label={t('settingsPrivacyDeleteAll')}
            description={canWipe ? t('settingsPrivacyDeleteAllDesc') : t('settingsPrivacyCannotDelete')}
            control={
              <Button color="error" variant="outlined" disabled={!canWipe} onClick={wipe}>
                {t('settingsPrivacyDeleteButton')}
              </Button>
            }
          />
        </RowCard>
      </Section>
      {boot?.telemetry && (
        <Section title={t('telPrivacyTitle')}>
          <RowCard>
            <SettingSwitchRow
              label={t('telShareUsage')}
              description={t('telShareUsageDesc')}
              checked={settings.usageStats}
              onChange={(v) => patchSettings({ usageStats: v })}
            />
          </RowCard>
          <Typography variant="caption" sx={{ display: 'block', mt: 1 }}>
            {t('telPrivacyGroup')}
          </Typography>
        </Section>
      )}
      {dialog}
    </>
  );
}
