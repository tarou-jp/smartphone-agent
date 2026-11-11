import { Box, ListItemButton, Typography, useTheme } from '@mui/material';
import PhonesIcon from '@mui/icons-material/SmartphoneOutlined';
import AgentsIcon from '@mui/icons-material/HubOutlined';
import SafetyIcon from '@mui/icons-material/ShieldOutlined';
import PrivacyIcon from '@mui/icons-material/VisibilityOffOutlined';
import ModelIcon from '@mui/icons-material/MemoryOutlined';
import AdvancedIcon from '@mui/icons-material/TuneOutlined';
import { PageHeader } from '@/components/common';
import { useT } from '@/i18n';
import { href } from '@/lib/router';
import { PhonesSection } from '@/components/settings/PhonesSection';
import { AgentsSection } from '@/components/settings/AgentsSection';
import { SafetySection } from '@/components/settings/SafetySection';
import { PrivacySection } from '@/components/settings/PrivacySection';
import { AdvancedSection } from '@/components/settings/AdvancedSection';
import { ModelSection } from '@/components/settings/ModelSection';

const sections = [
  { id: 'phones', label: 'settingsSectionPhones', view: PhonesSection, Icon: PhonesIcon },
  { id: 'agents', label: 'settingsSectionAgents', view: AgentsSection, Icon: AgentsIcon },
  { id: 'safety', label: 'settingsSectionSafety', view: SafetySection, Icon: SafetyIcon },
  { id: 'privacy', label: 'settingsSectionPrivacy', view: PrivacySection, Icon: PrivacyIcon },
  { id: 'model', label: 'settingsSectionModel', view: ModelSection, Icon: ModelIcon },
  { id: 'advanced', label: 'settingsSectionAdvanced', view: AdvancedSection, Icon: AdvancedIcon },
] as const;

export function SettingsPage({ section }: { section?: string }) {
  const t = useT();
  const p = useTheme().pinky;
  const cur = sections.find((s) => s.id === section) ?? sections[0];
  const View = cur.view;
  return (
    <>
      <PageHeader title={t('settingsPageTitle')} noAgent />
      <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <Box
          component="nav"
          aria-label={t('settingsSectionsLabel')}
          sx={{ width: 208, flexShrink: 0, borderRight: 1, borderColor: 'divider', px: 1.5, pt: 2, pb: 2, overflowY: 'auto' }}
        >
          {sections.map((s) => {
            const on = s.id === cur.id;
            return (
              <ListItemButton
                key={s.id}
                component="a"
                href={href({ page: 'settings', section: s.id })}
                selected={on}
                sx={{ minHeight: 36, py: 0, px: 1.25, gap: 1.5, mb: 0.25 }}
              >
                <s.Icon sx={{ fontSize: 18, color: on ? 'text.primary' : p.ink3 }} />
                <Typography sx={{ fontSize: 14, fontWeight: on ? 500 : 400, color: on ? 'text.primary' : p.ink2 }}>{t(s.label)}</Typography>
              </ListItemButton>
            );
          })}
        </Box>
        <Box sx={{ flex: 1, minWidth: 0, overflowY: 'auto', scrollbarGutter: 'stable' }}>
          <Box sx={{ maxWidth: 720, mx: 'auto', px: 5, pt: 3, pb: 6 }}>
            <View key={cur.id} />
          </Box>
        </Box>
      </Box>
    </>
  );
}
