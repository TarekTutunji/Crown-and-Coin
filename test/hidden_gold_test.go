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

// Hiding happens first, whatever order the merchant queued things in; after
// that merchants spend from their purse first and then from hidden gold.
func TestHidingComesFirstThenSpendingUsesPurseFirst(t *testing.T) {
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

	// Hiding the whole purse and still investing works: the investment comes out of hidden gold
	newState, _ = phase.Execute(state, []actions.Action{
		actions.NewMerchantInvestAction("anna", "anna", 7),
		actions.NewMerchantHideAction("anna", "anna", 6),
	})
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 0 || hidden != 3 || invested != 7 {
		t.Errorf("want purse 0 hidden 3 invested 7, got %d/%d/%d", purse, hidden, invested)
	}

	// Investing more than the purse dips into hidden gold
	newState, _ = phase.Execute(state, []actions.Action{
		actions.NewMerchantInvestAction("anna", "anna", 8),
	})
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 0 || hidden != 2 || invested != 8 {
		t.Errorf("want purse 0 hidden 2 invested 8, got %d/%d/%d", purse, hidden, invested)
	}
}

// Investment payouts land in the purse, so the monarch can tax them before
// the merchant gets a chance to hide them.
func TestInvestmentPayoutIsTaxable(t *testing.T) {
	state := twoKingdoms()
	anna := state.GetMerchant("anna")
	anna.StoredGold, anna.HiddenGold, anna.InvestedGold = 0, 4, 3

	war := phases.NewWarPhase(engine.NewFixedDice(1))
	newState, _ := war.Execute(state, nil)
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 11 || hidden != 4 || invested != 0 {
		t.Errorf("payout of 6 and income of 5 should go to the purse: want 11/4/0, got %d/%d/%d", purse, hidden, invested)
	}
}

// Fleeing merchants keep their purse and their hidden gold.
func TestFleeingKeepsHiddenGold(t *testing.T) {
	state := twoKingdoms()
	anna := state.GetMerchant("anna")
	anna.StoredGold, anna.HiddenGold, anna.InvestedGold = 2, 7, 5

	flee := actions.NewFleeAction("anna", "anna", "Britannia")
	newState, _ := flee.Apply(state, engine.NewFixedDice(1))
	if purse, hidden, invested := gold(t, newState, "anna"); purse != 2 || hidden != 7 || invested != 0 {
		t.Errorf("anna should keep 2 purse and 7 hidden and lose her investment, got %d/%d/%d", purse, hidden, invested)
	}
}

// A merchant can only queue hiding up to their purse, and spending up to all
// their purse and hidden gold together.
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
		t.Fatal("investing 3 of 10 should be accepted")
	}
	if submit("merchant_hide", 6) {
		t.Error("the purse only holds 5, hiding 6 should be rejected")
	}
	if !submit("merchant_hide", 5) {
		t.Error("hiding the whole purse should be accepted")
	}
	if !submit("merchant_invest", 7) {
		t.Error("investing the remaining 7 of her 10 gold should be accepted")
	}
	if submit("merchant_invest", 1) {
		t.Error("anna has nothing left to invest")
	}

	send(t, api, map[string]any{"type": "advance"})
	if purse, hidden, invested := gold(t, api.GetEngine().GetState(), "anna"); purse != 0 || hidden != 0 || invested != 10 {
		t.Errorf("all accepted actions should go through: want 0/0/10, got %d/%d/%d", purse, hidden, invested)
	}
}
