// Package good parses GOOD (Genshin Open Object Description) exports.
//
// The format is stable at the top level, but per the project spec, tools in
// the ecosystem (Genshin Optimizer, Inventory Kamera, Irminsul, ...) vary in
// small ways: alternate field casing, an optional precomputed main-stat
// value, missing level/rarity on maxed pieces, null vs. omitted "location".
// Parse is written to tolerate those variations rather than reject them.
package good

import (
	"encoding/json"
	"fmt"
	"strings"

	"artifact-optimizer/internal/model"
)

// ParseError describes one artifact or character entry that couldn't be
// normalized. Parse collects these but only fails the whole import if
// nothing usable was found.
type ParseError struct {
	Index int
	Kind  string // "artifact" | "character"
	Msg   string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("%s[%d]: %s", e.Kind, e.Index, e.Msg)
}

// Result is a successfully parsed import, plus any per-entry issues that
// were tolerated (skipped) along the way.
type Result struct {
	Export model.GoodExport
	Issues []ParseError
}

// Parse normalizes raw GOOD JSON bytes into a model.GoodExport.
func Parse(raw []byte) (*Result, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}

	res := &Result{}
	res.Export.Format, _ = asString(doc["format"])
	if v, ok := asFloat(doc["version"]); ok {
		res.Export.Version = int(v)
	}
	res.Export.Source, _ = asString(doc["source"])

	if arr, ok := asArray(firstOf(doc, "artifacts", "Artifacts")); ok {
		for i, raw := range arr {
			a, err := parseArtifact(raw)
			if err != nil {
				res.Issues = append(res.Issues, ParseError{Index: i, Kind: "artifact", Msg: err.Error()})
				continue
			}
			a.ID = len(res.Export.Artifacts)
			res.Export.Artifacts = append(res.Export.Artifacts, *a)
		}
	}

	if arr, ok := asArray(firstOf(doc, "characters", "Characters")); ok {
		for i, raw := range arr {
			c, err := parseCharacter(raw)
			if err != nil {
				res.Issues = append(res.Issues, ParseError{Index: i, Kind: "character", Msg: err.Error()})
				continue
			}
			res.Export.Characters = append(res.Export.Characters, *c)
		}
	}

	if arr, ok := asArray(firstOf(doc, "weapons", "Weapons")); ok {
		for i, raw := range arr {
			w, err := parseWeapon(raw)
			if err != nil {
				res.Issues = append(res.Issues, ParseError{Index: i, Kind: "weapon", Msg: err.Error()})
				continue
			}
			w.ID = len(res.Export.Weapons)
			res.Export.Weapons = append(res.Export.Weapons, *w)
		}
	}

	if len(res.Export.Artifacts) == 0 && len(res.Export.Characters) == 0 {
		return nil, fmt.Errorf("no recognizable \"artifacts\" or \"characters\" array found")
	}
	return res, nil
}

func parseArtifact(raw any) (*model.Artifact, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not an object")
	}
	setKey, ok := asString(firstOf(m, "setKey", "artifactSetKey", "set"))
	if !ok || setKey == "" {
		return nil, fmt.Errorf("missing setKey")
	}
	slotKey, ok := asString(firstOf(m, "slotKey", "slot"))
	if !ok || slotKey == "" {
		return nil, fmt.Errorf("missing slotKey")
	}
	slotKey = strings.ToLower(slotKey)
	mainKeyRaw, ok := asString(firstOf(m, "mainStatKey", "mainStat"))
	if !ok || mainKeyRaw == "" {
		// Some tools nest {"mainStatKey":{"key":..,"value":..}}.
		if sub, ok := firstOf(m, "mainStatKey", "mainStat").(map[string]any); ok {
			mainKeyRaw, _ = asString(sub["key"])
		}
	}
	mainKey := normalizeStatKey(mainKeyRaw)
	if mainKey == "" {
		return nil, fmt.Errorf("missing mainStatKey")
	}

	level := 20
	if v, ok := asFloat(m["level"]); ok {
		level = int(v)
	}
	rarity := 5
	if v, ok := asFloat(m["rarity"]); ok {
		rarity = int(v)
	}

	mainVal, hasMainVal := asFloat(firstOf(m, "mainStatValue", "mainVal", "mainStatValue_"))
	if !hasMainVal {
		if sub, ok := firstOf(m, "mainStat", "mainStatKey").(map[string]any); ok {
			mainVal, hasMainVal = asFloat(sub["value"])
		}
	}
	if !hasMainVal {
		mainVal = MainStatValue(mainKey, rarity, level)
	}

	location, _ := asString(m["location"])
	lock, _ := m["lock"].(bool)

	var subs []model.Stat
	rawSubs := firstOf(m, "substats", "subStats", "subStat")
	if arr, ok := asArray(rawSubs); ok {
		for _, s := range arr {
			sm, ok := s.(map[string]any)
			if !ok {
				continue
			}
			key := normalizeStatKey(firstString(sm, "key", "stat", "statKey"))
			val, ok := asFloat(firstOf(sm, "value", "val"))
			if key == "" || !ok {
				continue
			}
			subs = append(subs, model.Stat{Key: key, Value: val})
		}
	}

	return &model.Artifact{
		SetKey: setKey, SlotKey: slotKey, Level: level, Rarity: rarity,
		MainStatKey: mainKey, MainStatValue: round1(mainVal),
		Location: location, Lock: lock, SubStats: subs,
	}, nil
}

