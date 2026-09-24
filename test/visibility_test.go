package test

import (
	"encoding/json"
	"testing"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
	"crown_and_coin/phases"
)

// twoKingdoms builds Avalon (ruled by alice, with merchant anna) and
// Britannia (ruled by bob, with merchant ben)
func twoKingdoms() *engine.GameState {
	state := engine.NewGameState()
	state.AddCountry(engine.NewCountry("Avalon", "alice"))
	state.AddCountry(engine.NewCountry("Britannia", "bob"))
	state.AddMerchant(engine.NewMerchant("anna", "Avalon"))
	state.AddMerchant(engine.NewMerchant("ben", "Britannia"))
	return state
}

// A player sees their own country and merchant in full, but only the public
// side of other countries and none of the other merchants' gold.
func TestPlayerSeesOnlyOwnSecrets(t *testing.T) {
	state := twoKingdoms()
	state.GetCountry("Britannia").ArmyStrength = 7
	state.GetCountry("Britannia").PublicArmy = 3

	view := jsonapi.SerializeStateForPlayer(state, "anna")

	avalon := view.Countries["Avalon"]
	if avalon.Hidden || avalon.Gold != 10 || avalon.Peasants != 5 {
		t.Errorf("anna should see her own country in full, got %+v", avalon)
	}
	britannia := view.Countries["Britannia"]
	if !britannia.Hidden || britannia.Gold != 0 || britannia.Peasants != 0 || britannia.RevoltRisk != 0 {
		t.Errorf("anna should not see Britannia's secrets, got %+v", britannia)
	}
	if britannia.ArmyStrength != 3 {
		t.Errorf("anna should see Britannia's army as of the last war (3), got %d", britannia.ArmyStrength)
	}
	if britannia.HP != 10 || britannia.MonarchID != "bob" {
		t.Errorf("health and ruler are public, got %+v", britannia)
	}

	if m := view.Merchants["anna"]; m.Hidden || m.StoredGold != 5 {
		t.Errorf("anna should see her own gold, got %+v", m)
	}
	if m := view.Merchants["ben"]; !m.Hidden || m.StoredGold != 0 || m.CountryID != "Britannia" {
		t.Errorf("anna should see where ben is but not his gold, got %+v", m)
	}

	// Her own monarch does not get to see her gold either
	if m := jsonapi.SerializeStateForPlayer(state, "alice").Merchants["anna"]; !m.Hidden {
		t.Errorf("alice should not see anna's gold, got %+v", m)
	}

	// The full state (what the admin gets) is untouched
	if full := jsonapi.SerializeState(state); full.Countries["Britannia"].ArmyStrength != 7 || full.Merchants["ben"].StoredGold != 5 {
		t.Errorf("the admin view should show everything, got %+v", full.Countries["Britannia"])
	}
}

// Armies built during spending stay secret until the war phase is over; then
// everyone learns the strength left after maintenance.
func TestArmiesArePublishedAfterWar(t *testing.T) {
	state := twoKingdoms()

	spending := phases.NewSpendingPhase(engine.NewFixedDice(1))
	state, _ = spending.Execute(state, []actions.Action{
		actions.NewBuildArmyAction("bob", "Britannia", 8),
	})
	if got := jsonapi.SerializeStateForPlayer(state, "alice").Countries["Britannia"].ArmyStrength; got != 0 {
		t.Errorf("before the war alice should still see Britannia's old army (0), got %d", got)
	}

	war := phases.NewWarPhase(engine.NewFixedDice(1))
	state, _ = war.Execute(state, nil)
	if got := jsonapi.SerializeStateForPlayer(state, "alice").Countries["Britannia"].ArmyStrength; got != 4 {
		t.Errorf("after the war alice should see Britannia's halved army (4), got %d", got)
	}
}

// A monarch without any merchants can still tax their peasants.
func TestMonarchWithoutMerchantsCanTax(t *testing.T) {
	state := engine.NewGameState()
	state.AddCountry(engine.NewCountry("Avalon", "alice"))

	phase := phases.NewTaxationPhase(engine.NewFixedDice(6))
	valid := phase.ValidActions(state, "alice")
	if len(valid) != 2 {
		t.Fatalf("alice should be offered low and high tax, got %d actions", len(valid))
	}

	newState, _ := phase.Execute(state, []actions.Action{actions.NewTaxPeasantsAction("alice", "Avalon", true)})
	if got := newState.GetCountry("Avalon").Gold; got != 20 {
		t.Errorf("high tax on 5 peasants should raise 10 gold (10 + 10), got %d", got)
	}
}

