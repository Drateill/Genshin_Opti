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

// sumStat adds up one raw stat key across every piece's main stat + substats
// (e.g. every "atk_" contribution from the build's artifacts).
function sumStat(pieces: Artifact[], key: string): number {
  let v = 0;
  for (const p of pieces) {
    if (p.mainStatKey === key) v += p.mainStatValue;
    for (const s of p.substats) {
      if (s.key === key) v += s.value;
    }
  }
  return v;
}

// backOutAndReapplyPercent undoes then redoes the "base * (1 + pct/100) +
// flat" shape totals.atk/totals.hp are built from (see solver.go's
// totalsFor), without needing the actual base ATK/HP — only the two %
// sums (before and after whatever changed) and the build's own flat
// contribution. Used any time something feeding that %-multiplier changes
// outside of the artifact pieces themselves (a weapon's own %ATK/%HP
// substat, here) — pctCur/pctNext are the full known %-sum each side, flat
// is whatever stays constant throughout (the pieces' flat contribution).
function backOutAndReapplyPercent(totalCur: number, flat: number, pctCur: number, pctNext: number): number {
  const base = (totalCur - flat) / (1 + pctCur / 100);
  return base * (1 + pctNext / 100) + flat;
}

function checkValue(key: string, dmgKey: string, t: BuildTotals): number | undefined {
  if (key === 'critRate_') return t.critRate;
  if (key === 'critDMG_') return t.critDMG;
  if (key === 'em') return t.elementalMastery;
  if (key === 'enerRech_') return t.energyRecharge;
  if (key === 'atk') return t.atk;
  if (key === 'hp') return t.hp;
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
      const sumAtk_ = sumStat(build.pieces, 'atk_');
      const sumFlatAtk = sumStat(build.pieces, 'atk');
      totals.atk = backOutAndReapplyPercent(build.totals.atk, sumFlatAtk, sumAtk_, sumAtk_ + diff);
      break;
    }
    case 'hp':
      totals.hp += diff;
      break;
    case 'hp_': {
      // Same back-out-then-reapply trick as atk_ above, against totals.hp =
      // baseHP * (1 + sumHp_/100) + sumFlatHp.
      const sumHp_ = sumStat(build.pieces, 'hp_');
      const sumFlatHp = sumStat(build.pieces, 'hp');
      totals.hp = backOutAndReapplyPercent(build.totals.hp, sumFlatHp, sumHp_, sumHp_ + diff);
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
// flat ATK AND secondary stat (if any — e.g. a CRIT DMG% weapon like Beyond
// the Chrysalis) bumped from their current level to 90 (a weapon's max —
// see chardb.WeaponStatAt's linear level/90 approximation on the backend,
// which this mirrors to back out each level-90 value from the already-
// scaled numbers the API returned). weaponSubKey/weaponSubValue are the
// WeaponOption's current (already level-scaled) secondary stat, or
// undefined/0 for the rare weapon with none. Pieces and critValue are
// untouched since the weapon isn't part of the artifact build; only the
// totals field(s) the weapon actually feeds, and any constraint checks
// depending on them, change.
export function applyWeaponUpgrade(
  build: BuildResult,
  weaponAtk: number,
  weaponLevel: number,
  weaponSubKey: string | undefined,
  weaponSubValue: number | undefined,
  dmgKey: string
): BuildResult {
  if (weaponLevel >= 90 || weaponAtk <= 0) return build;
  const maxATK = (weaponAtk * 90) / weaponLevel;
  const diffATK = maxATK - weaponAtk;

  const subKey = weaponSubKey ?? '';
  const subCur = weaponSubValue ?? 0;
  const subMax = subKey ? (subCur * 90) / weaponLevel : 0;
  const diffSub = subMax - subCur;

  const totals: BuildTotals = { ...build.totals };
  const sumAtk_pieces = sumStat(build.pieces, 'atk_');
  if (subKey === 'atk_') {
    // The weapon's own %ATK substat is part of the same %-sum the flat ATK
    // bump multiplies against, so both must move together — back out
    // (base+weaponATK) using the %-sum as it stands today (pieces' atk_
    // plus the weapon's current atk_ substat), then reapply with the flat
    // weapon ATK and the %ATK substat both at their level-90 values.
    const sumFlatAtk = sumStat(build.pieces, 'atk');
    const baseAndWeaponCur = (build.totals.atk - sumFlatAtk) / (1 + (sumAtk_pieces + subCur) / 100);
    const baseAndWeapon90 = baseAndWeaponCur - weaponAtk + maxATK;
    totals.atk = baseAndWeapon90 * (1 + (sumAtk_pieces + subMax) / 100) + sumFlatAtk;
  } else {
    totals.atk = build.totals.atk + diffATK * (1 + sumAtk_pieces / 100);
    switch (subKey) {
      case 'critRate_':
        totals.critRate += diffSub;
        break;
      case 'critDMG_':
        totals.critDMG += diffSub;
        break;
      case 'em':
        totals.elementalMastery += diffSub;
        break;
      case 'enerRech_':
        totals.energyRecharge += diffSub;
        break;
      case 'hp_': {
        const sumHp_ = sumStat(build.pieces, 'hp_');
        const sumFlatHp = sumStat(build.pieces, 'hp');
        totals.hp = backOutAndReapplyPercent(build.totals.hp, sumFlatHp, sumHp_, sumHp_ + diffSub);
        break;
      }
      default:
        if (subKey === dmgKey) totals.elementalDMG += diffSub;
    }
  }

  const checks: ConstraintCheck[] = build.checks.map((c) => {
    const value = checkValue(c.key, dmgKey, totals) ?? c.value;
    const met = (c.min === undefined || value >= c.min) && (c.max === undefined || value <= c.max);
    return { ...c, value, met };
  });
  const allMet = checks.every((c) => c.met);

  return { ...build, totals, checks, allMet };
}
