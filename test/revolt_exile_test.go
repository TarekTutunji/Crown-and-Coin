package test

import (
	"testing"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/events"
	"crown_and_coin/phases"
)

// revoltSetup builds three monarchies with a single rich rebel in Avalon, rich
// enough that the revolt always succeeds.
func revoltSetup() *engine.GameState {
	state := engine.NewGameState()
	state.AddCountry(engine.NewCountry("Avalon", "alice"))
	state.AddCountry(engine.NewCountry("Britannia", "bob"))
	state.AddCountry(engine.NewCountry("Cathay", "carol"))

	rebel := engine.NewMerchant("rebel", "Avalon")
	rebel.StoredGold = 100
	state.AddMerchant(rebel)

	return state
}

// A monarch deposed by a revolt is exiled: they must never end up back in the
// country that just threw them out, however the dice fall. Avalon survives the
// revolt at 8 HP, so it would be a candidate if it were not excluded.
func TestDeposedMonarchIsExiledFromTheirOwnCountry(t *testing.T) {
	destinations := make(map[string]bool)

	for seed := int64(0); seed < 200; seed++ {
		state := revoltSetup()

		newState, _ := actions.ResolveRevolt(
			state, "Avalon", []string{"rebel"}, nil, engine.NewSeededDice(seed),
		)

		if avalon := newState.GetCountry("Avalon"); !avalon.IsAlive() {
			t.Fatalf("seed %d: Avalon should survive the revolt, so it is a possible destination", seed)
		}

		alice := newState.GetMerchant("alice")
		if alice == nil {
			t.Fatalf("seed %d: deposed monarch was not resettled as a merchant", seed)
		}
		if alice.CountryID == "Avalon" {
			t.Errorf("seed %d: deposed monarch stayed in Avalon instead of being exiled", seed)
		}
		destinations[alice.CountryID] = true
	}

	// Sanity check that the destination really is random and not a fixed pick
	if len(destinations) != 2 {
		t.Errorf("expected exiles to reach both other countries, got %v", destinations)
	}
}

// With no other country left to flee to, the deposed monarch simply drops out
// of the game rather than staying on as a merchant at home.
func TestDeposedMonarchWithNowhereToGoLeavesTheGame(t *testing.T) {
	state := engine.NewGameState()
	state.AddCountry(engine.NewCountry("Avalon", "alice"))

	rebel := engine.NewMerchant("rebel", "Avalon")
	rebel.StoredGold = 100
	state.AddMerchant(rebel)

	newState, _ := actions.ResolveRevolt(
		state, "Avalon", []string{"rebel"}, nil, engine.NewSeededDice(1),
	)

	if alice := newState.GetMerchant("alice"); alice != nil {
		t.Errorf("deposed monarch should have left the game, but is a merchant in %s", alice.CountryID)
	}
}

// Scenario 8: a country on its last life suffers a successful merchant
// revolt, and the 2 HP it costs kills it. The revolt is settled first: the
// monarch is exiled with exactly 5 gold and the rebel takes the whole
// treasury. Then the country collapses and every merchant, rebels included,
// moves to a surviving country, losing their investments.
func TestRevoltThatKillsTheCountryCollapsesIt(t *testing.T) {
	state := revoltSetup()
	avalon := state.GetCountry("Avalon")
	avalon.HP = 1
	avalon.DiedOnce = true
	avalon.Gold = 12
	state.GetMerchant("rebel").InvestedGold = 4

	newState, _ := actions.ResolveRevolt(
		state, "Avalon", []string{"rebel"}, nil, engine.NewSeededDice(1),
	)

	avalon = newState.GetCountry("Avalon")
	if avalon.IsAlive() || avalon.IsRepublic || avalon.Gold != 0 || avalon.Peasants != 0 {
		t.Errorf("Avalon should be dead with no treasury, no peasants and no republic, got %+v", avalon)
	}

	alice := newState.GetMerchant("alice")
	if alice == nil || alice.CountryID == "Avalon" || alice.StoredGold != 5 || alice.HiddenGold != 0 {
		t.Errorf("alice should be a merchant elsewhere with exactly 5 gold, got %+v", alice)
	}

	rebel := newState.GetMerchant("rebel")
	if rebel.CountryID == "Avalon" || !rebel.Arriving || rebel.StoredGold != 112 || rebel.InvestedGold != 0 {
		t.Errorf("rebel should move elsewhere with 100 + the 12 gold treasury and no investment, got %+v", rebel)
	}
}

// A successful revolt: the rebels share out the whole treasury (the dice give
// out the odd coin) and the monarch leaves with 5 gold from the bank.
// Investments do not count towards the revolt.
func TestSuccessfulRevoltSplitsTheWholeTreasury(t *testing.T) {
	state := revoltSetup()
	state.GetCountry("Avalon").Gold = 11
	rebel := state.GetMerchant("rebel")
	rebel.StoredGold, rebel.HiddenGold = 6, 6
	second := engine.NewMerchant("second", "Avalon")
	second.StoredGold = 0
	state.AddMerchant(second)
	loyal := engine.NewMerchant("loyal", "Avalon")
	loyal.StoredGold, loyal.InvestedGold = 0, 50 // invested gold never counts
	state.AddMerchant(loyal)

	newState, _ := actions.ResolveRevolt(
		state, "Avalon", []string{"rebel", "second"}, []string{"loyal"}, engine.NewSeededDice(1),
	)

	avalon := newState.GetCountry("Avalon")
	if !avalon.IsRepublic || avalon.HP != 8 || avalon.Gold != 0 {
		t.Fatalf("Avalon should be a republic at 8 HP with an empty treasury, got %+v", avalon)
	}
	rebelShare := newState.GetMerchant("rebel").StoredGold - 6
	secondShare := newState.GetMerchant("second").StoredGold
	if rebelShare+secondShare != 11 || (rebelShare != 6 && secondShare != 6) {
		t.Errorf("the rebels should split all 11 gold 6/5, got %d/%d", rebelShare, secondShare)
	}
	if alice := newState.GetMerchant("alice"); alice == nil || alice.StoredGold != 5 {
		t.Errorf("alice should start over with 5 gold, got %+v", alice)
	}
}

