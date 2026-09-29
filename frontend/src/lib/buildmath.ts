// Client-side "what if this piece were level 20" preview math for the
// Results screen's "to 20" button. Mirrors backend/internal/good/curve.go's
// maxMainStat5 table (the same simple linear approximation the app already
// relies on for main-stat values) — keep the two in sync if it ever changes.
//
// Only the piece's main stat is touched; substats are left exactly as
// imported, since how they'd actually roll on level-up isn't knowable ahead
// of time. Totals/checks/critValue are recomputed from the delta on that one
// stat rather than re-deriving the whole build, which keeps this in sync
// with solver.go's math without duplicating it wholesale.
import type { Artifact, BuildResult, BuildTotals, ConstraintCheck } from '../api/types';

const MAX_MAIN_STAT_5: Record<string, number> = {
  hp: 4780, atk: 311, hp_: 46.6, atk_: 46.6, def_: 58.3,
  em: 187, enerRech_: 51.8, critRate_: 31.1, critDMG_: 62.2, heal_: 35.9,
  pyro_dmg_: 46.6, hydro_dmg_: 46.6, cryo_dmg_: 46.6, electro_dmg_: 46.6,
  anemo_dmg_: 46.6, geo_dmg_: 46.6, dendro_dmg_: 46.6, physical_dmg_: 46.6,
};

// mainStatAtLevel20 returns undefined for a stat key it doesn't recognize.
// Rarity 5 is the only rarity that actually reaches level 20 in-game (4★
// caps at 16, 3★ at 12, ...) — callers should only offer the "to 20" action
// for rarity-5 pieces below level 20.
export function mainStatAtLevel20(key: string, rarity: number): number | undefined {
  const max5 = MAX_MAIN_STAT_5[key];
  if (max5 === undefined) return undefined;
  return (max5 * rarity) / 5;
}

function checkValue(key: string, dmgKey: string, t: BuildTotals): number | undefined {
  if (key === 'critRate_') return t.critRate;
  if (key === 'critDMG_') return t.critDMG;
  if (key === 'em') return t.elementalMastery;
  if (key === 'enerRech_') return t.energyRecharge;
  if (key === 'atk') return t.atk;
  if (key === dmgKey) return t.elementalDMG;
  return undefined;
}

// applyMainStatUpgrade returns a new BuildResult with one slot's piece
// leveled to 20 (main stat only) and every derived number — totals,
// critValue, constraint checks, allMet — updated to match. dmgKey is the
// solving character's elemental DMG stat key (RosterEntry.dmgKey), needed to
// know which totals field a goblet's elemental-DMG main stat feeds.
export function applyMainStatUpgrade(build: BuildResult, slot: string, dmgKey: string): BuildResult {
  const idx = build.pieces.findIndex((p) => p.slotKey === slot);
  if (idx === -1) return build;
  const piece = build.pieces[idx];
  const newVal = mainStatAtLevel20(piece.mainStatKey, piece.rarity);
  if (newVal === undefined || piece.level >= 20) return build;

  const diff = newVal - piece.mainStatValue;
  const pieces: Artifact[] = build.pieces.map((p, i) =>
    i === idx ? { ...p, level: 20, mainStatValue: newVal } : p
  );

  const totals: BuildTotals = { ...build.totals };
  switch (piece.mainStatKey) {
    case 'critRate_':
      totals.critRate += diff;
      break;
    case 'critDMG_':
      totals.critDMG += diff;
      break;
    case 'em':
      totals.elementalMastery += diff;
      break;
    case 'enerRech_':
      totals.energyRecharge += diff;
      break;
    case 'atk':
      totals.atk += diff;
      break;
    case 'atk_': {
      // totals.atk = (base+weapon) * (1 + sumAtk_/100) + sumFlatAtk. Back out
      // (base+weapon) from the build's own pieces (no character/weapon
      // context needed here), then re-apply with the bumped atk_%.
      let sumAtk_ = 0;
      let sumFlatAtk = 0;
      for (const p of build.pieces) {
        if (p.mainStatKey === 'atk_') sumAtk_ += p.mainStatValue;
        if (p.mainStatKey === 'atk') sumFlatAtk += p.mainStatValue;
        for (const s of p.substats) {
          if (s.key === 'atk_') sumAtk_ += s.value;
          if (s.key === 'atk') sumFlatAtk += s.value;
        }
      }
      const baseAndWeapon = (build.totals.atk - sumFlatAtk) / (1 + sumAtk_ / 100);
      totals.atk = baseAndWeapon * (1 + (sumAtk_ + diff) / 100) + sumFlatAtk;
      break;
    }
    default:
      if (piece.mainStatKey === dmgKey) totals.elementalDMG += diff;
  }

  let critValue = build.critValue;
  if (slot === 'circlet' && (piece.mainStatKey === 'critRate_' || piece.mainStatKey === 'critDMG_')) {
    critValue += diff * (piece.mainStatKey === 'critRate_' ? 2 : 1);
  }

  const checks: ConstraintCheck[] = build.checks.map((c) => {
    const value = checkValue(c.key, dmgKey, totals) ?? c.value;
    const met = (c.min === undefined || value >= c.min) && (c.max === undefined || value <= c.max);
    return { ...c, value, met };
  });
  const allMet = checks.every((c) => c.met);

  return { ...build, pieces, totals, critValue, checks, allMet };
}

// applyWeaponUpgrade returns a new BuildResult with the equipped weapon's
// flat ATK contribution bumped from its current level to 90 (a weapon's
// max — see chardb.WeaponStatAt's linear level/90 approximation on the
// backend, which this mirrors to back out the level-90 value from the
// already-scaled ATK the API returned). Only totals.atk and any constraint
// checks that depend on it change; pieces, critValue and every other stat
// are untouched since the weapon isn't part of the artifact build.
export function applyWeaponUpgrade(
  build: BuildResult,
  weaponAtk: number,
  weaponLevel: number,
  dmgKey: string
): BuildResult {
  if (weaponLevel >= 90 || weaponAtk <= 0) return build;
  const maxATK = (weaponAtk * 90) / weaponLevel;
  const diff = maxATK - weaponAtk;

  // totals.atk = (baseATK + weaponATK) * (1 + sumAtk_/100) + sumFlatAtk, so
  // bumping weaponATK by diff adds diff * (1 + sumAtk_/100) to the total —
  // sumAtk_ is the same "%ATK across every piece" sum backend/solver.go uses.
  let sumAtk_ = 0;
  for (const p of build.pieces) {
    if (p.mainStatKey === 'atk_') sumAtk_ += p.mainStatValue;
    for (const s of p.substats) {
      if (s.key === 'atk_') sumAtk_ += s.value;
    }
  }

  const totals: BuildTotals = { ...build.totals, atk: build.totals.atk + diff * (1 + sumAtk_ / 100) };

  const checks: ConstraintCheck[] = build.checks.map((c) => {
    const value = checkValue(c.key, dmgKey, totals) ?? c.value;
    const met = (c.min === undefined || value >= c.min) && (c.max === undefined || value <= c.max);
    return { ...c, value, met };
  });
  const allMet = checks.every((c) => c.met);

  return { ...build, totals, checks, allMet };
}
