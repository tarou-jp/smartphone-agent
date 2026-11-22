import { useState } from 'react';
import { Button, Stack, TextField } from '@mui/material';
import { useT } from '@/i18n';

/**
 * Keeping a suggested flow, under a name the person may change, or turning
 * it down: on the Flows page and on the card of the task it came from.
 */
export function KeepActions({
  name,
  onKeep,
  onDismiss,
  dense,
}: {
  name: string;
  onKeep: (name: string) => void;
  onDismiss: () => void;
  dense?: boolean;
}) {
  const t = useT();
  const [text, setText] = useState(name);
  const keep = () => {
    if (text.trim()) onKeep(text.trim());
  };
  return (
    <Stack direction="row" alignItems="center" spacing={1} sx={{ px: dense ? 0 : 2, pb: dense ? 0 : 1.5, pt: 0.5, flexWrap: 'wrap', rowGap: 1 }}>
      <TextField
        size="small"
        label={t('flowsKeepName')}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') keep();
        }}
        sx={{ flex: 1, minWidth: 180 }}
      />
      <Button variant="contained" disabled={!text.trim()} onClick={keep}>
        {t('flowsKeep')}
      </Button>
      <Button color="inherit" onClick={onDismiss}>
        {t('flowsDismiss')}
      </Button>
    </Stack>
  );
}