// Scenario 7: the rebels hold 12 in purse and hidden gold, against a treasury
// of 8 and one merchant backing the monarch with 4. The tie goes to the
// monarch. The rebels lose their purse and hidden gold to the treasury and
// their investments are destroyed; the loyal merchant keeps theirs.
func TestFailedRevoltTieGoesToTheMonarch(t *testing.T) {
	state := revoltSetup()
	state.GetCountry("Avalon").Gold = 8
	rebel := state.GetMerchant("rebel")
	rebel.StoredGold, rebel.HiddenGold, rebel.InvestedGold = 5, 3, 10
	second := engine.NewMerchant("second", "Avalon")
	second.StoredGold = 4
	state.AddMerchant(second)
	loyal := engine.NewMerchant("loyal", "Avalon")
	loyal.StoredGold, loyal.InvestedGold = 4, 3
	state.AddMerchant(loyal)

	newState, _ := actions.ResolveRevolt(
		state, "Avalon", []string{"rebel", "second"}, []string{"loyal"}, engine.NewSeededDice(1),
	)

	avalon := newState.GetCountry("Avalon")
	if avalon.IsRepublic || avalon.MonarchID != "alice" || avalon.HP != 10 {
		t.Fatalf("alice should keep her throne, got %+v", avalon)
	}
	if avalon.Gold != 20 {
		t.Errorf("the treasury should get the rebels' 12 gold on top of its 8, got %d", avalon.Gold)
	}
	if r := newState.GetMerchant("rebel"); r.TotalGold() != 0 {
		t.Errorf("the rebel should lose everything, investments included, got %+v", r)
	}
	if l := newState.GetMerchant("loyal"); l.StoredGold != 4 || l.InvestedGold != 3 {
		t.Errorf("the loyal merchant should keep their gold and investments, got %+v", l)
	}
}

// exileAbandonedSetup builds the republic of Avalon, whose only merchant ann is
// about to flee, next to Castile, where a rich rebel is about to overthrow
// carol, and any other monarchies given
func exileAbandonedSetup(others ...string) *engine.GameState {
	state := engine.NewGameState()
	avalon := engine.NewCountry("Avalon", "")
	avalon.BecomeRepublic()
	state.AddCountry(avalon)
	state.AddMerchant(engine.NewMerchant("ann", "Avalon"))

	state.AddCountry(engine.NewCountry("Castile", "carol"))
	rebel := engine.NewMerchant("rebel", "Castile")
	rebel.StoredGold = 100
	state.AddMerchant(rebel)

	for _, id := range others {
		state.AddCountry(engine.NewCountry(id, monarchOf(id)))
	}
	return state
}

func exiledTo(evts []events.Event, monarchID string) string {
	for _, evt := range evts {
		if d, ok := evt.(*events.MonarchDeposedEvent); ok && d.MonarchID == monarchID {
			return d.ToCountry
		}
	}
	return ""
}

// carol is overthrown and exiled to Avalon, but in the same phase Avalon's
// last merchant flees and the republic is abandoned. carol must still never
// land back in Castile: she goes to Britannia, the only other country left.
func TestExileNeverReturnsHomeWhenTheirExileIsAbandoned(t *testing.T) {
	tested := 0
	for seed := int64(0); seed < 100; seed++ {
		state := exileAbandonedSetup("Britannia")
		newState, evts := phases.NewAssessmentPhase(engine.NewSeededDice(seed)).Execute(state, []actions.Action{
			actions.NewRevoltAction("rebel", "rebel", "Castile"),
			actions.NewFleeAction("ann", "ann", "Britannia"),
		})
		if newState.GetCountry("Avalon").IsAlive() || !newState.GetCountry("Castile").IsRepublic {
			t.Fatalf("seed %d: Avalon should be abandoned and Castile a republic", seed)
		}
		carol := newState.GetMerchant("carol")
		if carol == nil || carol.CountryID != "Britannia" || carol.StoredGold != engine.FallenMonarchPurse {
			t.Errorf("seed %d: carol should start over in Britannia with 5 gold, got %+v", seed, carol)
		}
		if exiledTo(evts, "carol") == "Avalon" {
			tested++
		}
	}
	if tested == 0 {
		t.Error("the dice never sent carol to Avalon, so the case went untested")
	}
}

// Without any other country left, an exile whose country of exile is
// abandoned leaves the game rather than going home.
func TestExileLeavesTheGameWhenNothingElseIsLeft(t *testing.T) {
	state := exileAbandonedSetup()
	newState, evts := phases.NewAssessmentPhase(engine.NewSeededDice(1)).Execute(state, []actions.Action{
		actions.NewRevoltAction("rebel", "rebel", "Castile"),
		actions.NewFleeAction("ann", "ann", "Castile"),
	})
	if exiledTo(evts, "carol") != "Avalon" {
		t.Fatal("carol should first be exiled to Avalon, the only other country")
	}
	if carol := newState.GetMerchant("carol"); carol != nil {
		t.Errorf("carol should have left the game, but is a merchant in %s", carol.CountryID)
	}
}
