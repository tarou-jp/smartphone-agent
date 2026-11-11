// Pairing an Android phone over Wi-Fi, after ui/pairing.go: a QR code to
// scan in Wireless debugging, or the six-digit code, and connecting at an
// address when the phone paired but Pinky cannot find it.

import { useEffect, useState } from 'react';
import { Box, Button, CircularProgress, Link, Paper, Stack, TextField, Typography } from '@mui/material';
import { api, pairingQRURL, ApiError } from '@/api/client';
import type { Device } from '@/api/types';
import { useHas } from '@/state/AppState';
import { useT, type T } from '@/i18n';
import { navigate } from '@/lib/router';
import { GuideHeading, GuideNote, GuideProblem, GuideText, GuideWaiting } from './Guide';

const addrRe = /^[\w.:[\]-]+:\d{2,5}$/;

/** Puts a pairing error into the client's words. */
function pairingError(t: T, e: unknown): string {
  const msg = e instanceof ApiError || e instanceof Error ? e.message : String(e);
  const m = msg.toLowerCase();
  if (e instanceof ApiError && e.status === 501) return t('pairingErrUnsupported');
  if (m.includes('wrong') || m.includes('code')) return t('pairingErrWrongCode');
  if (m.includes('timeout') || m.includes('deadline')) return t('pairingErrTimeout');
  if (m.includes('refused') || m.includes('unreachable') || m.includes('no route')) return t('pairingErrUnreachable');
  if (m.includes('adb')) return t('pairingErrNoAdb');
  return t('pairingErrOther', { detail: msg });
}

function Paired({ d }: { d: Device }) {
  const t = useT();
  return (
    <Stack spacing={1.5} alignItems="flex-start" role="status">
      <Typography variant="h3">{d.name ? t('pairingPaired', { name: d.name }) : t('pairingPairedPhone')}</Typography>
      <Button variant="contained" onClick={() => navigate({ page: 'phones', deviceId: d.id })}>
        {t('addPhoneDone')}
      </Button>
    </Stack>
  );
}

function QRPairing({ onPaired, onCode }: { onPaired: (d: Device) => void; onCode: () => void }) {
  const t = useT();
  const [id, setID] = useState<string | null>(null);
  const [err, setErr] = useState('');
  const [waited, setWaited] = useState(0);

  useEffect(() => {
    let live = true;
    let pid: string | null = null;
    const started = Date.now();
    (async () => {
      try {
        const p = await api.startPairing();
        pid = p.id;
        if (!live) {
          void api.cancelPairing(p.id).catch(() => undefined);
          return;
        }
        setID(p.id);
        while (live) {
          const r = await api.waitPairing(p.id);
          setWaited(Date.now() - started);
          if (r.paired && r.device) {
            pid = null;
            onPaired(r.device);
            return;
          }
        }
      } catch (e) {
        if (live) setErr(pairingError(t, e));
      }
    })();
    return () => {
      live = false;
      if (pid) void api.cancelPairing(pid).catch(() => undefined);
    };
    // A pairing per showing of the code.
  }, [onPaired, t]);

  return (
    <Stack spacing={2}>
      <GuideText>{t('pairingQRDetail')}</GuideText>
      <Paper variant="outlined" sx={{ width: 232, height: 232, display: 'grid', placeItems: 'center', borderRadius: 3, bgcolor: '#fff' }}>
        {id ? (
          <Box component="img" src={pairingQRURL(id)} alt={t('pairingQRLabel')} sx={{ width: 208, height: 208, imageRendering: 'pixelated' }} />
        ) : err ? (
          <Typography variant="body2" sx={{ color: '#666', p: 2, textAlign: 'center' }}>
            {t('pairingNoCode')}
          </Typography>
        ) : (
          <Stack alignItems="center" spacing={1}>
            <CircularProgress size={20} />
            <Typography variant="body2" sx={{ color: '#666' }}>
              {t('pairingGettingCode')}
            </Typography>
          </Stack>
        )}
      </Paper>
      {err ? <GuideProblem title={err} /> : id && <GuideWaiting>{t('pairingWaiting')}</GuideWaiting>}
      {!err && waited > 50000 && <GuideProblem title={t('pairingStuckNotSeenTitle')} detail={t('pairingStuckNotSeen')} />}
      <Link component="button" variant="body1" onClick={onCode} sx={{ alignSelf: 'flex-start', fontWeight: 500 }}>
        {t('pairingUseCode')}
      </Link>
    </Stack>
  );
}

