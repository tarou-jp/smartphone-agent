// The client's words. The catalogs are the MyGo client's own
// (cmd/pinky/locales), so both clients say the same; words only the web
// client has live in ./web/*.ts, one file per area, each with en, ja and
// zh-Hans.

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';
import en from '@locales/en.json';
import ja from '@locales/ja.json';
import zh from '@locales/zh-Hans.json';
import { format, type Params } from './icu';

export type Locale = 'en' | 'ja' | 'zh-Hans';
export const locales: { id: Locale; name: string }[] = [
  { id: 'en', name: 'English' },
  { id: 'ja', name: '日本語' },
  { id: 'zh-Hans', name: '简体中文' },
];

type Catalog = Record<string, string>;
type WebModule = { default: Record<Locale, Catalog> };

function strings(c: Record<string, unknown>): Catalog {
  const out: Catalog = {};
  for (const [k, v] of Object.entries(c)) if (!k.startsWith('@') && typeof v === 'string') out[k] = v;
  return out;
}

const catalogs: Record<Locale, Catalog> = { en: strings(en), ja: strings(ja), 'zh-Hans': strings(zh) };
const web = import.meta.glob<WebModule>('./web/*.ts', { eager: true });
for (const mod of Object.values(web)) {
  for (const l of Object.keys(catalogs) as Locale[]) Object.assign(catalogs[l], mod.default[l] ?? {});
}

function systemLocale(): Locale {
  for (const l of navigator.languages ?? [navigator.language]) {
    const s = l.toLowerCase();
    if (s.startsWith('ja')) return 'ja';
    if (s.startsWith('zh')) return 'zh-Hans';
    if (s.startsWith('en')) return 'en';
  }
  return 'en';
}

const storeKey = 'pinky-locale';

function savedLocale(): Locale | null {
  try {
    const v = localStorage.getItem(storeKey);
    return v === 'en' || v === 'ja' || v === 'zh-Hans' ? v : null;
  } catch {
    return null;
  }
}

export type T = (key: string, params?: Params) => string;

interface I18n {
  locale: Locale;
  /** The choice in Settings: a locale, or null for the system's. */
  chosen: Locale | null;
  setLocale: (l: Locale | null) => void;
  t: T;
  has: (key: string) => boolean;
}

const Ctx = createContext<I18n | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [chosen, setChosen] = useState<Locale | null>(savedLocale);
  const locale = chosen ?? systemLocale();
  const setLocale = useCallback((l: Locale | null) => {
    try {
      if (l) localStorage.setItem(storeKey, l);
      else localStorage.removeItem(storeKey);
    } catch {
      // The choice lasts until the window closes.
    }
    setChosen(l);
  }, []);
  const value = useMemo<I18n>(() => {
    const cat = catalogs[locale];
    const t: T = (key, params) => {
      const src = cat[key] ?? catalogs.en[key];
      if (src === undefined) {
        if (import.meta.env.DEV) console.warn('i18n: missing', key);
        return key;
      }
      return format(src, params, locale);
    };
    return { locale, chosen, setLocale, t, has: (k) => k in cat || k in catalogs.en };
  }, [locale, chosen, setLocale]);
  document.documentElement.lang = locale;
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useI18n(): I18n {
  const v = useContext(Ctx);
  if (!v) throw new Error('useI18n outside I18nProvider');
  return v;
}

export function useT(): T {
  return useI18n().t;
}
