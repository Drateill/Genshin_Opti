package damage

import (
	"math"
	"testing"

	"artifact-optimizer/internal/model"
)

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestResMultiplier_Boundaries(t *testing.T) {
	cases := []struct {
		name   string
		resPct float64
		shred  float64
		want   float64
	}{
		{"negative res (res < 0)", -50, 0, 1.25},                  // 1 - (-0.5)/2
		{"mid res (< 75%)", 50, 0, 0.5},                           // 1 - 0.5
		{"high res (>= 75%)", 90, 0, 1 / (4*0.9 + 1)},             // 1/(4*0.9+1)
		{"shred pushes into negative band", 10, 60, 1 - (-0.5)/2}, // (10-60)/100 = -0.5
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResMultiplier(c.resPct, c.shred)
			if !almostEqual(got, c.want, 1e-9) {
				t.Errorf("ResMultiplier(%v,%v) = %v, want %v", c.resPct, c.shred, got, c.want)
			}
		})
	}
}

func TestDefMultiplier(t *testing.T) {
	// Equal caster/enemy level (both +100 offset) with no shred halves damage.
	got := DefMultiplier(90, 90, 0)
	want := 0.5
	if !almostEqual(got, want, 1e-9) {
		t.Errorf("DefMultiplier(90,90,0) = %v, want %v", got, want)
	}

	// 100% DEF shred should make the enemy's effective DEF term vanish.
	got = DefMultiplier(90, 90, 100)
	if !almostEqual(got, 1.0, 1e-9) {
		t.Errorf("DefMultiplier with 100%% shred = %v, want 1.0", got)
	}
}

// baseTotals is a simple round-number build: 1000 ATK, 50% CritRate, 100%
// CritDMG, no elemental DMG% or EM, chosen so every downstream multiplier
// stays easy to hand-verify.
func baseTotals() model.BuildTotals {
	return model.BuildTotals{ATK: 1000, CritRate: 50, CritDMG: 100}
}

func TestCalculate_NoReaction_ExpectedValue(t *testing.T) {
	req := Request{
		CasterLevel: 90,
		Element:     "pyro",
		Totals:      baseTotals(),
		Components:  []Multiplier{{Label: "1-Hit DMG", MultPctAtk: 100}},
		Enemy:       EnemyInput{Level: 90, ResPct: 10},
	}
	resp := Calculate(req)
	if len(resp.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(resp.Components))
	}
	c := resp.Components[0]
	// base = 1000*100/100 = 1000; defMult = 0.5; resMult = 0.9
	// nonCrit = 1000*0.5*0.9 = 450; crit = 450*2 = 900; average = 450*1.5 = 675
	if !almostEqual(c.NonCrit, 450, 1e-6) {
		t.Errorf("NonCrit = %v, want 450", c.NonCrit)
	}
	if !almostEqual(c.Crit, 900, 1e-6) {
		t.Errorf("Crit = %v, want 900", c.Crit)
	}
	if !almostEqual(c.Average, 675, 1e-6) {
		t.Errorf("Average = %v, want 675", c.Average)
	}
	if !almostEqual(resp.Total, 675, 1e-6) {
		t.Errorf("Total = %v, want 675", resp.Total)
	}
	if resp.ReactionDamage != 0 {
		t.Errorf("expected no ReactionDamage without a reaction, got %v", resp.ReactionDamage)
	}
}

func TestCalculate_Vaporize_SelectsMultiplierByAttackElement(t *testing.T) {
	mk := func(element string) Response {
		return Calculate(Request{
			CasterLevel: 0, // defMult = 100/(100+100) = 0.5, isolates the amplifying multiplier
			Element:     element,
			Totals:      model.BuildTotals{ATK: 1000},
			Components:  []Multiplier{{Label: "1-Hit DMG", MultPctAtk: 100}},
			Enemy:       EnemyInput{Level: 0, ResPct: 0},
			Reaction:    ReactionInput{Type: ReactionVaporize},
		})
	}
	pyro := mk("pyro")   // pyro vaporizing cryo/hydro-applied target: 1.5x
	hydro := mk("hydro") // hydro vaporizing pyro-applied target: 2x
	if !almostEqual(pyro.Total, 750, 1e-6) {
		t.Errorf("pyro vaporize total = %v, want 750", pyro.Total)
	}
	if !almostEqual(hydro.Total, 1000, 1e-6) {
		t.Errorf("hydro vaporize total = %v, want 1000", hydro.Total)
	}
}

