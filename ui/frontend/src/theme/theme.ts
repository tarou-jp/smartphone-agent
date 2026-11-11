// The client's MUI theme, from the MyGo client's palette (ui/style.go) and
// the brand (assets/BRAND.md). The chrome is ink: primary actions are ink
// on light and off-white on dark, so the screen's color goes to what
// matters. Indigo (overlay) marks only the agent at work, amber only Needs
// you, red only Stop, watermelon and lemon only the brand.

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
  /** Geist Mono, for times, counts, durations and keycaps. */
  mono: string;
  cardShadow: string;
  floatShadow: string;
  /** A top highlight for raised things on dark; none on light. */
  highlight: string;
}

declare module '@mui/material/styles' {
  interface Theme {
    pinky: PinkyTokens;
  }
  interface ThemeOptions {
    pinky?: PinkyTokens;
  }
}

// Warm neutral ramps, after Claude Desktop: a paper-like page, a slightly
// deeper sidebar and cards one step up, with clear steps between them so
// surfaces read apart without heavy borders.
const light = {
  bg: '#faf9f5',
  panel: '#ffffff',
  ink: '#1f1e1d',
  tokens: {
    bg2: '#f3f1ea',
    bg3: '#ebe9e0',
    sel: '#e8e6dc',
    line: 'rgba(31, 30, 29, 0.12)',
    line2: 'rgba(31, 30, 29, 0.24)',
    ink2: '#4a4844',
    ink3: '#6b6a64',
    accentTint: '#eeedfb',
    amber: '#8a5200',
    amberDot: '#d98200',
    amberTint: '#fcf2dc',
    amberLine: '#ecd3a0',
    red: '#b93a26',
    redTint: '#fbece8',
    redLine: '#ecc5bb',
    green: '#2f8f55',
    overlay: '#5048e5',
    phone: '#141413',
    brand: '#ee4d6c',
    lemon: '#ffbe1a',
    mono: '"Geist Mono Variable", ui-monospace, SFMono-Regular, Menlo, monospace',
    cardShadow: '0 1px 2px rgba(31, 30, 29, 0.04)',
    floatShadow: '0 8px 28px rgba(31, 30, 29, 0.12)',
    highlight: 'none',
  } satisfies PinkyTokens,
};

const dark = {
  bg: '#262624',
  panel: '#30302e',
  ink: '#faf9f5',
  tokens: {
    bg2: '#1f1e1d',
    bg3: '#3a3937',
    sel: '#3d3c39',
    line: 'rgba(222, 220, 209, 0.16)',
    line2: 'rgba(222, 220, 209, 0.30)',
    ink2: '#d1cfc5',
    ink3: '#aeaca3',
    accentTint: '#2f2d55',
    amber: '#f0b060',
    amberDot: '#eba64a',
    amberTint: '#3b2f1d',
    amberLine: '#6b5026',
    red: '#f08a72',
    redTint: '#3d2621',
    redLine: '#6f3d33',
    green: '#6cc596',
    overlay: '#8983ff',
    phone: '#0d0d0c',
    brand: '#ff8a9b',
    lemon: '#ffbe1a',
    mono: '"Geist Mono Variable", ui-monospace, SFMono-Regular, Menlo, monospace',
    cardShadow: 'none',
    floatShadow: '0 12px 32px rgba(0, 0, 0, 0.4)',
    highlight: 'none',
  } satisfies PinkyTokens,
};

