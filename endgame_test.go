package main

import (
	"strings"
	"testing"

	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
)

// The end-game summary is the one screen that reveals merchant gold, so these
// tests are about two things: that it only ever appears because the game leader
// asked for it, and that when it does, its numbers are the game's own.

func evt(eventType string, data map[string]interface{}) jsonapi.EventJSON {
	return jsonapi.EventJSON{Type: eventType, Data: data}
}

func TestStandingsPutTheStrongestFirstAndTheFallenLast(t *testing.T) {
	state := engine.NewGameState()
	state.AddCountry(&engine.Country{ID: "Middling", HP: 5, ArmyStrength: 9, Gold: 2, Peasants: 5})
	state.AddCountry(&engine.Country{ID: "Ruined", HP: 0, ArmyStrength: 40, Gold: 99, Peasants: 9, DiedOnce: true})
	state.AddCountry(&engine.Country{ID: "Strong", HP: 9, ArmyStrength: 1, Gold: 0, Peasants: 5})
	// Same health as Middling, and a bigger army, so it comes first of the two
	state.AddCountry(&engine.Country{ID: "Armed", HP: 5, ArmyStrength: 12, Gold: 0, Peasants: 5})

	var order []string
	for _, realm := range finalRealms(state) {
		order = append(order, realm.CountryID)
	}
	want := []string{"Strong", "Armed", "Middling", "Ruined"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("the standings should read %v, got %v", want, order)
		}
	}
}

func TestTheSummaryCountsEveryKindOfGold(t *testing.T) {
	state := engine.NewGameState()
	state.AddCountry(&engine.Country{ID: "Avalon", HP: 10, Gold: 7, Peasants: 5})
	state.AddMerchant(&engine.Merchant{ID: "rich", CountryID: "Avalon", StoredGold: 2, HiddenGold: 30, InvestedGold: 1})
	state.AddMerchant(&engine.Merchant{ID: "poor", CountryID: "Avalon", StoredGold: 1})
	// Two merchants on the same fortune share a rank
	state.AddMerchant(&engine.Merchant{ID: "abel", CountryID: "Avalon", StoredGold: 5, HiddenGold: 5})
	state.AddMerchant(&engine.Merchant{ID: "zeno", CountryID: "Avalon", InvestedGold: 10})

	merchants := finalMerchants(state)
	want := []struct {
		id    string
		rank  int
		total int
	}{
		{"rich", 1, 33},
		{"abel", 2, 10},
		{"zeno", 2, 10},
		{"poor", 4, 1},
	}
	if len(merchants) != len(want) {
		t.Fatalf("every merchant should be counted, got %+v", merchants)
	}
	for i, w := range want {
		got := merchants[i]
		if got.PlayerID != w.id || got.Rank != w.rank || got.Total != w.total {
			t.Errorf("place %d should be %s, rank %d, %d gold in all; got %+v", i+1, w.id, w.rank, w.total, got)
		}
	}

	// The hidden gold is the whole point of the screen, so it is broken out
	if merchants[0].Hidden != 30 || merchants[0].Purse != 2 || merchants[0].Invested != 1 {
		t.Errorf("the richest merchant's gold should be shown in full, got %+v", merchants[0])
	}
}

