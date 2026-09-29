// Package api wires the HTTP endpoints described in the spec
// (POST /api/import, GET /api/characters, GET /api/artifacts/sets,
// POST /api/solve) plus a GET /api/sample helper for the "Load sample
// export" flow.
package api

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"

	"github.com/go-chi/chi/v5"

	"artifact-optimizer/internal/chardb"
	"artifact-optimizer/internal/good"
	"artifact-optimizer/internal/model"
	"artifact-optimizer/internal/sample"
	"artifact-optimizer/internal/store"
)

type API struct {
	store *store.Store
}

func New(s *store.Store) *API {
	return &API{store: s}
}

func (a *API) Routes(r chi.Router) {
	r.Post("/api/import", a.handleImport)
	r.Get("/api/sample", a.handleSample)
	r.Get("/api/characters", a.handleCharacters)
	r.Get("/api/artifacts/sets", a.handleSets)
	r.Get("/api/weapons", a.handleWeapons)
	r.Post("/api/solve", a.handleSolveStart)
	r.Get("/api/solve/{id}/progress", a.handleSolveProgress)
	r.Get("/api/characters/{key}/talents", a.handleCharacterTalents)
	r.Get("/api/enemies", a.handleEnemies)
	r.Post("/api/damage", a.handleDamage)
}

func (a *API) handleImport(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read request body")
		return
	}
	res, err := good.Parse(raw)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "couldn't parse GOOD export: "+err.Error())
		return
	}
	a.store.Set(res.Export)
	writeJSON(w, http.StatusOK, summaryFor(res.Export))
}

func (a *API) handleSample(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sample.Generate())
}

func (a *API) handleCharacters(w http.ResponseWriter, r *http.Request) {
	lang := chardb.NormalizeLang(r.URL.Query().Get("lang"))
	export := a.store.Get()
	equipped := map[string]model.Weapon{}
	for _, wp := range export.Weapons {
		if wp.Location != "" {
			equipped[wp.Location] = wp
		}
	}
	entries := make([]model.RosterEntry, 0, len(export.Characters))
	for _, c := range export.Characters {
		ref, known := chardb.Chars[c.Key]
		entry := model.RosterEntry{Key: c.Key, Level: c.Level, Constellation: c.Constellation, Talent: c.Talent, Known: known}
		if known {
			entry.Name = ref.NameFor(lang)
			entry.Element = ref.ElementFor(lang)
			entry.RecSet = ref.RecSet
			entry.DmgKey = ref.DmgKey
			entry.WeaponType = ref.WeaponType
			entry.Icon = ref.Icon
			entry.Rarity = ref.Rarity
		} else {
			entry.Name = spaceOutKey(c.Key)
		}
		if wp, ok := equipped[c.Key]; ok {
			if wref, known := chardb.Weapons[wp.Key]; known {
				entry.EquippedWeapon, entry.EquippedWeaponKnown = wref.NameFor(lang), true
			} else {
				entry.EquippedWeapon = spaceOutKey(wp.Key)
			}
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Known != entries[j].Known {
			return entries[i].Known
		}
		return entries[i].Name < entries[j].Name
	})
	writeJSON(w, http.StatusOK, entries)
}

func (a *API) handleSets(w http.ResponseWriter, r *http.Request) {
	lang := chardb.NormalizeLang(r.URL.Query().Get("lang"))
	export := a.store.Get()
	counts := map[string]map[string]int{}
	for _, art := range export.Artifacts {
		if counts[art.SetKey] == nil {
			counts[art.SetKey] = map[string]int{}
		}
		counts[art.SetKey][art.SlotKey]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]model.SetInfo, 0, len(keys))
	for _, k := range keys {
		ref, known := chardb.KnownSets[k]
		info := model.SetInfo{Key: k, Counts: counts[k]}
		if known {
			info.Name, info.Short, info.Description = ref.NameFor(lang), ref.ShortFor(lang), ref.DescriptionFor(lang)
			info.Icon = ref.Icon
		} else {
			info.Name, info.Short = spaceOutKey(k), shortCode(k)
		}
		out = append(out, info)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleWeapons lists the player's distinct owned weapons that we have
// reference data for (chardb.Weapons), optionally filtered to one weapon
// type via ?type=sword|claymore|polearm|bow|catalyst. A weapon we don't
// have data for is left out entirely — we can't even place it in the right
// type bucket without knowing its type, so unlike characters/sets it can't
// be shown as a disabled "unsupported" row here.
//
// Owning several copies of the same weapon (common — spare 4-stars,
// duplicate wishes) only offers the single best copy (highest level, then
// highest refinement); WeaponOption.Count says how many copies that
// collapsed.
func (a *API) handleWeapons(w http.ResponseWriter, r *http.Request) {
	lang := chardb.NormalizeLang(r.URL.Query().Get("lang"))
	export := a.store.Get()
	typeFilter := r.URL.Query().Get("type")

	best := map[string]model.Weapon{}
	counts := map[string]int{}
	for _, wp := range export.Weapons {
		ref, known := chardb.Weapons[wp.Key]
		if !known || (typeFilter != "" && ref.Type != typeFilter) {
			continue
		}
		counts[wp.Key]++
		cur, seen := best[wp.Key]
		if !seen || wp.Level > cur.Level || (wp.Level == cur.Level && wp.Refinement > cur.Refinement) {
			best[wp.Key] = wp
		}
	}

	out := make([]model.WeaponOption, 0, len(best))
	for key, wp := range best {
		ref := chardb.Weapons[key]
		atk, subKey, subVal, _ := chardb.WeaponStatAt(wp.Key, wp.Level)
		out = append(out, model.WeaponOption{
			ID: wp.ID, Key: wp.Key, Name: ref.NameFor(lang), Type: ref.Type, Rarity: ref.Rarity,
			Level: wp.Level, Refinement: wp.Refinement, Count: counts[key],
			ATK: round1(atk), SubStatKey: subKey, SubStatValue: round1(subVal), Icon: ref.Icon, Known: true,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rarity != out[j].Rarity {
			return out[i].Rarity > out[j].Rarity
		}
		return out[i].Name < out[j].Name
	})
	writeJSON(w, http.StatusOK, out)
}

func summaryFor(export model.GoodExport) model.ImportSummary {
	setSeen := map[string]bool{}
	for _, a := range export.Artifacts {
		setSeen[a.SetKey] = true
	}
	keys := make([]string, 0, len(setSeen))
	for k := range setSeen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return model.ImportSummary{
		Characters: len(export.Characters), Artifacts: len(export.Artifacts),
		Sets: len(keys), SetKeys: keys,
	}
}

var (
	pascalAcronymBoundary = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	pascalWordBoundary    = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

// spaceOutKey turns a raw GOOD PascalCase key (e.g. "KamisatoAyaka") into a
// readable fallback display name ("Kamisato Ayaka") for a character or set
// we don't have curated display data for yet — otherwise the UI would show
// the concatenated key with no spaces at all.
func spaceOutKey(key string) string {
	s := pascalAcronymBoundary.ReplaceAllString(key, "$1 $2")
	s = pascalWordBoundary.ReplaceAllString(s, "$1 $2")
	return s
}

func shortCode(setKey string) string {
	if len(setKey) <= 2 {
		return setKey
	}
	out := []byte{setKey[0]}
	for i := 1; i < len(setKey); i++ {
		if setKey[i] >= 'A' && setKey[i] <= 'Z' {
			out = append(out, setKey[i])
		}
	}
	if len(out) < 2 {
		return setKey[:2]
	}
	if len(out) > 2 {
		out = out[:2]
	}
	return string(out)
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
