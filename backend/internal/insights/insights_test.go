package insights

import (
	"testing"

	"artifact-optimizer/internal/model"
)

func TestCompute_Basic(t *testing.T) {
	export := model.GoodExport{
		Characters: []model.Character{
			{Key: "HuTao", Level: 90, Constellation: 1, Talent: model.Talent{Auto: 9, Skill: 9, Burst: 9}},
			{Key: "NotARealCharacter", Level: 20, Talent: model.Talent{Auto: 1, Skill: 1, Burst: 1}},
		},
		Weapons: []model.Weapon{
			{Key: "DragonsBane", Level: 90, Refinement: 1, Location: "HuTao"},
			{Key: "DragonsBane", Level: 1, Refinement: 1}, // idle
			{Key: "DragonsBane", Level: 1, Refinement: 1}, // idle, same model
		},
		Artifacts: []model.Artifact{
			{SetKey: "CrimsonWitchOfFlames", SlotKey: "flower", Level: 20, Rarity: 5, Lock: true, Location: "HuTao"},
			{
				// A "perfect roll" 5-star piece: every substat landed
				// exactly on its highest tier every time (2+1+1+4 = 8
				// rolls total, matching totalRolls at level 20), so RV%
				// should come out to exactly 100.
				SetKey: "CrimsonWitchOfFlames", SlotKey: "plume", Level: 20, Rarity: 5, MainStatKey: "hp", Location: "HuTao",
				SubStats: []model.Stat{
					{Key: "critRate_", Value: 7.78}, {Key: "critDMG_", Value: 7.77},
					{Key: "em", Value: 23.31}, {Key: "atk_", Value: 23.32},
				},
			},
			{SetKey: "GladiatorsFinale", SlotKey: "sands", Level: 4, Rarity: 4}, // unequipped, not rated (not 5-star)
			{
				// Unequipped, unlocked, high Crit Value — should surface as a hidden gem.
				SetKey: "GladiatorsFinale", SlotKey: "circlet", Level: 20, Rarity: 5, MainStatKey: "critDMG_", MainStatValue: 62.2,
				SubStats: []model.Stat{{Key: "critRate_", Value: 10}},
			},
			{
				// Locked but worthless (no crit stats at all) — should surface as fodder.
				SetKey: "GladiatorsFinale", SlotKey: "flower", Level: 0, Rarity: 5, MainStatKey: "hp", Lock: true,
				SubStats: []model.Stat{{Key: "def_", Value: 5}},
			},
		},
	}

	out := Compute(export, "en")

	if out.Overview.Characters != 2 || out.Overview.Weapons != 3 || out.Overview.Artifacts != 5 {
		t.Fatalf("unexpected overview counts: %+v", out.Overview)
	}
	if out.Overview.EquippedArtifacts != 2 || out.Overview.BenchedArtifacts != 3 {
		t.Errorf("expected 2 equipped / 3 benched artifact, got %+v", out.Overview)
	}
	if out.Overview.FiveStarArtifacts != 4 {
		t.Errorf("expected 4 five-star artifacts, got %d", out.Overview.FiveStarArtifacts)
	}

	if len(out.Characters) != 2 {
		t.Fatalf("expected 2 characters, got %d", len(out.Characters))
	}
	hutao := out.Characters[0]
	if hutao.Key != "HuTao" || !hutao.Known || hutao.Element != "Pyro" {
		t.Errorf("expected HuTao first, known, Pyro — got %+v", hutao)
	}
	if hutao.Investment <= out.Characters[1].Investment {
		t.Errorf("expected HuTao (built) to outscore the untouched character: %+v vs %+v", hutao, out.Characters[1])
	}
	if out.Characters[1].Known {
		t.Errorf("expected NotARealCharacter to be unknown")
	}

	if len(out.Elements) != 1 || out.Elements[0].Element != "Pyro" || out.Elements[0].Count != 1 {
		t.Errorf("expected 1 Pyro element entry, got %+v", out.Elements)
	}

	foundIdle := false
	for _, g := range out.IdleWeapons {
		if g.Key == "DragonsBane" {
			foundIdle = true
			if g.Count != 2 {
				t.Errorf("expected 2 idle DragonsBane copies, got %d", g.Count)
			}
		}
	}
	if !foundIdle {
		t.Errorf("expected DragonsBane to show up as an idle weapon group, got %+v", out.IdleWeapons)
	}

	if len(out.Sets) != 2 {
		t.Fatalf("expected 2 distinct sets, got %+v", out.Sets)
	}

	// Artifact quality: 3 of the 5 pieces are rated (5-star with substats) —
	// the "perfect roll" plume should land at exactly 100% RV.
	if out.ArtifactQuality.RatedArtifacts != 3 {
		t.Fatalf("expected 3 rated artifacts, got %+v", out.ArtifactQuality)
	}
	if len(out.HiddenGems) != 1 || out.HiddenGems[0].SlotKey != "circlet" || out.HiddenGems[0].CritValue != 82.2 {
		t.Errorf("expected the unequipped circlet (CV 82.2) as the sole hidden gem, got %+v", out.HiddenGems)
	}
	if len(out.FodderCandidates) != 2 {
		t.Errorf("expected 2 locked, zero-CV fodder candidates, got %+v", out.FodderCandidates)
	}
	for _, a := range out.FodderCandidates {
		if !a.Lock || a.CritValue != 0 {
			t.Errorf("fodder candidate should be locked with 0 crit value, got %+v", a)
		}
	}

	if out.ArtifactQuality.BelowAverageCount != 2 {
		t.Errorf("expected 2 rated artifacts below 70%% RV, got %+v", out.ArtifactQuality)
	}
	if out.ArtifactQuality.AvgRollQuality != 51.7 {
		t.Errorf("expected avg roll quality 51.7, got %v", out.ArtifactQuality.AvgRollQuality)
	}
	if out.ArtifactQuality.AvgCritValue != 21.1 {
		t.Errorf("expected avg crit value 21.1, got %v", out.ArtifactQuality.AvgCritValue)
	}

	bucketCounts := map[string]int{}
	for _, b := range out.RollQualityBuckets {
		bucketCounts[b.Label] = b.Count
	}
	if bucketCounts["100%+"] != 1 || bucketCounts["<70%"] != 2 {
		t.Errorf("unexpected roll quality buckets: %+v", out.RollQualityBuckets)
	}
}

