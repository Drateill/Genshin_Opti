package damage

// Reaction type constants accepted in ReactionInput.Type. Only the original
// nine transformative reactions plus vaporize/melt (amplifying) and
// spread/aggravate (additive, Dendro) are supported — no Natlan-era
// lunar/stellar/crystallize reactions.
const (
	ReactionNone = ""

	ReactionVaporize = "vaporize"
	ReactionMelt     = "melt"

	ReactionOverloaded     = "overloaded"
	ReactionSuperconduct   = "superconduct"
	ReactionElectroCharged = "electrocharged"
	ReactionSwirl          = "swirl"
	ReactionShattered      = "shattered"
	ReactionBurning        = "burning"
	ReactionBloom          = "bloom"
	ReactionBurgeon        = "burgeon"
	ReactionHyperbloom     = "hyperbloom"

	ReactionSpread    = "spread"
	ReactionAggravate = "aggravate"
)

// amplifyingMultiplier is keyed [reaction][attackingElement] — vaporize/melt
// have a different fixed multiplier depending on whether the character's
// own damage element is the "catalyst" (pyro melting cryo, hydro vaporizing
// pyro) or the "reagent" element (cryo melting, pyro vaporizing).
var amplifyingMultiplier = map[string]map[string]float64{
	ReactionVaporize: {"hydro": 2, "pyro": 1.5},
	ReactionMelt:     {"pyro": 2, "cryo": 1.5},
}

// transformativeMultiplier is each transformative reaction's fixed base
// multiplier, current as of the 5.2 update (which raised Overloaded from 2
// to 2.75 and Superconduct from 0.5 to 1.5).
var transformativeMultiplier = map[string]float64{
	ReactionOverloaded:     2.75,
	ReactionShattered:      3,
	ReactionElectroCharged: 2,
	ReactionSuperconduct:   1.5,
	ReactionSwirl:          0.6,
	ReactionBurning:        0.25,
	ReactionBloom:          2,
	ReactionBurgeon:        3,
	ReactionHyperbloom:     3,
}

// additiveMultiplier is each Dendro additive reaction's fixed base
// multiplier, applied to a bonus flat-damage term folded into the
// triggering hit rather than reported as a separate instance.
var additiveMultiplier = map[string]float64{
	ReactionSpread:    1.25,
	ReactionAggravate: 1.15,
}

func isAmplifying(reaction string) bool     { _, ok := amplifyingMultiplier[reaction]; return ok }
func isTransformative(reaction string) bool { _, ok := transformativeMultiplier[reaction]; return ok }
func isAdditive(reaction string) bool       { _, ok := additiveMultiplier[reaction]; return ok }

// ampEMBonus, addEMBonus and transEMBonus are the standard Elemental
// Mastery scaling curves for amplifying, additive and transformative
// reactions respectively — unchanged since the reactions' introduction.
func ampEMBonus(em float64) float64   { return 1 + (25.0/9)*em/(1400+em) }
func addEMBonus(em float64) float64   { return 1 + 5*em/(1200+em) }
func transEMBonus(em float64) float64 { return 16 * em / (2000 + em) }
