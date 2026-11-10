import { Button, Typography } from '@mui/material';
import { Row, RowCard, Section } from '@/components/common';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { locales, useI18n, type Locale } from '@/i18n';
import { SettingSelectRow, SettingSwitchRow, errorText } from './controls';
import { Changelog } from './advanced/Changelog';

export function AdvancedSection() {
  const { t, chosen, setLocale } = useI18n();
  const { settings, patchSettings, boot, toast } = useApp();
  if (!settings) return null;
  const rules = boot?.rulesFile;
  return (
    <>
      <Section title={t('settingsSectionAdvanced')} description={t('settingsAdvancedIntro')}>
        <RowCard>
          <SettingSelectRow<Locale | 'system'>
            label={t('settingsLanguage')}
            description={t('settingsLanguageDesc')}
            value={chosen ?? 'system'}
            options={[
              { value: 'system', label: t('settingsLanguageSystem') },
              ...locales.map((l) => ({ value: l.id, label: l.name })),
            ]}
            onChange={(v) => setLocale(v === 'system' ? null : v)}
          />
          <SettingSwitchRow
            label={t('settingsAdvancedDevMode')}
            description={t('settingsAdvancedDevModeDesc')}
            checked={settings.developerMode}
            onChange={(v) => patchSettings({ developerMode: v })}
          />
          <Row
            label={t('settingsAdvancedRulesFile')}
            description={rules ? t('settingsAdvancedRulesFileDesc') : t('settingsAdvancedNoRulesFile')}
            control={
              rules && (
                <Button
                  variant="outlined"
                  color="inherit"
                  onClick={() =>
                    api.open(rules).catch((e) => toast(t('settingsAdvancedRulesOpenFailed', { reason: errorText(e) }), 'error'))
                  }
                >
                  {t('settingsAdvancedEditRules')}
                </Button>
              )
            }
          >
            {rules && (
              <Typography variant="caption" className="selectable" sx={{ fontFamily: 'ui-monospace, Menlo, monospace' }}>
                {rules}
              </Typography>
            )}
          </Row>
        </RowCard>
      </Section>
      <Section title={t('updateSettingsTitle')}>
        <RowCard>
          <SettingSwitchRow
            label={t('updateSettingsAuto')}
            description={t('updateSettingsAutoDesc')}
            checked={settings.autoUpdate}
            onChange={(v) => patchSettings({ autoUpdate: v })}
          />
          <Row label={t('webVersion')} control={<Typography className="selectable">{boot?.version}</Typography>} />
        </RowCard>
      </Section>
      {boot?.changelog && (
        <Section title={t('updateWhatsNew')} description={t('updateSettingsNewsDesc')}>
          <RowCard sx={{ px: 2, pt: 2 }}>
            <Changelog text={boot.changelog} />
          </RowCard>
        </Section>
      )}
    </>
  );
}
