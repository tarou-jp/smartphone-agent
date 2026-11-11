// Connecting an Android phone, after ui/addandroid.go: pick the maker,
// turn on Developer options and USB debugging, plug in and allow, then a
// checkup. Pinky follows the phone as it shows up.

import { useEffect, useState } from 'react';
import { Box, Button, Chip, Link, Paper, Stack, Typography, useTheme } from '@mui/material';
import CheckIcon from '@mui/icons-material/CheckCircleRounded';
import { api } from '@/api/client';
import type { Checkup, Device, HostCheck } from '@/api/types';
import { CheckList as CheckItems } from '@/components/common/CheckList';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import { navigate } from '@/lib/router';
import { brands, brandByID, type Brand } from './brands';
import { GuideHeading, GuideNote, GuideProblem, GuideStep, GuideText, GuideWaiting, MenuPath } from './Guide';
import { useAdding, useHostCheck, useSince } from './useAdding';

const hostWords: Record<string, string> = {
  adb: 'checkHostNoADB',
  adb_server: 'checkHostADBServer',
  adb_old: 'checkHostADBOld',
  adb_restarts: 'checkHostADBRestarts',
};

function HostProblem({ host }: { host: HostCheck }) {
  const t = useT();
  const key = host.missing ? hostWords[host.missing] : undefined;
  if (!key) return null;
  return <GuideProblem title={t(key)} detail={host.detail} />;
}

function BrandPicker({ value, onChange }: { value: string; onChange: (id: string) => void }) {
  const t = useT();
  return (
    <Box sx={{ mb: 2.5 }}>
      <Typography variant="subtitle1" sx={{ mb: 1 }}>
        {t('addAndroidBrandQuestion')}
      </Typography>
      <Stack direction="row" flexWrap="wrap" useFlexGap spacing={1}>
        {brands.map((b) => (
          <Chip
            key={b.id}
            label={t(b.name)}
            clickable
            color={b.id === value ? 'primary' : 'default'}
            variant={b.id === value ? 'filled' : 'outlined'}
            onClick={() => onChange(b.id)}
            sx={{ height: 28, fontSize: 12 }}
          />
        ))}
      </Stack>
      <Typography variant="caption" sx={{ display: 'block', mt: 1 }}>
        {t('addAndroidNamesDiffer')}
      </Typography>
    </Box>
  );
}

function Switches({ b }: { b: Brand }) {
  const t = useT();
  return (
    <Stack spacing={1}>
      {b.switches.map((s) => (
        <Box key={s.name}>
          {s.path && <MenuPath path={t(s.path)} />}
          <Typography variant="body1" sx={{ fontWeight: 500, mt: s.path ? 0.75 : 0 }}>
            {s.off ? t('addAndroidTurnOff', { name: t(s.name) }) : t('addAndroidTurnOn', { name: t(s.name) })}
          </Typography>
          {s.why && <GuideNote>{t(s.why)}</GuideNote>}
        </Box>
      ))}
    </Stack>
  );
}

/** A drawing of the phone's "Allow USB debugging?" prompt. */
function AllowSketch({ always, allow }: { always: string; allow: string }) {
  const t = useT();
  const p = useTheme().pinky;
  return (
    <Paper variant="outlined" aria-label={t('addAndroidSketchLabel')} sx={{ p: 2, maxWidth: 320, borderRadius: 3, bgcolor: p.bg2 }}>
      <Typography sx={{ fontWeight: 600, fontSize: 15, mb: 1 }}>{t('addAndroidSketchTitle')}</Typography>
      <Stack direction="row" spacing={1} alignItems="center">
        <CheckIcon color="primary" sx={{ fontSize: 18 }} />
        <Typography variant="body2">{always}</Typography>
      </Stack>
      <Stack direction="row" justifyContent="flex-end" spacing={2} sx={{ mt: 1.5 }}>
        <Typography variant="body2" sx={{ color: p.ink3, fontWeight: 500 }}>
          {t('addAndroidSketchCancel')}
        </Typography>
        <Typography variant="body2" color="primary" sx={{ fontWeight: 600 }}>
          {allow}
        </Typography>
      </Stack>
    </Paper>
  );
}

