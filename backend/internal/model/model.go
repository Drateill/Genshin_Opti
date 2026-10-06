// Package model defines the core domain types shared across the GOOD
// parser, the solver and the HTTP API, mirroring the Go structs in the
// project spec (uploads/genshin-artifact-optimizer-spec.md).
package model

// Stat is a single artifact stat: either a substat or a computed main stat.
type Stat struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
}

// Artifact is a single piece of gear, normalized from a GOOD export.
type Artifact struct {
	ID            int     `json:"id"`
	SetKey        string  `json:"setKey"`
	SlotKey       string  `json:"slotKey"`
	Level         int     `json:"level"`
	Rarity        int     `json:"rarity"`
	MainStatKey   string  `json:"mainStatKey"`
	MainStatValue float64 `json:"mainStatValue"`
	Location      string  `json:"location"`
	Lock          bool    `json:"lock"`
	SubStats      []Stat  `json:"substats"`
}

// Talent holds the three levelable talents of a character.
type Talent struct {
	Auto  int `json:"auto"`
	Skill int `json:"skill"`
	Burst int `json:"burst"`
}

// Character is a single roster entry from a GOOD export.
type Character struct {
	Key           string `json:"key"`
	Level         int    `json:"level"`
	Ascension     int    `json:"ascension"`
	Constellation int    `json:"constellation"`
	Talent        Talent `json:"talent"`
}

// Weapon is a single weapon, normalized from a GOOD export.
type Weapon struct {
	ID         int    `json:"id"`
	Key        string `json:"key"`
	Level      int    `json:"level"`
	Ascension  int    `json:"ascension"`
	Refinement int    `json:"refinement"`
	Location   string `json:"location"`
	Lock       bool   `json:"lock"`
}

// GoodExport is the parsed, normalized form of a GOOD JSON document.
type GoodExport struct {
	Format     string      `json:"format"`
	Version    int         `json:"version"`
	Source     string      `json:"source,omitempty"`
	Characters []Character `json:"characters"`
	Artifacts  []Artifact  `json:"artifacts"`
	Weapons    []Weapon    `json:"weapons"`
}

// ImportSummary is returned from POST /api/import.
type ImportSummary struct {
	Characters int      `json:"characters"`
	Artifacts  int      `json:"artifacts"`
	Sets       int      `json:"sets"`
	SetKeys    []string `json:"setKeys"`
}

// RosterEntry is one character available to solve for: a GOOD import entry
// joined against our known reference stat data (see internal/chardb).
type RosterEntry struct {
	Key           string `json:"key"`
	Name          string `json:"name"`
	Element       string `json:"element"`
	Level         int    `json:"level"`
	Constellation int    `json:"constellation"`
	RecSet        string `json:"recSet"`
	DmgKey        string `json:"dmgKey"`
	WeaponType    string `json:"weaponType"`       // sword | claymore | polearm | bow | catalyst; empty when unknown
	Rarity        int    `json:"rarity,omitempty"` // 4 or 5, 0 when unknown
	Talent        Talent `json:"talent"`
	Icon          string `json:"icon,omitempty"` // avatar icon URL, empty when unknown
	Known         bool   `json:"known"`          // has reference base-stat data usable by the solver

	EquippedWeapon      string `json:"equippedWeapon,omitempty"` // display name of the weapon equipped on this character in the import, if any
	EquippedWeaponKnown bool   `json:"equippedWeaponKnown"`      // whether EquippedWeapon has reference data (see chardb.Weapons)
}

// WeaponOption is one distinct weapon owned by the player, offered for a
// given character's weapon-type slot in the Configure UI. When the player
// owns more than one copy of the same weapon, only the best copy (highest
// level, then highest refinement) is offered and Count says how many
// copies were collapsed into it.
type WeaponOption struct {
	ID           int     `json:"id"`
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Rarity       int     `json:"rarity"`
	Level        int     `json:"level"`
	Refinement   int     `json:"refinement"`
	Count        int     `json:"count"` // how many copies of this weapon the player owns
	ATK          float64 `json:"atk"`
	SubStatKey   string  `json:"subStatKey,omitempty"`
	SubStatValue float64 `json:"subStatValue,omitempty"`
	Icon         string  `json:"icon,omitempty"` // weapon icon URL
	Known        bool    `json:"known"`          // has reference data usable by the solver
}

// SetInfo is one artifact set detected in the imported inventory.
type SetInfo struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Short       string         `json:"short"`
	Description string         `json:"description,omitempty"` // 2pc/4pc bonus text, empty when the set isn't in chardb.KnownSets
	Icon        string         `json:"icon,omitempty"`        // one representative piece's icon URL
	Counts      map[string]int `json:"counts"`                // slotKey -> count
}