export function makeTheme(mode: 'light' | 'dark'): Theme {
  const p = mode === 'dark' ? dark : light;
  const t = p.tokens;
  return createTheme({
    pinky: t,
    palette: {
      mode,
      primary:
        mode === 'dark'
          ? { main: p.ink, dark: '#e3e1d8', contrastText: '#1f1e1d' }
          : { main: p.ink, dark: '#3d3d3a', contrastText: '#ffffff' },
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
      // The system's own face, so Latin and Japanese sit together the way
      // they do in the rest of the desktop; Geist Mono only for figures.
      fontFamily:
        '-apple-system, BlinkMacSystemFont, "Hiragino Sans", "Hiragino Kaku Gothic ProN", "PingFang SC", "Segoe UI Variable", "Segoe UI", "Microsoft YaHei", system-ui, sans-serif',
      fontSize: 14,
      // One scale and three weights: 600 for headings only, 500 for labels
      // and controls, 400 for the rest.
      h1: { fontSize: 24, fontWeight: 600, lineHeight: 1.2 },
      h2: { fontSize: 18, fontWeight: 600, lineHeight: 1.3 },
      h3: { fontSize: 15, fontWeight: 600, lineHeight: 1.35 },
      subtitle1: { fontSize: 14, fontWeight: 500, lineHeight: 1.45 },
      subtitle2: { fontSize: 13, fontWeight: 500, color: t.ink2 },
      body1: { fontSize: 14, lineHeight: 1.5 },
      body2: { fontSize: 13, lineHeight: 1.55 },
      caption: { fontSize: 12, color: t.ink3, lineHeight: 1.5 },
      button: { textTransform: 'none', fontWeight: 500, fontSize: 13 },
    },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          'html, body, #root': { height: '100%' },
          // Tabular figures everywhere, so times, counts and durations line
          // up in their columns and don't shift as they tick.
          body: { overflow: 'hidden', WebkitFontSmoothing: 'antialiased', userSelect: 'none', fontVariantNumeric: 'tabular-nums' },
          'input, textarea, [contenteditable], .selectable': { userSelect: 'text' },
          '.mono': { fontFamily: t.mono, fontSize: '0.92em', letterSpacing: 0 },
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
        styleOverrides: {
          root: {
            borderRadius: 8,
            paddingInline: 12,
            minHeight: 32,
            whiteSpace: 'nowrap',
            transition: 'background-color .15s cubic-bezier(0.22, 1, 0.36, 1), border-color .15s, color .15s, transform .12s',
            '&:active': { transform: 'scale(0.98)' },
          },
          contained: { boxShadow: t.highlight },
          // Secondary buttons: a hairline and the surface behind them, so a
          // list of them stays quiet.
          outlined: { backgroundColor: 'transparent' },
          outlinedInherit: { borderColor: t.line2, '&:hover': { borderColor: t.line2, backgroundColor: alpha(p.ink, 0.06) } },
          outlinedPrimary: { borderColor: t.line2, '&:hover': { borderColor: t.line2, backgroundColor: alpha(p.ink, 0.06) } },
          textInherit: { color: t.ink2, '&:hover': { color: p.ink, backgroundColor: alpha(p.ink, 0.06) } },
        },
      },
      MuiIconButton: { defaultProps: { size: 'small' }, styleOverrides: { root: { borderRadius: 8, color: t.ink2 } } },
      MuiTooltip: { defaultProps: { arrow: false, enterDelay: 400 } },
      MuiPaper: {
        defaultProps: { elevation: 0 },
        styleOverrides: { root: { backgroundImage: 'none' }, outlined: { borderColor: t.line, boxShadow: t.cardShadow } },
      },
      MuiCard: {
        defaultProps: { variant: 'outlined' },
        styleOverrides: { root: { borderRadius: 14 } },
      },
      MuiTextField: { defaultProps: { size: 'small' } },
      MuiOutlinedInput: {
        styleOverrides: {
          root: { borderRadius: 8, backgroundColor: p.panel },
          notchedOutline: { borderColor: t.line2 },
        },
      },
      MuiSelect: { defaultProps: { size: 'small' } },
      MuiSwitch: { defaultProps: { size: 'small' } },
      MuiChip: { defaultProps: { size: 'small' }, styleOverrides: { root: { fontWeight: 500, borderRadius: 6 } } },
      MuiListItemButton: {
        styleOverrides: {
          root: {
            borderRadius: 8,
            transition: 'background-color .15s cubic-bezier(0.22, 1, 0.36, 1)',
            '&.Mui-selected': { background: t.sel, boxShadow: t.highlight },
            '&.Mui-selected:hover': { background: t.sel },
          },
        },
      },
      MuiDialog: { styleOverrides: { paper: { borderRadius: 16, boxShadow: t.floatShadow, backgroundImage: 'none' } } },
      MuiMenu: { styleOverrides: { paper: { borderRadius: 10, border: `1px solid ${t.line}`, boxShadow: t.floatShadow } } },
      MuiPopover: { styleOverrides: { paper: { backgroundImage: 'none' } } },
      MuiTab: { styleOverrides: { root: { minHeight: 40, textTransform: 'none', fontWeight: 500, fontSize: 13 } } },
      MuiTabs: { styleOverrides: { root: { minHeight: 40 }, indicator: { height: 2, borderRadius: 2 } } },
      // Notes inside a page are quiet: a neutral surface, and the icon
      // alone carries the color.
      MuiAlert: {
        styleOverrides: {
          root: { borderRadius: 10, fontSize: 13, lineHeight: 1.5, alignItems: 'center', padding: '4px 12px' },
          standardSuccess: { backgroundColor: alpha(p.ink, 0.05), color: p.ink, '& .MuiAlert-icon': { color: t.green } },
          standardInfo: { backgroundColor: alpha(p.ink, 0.05), color: p.ink, '& .MuiAlert-icon': { color: t.ink2 } },
          standardWarning: { backgroundColor: t.amberTint, color: p.ink, '& .MuiAlert-icon': { color: t.amberDot } },
          standardError: { backgroundColor: t.redTint, color: p.ink, '& .MuiAlert-icon': { color: t.red } },
        },
      },
      MuiLink: { styleOverrides: { root: { color: p.ink, textDecorationColor: t.line2, textUnderlineOffset: 3 } } },
      MuiBadge: { styleOverrides: { badge: { fontFamily: t.mono, fontWeight: 500, fontSize: 11 } } },
    },
  });
}
