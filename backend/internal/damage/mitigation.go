package damage

// DefMultiplier is the standard Genshin Impact defense-mitigation formula:
// the attacker's own level (offset +100) against the defender's DEF (also
// offset +100, reduced by any DEF-shred%). Enemy DEF is not tracked as its
// own stat here — the standard baseline approximation `enemyLevel+100` is
// used, which is exact for nearly every overworld enemy and very close for
// bosses (the small deviations bosses have aren't worth a separate curated
// DEF table given RES already needs one).
func DefMultiplier(casterLevel, enemyLevel int, defShredPct float64) float64 {
	enemyDEF := float64(enemyLevel+100) * (1 - defShredPct/100)
	if enemyDEF < 0 {
		enemyDEF = 0
	}
	return float64(casterLevel+100) / (float64(casterLevel+100) + enemyDEF)
}

// ResMultiplier is the standard piecewise elemental-resistance formula.
// resPct/resShredPct are in percent (e.g. 10 for 10%), res itself is
// converted to a fraction before the three RES bands are applied.
func ResMultiplier(resPct, resShredPct float64) float64 {
	res := (resPct - resShredPct) / 100
	switch {
	case res < 0:
		return 1 - res/2
	case res < 0.75:
		return 1 - res
	default:
		return 1 / (4*res + 1)
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