// StatRange is an optional min and/or max bound on one build stat. Either
// bound may be nil, meaning that side is unconstrained.
type StatRange struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// CharacterInsight is one roster entry enriched with an "investment score"
// (0-100) and its equipped gear, for GET /api/insights.
type CharacterInsight struct {
	Key           string  `json:"key"`
	Name          string  `json:"name"`
	Element       string  `json:"element,omitempty"`      // stable English key (Pyro, Hydro, ...), empty when unknown
	ElementLabel  string  `json:"elementLabel,omitempty"` // localized display text
	WeaponType    string  `json:"weaponType,omitempty"`
	Rarity        int     `json:"rarity,omitempty"`
	Level         int     `json:"level"`
	Constellation int     `json:"constellation"`
	AvgTalent     float64 `json:"avgTalent"`
	Known         bool    `json:"known"`

	WeaponName       string `json:"weaponName,omitempty"`
	WeaponRarity     int    `json:"weaponRarity,omitempty"`
	WeaponRefinement int    `json:"weaponRefinement,omitempty"`
	WeaponLevel      int    `json:"weaponLevel,omitempty"`

	ArtifactsEquipped int     `json:"artifactsEquipped"`
	AvgArtifactLevel  float64 `json:"avgArtifactLevel"`

	// Investment is a composite 0-100 score: level (20%, capped at 90 —
	// Stella Fortuna can push a character to 95/100, but that's a rare bonus
	// on top of "fully leveled," not a requirement for it), constellation
	// (15%, capped at C2 for a 5-star and C6 for a 4-star — see
	// insights.constellationCap), average talent level (20%), equipped
	// weapon level+refinement (15%, refinement capped at R1 for a 5-star and
	// R5 for a 4-star/3-star, same reasoning as the constellation cap),
	// number of equipped artifacts (10%), and the quality of their rolls
	// (20% — see ScoreBreakdown.ArtifactQuality).
	// It's a rough "how built is this character" heuristic, not a solver
	// input — purely for the account-insights overview.
	Investment float64 `json:"investment"`
	// Breakdown is Investment's six weighted components, for a per-character
	// detail view — their Points always sum to Investment (modulo rounding).
	Breakdown ScoreBreakdown `json:"breakdown"`
}

// ScoreComponent is one weighted component of a CharacterInsight.Investment
// score.
type ScoreComponent struct {
	Fraction float64 `json:"fraction"` // 0-1, how "complete" this component is (see insights.Compute for how each is capped)
	Weight   float64 `json:"weight"`   // this component's share of the 100-point total, e.g. 20
	Points   float64 `json:"points"`   // Fraction * Weight — the actual points earned, 0..Weight
}

// ScoreBreakdown is CharacterInsight.Investment split into its six inputs:
// level, constellation, talents, weapon, number of equipped artifacts, and
// their roll quality — see insights.Compute for the exact formula.
type ScoreBreakdown struct {
	Level         ScoreComponent `json:"level"`
	Constellation ScoreComponent `json:"constellation"`
	Talents       ScoreComponent `json:"talents"`
	// Weapon is half the equipped weapon's level (/90) and half its
	// refinement, the refinement half capped by rarity like Constellation
	// is — R1 is "complete" for a 5-star (rare to duplicate, signature or
	// not), R5 is still expected for a cheaply-farmed/crafted 4-star or
	// 3-star.
	Weapon        ScoreComponent `json:"weapon"`
	ArtifactCount ScoreComponent `json:"artifactCount"`
	// ArtifactQuality averages each equipped artifact's "relevant roll
	// quality" (see insights.relevantRollQuality): how close its CRIT
	// Rate/DMG, ATK%, EM and Energy Recharge% substats are to their max
	// possible rolls, counting a roll spent on HP/DEF/flat ATK as wasted
	// rather than judging it on its own luck. Falls back to the artifact's
	// raw level for a piece that isn't 5-star (so it still counts for
	// something, just not for its substats).
	ArtifactQuality ScoreComponent `json:"artifactQuality"`
}

// ElementCount is the number of owned characters of one element.
type ElementCount struct {
	Element string `json:"element"` // stable English key (Pyro, Hydro, ...)
	Label   string `json:"label"`   // localized display text
	Count   int    `json:"count"`
}

// WeaponTypeStat is the equipped-vs-benched split of owned weapons of one
// weapon type.
type WeaponTypeStat struct {
	Type     string `json:"type"`
	Equipped int    `json:"equipped"`
	Benched  int    `json:"benched"`
}

