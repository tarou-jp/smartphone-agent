// Connecting an iPhone, after ui/addiphone.go: Apple's driver or usbmuxd
// where the computer lacks it, plug in, Trust, Developer Mode, then Pinky
// prepares the phone. The steps follow the phone's state.

import { useEffect, useState } from 'react';
import { Box, Button, LinearProgress, Link, Paper, Stack, Typography, useTheme } from '@mui/material';
import { api } from '@/api/client';
import type { Device, DiskImage, HostCheck } from '@/api/types';
import { useApp, useHas } from '@/state/AppState';
import { useT } from '@/i18n';
import { href, navigate } from '@/lib/router';
import { GuideHeading, GuideNote, GuideProblem, GuideStep, GuideText, GuideWaiting, type GuideState } from './Guide';
import { useAdding, useHostCheck, useSince } from './useAdding';

const xcodeAppStore = 'https://apps.apple.com/app/xcode/id497799835';

enum Step {
  Host,
  Plug,
  Trust,
  DevMode,
  Prepare,
  Ready,
}

function stepOf(d: Device | undefined, host: HostCheck): Step {
  if (host.missing && host.missing !== 'usbmuxd_stopped') return Step.Host;
  if (!d || d.state === 'offline') return Step.Plug;
  if (d.state === 'online') return Step.Ready;
  if (d.state === 'unauthorized' || d.setup === 'trust') return Step.Trust;
  if (d.setup === 'developer_mode') return Step.DevMode;
  return Step.Prepare;
}

function bytes(locale: string, n: number): string {
  return new Intl.NumberFormat(locale, { style: 'unit', unit: 'megabyte', maximumFractionDigits: 1 }).format(n / 1e6);
}

/** The third-party mirror of Apple's developer disk image, on a click. */
function MirrorOffer() {
  const t = useT();
  const { diskImage, run } = useApp();
  const [info, setInfo] = useState<DiskImage | null>(null);
  const locale = document.documentElement.lang || 'en';
  useEffect(() => {
    api.diskImage().then(setInfo, () => undefined);
  }, []);
  if (!info || !info.needed) return null;
  const prog = diskImage ?? info.progress;
  return (
    <Paper variant="outlined" sx={{ p: 2, borderRadius: 2 }}>
      <Typography variant="subtitle1">{t('addPhoneIPhoneMirrorTitle')}</Typography>
      <GuideNote>{t('addPhoneIPhoneMirrorDetail', { source: info.name, image: info.image, size: bytes(locale, info.size) })}</GuideNote>
      <Box sx={{ mt: 1.5 }}>
        {prog.running ? (
          <Stack spacing={0.75}>
            <Typography variant="body2">
              {t('addPhoneIPhoneMirrorDownloading', { done: bytes(locale, prog.done), total: bytes(locale, prog.total || info.size) })}
            </Typography>
            <LinearProgress variant={prog.total ? 'determinate' : 'indeterminate'} value={prog.total ? (100 * prog.done) / prog.total : 0} />
          </Stack>
        ) : prog.ok ? (
          <GuideWaiting>{t('addPhoneIPhoneMirrorDone')}</GuideWaiting>
        ) : (
          <Stack spacing={1} alignItems="flex-start">
            {prog.error && (
              <GuideProblem
                title={/checksum|mismatch/i.test(prog.error) ? t('addPhoneIPhoneMirrorMismatch') : t('addPhoneIPhoneMirrorFailed')}
                detail={prog.error}
              >
                <Box sx={{ mt: 0.5 }}>{t('addPhoneIPhoneMirrorOfficial')}</Box>
              </GuideProblem>
            )}
            <Button variant="outlined" onClick={() => void run(t('webAddDownloadImage'), () => api.downloadDiskImage())}>
              {t('addPhoneIPhoneMirrorDownload', { size: bytes(locale, info.size) })}
            </Button>
          </Stack>
        )}
      </Box>
    </Paper>
  );
}

function Prepare({ d, platform }: { d: Device; platform: string }) {
  const t = useT();
  const hasMirror = useHas('diskImage');
  switch (d.setup) {
    case 'preparing':
      return <GuideWaiting>{t('addPhoneIPhoneStepPreparing')}</GuideWaiting>;
    case 'unlock':
      return <GuideProblem title={t('addPhoneIPhoneStepUnlock')} />;
    case 'passcode':
      return <GuideProblem title={t('addPhoneIPhoneStepPasscodeDetail')} />;
    case 'connection':
      return (
        <GuideProblem title={t('connectionChosenNeedsIOS27', { version: d.osVersion })}>
          <Box sx={{ mt: 0.5 }}>
            <Link href={href({ page: 'settings', section: 'phones' })} sx={{ fontWeight: 600 }}>
              {t('connectionChoose')}
            </Link>
          </Box>
        </GuideProblem>
      );
    case 'no_disk_image':
      return (
        <>
          {platform === 'darwin' ? (
            <GuideProblem title={t('addPhoneIPhoneNoImageTitle')} detail={t('addPhoneIPhoneNoImageMac')}>
              <Box sx={{ mt: 0.5 }}>
                <Link href={xcodeAppStore} target="_blank" rel="noreferrer" sx={{ fontWeight: 600 }}>
                  {t('addPhoneIPhoneGetXcode')}
                </Link>
              </Box>
            </GuideProblem>
          ) : (
            <>
              <GuideProblem title={t('addPhoneIPhoneNoImageTitle')} detail={t('addPhoneIPhoneNoImageCopy')} />
              {d.detail && <GuideNote>{d.detail}</GuideNote>}
            </>
          )}
          {hasMirror && <MirrorOffer />}
        </>
      );
    default:
      return (
        <>
          <GuideProblem title={t('addPhoneIPhoneStuck')} detail={d.detail} />
          <GuideNote>{t('addPhoneIPhoneRetrying')}</GuideNote>
        </>
      );
  }
}

