// The client's MUI theme, from the MyGo client's palette (ui/style.go) and
// the brand (assets/BRAND.md): indigo for actions, amber only for Needs
// you, red only for Stop, watermelon and lemon only for the brand.

import { createTheme, alpha, type Theme } from '@mui/material/styles';

export interface PinkyTokens {
  bg2: string;
  bg3: string;
  sel: string;
  line: string;
  line2: string;
  ink2: string;
  ink3: string;
  accentTint: string;
  amber: string;
  amberDot: string;
  amberTint: string;
  amberLine: string;
  red: string;
  redTint: string;
  redLine: string;
  green: string;
  overlay: string;
  phone: string;
  brand: string;
  lemon: string;
}

declare module '@mui/material/styles' {
  interface Theme {
    pinky: PinkyTokens;
  }
  interface ThemeOptions {
    pinky?: PinkyTokens;
  }
}

const light = {
  bg: '#ffffff',
  panel: '#ffffff',
  ink: '#1d1d22',
  accent: '#5450d6',
  tokens: {
    bg2: '#f7f7f8',
    bg3: '#efeff2',
    sel: '#e7e7ed',
    line: '#ebebef',
    line2: '#d9d9e0',
    ink2: '#5b5b64',
    ink3: '#6c6c75',
    accentTint: '#f1f0fd',
    amber: '#975800',
    amberDot: '#df8601',
    amberTint: '#fff6eb',
    amberLine: '#f6d7b1',
    red: '#c32c18',
    redTint: '#fef0ed',
    redLine: '#f0c6bd',
    green: '#2f9e5a',
    overlay: '#5048e5',
    phone: '#131317',
    brand: '#ee4d6c',
    lemon: '#ffbe1a',
  } satisfies PinkyTokens,
};

const dark = {
  bg: '#1c1c1f',
  panel: '#202024',
  ink: '#ededf2',
  accent: '#9895ff',
  tokens: {
    bg2: '#212125',
    bg3: '#2b2b30',
    sel: '#323238',
    line: '#2e2e34',
    line2: '#404047',
    ink2: '#b3b3bc',
    ink3: '#a3a3ad',
    accentTint: '#262549',
    amber: '#f7a74e',
    amberDot: '#f7a74e',
    amberTint: '#342210',
    amberLine: '#674520',
    red: '#f87a60',
    redTint: '#3a1e19',
    redLine: '#723b31',
    green: '#4cc38a',
    overlay: '#7a73ff',
    phone: '#050506',
    brand: '#ff8a9b',
    lemon: '#ffbe1a',
  } satisfies PinkyTokens,
};

export function makeTheme(mode: 'light' | 'dark'): Theme {
  const p = mode === 'dark' ? dark : light;
  const t = p.tokens;
  return createTheme({
    pinky: t,
    palette: {
      mode,
      primary: { main: mode === 'dark' ? '#5b57e0' : p.accent, contrastText: '#ffffff' },
      secondary: { main: t.brand },
      error: { main: t.red },
      warning: { main: t.amberDot, contrastText: '#ffffff' },
      success: { main: t.green },
      background: { default: p.bg, paper: p.panel },
      text: { primary: p.ink, secondary: t.ink2, disabled: t.ink3 },
      divider: t.line,
      action: { selected: t.sel, hover: alpha(p.ink, 0.05) },
    },
    shape: { borderRadius: 8 },
    typography: {
      fontFamily:
        '-apple-system, BlinkMacSystemFont, "Segoe UI Variable", "Segoe UI", system-ui, "Hiragino Sans", "PingFang SC", "Microsoft YaHei", sans-serif',
      fontSize: 13,
      h1: { fontSize: 22, fontWeight: 650 },
      h2: { fontSize: 17, fontWeight: 650 },
      h3: { fontSize: 15, fontWeight: 650 },
      subtitle1: { fontSize: 13, fontWeight: 600 },
      subtitle2: { fontSize: 12, fontWeight: 600, color: t.ink2 },
      body1: { fontSize: 13 },
      body2: { fontSize: 12 },
      caption: { fontSize: 11, color: t.ink3 },
      button: { textTransform: 'none', fontWeight: 600 },
    },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          'html, body, #root': { height: '100%' },
          body: { overflow: 'hidden', WebkitFontSmoothing: 'antialiased', userSelect: 'none' },
          'input, textarea, [contenteditable], .selectable': { userSelect: 'text' },
          '*::-webkit-scrollbar': { width: 10, height: 10 },
          '*::-webkit-scrollbar-thumb': {
            background: t.line2,
            borderRadius: 8,
            border: `3px solid transparent`,
            backgroundClip: 'content-box',
          },
          '*::-webkit-scrollbar-track': { background: 'transparent' },
        },
      },
      MuiButtonBase: { defaultProps: { disableRipple: true } },
      MuiButton: {
        defaultProps: { disableElevation: true, size: 'small' },
        styleOverrides: { root: { borderRadius: 7, paddingInline: 12, minHeight: 30 } },
      },
      MuiIconButton: { defaultProps: { size: 'small' }, styleOverrides: { root: { borderRadius: 7 } } },
      MuiTooltip: { defaultProps: { arrow: false, enterDelay: 400 } },
      MuiPaper: {
        defaultProps: { elevation: 0 },
        styleOverrides: { outlined: { borderColor: t.line } },
      },
      MuiCard: {
        defaultProps: { variant: 'outlined' },
        styleOverrides: { root: { borderRadius: 12 } },
      },
      MuiTextField: { defaultProps: { size: 'small' } },
      MuiSelect: { defaultProps: { size: 'small' } },
      MuiSwitch: { defaultProps: { size: 'small' } },
      MuiChip: { defaultProps: { size: 'small' }, styleOverrides: { root: { fontWeight: 600 } } },
      MuiListItemButton: {
        styleOverrides: {
          root: {
            borderRadius: 7,
            '&.Mui-selected': { background: t.sel },
            '&.Mui-selected:hover': { background: t.sel },
          },
        },
      },
      MuiDialog: { styleOverrides: { paper: { borderRadius: 14 } } },
      MuiTab: { styleOverrides: { root: { minHeight: 36, textTransform: 'none', fontWeight: 600 } } },
      MuiTabs: { styleOverrides: { root: { minHeight: 36 } } },
    },
  });
}
