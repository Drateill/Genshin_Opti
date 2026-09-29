// Package damage computes expected (average) hit damage for a character
// build against an enemy, given the character's talent %ATK multipliers
// (see internal/chardb.TalentMultiplier, extracted from genshin-db) and the
// build's already-computed stat totals (internal/model.BuildTotals, as
// produced by internal/solver.totalsFor — this package never recomputes
// stat totals itself).
//
// Deliberately out of scope (see the project plan): no automatic
// character-passive or artifact-set 2pc/4pc buffs — callers approximate
// those with ExtraBonus instead; no genshin-db-sourced enemy DEF/RES —
// enemy DEF is derived from level alone and RES is whatever the caller
// supplies (see internal/enemydb for hand-curated presets). Transformative
// reactions ignore DEF entirely (matches the documented/standard behavior)
// and are always reported as a non-crit average, even for the four
// "crittable" ones (burning/bloom/burgeon/hyperbloom) — a fast-follow, not
// done here.
package damage

import "artifact-optimizer/internal/model"

// ExtraBonus approximates whatever character-passive / artifact-set 2pc-4pc
// buffs the automatic talent-multiplier extraction can't see (genshin-db
// has no numeric data for those) — the caller fills these in by hand.
type ExtraBonus struct {
	DmgPct      float64 `json:"dmgPct"`
	CritRatePct float64 `json:"critRatePct"`
	CritDmgPct  float64 `json:"critDmgPct"`
	FlatATK     float64 `json:"flatATK"`
	ATKPct      float64 `json:"atkPct"`
	DefShredPct float64 `json:"defShredPct"`
	ResShredPct float64 `json:"resShredPct"`
}

// EnemyInput is the target's level and elemental resistance (as a percent,
// e.g. 10 for the game's 10% baseline) for whichever element this
// calculation's damage or reaction instance deals.
type EnemyInput struct {
	Level  int     `json:"level"`
	ResPct float64 `json:"resPct"`
}

// ReactionInput selects an elemental reaction to apply on top of the base
// damage, plus any manual reaction-DMG-bonus% (e.g. Viridescent Venerer,
// Nahida's Burst, Furina's kit).
type ReactionInput struct {
	Type     string  `json:"type"` // "" | one of the Reaction* constants
	BonusPct float64 `json:"bonusPct"`
}

// Multiplier is one selected talent damage-multiplier curve, already
// resolved to a single talent-level value by the caller (the API handler
// looks this up from internal/chardb so the client only ever sends a
// label, never a multiplier value it could spoof).
type Multiplier struct {
	Label      string  `json:"label"`
	MultPctAtk float64 `json:"multPctAtk"` // %ATK at the selected talent level
}

// Request is everything Calculate needs for one damage computation. Element
// is the character's own damage element (chardb.CharRef.DmgKey, e.g.
// "pyro") — it selects the amplifying-reaction multiplier row, since
// vaporize/melt's multiplier depends on which element is doing the hitting.
type Request struct {
	CasterLevel int               `json:"casterLevel"`
	Element     string            `json:"element"`
	Totals      model.BuildTotals `json:"totals"`
	Components  []Multiplier      `json:"components"`
	Extra       ExtraBonus        `json:"extra"`
	Enemy       EnemyInput        `json:"enemy"`
	Reaction    ReactionInput     `json:"reaction"`
}

// ComponentResult is one selected talent component's computed damage.
type ComponentResult struct {
	Label   string  `json:"label"`
	NonCrit float64 `json:"nonCrit"`
	Crit    float64 `json:"crit"`
	Average float64 `json:"average"` // nonCrit*(1+critRate*critDMG), the expected-value damage
}

// Response is the full computed result: every requested component plus,
// separately, any transformative-reaction instance (which doesn't scale off
// ATK/talent multipliers at all).
type Response struct {
	Components     []ComponentResult `json:"components"`
	Total          float64           `json:"total"`
	ReactionDamage float64           `json:"reactionDamage,omitempty"`
}

// Calculate computes expected damage for every requested talent component,
// applying CRIT, DMG% bonuses, DEF/RES mitigation and any selected
// elemental reaction.
func Calculate(req Request) Response {
	totals := req.Totals
	em := totals.ElementMaster

	atk := totals.ATK*(1+req.Extra.ATKPct/100) + req.Extra.FlatATK
	dmgBonusMult := 1 + (totals.ElementalDMG+req.Extra.DmgPct)/100
	critRate := clamp01((totals.CritRate + req.Extra.CritRatePct) / 100)
	critDMG := (totals.CritDMG + req.Extra.CritDmgPct) / 100
	defMult := DefMultiplier(req.CasterLevel, req.Enemy.Level, req.Extra.DefShredPct)
	resMult := ResMultiplier(req.Enemy.ResPct, req.Extra.ResShredPct)

	additiveBonus := 0.0
	if isAdditive(req.Reaction.Type) {
		additiveBonus = levelMultiplier(req.CasterLevel) * additiveMultiplier[req.Reaction.Type] *
			(addEMBonus(em) + req.Reaction.BonusPct/100)
	}

	ampMult := 1.0
	if isAmplifying(req.Reaction.Type) {
		if row, ok := amplifyingMultiplier[req.Reaction.Type][req.Element]; ok {
			ampMult = row * (ampEMBonus(em) + req.Reaction.BonusPct/100)
		}
	}

	components := make([]ComponentResult, 0, len(req.Components))
	total := 0.0
	for _, c := range req.Components {
		base := atk*c.MultPctAtk/100 + additiveBonus
		nonCrit := base * dmgBonusMult * defMult * resMult * ampMult
		crit := nonCrit * (1 + critDMG)
		average := nonCrit * (1 + critRate*critDMG)
		components = append(components, ComponentResult{Label: c.Label, NonCrit: nonCrit, Crit: crit, Average: average})
		total += average
	}

	resp := Response{Components: components, Total: total}
	if isTransformative(req.Reaction.Type) {
		resp.ReactionDamage = transformativeMultiplier[req.Reaction.Type] * levelMultiplier(req.CasterLevel) *
			(1 + transEMBonus(em) + req.Reaction.BonusPct/100) * resMult
	}
	return resp
}