func TestCalculate_Melt_SelectsMultiplierByAttackElement(t *testing.T) {
	mk := func(element string) Response {
		return Calculate(Request{
			CasterLevel: 0,
			Element:     element,
			Totals:      model.BuildTotals{ATK: 1000},
			Components:  []Multiplier{{Label: "1-Hit DMG", MultPctAtk: 100}},
			Enemy:       EnemyInput{Level: 0, ResPct: 0},
			Reaction:    ReactionInput{Type: ReactionMelt},
		})
	}
	pyro := mk("pyro") // pyro melting cryo-applied target: 2x
	cryo := mk("cryo") // cryo melting pyro-applied target: 1.5x
	if !almostEqual(pyro.Total, 1000, 1e-6) {
		t.Errorf("pyro melt total = %v, want 1000", pyro.Total)
	}
	if !almostEqual(cryo.Total, 750, 1e-6) {
		t.Errorf("cryo melt total = %v, want 750", cryo.Total)
	}
}

func TestCalculate_Transformative_Overloaded(t *testing.T) {
	resp := Calculate(Request{
		CasterLevel: 90,
		Element:     "pyro",
		Totals:      model.BuildTotals{ATK: 1000},
		Enemy:       EnemyInput{Level: 90, ResPct: 10},
		Reaction:    ReactionInput{Type: ReactionOverloaded},
	})
	// transformativeMultiplier[overloaded] * level[90] * (1+transEMBonus(0)) * resMult(10,0)
	want := 2.75 * TransformativeLevelMultiplier[90] * 1 * 0.9
	if !almostEqual(resp.ReactionDamage, want, 1e-3) {
		t.Errorf("ReactionDamage = %v, want %v", resp.ReactionDamage, want)
	}
	if resp.Total != 0 {
		t.Errorf("expected zero talent-component damage with no components requested, got %v", resp.Total)
	}
}

func TestCalculate_Additive_FoldsIntoComponentBeforeMitigation(t *testing.T) {
	without := Calculate(Request{
		CasterLevel: 90,
		Element:     "dendro",
		Totals:      model.BuildTotals{ATK: 1000},
		Components:  []Multiplier{{Label: "Skill DMG", MultPctAtk: 100}},
		Enemy:       EnemyInput{Level: 90, ResPct: 10},
	})
	with := Calculate(Request{
		CasterLevel: 90,
		Element:     "dendro",
		Totals:      model.BuildTotals{ATK: 1000},
		Components:  []Multiplier{{Label: "Skill DMG", MultPctAtk: 100}},
		Enemy:       EnemyInput{Level: 90, ResPct: 10},
		Reaction:    ReactionInput{Type: ReactionAggravate},
	})
	if with.Total <= without.Total {
		t.Errorf("aggravate should increase damage over the no-reaction baseline: with=%v without=%v", with.Total, without.Total)
	}
}

func TestExtraBonus_AppliesOnTopOfTotals(t *testing.T) {
	base := Calculate(Request{
		CasterLevel: 90,
		Element:     "pyro",
		Totals:      baseTotals(),
		Components:  []Multiplier{{Label: "1-Hit DMG", MultPctAtk: 100}},
		Enemy:       EnemyInput{Level: 90, ResPct: 10},
	})
	boosted := Calculate(Request{
		CasterLevel: 90,
		Element:     "pyro",
		Totals:      baseTotals(),
		Components:  []Multiplier{{Label: "1-Hit DMG", MultPctAtk: 100}},
		Enemy:       EnemyInput{Level: 90, ResPct: 10},
		Extra:       ExtraBonus{DmgPct: 50, FlatATK: 100},
	})
	if boosted.Total <= base.Total {
		t.Errorf("extra bonus should increase damage: base=%v boosted=%v", base.Total, boosted.Total)
	}
}
