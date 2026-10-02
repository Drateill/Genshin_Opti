// Package solver implements the branch-and-bound artifact build solver
// described in the spec: 5 fixed slots, filtered by target set and per-slot
// main-stat constraints, maximizing Crit Value (V1 objective) subject to
// hard minimum-stat constraints, returning the top N distinct builds.
//
// A 4-piece set bonus only needs 4 of the 5 pieces on-set, so the search
// branches once per candidate "free" slot (plus the all-5-on-set case) and
// keeps a single shared best-list across branches so pruning still works.
// Requesting a second target set (Request.TargetSetKey2) switches to dual-set
// mode instead: 2pc of each target set rather than 4pc of one, branching over
// which 2 (of 5) slots are pinned to the first set and which 2 (of the
// remaining 3) to the second, leaving 1 free.
package solver

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"artifact-optimizer/internal/chardb"
	"artifact-optimizer/internal/model"
)

type Solver struct {
	Inventory []model.Artifact
}

func New(inventory []model.Artifact) *Solver {
	return &Solver{Inventory: inventory}
}

// Progress is a concurrency-safe live view into an in-flight Solve call —
// share one pointer between the goroutine running Solve and whatever's
// polling it (see api.handleSolveProgress). Total is a fixed upper bound on
// the combinations Solve *could* visit (every 5-slot pick before pruning
// cuts a branch short); Tested only counts combinations pruning didn't
// skip, so it won't necessarily reach Total even once Solve returns — the
// caller should treat "done" as 100%, not Tested==Total.
type Progress struct {
	Tested atomic.Int64
	Total  atomic.Int64
}

type Request struct {
	CharacterKey string
	TargetSetKey string
	// TargetSetKey2, when non-empty, switches the search into dual-set mode:
	// it requires at least 2 pieces of TargetSetKey AND at least 2 of
	// TargetSetKey2 (instead of the default single-set "4 of 5" rule).
	TargetSetKey2   string
	WeaponATK       float64 // flat ATK contributed by the equipped weapon, added before %ATK, like real Base+Weapon ATK
	WeaponSubKey    string  // the equipped weapon's secondary stat key, empty if none/no weapon
	WeaponSubValue  float64
	SlotConstraints map[string][]string
	Constraints     map[string]model.StatRange // stat key -> min/max bounds on the finished build's total
	// Objective selects which stat Solve maximizes — see
	// model.SolveRequest.Objective's doc comment for the recognized values.
	// Empty/unrecognized behaves like "critValue".
	Objective string
	TopN      int
	Progress  *Progress // optional; when set, Solve reports live progress into it
	// IncludeEquippedByOthers, when false, excludes artifacts whose Location
	// is a character other than CharacterKey from consideration — only
	// unequipped pieces and CharacterKey's own gear remain eligible.
	IncludeEquippedByOthers bool
	// Lang is the UI language ("en" | "fr") for the Reason message built
	// when zero builds are found; empty/unrecognized falls back to English.
	Lang string
	// TeamElements holds 0-3 teammate elemental keys for resonance — see
	// resonanceBonus.
	TeamElements []string
}

// elementFromDmgKey derives a character's canonical element key (e.g.
// "Pyro") from their chardb.CharRef.DmgKey (e.g. "pyro_dmg_"), the same
// value resonanceBonus and the frontend's team-element picker key against.
func elementFromDmgKey(dmgKey string) string {
	el := strings.TrimSuffix(dmgKey, "_dmg_")
	if el == "" {
		return ""
	}
	return strings.ToUpper(el[:1]) + el[1:]
}

// resonanceBonus returns the flat stat bonuses granted by elemental
// resonance — 2+ of the 4 party members (the optimized character plus up to
// 3 teammates in Request.TeamElements) sharing an element — matching the
// real game's resonance system. Only the 4 resonances that feed a stat this
// solver already tracks are modeled (Pyro ATK%, Electro Energy Recharge%,
// Cryo CRIT Rate, Dendro Elemental Mastery); Hydro/Anemo/Geo resonance has
// no equivalent tracked stat and is ignored here (the frontend still shows
// them as informational).
func resonanceBonus(charElement string, teamElements []string) (atkPct, erPct, critRate, em float64) {
	counts := map[string]int{}
	if charElement != "" {
		counts[charElement]++
	}
	for _, el := range teamElements {
		if el != "" {
			counts[el]++
		}
	}
	if counts["Pyro"] >= 2 {
		atkPct = 25
	}
	if counts["Electro"] >= 2 {
		erPct = 25
	}
	if counts["Cryo"] >= 2 {
		critRate = 15
	}
	if counts["Dendro"] >= 2 {
		em = 30
	}
	return
}

