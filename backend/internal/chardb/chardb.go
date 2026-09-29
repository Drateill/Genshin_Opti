// Package chardb holds the reference dataset the V1 solver needs
// (per-character base stats, weapon types, known artifact sets, stat
// labels). Chars, Weapons and KnownSets are generated from genshin-db
// (https://github.com/theBowja/genshin-db), which tracks the live game's
// official data — see tools/gendata/generate.js at the repo root. Run
// `node generate.js` there (after `npm install`) to refresh
// internal/chardb/data/*.json to a newer game version; this package just
// embeds and loads whatever is in those files.
//
// A character or weapon absent from the generated data (because genshin-db
// itself doesn't have it yet, or its name didn't map cleanly to a GOOD key
// — see generate.js's goodKey()) is still listed in the UI from the raw
// import, just marked unsupported (RosterEntry.Known / WeaponOption.Known)
// rather than guessed.
package chardb

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed data/characters.json data/weapons.json data/sets.json
var dataFS embed.FS

// DefaultCritRate, DefaultCritDMG and DefaultEnergyRecharge are every
// character's un-ascended baseline for these three stats (5% / 50% / 100%)
// — CharRef.SpecializedValue is the *additional* ascension-4 bonus on top
// of these, not a replacement for them.
const (
	DefaultCritRate       = 5.0
	DefaultCritDMG        = 50.0
	DefaultEnergyRecharge = 100.0
)

// DefaultLang is used whenever a request names no language, or one this
// package has no data for.
const DefaultLang = "en"

// SupportedLangs are the locales generate.js produces display text for.
var SupportedLangs = map[string]bool{"en": true, "fr": true}

// NormalizeLang maps an arbitrary request lang value to one this package
// actually has data for, falling back to DefaultLang.
func NormalizeLang(lang string) string {
	if SupportedLangs[lang] {
		return lang
	}
	return DefaultLang
}

// localeText picks lang out of an {en, fr, ...} map, falling back to
// DefaultLang and then to whatever's present, so a locale genshin-db hasn't
// translated yet never renders as a blank string.
func localeText(m map[string]string, lang string) string {
	if v, ok := m[lang]; ok && v != "" {
		return v
	}
	if v, ok := m[DefaultLang]; ok && v != "" {
		return v
	}
	for _, v := range m {
		return v
	}
	return ""
}

// CharRef is the reference data for one character at level 90 / ascension
// 6 (their max, pre-artifact build), used to seed a build's totals before
// artifacts are added.
type CharRef struct {
	Key              string
	Name             map[string]string // lang -> display name
	Element          map[string]string // lang -> element display text (e.g. "Pyro")
	DmgKey           string            // e.g. "pyro_dmg_" — the elemental DMG% stat key this character scales with
	RecSet           string            // recommended default target set; empty when none is curated
	WeaponType       string            // sword | claymore | polearm | bow | catalyst
	Rarity           int               // 4 or 5
	SpecializedKey   string            // the stat key their ascension-4 passive bonus grants (e.g. "critDMG_", "atk_", "em")
	SpecializedValue float64
	BaseATK          float64
	BaseHP           float64
	Talents          CharTalents
	Icon             string // URL of this character's avatar icon, from genshin-db
}

func (r CharRef) NameFor(lang string) string    { return localeText(r.Name, lang) }
func (r CharRef) ElementFor(lang string) string { return localeText(r.Element, lang) }

// TalentMultiplier is one named %ATK damage-multiplier curve belonging to a
// combat talent (e.g. a normal attack's "1-Hit DMG"), one value per talent
// level 1-15. Extracted by tools/gendata/generate.js from genshin-db's
// talent attribute data.
type TalentMultiplier struct {
	Label  map[string]string // lang -> display name, e.g. "1-Hit DMG"
	Values []float64         // len 15, %ATK per talent level 1-15 (index 0 = level 1)
}

func (m TalentMultiplier) LabelFor(lang string) string { return localeText(m.Label, lang) }

// AtLevel returns the %ATK multiplier for the given talent level (1-15),
// clamping out-of-range levels to the nearest valid one.
func (m TalentMultiplier) AtLevel(level int) float64 {
	if len(m.Values) == 0 {
		return 0
	}
	if level < 1 {
		level = 1
	}
	if level > len(m.Values) {
		level = len(m.Values)
	}
	return m.Values[level-1]
}

// CharTalents holds a character's normal-attack, elemental-skill and
// elemental-burst damage-multiplier curves.
type CharTalents struct {
	Auto  []TalentMultiplier
	Skill []TalentMultiplier
	Burst []TalentMultiplier
}

// WeaponRef is the reference data for one weapon at level 90 / max
// ascension.
type WeaponRef struct {
	Key             string
	Name            map[string]string // lang -> display name
	Type            string            // sword | claymore | polearm | bow | catalyst
	Rarity          int               // 3, 4 or 5
	SubStatKey      string            // the secondary stat this weapon grants, empty for the rare weapon with none
	MaxATK          float64
	MaxSubStatValue float64
	Icon            string // URL of this weapon's icon, from genshin-db
}

func (r WeaponRef) NameFor(lang string) string { return localeText(r.Name, lang) }

// SetRef is the display name / short code / hover description for a known
// artifact set.
type SetRef struct {
	Key         string
	Name        map[string]string // lang -> display name
	Short       map[string]string // lang -> short code
	Description map[string]string // lang -> 2pc + 4pc set bonus text, shown on hover in the Configure UI
	Icon        string            // URL of one representative piece's icon, from genshin-db
}

