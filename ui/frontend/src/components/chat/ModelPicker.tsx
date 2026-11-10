import { useEffect, useState } from 'react';
import { Button, ListSubheader, Menu, MenuItem, Typography, useTheme } from '@mui/material';
import ExpandIcon from '@mui/icons-material/ExpandMoreRounded';
import CheckIcon from '@mui/icons-material/CheckRounded';
import { api } from '@/api/client';
import type { ModelView } from '@/api/types';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import { href } from '@/lib/router';

const providerName: Record<string, string> = { anthropic: 'Anthropic', openai: 'OpenAI', ollama: 'Ollama' };

/** The composer's model menu: the models with a key, by provider. */
export function ModelPicker() {
  const t = useT();
  const p = useTheme().pinky;
  const { settings, patchSettings } = useApp();
  const [model, setModel] = useState<ModelView | null>(null);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  // The server resolves the model the chat uses; read it again whenever
  // the settings change (keys, Ollama models, the choice).
  useEffect(() => {
    let live = true;
    api.model().then((m) => live && setModel(m), () => undefined);
    return () => {
      live = false;
    };
  }, [settings]);

  const ready = model?.models.filter((m) => m.ready) ?? [];
  const current = model?.models.find((m) => m.id === model.chatModel && m.ready);
  const groups = Array.from(new Set(ready.map((m) => m.provider)));

  return (
    <>
      <Button
        size="small"
        color="inherit"
        endIcon={<ExpandIcon sx={{ fontSize: '16px !important' }} />}
        onClick={(e) => setAnchor(e.currentTarget)}
        sx={{ minHeight: 24, px: 1, py: 0, color: p.ink2, fontWeight: 500, fontSize: 12, maxWidth: 170 }}
      >
        <Typography noWrap sx={{ fontSize: 12 }}>
          {current?.label ?? t('chatModelNone')}
        </Typography>
      </Button>
      <Menu anchorEl={anchor} open={!!anchor} onClose={() => setAnchor(null)} slotProps={{ paper: { sx: { minWidth: 220 } } }}>
        {groups.map((g) => [
          <ListSubheader key={'h' + g} sx={{ lineHeight: '28px', fontSize: 11 }}>
            {providerName[g] ?? g}
          </ListSubheader>,
          ...ready
            .filter((m) => m.provider === g)
            .map((m) => (
              <MenuItem
                key={m.id}
                dense
                selected={m.id === current?.id}
                onClick={() => {
                  setAnchor(null);
                  void patchSettings({ chatModel: m.id });
                }}
              >
                <Typography sx={{ flex: 1, fontSize: 13 }}>{m.label}</Typography>
                {m.id === current?.id && <CheckIcon sx={{ fontSize: 16 }} />}
              </MenuItem>
            )),
        ])}
        {ready.length === 0 && (
          <MenuItem dense component="a" href={href({ page: 'settings', section: 'model' })} onClick={() => setAnchor(null)}>
            {t('chatNeedKeyLink')}
          </MenuItem>
        )}
      </Menu>
    </>
  );
}