func (s *Solver) pieceCV(a model.Artifact) float64 {
	cv := 0.0
	for _, sub := range a.SubStats {
		switch sub.Key {
		case "critRate_":
			cv += sub.Value * 2
		case "critDMG_":
			cv += sub.Value
		}
	}
	if a.SlotKey == "circlet" {
		switch a.MainStatKey {
		case "critRate_":
			cv += a.MainStatValue * 2
		case "critDMG_":
			cv += a.MainStatValue
		}
	}
	return cv
}

// objectives lists every Request.Objective value Solve recognizes besides
// the default "critValue" — see model.SolveRequest.Objective's doc comment.
var objectives = map[string]bool{
	"hp": true, "atk": true, "enerRech_": true, "em": true,
	"critRate_": true, "critDMG_": true, "elementalDmg": true,
}

// normalizeObjective falls back to "critValue" for empty/unrecognized input.
func normalizeObjective(o string) string {
	if objectives[o] {
		return o
	}
	return "critValue"
}

// pieceScore is one artifact's contribution toward the chosen Solve
// objective — the per-piece quantity the branch-and-bound search sums,
// sorts and prunes on. For "critValue" this is exactly pieceCV. For every
// other objective it's the piece's marginal contribution to the matching
// model.BuildTotals field: a %-based stat (atk_, hp_) is scaled by the
// character/weapon's fixed base so it's directly comparable to a flat
// contribution of the same stat, which keeps the score additive across
// pieces — required for the DFS's suffix-sum pruning bound to stay valid,
// exactly like pieceCV already is.
func (s *Solver) pieceScore(a model.Artifact, objective string, ref chardb.CharRef, req Request) float64 {
	switch objective {
	case "hp":
		return statValueOf(a, "hp") + ref.BaseHP/100*statValueOf(a, "hp_")
	case "atk":
		return statValueOf(a, "atk") + (ref.BaseATK+req.WeaponATK)/100*statValueOf(a, "atk_")
	case "enerRech_":
		return statValueOf(a, "enerRech_")
	case "em":
		return statValueOf(a, "em")
	case "critRate_":
		return statValueOf(a, "critRate_")
	case "critDMG_":
		return statValueOf(a, "critDMG_")
	case "elementalDmg":
		return statValueOf(a, ref.DmgKey)
	default:
		return s.pieceCV(a)
	}
}

func (s *Solver) totalsFor(pieces []model.Artifact, ref chardb.CharRef, req Request) model.BuildTotals {
	sums := map[string]float64{}
	add := func(k string, v float64) { sums[k] += v }
	if req.WeaponSubKey != "" {
		add(req.WeaponSubKey, req.WeaponSubValue)
	}
	if ref.SpecializedKey != "" {
		add(ref.SpecializedKey, ref.SpecializedValue)
	}
	for _, a := range pieces {
		add(a.MainStatKey, a.MainStatValue)
		for _, sub := range a.SubStats {
			add(sub.Key, sub.Value)
		}
	}
	resAtk, resEr, resCr, resEm := resonanceBonus(elementFromDmgKey(ref.DmgKey), req.TeamElements)
	return model.BuildTotals{
		CritRate:       sums["critRate_"] + chardb.DefaultCritRate + resCr,
		CritDMG:        sums["critDMG_"] + chardb.DefaultCritDMG,
		ElementMaster:  sums["em"] + resEm,
		EnergyRecharge: sums["enerRech_"] + chardb.DefaultEnergyRecharge + resEr,
		ATK:            (ref.BaseATK+req.WeaponATK)*(1+(sums["atk_"]+resAtk)/100) + sums["atk"],
		HP:             ref.BaseHP*(1+sums["hp_"]/100) + sums["hp"],
		ElementalDMG:   sums[ref.DmgKey],
	}
}

