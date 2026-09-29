package solver

import (
	"testing"
	"time"

	"artifact-optimizer/internal/model"
)

func mkArt(id int, setKey, slot, mainKey string, mainVal float64, subs ...model.Stat) model.Artifact {
	return model.Artifact{
		ID: id, SetKey: setKey, SlotKey: slot, Level: 20, Rarity: 5,
		MainStatKey: mainKey, MainStatValue: mainVal, SubStats: subs,
	}
}

func stat(key string, val float64) model.Stat { return model.Stat{Key: key, Value: val} }

// baseInventory returns one on-set piece per slot for "SetA", each
// contributing a modest, easy-to-reason-about Crit Value.
func baseInventory() []model.Artifact {
	return []model.Artifact{
		mkArt(1, "SetA", "flower", "hp", 4780, stat("critRate_", 2.5)),     // cv 5
		mkArt(2, "SetA", "plume", "atk", 311, stat("critDMG_", 5)),         // cv 5
		mkArt(3, "SetA", "sands", "em", 187, stat("critRate_", 2.5)),       // cv 5
		mkArt(4, "SetA", "goblet", "pyro_dmg_", 46.6, stat("critDMG_", 5)), // cv 5
		mkArt(5, "SetA", "circlet", "critRate_", 10, stat("critDMG_", 5)),  // main 20 + sub 5 = cv 25
	}
}

func TestSolve_BasicOnSetBuild(t *testing.T) {
	s := New(baseInventory())
	res, err := s.Solve(Request{CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) != 1 {
		t.Fatalf("expected exactly 1 feasible build from a single candidate per slot, got %d", len(res.Builds))
	}
	b := res.Builds[0]
	wantCV := 5.0 + 5.0 + 5.0 + 5.0 + 25.0
	if b.CritValue != wantCV {
		t.Errorf("expected CV %v, got %v", wantCV, b.CritValue)
	}
	if b.OnSetCount != 5 {
		t.Errorf("expected all 5 pieces on-set, got %d", b.OnSetCount)
	}
}

func TestSolve_PrefersOffSetWhenHigherValue(t *testing.T) {
	inv := baseInventory()
	// A much better flower is available, but from a different set — only
	// usable if the solver branches over "flower is the free slot".
	inv = append(inv, mkArt(6, "SetB", "flower", "hp", 4780, stat("critRate_", 20))) // cv 40
	s := New(inv)
	res, err := s.Solve(Request{CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) == 0 {
		t.Fatal("expected at least one feasible build")
	}
	top := res.Builds[0]
	if top.OnSetCount != 4 {
		t.Fatalf("expected the best build to use the 4+1 off-set flower, got onSetCount=%d", top.OnSetCount)
	}
	var flower model.Artifact
	for _, p := range top.Pieces {
		if p.SlotKey == "flower" {
			flower = p
		}
	}
	if flower.SetKey != "SetB" {
		t.Errorf("expected the off-set SetB flower to be chosen for its higher CV, got setKey=%s", flower.SetKey)
	}
	wantCV := 40.0 + 5.0 + 5.0 + 5.0 + 25.0
	if top.CritValue != wantCV {
		t.Errorf("expected CV %v, got %v", wantCV, top.CritValue)
	}
}

func TestSolve_RequiresAtLeastFourOnSetPieces(t *testing.T) {
	// Two slots (flower, plume) only have off-set alternatives — a valid
	// build can have at most 3 on-set pieces, which must never qualify.
	inv := []model.Artifact{
		mkArt(1, "SetB", "flower", "hp", 4780),
		mkArt(2, "SetB", "plume", "atk", 311),
		mkArt(3, "SetA", "sands", "em", 187),
		mkArt(4, "SetA", "goblet", "pyro_dmg_", 46.6),
		mkArt(5, "SetA", "circlet", "critRate_", 10),
	}
	s := New(inv)
	res, err := s.Solve(Request{CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) != 0 {
		t.Fatalf("expected no feasible build (max 3 on-set pieces available), got %d", len(res.Builds))
	}
}

func TestSolve_MinConstraintsAreHardGates(t *testing.T) {
	s := New(baseInventory())
	impossible := 9999.0
	res, err := s.Solve(Request{
		CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5,
		Constraints: map[string]model.StatRange{"critRate_": {Min: &impossible}},
	})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) != 0 {
		t.Fatalf("expected the impossible min constraint to reject every build, got %d results", len(res.Builds))
	}
	if res.Reason == "" {
		t.Error("expected a non-empty reason when the only cause is failed constraints")
	}
}

func TestSolve_SlotMainStatFilterCanEliminateEverything(t *testing.T) {
	s := New(baseInventory())
	res, err := s.Solve(Request{
		CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5,
		SlotConstraints: map[string][]string{"sands": {"enerRech_"}}, // the only sands piece has mainStatKey "em"
	})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) != 0 {
		t.Fatalf("expected the slot filter to eliminate the only sands candidate, got %d results", len(res.Builds))
	}
}

func TestSolve_TopNLimitsResultCount(t *testing.T) {
	inv := []model.Artifact{}
	id := 0
	add := func(setKey, slot, mainKey string, mainVal float64, cr float64) {
		id++
		inv = append(inv, mkArt(id, setKey, slot, mainKey, mainVal, stat("critRate_", cr)))
	}
	// 3 distinct candidates per slot, all on-set, so many complete combinations exist.
	for i := 0; i < 3; i++ {
		add("SetA", "flower", "hp", 4780, float64(i))
		add("SetA", "plume", "atk", 311, float64(i))
		add("SetA", "sands", "em", 187, float64(i))
		add("SetA", "goblet", "pyro_dmg_", 46.6, float64(i))
		add("SetA", "circlet", "critRate_", 10, float64(i))
	}
	s := New(inv)
	res, err := s.Solve(Request{CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 2})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) != 2 {
		t.Fatalf("expected exactly topN=2 results, got %d", len(res.Builds))
	}
	if res.Builds[0].CritValue < res.Builds[1].CritValue {
		t.Errorf("expected results sorted descending by CV, got %v then %v", res.Builds[0].CritValue, res.Builds[1].CritValue)
	}
}

