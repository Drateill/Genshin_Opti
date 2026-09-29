import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';
import { en, fr, type Dict } from './translations';

export type Lang = 'en' | 'fr';
export const LANGS: Lang[] = ['en', 'fr'];

const DICTS: Record<Lang, Dict> = { en, fr };

// The API tags a handful of server-generated names/descriptions with the
// same two-letter code, so this is also what gets sent as ?lang=.
export function localeTag(lang: Lang): string {
  return lang === 'fr' ? 'fr-FR' : 'en-US';
}

interface LanguageContextValue {
  lang: Lang;
  setLang: (l: Lang) => void;
  t: Dict;
}

const LanguageContext = createContext<LanguageContextValue | null>(null);

export function LanguageProvider(props: { initialLang?: Lang; onChange?: (l: Lang) => void; children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(props.initialLang ?? 'en');
  const setLang = (l: Lang) => {
    setLangState(l);
    props.onChange?.(l);
  };
  const value = useMemo<LanguageContextValue>(() => ({ lang, setLang, t: DICTS[lang] }), [lang]);
  return <LanguageContext.Provider value={value}>{props.children}</LanguageContext.Provider>;
}

export function useLanguage(): LanguageContextValue {
  const ctx = useContext(LanguageContext);
  if (!ctx) throw new Error('useLanguage must be used within a LanguageProvider');
  return ctx;
}

// Convenience for components that only need the translation dictionary.
export function useT(): Dict {
  return useLanguage().t;
}