// TestCompute_ConstellationCapByRarity locks in the fix for a real bug
// report: a maxed-out 5-star sitting at C0/C1 (normal — C2 already costs 3
// copies of a limited banner character) scored below a middling 4-star at
// C6 (cheap — 4-stars are easy to duplicate), because the old formula
// divided every character's constellation by 6 regardless of rarity. A
// 5-star should cap out its constellation credit at C2; a 4-star still
// needs C6.
func TestCompute_ConstellationCapByRarity(t *testing.T) {
	mk := func(key string, cons int) model.Character {
		return model.Character{Key: key, Level: 90, Constellation: cons, Talent: model.Talent{Auto: 9, Skill: 9, Burst: 9}}
	}
	// HuTao is a 5-star; Xiangling is a 4-star (see backend/internal/chardb/data/characters.json).
	export := model.GoodExport{Characters: []model.Character{
		mk("HuTao", 0), mk("HuTao", 1), mk("HuTao", 2), mk("HuTao", 6),
		mk("Xiangling", 2), mk("Xiangling", 6),
	}}
	out := Compute(export, "en")

	huTaoByConst := map[int]float64{}
	xianglingByConst := map[int]float64{}
	for _, c := range out.Characters {
		switch c.Key {
		case "HuTao":
			huTaoByConst[c.Constellation] = c.Investment
		case "Xiangling":
			xianglingByConst[c.Constellation] = c.Investment
		}
	}

	if huTaoByConst[2] != huTaoByConst[6] {
		t.Errorf("expected a 5-star's C2 and C6 to score identically (capped), got C2=%v C6=%v", huTaoByConst[2], huTaoByConst[6])
	}
	if huTaoByConst[0] >= huTaoByConst[2] {
		t.Errorf("expected a 5-star's C0 to score below its (capped) C2, got C0=%v C2=%v", huTaoByConst[0], huTaoByConst[2])
	}
	if huTaoByConst[1] >= huTaoByConst[2] {
		t.Errorf("expected a 5-star's C1 to score below its (capped) C2, got C1=%v C2=%v", huTaoByConst[1], huTaoByConst[2])
	}
	if xianglingByConst[2] >= xianglingByConst[6] {
		t.Errorf("expected a 4-star's C6 to still outscore its C2 (not capped that early), got C2=%v C6=%v", xianglingByConst[2], xianglingByConst[6])
	}
}

