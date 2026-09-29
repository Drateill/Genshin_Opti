package good

import "testing"

func TestParse_StandardExport(t *testing.T) {
	raw := []byte(`{
		"format": "GOOD", "version": 2,
		"characters": [{"key":"HuTao","level":90,"ascension":6,"constellation":1,"talent":{"auto":9,"skill":9,"burst":9}}],
		"artifacts": [{
			"setKey":"CrimsonWitchOfFlames","slotKey":"flower","level":20,"rarity":5,"mainStatKey":"hp",
			"location":"HuTao","lock":true,
			"substats":[{"key":"critRate_","value":7.8},{"key":"critDMG_","value":15.5}]
		}]
	}`)
	res, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(res.Export.Characters) != 1 || res.Export.Characters[0].Key != "HuTao" {
		t.Fatalf("expected 1 character HuTao, got %+v", res.Export.Characters)
	}
	if len(res.Export.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(res.Export.Artifacts))
	}
	a := res.Export.Artifacts[0]
	if a.MainStatKey != "hp" || a.MainStatValue != 4780 {
		t.Errorf("expected main stat hp=4780, got %s=%v", a.MainStatKey, a.MainStatValue)
	}
	if !a.Lock || a.Location != "HuTao" {
		t.Errorf("expected lock=true location=HuTao, got lock=%v location=%q", a.Lock, a.Location)
	}
	if len(a.SubStats) != 2 {
		t.Fatalf("expected 2 substats, got %d", len(a.SubStats))
	}
}

func TestParse_CaseVariantStatKeys(t *testing.T) {
	// Tools vary in casing/spelling: "HP" (flat, title-case), "CritRate"
	// (no trailing underscore), "subStats" (alt field name).
	raw := []byte(`{
		"format":"GOOD","version":2,
		"artifacts":[{
			"setKey":"SomeObscureSet","slotKey":"flower","level":16,"rarity":4,"mainStatKey":"HP",
			"subStats":[{"key":"CritRate","value":3.1},{"stat":"critDMG_","val":6.2}]
		}]
	}`)
	res, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	a := res.Export.Artifacts[0]
	if a.MainStatKey != "hp" {
		t.Errorf("expected mainStatKey normalized to \"hp\", got %q", a.MainStatKey)
	}
	wantMain := MainStatValue("hp", 4, 16)
	if a.MainStatValue != round1(wantMain) {
		t.Errorf("expected computed main stat value %v, got %v", round1(wantMain), a.MainStatValue)
	}
	if len(a.SubStats) != 2 {
		t.Fatalf("expected 2 substats (key/value and stat/val forms), got %d: %+v", len(a.SubStats), a.SubStats)
	}
	if a.SubStats[0].Key != "critRate_" {
		t.Errorf("expected first substat normalized to critRate_, got %q", a.SubStats[0].Key)
	}
	if a.SubStats[1].Key != "critDMG_" || a.SubStats[1].Value != 6.2 {
		t.Errorf("expected second substat critDMG_=6.2 (stat/val alt names), got %+v", a.SubStats[1])
	}
}

func TestParse_DefaultsAndPrecomputedMainValue(t *testing.T) {
	raw := []byte(`{
		"artifacts":[{"setKey":"GildedDreams","slotKey":"circlet","mainStatKey":"critRate_","mainStatValue":31.1,"substats":[]}]
	}`)
	res, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	a := res.Export.Artifacts[0]
	if a.Level != 20 || a.Rarity != 5 {
		t.Errorf("expected default level=20 rarity=5, got level=%d rarity=%d", a.Level, a.Rarity)
	}
	if a.MainStatValue != 31.1 {
		t.Errorf("expected precomputed mainStatValue to be used as-is, got %v", a.MainStatValue)
	}
}

func TestParse_TolerantOfBadEntries(t *testing.T) {
	// One artifact is missing setKey; Parse should skip it and keep the rest,
	// per the spec's tolerant-parsing requirement, rather than failing outright.
	raw := []byte(`{
		"artifacts":[
			{"slotKey":"flower","mainStatKey":"hp"},
			{"setKey":"CrimsonWitchOfFlames","slotKey":"plume","mainStatKey":"atk"}
		]
	}`)
	res, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse should tolerate one bad entry, got error: %v", err)
	}
	if len(res.Export.Artifacts) != 1 {
		t.Fatalf("expected 1 surviving artifact, got %d", len(res.Export.Artifacts))
	}
	if len(res.Issues) != 1 {
		t.Fatalf("expected 1 recorded issue, got %d: %+v", len(res.Issues), res.Issues)
	}
}

func TestParse_RejectsEmptyDocument(t *testing.T) {
	_, err := Parse([]byte(`{"format":"GOOD","version":2}`))
	if err == nil {
		t.Fatal("expected an error for a document with no artifacts or characters")
	}
}

func TestParse_RejectsInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`not json`))
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}