// TestSolve_LargeInventoryStaysFast is a regression guard for the
// performance fix: each slot gets 300 on-set candidates (well past
// maxCandidatesPerSlot), which used to make the DFS branch on the full
// 300^5-scale space. It must still finish quickly and find the true best
// build — the single highest-CV piece per slot, which the CV-based cap
// always keeps regardless of pool size.
func TestSolve_LargeInventoryStaysFast(t *testing.T) {
	inv := []model.Artifact{}
	id := 0
	slots := []string{"flower", "plume", "sands", "goblet", "circlet"}
	// None of these main stats is critRate_/critDMG_, so every piece's Crit
	// Value comes only from its critRate_ substat below — keeps the expected
	// best-build CV simple to compute by hand.
	mains := map[string]string{"flower": "hp", "plume": "atk", "sands": "em", "goblet": "pyro_dmg_", "circlet": "hp"}
	const perSlot = 300
	for _, slot := range slots {
		for i := 0; i < perSlot; i++ {
			id++
			// CV ranges 0..299 per slot in random-ish (reverse) order so the
			// best piece isn't trivially first/last.
			cr := float64((i * 37) % perSlot)
			inv = append(inv, mkArt(id, "SetA", slot, mains[slot], 100, stat("critRate_", cr)))
		}
	}
	s := New(inv)
	start := time.Now()
	res, err := s.Solve(Request{CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Solve took %v on a 300-per-slot inventory — candidate capping should keep this well under a second", elapsed)
	}
	if len(res.Builds) == 0 {
		t.Fatal("expected at least one feasible build")
	}
	wantCV := 299.0 * 2 * 5 // best critRate_ (299) per slot * 2 (CV weight) * 5 slots
	if res.Builds[0].CritValue != wantCV {
		t.Errorf("expected the top build to use the single best-CV piece in every slot (CV=%v), got %v", wantCV, res.Builds[0].CritValue)
	}
}

// TestSolve_ExcludeEquippedByOthers verifies IncludeEquippedByOthers gates
// artifacts equipped on a different character: the higher-CV circlet is
// equipped on "Ayaka", so with the flag off it must be skipped in favor of
// the lower-CV unequipped circlet, and with it on (or equipped on the
// solving character itself) it's eligible again.
func TestSolve_ExcludeEquippedByOthers(t *testing.T) {
	inv := baseInventory()                                                              // circlet id 5 has cv 25, unequipped
	betterCirclet := mkArt(6, "SetA", "circlet", "critRate_", 10, stat("critDMG_", 40)) // cv 60
	betterCirclet.Location = "Ayaka"
	inv = append(inv, betterCirclet)

	res, err := New(inv).Solve(Request{
		CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5, IncludeEquippedByOthers: false,
	})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) != 1 {
		t.Fatalf("expected exactly 1 feasible build, got %d", len(res.Builds))
	}
	for _, p := range res.Builds[0].Pieces {
		if p.ID == betterCirclet.ID {
			t.Fatal("expected the circlet equipped on another character to be excluded")
		}
	}

	res, err = New(inv).Solve(Request{
		CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5, IncludeEquippedByOthers: true,
	})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	found := false
	for _, p := range res.Builds[0].Pieces {
		if p.ID == betterCirclet.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected the higher-CV circlet equipped on another character to be picked once included")
	}

	betterCirclet.Location = "HuTao"
	inv2 := append(baseInventory(), betterCirclet)
	res, err = New(inv2).Solve(Request{
		CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5, IncludeEquippedByOthers: false,
	})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	found = false
	for _, p := range res.Builds[0].Pieces {
		if p.ID == betterCirclet.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected the solving character's own equipped circlet to remain eligible even with IncludeEquippedByOthers=false")
	}
}