// TestCompute_LevelCapAt90 checks that Stella Fortuna (which lets a
// character push past the normal level-90 cap, to 95 or 100) doesn't
// inflate the Investment score — 90, 95 and 100 should score identically.
func TestCompute_LevelCapAt90(t *testing.T) {
	mk := func(level int) model.Character {
		return model.Character{Key: "HuTao", Level: level, Constellation: 2, Talent: model.Talent{Auto: 9, Skill: 9, Burst: 9}}
	}
	export := model.GoodExport{Characters: []model.Character{mk(90), mk(95), mk(100)}}
	out := Compute(export, "en")

	byLevel := map[int]float64{}
	for _, c := range out.Characters {
		byLevel[c.Level] = c.Investment
	}
	if byLevel[90] != byLevel[95] || byLevel[90] != byLevel[100] {
		t.Errorf("expected level 90/95/100 to score identically (capped), got 90=%v 95=%v 100=%v", byLevel[90], byLevel[95], byLevel[100])
	}
}

// TestCompute_ScoreBreakdown checks that the six ScoreBreakdown components
// (shown in the frontend's per-character detail modal) sum back to
// Investment, and that a fully-capped 5-star at C2/Lv90 shows full marks on
// both the level and constellation components.
func TestCompute_ScoreBreakdown(t *testing.T) {
	export := model.GoodExport{Characters: []model.Character{
		{Key: "HuTao", Level: 90, Constellation: 2, Talent: model.Talent{Auto: 9, Skill: 9, Burst: 9}},
	}}
	out := Compute(export, "en")
	c := out.Characters[0]
	b := c.Breakdown

	sum := round1(b.Level.Points + b.Constellation.Points + b.Talents.Points + b.Weapon.Points + b.ArtifactCount.Points + b.ArtifactQuality.Points)
	if sum != c.Investment {
		t.Errorf("expected breakdown points to sum to Investment: sum=%v investment=%v", sum, c.Investment)
	}

	if b.Level.Fraction != 1 || b.Level.Weight != 20 || b.Level.Points != 20 {
		t.Errorf("expected a Lv90 character to max out the level component, got %+v", b.Level)
	}
	if b.Constellation.Fraction != 1 || b.Constellation.Weight != 15 || b.Constellation.Points != 15 {
		t.Errorf("expected a C2 5-star to max out the constellation component, got %+v", b.Constellation)
	}
	// No weapon and no artifacts equipped — those two components should be zero, not missing/NaN.
	if b.Weapon.Points != 0 || b.ArtifactCount.Points != 0 || b.ArtifactQuality.Points != 0 {
		t.Errorf("expected zero weapon/artifact points with none equipped, got weapon=%+v count=%+v level=%+v", b.Weapon, b.ArtifactCount, b.ArtifactQuality)
	}
}

