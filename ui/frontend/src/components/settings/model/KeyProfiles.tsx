import { useState } from 'react';
import { Alert, Box, Button, MenuItem, Select, Stack, TextField, Typography } from '@mui/material';
import { RowCard, StatusText } from '@/components/common';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT } from '@/i18n';
import type { ModelView, ProfileView } from '@/api/types';
import { errorText } from '../controls';

/** A key field: never shows a saved key, only takes a new one. */
function KeyForm({
  providers,
  withProvider,
  onSave,
  onCancel,
}: {
  providers: ModelView['providers'];
  withProvider: boolean;
  onSave: (provider: string, label: string, key: string) => Promise<void>;
  onCancel: () => void;
}) {
  const t = useT();
  const [provider, setProvider] = useState(providers[0]?.id ?? 'anthropic');
  const [label, setLabel] = useState('');
  const [key, setKey] = useState('');
  const [busy, setBusy] = useState(false);
  const save = async () => {
    if (!key.trim()) return;
    setBusy(true);
    try {
      await onSave(provider, label, key);
    } finally {
      setBusy(false);
    }
  };
  return (
    <Stack spacing={1.25} sx={{ p: 2 }}>
      {withProvider && (
        <Stack direction="row" spacing={1}>
          <Select value={provider} onChange={(e) => setProvider(e.target.value)} sx={{ minWidth: 160 }} aria-label={t('settingsModelProvider')}>
            {providers.map((p) => (
              <MenuItem key={p.id} value={p.id}>
                {p.label}
              </MenuItem>
            ))}
          </Select>
          <TextField
            label={t('settingsModelKeyLabel')}
            placeholder={t('settingsModelKeyLabelPlaceholder')}
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            sx={{ flex: 1 }}
          />
        </Stack>
      )}
      <TextField
        type="password"
        autoComplete="off"
        autoFocus
        label={t('settingsModelKey')}
        placeholder={t('settingsModelKeyPlaceholder')}
        value={key}
        onChange={(e) => setKey(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && void save()}
      />
      <Stack direction="row" spacing={1}>
        <Button variant="contained" disabled={busy || !key.trim()} onClick={save}>
          {t('settingsModelKeySave')}
        </Button>
        <Button color="inherit" onClick={onCancel}>
          {t('settingsCancel')}
        </Button>
      </Stack>
    </Stack>
  );
}

function ProfileRow({ p, m, onModel }: { p: ProfileView; m: ModelView; onModel: (v: ModelView) => void }) {
  const t = useT();
  const { toast } = useApp();
  const [editing, setEditing] = useState(false);
  const prov = m.providers.find((x) => x.id === p.provider)?.label ?? p.provider;
  const active = m.active === p.id;
  return (
    <Box>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ px: 2, py: 1.25 }}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="subtitle1">{p.label}</Typography>
          <Typography variant="body2" color="text.secondary">
            {prov} · {p.hasKey ? t('settingsModelKeySaved') : t('settingsModelKeyMissing')}
          </Typography>
        </Box>
        {active ? (
          <StatusText tone="good">{t('settingsModelKeyInUse')}</StatusText>
        ) : (
          <Button
            color="inherit"
            onClick={() => api.setActiveKey(p.id).then(onModel, (e) => toast(errorText(e), 'error'))}
          >
            {t('settingsModelKeyUse')}
          </Button>
        )}
        <Button color="inherit" variant="outlined" onClick={() => setEditing(!editing)}>
          {t('settingsModelKeyChange')}
        </Button>
        <Button
          color="error"
          onClick={() =>
            api.removeModelKey(p.id).then(
              (v) => {
                onModel(v);
                toast(t('settingsModelKeyRemovedToast', { provider: prov }));
              },
              (e) => toast(t('settingsModelKeyErrRemove', { detail: errorText(e) }), 'error'),
            )
          }
        >
          {t('settingsModelKeyRemove')}
        </Button>
      </Stack>
      {editing && (
        <KeyForm
          providers={m.providers}
          withProvider={false}
          onCancel={() => setEditing(false)}
          onSave={async (_, __, key) => {
            try {
              onModel(await api.editModelKey(p.id, { key }));
              setEditing(false);
              toast(t('settingsModelKeySavedToast', { provider: prov }), 'success');
            } catch (e) {
              toast(t('settingsModelKeyErrSave', { detail: errorText(e) }), 'error');
            }
          }}
        />
      )}
    </Box>
  );
}

/** Settings › Model's API key profiles. */
export function KeyProfiles({ m, onModel }: { m: ModelView; onModel: (v: ModelView) => void }) {
  const t = useT();
  const { toast } = useApp();
  const [adding, setAdding] = useState(false);
  const [err, setErr] = useState('');
  if (!m.keychain) return <Alert severity="info">{t('unlockNotAvailable')}</Alert>;
  return (
    <>
      <RowCard>
        {m.profiles.map((p) => (
          <ProfileRow key={p.id} p={p} m={m} onModel={onModel} />
        ))}
        {adding ? (
          <KeyForm
            providers={m.providers}
            withProvider
            onCancel={() => setAdding(false)}
            onSave={async (provider, label, key) => {
              try {
                onModel(await api.addModelKey(provider, label, key));
                setAdding(false);
                setErr('');
                toast(
                  t('settingsModelKeySavedToast', { provider: m.providers.find((p) => p.id === provider)?.label ?? provider }),
                  'success',
                );
              } catch (e) {
                setErr(t('settingsModelKeyErrSave', { detail: errorText(e) }));
              }
            }}
          />
        ) : (
          <Box sx={{ px: 2, py: 1 }}>
            <Button onClick={() => setAdding(true)}>{t('settingsModelKeyAdd')}</Button>
          </Box>
        )}
      </RowCard>
      {err && (
        <Alert severity="error" sx={{ mt: 1 }}>
          {err}
        </Alert>
      )}
    </>
  );
}
