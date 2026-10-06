// Package insights computes an account-wide audit of the current GOOD
// import — per-character "investment" scores, element/weapon-type/set
// distributions, and idle gear — independent of the solver. It's the
// backing logic for GET /api/insights.
package insights

import (
	"math"
	"sort"

	"artifact-optimizer/internal/chardb"
	"artifact-optimizer/internal/model"
)

// Compute builds the full insights response for one parsed GOOD export.
func Compute(export model.GoodExport, lang string) model.InsightsResponse {
	weaponByLoc := map[string]model.Weapon{}
	for _, w := range export.Weapons {
		if w.Location != "" {
			weaponByLoc[w.Location] = w
		}
	}
	artifactsByLoc := map[string][]model.Artifact{}
	for _, a := range export.Artifacts {
		if a.Location != "" {
			artifactsByLoc[a.Location] = append(artifactsByLoc[a.Location], a)
		}
	}

	characters := make([]model.CharacterInsight, 0, len(export.Characters))
	elementCounts := map[string]*model.ElementCount{}
	var investmentSum float64

	for _, c := range export.Characters {
		ref, known := chardb.Chars[c.Key]
		avgTalent := float64(c.Talent.Auto+c.Talent.Skill+c.Talent.Burst) / 3

		ci := model.CharacterInsight{
			Key: c.Key, Level: c.Level, Constellation: c.Constellation,
			AvgTalent: round1(avgTalent), Known: known,
		}
		if known {
			ci.Name = ref.NameFor(lang)
			ci.Element = ref.Element["en"]
			ci.ElementLabel = ref.ElementFor(lang)
			ci.WeaponType = ref.WeaponType
			ci.Rarity = ref.Rarity
		} else {
			ci.Name = chardb.SpaceOutKey(c.Key)
		}

		weaponScore := 0.0
		if w, ok := weaponByLoc[c.Key]; ok {
			refineCap := 5.0
			if wref, known := chardb.Weapons[w.Key]; known {
				ci.WeaponName = wref.NameFor(lang)
				ci.WeaponRarity = wref.Rarity
				if wref.Rarity == 5 {
					refineCap = 1
				}
			} else {
				ci.WeaponName = chardb.SpaceOutKey(w.Key)
			}
			ci.WeaponRefinement = w.Refinement
			ci.WeaponLevel = w.Level
			// Refinement is capped by weapon rarity, same reasoning as
			// constellationCap: a 5-star (signature or standard-banner) is
			// rare to duplicate, so R1 already counts as "complete" — only
			// a cheaply-duplicated 4-star/3-star needs R5 for full credit.
			weaponScore = float64(w.Level)/90*0.5 + math.Min(float64(w.Refinement)/refineCap, 1)*0.5
		}

		arts := artifactsByLoc[c.Key]
		ci.ArtifactsEquipped = len(arts)
		artifactQualityScore := 0.0
		if len(arts) > 0 {
			sum := 0
			qualitySum := 0.0
			for _, a := range arts {
				sum += a.Level
				if q := relevantRollQuality(a); q != nil {
					qualitySum += *q
				} else {
					// Not ratable (not a 5-star, or no substats yet) — fall
					// back to its raw level so the piece still counts for
					// something instead of silently scoring 0.
					qualitySum += float64(a.Level) / 20 * 100
				}
			}
			ci.AvgArtifactLevel = round1(float64(sum) / float64(len(arts)))
			artifactQualityScore = math.Min(qualitySum/float64(len(arts))/100, 1)
		}

		// Capped at 90: Stella Fortuna lets a character push past 90 (up to
		// 100), but that's a rare whale-tier bonus — reaching 90 is already
		// "fully leveled" for this score, same as the constellation cap above.
		levelScore := math.Min(float64(c.Level)/90, 1)
		constScore := math.Min(float64(c.Constellation)/constellationCap(ci.Rarity), 1)
		talentScore := math.Max(0, (avgTalent-1)/9)
		artifactCountScore := math.Min(float64(len(arts)), 5) / 5

		ci.Breakdown = model.ScoreBreakdown{
			Level:           scoreComponent(levelScore, 20),
			Constellation:   scoreComponent(constScore, 15),
			Talents:         scoreComponent(talentScore, 20),
			Weapon:          scoreComponent(weaponScore, 15),
			ArtifactCount:   scoreComponent(artifactCountScore, 10),
			ArtifactQuality: scoreComponent(artifactQualityScore, 20),
		}
		ci.Investment = round1(ci.Breakdown.Level.Points + ci.Breakdown.Constellation.Points + ci.Breakdown.Talents.Points +
			ci.Breakdown.Weapon.Points + ci.Breakdown.ArtifactCount.Points + ci.Breakdown.ArtifactQuality.Points)
		investmentSum += ci.Investment

		if ci.Element != "" {
			ec, ok := elementCounts[ci.Element]
			if !ok {
				ec = &model.ElementCount{Element: ci.Element, Label: ci.ElementLabel}
				elementCounts[ci.Element] = ec
			}
			ec.Count++
		}

		characters = append(characters, ci)
	}
	sort.Slice(characters, func(i, j int) bool {
		if characters[i].Investment != characters[j].Investment {
			return characters[i].Investment > characters[j].Investment
		}
		return characters[i].Name < characters[j].Name
	})

	elements := make([]model.ElementCount, 0, len(elementCounts))
	for _, ec := range elementCounts {
		elements = append(elements, *ec)
	}
	sort.Slice(elements, func(i, j int) bool {
		if elements[i].Count != elements[j].Count {
			return elements[i].Count > elements[j].Count
		}
		return elements[i].Element < elements[j].Element
	})

	weaponTypeCounts := map[string]*model.WeaponTypeStat{}
	for _, w := range export.Weapons {
		ref, known := chardb.Weapons[w.Key]
		if !known {
			continue // can't bucket a weapon we have no type for
		}
		ts, ok := weaponTypeCounts[ref.Type]
		if !ok {
			ts = &model.WeaponTypeStat{Type: ref.Type}
			weaponTypeCounts[ref.Type] = ts
		}
		if w.Location != "" {
			ts.Equipped++
		} else {
			ts.Benched++
		}
	}
	weaponTypes := make([]model.WeaponTypeStat, 0, len(weaponTypeCounts))
	for _, ts := range weaponTypeCounts {
		weaponTypes = append(weaponTypes, *ts)
	}
	sort.Slice(weaponTypes, func(i, j int) bool {
		ti, tj := weaponTypes[i], weaponTypes[j]
		return ti.Equipped+ti.Benched > tj.Equipped+tj.Benched
	})

	setCounts := map[string]int{}
	for _, a := range export.Artifacts {
		setCounts[a.SetKey]++
	}
	sets := make([]model.SetCount, 0, len(setCounts))
	for key, count := range setCounts {
		sets = append(sets, model.SetCount{Key: key, Name: setDisplayName(key, lang), Short: setShortCode(key, lang), Count: count})
	}
	sort.Slice(sets, func(i, j int) bool {
		if sets[i].Count != sets[j].Count {
			return sets[i].Count > sets[j].Count
		}
		return sets[i].Name < sets[j].Name
	})

	idleGroups := map[string]*model.IdleWeaponGroup{}
	for _, w := range export.Weapons {
		if w.Location != "" {
			continue
		}
		ref, known := chardb.Weapons[w.Key]
		if !known || ref.Rarity < 4 {
			continue
		}
		g, ok := idleGroups[w.Key]
		if !ok {
			g = &model.IdleWeaponGroup{Key: w.Key, Name: ref.NameFor(lang), Type: ref.Type, Rarity: ref.Rarity}
			idleGroups[w.Key] = g
		}
		g.Count++
		if w.Level > g.MaxLevel {
			g.MaxLevel = w.Level
		}
		if w.Refinement > g.MaxRefinement {
			g.MaxRefinement = w.Refinement
		}
	}
	idleWeapons := make([]model.IdleWeaponGroup, 0, len(idleGroups))
	for _, g := range idleGroups {
		idleWeapons = append(idleWeapons, *g)
	}
	sort.Slice(idleWeapons, func(i, j int) bool {
		if idleWeapons[i].Rarity != idleWeapons[j].Rarity {
			return idleWeapons[i].Rarity > idleWeapons[j].Rarity
		}
		if idleWeapons[i].Count != idleWeapons[j].Count {
			return idleWeapons[i].Count > idleWeapons[j].Count
		}
		return idleWeapons[i].Name < idleWeapons[j].Name
	})

	equippedArtifacts := 0
	fiveStar, locked, maxLevel := 0, 0, 0
	for _, a := range export.Artifacts {
		if a.Location != "" {
			equippedArtifacts++
		}
		if a.Rarity == 5 {
			fiveStar++
		}
		if a.Lock {
			locked++
		}
		if a.Level == 20 {
			maxLevel++
		}
	}
	equippedWeapons := 0
	for _, w := range export.Weapons {
		if w.Location != "" {
			equippedWeapons++
		}
	}
	avgInvestment := 0.0
	if len(characters) > 0 {
		avgInvestment = round1(investmentSum / float64(len(characters)))
	}

	quality := make([]model.ArtifactQuality, 0, len(export.Artifacts))
	var cvSum, rvSum float64
	var ratedCount, belowAverageCount int
	rollQualityCounts := map[string]int{}
	for _, a := range export.Artifacts {
		aq := model.ArtifactQuality{
			ID: a.ID, SetKey: a.SetKey, SetName: setDisplayName(a.SetKey, lang), SetShort: setShortCode(a.SetKey, lang), SlotKey: a.SlotKey,
			Level: a.Level, Rarity: a.Rarity, MainStatKey: a.MainStatKey, MainStatValue: a.MainStatValue,
			Location: a.Location, Lock: a.Lock, CritValue: critValue(a), SubStats: a.SubStats,
		}
		if a.Location != "" {
			aq.LocationName = charDisplayName(a.Location, lang)
		}
		if rv := rollQuality(a); rv != nil {
			aq.RollQuality = rv
			ratedCount++
			rvSum += *rv
			if *rv < 70 {
				belowAverageCount++
			}
			rollQualityCounts[rollQualityBucket(*rv)]++
		}
		cvSum += aq.CritValue
		quality = append(quality, aq)
	}

	avgCritValue, avgRollQuality := 0.0, 0.0
	if len(quality) > 0 {
		avgCritValue = round1(cvSum / float64(len(quality)))
	}
	if ratedCount > 0 {
		avgRollQuality = round1(rvSum / float64(ratedCount))
	}

	buckets := make([]model.RollQualityBucket, 0, len(rollQualityBucketOrder))
	for _, label := range rollQualityBucketOrder {
		buckets = append(buckets, model.RollQualityBucket{Label: label, Count: rollQualityCounts[label]})
	}

	hiddenGems := make([]model.ArtifactQuality, len(quality))
	copy(hiddenGems, quality)
	hiddenGems = filterArtifacts(hiddenGems, func(a model.ArtifactQuality) bool {
		return a.Location == "" && a.CritValue > 0
	})
	sort.Slice(hiddenGems, func(i, j int) bool { return hiddenGems[i].CritValue > hiddenGems[j].CritValue })
	if len(hiddenGems) > 10 {
		hiddenGems = hiddenGems[:10]
	}

	fodder := make([]model.ArtifactQuality, len(quality))
	copy(fodder, quality)
	fodder = filterArtifacts(fodder, func(a model.ArtifactQuality) bool { return a.Lock })
	sort.Slice(fodder, func(i, j int) bool { return fodder[i].CritValue < fodder[j].CritValue })
	if len(fodder) > 10 {
		fodder = fodder[:10]
	}

	return model.InsightsResponse{
		Overview: model.InsightsOverview{
			Characters: len(export.Characters), Artifacts: len(export.Artifacts), Weapons: len(export.Weapons),
			FiveStarArtifacts: fiveStar, LockedArtifacts: locked, MaxLevelArtifacts: maxLevel,
			EquippedArtifacts: equippedArtifacts, BenchedArtifacts: len(export.Artifacts) - equippedArtifacts,
			EquippedWeapons: equippedWeapons, BenchedWeapons: len(export.Weapons) - equippedWeapons,
			AvgInvestment: avgInvestment,
		},
		Characters: characters, Elements: elements, WeaponTypes: weaponTypes, Sets: sets, IdleWeapons: idleWeapons,
		ArtifactQuality: model.ArtifactQualityOverview{
			RatedArtifacts: ratedCount, AvgCritValue: avgCritValue, AvgRollQuality: avgRollQuality,
			BelowAverageCount: belowAverageCount,
		},
		RollQualityBuckets: buckets, HiddenGems: hiddenGems, FodderCandidates: fodder,
	}
}