// TestCompute_ArtifactQualityCountsRelevantRollsOnly locks in the fix for a
// second bug report: the artifact-quality component used to just average
// AvgArtifactLevel, so a level-20 piece scored full marks regardless of
// whether its rolls went into CRIT/EM/ER% or into dead weight like flat DEF.
// Two level-20 5-star pieces with identically "lucky" rolls (every roll hit
// its own stat's max tier) should score very differently here: one rolled
// entirely into relevant stats, the other entirely into irrelevant ones.
func TestCompute_ArtifactQualityCountsRelevantRollsOnly(t *testing.T) {
	good := model.Character{Key: "HuTao", Level: 1, Talent: model.Talent{Auto: 1, Skill: 1, Burst: 1}}
	bad := model.Character{Key: "KamisatoAyaka", Level: 1, Talent: model.Talent{Auto: 1, Skill: 1, Burst: 1}}
	export := model.GoodExport{
		Characters: []model.Character{good, bad},
		Artifacts: []model.Artifact{
			{
				// All 4 substats are on the "relevant" list, all at max roll.
				SetKey: "CrimsonWitchOfFlames", SlotKey: "flower", Level: 20, Rarity: 5, MainStatKey: "hp", Location: "HuTao",
				SubStats: []model.Stat{
					{Key: "critRate_", Value: 7.78}, {Key: "critDMG_", Value: 7.77},
					{Key: "em", Value: 23.31}, {Key: "atk_", Value: 23.32},
				},
			},
			{
				// Same level, same "every roll at max tier" luck, but all 4
				// substats are on the dead-weight list.
				SetKey: "CrimsonWitchOfFlames", SlotKey: "flower", Level: 20, Rarity: 5, MainStatKey: "hp", Location: "KamisatoAyaka",
				SubStats: []model.Stat{
					{Key: "hp_", Value: 5.83}, {Key: "def_", Value: 7.29},
					{Key: "def", Value: 23.15}, {Key: "hp", Value: 298.75},
				},
			},
		},
	}
	out := Compute(export, "en")

	byKey := map[string]float64{}
	for _, c := range out.Characters {
		byKey[c.Key] = c.Breakdown.ArtifactQuality.Fraction
	}
	if byKey["HuTao"] <= 0.9 {
		t.Errorf("expected the all-relevant-substats piece to score near-max artifact quality, got %v", byKey["HuTao"])
	}
	if byKey["KamisatoAyaka"] > 0.1 {
		t.Errorf("expected the all-dead-substats piece to score near-zero artifact quality despite being level 20, got %v", byKey["KamisatoAyaka"])
	}
}

// TestCompute_WeaponRefinementCapByRarity locks in the fix for a third bug
// report: a R1 5-star weapon (often already its ceiling — duplicating a
// limited or even standard-banner 5-star is rare) scored worse than a R5
// 4-star (cheap to duplicate/craft), because the old formula divided every
// weapon's refinement by 5 regardless of rarity. A 5-star should cap its
// refinement credit at R1; a 4-star still needs R5.
func TestCompute_WeaponRefinementCapByRarity(t *testing.T) {
	mk := func(key string, refine int) (model.Character, model.Weapon) {
		return model.Character{Key: "HuTao", Level: 90, Talent: model.Talent{Auto: 9, Skill: 9, Burst: 9}},
			model.Weapon{Key: key, Level: 90, Refinement: refine, Location: "HuTao"}
	}
	// SkywardSpine is a 5-star polearm; DragonsBane is a 4-star polearm
	// (see backend/internal/chardb/data/weapons.json). Compute doesn't
	// check weapon-type/character compatibility, so reusing HuTao's key
	// across cases is fine — each case is its own isolated export.
	run := func(weaponKey string, refine int) float64 {
		c, w := mk(weaponKey, refine)
		out := Compute(model.GoodExport{Characters: []model.Character{c}, Weapons: []model.Weapon{w}}, "en")
		return out.Characters[0].Breakdown.Weapon.Fraction
	}

	fiveStarR1 := run("SkywardSpine", 1)
	fiveStarR5 := run("SkywardSpine", 5)
	fourStarR1 := run("DragonsBane", 1)
	fourStarR5 := run("DragonsBane", 5)

	if fiveStarR1 != fiveStarR5 {
		t.Errorf("expected a 5-star's R1 and R5 to score identically (capped), got R1=%v R5=%v", fiveStarR1, fiveStarR5)
	}
	if fourStarR1 >= fourStarR5 {
		t.Errorf("expected a 4-star's R5 to still outscore its R1 (not capped that early), got R1=%v R5=%v", fourStarR1, fourStarR5)
	}
	if fiveStarR1 != fourStarR5 {
		t.Errorf("expected a capped 5-star R1 to match an uncapped 4-star R5 (both 'complete'), got 5*R1=%v 4*R5=%v", fiveStarR1, fourStarR5)
	}
}