// TestSolver_Evaluate covers the "current build" snapshot: scoring an
// arbitrary fixed piece set the same way Solve scores a candidate, but
// without the on-set-≥4 gate (a real current build is often off-set) and
// without requiring all 5 slots (Complete reports whether it does).
func TestSolver_Evaluate(t *testing.T) {
	s := New(nil) // Evaluate doesn't consult s.Inventory
	pieces := []model.Artifact{
		mkArt(1, "SetB", "flower", "hp", 4780, stat("critRate_", 2.5)), // cv 5, off-set
		mkArt(2, "SetB", "plume", "atk", 311, stat("critDMG_", 5)),     // cv 5, off-set
	}
	res, err := s.Evaluate(pieces, Request{CharacterKey: "HuTao", TargetSetKey: "SetA"})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if res.CritValue != 10 {
		t.Errorf("expected CV 10, got %v", res.CritValue)
	}
	if res.OnSetCount != 0 {
		t.Errorf("expected onSetCount 0 (both pieces are SetB, target is SetA), got %d", res.OnSetCount)
	}
	if res.Complete {
		t.Error("expected Complete=false with only 2 of 5 slots given")
	}

	full := baseInventory() // 5 pieces, one per slot, all SetA
	res, err = s.Evaluate(full, Request{CharacterKey: "HuTao", TargetSetKey: "SetA"})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !res.Complete {
		t.Error("expected Complete=true with all 5 slots given")
	}
	if res.OnSetCount != 5 {
		t.Errorf("expected onSetCount 5, got %d", res.OnSetCount)
	}
}

func TestSolve_UnknownCharacterErrors(t *testing.T) {
	s := New(baseInventory())
	_, err := s.Solve(Request{CharacterKey: "NotACharacter", TargetSetKey: "SetA", TopN: 5})
	if err == nil {
		t.Fatal("expected an error for an unrecognized character key")
	}
}

// dualSetInventory has a single candidate per slot: 2 pieces of SetA, 2 of
// SetB, and 1 of a third set entirely (SetC) — the minimal shape a 2pc+2pc
// request can be satisfied by, with the 5th (free) slot not belonging to
// either target set.
func dualSetInventory() []model.Artifact {
	return []model.Artifact{
		mkArt(1, "SetA", "flower", "hp", 4780, stat("critRate_", 2.5)),
		mkArt(2, "SetA", "plume", "atk", 311, stat("critDMG_", 5)),
		mkArt(3, "SetB", "sands", "em", 187, stat("critRate_", 2.5)),
		mkArt(4, "SetB", "goblet", "pyro_dmg_", 46.6, stat("critDMG_", 5)),
		mkArt(5, "SetC", "circlet", "critRate_", 10, stat("critDMG_", 5)),
	}
}

func TestSolve_DualSetMode_FindsTwoPlusTwoBuild(t *testing.T) {
	s := New(dualSetInventory())
	res, err := s.Solve(Request{CharacterKey: "HuTao", TargetSetKey: "SetA", TargetSetKey2: "SetB", TopN: 5})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) != 1 {
		t.Fatalf("expected exactly 1 feasible build from a single candidate per slot, got %d", len(res.Builds))
	}
	b := res.Builds[0]
	if b.OnSetCount != 2 {
		t.Errorf("expected 2 pieces on SetA, got %d", b.OnSetCount)
	}
	if b.OnSetCount2 != 2 {
		t.Errorf("expected 2 pieces on SetB, got %d", b.OnSetCount2)
	}
}

func TestSolve_DualSetMode_NoFeasibleWhenOneSetShort(t *testing.T) {
	inv := dualSetInventory()
	// Only SetA has 2 pieces now — SetB has none, so 2pc of SetB is unreachable.
	inv[2].SetKey = "SetC"
	inv[3].SetKey = "SetC"
	s := New(inv)
	res, err := s.Solve(Request{CharacterKey: "HuTao", TargetSetKey: "SetA", TargetSetKey2: "SetB", TopN: 5})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) != 0 {
		t.Fatalf("expected no feasible builds when SetB has zero pieces, got %d", len(res.Builds))
	}
}

func TestSolve_DualSetMode_DoesNotAffectSingleSetMode(t *testing.T) {
	// TargetSetKey2 left empty must behave exactly like the pre-existing
	// single-set path — regression guard for the slotCands/branchRoles refactor.
	s := New(baseInventory())
	res, err := s.Solve(Request{CharacterKey: "HuTao", TargetSetKey: "SetA", TopN: 5})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}
	if len(res.Builds) != 1 || res.Builds[0].OnSetCount != 5 || res.Builds[0].OnSetCount2 != 0 {
		t.Fatalf("expected single-set behavior unchanged, got %+v", res.Builds)
	}
}