export function IPhoneSteps() {
  const t = useT();
  const p = useTheme().pinky;
  const { boot } = useApp();
  const platform = boot?.platform ?? 'darwin';
  let host = useHostCheck('ios');
  if (platform !== 'windows' && platform !== 'linux') host = {}; // macOS has usbmuxd built in
  const { device: d, already } = useAdding('ios');
  const cur = stepOf(d, host);
  const waited = useSince(String(cur));

  if (d && d.setup === 'update') {
    return (
      <>
        <GuideHeading title={t('addPhoneIPhoneTitle')} detail={t('addPhoneIPhoneDetail')} />
        <GuideProblem title={t('addPhoneIPhoneTooOldTitle', { name: d.name })} detail={t('addPhoneIPhoneTooOld', { version: d.osVersion })} />
      </>
    );
  }

  const state = (s: Step): GuideState => (s < cur ? 'done' : s === cur ? 'now' : 'later');
  let n = 0;
  return (
    <>
      <GuideHeading title={t('addPhoneIPhoneTitle')} detail={t('addPhoneIPhoneDetail')} />
      <Stack spacing={2.5} role="list" aria-label={t('addPhoneIPhoneStepsLabel')}>
        {(platform === 'windows' || platform === 'linux') && (
          <GuideStep
            n={++n}
            state={state(Step.Host)}
            title={platform === 'linux' ? t('addPhoneIPhoneStepUsbmuxd') : t('addPhoneIPhoneStepDriver')}
          >
            <GuideText>{platform === 'linux' ? t('addPhoneIPhoneStepUsbmuxdDetail') : t('addPhoneIPhoneStepDriverDetail')}</GuideText>
            <GuideWaiting>{t('addPhoneIPhoneStepDriverWaiting')}</GuideWaiting>
          </GuideStep>
        )}
        <GuideStep n={++n} state={state(Step.Plug)} title={t('addPhoneIPhoneStepPlug')}>
          <GuideText>{t('addPhoneIPhoneStepPlugDetail')}</GuideText>
          {d && d.state === 'offline' ? (
            <GuideProblem title={t('addPhoneIPhoneNoAnswer', { name: d.name })} detail={d.detail} />
          ) : (
            <GuideWaiting>{t('addPhoneLookingIPhone')}</GuideWaiting>
          )}
          {host.missing === 'usbmuxd_stopped' && waited > 8000 && <GuideProblem title={t('addPhoneIPhoneUsbmuxdStopped')} />}
        </GuideStep>
        {cur > Step.Plug && d && (
          <Typography variant="body2" sx={{ color: p.ink2, mt: '0px !important', pl: 4.5 }}>
            {t('addPhoneFoundIOS', { name: d.name, version: d.osVersion }).trim()}
          </Typography>
        )}
        <GuideStep n={++n} state={state(Step.Trust)} title={t('addPhoneIPhoneStepTrust')}>
          <GuideText>{t('addPhoneIPhoneStepTrustDetail')}</GuideText>
          <GuideWaiting>{t('addPhoneIPhoneStepTrustWaiting')}</GuideWaiting>
        </GuideStep>
        <GuideStep n={++n} state={state(Step.DevMode)} title={t('addPhoneIPhoneStepDevMode')}>
          <GuideText>{t('addPhoneIPhoneStepDevModeDetail')}</GuideText>
          <GuideNote>{t('addPhoneIPhoneStepDevModeNote')}</GuideNote>
        </GuideStep>
        <GuideStep n={++n} state={state(Step.Prepare)} title={t('addPhoneIPhoneStepPrepare')}>
          <GuideText>{t('addPhoneIPhoneStepPrepareDetail')}</GuideText>
          {d && cur === Step.Prepare && <Prepare d={d} platform={platform} />}
        </GuideStep>
      </Stack>
      {cur === Step.Ready && d && (
        <Stack spacing={1.5} sx={{ mt: 3 }} alignItems="flex-start" role="status">
          <Stack direction="row" spacing={1} alignItems="center">
            <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: p.green }} />
            <Typography sx={{ fontWeight: 600 }}>{t('addPhoneReady', { name: d.name })}</Typography>
          </Stack>
          {already && <GuideNote>{t('addPhoneIPhoneAnother')}</GuideNote>}
          <Button variant="contained" onClick={() => navigate({ page: 'phones', deviceId: d.id })}>
            {t('addPhoneDone')}
          </Button>
        </Stack>
      )}
    </>
  );
}
