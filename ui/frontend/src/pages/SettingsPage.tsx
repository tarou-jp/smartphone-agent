import { Box, List, ListItemButton, ListItemText } from '@mui/material';
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
  { id: 'phones', label: 'settingsSectionPhones', view: PhonesSection },
  { id: 'agents', label: 'settingsSectionAgents', view: AgentsSection },
  { id: 'safety', label: 'settingsSectionSafety', view: SafetySection },
  { id: 'privacy', label: 'settingsSectionPrivacy', view: PrivacySection },
  { id: 'model', label: 'settingsSectionModel', view: ModelSection },
  { id: 'advanced', label: 'settingsSectionAdvanced', view: AdvancedSection },
] as const;

export function SettingsPage({ section }: { section?: string }) {
  const t = useT();
  const cur = sections.find((s) => s.id === section) ?? sections[0];
  const View = cur.view;
  return (
    <>
      <PageHeader title={t('settingsPageTitle')} />
      <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <Box
          component="nav"
          aria-label={t('settingsSectionsLabel')}
          sx={{ width: 200, flexShrink: 0, borderRight: 1, borderColor: 'divider', p: 1, overflowY: 'auto' }}
        >
          <List dense>
            {sections.map((s) => (
              <ListItemButton key={s.id} component="a" href={href({ page: 'settings', section: s.id })} selected={s.id === cur.id}>
                <ListItemText primary={t(s.label)} primaryTypographyProps={{ fontSize: 13, fontWeight: 500 }} />
              </ListItemButton>
            ))}
          </List>
        </Box>
        <Box sx={{ flex: 1, minWidth: 0, overflowY: 'auto', scrollbarGutter: 'stable' }}>
          <Box sx={{ maxWidth: 760, mx: 'auto', px: 4, py: 3 }}>
            <View key={cur.id} />
          </Box>
        </Box>
      </Box>
    </>
  );
}