func filterArtifacts(in []model.ArtifactQuality, keep func(model.ArtifactQuality) bool) []model.ArtifactQuality {
	out := in[:0]
	for _, a := range in {
		if keep(a) {
			out = append(out, a)
		}
	}
	return out
}

func setDisplayName(key, lang string) string {
	if ref, known := chardb.KnownSets[key]; known {
		return ref.NameFor(lang)
	}
	return chardb.SpaceOutKey(key)
}

func setShortCode(key, lang string) string {
	if ref, known := chardb.KnownSets[key]; known {
		return ref.ShortFor(lang)
	}
	return chardb.ShortCode(key)
}

func charDisplayName(key, lang string) string {
	if ref, known := chardb.Chars[key]; known {
		return ref.NameFor(lang)
	}
	return chardb.SpaceOutKey(key)
}

// critValue is the community-standard "Crit Value" heuristic: CRIT Rate%
// counts double CRIT DMG%, since (in the base damage formula) 1% CRIT Rate
// is worth roughly twice what 1% CRIT DMG is worth. Counts both the main
// stat (relevant for Circlets) and every substat.
func critValue(a model.Artifact) float64 {
	cv := 0.0
	switch a.MainStatKey {
	case "critRate_":
		cv += a.MainStatValue * 2
	case "critDMG_":
		cv += a.MainStatValue
	}
	for _, s := range a.SubStats {
		switch s.Key {
		case "critRate_":
			cv += s.Value * 2
		case "critDMG_":
			cv += s.Value
		}
	}
	return round1(cv)
}