func valueForKey(t model.BuildTotals, dmgKey, key string) (float64, bool) {
	switch key {
	case "critRate_":
		return t.CritRate, true
	case "critDMG_":
		return t.CritDMG, true
	case "em":
		return t.ElementMaster, true
	case "enerRech_":
		return t.EnergyRecharge, true
	case "atk":
		return t.ATK, true
	case "hp":
		return t.HP, true
	case dmgKey:
		return t.ElementalDMG, true
	}
	if strings.HasSuffix(key, "_dmg_") {
		return t.ElementalDMG, true
	}
	return 0, false
}

func (s *Solver) buildChecks(t model.BuildTotals, dmgKey string, constraints map[string]model.StatRange) ([]model.ConstraintCheck, bool) {
	keys := make([]string, 0, len(constraints))
	for k := range constraints {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	allMet := true
	checks := make([]model.ConstraintCheck, 0, len(keys))
	for _, k := range keys {
		rng := constraints[k]
		cur, _ := valueForKey(t, dmgKey, k)
		met := true
		if rng.Min != nil && cur < *rng.Min {
			met = false
		}
		if rng.Max != nil && cur > *rng.Max {
			met = false
		}
		if !met {
			allMet = false
		}
		checks = append(checks, model.ConstraintCheck{
			Key: k, Min: rng.Min, Max: rng.Max, Value: round1(cur), Met: met,
		})
	}
	return checks, allMet
}

type candidate struct {
	a     model.Artifact
	score float64 // this piece's contribution under the active Solve objective — see pieceScore
}

const (
	// maxCandidatesPerSlot bounds how many artifacts of one slot the DFS
	// considers. Without this, a slot with hundreds of eligible pieces
	// (common on a real, well-worn inventory) makes the search space blow
	// up combinatorially — 5 slots × hundreds of pieces each is billions of
	// combinations, which is what made real-inventory solves take minutes.
	// Instead of every eligible piece, each slot keeps only the ones that
	// could plausibly matter: the highest Crit Value pieces (what's being
	// optimized) plus the highest pieces on each stat under a min/max
	// constraint, so a low-CV piece needed to hit an EM/ER/etc. constraint
	// isn't silently discarded. A slot with fewer candidates than this is
	// returned unchanged — the cap only kicks in where it's actually
	// shrinking the search.
	maxCandidatesPerSlot = 30
	// maxSolveDuration is a hard ceiling on how long one Solve call keeps
	// searching. Past it, the search stops expanding new branches and
	// returns the best builds found so far (SolveResponse.TimedOut is set)
	// rather than risk an effectively unbounded wait on a pathological
	// inventory/constraint combination.
	maxSolveDuration = 15 * time.Second
)

// relatedStatKeys returns the artifact stat keys that feed into a build's
// total for the given constraint key — almost always just the key itself,
// except "atk" (total ATK), which real artifacts reach via both a flat atk
// substat and an atk_% substat.
func relatedStatKeys(key string) []string {
	if key == "atk" {
		return []string{"atk", "atk_"}
	}
	return []string{key}
}

func statValueOf(a model.Artifact, key string) float64 {
	v := 0.0
	if a.MainStatKey == key {
		v += a.MainStatValue
	}
	for _, sub := range a.SubStats {
		if sub.Key == key {
			v += sub.Value
		}
	}
	return v
}

func capCandidates(all []candidate, constraints map[string]model.StatRange, limit int) []candidate {
	if len(all) <= limit {
		return all
	}
	keep := make(map[int]candidate, limit*2)
	keepTop := func(by func(candidate) float64, n int) {
		byKey := append([]candidate{}, all...)
		sort.Slice(byKey, func(i, j int) bool { return by(byKey[i]) > by(byKey[j]) })
		for i := 0; i < n && i < len(byKey); i++ {
			keep[byKey[i].a.ID] = byKey[i]
		}
	}
	keepTop(func(c candidate) float64 { return c.score }, limit)
	for key := range constraints {
		for _, k := range relatedStatKeys(key) {
			keepTop(func(c candidate) float64 { return statValueOf(c.a, k) }, limit/2)
		}
	}
	out := make([]candidate, 0, len(keep))
	for _, c := range keep {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// pairIndices returns every 2-element combination of {0,...,n-1}, e.g.
// pairIndices(3) -> [{0,1} {0,2} {1,2}].
func pairIndices(n int) [][2]int {
	var out [][2]int
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			out = append(out, [2]int{i, j})
		}
	}
	return out
}

// branchRoles enumerates every way to assign each of order's slots a
// required set ("" = free, any set) for one Solve call, mirroring how many
// pieces each target set needs on-set:
//
//   - Single-set mode (TargetSetKey2 == ""): the original 6 branches — every
//     slot pinned to TargetSetKey (the "all 5 on-set" case), plus one branch
//     per choice of a single free slot (the "4 on-set + 1 free" case). Trying
//     both, rather than only the minimal 4-pinned pattern, guards against
//     maxCandidatesPerSlot capping a good on-set piece out of the free
//     slot's (unfiltered) candidate pool.
//   - Dual-set mode: 2 of the 5 slots pinned to TargetSetKey, 2 of the
//     remaining 3 pinned to TargetSetKey2, the last free — 10*3 = 30
//     branches. Unlike single-set mode this only tries the minimal 2+2+1
//     pinning, not every over-pinned variant: a 2pc requirement is far less
//     likely to be starved by the per-slot candidate cap than a 4pc one, so
//     the extra branches wouldn't meaningfully change the result.
func branchRoles(req Request, order []string) [][]string {
	n := len(order)
	if req.TargetSetKey2 == "" {
		var out [][]string
		for _, freeIdx := range []int{-1, 0, 1, 2, 3, 4} {
			roles := make([]string, n)
			for i := range order {
				if i != freeIdx {
					roles[i] = req.TargetSetKey
				}
			}
			out = append(out, roles)
		}
		return out
	}

	var out [][]string
	for _, aPair := range pairIndices(n) {
		aSet := map[int]bool{aPair[0]: true, aPair[1]: true}
		var remaining []int
		for i := 0; i < n; i++ {
			if !aSet[i] {
				remaining = append(remaining, i)
			}
		}
		for _, bPair := range pairIndices(len(remaining)) {
			roles := make([]string, n)
			roles[aPair[0]] = req.TargetSetKey
			roles[aPair[1]] = req.TargetSetKey
			roles[remaining[bPair[0]]] = req.TargetSetKey2
			roles[remaining[bPair[1]]] = req.TargetSetKey2
			out = append(out, roles)
		}
	}
	return out
}

// Solve runs the branch-and-bound search and returns the top-N builds.
func (s *Solver) Solve(req Request) (model.SolveResponse, error) {
	ref, ok := chardb.Chars[req.CharacterKey]
	if !ok {
		return model.SolveResponse{}, fmt.Errorf("unknown or unsupported character %q", req.CharacterKey)
	}
	order := chardb.SlotOrder
	topN := req.TopN
	if topN <= 0 {
		topN = 5
	}
	objective := normalizeObjective(req.Objective)

	t0 := time.Now()
	pruned, rejected, leafCount := 0, 0, 0
	stopped := false

	// slotCands lists a slot's eligible candidates, optionally pinned to one
	// required set — requiredSetKey == "" means any set is fine (today's
	// "anySet"/free-slot case).
	slotCands := func(slot string, requiredSetKey string) []candidate {
		allow := req.SlotConstraints[slot]
		var out []candidate
		for _, a := range s.Inventory {
			if a.SlotKey != slot {
				continue
			}
			if requiredSetKey != "" && a.SetKey != requiredSetKey {
				continue
			}
			if !req.IncludeEquippedByOthers && a.Location != "" && a.Location != req.CharacterKey {
				continue
			}
			if len(allow) > 0 && !contains(allow, a.MainStatKey) {
				continue
			}
			out = append(out, candidate{a: a, score: s.pieceScore(a, objective, ref, req)})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].score > out[j].score })
		return capCandidates(out, req.Constraints, maxCandidatesPerSlot)
	}

	seen := map[string]bool{}
	type scored struct {
		score  float64
		cv     float64 // the build's real Crit Value, always tracked for display regardless of objective
		pieces []model.Artifact
	}
	var best []scored

	worst := func() float64 {
		if len(best) < topN {
			return math.Inf(-1)
		}
		return best[len(best)-1].score
	}

	consider := func(score float64, pick []model.Artifact) {
		if req.Progress != nil {
			req.Progress.Tested.Add(1)
		}
		leafCount++
		if leafCount%4096 == 0 && time.Since(t0) > maxSolveDuration {
			stopped = true
		}
		onSet1, onSet2 := countOnSet(pick, req.TargetSetKey, req.TargetSetKey2)
		if req.TargetSetKey2 != "" {
			if onSet1 < 2 || onSet2 < 2 {
				return
			}
		} else if onSet1 < 4 {
			return
		}
		ids := make([]int, len(pick))
		for i, p := range pick {
			ids[i] = p.ID
		}
		sort.Ints(ids)
		sig := fmt.Sprint(ids)
		if seen[sig] {
			return
		}
		seen[sig] = true
		totals := s.totalsFor(pick, ref, req)
		_, allMet := s.buildChecks(totals, ref.DmgKey, req.Constraints)
		if !allMet {
			rejected++
			return
		}
		cv := 0.0
		for _, p := range pick {
			cv += s.pieceCV(p)
		}
		pieces := append([]model.Artifact{}, pick...)
		best = append(best, scored{score, cv, pieces})
		sort.Slice(best, func(i, j int) bool { return best[i].score > best[j].score })
		if len(best) > topN {
			best = best[:topN]
		}
	}

	for _, roles := range branchRoles(req, order) {
		if stopped {
			break
		}
		cands := map[string][]candidate{}
		viable := true
		for i, slot := range order {
			cands[slot] = slotCands(slot, roles[i])
			if len(cands[slot]) == 0 {
				viable = false
			}
		}
		if !viable {
			continue
		}
		if req.Progress != nil {
			branchTotal := int64(1)
			for _, slot := range order {
				branchTotal *= int64(len(cands[slot]))
			}
			req.Progress.Total.Add(branchTotal)
		}
		suffix := make([]float64, len(order))
		acc := 0.0
		for i := len(order) - 1; i >= 0; i-- {
			acc += cands[order[i]][0].score
			suffix[i] = acc
		}
		var pick []model.Artifact
		var dfs func(i int, pscore float64)
		dfs = func(i int, pscore float64) {
			if stopped {
				return
			}
			if i == len(order) {
				consider(pscore, pick)
				return
			}
			if pscore+suffix[i] <= worst() {
				pruned++
				return
			}
			for _, e := range cands[order[i]] {
				pick = append(pick, e.a)
				dfs(i+1, pscore+e.score)
				pick = pick[:len(pick)-1]
			}
		}
		dfs(0, 0)
	}

	ms := time.Since(t0).Milliseconds()
	if len(best) == 0 {
		reason := noFeasibleReason(req.Lang, rejected, req.IncludeEquippedByOthers, stopped, req.TargetSetKey2 != "")
		return model.SolveResponse{Meta: "no feasible builds", Reason: reason, SolveMS: ms, PrunedBranches: pruned, TimedOut: stopped}, nil
	}

	results := make([]model.BuildResult, len(best))
	for i, b := range best {
		totals := s.totalsFor(b.pieces, ref, req)
		checks, allMet := s.buildChecks(totals, ref.DmgKey, req.Constraints)
		onSet1, onSet2 := countOnSet(b.pieces, req.TargetSetKey, req.TargetSetKey2)
		results[i] = model.BuildResult{
			Rank: i + 1, CritValue: round1(b.cv), Pieces: b.pieces, Complete: true,
			OnSetCount: onSet1, OnSetCount2: onSet2,
			Totals: totals, Checks: checks, AllMet: allMet,
		}
	}
	meta := fmt.Sprintf("%d results · solved in %d ms · %d branches pruned", len(results), ms, pruned)
	if stopped {
		meta += fmt.Sprintf(" · capped at %s, may not be exhaustive", maxSolveDuration)
	}
	return model.SolveResponse{Builds: results, Meta: meta, SolveMS: ms, PrunedBranches: pruned, TimedOut: stopped}, nil
}