func parseCharacter(raw any) (*model.Character, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not an object")
	}
	key, ok := asString(m["key"])
	if !ok || key == "" {
		return nil, fmt.Errorf("missing key")
	}
	c := &model.Character{Key: key}
	if v, ok := asFloat(m["level"]); ok {
		c.Level = int(v)
	}
	if v, ok := asFloat(m["ascension"]); ok {
		c.Ascension = int(v)
	}
	if v, ok := asFloat(m["constellation"]); ok {
		c.Constellation = int(v)
	}
	if t, ok := m["talent"].(map[string]any); ok {
		if v, ok := asFloat(t["auto"]); ok {
			c.Talent.Auto = int(v)
		}
		if v, ok := asFloat(t["skill"]); ok {
			c.Talent.Skill = int(v)
		}
		if v, ok := asFloat(t["burst"]); ok {
			c.Talent.Burst = int(v)
		}
	} else {
		// Flat variant some tools use: autoTalent/skillTalent/burstTalent.
		if v, ok := asFloat(m["autoTalent"]); ok {
			c.Talent.Auto = int(v)
		}
		if v, ok := asFloat(m["skillTalent"]); ok {
			c.Talent.Skill = int(v)
		}
		if v, ok := asFloat(m["burstTalent"]); ok {
			c.Talent.Burst = int(v)
		}
	}
	return c, nil
}

func parseWeapon(raw any) (*model.Weapon, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not an object")
	}
	key, ok := asString(m["key"])
	if !ok || key == "" {
		return nil, fmt.Errorf("missing key")
	}
	w := &model.Weapon{Key: key, Refinement: 1}
	if v, ok := asFloat(m["level"]); ok {
		w.Level = int(v)
	}
	if v, ok := asFloat(m["ascension"]); ok {
		w.Ascension = int(v)
	}
	if v, ok := asFloat(m["refinement"]); ok {
		w.Refinement = int(v)
	}
	w.Location, _ = asString(m["location"])
	w.Lock, _ = m["lock"].(bool)
	return w, nil
}

// statKeyAliases maps common non-standard spellings seen across tools to the
// canonical GOOD stat key.
var statKeyAliases = map[string]string{
	"critrate": "critRate_", "critrate_": "critRate_", "cr": "critRate_", "criticalrate": "critRate_",
	"critdmg": "critDMG_", "critdmg_": "critDMG_", "cd": "critDMG_", "criticaldmg": "critDMG_", "criticaldamage": "critDMG_",
	"atk%": "atk_", "atk_": "atk_", "atkpercent": "atk_", "hp%": "hp_", "hp_": "hp_", "hppercent": "hp_",
	"def%": "def_", "def_": "def_", "defpercent": "def_", "er": "enerRech_", "enerrech_": "enerRech_", "energyrecharge": "enerRech_",
	"elementalmastery": "em", "elemas": "em", "healingbonus": "heal_", "heal_": "heal_",
	// Flat stats: identity-mapped so a differently-cased spelling (e.g. "HP",
	// "ATK") still normalizes to the canonical lowercase key.
	"hp": "hp", "atk": "atk", "def": "def", "em": "em",
	// Elemental DMG% bonus, any casing of the element name.
	"pyrodmg_": "pyro_dmg_", "hydrodmg_": "hydro_dmg_", "cryodmg_": "cryo_dmg_", "electrodmg_": "electro_dmg_",
	"anemodmg_": "anemo_dmg_", "geodmg_": "geo_dmg_", "dendrodmg_": "dendro_dmg_", "physicaldmg_": "physical_dmg_",
}

// normalizeStatKey lowercases known aliases to the canonical GOOD key while
// leaving already-canonical keys (which are case-sensitive, e.g. "critRate_")
// untouched.
func normalizeStatKey(k string) string {
	k = strings.TrimSpace(k)
	if k == "" {
		return ""
	}
	if canon, ok := statKeyAliases[strings.ToLower(k)]; ok {
		return canon
	}
	return k
}

func firstOf(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstString(m map[string]any, keys ...string) string {
	s, _ := asString(firstOf(m, keys...))
	return s
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func asArray(v any) ([]any, bool) {
	a, ok := v.([]any)
	return a, ok
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