// maxSubstatRoll5Star is each substat's highest possible single-roll value
// on a 5-star artifact (the well-known, game-version-independent constants;
// see e.g. the Genshin Impact Wiki's artifact substat tables). Used by
// rollQuality to estimate how lucky a piece's rolls were.
var maxSubstatRoll5Star = map[string]float64{
	"hp": 298.75, "hp_": 5.83, "atk": 19.45, "atk_": 5.83,
	"def": 23.15, "def_": 7.29, "em": 23.31, "enerRech_": 6.48,
	"critRate_": 3.89, "critDMG_": 7.77,
}

// rollQuality estimates "RV%" (roll value %): how close a 5-star artifact's
// substats are to having hit the highest possible tier on every roll,
// averaged across all of them. It only needs each substat's own max-roll
// constant and the piece's total roll count — not which specific roll went
// to which substat — because summing value/max across substats and
// dividing by the total roll count cancels out exactly how rolls were
// distributed between lines.
//
// totalRolls assumes the piece started with 3 initial substats (true for
// ~80% of 5-star drops) plus one roll per +4 level breakpoint; for the
// ~20% that started with 4 lines this undercounts rolls by exactly 1,
// which makes their RV% read a little high rather than a little low — an
// intentionally generous bias for what both this and other community
// tools treat as a fun approximation, not an exact figure.
func rollQuality(a model.Artifact) *float64 {
	if a.Rarity != 5 || len(a.SubStats) == 0 {
		return nil
	}
	totalRolls := 3 + a.Level/4
	sumFrac := 0.0
	for _, s := range a.SubStats {
		max, ok := maxSubstatRoll5Star[s.Key]
		if !ok || max <= 0 {
			return nil
		}
		sumFrac += s.Value / max
	}
	rv := round1(sumFrac / float64(totalRolls) * 100)
	return &rv
}