// Evaluate scores a fixed, already-chosen set of pieces the same way Solve
// scores a candidate combination — used to turn a character's real
// currently-equipped artifacts into a BuildResult directly comparable
// against the search results, without running them through the search
// itself (Evaluate doesn't enforce Solve's on-set-count-≥4 gate, since a
// real current build is often off the target set entirely).
func (s *Solver) Evaluate(pieces []model.Artifact, req Request) (model.BuildResult, error) {
	ref, ok := chardb.Chars[req.CharacterKey]
	if !ok {
		return model.BuildResult{}, fmt.Errorf("unknown or unsupported character %q", req.CharacterKey)
	}
	cv := 0.0
	for _, p := range pieces {
		cv += s.pieceCV(p)
	}
	onSet1, onSet2 := countOnSet(pieces, req.TargetSetKey, req.TargetSetKey2)
	totals := s.totalsFor(pieces, ref, req)
	checks, allMet := s.buildChecks(totals, ref.DmgKey, req.Constraints)
	return model.BuildResult{
		CritValue: round1(cv), Pieces: pieces, Complete: len(pieces) == len(chardb.SlotOrder),
		OnSetCount: onSet1, OnSetCount2: onSet2, Totals: totals, Checks: checks, AllMet: allMet,
	}, nil
}

