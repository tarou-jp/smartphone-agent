import { Alert, AlertTitle, Button, Typography } from '@mui/material';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { FileProblem } from '@/api/types';
import { errorText } from '../controls';

/** Rules files edited by hand that Pinky cannot read. */
export function FileProblems({ problems }: { problems: FileProblem[] }) {
  const t = useT();
  const { toast } = useApp();
  return (
    <>
      {problems.map((p) => (
        <Alert
          key={p.path}
          severity="error"
          sx={{ mb: 2 }}
          action={
            <Button
              color="inherit"
              onClick={() =>
                api.open(p.path).catch((e) => toast(t('settingsFileProblemOpenFailed', { file: p.file, reason: errorText(e) }), 'error'))
              }
            >
              {t('settingsFileProblemEdit', { file: p.file })}
            </Button>
          }
        >
          <AlertTitle>{t('settingsFileProblemTitle', { file: p.file })}</AlertTitle>
          {t('settingsFileProblemDesc')}
          <Typography variant="body2" className="selectable" sx={{ mt: 0.5, fontFamily: 'ui-monospace, Menlo, monospace' }}>
            {t('settingsFileProblemError', { error: p.error })}
          </Typography>
        </Alert>
      ))}
    </>
  );
}
