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
	Objective       string               `json:"objective"`          // "critValue" (V1) | "dmg" (V2, not implemented)
	TopN            int                  `json:"topN"`
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