// relevantSubstats are the five substats that matter for most builds
// regardless of character (CRIT Rate/DMG, ATK%, Elemental Mastery, Energy
// Recharge%) — the other five (flat HP/ATK/DEF, HP%, DEF%) are usually dead
// weight. Used by relevantRollQuality to score a character's equipped gear
// on whether its rolls landed somewhere useful, not just on raw luck.
var relevantSubstats = map[string]bool{
	"critRate_": true, "critDMG_": true, "atk_": true, "em": true, "enerRech_": true,
}

// relevantRollQuality is rollQuality restricted to relevantSubstats: a roll
// spent on a dead substat (flat HP, DEF%, ...) scores as 0 contribution
// instead of being judged on its own luck. This is what feeds the
// Investment score's artifact-quality component — Crit Value alone would
// ignore an EM/ER%-built support's reasonable rolls, and raw roll quality
// (rollQuality) would reward a "well-rolled" piece that dumped its rolls
// into DEF%. Same nil cases and the same rare upward bias as rollQuality
// (see its comment) — only rated for 5-star pieces.
func relevantRollQuality(a model.Artifact) *float64 {
	if a.Rarity != 5 || len(a.SubStats) == 0 {
		return nil
	}
	totalRolls := 3 + a.Level/4
	sumFrac := 0.0
	for _, s := range a.SubStats {
		if !relevantSubstats[s.Key] {
			continue
		}
		max, ok := maxSubstatRoll5Star[s.Key]
		if !ok || max <= 0 {
			continue
		}
		sumFrac += s.Value / max
	}
	rv := round1(sumFrac / float64(totalRolls) * 100)
	return &rv
}

var rollQualityBucketOrder = []string{"<70%", "70–80%", "80–90%", "90–100%", "100%+"}

func rollQualityBucket(rv float64) string {
	switch {
	case rv < 70:
		return "<70%"
	case rv < 80:
		return "70–80%"
	case rv < 90:
		return "80–90%"
	case rv < 100:
		return "90–100%"
	default:
		return "100%+"
	}
}

// constellationCap is the constellation level this app treats as "fully
// invested" for the Investment score — past it, more constellations don't
// add anything. A 4-star is cheap to duplicate (banner guarantees, the
// Starglitter shop), so C6 is a realistic target; a 5-star, especially a
// limited one, is not — C2 already costs 3 copies, so holding a 5-star at
// C0 or C1 isn't "less invested," it's normal. Without this, a C0 signature
// 5-star could never outscore a C6 4-star even when otherwise maxed, which
// rewards the wrong kind of "investment." Unknown rarity (0) falls back to
// the 4-star cap, matching the old unconditional /6 behavior.
func constellationCap(rarity int) float64 {
	if rarity == 5 {
		return 2
	}
	return 6
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func scoreComponent(fraction, weight float64) model.ScoreComponent {
	return model.ScoreComponent{Fraction: math.Round(fraction*1000) / 1000, Weight: weight, Points: round1(fraction * weight)}
}