// SetCount is the total number of owned artifact pieces belonging to one
// set, across every slot and character.
type SetCount struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Short string `json:"short"`
	Count int    `json:"count"`
}

// IdleWeaponGroup is one weapon model sitting unequipped in the inventory,
// with its copies collapsed into a single row.
type IdleWeaponGroup struct {
	Key           string `json:"key"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Rarity        int    `json:"rarity"`
	Count         int    `json:"count"` // how many idle copies of this weapon
	MaxLevel      int    `json:"maxLevel"`
	MaxRefinement int    `json:"maxRefinement"`
}

// InsightsOverview is the top-line counts shown in GET /api/insights.
type InsightsOverview struct {
	Characters        int     `json:"characters"`
	Artifacts         int     `json:"artifacts"`
	Weapons           int     `json:"weapons"`
	FiveStarArtifacts int     `json:"fiveStarArtifacts"`
	LockedArtifacts   int     `json:"lockedArtifacts"`
	MaxLevelArtifacts int     `json:"maxLevelArtifacts"` // artifacts at level 20
	EquippedArtifacts int     `json:"equippedArtifacts"`
	BenchedArtifacts  int     `json:"benchedArtifacts"`
	EquippedWeapons   int     `json:"equippedWeapons"`
	BenchedWeapons    int     `json:"benchedWeapons"`
	AvgInvestment     float64 `json:"avgInvestment"`
}

// ArtifactQuality is one artifact enriched with its Crit Value and an
// estimated substat roll quality, for GET /api/insights.
type ArtifactQuality struct {
	ID            int     `json:"id"`
	SetKey        string  `json:"setKey"`
	SetName       string  `json:"setName"`
	SetShort      string  `json:"setShort"`
	SlotKey       string  `json:"slotKey"`
	Level         int     `json:"level"`
	Rarity        int     `json:"rarity"`
	MainStatKey   string  `json:"mainStatKey"`
	MainStatValue float64 `json:"mainStatValue"`
	Location      string  `json:"location,omitempty"`
	LocationName  string  `json:"locationName,omitempty"`
	Lock          bool    `json:"lock"`
	CritValue     float64 `json:"critValue"`
	// RollQuality is "RV%": the substats' total value against the total
	// value they'd have if every roll had hit its highest possible tier —
	// nil when not computable (only rated for 5-star pieces, see
	// insights.rollQuality).
	RollQuality *float64 `json:"rollQuality,omitempty"`
	// SubStats is included so the frontend can show the full roll detail
	// (e.g. in a click-to-expand artifact card) instead of just the
	// summary CritValue/RollQuality numbers.
	SubStats []Stat `json:"substats"`
}

// RollQualityBucket is one bucket of a roll-quality histogram.
type RollQualityBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// ArtifactQualityOverview summarizes Crit Value and roll quality across the
// whole inventory.
type ArtifactQualityOverview struct {
	RatedArtifacts    int     `json:"ratedArtifacts"` // 5-star artifacts with a computed RollQuality
	AvgCritValue      float64 `json:"avgCritValue"`
	AvgRollQuality    float64 `json:"avgRollQuality"`
	BelowAverageCount int     `json:"belowAverageCount"` // rated artifacts under 70% roll quality
}

// InsightsResponse is returned from GET /api/insights — an account-wide
// audit of the current import, independent of the solver.
type InsightsResponse struct {
	Overview           InsightsOverview        `json:"overview"`
	Characters         []CharacterInsight      `json:"characters"` // sorted by Investment, descending
	Elements           []ElementCount          `json:"elements"`
	WeaponTypes        []WeaponTypeStat        `json:"weaponTypes"`
	Sets               []SetCount              `json:"sets"`        // sorted by Count, descending
	IdleWeapons        []IdleWeaponGroup       `json:"idleWeapons"` // benched 4-5* weapons, sorted by rarity then count, descending
	ArtifactQuality    ArtifactQualityOverview `json:"artifactQuality"`
	RollQualityBuckets []RollQualityBucket     `json:"rollQualityBuckets"`
	HiddenGems         []ArtifactQuality       `json:"hiddenGems"`       // best Crit Value, currently unequipped
	FodderCandidates   []ArtifactQuality       `json:"fodderCandidates"` // locked pieces with the worst Crit Value
}

// SolveRequest is the body of POST /api/solve.
type SolveRequest struct {
	CharacterKey string `json:"characterKey"`
	TargetSetKey string `json:"targetSetKey"`
	// TargetSetKey2, when set, switches the solver into dual-set mode: it
	// requires at least 2 pieces of TargetSetKey AND at least 2 of
	// TargetSetKey2, instead of the default single-set "4 of 5" requirement.
	TargetSetKey2   string               `json:"targetSetKey2,omitempty"`
	WeaponID        *int                 `json:"weaponId,omitempty"` // ID of an owned WeaponOption to equip; omitted/nil = no weapon
	SlotConstraints map[string][]string  `json:"slotConstraints"`    // sands/goblet/circlet -> allowed main stat keys
	Constraints     map[string]StatRange `json:"constraints"`        // stat key -> min/max bounds on the finished build's total
	// Objective is the stat the solver maximizes when picking/ranking builds:
	// "critValue" (default) | "hp" | "atk" | "enerRech_" | "em" | "critRate_"
	// | "critDMG_" | "elementalDmg" (the solving character's own elemental
	// DMG% stat). Empty or unrecognized falls back to "critValue". "dmg" (the
	// spec's V2 real-damage objective) is not implemented.
	Objective string `json:"objective"`
	TopN      int    `json:"topN"`
	// IncludeEquippedByOthers controls whether artifacts currently equipped
	// on a character other than CharacterKey are eligible for the build.
	// Unequipped artifacts and CharacterKey's own gear are always eligible.
	IncludeEquippedByOthers bool `json:"includeEquippedByOthers"`
	// TeamElements holds 0-3 elemental keys ("Pyro", "Hydro", ...) for the
	// optimized character's teammates. Combined with the character's own
	// element, 2+ party members sharing an element triggers that element's
	// resonance (see solver.resonanceBonus) — only the resonances that feed
	// a stat this solver tracks (Pyro/Electro/Cryo/Dendro) affect the build
	// totals; the rest are display-only in the frontend.
	TeamElements []string `json:"teamElements,omitempty"`
	// Lang is the UI language ("en" | "fr") the solver's free-text Reason
	// message (SolveResponse.Reason) should be written in when no builds are
	// found. Everything else in the response is plain data the frontend
	// localizes itself. Empty/unrecognized falls back to English.
	Lang string `json:"lang,omitempty"`
}

// BuildTotals is the summed stat line for a complete 5-piece build.
type BuildTotals struct {
	CritRate       float64 `json:"critRate"`
	CritDMG        float64 `json:"critDMG"`
	ElementMaster  float64 `json:"elementalMastery"`
	EnergyRecharge float64 `json:"energyRecharge"`
	ATK            float64 `json:"atk"`
	HP             float64 `json:"hp"`
	ElementalDMG   float64 `json:"elementalDMG"`
}

// ConstraintCheck reports whether one requested min/max range was met. The
// display label isn't built here — it depends on the caller's UI language,
// so the frontend composes it client-side from Key/Min/Max (see
// STAT_LABEL/labelFor in the frontend's ResultsView).
type ConstraintCheck struct {
	Key   string   `json:"key"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Value float64  `json:"value"`
	Met   bool     `json:"met"`
}

