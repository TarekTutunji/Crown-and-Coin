package test

import (
	"testing"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
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
