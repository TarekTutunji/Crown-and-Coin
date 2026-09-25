package test

import (
	"testing"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
	"crown_and_coin/phases"
)

func gold(t *testing.T, state *engine.GameState, merchantID string) (purse, hidden, invested int) {
	t.Helper()
	m := state.GetMerchant(merchantID)
	if m == nil {
		t.Fatalf("merchant %s not found", merchantID)
	}
	return m.StoredGold, m.HiddenGold, m.InvestedGold
}

// Hidden gold is safe from the monarch: only the purse can be taxed.
func TestHiddenGoldCannotBeTaxed(t *testing.T) {
	state := twoKingdoms()
	anna := state.GetMerchant("anna")
	anna.StoredGold, anna.HiddenGold = 3, 7

	phase := phases.NewTaxationPhase(engine.NewFixedDice(6))
	for _, action := range phase.ValidActions(state, "alice") {
		if tax, ok := action.(*actions.TaxMerchantsAction); ok && tax.Amount != 3 {
			t.Errorf("alice should be offered to tax at most anna's purse of 3, got %d", tax.Amount)
		}
	}

	newState, _ := phase.Execute(state, []actions.Action{
		actions.NewTaxMerchantsAction("alice", "Avalon", "anna", 10),
	})
	if purse, hidden, _ := gold(t, newState, "anna"); purse != 0 || hidden != 7 {
		t.Errorf("the tax should empty the purse but leave the 7 hidden gold, got purse %d hidden %d", purse, hidden)
	}
	// 10 treasury + 5 default low tax + 3 from anna
	if got := newState.GetCountry("Avalon").Gold; got != 18 {
		t.Errorf("Avalon should collect only the 3 gold in anna's purse, got treasury %d", got)
	}
}

// Hiding happens first, whatever order the merchant queued things in.
// Investing only ever takes gold from the purse, and gold unhidden this round
// reaches the purse too late: it can only be invested next round.
func TestHidingComesFirstAndInvestingUsesThePurse(t *testing.T) {
	state := twoKingdoms()
	anna := state.GetMerchant("anna")
	anna.StoredGold, anna.HiddenGold = 6, 4

	phase := phases.NewSpendingPhase(engine.NewFixedDice(1))
	newState, _ := phase.Execute(state, []actions.Action{
		actions.NewMerchantInvestAction("anna", "anna", 4),
		actions.NewMerchantHideAction("anna", "anna", 2),
	})
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 0 || hidden != 6 || invested != 4 {
		t.Errorf("anna should hide 2, then invest the 4 left in her purse: want purse 0 hidden 6 invested 4, got %d/%d/%d", purse, hidden, invested)
	}

	// Unhiding does not let her invest more than her purse held this round
	newState, _ = phase.Execute(state, []actions.Action{
		actions.NewMerchantUnhideAction("anna", "anna", 3),
		actions.NewMerchantInvestAction("anna", "anna", 7),
	})
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 9 || hidden != 1 || invested != 0 {
		t.Errorf("anna's 3 unhidden gold should reach her purse but not be invested: want 9/1/0, got %d/%d/%d", purse, hidden, invested)
	}
	// ...but she can invest part of the purse she had, and the unhidden gold
	// joins the purse afterwards
	newState, _ = phase.Execute(state, []actions.Action{
		actions.NewMerchantUnhideAction("anna", "anna", 3),
		actions.NewMerchantInvestAction("anna", "anna", 6),
	})
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 3 || hidden != 1 || invested != 6 {
		t.Errorf("anna should invest the 6 in her purse, then get the 3 unhidden: want 3/1/6, got %d/%d/%d", purse, hidden, invested)
	}
	// Next round the unhidden gold is ordinary purse gold and can be invested
	newState.Phase = engine.PhaseSpending
	newState, _ = phase.Execute(newState, []actions.Action{
		actions.NewMerchantInvestAction("anna", "anna", 3),
	})
	if purse, _, invested := gold(t, newState, "anna"); purse != 0 || invested != 9 {
		t.Errorf("the next round anna should be able to invest the 3 she unhid: want purse 0 invested 9, got %d/%d", purse, invested)
	}

	// Without unhiding, investing more than the purse does not touch hidden gold
	newState, _ = phase.Execute(state, []actions.Action{
		actions.NewMerchantInvestAction("anna", "anna", 8),
	})
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 6 || hidden != 4 || invested != 0 {
		t.Errorf("the purse only holds 6, so investing 8 should not happen: want 6/4/0, got %d/%d/%d", purse, hidden, invested)
	}
}

