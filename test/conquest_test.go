package test

import (
	"testing"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/phases"
)

// conquestSetup builds Carthage (on its last life, 5 peasants, 3 merchants)
// about to be destroyed by both Avalon and Britannia.
func conquestSetup() *engine.GameState {
	state := engine.NewGameState()
	for _, c := range []*engine.Country{
		engine.NewCountry("Avalon", "alice"),
		engine.NewCountry("Britannia", "bob"),
		engine.NewCountry("Carthage", "carol"),
	} {
		state.AddCountry(c)
	}
	state.GetCountry("Avalon").ArmyStrength = 20
	state.GetCountry("Britannia").ArmyStrength = 20
	state.GetCountry("Carthage").DiedOnce = true
	for _, id := range []string{"m1", "m2", "m3"} {
		state.AddMerchant(engine.NewMerchant(id, "Carthage"))
	}
	return state
}

func conquer(state *engine.GameState, seed int64) *engine.GameState {
	newState, _ := phases.NewWarPhase(engine.NewSeededDice(seed)).Execute(state, []actions.Action{
		actions.NewAttackAction("alice", "Avalon", "Carthage"),
		actions.NewAttackAction("bob", "Britannia", "Carthage"),
	})
	return newState
}

// With two winners the spoils are still split evenly, but which of them gets
// the leftover merchant and peasant is up to the dice, not the alphabet.
func TestConquestSpoilsGoToARandomWinner(t *testing.T) {
	extraPeasantWinners := make(map[string]bool)
	for seed := int64(1); seed <= 40; seed++ {
		state := conquer(conquestSetup(), seed)

		if state.GetCountry("Carthage").IsAlive() {
			t.Fatalf("seed %d: Carthage should be destroyed", seed)
		}

		// 5 peasants on top of the 5 each winner started with: 3 + 2
		avalon := state.GetCountry("Avalon").Peasants - 5
		britannia := state.GetCountry("Britannia").Peasants - 5
		if avalon+britannia != 5 || (avalon != 3 && britannia != 3) {
			t.Fatalf("seed %d: peasants split %d/%d, want 3/2", seed, avalon, britannia)
		}
		extra := "Avalon"
		if britannia == 3 {
			extra = "Britannia"
		}
		extraPeasantWinners[extra] = true

		// The same winner also gets the leftover merchant (2 of 3)
		merchants := len(state.GetMerchantsByCountry(extra))
		if state.GetMerchant("carol") != nil && state.GetMerchant("carol").CountryID == extra {
			merchants-- // the deposed monarch is placed separately
		}
		if merchants != 2 {
			t.Fatalf("seed %d: %s got the extra peasant but %d merchants, want 2", seed, extra, merchants)
		}
	}

	if !extraPeasantWinners["Avalon"] || !extraPeasantWinners["Britannia"] {
		t.Errorf("the leftovers should go to either winner depending on the dice, got %v", extraPeasantWinners)
	}
}

// A lone conqueror takes every merchant and peasant.
func TestLoneConquerorTakesEverything(t *testing.T) {
	state := conquestSetup()
	newState, _ := phases.NewWarPhase(engine.NewSeededDice(1)).Execute(state, []actions.Action{
		actions.NewAttackAction("alice", "Avalon", "Carthage"),
	})

	if got := newState.GetCountry("Avalon").Peasants; got != 10 {
		t.Errorf("Avalon should have 5 + 5 peasants, got %d", got)
	}
	for _, id := range []string{"m1", "m2", "m3", "carol"} {
		if got := newState.GetMerchant(id).CountryID; got != "Avalon" {
			t.Errorf("%s should now belong to Avalon, got %s", id, got)
		}
	}
}
