package test

import (
	"testing"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/phases"
)

// Scenario 1: every high tax that does not set off a revolt raises the risk
// by one, up to 5 in 6. A low tax resets it to 2.
func TestRevoltRiskClimbsToFiveAndLowTaxResetsIt(t *testing.T) {
	state := twoKingdoms()
	phase := phases.NewTaxationPhase(engine.NewFixedDice(6)) // 6 never sets off a revolt

	for _, want := range []int{3, 4, 5, 5} {
		state, _ = phase.Execute(state, []actions.Action{
			actions.NewTaxPeasantsAction("alice", "Avalon", true),
		})
		if got := state.GetCountry("Avalon").RevoltRisk; got != want {
			t.Fatalf("after another high tax the risk should be %d, got %d", want, got)
		}
	}

	state, _ = phase.Execute(state, []actions.Action{
		actions.NewTaxPeasantsAction("alice", "Avalon", false),
	})
	if got := state.GetCountry("Avalon").RevoltRisk; got != 2 {
		t.Errorf("a low tax should reset the risk to 2, got %d", got)
	}
}

// warSetup builds monarchies with the given army strengths, each ruled by a
// monarch named after the country in lower case
func warSetup(armies map[string]int) *engine.GameState {
	state := engine.NewGameState()
	for id, army := range armies {
		country := engine.NewCountry(id, monarchOf(id))
		country.ArmyStrength = army
		state.AddCountry(country)
	}
	return state
}

func monarchOf(countryID string) string {
	return "ruler of " + countryID
}

func attack(attackerID, defenderID string) actions.Action {
	return actions.NewAttackAction(monarchOf(attackerID), attackerID, defenderID)
}

// Scenario 4: when A (army 8) and B (army 3) attack each other it is a single
// battle. B loses 5 HP and A earns 5 gold, once.
func TestMutualAttackIsOneBattle(t *testing.T) {
	state := warSetup(map[string]int{"A": 8, "B": 3})

	newState, evts := phases.NewWarPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		attack("A", "B"),
		attack("B", "A"),
	})

	battles := 0
	for _, evt := range evts {
		if evt.Type() == "battle_resolved" {
			battles++
		}
	}
	if battles != 1 {
		t.Errorf("a mutual attack should be one battle, got %d", battles)
	}
	if a, b := newState.GetCountry("A"), newState.GetCountry("B"); a.HP != 10 || b.HP != 5 || a.Gold != 15 || b.Gold != 10 {
		t.Errorf("B should lose 5 HP and A earn 5 gold once, got A %+v and B %+v", a, b)
	}
}

// Scenario 5: C (army 10) fights off D (4) and E (6) with its full army each
// time. D loses 6 HP, E loses 4 HP, and C, a winning defender, earns nothing.
func TestDefenderUsesItsFullArmyAndEarnsNothing(t *testing.T) {
	state := warSetup(map[string]int{"C": 10, "D": 4, "E": 6})

	newState, _ := phases.NewWarPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		attack("D", "C"),
		attack("E", "C"),
	})

	if got := newState.GetCountry("D").HP; got != 4 {
		t.Errorf("D should lose 6 HP, has %d", got)
	}
	if got := newState.GetCountry("E").HP; got != 6 {
		t.Errorf("E should lose 4 HP, has %d", got)
	}
	if c := newState.GetCountry("C"); c.HP != 10 || c.Gold != 10 {
		t.Errorf("C should take no damage and earn no gold, got %+v", c)
	}
}

// Scenario 6: at the end of the war every army is halved, rounding down.
func TestArmyIsHalvedRoundingDown(t *testing.T) {
	state := warSetup(map[string]int{"A": 7})

	newState, _ := phases.NewWarPhase(engine.NewFixedDice(1)).Execute(state, nil)
	if got := newState.GetCountry("A").ArmyStrength; got != 3 {
		t.Errorf("an army of 7 should become 3, got %d", got)
	}
}

// Scenario 10: a merchant who flees in the Assessment is absent from their new
// country for the rest of the round. They arrive at the start of the next
// round, in time to be taxed there.
func TestFleeingMerchantArrivesAtTheStartOfTheNextRound(t *testing.T) {
	state := twoKingdoms()
	// anna has just fled; the round is not over yet
	state.GetMerchant("anna").FleeToCountry("Britannia")

	if !state.GetMerchant("anna").Arriving {
		t.Fatal("anna should still be on her way")
	}
	for _, m := range state.GetMerchantsByCountry("Britannia") {
		if m.ID == "anna" {
			t.Error("anna should not count as one of Britannia's merchants yet")
		}
	}
	if len(phases.NewAssessmentPhase(engine.NewFixedDice(1)).ValidActions(state, "anna")) != 0 {
		t.Error("anna should sit out the rest of the round")
	}
	tax := actions.NewTaxMerchantsAction("bob", "Britannia", "anna", 5)
	if err := tax.Validate(state); err == nil {
		t.Error("bob should not be able to tax anna before she arrives")
	}

	// The assessment ends the round and the next one starts
	state, _ = phases.NewAssessmentPhase(engine.NewFixedDice(1)).Execute(state, nil)
	if state.GetMerchant("anna").Arriving {
		t.Fatal("anna should have arrived at the start of the round")
	}

	state, _ = phases.NewTaxationPhase(engine.NewFixedDice(6)).Execute(state, []actions.Action{tax})
	if got := state.GetMerchant("anna").StoredGold; got != 0 {
		t.Errorf("bob should be able to tax all 5 of anna's gold now, she has %d left", got)
	}
}

// Through a full Assessment: a merchant who flees keeps their gold and loses
// their investments, and is in the new country when the next round begins.
func TestFleeInAssessment(t *testing.T) {
	state := twoKingdoms()
	state.GetMerchant("anna").InvestedGold = 4

	newState, _ := phases.NewAssessmentPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		actions.NewFleeAction("anna", "anna", "Britannia"),
	})

	anna := newState.GetMerchant("anna")
	if anna.CountryID != "Britannia" || anna.Arriving || anna.StoredGold != 5 || anna.InvestedGold != 0 {
		t.Errorf("anna should start the new round in Britannia with her 5 gold and no investment, got %+v", anna)
	}
}
