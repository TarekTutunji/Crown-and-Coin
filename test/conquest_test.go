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

// Scenario 9: two attackers conquer a country with a treasury of 11 and three
// merchants. Each winner gets 5 gold of the treasury and the dice give out the
// extra 1 (and the extra peasant). The merchants and the fallen monarch are
// split between the winners at random, and the merchants' investments still
// pay out double at the start of the next round in their new country.
func TestConquestSpoilsAreSplitAtRandom(t *testing.T) {
	extraWinners := make(map[string]bool)
	carolWent := make(map[string]bool)
	m1PairedWith := make(map[string]bool)
	for seed := int64(1); seed <= 40; seed++ {
		state := conquestSetup()
		state.GetCountry("Carthage").Gold = 11
		state.GetMerchant("m1").InvestedGold = 4
		state = conquer(state, seed)

		if state.GetCountry("Carthage").IsAlive() {
			t.Fatalf("seed %d: Carthage should be destroyed", seed)
		}
		if got := state.GetCountry("Carthage").Gold; got != 0 {
			t.Fatalf("seed %d: Carthage's treasury should be gone, got %d", seed, got)
		}

		// Each winner started with 10 gold and earned 5 for winning, then got
		// 5 or 6 of Carthage's 11
		avalonGold := state.GetCountry("Avalon").Gold - 15
		britanniaGold := state.GetCountry("Britannia").Gold - 15
		if avalonGold+britanniaGold != 11 || (avalonGold != 6 && britanniaGold != 6) {
			t.Fatalf("seed %d: treasury split %d/%d, want 6/5", seed, avalonGold, britanniaGold)
		}
		extra := "Avalon"
		if britanniaGold == 6 {
			extra = "Britannia"
		}
		extraWinners[extra] = true

		// The same winner gets the extra peasant: 5 on top of their own 5 is 3 + 2
		if got := state.GetCountry(extra).Peasants; got != 8 {
			t.Fatalf("seed %d: %s got the extra gold, so should have 5 + 3 peasants, got %d", seed, extra, got)
		}

		// Three merchants plus carol, the fallen monarch, go two each way
		counts := make(map[string]int)
		for _, id := range []string{"m1", "m2", "m3", "carol"} {
			m := state.GetMerchant(id)
			if m == nil || !m.Arriving {
				t.Fatalf("seed %d: %s should be on the way to a winner, got %+v", seed, id, m)
			}
			counts[m.CountryID]++
		}
		if counts["Avalon"] != 2 || counts["Britannia"] != 2 {
			t.Fatalf("seed %d: merchants split %v, want 2/2", seed, counts)
		}
		carol := state.GetMerchant("carol")
		// 5 from the bank, plus the income every merchant gets after the war
		if carol.StoredGold != 10 || carol.HiddenGold != 0 {
			t.Fatalf("seed %d: carol should start over with 5 gold and get 5 income, got %+v", seed, carol)
		}
		carolWent[carol.CountryID] = true
		for _, id := range []string{"m2", "m3", "carol"} {
			if state.GetMerchant(id).CountryID == state.GetMerchant("m1").CountryID {
				m1PairedWith[id] = true
			}
		}

		// m1 keeps the investment, which pays out 8 into the purse (on top of
		// the 5 starting gold and 5 income) at the start of the next round
		if got := state.GetMerchant("m1").InvestedGold; got != 4 {
			t.Fatalf("seed %d: m1 should keep the 4 invested, got %d", seed, got)
		}
		phases.StartRound(state)
		m1 := state.GetMerchant("m1")
		if m1.Arriving || m1.StoredGold != 18 || m1.InvestedGold != 0 {
			t.Fatalf("seed %d: m1 should have arrived with 5 + 5 + 8 gold, got %+v", seed, m1)
		}
	}

	if !extraWinners["Avalon"] || !extraWinners["Britannia"] {
		t.Errorf("the leftovers should go to either winner depending on the dice, got %v", extraWinners)
	}
	if !carolWent["Avalon"] || !carolWent["Britannia"] {
		t.Errorf("the fallen monarch should end up with either winner depending on the dice, got %v", carolWent)
	}
	if len(m1PairedWith) < 3 {
		t.Errorf("who ends up together should be up to the dice, m1 was only ever paired with %v", m1PairedWith)
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

// A country that falls apart from within scatters its merchants evenly over
// the survivors, and the dice decide which survivor gets the leftover one.
func TestCollapseLeftoversGoToARandomSurvivor(t *testing.T) {
	extraMerchantSurvivors := make(map[string]bool)
	for seed := int64(1); seed <= 40; seed++ {
		state := conquestSetup()
		state.GetCountry("Carthage").Eliminate()
		actions.CollapseCountry(state, "Carthage", "peasants", engine.NewSeededDice(seed))

		counts := make(map[string]int)
		for _, id := range []string{"m1", "m2", "m3"} {
			counts[state.GetMerchant(id).CountryID]++
		}
		if counts["Avalon"]+counts["Britannia"] != 3 || (counts["Avalon"] != 2 && counts["Britannia"] != 2) {
			t.Fatalf("seed %d: merchants split %v, want 2/1 over Avalon and Britannia", seed, counts)
		}
		if counts["Avalon"] == 2 {
			extraMerchantSurvivors["Avalon"] = true
		} else {
			extraMerchantSurvivors["Britannia"] = true
		}
	}

	if !extraMerchantSurvivors["Avalon"] || !extraMerchantSurvivors["Britannia"] {
		t.Errorf("the leftover merchant should go to either survivor depending on the dice, got %v", extraMerchantSurvivors)
	}
}