// countOnSet counts pieces on setKey1 and, when setKey2 is non-empty, on
// setKey2 too (used for both the single-set "≥4" gate and the dual-set
// "≥2 and ≥2" gate).
func countOnSet(pieces []model.Artifact, setKey1, setKey2 string) (int, int) {
	c1, c2 := 0, 0
	for _, p := range pieces {
		if p.SetKey == setKey1 {
			c1++
		}
		if setKey2 != "" && p.SetKey == setKey2 {
			c2++
		}
	}
	return c1, c2
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// noFeasibleReason builds SolveResponse.Reason's human-readable explanation
// for why zero builds came back, in the caller's UI language — this is the
// one piece of free text the solver still produces itself (everything else
// in the response is plain data the frontend localizes), since it depends
// on solver-internal state (rejected count, the timeout constant) that
// isn't otherwise exposed.
func noFeasibleReason(lang string, rejected int, includeEquippedByOthers, stopped, dualMode bool) string {
	fr := chardb.NormalizeLang(lang) == "fr"
	var reason string
	if rejected > 0 {
		if fr {
			reason = fmt.Sprintf("%d builds complets ont été trouvés, mais aucun ne respecte toutes les contraintes min/max. Assouplissez-en une et relancez la recherche.", rejected)
		} else {
			reason = fmt.Sprintf("%d complete builds were found, but none met every min/max constraint. Loosen one and solve again.", rejected)
		}
	} else {
		if fr {
			target := "cet ensemble"
			if dualMode {
				target = "ces ensembles"
			}
			reason = "Aucun artéfact de l'inventaire ne satisfait tous les filtres de stat principale pour " + target + ". Assouplissez un filtre d'emplacement et relancez la recherche."
			if !includeEquippedByOthers {
				reason += " Essayez d'activer « inclure les artéfacts équipés par d'autres personnages »."
			}
		} else {
			target := "this set"
			if dualMode {
				target = "these sets"
			}
			reason = "No artifacts in the inventory satisfy every slot main-stat filter for " + target + ". Loosen a slot filter and solve again."
			if !includeEquippedByOthers {
				reason += " Try enabling \"include artifacts worn by other characters\"."
			}
		}
	}
	if stopped {
		if fr {
			reason += fmt.Sprintf(" (la recherche a été limitée à %s avant de terminer — essayez un filtre d'emplacement plus étroit.)", maxSolveDuration)
		} else {
			reason += fmt.Sprintf(" (search was capped at %s before finishing — try a narrower slot filter.)", maxSolveDuration)
		}
	}
	return reason
}
