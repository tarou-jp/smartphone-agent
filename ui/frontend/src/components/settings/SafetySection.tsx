import { useState } from 'react';
import { Box, FormControlLabel, Radio, RadioGroup, Typography } from '@mui/material';
import { Row, RowCard, Section } from '@/components/common';
import { api } from '@/api/client';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import type { FileProblem, Settings } from '@/api/types';
import { SettingSwitchRow, usePoll } from './controls';
import { BlockedApps } from './safety/BlockedApps';
import { Exemptions } from './safety/Exemptions';
import { FileProblems } from './safety/FileProblems';
import { RulesEditor } from './safety/RulesEditor';

const presets = [
  { v: 2, label: 'settingsSafetyPresetPayments', desc: 'settingsSafetyPresetPaymentsDesc' },
  { v: 1, label: 'settingsSafetyPresetCommits', desc: 'settingsSafetyPresetCommitsDesc' },
  { v: 3, label: 'settingsSafetyPresetEvery', desc: 'settingsSafetyPresetEveryDesc' },
];

export function SafetySection() {
  const t = useT();
  const { settings, patchSettings, boot } = useApp();
  const hasExemptions = useHas('exemptions');
  const hasRules = useHas('rules');
  const hasProblems = useHas('fileProblems');
  // What the rules files say, read again every 2 s, as hand edits of
  // rules.yaml and apps.yaml change them while Pinky runs.
  const [polled, setPolled] = useState<Settings | null>(null);
  const [problems, setProblems] = useState<FileProblem[]>([]);
  usePoll(() => {
    api.settings().then(setPolled, () => undefined);
    if (hasProblems) api.fileProblems().then(setProblems, () => undefined);
  }, 2000);
  const s = polled ?? settings;
  if (!s) return null;
  const patch = (p: Partial<Settings>) => {
    setPolled((cur) => (cur ? { ...cur, ...p } : cur));
    void patchSettings(p);
  };
  const unlockDesc =
    boot?.platform === 'darwin'
      ? 'settingsSafetyUnlockDescMac'
      : boot?.platform === 'windows'
        ? 'settingsSafetyUnlockDescWindows'
        : 'settingsSafetyUnlockDescOther';
  return (
    <>
      <Section title={t('settingsSectionSafety')} description={t('settingsSafetyIntro')}>
        <FileProblems problems={problems} />
        <Typography variant="subtitle2" sx={{ mb: 0.75 }}>
          {t('settingsSafetyAskTitle')}
        </Typography>
        <RowCard sx={{ mb: 3 }}>
          <Box sx={{ px: 2, py: 1 }}>
            <RadioGroup
              value={String(s.preset)}
              onChange={(e) => patch({ preset: Number(e.target.value) })}
              aria-label={t('settingsSafetyAskGroup')}
            >
              {presets.map((p) => (
                <FormControlLabel
                  key={p.v}
                  value={String(p.v)}
                  control={<Radio size="small" />}
                  sx={{ alignItems: 'flex-start', my: 0.5, '& .MuiRadio-root': { pt: 0.25 } }}
                  label={
                    <Box>
                      <Typography variant="subtitle1">{t(p.label)}</Typography>
                      <Typography variant="body2" color="text.secondary">
                        {t(p.desc)}
                      </Typography>
                    </Box>
                  }
                />
              ))}
            </RadioGroup>
          </Box>
        </RowCard>
        <Typography variant="subtitle2" sx={{ mb: 0.75 }}>
          {t('settingsSafetyProtections')}
        </Typography>
        <RowCard>
          <SettingSwitchRow
            label={t('settingsSafetyReadOnly')}
            description={t('settingsSafetyReadOnlyDesc')}
            checked={s.readOnly}
            onChange={(v) => patch({ readOnly: v })}
          />
          <SettingSwitchRow
            label={t('settingsSafetyHideCodes')}
            description={t('settingsSafetyHideCodesDesc')}
            checked={s.hideCodes}
            onChange={(v) => patch({ hideCodes: v })}
          />
          {boot?.keychain ? (
            <SettingSwitchRow
              label={t('settingsUnlockFromComputer')}
              description={t(unlockDesc)}
              checked={s.unlockFromComputer}
              onChange={(v) => patch({ unlockFromComputer: v })}
            />
          ) : (
            <Row label={t('settingsUnlockFromComputer')} description={t('unlockNotAvailable')} />
          )}
          <BlockedApps apps={s.blockedApps} onChange={(blockedApps) => patch({ blockedApps })} />
        </RowCard>
      </Section>
      {hasExemptions && <Exemptions every={s.preset === 3} />}
      {hasRules && <RulesEditor />}
    </>
  );
}