func TestTheTimelineKeepsTheStoryAndDropsTheBookkeeping(t *testing.T) {
	snapshots := []StateSnapshot{
		{Turn: 1, Phase: "taxation", Events: []jsonapi.EventJSON{
			evt("merchant_income", map[string]interface{}{"merchant_id": "anna", "amount": 5}),
			evt("peasant_tax", map[string]interface{}{"country_id": "Avalon", "amount": 5}),
			evt("peasant_revolt", map[string]interface{}{"country_id": "Castile", "damage": 2}),
		}},
		{Turn: 1, Phase: "war", Events: []jsonapi.EventJSON{
			evt("battle_resolved", map[string]interface{}{
				"attacker_id": "Avalon", "defender_id": "Brittany",
				"attacker_strength": 3, "defender_strength": 1,
				"winner_id": "Avalon", "damage": 2}),
			evt("army_maintenance", map[string]interface{}{"country_id": "Avalon", "old_strength": 3, "new_strength": 1}),
		}},
		{Turn: 2, Phase: "war", Events: []jsonapi.EventJSON{
			// The biggest clash of the game, by what both sides brought
			evt("battle_resolved", map[string]interface{}{
				"attacker_id": "Avalon", "defender_id": "Castile",
				"attacker_strength": 12, "defender_strength": 9,
				"winner_id": "Castile", "damage": 3}),
			evt("annexation", map[string]interface{}{
				"defeated_id": "Brittany", "winner_ids": []string{"Avalon"},
				"treasury": 4, "merchants": []string{"bram"}}),
			evt("monarch_deposed", map[string]interface{}{
				"monarch_id": "bob", "from_country": "Brittany", "to_country": "Castile", "reason": "conquest"}),
			evt("investment_payout", map[string]interface{}{"merchant_id": "anna", "amount": 4}),
		}},
	}

	timeline := gameTimeline(snapshots)
	if len(timeline) != 2 || timeline[0].Turn != 1 || timeline[1].Turn != 2 {
		t.Fatalf("the story should be two rounds, in order, got %+v", timeline)
	}

	// Round 1: the peasant revolt and the battle, but not income or the tax
	// itself, and not upkeep
	if len(timeline[0].Beats) != 2 {
		t.Fatalf("round 1 should keep the revolt and the battle, got %+v", timeline[0].Beats)
	}
	if timeline[0].Beats[0].Kind != "revolt" || timeline[0].Beats[1].Kind != "battle" {
		t.Errorf("round 1 should read revolt then battle, got %+v", timeline[0].Beats)
	}
	if timeline[0].Beats[0].Phase != "taxation" || timeline[0].Beats[1].Phase != "war" {
		t.Errorf("each beat should say which phase it happened in, got %+v", timeline[0].Beats)
	}

	// Round 2: the battle, the conquest and the fallen monarch, but not the
	// investment paying out
	if len(timeline[1].Beats) != 3 {
		t.Fatalf("round 2 should keep the battle, the conquest and the deposing, got %+v", timeline[1].Beats)
	}

	biggest := 0
	for _, round := range timeline {
		for _, beat := range round.Beats {
			if beat.Biggest {
				biggest++
				if beat.Kind != "battle" || beat.Phase != "war" || round.Turn != 2 {
					t.Errorf("the greatest clash was round 2's battle, not %+v", beat)
				}
			}
		}
	}
	if biggest != 1 {
		t.Errorf("exactly one battle was the greatest clash, %d were marked", biggest)
	}

	// No merchant's gold is in the story either, even though the game is over:
	// the timeline retells events, and what a merchant did with their coin was
	// never an event the room saw
	for _, round := range timeline {
		for _, beat := range round.Beats {
			if beat.Text == "" {
				t.Errorf("every beat needs something to say: %+v", beat)
			}
		}
	}
}

func TestATimelineOfNothingIsEmptyRatherThanMissing(t *testing.T) {
	timeline := gameTimeline([]StateSnapshot{
		{Turn: 1, Phase: "taxation", Events: []jsonapi.EventJSON{
			evt("merchant_income", map[string]interface{}{"merchant_id": "anna", "amount": 5}),
		}},
	})
	if timeline == nil || len(timeline) != 0 {
		t.Errorf("a quiet game should leave an empty story, not a missing one: %+v", timeline)
	}
}

