package test

import (
	"encoding/json"
	"testing"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
	"crown_and_coin/phases"
)

// republicSetup builds the merchant republic Avalon with the given merchants
// (each holding 10 gold) next to the monarchy Britannia ruled by bob.
func republicSetup(merchantIDs ...string) *engine.GameState {
	state := engine.NewGameState()

	avalon := engine.NewCountry("Avalon", "alice")
	avalon.BecomeRepublic()
	avalon.Gold = 0
	state.AddCountry(avalon)
	state.AddCountry(engine.NewCountry("Britannia", "bob"))

	for _, id := range merchantIDs {
		merchant := engine.NewMerchant(id, "Avalon")
		merchant.StoredGold = 10
		state.AddMerchant(merchant)
	}
	return state
}

func voteTax(merchantID string, high bool) actions.Action {
	return actions.NewVoteTaxAction(merchantID, merchantID, high)
}

func voteAttack(merchantID, targetID string) actions.Action {
	return actions.NewVoteAttackAction(merchantID, merchantID, targetID)
}

func storedGold(t *testing.T, state *engine.GameState, merchantID string) int {
	t.Helper()
	merchant := state.GetMerchant(merchantID)
	if merchant == nil {
		t.Fatalf("merchant %s not found", merchantID)
	}
	return merchant.StoredGold
}

// A tied vote means low tax: 5 peasants pay 5 gold, shared evenly among all
// merchants with the odd coin going to the lowest ID, whoever voted how.
func TestRepublicTaxTieResolvesAsLowAndIsShared(t *testing.T) {
	state := republicSetup("anna", "ben")
	state.GetCountry("Avalon").RevoltRisk = 4

	phase := phases.NewTaxationPhase(engine.NewFixedDice(1))
	newState, _ := phase.Execute(state, []actions.Action{
		voteTax("anna", true),
		voteTax("ben", false),
	})

	if got := storedGold(t, newState, "anna"); got != 13 {
		t.Errorf("anna should have 10 + 3 gold, got %d", got)
	}
	if got := storedGold(t, newState, "ben"); got != 12 {
		t.Errorf("ben should have 10 + 2 gold, got %d", got)
	}
	avalon := newState.GetCountry("Avalon")
	if avalon.RevoltRisk != 2 || avalon.HP != 10 || avalon.Gold != 0 {
		t.Errorf("low tax should reset revolt risk and leave the treasury empty, got %+v", avalon)
	}
}

// With nobody voting, the tie still means the peasants are taxed low.
func TestRepublicTaxWithoutVotesIsLow(t *testing.T) {
	state := republicSetup("anna")

	phase := phases.NewTaxationPhase(engine.NewFixedDice(1))
	newState, _ := phase.Execute(state, nil)

	if got := storedGold(t, newState, "anna"); got != 15 {
		t.Errorf("anna should receive the whole low tax of 5, got %d gold", got)
	}
}

// A majority for high tax doubles the take, and runs the same revolt-risk
// roll as a monarchy's high tax.
func TestRepublicHighTaxMajority(t *testing.T) {
	votes := []actions.Action{
		voteTax("anna", true),
		voteTax("ben", true),
		voteTax("cleo", false),
	}

	// Roll 6 beats the revolt risk of 2: 10 gold split 4/3/3, risk goes up
	state := republicSetup("anna", "ben", "cleo")
	newState, _ := phases.NewTaxationPhase(engine.NewFixedDice(6)).Execute(state, votes)
	for id, want := range map[string]int{"anna": 14, "ben": 13, "cleo": 13} {
		if got := storedGold(t, newState, id); got != want {
			t.Errorf("after high tax %s should have %d gold, got %d", id, want, got)
		}
	}
	if risk := newState.GetCountry("Avalon").RevoltRisk; risk != 3 {
		t.Errorf("revolt risk should rise to 3, got %d", risk)
	}

	// Roll 1 means the peasants revolt: no gold and 2 HP of damage
	state = republicSetup("anna", "ben", "cleo")
	newState, _ = phases.NewTaxationPhase(engine.NewFixedDice(1)).Execute(state, votes)
	if got := storedGold(t, newState, "anna"); got != 10 {
		t.Errorf("a peasant revolt should bring in no gold, anna has %d", got)
	}
	if hp := newState.GetCountry("Avalon").HP; hp != 8 {
		t.Errorf("a peasant revolt should cost 2 HP, got %d", hp)
	}
}

