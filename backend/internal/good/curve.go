package good

// maxMainStat5 gives each main stat's value at rarity 5, level 20 (an
// artifact's max) — these are the standard, stable Genshin Impact game
// values and are the same numbers the design prototype used for its mock
// inventory.
var maxMainStat5 = map[string]float64{
	"hp": 4780, "atk": 311, "hp_": 46.6, "atk_": 46.6, "def_": 58.3,
	"em": 187, "enerRech_": 51.8, "critRate_": 31.1, "critDMG_": 62.2, "heal_": 35.9,
	"pyro_dmg_": 46.6, "hydro_dmg_": 46.6, "cryo_dmg_": 46.6, "electro_dmg_": 46.6,
	"anemo_dmg_": 46.6, "geo_dmg_": 46.6, "dendro_dmg_": 46.6, "physical_dmg_": 46.6,
}

// MainStatValue estimates a main stat's value from its key, rarity and
// level, for GOOD exports that omit the precomputed value (the format
// derives it deterministically from in-game tables, which not every export
// tool inlines). This is a deliberately simple linear approximation of the
// real per-level growth curve and of the rarity 3/4 sub-tables — it is only
// used as a fallback when an export doesn't already carry the value, and it
// is close enough for ranking builds. Exact per-level game tables are out of
// scope for V1 (see the spec's V2 notes on importing a full reference DB).
func MainStatValue(statKey string, rarity, level int) float64 {
	max5, ok := maxMainStat5[statKey]
	if !ok {
		return 0
	}
	if rarity <= 0 {
		rarity = 5
	}
	if level < 0 {
		level = 0
	}
	if level > 20 {
		level = 20
	}
	rarityFrac := float64(rarity) / 5
	levelFrac := float64(level) / 20
	return max5 * rarityFrac * levelFrac
}