func (r SetRef) NameFor(lang string) string        { return localeText(r.Name, lang) }
func (r SetRef) ShortFor(lang string) string       { return localeText(r.Short, lang) }
func (r SetRef) DescriptionFor(lang string) string { return localeText(r.Description, lang) }

// Chars is the reference roster, keyed by GOOD character key. A character
// present in a GOOD import but absent here can still be listed, just not
// solved (see RosterEntry.Known).
var Chars map[string]CharRef

// Weapons is the reference weapon roster, keyed by GOOD weapon key. An
// owned weapon absent here can still be picked in the UI, it just
// contributes nothing (see model.WeaponOption.Known).
var Weapons map[string]WeaponRef

// KnownSets are the artifact sets the Configure UI can show a display name
// and hover description for. Any set key seen in an import (including ones
// not listed here) is still counted in the import summary and offered as a
// target — just under its raw GOOD key (spaced out) instead of a curated
// name.
var KnownSets map[string]SetRef

type generatedTalentMultiplier struct {
	Label  map[string]string `json:"label"`
	Values []float64         `json:"values"`
}

type generatedTalents struct {
	Auto  []generatedTalentMultiplier `json:"auto"`
	Skill []generatedTalentMultiplier `json:"skill"`
	Burst []generatedTalentMultiplier `json:"burst"`
}

type generatedChar struct {
	Key              string            `json:"key"`
	Name             map[string]string `json:"name"`
	Element          map[string]string `json:"element"`
	WeaponType       string            `json:"weaponType"`
	Rarity           int               `json:"rarity"`
	DmgKey           string            `json:"dmgKey"`
	SpecializedKey   string            `json:"specializedKey"`
	SpecializedValue float64           `json:"specializedValue"`
	MaxATK           float64           `json:"maxATK"`
	MaxHP            float64           `json:"maxHP"`
	Talents          generatedTalents  `json:"talents"`
	Icon             string            `json:"icon"`
}

func toTalentMultipliers(gm []generatedTalentMultiplier) []TalentMultiplier {
	out := make([]TalentMultiplier, len(gm))
	for i, m := range gm {
		out[i] = TalentMultiplier{Label: m.Label, Values: m.Values}
	}
	return out
}

type generatedWeapon struct {
	Key             string            `json:"key"`
	Name            map[string]string `json:"name"`
	Type            string            `json:"type"`
	Rarity          int               `json:"rarity"`
	SubStatKey      string            `json:"subStatKey"`
	MaxATK          float64           `json:"maxATK"`
	MaxSubStatValue float64           `json:"maxSubStatValue"`
	Icon            string            `json:"icon"`
}

type generatedSet struct {
	Key         string            `json:"key"`
	Name        map[string]string `json:"name"`
	Short       map[string]string `json:"short"`
	Description map[string]string `json:"description"`
	Icon        string            `json:"icon"`
}

func init() {
	var chars []generatedChar
	mustLoad("data/characters.json", &chars)
	Chars = make(map[string]CharRef, len(chars))
	for _, c := range chars {
		Chars[c.Key] = CharRef{
			Key: c.Key, Name: c.Name, Element: c.Element, DmgKey: c.DmgKey, WeaponType: c.WeaponType, Rarity: c.Rarity,
			SpecializedKey: c.SpecializedKey, SpecializedValue: c.SpecializedValue,
			BaseATK: c.MaxATK, BaseHP: c.MaxHP, Icon: c.Icon,
			Talents: CharTalents{
				Auto:  toTalentMultipliers(c.Talents.Auto),
				Skill: toTalentMultipliers(c.Talents.Skill),
				Burst: toTalentMultipliers(c.Talents.Burst),
			},
		}
	}

	var weapons []generatedWeapon
	mustLoad("data/weapons.json", &weapons)
	Weapons = make(map[string]WeaponRef, len(weapons))
	for _, w := range weapons {
		Weapons[w.Key] = WeaponRef{
			Key: w.Key, Name: w.Name, Type: w.Type, Rarity: w.Rarity, SubStatKey: w.SubStatKey,
			MaxATK: w.MaxATK, MaxSubStatValue: w.MaxSubStatValue, Icon: w.Icon,
		}
	}

	var sets []generatedSet
	mustLoad("data/sets.json", &sets)
	KnownSets = make(map[string]SetRef, len(sets))
	for _, s := range sets {
		KnownSets[s.Key] = SetRef{Key: s.Key, Name: s.Name, Short: s.Short, Description: s.Description, Icon: s.Icon}
	}
}

func mustLoad(path string, out any) {
	raw, err := dataFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("chardb: embedded %s missing — run tools/gendata/generate.js: %v", path, err))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		panic(fmt.Sprintf("chardb: malformed %s: %v", path, err))
	}
}

// WeaponStatAt scales a weapon's ATK and secondary stat by level (1-90) —
// a simple linear approximation of the real per-ascension curve (the exact
// curve isn't embedded, only its level-90 endpoint from genshin-db); close
// enough to rank builds, matching the rest of this package's approach.
func WeaponStatAt(key string, level int) (atk float64, subKey string, subVal float64, ok bool) {
	ref, found := Weapons[key]
	if !found {
		return 0, "", 0, false
	}
	if level < 1 {
		level = 1
	}
	if level > 90 {
		level = 90
	}
	frac := float64(level) / 90
	return ref.MaxATK * frac, ref.SubStatKey, ref.MaxSubStatValue * frac, true
}

// SlotOrder is the fixed slot iteration order the solver and UI use.
var SlotOrder = []string{"flower", "plume", "sands", "goblet", "circlet"}