// BuildResult is one ranked build in a solve response, or the character's
// real currently-equipped build when returned as SolveResponse.CurrentBuild.
type BuildResult struct {
	Rank       int        `json:"rank,omitempty"` // 0 for CurrentBuild, which isn't ranked among the search results
	CritValue  float64    `json:"critValue"`
	Pieces     []Artifact `json:"pieces"`
	Complete   bool       `json:"complete"` // true once Pieces covers all 5 slots — always true for a ranked build; may be false for CurrentBuild if a slot is bare
	OnSetCount int        `json:"onSetCount"`
	// OnSetCount2 is only meaningful in dual-set mode (SolveRequest.TargetSetKey2
	// non-empty) — the count of Pieces on TargetSetKey2.
	OnSetCount2 int               `json:"onSetCount2,omitempty"`
	Totals      BuildTotals       `json:"totals"`
	Checks      []ConstraintCheck `json:"checks"`
	AllMet      bool              `json:"allMet"`
}

// SolveResponse is returned from POST /api/solve.
type SolveResponse struct {
	Builds         []BuildResult `json:"builds"`
	CurrentBuild   *BuildResult  `json:"currentBuild,omitempty"` // the character's real, currently-equipped build, scored the same way, for a before/after comparison against Builds; nil if they have no artifacts equipped at all
	Meta           string        `json:"meta"`
	Reason         string        `json:"reason,omitempty"`
	SolveMS        int64         `json:"solveMs"`
	PrunedBranches int           `json:"prunedBranches"`
	TimedOut       bool          `json:"timedOut,omitempty"` // true if the search hit solver.maxSolveDuration before finishing
}
