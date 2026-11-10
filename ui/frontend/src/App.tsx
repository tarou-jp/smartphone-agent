import { useEffect, useMemo, useState } from 'react';
import { Alert, Box, CssBaseline, LinearProgress, ThemeProvider, useMediaQuery } from '@mui/material';
import { makeTheme } from '@/theme/theme';
import { I18nProvider, useT } from '@/i18n';
import { AppStateProvider, useApp } from '@/state/AppState';
import { navigate, useRoute } from '@/lib/router';
import { forcedTheme, api } from '@/api/client';
import { Sidebar } from '@/components/layout/Sidebar';
import { Toasts } from '@/components/common';
import { PhonesPage } from '@/pages/PhonesPage';
import { ActivityPage } from '@/pages/ActivityPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { AddPhonePage } from '@/pages/AddPhonePage';
import { isRunning } from '@/lib/format';

function Shell() {
  const route = useRoute();
  const t = useT();
  const { boot, loaded, connected, live, run } = useApp();

  // The start page the app was asked for, once.
  const [started, setStarted] = useState(false);
  useEffect(() => {
    if (started || !boot) return;
    setStarted(true);
    if (!window.location.hash && boot.startPage) {
      const page = boot.startPage as 'activity' | 'settings' | 'add-phone';
      navigate({ page }, true);
    }
  }, [boot, started]);

  // Stop all: Option-Command-Period, or Ctrl+Alt+Period, while the window
  // has focus. (The MyGo client also registered it system-wide.)
  useEffect(() => {
    const on = (e: KeyboardEvent) => {
      const mac = navigator.platform.startsWith('Mac');
      if (e.key === '.' || e.code === 'Period') {
        if (mac ? e.metaKey && e.altKey : e.ctrlKey && e.altKey) {
          e.preventDefault();
          if (live.tasks.some(isRunning)) void run(t('callStopAll'), () => api.stopAll());
        }
      }
    };
    window.addEventListener('keydown', on);
    return () => window.removeEventListener('keydown', on);
  }, [live.tasks, run, t]);

  // The title says how many things need the person.
  useEffect(() => {
    const n = live.needs.length;
    document.title = n > 0 ? `(${n}) Pinky` : 'Pinky';
  }, [live.needs.length]);

  let page;
  switch (route.page) {
    case 'activity':
      page = <ActivityPage taskId={route.taskId} />;
      break;
    case 'settings':
      page = <SettingsPage section={route.section} />;
      break;
    case 'add-phone':
      page = <AddPhonePage platform={route.platform} />;
      break;
    default:
      page = <PhonesPage deviceId={route.deviceId} />;
  }
  return (
    <Box sx={{ display: 'flex', height: '100vh', bgcolor: 'background.default', color: 'text.primary' }}>
      <Sidebar route={route} />
      <Box component="main" sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', position: 'relative' }}>
        {!loaded && <LinearProgress sx={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 2 }} />}
        {!connected && (
          <Alert severity="warning" sx={{ borderRadius: 0 }}>
            {t('webDisconnected')}
          </Alert>
        )}
        {page}
      </Box>
      <Toasts />
    </Box>
  );
}

function Themed() {
  const dark = useMediaQuery('(prefers-color-scheme: dark)');
  const mode = forcedTheme ?? (dark ? 'dark' : 'light');
  const theme = useMemo(() => makeTheme(mode), [mode]);
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <AppStateProvider>
        <Shell />
      </AppStateProvider>
    </ThemeProvider>
  );
}

export default function App() {
  return (
    <I18nProvider>
      <Themed />
    </I18nProvider>
  );
}
