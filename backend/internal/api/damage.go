package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"artifact-optimizer/internal/chardb"
	"artifact-optimizer/internal/damage"
	"artifact-optimizer/internal/enemydb"
	"artifact-optimizer/internal/model"
)

type talentMultiplierDTO struct {
	Label  string    `json:"label"`
	Values []float64 `json:"values"`
}

type characterTalentsDTO struct {
	Auto  []talentMultiplierDTO `json:"auto"`
	Skill []talentMultiplierDTO `json:"skill"`
	Burst []talentMultiplierDTO `json:"burst"`
}

func toTalentDTOs(ms []chardb.TalentMultiplier, lang string) []talentMultiplierDTO {
	out := make([]talentMultiplierDTO, len(ms))
	for i, m := range ms {
		out[i] = talentMultiplierDTO{Label: m.LabelFor(lang), Values: m.Values}
	}
	return out
}

// handleCharacterTalents returns a character's auto/skill/burst %ATK
// damage-multiplier curves (one value per talent level 1-15), for the
// damage calculator's component picker.
func (a *API) handleCharacterTalents(w http.ResponseWriter, r *http.Request) {
	lang := chardb.NormalizeLang(r.URL.Query().Get("lang"))
	key := chi.URLParam(r, "key")
	ref, known := chardb.Chars[key]
	if !known {
		writeError(w, http.StatusNotFound, "unknown character: "+key)
		return
	}
	writeJSON(w, http.StatusOK, characterTalentsDTO{
		Auto:  toTalentDTOs(ref.Talents.Auto, lang),
		Skill: toTalentDTOs(ref.Talents.Skill, lang),
		Burst: toTalentDTOs(ref.Talents.Burst, lang),
	})
}

type enemyPresetDTO struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Level    int     `json:"level"`
	ResPct   float64 `json:"resPct"`
	Category string  `json:"category"`
}

// handleEnemies returns the hand-curated enemy level/resistance presets
// (internal/enemydb) the damage calculator's enemy picker can offer —
// selecting one just pre-fills the level/resPct fields, it isn't looked up
// server-side during a damage calculation.
func (a *API) handleEnemies(w http.ResponseWriter, r *http.Request) {
	lang := chardb.NormalizeLang(r.URL.Query().Get("lang"))
	presets := enemydb.Sorted()
	out := make([]enemyPresetDTO, len(presets))
	for i, p := range presets {
		out[i] = enemyPresetDTO{Key: p.Key, Name: p.NameFor(lang), Level: p.Level, ResPct: p.ResPct, Category: p.Category}
	}
	writeJSON(w, http.StatusOK, out)
}

// damageRequestDTO is the wire shape for POST /api/damage. The client never
// sends a talent multiplier value directly — only which character/group/
// level/component indices it wants — so the server always resolves the
// actual %ATK numbers from chardb itself.
type damageRequestDTO struct {
	CharacterKey     string               `json:"characterKey"`
	CasterLevel      int                  `json:"casterLevel"`
	Totals           model.BuildTotals    `json:"totals"`
	TalentGroup      string               `json:"talentGroup"` // auto | skill | burst
	TalentLevel      int                  `json:"talentLevel"`
	ComponentIndices []int                `json:"componentIndices,omitempty"` // indices into the talent group; empty = all
	Extra            damage.ExtraBonus    `json:"extra"`
	Enemy            damage.EnemyInput    `json:"enemy"`
	Reaction         damage.ReactionInput `json:"reaction"`
}

// elementFromDmgKey turns a CharRef.DmgKey (e.g. "pyro_dmg_") into the bare
// element name ("pyro") the damage package uses to pick an amplifying
// reaction's multiplier row.
func elementFromDmgKey(dmgKey string) string {
	return strings.TrimSuffix(dmgKey, "_dmg_")
}

// handleDamage computes expected damage for a build against an enemy. See
// internal/damage for the formula and internal/chardb for where the talent
// %ATK multipliers come from.
func (a *API) handleDamage(w http.ResponseWriter, r *http.Request) {
	var req damageRequestDTO
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ref, known := chardb.Chars[req.CharacterKey]
	if !known {
		writeError(w, http.StatusNotFound, "unknown character: "+req.CharacterKey)
		return
	}
	var group []chardb.TalentMultiplier
	switch req.TalentGroup {
	case "auto":
		group = ref.Talents.Auto
	case "skill":
		group = ref.Talents.Skill
	case "burst":
		group = ref.Talents.Burst
	default:
		writeError(w, http.StatusBadRequest, "talentGroup must be auto, skill or burst")
		return
	}

	lang := chardb.NormalizeLang(r.URL.Query().Get("lang"))
	indices := req.ComponentIndices
	if len(indices) == 0 {
		indices = make([]int, len(group))
		for i := range group {
			indices[i] = i
		}
	}
	components := make([]damage.Multiplier, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(group) {
			writeError(w, http.StatusBadRequest, "componentIndices out of range")
			return
		}
		m := group[idx]
		components = append(components, damage.Multiplier{Label: m.LabelFor(lang), MultPctAtk: m.AtLevel(req.TalentLevel)})
	}

	resp := damage.Calculate(damage.Request{
		CasterLevel: req.CasterLevel,
		Element:     elementFromDmgKey(ref.DmgKey),
		Totals:      req.Totals,
		Components:  components,
		Extra:       req.Extra,
		Enemy:       req.Enemy,
		Reaction:    req.Reaction,
	})
	writeJSON(w, http.StatusOK, resp)
}
