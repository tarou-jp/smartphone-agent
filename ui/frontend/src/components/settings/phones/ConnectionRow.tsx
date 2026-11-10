import { useEffect, useState } from 'react';
import { FormControlLabel, Radio, RadioGroup, Typography, Box } from '@mui/material';
import { Row } from '@/components/common';
import { api } from '@/api/client';
import { useApp } from '@/state/AppState';
import { useT, type T } from '@/i18n';
import type { Connections, ConnectionWay, Device } from '@/api/types';
import { errorText } from '../controls';

function wayName(t: T, id: string): string {
  switch (id) {
    case 'auto':
    case '':
      return t('connectionAuto');
    case 'coredevice':
      return t('connectionCoreDevice');
    case 'wda':
      return t('connectionWDA');
  }
  return id;
}

function wayDesc(t: T, w: ConnectionWay, d: Device): string {
  if (!w.available) {
    switch (w.reason) {
      case 'os_version':
        return w.id === 'coredevice'
          ? t('connectionNeedsIOS27', { version: d.osVersion })
          : t('connectionIOSTooOld', { version: d.osVersion });
      case 'not_installed':
        return t('connectionNotInstalled');
      case 'offline':
        return t('connectionNoAnswer');
    }
    return w.detail ?? '';
  }
  switch (w.id) {
    case 'auto':
      return t('connectionAutoDesc');
    case 'coredevice':
      return t('connectionCoreDeviceDesc');
    case 'wda':
      return t('connectionWDADesc');
  }
  return w.detail ?? '';
}

/** How Pinky reaches an iPhone: automatic, CoreDevice or the WDA helper. */
export function ConnectionRow({ d }: { d: Device }) {
  const t = useT();
  const { toast } = useApp();
  const [c, setC] = useState<Connections | null>(null);
  const load = () => api.connections(d.id).then(setC, () => setC(null));
  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [d.id]);
  if (!c || c.ways.length === 0) return null;
  const chosen = c.chosen || 'auto';
  return (
    <Row label={t('connectionTitle')}>
      <RadioGroup
        value={chosen}
        onChange={async (e) => {
          const way = e.target.value;
          setC({ ...c, chosen: way });
          try {
            await api.setConnection(d.id, way);
            toast(t('connectionReconnecting', { phone: d.name }));
          } catch (err) {
            toast(t('connectionChangeFailed', { phone: d.name, reason: errorText(err) }), 'error');
          }
          void load();
        }}
        sx={{ mt: 1 }}
      >
        {c.ways.map((w) => (
          <FormControlLabel
            key={w.id}
            value={w.id}
            disabled={!w.available && w.id !== chosen}
            control={<Radio size="small" />}
            sx={{ alignItems: 'flex-start', mb: 0.75, '& .MuiRadio-root': { pt: 0.25 } }}
            label={
              <Box>
                <Typography variant="subtitle1">
                  {w.id === 'auto' && c.current
                    ? t('connectionAutoNow', { way: wayName(t, c.current) })
                    : wayName(t, w.id)}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {wayDesc(t, w, d)}
                </Typography>
              </Box>
            }
          />
        ))}
      </RadioGroup>
    </Row>
  );
}
