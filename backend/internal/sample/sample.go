// Package sample generates a deterministic mock GOOD export, standing in
// for the "Load sample export" flow when no real inventory has been
// exported yet. It mirrors the shape (set/slot distribution, stat value
// pools) the design prototype used for its mock inventory.
package sample

import (
	"artifact-optimizer/internal/chardb"
	"artifact-optimizer/internal/model"
)

// rng is a small deterministic PRNG (mulberry32) so the sample export is
// stable across runs, matching the prototype's behavior.
type rng struct{ seed uint32 }

func newRNG(seed uint32) *rng { return &rng{seed: seed} }

func (r *rng) next() float64 {
	r.seed += 0x6D2B79F5
	t := r.seed
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64((t^(t>>14))>>0) / 4294967296
}

func (r *rng) pick(vals []float64) float64 {
	return vals[int(r.next()*float64(len(vals)))]
}

var valPools = map[string][]float64{
	"critRate_": {3.1, 3.5, 5.4, 6.2, 7.0, 10.5}, "critDMG_": {6.2, 7.8, 12.4, 14.0, 21.0, 24.1},
	"atk_": {4.7, 5.8, 9.3, 11.7, 15.6}, "hp_": {4.7, 5.8, 9.9, 14.6}, "def_": {5.8, 7.3, 13.1},
	"em": {16, 21, 40, 63}, "enerRech_": {5.2, 6.5, 11.0, 16.2}, "atk": {16, 19, 33, 49},
	"hp": {209, 239, 478, 568}, "def": {21, 44, 58},
}

var subKeys = []string{"critRate_", "critDMG_", "atk_", "hp_", "def_", "em", "enerRech_", "atk", "hp", "def"}

// Generate builds a deterministic sample GOOD export: the 3 reference
// characters plus a full artifact inventory spread across the 5 known sets.
func Generate() model.GoodExport {
	r := newRNG(20240630)

	mkSubs := func(mainKey string) []model.Stat {
		pool := make([]string, 0, len(subKeys)-1)
		for _, k := range subKeys {
			if k != mainKey {
				pool = append(pool, k)
			}
		}
		for i := len(pool) - 1; i > 0; i-- {
			j := int(r.next() * float64(i+1))
			pool[i], pool[j] = pool[j], pool[i]
		}
		subs := make([]model.Stat, 0, 4)
		for _, k := range pool[:4] {
			subs = append(subs, model.Stat{Key: k, Value: r.pick(valPools[k])})
		}
		return subs
	}

	var artifacts []model.Artifact
	add := func(setKey, slotKey, mainKey string, mainVal float64) {
		artifacts = append(artifacts, model.Artifact{
			SetKey: setKey, SlotKey: slotKey, Level: 20, Rarity: 5,
			MainStatKey: mainKey, MainStatValue: mainVal, SubStats: mkSubs(mainKey),
		})
	}

	elems := []string{"pyro_dmg_", "hydro_dmg_", "cryo_dmg_", "electro_dmg_"}
	for setKey := range chardb.KnownSets {
		for i := 0; i < 3; i++ {
			add(setKey, "flower", "hp", 4780)
		}
		for i := 0; i < 3; i++ {
			add(setKey, "plume", "atk", 311)
		}
		add(setKey, "sands", "em", 187)
		add(setKey, "sands", "atk_", 46.6)
		add(setKey, "sands", "enerRech_", 51.8)
		add(setKey, "sands", "hp_", 46.6)
		for _, e := range elems {
			add(setKey, "goblet", e, 46.6)
		}
		add(setKey, "goblet", "atk_", 46.6)
		add(setKey, "goblet", "em", 187)
		add(setKey, "goblet", "hp_", 46.6)
		add(setKey, "circlet", "critRate_", 31.1)
		add(setKey, "circlet", "critDMG_", 62.2)
		add(setKey, "circlet", "critDMG_", 62.2)
		add(setKey, "circlet", "atk_", 46.6)
		add(setKey, "circlet", "em", 187)
	}
	for i := range artifacts {
		artifacts[i].ID = i
	}

	characters := []model.Character{
		{Key: "HuTao", Level: 90, Ascension: 6, Constellation: 1, Talent: model.Talent{Auto: 9, Skill: 9, Burst: 9}},
		{Key: "RaidenShogun", Level: 90, Ascension: 6, Constellation: 0, Talent: model.Talent{Auto: 9, Skill: 9, Burst: 9}},
		{Key: "KamisatoAyaka", Level: 90, Ascension: 6, Constellation: 0, Talent: model.Talent{Auto: 9, Skill: 9, Burst: 9}},
	}

	return model.GoodExport{
		Format: "GOOD", Version: 2, Source: "Artifact Optimizer sample export",
		Characters: characters, Artifacts: artifacts,
	}
}