// Merchants of a republic see the tax vote, not the monarch's tax actions.
func TestRepublicMerchantActions(t *testing.T) {
	state := republicSetup("anna")

	types := func(phase phases.Phase) map[actions.ActionType]bool {
		found := make(map[actions.ActionType]bool)
		for _, a := range phase.ValidActions(state, "anna") {
			found[a.Type()] = true
		}
		return found
	}

	taxation := types(phases.NewTaxationPhase(engine.NewFixedDice(1)))
	if !taxation[actions.ActionVoteTaxLow] || !taxation[actions.ActionVoteTaxHigh] {
		t.Errorf("republic merchant should be able to vote on tax, got %v", taxation)
	}

	spending := types(phases.NewSpendingPhase(engine.NewFixedDice(1)))
	for _, want := range []actions.ActionType{actions.ActionMerchantInvest, actions.ActionMerchantHide, actions.ActionContributeArmy} {
		if !spending[want] {
			t.Errorf("republic merchant should be offered %s, got %v", want, spending)
		}
	}

	war := types(phases.NewWarPhase(engine.NewFixedDice(1)))
	if !war[actions.ActionVoteAttack] || !war[actions.ActionVoteNoAttack] {
		t.Errorf("republic merchant should be able to vote on war, got %v", war)
	}

	assessment := types(phases.NewAssessmentPhase(engine.NewFixedDice(1)))
	if assessment[actions.ActionRevolt] {
		t.Error("there is no monarch to revolt against in a republic")
	}
	if !assessment[actions.ActionRemain] || !assessment[actions.ActionFlee] {
		t.Errorf("republic merchant should be able to remain or flee, got %v", assessment)
	}
	if err := actions.NewRevoltAction("anna", "anna", "Avalon").Validate(state); err == nil {
		t.Error("a revolt in a republic should be rejected")
	}
}

// Contributions turn gold into army strength one for one, and the communal
// army is halved after the war like any other.
func TestRepublicCommunalArmy(t *testing.T) {
	state := republicSetup("anna", "ben")

	newState, _ := phases.NewSpendingPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		actions.NewContributeArmyAction("anna", "anna", 6),
		actions.NewContributeArmyAction("ben", "ben", 3),
	})

	if army := newState.GetCountry("Avalon").ArmyStrength; army != 9 {
		t.Errorf("communal army should be 9, got %d", army)
	}
	if got := storedGold(t, newState, "anna"); got != 4 {
		t.Errorf("anna should have 4 gold left, got %d", got)
	}

	newState, _ = phases.NewWarPhase(engine.NewFixedDice(1)).Execute(newState, nil)
	if army := newState.GetCountry("Avalon").ArmyStrength; army != 4 {
		t.Errorf("communal army should be halved to 4 after the war, got %d", army)
	}
}

// An attack needs more than half of all the republic's merchants, counting
// those who did not vote at all.
func TestRepublicWarVoteNeedsStrictMajority(t *testing.T) {
	cases := []struct {
		name     string
		votes    []actions.Action
		attacked bool
	}{
		{"two of three", []actions.Action{voteAttack("anna", "Britannia"), voteAttack("ben", "Britannia")}, true},
		{"one of three", []actions.Action{voteAttack("anna", "Britannia")}, false},
		{"votes split", []actions.Action{
			voteAttack("anna", "Britannia"),
			voteAttack("ben", "Camelot"),
			actions.NewVoteNoAttackAction("cleo", "cleo"),
		}, false},
		{"a merchant voting twice counts once", []actions.Action{
			voteAttack("anna", "Britannia"),
			voteAttack("anna", "Britannia"),
		}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := republicSetup("anna", "ben", "cleo")
			state.AddCountry(engine.NewCountry("Camelot", "carl"))
			state.GetCountry("Avalon").ArmyStrength = 4

			newState, _ := phases.NewWarPhase(engine.NewFixedDice(1)).Execute(state, tc.votes)

			britannia := newState.GetCountry("Britannia")
			if tc.attacked && britannia.HP != 6 {
				t.Errorf("Britannia should have taken 4 damage, has %d HP", britannia.HP)
			}
			if !tc.attacked && britannia.HP != 10 {
				t.Errorf("there should be no attack, but Britannia has %d HP", britannia.HP)
			}
		})
	}
}