// TestTheSummaryOnlyAppearsWhenTheGameLeaderCallsIt plays a short game over the
// real server and then ends it, which is the only way the board may ever show
// what the merchants were sitting on.
func TestTheSummaryOnlyAppearsWhenTheGameLeaderCallsIt(t *testing.T) {
	server, url := startServer(t)
	// A 6 never sets off a peasant revolt, so the game does only what it is told
	server.api = jsonapi.NewGameAPIWithDice(engine.NewFixedDice(6))

	admin := connect(t, url, "admin", "crown", false)
	admin.ask(map[string]any{"type": "add_country", "country_id": "Avalon", "monarch_id": "alice"})
	admin.ask(map[string]any{"type": "add_country", "country_id": "Brittany", "monarch_id": "bob"})
	admin.ask(map[string]any{"type": "add_merchant", "player_id": "anna", "country_id": "Avalon"})
	admin.ask(map[string]any{"type": "add_merchant", "player_id": "bram", "country_id": "Brittany"})

	players := map[string]*wsPlayer{}
	for _, name := range []string{"alice", "bob", "anna", "bram"} {
		players[name] = connect(t, url, name, "secret-"+name, true)
	}
	move := func(name string, action map[string]any) {
		t.Helper()
		action["player_id"] = name
		if resp := players[name].ask(map[string]any{"type": "submit", "action": action}); resp == nil || resp["success"] != true {
			t.Fatalf("%s could not play %v: %v", name, action, resp)
		}
	}

	// One round: taxes, a merchant hiding gold and another investing, then a
	// battle Avalon wins
	move("alice", map[string]any{"type": "tax_peasants_low", "country_id": "Avalon"})
	move("bob", map[string]any{"type": "tax_peasants_low", "country_id": "Brittany"})
	admin.ask(map[string]any{"type": "advance"}) // Taxation
	admin.ask(map[string]any{"type": "advance"}) // Negotiation
	move("alice", map[string]any{"type": "build_army", "country_id": "Avalon", "amount": 6})
	move("anna", map[string]any{"type": "merchant_hide", "merchant_id": "anna", "amount": 4})
	move("bram", map[string]any{"type": "merchant_invest", "merchant_id": "bram", "amount": 3})
	admin.ask(map[string]any{"type": "advance"}) // Spending
	move("alice", map[string]any{"type": "attack", "country_id": "Avalon", "target_id": "Brittany"})
	move("bob", map[string]any{"type": "no_attack", "country_id": "Brittany"})
	admin.ask(map[string]any{"type": "advance"}) // War

	// Nothing of the sort while the game is being played
	data, loose := fetchBoard(t, url)
	checkBoardIsAllowed(t, data, loose, server.api.GetEngine().GetState())

	// And no player may call the game over
	if r := players["anna"].ask(map[string]any{"type": "end_game", "ended": true}); r == nil || r["error"] != "permission denied" {
		t.Errorf("anna was allowed to end the game: %v", r)
	}
	data, _ = fetchBoard(t, url)
	if data.Final != nil {
		t.Fatalf("a player was refused, so the board must still be live: %+v", data.Final)
	}

	// The game leader calls it
	if r := admin.ask(map[string]any{"type": "end_game", "ended": true}); r == nil || r["success"] != true || r["ended"] != true {
		t.Fatalf("the game leader could not end the game: %v", r)
	}
	data, _ = fetchBoard(t, url)
	if data.Final == nil {
		t.Fatalf("the summary should be on the board now, got %+v", data)
	}
	final := data.Final
	state := server.api.GetEngine().GetState()

	// The summary and the game agree about every coin
	if len(final.Merchants) != len(state.Merchants) {
		t.Fatalf("every merchant should be in the summary, got %+v", final.Merchants)
	}
	for _, shown := range final.Merchants {
		real := state.GetMerchant(shown.PlayerID)
		if real == nil {
			t.Errorf("the summary shows %s, who is not in the game", shown.PlayerID)
			continue
		}
		if shown.Purse != real.StoredGold || shown.Hidden != real.HiddenGold ||
			shown.Invested != real.InvestedGold || shown.Total != real.TotalGold() {
			t.Errorf("%s: the summary says %+v, the game says purse %d, hidden %d, invested %d",
				shown.PlayerID, shown, real.StoredGold, real.HiddenGold, real.InvestedGold)
		}
	}
	for _, shown := range final.Realms {
		real := state.GetCountry(shown.CountryID)
		if real == nil {
			t.Errorf("the summary shows %s, which is not in the game", shown.CountryID)
			continue
		}
		// The army it really has now, not the one it was last seen with
		if shown.Army != real.ArmyStrength || shown.Treasury != real.Gold || shown.Peasants != real.Peasants {
			t.Errorf("%s: the summary says %+v, the game says army %d, treasury %d, peasants %d",
				shown.CountryID, shown, real.ArmyStrength, real.Gold, real.Peasants)
		}
	}

	// anna hid 4 gold, which no screen has shown all game, and the tally adds
	// the hidden gold up
	anna := findFinalMerchant(final, "anna")
	if anna == nil || anna.Hidden != 4 {
		t.Errorf("anna hid 4 gold and the summary should say so, got %+v", anna)
	}
	hidden, total := 0, 0
	for _, m := range final.Merchants {
		hidden += m.Hidden
		total += m.Total
	}
	if final.Tally.HiddenGold != hidden || final.Tally.MerchantGold != total {
		t.Errorf("the tally says %d hidden of %d, the merchants add up to %d of %d",
			final.Tally.HiddenGold, final.Tally.MerchantGold, hidden, total)
	}
	if final.Tally.Battles != 1 {
		t.Errorf("one battle was fought, the tally says %d", final.Tally.Battles)
	}
	if final.Tally.RealmsStanding != 2 || final.Tally.RealmsFallen != 0 {
		t.Errorf("both realms are still standing, the tally says %+v", final.Tally)
	}
	// The round is not over - Assessment has not been resolved - so no round has
	// been played to the end yet, whatever round the game is in
	if final.Tally.Rounds != 0 {
		t.Errorf("no round has been played out yet, the tally says %d", final.Tally.Rounds)
	}
	if final.Turn != 1 || final.Phase != "assessment" {
		t.Errorf("the summary should say where the game stopped, got round %d %s", final.Turn, final.Phase)
	}

	// The story of the game records the battle
	if !timelineMentions(final, "Avalon (6) attacked Brittany (0)") {
		t.Errorf("the battle should be in the story of the game, got %+v", final.Timeline)
	}

	// Ending the game changes nothing in the game itself: the phase, the round
	// and the players carry on exactly as they were
	if state.Turn != 1 || state.Phase.String() != "assessment" {
		t.Errorf("ending the game moved the game itself, to round %d %s", state.Turn, state.Phase)
	}
	if r := players["anna"].ask(map[string]any{"type": "get_state"}); r == nil || r["success"] != true {
		t.Errorf("anna cannot play on after the game was called: %v", r)
	}

	// The game leader can put the live board back
	if r := admin.ask(map[string]any{"type": "end_game", "ended": false}); r == nil || r["ended"] != false {
		t.Fatalf("the game leader could not dismiss the summary: %v", r)
	}
	data, loose = fetchBoard(t, url)
	checkBoardIsAllowed(t, data, loose, server.api.GetEngine().GetState())

	// And resolving another phase takes the summary down by itself, rather than
	// leaving a stale reveal on the wall
	admin.ask(map[string]any{"type": "end_game", "ended": true})
	if data, _ = fetchBoard(t, url); data.Final == nil {
		t.Fatalf("the summary should be up again: %+v", data)
	}
	admin.ask(map[string]any{"type": "advance"}) // Assessment
	data, loose = fetchBoard(t, url)
	checkBoardIsAllowed(t, data, loose, server.api.GetEngine().GetState())

	// That round was played to the end, so now it counts
	admin.ask(map[string]any{"type": "end_game", "ended": true})
	data, _ = fetchBoard(t, url)
	if data.Final == nil || data.Final.Tally.Rounds != 1 {
		t.Errorf("one round has been played out, the tally says %+v", data.Final)
	}
	admin.ask(map[string]any{"type": "end_game", "ended": false})

	// A new game leaves nothing of the old one to reveal
	admin.ask(map[string]any{"type": "end_game", "ended": true})
	if r := admin.ask(map[string]any{"type": "new_game"}); r == nil || r["success"] != true {
		t.Fatalf("the game leader could not start a new game: %v", r)
	}
	data, loose = fetchBoard(t, url)
	checkBoardIsAllowed(t, data, loose, server.api.GetEngine().GetState())
}

func findFinalMerchant(final *FinalSummary, id string) *FinalMerchant {
	for i := range final.Merchants {
		if final.Merchants[i].PlayerID == id {
			return &final.Merchants[i]
		}
	}
	return nil
}

func timelineMentions(final *FinalSummary, text string) bool {
	for _, round := range final.Timeline {
		for _, beat := range round.Beats {
			if strings.Contains(beat.Text, text) {
				return true
			}
		}
	}
	return false
}
