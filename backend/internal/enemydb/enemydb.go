// Package enemydb holds a small, hand-curated table of common-enemy and
// boss level/resistance presets for the damage calculator's enemy-picker
// dropdown. Unlike internal/chardb, NONE of this data comes from genshin-db
// — that package's own enemy data (db.enemies()) exposes level-scaled HP/
// ATK/DEF but has no elemental-resistance field at all (its type definition
// literally comments it out as a TODO), so resistance values here are
// approximate community/wiki knowledge, not machine-generated. A preset
// only pre-fills the damage calculator's level + resPct inputs — enemy DEF
// itself is always derived from level alone (see internal/damage), never
// looked up here.
//
// Update this file by hand as better data becomes available; there is no
// regeneration script for it.
package enemydb

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed data/enemies.json
var dataFS embed.FS

// EnemyPreset is one hand-curated enemy the damage calculator's UI can
// offer as a level/resistance preset.
type EnemyPreset struct {
	Key      string
	Name     map[string]string // lang -> display name
	Level    int
	ResPct   float64
	Category string // "common" | "boss"
}

func (p EnemyPreset) NameFor(lang string) string {
	if v, ok := p.Name[lang]; ok && v != "" {
		return v
	}
	if v, ok := p.Name["en"]; ok && v != "" {
		return v
	}
	for _, v := range p.Name {
		return v
	}
	return ""
}

// Presets is the enemy preset table, keyed by key.
var Presets map[string]EnemyPreset

type generatedPreset struct {
	Key      string            `json:"key"`
	Name     map[string]string `json:"name"`
	Level    int               `json:"level"`
	ResPct   float64           `json:"resPct"`
	Category string            `json:"category"`
}

func init() {
	raw, err := dataFS.ReadFile("data/enemies.json")
	if err != nil {
		panic(fmt.Sprintf("enemydb: embedded data/enemies.json missing: %v", err))
	}
	var presets []generatedPreset
	if err := json.Unmarshal(raw, &presets); err != nil {
		panic(fmt.Sprintf("enemydb: malformed data/enemies.json: %v", err))
	}
	Presets = make(map[string]EnemyPreset, len(presets))
	for _, p := range presets {
		Presets[p.Key] = EnemyPreset{Key: p.Key, Name: p.Name, Level: p.Level, ResPct: p.ResPct, Category: p.Category}
	}
}

// Sorted returns every preset ordered by category (common before boss) then
// by level, for a stable UI listing.
func Sorted() []EnemyPreset {
	out := make([]EnemyPreset, 0, len(Presets))
	for _, p := range Presets {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].Level != out[j].Level {
			return out[i].Level < out[j].Level
		}
		return out[i].Key < out[j].Key
	})
	return out
}
