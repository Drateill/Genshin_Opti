package damage

// TransformativeLevelMultiplier is the game's per-character-level base
// damage constant used by transformative elemental reactions (overloaded,
// superconduct, electro-charged, swirl, burning, bloom/burgeon/hyperbloom,
// shattered), index = character level (0-90). Index 0 is unused (-1, no
// level-0 character exists) — always index with a level in [1,90].
//
// Copied verbatim from the game's own PlayerElementLevelCo data table, via
// the actively-maintained open-source project frzyc/genshin-optimizer
// (libs/gi/keymap/src/StatConstants.ts, transformativeReactionLevelMultipliers),
// fetched 2026-09-12. This is a stable game-design constant, not something
// genshin-db exposes or that changes with content updates.
var TransformativeLevelMultiplier = [91]float64{
	-1.0, // lvl 0 (unused)
	17.165606, 18.535048, 19.904854, 21.274902, 22.6454, 24.649612, 26.640642, 28.868587, 31.36768, 34.143345,
	37.201, 40.66, 44.446667, 48.56352, 53.74848, 59.081898, 64.420044, 69.72446, 75.12314, 80.58478,
	86.11203, 91.70374, 97.24463, 102.812645, 108.40956, 113.20169, 118.102905, 122.97932, 129.72733, 136.29291,
	142.67085, 149.02902, 155.41699, 161.8255, 169.10631, 176.51808, 184.07274, 191.70952, 199.55692, 207.38205,
	215.3989, 224.16566, 233.50217, 243.35057, 256.06308, 268.5435, 281.52606, 295.01364, 309.0672, 323.6016,
	336.75754, 350.5303, 364.4827, 378.61917, 398.6004, 416.39825, 434.387, 452.95105, 472.60623, 492.8849,
	513.56854, 539.1032, 565.51056, 592.53876, 624.4434, 651.47015, 679.4968, 707.79407, 736.67145, 765.64026,
	794.7734, 824.67737, 851.1578, 877.74207, 914.2291, 946.74677, 979.4114, 1011.223, 1044.7917, 1077.4437,
	1109.9976, 1142.9766, 1176.3695, 1210.1844, 1253.8357, 1288.9528, 1325.4841, 1363.4569, 1405.0974, 1446.8535,
}

// levelMultiplier clamps an arbitrary character level into [1,90] and looks
// up its transformative-reaction base damage constant.
func levelMultiplier(level int) float64 {
	if level < 1 {
		level = 1
	}
	if level > 90 {
		level = 90
	}
	return TransformativeLevelMultiplier[level]
}
