// Generates characters.json, weapons.json, and sets.json for the Go
// backend from genshin-db (https://github.com/theBowja/genshin-db), which
// tracks the live game's official data. Run with `node generate.js` from
// this directory whenever genshin-db is updated to a newer game version.
//
// Each entry carries its display text (name / element / short / description)
// as a { en, fr } map instead of a single string, so the Go backend can pick
// a locale at request time (see internal/chardb's NameFor/ElementFor/etc.).
// The GOOD `key` is always derived from the ENGLISH name — GOOD keys are a
// fixed English identifier shared by every GOOD-compatible tool regardless
// of the player's UI language — and genshin-db's stable per-entry `id` is
// what lets the English and French queries be matched back up to each other
// (their `name` fields obviously differ, but `id` doesn't).
const fs = require('fs');
const path = require('path');
const db = require('genshin-db');

// cleanName strips the stray wrapping quote marks some weapon names carry
// in genshin-db's data (e.g. `"The Catch"`, literal quotes and all).
function cleanName(name) {
  return name.replace(/["“”]/g, '').trim();
}

function goodKey(name) {
  return cleanName(name)
    .split(/[\s-]+/) // split on spaces AND hyphens (e.g. "Ash-Graven Drinking Horn")
    .filter(Boolean)
    .map((w) => (w[0] ? w[0].toUpperCase() + w.slice(1) : w))
    .join('')
    .replace(/['’]/g, '');
}

// byId indexes a genshin-db result list by its stable `id` field, so a
// same-language-independent lookup can pull the matching entry out of a
// different-language query of the same list.
function byId(list) {
  const m = new Map();
  for (const x of list) m.set(x.id, x);
  return m;
}

function shortCodeFromName(name) {
  return name
    .split(/\s+/)
    .filter((w) => /^[A-Za-zÀ-ÿ]/.test(w))
    .map((w) => w[0].toUpperCase())
    .slice(0, 2)
    .join('');
}

// specializedValue's raw number from genshin-db is a 0-1 fraction for every
// percentage-based stat (e.g. 0.613 -> 61.3%), but Elemental Mastery is a
// flat number already in its real units (e.g. 96, not 0.96) — it has no
// "percent" to begin with.
function scaleSpecialized(key, raw) {
  if (typeof raw !== 'number') return 0;
  return key === 'em' ? Math.round(raw * 10) / 10 : Math.round(raw * 1000) / 10;
}

// genshin-db's calcStatsCharacter (see node_modules/genshin-db/src/getdata.js)
// bakes the universal baseline into `specialized` ONLY for the two crit
// stats: when a character's ascension stat is CRITICAL or CRITICAL_HURT, the
// raw value already includes the base 5% CRIT Rate / 50% CRIT DMG every
// character has innately — every other stat type (EM, ATK%, ER%, elemental
// DMG%, ...) is a pure ascension increment with no baseline folded in.
// chardb.go's solver already adds those two baselines once via
// DefaultCritRate/DefaultCritDMG, so this must strip them back out here or
// a build's CRIT DMG comes out 50 points too high (that's the exact bug
// this comment is guarding against — don't remove it without re-checking
// the totals math in internal/solver/solver.go).
const CHAR_BASELINE_INCLUDED = { critRate_: 5, critDMG_: 50 };

function characterSpecializedValue(key, raw) {
  const v = scaleSpecialized(key, raw);
  const baseline = CHAR_BASELINE_INCLUDED[key];
  return baseline ? Math.round((v - baseline) * 10) / 10 : v;
}

const WEAPON_TYPE = {
  WEAPON_SWORD_ONE_HAND: 'sword',
  WEAPON_CLAYMORE: 'claymore',
  WEAPON_POLE: 'polearm',
  WEAPON_BOW: 'bow',
  WEAPON_CATALYST: 'catalyst',
};

const STAT_KEY = {
  FIGHT_PROP_CRITICAL: 'critRate_',
  FIGHT_PROP_CRITICAL_HURT: 'critDMG_',
  FIGHT_PROP_ATTACK_PERCENT: 'atk_',
  FIGHT_PROP_HP_PERCENT: 'hp_',
  FIGHT_PROP_DEFENSE_PERCENT: 'def_',
  FIGHT_PROP_ELEMENT_MASTERY: 'em',
  FIGHT_PROP_CHARGE_EFFICIENCY: 'enerRech_',
  FIGHT_PROP_HEAL_ADD: 'heal_',
  FIGHT_PROP_PHYSICAL_ADD_HURT: 'physical_dmg_',
  FIGHT_PROP_FIRE_ADD_HURT: 'pyro_dmg_',
  FIGHT_PROP_WATER_ADD_HURT: 'hydro_dmg_',
  FIGHT_PROP_ICE_ADD_HURT: 'cryo_dmg_',
  FIGHT_PROP_ELEC_ADD_HURT: 'electro_dmg_',
  FIGHT_PROP_WIND_ADD_HURT: 'anemo_dmg_',
  FIGHT_PROP_ROCK_ADD_HURT: 'geo_dmg_',
  FIGHT_PROP_GRASS_ADD_HURT: 'dendro_dmg_',
};

const DMG_KEY_BY_ELEMENT = {
  ELEMENT_PYRO: 'pyro_dmg_',
  ELEMENT_HYDRO: 'hydro_dmg_',
  ELEMENT_CRYO: 'cryo_dmg_',
  ELEMENT_ELECTRO: 'electro_dmg_',
  ELEMENT_ANEMO: 'anemo_dmg_',
  ELEMENT_GEO: 'geo_dmg_',
  ELEMENT_DENDRO: 'dendro_dmg_',
};

// genshin-db's talent attribute labels are structured as "Name|{paramN:FMT}"
// (sometimes several "{paramN:FMT}" tokens sharing one name, e.g. combo hits
// or Low/High variants). FMT ending in "P" (F1P, F2P, P) is a percentage —
// almost always %ATK scaling for a labeled "DMG" line — anything else (F1,
// I) is a plain number (CD seconds, energy cost, stamina, duration) we don't
// want. A talent's own passive/heal/shield lines never mention "DMG" in
// their name (verified against Bennett's HP-regen and Zhongli's shield
// lines), so filtering on that alone is enough to isolate damage instances
// without a hand-curated per-character list. This detection only works
// reliably in ENGLISH — French spells it "DGT" (most of the time, but not
// always: e.g. Kokomi's burst calls a DMG-bonus param "Bonus ATQ normale"
// with no "DGT" at all) — so English is the sole source of truth for which
// params are damage instances; French only supplies display text for
// whichever param numbers English already picked (see paramNamesFromLabels).
const TALENT_LABEL_RE = /^(.*?)\|(.*)$/;
const TALENT_PARAM_RE = /\{param(\d+):([A-Za-z0-9]+)\}/g;
const isPercentFormat = (fmt) => /P$/.test(fmt);

// namesForTokens turns one label's name part ("Low/High Plunge DMG") and its
// param count into one display name per param. When the "/"-separated
// segments line up 1:1 with the params, each segment gets a name of its
// own — backfilling any segment that itself doesn't say "DMG" with the
// trailing words of a segment that does (so "Low" becomes "Low Plunge DMG"
// instead of losing the shared suffix). Otherwise (multiple params sharing
// one name, e.g. a 4th hit split into two damage instances) the params are
// just numbered.
function namesForTokens(namePart, tokenCount) {
  if (tokenCount <= 1) return [namePart];
  const segs = namePart.split('/').map((s) => s.trim());
  if (segs.length === tokenCount) {
    const ref = segs.find((s) => /dmg/i.test(s)) || segs[segs.length - 1];
    const suffix = ref.split(/\s+/).slice(1).join(' ');
    return segs.map((s) => (/dmg/i.test(s) ? s : suffix ? `${s} ${suffix}`.trim() : s));
  }
  return segs.length === 1 ? Array.from({ length: tokenCount }, (_, i) => `${namePart} #${i + 1}`) : [namePart];
}

// paramNamesFromLabels maps every paramN token found in a talent's labels to
// its display name in whatever language those labels are in, with no "is
// this damage" filtering at all — used to look up a display name for a
// param number that English already decided is a damage instance.
function paramNamesFromLabels(labels) {
  const map = new Map();
  for (const raw of labels ?? []) {
    const m = TALENT_LABEL_RE.exec(raw);
    if (!m) continue;
    const [, namePart, expr] = m;
    const tokens = [...expr.matchAll(TALENT_PARAM_RE)];
    if (tokens.length === 0) continue;
    const names = namesForTokens(namePart, tokens.length);
    tokens.forEach(([, paramNum], i) => map.set(paramNum, names[i] ?? namePart));
  }
  return map;
}

// extractDmgMultipliers pulls every %ATK damage-multiplier curve (15 values,
// one per talent level 1-15) out of one combat talent's `attributes`,
// deciding which params qualify from the ENGLISH labels/format-codes, then
// attaching a French display name for the same param number (falling back
// to the English name if French has nothing at that param).
function extractDmgMultipliers(enLabels, parameters, frLabels) {
  const frNames = paramNamesFromLabels(frLabels);
  const out = [];
  for (const raw of enLabels ?? []) {
    const m = TALENT_LABEL_RE.exec(raw);
    if (!m) continue;
    const [, namePart, expr] = m;
    if (!/dmg/i.test(namePart)) continue;
    const tokens = [...expr.matchAll(TALENT_PARAM_RE)];
    if (tokens.length === 0) continue;
    const names = namesForTokens(namePart, tokens.length);
    tokens.forEach(([, paramNum, fmt], i) => {
      if (!isPercentFormat(fmt)) return;
      const values = parameters['param' + paramNum];
      if (!Array.isArray(values) || values.length === 0) return;
      const enName = names[i] ?? namePart;
      out.push({ label: { en: enName, fr: frNames.get(paramNum) ?? enName }, values: values.map((v) => Math.round(v * 1000) / 10) });
    });
  }
  return out;
}

// talentsFor looks up a character's normal-attack/skill/burst talents by
// name (matches cleanly for all 118 non-Traveler characters as of this
// writing) and extracts their damage multipliers. combatsp/combatju
// (Mona/Ayaka's alt-dash, Ororon's alt-form) are skipped: they don't map
// cleanly onto GOOD's 3-field auto/skill/burst talent shape.
function talentsFor(c) {
  try {
    const en = db.talents(c.name, { matchNames: true, resultLanguage: 'English' });
    const fr = db.talents(c.name, { matchNames: true, resultLanguage: 'French' });
    const group = (comboEn, comboFr) =>
      extractDmgMultipliers(comboEn?.attributes?.labels, comboEn?.attributes?.parameters, comboFr?.attributes?.labels);
    return { auto: group(en.combat1, fr.combat1), skill: group(en.combat2, fr.combat2), burst: group(en.combat3, fr.combat3) };
  } catch (e) {
    console.warn('skip talents (no match):', c.name, e.message);
    return { auto: [], skill: [], burst: [] };
  }
}

// --- characters ---
const rawCharsEn = db.characters('names', { matchCategories: true, verboseCategories: true, resultLanguage: 'English' });
const rawCharsFr = byId(db.characters('names', { matchCategories: true, verboseCategories: true, resultLanguage: 'French' }));
const characters = [];
for (const c of rawCharsEn) {
  if (c.elementType === 'ELEMENT_NONE') continue; // Traveler pre-element-choice entries; ambiguous GOOD key, skip
  const weaponType = WEAPON_TYPE[c.weaponType];
  const dmgKey = DMG_KEY_BY_ELEMENT[c.elementType];
  const specializedKey = STAT_KEY[c.substatType];
  if (!weaponType || !dmgKey || !specializedKey) {
    console.warn('skip character (unmapped enum):', c.name, c.weaponType, c.elementType, c.substatType);
    continue;
  }
  const stats = c.stats(90, 6);
  if (!stats) {
    console.warn('skip character (no stats):', c.name);
    continue;
  }
  const fr = rawCharsFr.get(c.id);
  characters.push({
    key: goodKey(c.name),
    name: { en: cleanName(c.name), fr: cleanName(fr ? fr.name : c.name) },
    element: { en: c.elementText, fr: fr ? fr.elementText : c.elementText },
    weaponType,
    rarity: c.rarity,
    dmgKey,
    specializedKey,
    specializedValue: characterSpecializedValue(specializedKey, stats.specialized),
    maxATK: Math.round(stats.attack * 10) / 10,
    maxHP: Math.round(stats.hp * 10) / 10,
    talents: talentsFor(c),
    // hoyowiki_icon resolves reliably for every character, old and new
    // alike (spot-checked); mihoyo_icon's game_record CDN path 404s for a
    // large share of recent (5.x-era) characters — it's seemingly only
    // populated for characters that have shown up in cached player
    // showcases, not a general asset host.
    icon: c.images?.hoyowiki_icon || '',
  });
}

// --- weapons ---
const rawWeaponsEn = db.weapons('names', { matchCategories: true, verboseCategories: true, resultLanguage: 'English' });
const rawWeaponsFr = byId(db.weapons('names', { matchCategories: true, verboseCategories: true, resultLanguage: 'French' }));
const weapons = [];
for (const w of rawWeaponsEn) {
  const type = WEAPON_TYPE[w.weaponType];
  if (!type) {
    console.warn('skip weapon (unmapped type):', w.name, w.weaponType);
    continue;
  }
  const stats = typeof w.stats === 'function' ? w.stats(90, 6) : undefined;
  if (!stats) {
    console.warn('skip weapon (no stats):', w.name);
    continue;
  }
  const subStatKey = STAT_KEY[w.mainStatType] || '';
  const fr = rawWeaponsFr.get(w.id);
  weapons.push({
    key: goodKey(w.name),
    name: { en: cleanName(w.name), fr: cleanName(fr ? fr.name : w.name) },
    type,
    rarity: w.rarity,
    subStatKey,
    maxATK: Math.round(stats.attack * 10) / 10,
    maxSubStatValue: subStatKey ? scaleSpecialized(subStatKey, stats.specialized) : 0,
    // Unlike characters (see hoyowiki_icon above), genshin-db has no
    // hoyowiki-hosted icon for weapons — mihoyo_icon's game_record CDN is
    // the only option, and it 404s for a large share of weapons (roughly
    // half, in spot checks, unrelated to rarity or release date) since it
    // only seems to cache icons for items that show up in cached player
    // showcases. The frontend's <img onError> falls back to a plain
    // rarity badge when that happens — this is a known, accepted gap, not
    // a bug. A handful of weapons (e.g. "Prized Isshin Blade") only carry
    // an awakened-form icon in genshin-db's data, not a base one.
    icon: w.images?.mihoyo_icon || w.images?.mihoyo_awakenIcon || '',
  });
}

// --- artifact sets ---
const rawSetsEn = db.artifacts('names', { matchCategories: true, verboseCategories: true, resultLanguage: 'English' });
const rawSetsFr = byId(db.artifacts('names', { matchCategories: true, verboseCategories: true, resultLanguage: 'French' }));
const sets = [];
for (const s of rawSetsEn) {
  if (!s.effect2Pc) continue; // a handful of non-set "relic" entries have no set bonus text
  const fr = rawSetsFr.get(s.id);
  const nameEn = cleanName(s.name);
  const nameFr = cleanName(fr ? fr.name : s.name);
  const e2En = s.effect2Pc.trim().replace(/([^.!])$/, '$1.');
  let descEn = '2pc: ' + e2En;
  if (s.effect4Pc) descEn += ' 4pc: ' + s.effect4Pc.trim();
  let descFr = descEn;
  if (fr && fr.effect2Pc) {
    const e2Fr = fr.effect2Pc.trim().replace(/([^.!])$/, '$1.');
    descFr = '2 pièces : ' + e2Fr;
    if (fr.effect4Pc) descFr += ' 4 pièces : ' + fr.effect4Pc.trim();
  }
  sets.push({
    key: goodKey(s.name),
    name: { en: nameEn, fr: nameFr },
    short: { en: shortCodeFromName(nameEn), fr: shortCodeFromName(nameFr) },
    description: { en: descEn, fr: descFr },
    // Sets have no single "icon" in genshin-db, only one per equipment slot
    // — use whichever slot the set actually has, in a fixed preference
    // order (a handful of sets skip flower/plume, e.g. single-piece ones).
    // Same game_record-CDN reliability caveat as weapons applies here (no
    // hoyowiki-hosted alternative exists for artifact icons either) — the
    // frontend falls back to the plain short-code badge on a 404.
    icon:
      s.images?.mihoyo_flower ||
      s.images?.mihoyo_circlet ||
      s.images?.mihoyo_sands ||
      s.images?.mihoyo_goblet ||
      s.images?.mihoyo_plume ||
      '',
  });
}

const outDir = path.join(__dirname, '..', '..', 'backend', 'internal', 'chardb', 'data');
fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(path.join(outDir, 'characters.json'), JSON.stringify(characters, null, 2));
fs.writeFileSync(path.join(outDir, 'weapons.json'), JSON.stringify(weapons, null, 2));
fs.writeFileSync(path.join(outDir, 'sets.json'), JSON.stringify(sets, null, 2));

console.log(`wrote ${characters.length} characters, ${weapons.length} weapons, ${sets.length} sets to ${outDir}`);