// Scenario 2: a merchant invests 4 in Phase 3. The 8 it pays out lands in her
// purse at the start of the next round, before Taxation, so her monarch can
// take all of it.
func TestInvestmentPaysOutAtTheStartOfTheNextRound(t *testing.T) {
	state := twoKingdoms()
	anna := state.GetMerchant("anna")
	anna.StoredGold = 4

	state, _ = phases.NewSpendingPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		actions.NewMerchantInvestAction("anna", "anna", 4),
	})
	state, _ = phases.NewWarPhase(engine.NewFixedDice(1)).Execute(state, nil)
	if purse, _, invested := gold(t, state, "anna"); purse != 5 || invested != 4 {
		t.Errorf("after the war anna should have only her income of 5 and still 4 invested, got purse %d invested %d", purse, invested)
	}

	state, _ = phases.NewAssessmentPhase(engine.NewFixedDice(1)).Execute(state, nil)
	if purse, _, invested := gold(t, state, "anna"); purse != 13 || invested != 0 {
		t.Errorf("the 8 payout should be in the purse when the round starts: want 13, got purse %d invested %d", purse, invested)
	}

	state, _ = phases.NewTaxationPhase(engine.NewFixedDice(6)).Execute(state, []actions.Action{
		actions.NewTaxMerchantsAction("alice", "Avalon", "anna", 13),
	})
	if purse, _, _ := gold(t, state, "anna"); purse != 0 {
		t.Errorf("alice should be able to tax all of it, anna has %d left", purse)
	}
}

// Scenario 3: a monarch's gift arrives after the merchants have acted, so it
// cannot be hidden or invested that round.
func TestGiftCannotBeHiddenOrInvestedThatRound(t *testing.T) {
	state := twoKingdoms()
	state.GetMerchant("anna").StoredGold = 0

	newState, _ := phases.NewSpendingPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		actions.NewMonarchInvestAction("alice", "Avalon", "anna", 6),
		actions.NewMerchantHideAction("anna", "anna", 3),
		actions.NewMerchantInvestAction("anna", "anna", 3),
	})
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 6 || hidden != 0 || invested != 0 {
		t.Errorf("the gift of 6 should sit in anna's purse untouched: want 6/0/0, got %d/%d/%d", purse, hidden, invested)
	}
	if got := newState.GetCountry("Avalon").Gold; got != 4 {
		t.Errorf("Avalon should have 10 - 6 gold left, got %d", got)
	}

	// Through the game server she cannot even queue it
	api := jsonapi.NewGameAPIWithDice(engine.NewFixedDice(1))
	state.Phase = engine.PhaseSpending
	api.GetEngine().SetState(state)
	send(t, api, map[string]any{"type": "submit", "action": map[string]any{
		"type": "monarch_invest", "player_id": "alice", "country_id": "Avalon", "merchant_id": "anna", "amount": 6}})
	resp := send(t, api, map[string]any{"type": "submit", "action": map[string]any{
		"type": "merchant_hide", "player_id": "anna", "merchant_id": "anna", "amount": 6}})
	if resp["success"] == true {
		t.Error("anna should not be able to hide a gift she has not received yet")
	}
}

// Fleeing merchants keep their purse and their hidden gold, and only arrive
// in their new country at the start of the next round.
func TestFleeingKeepsHiddenGold(t *testing.T) {
	state := twoKingdoms()
	anna := state.GetMerchant("anna")
	anna.StoredGold, anna.HiddenGold, anna.InvestedGold = 2, 7, 5

	flee := actions.NewFleeAction("anna", "anna", "Britannia")
	newState, _ := flee.Apply(state, engine.NewFixedDice(1))
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 2 || hidden != 7 || invested != 0 {
		t.Errorf("anna should keep 2 purse and 7 hidden and lose her investment, got %d/%d/%d", purse, hidden, invested)
	}
	if !newState.GetMerchant("anna").Arriving {
		t.Error("anna should still be on her way to Britannia")
	}
}