// With exactly half the merchants behind a target, there is no attack.
func TestRepublicWarVoteTieMeansNoAttack(t *testing.T) {
	state := republicSetup("anna", "ben")
	state.GetCountry("Avalon").ArmyStrength = 4

	newState, _ := phases.NewWarPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		voteAttack("anna", "Britannia"),
	})

	if hp := newState.GetCountry("Britannia").HP; hp != 10 {
		t.Errorf("1 of 2 votes is not a strict majority, but Britannia has %d HP", hp)
	}
}

// A republic's first death overall revives it at 1 HP, still a republic,
// with its merchants still at home.
func TestRepublicFirstDeathRevives(t *testing.T) {
	state := republicSetup("anna")
	state.GetCountry("Avalon").HP = 3
	state.GetCountry("Britannia").ArmyStrength = 5

	newState, _ := phases.NewWarPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		actions.NewAttackAction("bob", "Britannia", "Avalon"),
	})

	avalon := newState.GetCountry("Avalon")
	if avalon.HP != 1 || !avalon.IsRepublic || !avalon.DiedOnce {
		t.Errorf("Avalon should be revived at 1 HP as a republic, got %+v", avalon)
	}
	if anna := newState.GetMerchant("anna"); anna.CountryID != "Avalon" {
		t.Errorf("anna should still be in Avalon, is in %s", anna.CountryID)
	}
}

// A republic that already died once as a monarchy is eliminated for good.
// Its merchants move to the victor keeping only their hidden gold.
func TestRepublicSecondDeathKeepsOnlyHiddenGold(t *testing.T) {
	state := republicSetup("anna", "ben")
	avalon := state.GetCountry("Avalon")
	avalon.HP = 1
	avalon.DiedOnce = true
	avalon.ArmyStrength = 2
	state.GetCountry("Britannia").ArmyStrength = 5
	state.GetMerchant("anna").InvestedGold = 7

	newState, _ := phases.NewWarPhase(engine.NewFixedDice(1)).Execute(state, []actions.Action{
		actions.NewAttackAction("bob", "Britannia", "Avalon"),
	})

	if newState.GetCountry("Avalon").IsAlive() {
		t.Fatal("Avalon should be eliminated on its second death")
	}
	for _, id := range []string{"anna", "ben"} {
		merchant := newState.GetMerchant(id)
		if merchant.CountryID != "Britannia" {
			t.Errorf("%s should have moved to Britannia, is in %s", id, merchant.CountryID)
		}
		// 10 hidden gold plus the usual 5 income, and no investment payout
		if merchant.StoredGold != 15 || merchant.InvestedGold != 0 {
			t.Errorf("%s should keep only hidden gold (15 stored, 0 invested), got %d stored, %d invested",
				id, merchant.StoredGold, merchant.InvestedGold)
		}
	}
}

// Through the API, a republic merchant gets one spending choice and one vote
// per decision; a second one is rejected.
func TestRepublicMerchantGetsOneChoicePerDecision(t *testing.T) {
	api := jsonapi.NewGameAPIWithDice(engine.NewFixedDice(1))
	api.GetEngine().SetState(republicSetup("anna"))
	api.GetEngine().GetState().Phase = engine.PhaseSpending

	submit := func(action map[string]any) (bool, string) {
		msg, _ := json.Marshal(map[string]any{"type": "submit", "action": action})
		raw, err := api.ProcessMessage(msg)
		if err != nil {
			t.Fatal(err)
		}
		var resp struct {
			Success         bool   `json:"success"`
			RejectionReason string `json:"rejection_reason"`
		}
		json.Unmarshal(raw, &resp)
		return resp.Success, resp.RejectionReason
	}

	if ok, reason := submit(map[string]any{"type": "contribute_army", "player_id": "anna", "merchant_id": "anna", "amount": 4}); !ok {
		t.Fatalf("first spending choice should be accepted: %s", reason)
	}
	if ok, _ := submit(map[string]any{"type": "merchant_invest", "player_id": "anna", "merchant_id": "anna", "amount": 2}); ok {
		t.Error("a second spending choice should be rejected")
	}
}