// A monarch who does not choose a peasant tax taxes low: 5 peasants pay 5
// gold and the revolt risk goes back to its minimum.
func TestMonarchWhoDoesNotChooseTaxesLow(t *testing.T) {
	state := twoKingdoms()
	state.GetCountry("Avalon").RevoltRisk = 4

	phase := phases.NewTaxationPhase(engine.NewFixedDice(1))
	newState, _ := phase.Execute(state, []actions.Action{
		actions.NewTaxPeasantsAction("bob", "Britannia", true),
	})

	avalon := newState.GetCountry("Avalon")
	if avalon.Gold != 15 || avalon.RevoltRisk != 2 || avalon.HP != 10 {
		t.Errorf("alice chose nothing, so Avalon should tax low (10 + 5 gold, risk 2), got %+v", avalon)
	}
	// bob's own choice still counts: high tax with a roll of 1 means revolt
	if britannia := newState.GetCountry("Britannia"); britannia.HP != 8 || britannia.Gold != 10 {
		t.Errorf("bob's high tax should have caused a revolt, got %+v", britannia)
	}
}

func send(t *testing.T, api *jsonapi.GameAPI, request map[string]any) map[string]any {
	t.Helper()
	data, _ := json.Marshal(request)
	resp, err := api.ProcessMessage(data)
	if err != nil {
		t.Fatalf("ProcessMessage error: %v", err)
	}
	var result map[string]any
	json.Unmarshal(resp, &result)
	return result
}

func assignRole(t *testing.T, api *jsonapi.GameAPI, playerID, role, countryID string) map[string]any {
	t.Helper()
	return send(t, api, map[string]any{"type": "assign_role", "player_id": playerID, "role": role, "country_id": countryID})
}

func TestAssignRole(t *testing.T) {
	api := jsonapi.NewGameAPIWithDice(engine.NewFixedDice(1))
	api.GetEngine().SetState(twoKingdoms())
	state := func() *engine.GameState { return api.GetEngine().GetState() }

	// alice has a tax queued that must not survive losing her throne
	send(t, api, map[string]any{"type": "submit", "action": map[string]any{"type": "tax_peasants_low", "player_id": "alice", "country_id": "Avalon"}})

	// Moving a merchant to another country keeps their gold
	state().GetMerchant("anna").StoredGold = 9
	if resp := assignRole(t, api, "anna", "merchant", "Britannia"); resp["success"] != true {
		t.Fatalf("moving anna failed: %v", resp)
	}
	if m := state().GetMerchant("anna"); m.CountryID != "Britannia" || m.StoredGold != 9 {
		t.Errorf("anna should be in Britannia with her 9 gold, got %+v", m)
	}

	// A merchant crowned monarch puts their gold into the treasury, and the old
	// monarch loses their role and their queued actions
	if resp := assignRole(t, api, "anna", "monarch", "Avalon"); resp["success"] != true {
		t.Fatalf("crowning anna failed: %v", resp)
	}
	avalon := state().GetCountry("Avalon")
	if avalon.MonarchID != "anna" || avalon.Gold != 19 || state().GetMerchant("anna") != nil {
		t.Errorf("anna should rule Avalon with 10 + 9 gold, got %+v", avalon)
	}
	if state().PlayerCountryID("alice") != "" {
		t.Errorf("alice should have no role left")
	}
	if n := len(api.GetEngine().GetPendingActions()); n != 0 {
		t.Errorf("alice's queued tax should have been cancelled, %d actions left", n)
	}

	// A monarch turned merchant leaves the throne empty
	if resp := assignRole(t, api, "bob", "merchant", "Avalon"); resp["success"] != true {
		t.Fatalf("demoting bob failed: %v", resp)
	}
	if state().GetCountry("Britannia").MonarchID != "" || state().GetMerchant("bob").CountryID != "Avalon" {
		t.Errorf("bob should be a merchant of Avalon and Britannia kingless")
	}

	// Crowning someone in a republic turns it back into a monarchy
	state().GetCountry("Britannia").BecomeRepublic()
	if resp := assignRole(t, api, "alice", "monarch", "Britannia"); resp["success"] != true {
		t.Fatalf("crowning alice failed: %v", resp)
	}
	if b := state().GetCountry("Britannia"); b.IsRepublic || b.MonarchID != "alice" {
		t.Errorf("Britannia should be alice's monarchy again, got %+v", b)
	}

	// Removing a player takes them out of the game
	if resp := assignRole(t, api, "ben", "none", ""); resp["success"] != true {
		t.Fatalf("removing ben failed: %v", resp)
	}
	if state().GetMerchant("ben") != nil {
		t.Errorf("ben should no longer be a merchant")
	}

	// Mistakes are rejected
	for _, bad := range []struct{ player, role, country string }{
		{"anna", "monarch", "Nowhere"},
		{"anna", "king", "Avalon"},
		{"anna", "monarch", "Avalon"}, // already rules it
		{"", "merchant", "Avalon"},
	} {
		if resp := assignRole(t, api, bad.player, bad.role, bad.country); resp["success"] != false {
			t.Errorf("assign %+v should be rejected, got %v", bad, resp)
		}
	}
}