function CodePairing({ onPaired, onQR }: { onPaired: (d: Device) => void; onQR: () => void }) {
  const t = useT();
  const [addr, setAddr] = useState('');
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');
  const addrOK = addrRe.test(addr.trim());
  const codeOK = /^\d{6}$/.test(code.trim());
  const pair = async () => {
    if (!addrOK) return setErr(t('pairingAddrInvalid'));
    if (!codeOK) return setErr(t('pairingCodeIncomplete'));
    setBusy(true);
    setErr('');
    try {
      onPaired(await api.pairWithCode(addr.trim(), code.trim()));
    } catch (e) {
      setErr(pairingError(t, e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Stack spacing={2} component="form" onSubmit={(e) => (e.preventDefault(), void pair())}>
      <Typography variant="h3">{t('pairingCodeTitle')}</Typography>
      <GuideText>{t('pairingCodeDetail')}</GuideText>
      <Stack direction="row" spacing={1.5}>
        <TextField label={t('pairingAddrField')} value={addr} onChange={(e) => setAddr(e.target.value)} placeholder="192.168.1.20:37123" sx={{ width: 240 }} />
        <TextField
          label={t('pairingCodeField')}
          value={code}
          onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
          placeholder={t('pairingCodePlaceholder')}
          inputProps={{ inputMode: 'numeric' }}
          sx={{ width: 160 }}
        />
      </Stack>
      {err && <GuideProblem title={err} />}
      <Stack direction="row" spacing={1.5} alignItems="center">
        <Button type="submit" variant="contained" disabled={busy}>
          {busy ? t('pairingBusy') : t('pairingPair')}
        </Button>
        <Link component="button" type="button" variant="body1" onClick={onQR} sx={{ fontWeight: 500 }}>
          {t('pairingUseQR')}
        </Link>
      </Stack>
    </Stack>
  );
}

/** Connecting at the address Wireless debugging shows at its top. */
function ConnectAt({ onConnected }: { onConnected: (d: Device) => void }) {
  const t = useT();
  const [addr, setAddr] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');
  const connect = async () => {
    if (!addrRe.test(addr.trim())) return setErr(t('pairingAddrInvalid'));
    setBusy(true);
    setErr('');
    try {
      onConnected(await api.connectWiFi(addr.trim()));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setErr(/refused|unreachable|timeout/i.test(msg) ? t('pairingConnectErrUnreachable') : t('pairingConnectErrOther', { detail: msg }));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Stack spacing={1.5} component="form" onSubmit={(e) => (e.preventDefault(), void connect())}>
      <Typography variant="h3">{t('pairingConnectTitle')}</Typography>
      <GuideText>{t('pairingConnectDetail')}</GuideText>
      <Stack direction="row" spacing={1.5} alignItems="center">
        <TextField label={t('pairingAddrField')} value={addr} onChange={(e) => setAddr(e.target.value)} sx={{ width: 240 }} />
        <Button type="submit" variant="contained" disabled={busy}>
          {busy ? t('pairingConnectBusy') : t('pairingConnect')}
        </Button>
      </Stack>
      {err && <GuideProblem title={err} />}
    </Stack>
  );
}

export function WirelessPairing({ onUSB }: { onUSB: () => void }) {
  const t = useT();
  const can = useHas('pairing');
  const wifi = useHas('wifi');
  const [mode, setMode] = useState<'qr' | 'code'>('qr');
  const [paired, setPaired] = useState<Device | null>(null);
  // A paired phone Pinky then cannot reach shows offline.
  const unreachable = paired && paired.state !== 'online' && wifi;
  return (
    <>
      <GuideHeading title={t('pairingTitle')} />
      {!can ? (
        <GuideProblem title={t('pairingUnsupported')} />
      ) : paired && !unreachable ? (
        <Paired d={paired} />
      ) : unreachable ? (
        <ConnectAt onConnected={setPaired} />
      ) : mode === 'qr' ? (
        <QRPairing onPaired={setPaired} onCode={() => setMode('code')} />
      ) : (
        <CodePairing onPaired={setPaired} onQR={() => setMode('qr')} />
      )}
      <Box sx={{ mt: 3 }}>
        <GuideNote>
          <Link component="button" variant="body2" onClick={onUSB} sx={{ fontWeight: 500 }}>
            {t('pairingUseUSB')}
          </Link>
        </GuideNote>
      </Box>
    </>
  );
}