// A merchant can only queue hiding up to their purse, unhiding up to their
// hidden gold, and investing up to what is left in their purse after hiding.
// Gold they unhide cannot be invested the same round.
func TestQueuedHidingAndSpendingMustBeAffordable(t *testing.T) {
	api := jsonapi.NewGameAPIWithDice(engine.NewFixedDice(1))
	state := twoKingdoms()
	state.Phase = engine.PhaseSpending
	anna := state.GetMerchant("anna")
	anna.StoredGold, anna.HiddenGold = 5, 5
	api.GetEngine().SetState(state)

	submit := func(actionType string, amount int) bool {
		resp := send(t, api, map[string]any{"type": "submit", "action": map[string]any{
			"type": actionType, "player_id": "anna", "merchant_id": "anna", "amount": amount}})
		return resp["success"] == true
	}

	if !submit("merchant_invest", 3) {
		t.Fatal("investing 3 of her purse of 5 should be accepted")
	}
	if submit("merchant_hide", 6) {
		t.Error("the purse only holds 5, hiding 6 should be rejected")
	}
	if !submit("merchant_hide", 2) {
		t.Error("hiding the other 2 should be accepted")
	}
	if submit("merchant_invest", 1) {
		t.Error("her purse is used up, investing more should be rejected")
	}
	if !submit("merchant_unhide", 4) {
		t.Error("unhiding 4 of her 5 hidden gold should be accepted")
	}
	if submit("merchant_invest", 4) {
		t.Error("investing the 4 she unhides this round should be rejected")
	}
	if submit("merchant_unhide", 4) {
		t.Error("she only has 5 hidden gold, unhiding 8 in total should be rejected")
	}

	send(t, api, map[string]any{"type": "advance"})
	if purse, hidden, invested := gold(t, api.GetEngine().GetState(), "anna"); purse != 4 || hidden != 3 || invested != 3 {
		t.Errorf("all accepted actions should go through: want 4/3/3, got %d/%d/%d", purse, hidden, invested)
	}
}

// A republic merchant can only pay purse gold into the army: never hidden
// gold, and not gold they unhide this same round
func TestHiddenGoldCannotGoToTheArmy(t *testing.T) {
	api := jsonapi.NewGameAPIWithDice(engine.NewFixedDice(1))
	state := republicSetup("anna")
	state.Phase = engine.PhaseSpending
	anna := state.GetMerchant("anna")
	anna.StoredGold, anna.HiddenGold = 2, 8
	api.GetEngine().SetState(state)

	submit := func(actionType string, amount int) bool {
		resp := send(t, api, map[string]any{"type": "submit", "action": map[string]any{
			"type": actionType, "player_id": "anna", "merchant_id": "anna", "amount": amount}})
		return resp["success"] == true
	}

	if submit("contribute_army", 3) {
		t.Error("her purse only holds 2, paying 3 into the army should be rejected")
	}
	if !submit("merchant_unhide", 5) {
		t.Fatal("unhiding 5 of her 8 hidden gold should be accepted")
	}
	if submit("contribute_army", 3) {
		t.Error("paying gold she unhides this round into the army should be rejected")
	}
	if !submit("contribute_army", 2) {
		t.Error("paying her purse of 2 into the army should be accepted")
	}

	send(t, api, map[string]any{"type": "advance"})
	after := api.GetEngine().GetState()
	if purse, hidden, _ := gold(t, after, "anna"); purse != 5 || hidden != 3 {
		t.Errorf("she should end with the 5 unhidden gold in her purse and 3 hidden, got %d/%d", purse, hidden)
	}
	if army := after.GetCountry("Avalon").ArmyStrength; army != 2 {
		t.Errorf("the army should be 2, got %d", army)
	}

	// The phase itself refuses hidden gold too, and only offers the purse
	state = republicSetup("anna")
	anna = state.GetMerchant("anna")
	anna.StoredGold, anna.HiddenGold = 2, 8
	for _, a := range phases.NewSpendingPhase(engine.NewFixedDice(1)).ValidActions(state, "anna") {
		if c, ok := a.(*actions.ContributeArmyAction); ok && c.Amount != 2 {
			t.Errorf("anna should be offered to pay up to her purse of 2 into the army, offered %d", c.Amount)
		}
	}
	newState, _ := phases.NewSpendingPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		actions.NewContributeArmyAction("anna", "anna", 5),
	})
	if army := newState.GetCountry("Avalon").ArmyStrength; army != 0 {
		t.Errorf("paying 5 with only 2 in the purse should do nothing, army is %d", army)
	}
}
