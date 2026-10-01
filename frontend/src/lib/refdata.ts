// Static UI vocabulary — the fixed slot main-stat choices and minimum-
// constraint presets the Configure screen offers, plus small label lookups.
// This mirrors the prototype's static defs. Unlike the prototype, which set
// of target sets is offered comes from the real imported inventory (see
// api.sets), not a hardcoded list — any set key a GOOD export contains can
// be targeted.
//
// The actual label text lives in i18n/translations.ts (t.slots / t.stats) —
// everything here is locale-agnostic shape (which keys exist, in what
// order) parameterized by the caller's translation dict.
import type { Dict } from '../i18n/translations';

export const SLOT_TAG: Record<string, string> = {
  flower: 'FL',
  plume: 'PL',
  sands: 'SA',
  goblet: 'GO',
  circlet: 'CI',
};

export const chardb = {
  pctStats: new Set([
    'critRate_', 'critDMG_', 'atk_', 'hp_', 'def_', 'enerRech_', 'heal_',
    'pyro_dmg_', 'hydro_dmg_', 'cryo_dmg_', 'electro_dmg_', 'anemo_dmg_',
    'geo_dmg_', 'dendro_dmg_', 'physical_dmg_',
  ]),
};

function abbrev(t: Dict, key: string): string {
  return (t.statsAbbrev as Record<string, string>)[key] ?? t.stats[key as keyof Dict['stats']] ?? key;
}

export function sandsOpts(t: Dict): [string, string][] {
  return [
    ['em', abbrev(t, 'em')],
    ['atk_', abbrev(t, 'atk_')],
    ['enerRech_', abbrev(t, 'enerRech_')],
    ['hp_', abbrev(t, 'hp_')],
  ];
}

export function circletOpts(t: Dict): [string, string][] {
  return [
    ['critRate_', abbrev(t, 'critRate_')],
    ['critDMG_', abbrev(t, 'critDMG_')],
    ['atk_', abbrev(t, 'atk_')],
    ['em', abbrev(t, 'em')],
    ['hp_', abbrev(t, 'hp_')],
  ];
}

export function gobletOpts(t: Dict, dmgKey: string): [string, string][] {
  return [
    [dmgKey, t.stats[dmgKey as keyof Dict['stats']] ?? dmgKey],
    ['atk_', abbrev(t, 'atk_')],
    ['em', abbrev(t, 'em')],
    ['hp_', abbrev(t, 'hp_')],
  ];
}

export interface ConstraintStatOpt {
  key: string;
  label: string;
  pct: boolean;
}

// The fixed (non-elemental-DMG) stats a min/max range can be set on.
// Elemental DMG is added separately in ConfigureView since its key/label
// depend on the selected character.
export function constraintStats(t: Dict): ConstraintStatOpt[] {
  return [
    { key: 'critRate_', label: t.stats.critRate_, pct: true },
    { key: 'critDMG_', label: t.stats.critDMG_, pct: true },
    { key: 'enerRech_', label: t.stats.enerRech_, pct: true },
    { key: 'em', label: t.stats.em, pct: false },
    { key: 'atk', label: t.stats.atk, pct: false },
  ];
}

export const TOPN_OPTS = [3, 5, 8, 10];

export const ELEMENTS = ['Pyro', 'Hydro', 'Anemo', 'Electro', 'Dendro', 'Cryo', 'Geo'] as const;
export type Element = (typeof ELEMENTS)[number];

// primary = this resonance feeds a stat the backend solver actually applies
// to build totals (see backend/internal/solver resonanceBonus) — the rest
// are shown for awareness only, since this app doesn't model shields,
// stamina, or per-enemy elemental-affliction state.
export const RESONANCE_PRIMARY: Record<Element, boolean> = {
  Pyro: true, Electro: true, Cryo: true, Dendro: true,
  Hydro: false, Anemo: false, Geo: false,
};

export interface ActiveResonance {
  element: Element;
  primary: boolean;
}

// OKLCH "L C H" triples (no oklch()/alpha wrapper, same shape as the
// --acc/--acc-lt custom properties set in app.css) — one hue per element,
// used to color element tiles and the active-resonance box so each
// resonance reads at a glance instead of every element looking identical.
export const ELEM_COLOR: Record<Element, string> = {
  Pyro: '0.72 0.18 38',
  Hydro: '0.7 0.14 240',
  Anemo: '0.8 0.12 170',
  Electro: '0.7 0.17 300',
  Dendro: '0.78 0.17 135',
  Cryo: '0.84 0.08 215',
  Geo: '0.8 0.14 85',
};

// activeResonances mirrors the real game's elemental resonance trigger: 2+
// of the 4 party members (the optimized character's own element plus up to
// 3 teammates) sharing an element. Sorted primary resonances first.
export function activeResonances(charElement: string, team: (string | null | undefined)[]): ActiveResonance[] {
  const counts: Record<string, number> = {};
  if (charElement) counts[charElement] = (counts[charElement] ?? 0) + 1;
  for (const el of team) if (el) counts[el] = (counts[el] ?? 0) + 1;
  return (ELEMENTS as readonly string[])
    .filter((el) => (counts[el] ?? 0) >= 2)
    .map((el) => ({ element: el as Element, primary: RESONANCE_PRIMARY[el as Element] }))
    .sort((a, b) => Number(b.primary) - Number(a.primary));
}

export const ACCENTS = ['orange', 'teal', 'purple', 'green'] as const;
export type Accent = (typeof ACCENTS)[number];