function PlugStatus({ d }: { d?: Device }) {
  const t = useT();
  const waited = useSince(d ? d.id + d.state : undefined);
  if (!d) {
    return (
      <>
        <GuideWaiting>{t('addPhoneLooking')}</GuideWaiting>
        <GuideNote>{t('addAndroidNotSeen')}</GuideNote>
      </>
    );
  }
  if (d.state === 'unauthorized') {
    return (
      <>
        <GuideWaiting>{t('addAndroidFoundAllow', { name: d.name })}</GuideWaiting>
        {waited > 20000 && <GuideProblem title={t('addAndroidNoPrompt')} />}
      </>
    );
  }
  if (d.state === 'needs_setup') {
    if (d.setup === 'usb_permission') {
      return (
        <GuideProblem title={t('checkUSBPermission')} detail={d.detail}>
          <Box sx={{ mt: 0.5 }}>{t('checkUSBPermissionNote')}</Box>
        </GuideProblem>
      );
    }
    return <GuideProblem title={t('checkUSBNotReady')} detail={d.detail} />;
  }
  if (d.state === 'offline') return <GuideProblem title={t('addAndroidOffline', { name: d.name })} detail={d.detail} />;
  return null;
}

function CheckList({ d }: { d: Device }) {
  const t = useT();
  const { run } = useApp();
  const [result, setResult] = useState<Checkup | null>(null);
  const [busy, setBusy] = useState(false);
  const check = async () => {
    setBusy(true);
    const r = await run(t('webAddRunCheckup'), () => api.checkup(d.id));
    if (r) setResult(r);
    setBusy(false);
  };
  useEffect(() => {
    void check();
    // Once per phone.
  }, [d.id]);
  return (
    <>
      <Stack direction="row" spacing={1}>
        <Button variant="contained" onClick={() => navigate({ page: 'phones', deviceId: d.id })}>
          {t('addPhoneDone')}
        </Button>
        <Button variant="outlined" disabled={busy} onClick={check}>
          {t('checkupAgain')}
        </Button>
      </Stack>
      {busy && <GuideWaiting>{t('checkupRunning')}</GuideWaiting>}
      {result && (
        <CheckItems checks={result.checks} />
      )}
    </>
  );
}

export function AndroidSteps({ onWireless }: { onWireless: () => void }) {
  const t = useT();
  const host = useHostCheck('android');
  const pairing = useHas('pairing');
  const { device: d } = useAdding('android');
  const [brand, setBrand] = useState('other');
  const b = brandByID(brand);
  const online = d?.state === 'online';
  const seen = !!d;
  const always = t(b.promptAlways ?? 'addAndroidSketchAlways');
  const allow = t(b.promptAllow ?? 'addAndroidSketchAllow');
  return (
    <>
      <GuideHeading title={t('addAndroidTitle')} detail={t('addAndroidDetail')} />
      <BrandPicker value={brand} onChange={setBrand} />
      <Box sx={{ mb: 2 }}>
        <HostProblem host={host} />
      </Box>
      <Stack spacing={2.5} role="list" aria-label={t('addAndroidStepsLabel')}>
        <GuideStep n={1} title={t('addAndroidStepDev')} state={seen ? 'done' : 'now'}>
          <MenuPath path={t(b.about)} item={t(b.tap)} />
          <GuideText>{t('addAndroidStepDevTap', { item: t(b.tap) })}</GuideText>
          {b.aboutNote && <GuideNote>{t(b.aboutNote)}</GuideNote>}
        </GuideStep>
        <GuideStep n={2} title={t('addAndroidStepUSB')} state={seen ? 'done' : 'now'}>
          <MenuPath path={t(b.dev)} />
          <Switches b={b} />
          {b.note && <GuideNote>{t(b.note)}</GuideNote>}
        </GuideStep>
        <GuideStep n={3} title={t('addAndroidStepPlug')} state={online ? 'done' : 'now'}>
          <GuideText>{t('addAndroidStepPlugDetail', { button: allow })}</GuideText>
          <AllowSketch always={always} allow={allow} />
          <PlugStatus d={d} />
        </GuideStep>
        <GuideStep n={4} title={t('addAndroidStepCheck')} state={online ? 'now' : 'later'}>
          <GuideText>{t('addAndroidStepCheckDetail')}</GuideText>
          {online && d && <CheckList d={d} />}
        </GuideStep>
      </Stack>
      {!online && pairing && (
        <Box sx={{ mt: 3 }}>
          <Link component="button" variant="body1" onClick={onWireless} sx={{ fontWeight: 500 }}>
            {t('addPhoneOrWiFi')}
          </Link>
        </Box>
      )}
    </>
  );
}
