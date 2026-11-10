import { useEffect, useState } from 'react';
import { Box, Button, Stack, Typography } from '@mui/material';
import { RowCard, Section } from '@/components/common';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { Exemption } from '@/api/types';
import { ruleAction } from '../labels';

/** The person's Always allow answers, each with Ask again. */
export function Exemptions({ every }: { every: boolean }) {
  const t = useT();
  const { run, live } = useApp();
  const [list, setList] = useState<Exemption[]>([]);
  const load = () => api.exemptions().then(setList, () => setList([]));
  useEffect(() => {
    void load();
  }, [live.needs.length]);
  return (
    <Section title={t('settingsSafetyAlways')} description={t('settingsSafetyAlwaysIntro')}>
      {every && list.length > 0 && (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
          {t('settingsSafetyAlwaysEvery')}
        </Typography>
      )}
      {list.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          {t('settingsSafetyAlwaysNone')}
        </Typography>
      ) : (
        <RowCard>
          {list.map((x) => {
            const app = x.appName || x.appId;
            const kind = ruleAction(t, x.rule);
            return (
              <Stack key={`${x.rule}|${x.appId}|${x.appName}`} direction="row" alignItems="center" sx={{ px: 2, py: 1.25 }}>
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography variant="subtitle1">{t('needsAlwaysAllowIn', { kind, app })}</Typography>
                </Box>
                <Button
                  color="inherit"
                  variant="outlined"
                  aria-label={t('settingsSafetyAlwaysRemoveItem', { kind, app })}
                  onClick={async () => {
                    await run(t('callTakeBackAlways'), () => api.removeExemption(x));
                    void load();
                  }}
                >
                  {t('settingsSafetyAlwaysRemove')}
                </Button>
              </Stack>
            );
          })}
        </RowCard>
      )}
    </Section>
  );
}
