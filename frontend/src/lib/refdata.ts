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

export const ACCENTS = ['orange', 'teal', 'purple', 'green'] as const;
export type Accent = (typeof ACCENTS)[number];
