package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
)

// A whole sample game played through the real server, watching the board at
// every step. The narrative is printed (go test -v) so it can be read through,
// but what is checked are the rules the board must never break, whatever the
// game does.

// fetchBoard reads the board the way the projector page does: plain HTTP, no
// login, no player identity
func fetchBoard(t *testing.T, url string) (*BoardData, map[string]any) {
	t.Helper()
	resp, err := http.Get(url + "/board.json")
	if err != nil {
		t.Fatalf("the board is unreachable: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the board answered %s", resp.Status)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("the board sent something unreadable: %v", err)
	}
	var typed BoardData
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatalf("the board sent something unreadable: %v", err)
	}
	var loose map[string]any
	json.Unmarshal(raw, &loose)
	return &typed, loose
}

// secretFieldNames are the names of everything the board must not carry a
// field for: no gold of any kind, no peasants, no revolt risk
var secretFieldNames = []string{"gold", "purse", "invest", "hidden", "treasur", "peasant", "revolt_risk"}

// checkNoSecretFields walks the whole payload looking for a field that should
// never be on a public board. Checking the field names, rather than the
// numbers, also catches a field somebody adds later.
func checkNoSecretFields(t *testing.T, where string, value any) {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		for key, inner := range v {
			for _, banned := range secretFieldNames {
				if strings.Contains(strings.ToLower(key), banned) {
					t.Errorf("the board must not have a %q field (at %s)", key, where)
				}
			}
			checkNoSecretFields(t, where+"."+key, inner)
		}
	case []any:
		for i, inner := range v {
			checkNoSecretFields(t, fmt.Sprintf("%s[%d]", where, i), inner)
		}
	}
}

// checkBoardIsAllowed applies the visibility rules to whatever the board is
// showing at this moment
func checkBoardIsAllowed(t *testing.T, data *BoardData, loose map[string]any, state *engine.GameState) {
	t.Helper()
	checkNoSecretFields(t, "board", loose)

	// The end-game summary is the one part of the board that carries gold, and
	// only the game leader calling the game may put it there
	if _, ok := loose["final"]; ok {
		t.Errorf("the game is still being played, so the board must carry no summary: %v", loose["final"])
	}
	if data.Final != nil {
		t.Errorf("the game is still being played, so the board must carry no summary: %+v", data.Final)
	}

	for _, realm := range data.Realms {
		country := state.GetCountry(realm.CountryID)
		if country == nil {
			t.Errorf("the board shows %s, which is not in the game", realm.CountryID)
			continue
		}
		if realm.ArmyLastWar != country.PublicArmy {
			t.Errorf("%s: the board should show the army as of the last war (%d), not %d",
				realm.CountryID, country.PublicArmy, realm.ArmyLastWar)
		}
		if realm.HP != max(0, country.HP) || realm.Alive != country.IsAlive() {
			t.Errorf("%s: health and standing should be public and current, got %+v", realm.CountryID, realm)
		}
	}

	// A live beat never mentions gold: country gold waits for the war that
	// unseals it, and merchant gold for the end of the game
	if data.LastPhase != nil {
		for _, beat := range data.LastPhase.Beats {
			if strings.Contains(strings.ToLower(beat.Text), "gold") {
				t.Errorf("a live beat must not mention gold: %q", beat.Text)
			}
			if beat.Text == "" {
				t.Errorf("every beat needs something to say: %+v", beat)
			}
		}
	}
}

// TestBoardThroughASampleGame plays three rounds with monarchs, merchants, a
// peasant revolt, battles and a conquest, and checks the board after every
// phase the game leader resolves.
func TestBoardThroughASampleGame(t *testing.T) {
	server, url := startServer(t)
	// Fixed dice so the sample game tells the same story every time. A roll of
	// 1 always loses the gamble on a high tax, so the peasants revolt.
	server.api = jsonapi.NewGameAPIWithDice(engine.NewFixedDice(1))

	admin := connect(t, url, "admin", "crown", false)

	realms := map[string]string{"Avalon": "alice", "Brittany": "bob", "Castile": "carol"}
	for _, name := range []string{"Avalon", "Brittany", "Castile"} {
		admin.ask(map[string]any{"type": "add_country", "country_id": name, "monarch_id": realms[name]})
	}
	merchants := map[string]string{"anna": "Avalon", "arne": "Avalon", "bram": "Brittany", "cato": "Castile"}
	for _, name := range []string{"anna", "arne", "bram", "cato"} {
		admin.ask(map[string]any{"type": "add_merchant", "player_id": name, "country_id": merchants[name]})
	}

	players := map[string]*wsPlayer{}
	for _, name := range []string{"alice", "bob", "carol", "anna", "arne", "bram", "cato"} {
		players[name] = connect(t, url, name, "secret-"+name, true)
	}

	move := func(name string, action map[string]any) {
		t.Helper()
		action["player_id"] = name
		if resp := players[name].ask(map[string]any{"type": "submit", "action": action}); resp == nil || resp["success"] != true {
			t.Fatalf("%s could not play %v: %v", name, action, resp)
		}
	}

	// The board is reachable and empty before anything has been resolved
	data, loose := fetchBoard(t, url)
	if data.LastPhase != nil || len(data.Chronicle) != 0 {
		t.Errorf("nothing has been resolved yet, so there is nothing to replay: %+v", data)
	}
	if len(data.Realms) != 3 {
		t.Fatalf("all three realms should be on the board, got %+v", data.Realms)
	}
	checkBoardIsAllowed(t, data, loose, server.api.GetEngine().GetState())

	advance := func() *BoardData {
		t.Helper()
		before := server.api.GetEngine().GetState()
		turn, phase := before.Turn, before.Phase.String()

		if resp := admin.ask(map[string]any{"type": "advance"}); resp == nil || resp["success"] != true {
			t.Fatalf("the game leader could not resolve %s: %v", phase, resp)
		}

		data, loose := fetchBoard(t, url)
		checkBoardIsAllowed(t, data, loose, server.api.GetEngine().GetState())

		if data.LastPhase == nil || data.LastPhase.Turn != turn || data.LastPhase.Phase != phase {
			t.Fatalf("the board should be replaying round %d %s, got %+v", turn, phase, data.LastPhase)
		}
		if len(data.LastPhase.Beats) == 0 {
			t.Errorf("round %d %s left the board with nothing to play", turn, phase)
		}

		t.Logf("--- Round %d, %s ---", turn, phase)
		for _, beat := range data.LastPhase.Beats {
			t.Logf("  [%s] %s", beat.Kind, beat.Text)
			if beat.Battle != nil {
				b := beat.Battle
				t.Logf("        battle: %s (%d) vs %s (%d), winner %q",
					b.AttackerID, b.AttackerStrength, b.DefenderID, b.DefenderStrength, b.WinnerID)
			}
			if beat.Change != nil {
				t.Logf("        %s %s: %d -> %d", beat.Change.Subject, beat.Change.Label, beat.Change.From, beat.Change.To)
			}
			for _, army := range beat.Armies {
				t.Logf("        army %s: %d -> %d", army.Subject, army.From, army.To)
			}
		}
		return data
	}

	// ---- Round 1 ----

	// Castile gambles on a high tax and the peasants rise; the others play safe
	move("carol", map[string]any{"type": "tax_peasants_high", "country_id": "Castile"})
	move("alice", map[string]any{"type": "tax_peasants_low", "country_id": "Avalon"})
	move("alice", map[string]any{"type": "tax_merchants", "country_id": "Avalon", "merchant_id": "anna", "amount": 3})
	move("bob", map[string]any{"type": "tax_peasants_low", "country_id": "Brittany"})
	data = advance() // Taxation

	if len(data.Chronicle) != 0 {
		t.Errorf("no war has finished, so the taxes stay sealed: %+v", data.Chronicle)
	}
	if !hasBeat(data, "The peasants of Castile rise against the tax.") {
		t.Errorf("the peasant revolt in Castile is public and should be on the board: %+v", data.LastPhase.Beats)
	}

	advance() // Negotiation

	// Avalon builds a real army, Brittany a small one, Castile nothing
	move("alice", map[string]any{"type": "build_army", "country_id": "Avalon", "amount": 8})
	move("bob", map[string]any{"type": "build_army", "country_id": "Brittany", "amount": 3})
	move("anna", map[string]any{"type": "merchant_invest", "merchant_id": "anna", "amount": 2})
	move("arne", map[string]any{"type": "merchant_hide", "merchant_id": "arne", "amount": 4})
	data = advance() // Spending

	// Nothing about the armies, and nothing about who spent nothing either
	if beats := data.LastPhase.Beats; len(beats) != 1 || beats[0].Kind != "quiet" {
		t.Errorf("spending is secret, so the board should say only that: %+v", beats)
	}
	if len(data.Chronicle) != 0 {
		t.Errorf("the armies are still secret, so nothing is unsealed: %+v", data.Chronicle)
	}

	// Avalon marches on Brittany and wins
	move("alice", map[string]any{"type": "attack", "country_id": "Avalon", "target_id": "Brittany"})
	move("bob", map[string]any{"type": "no_attack", "country_id": "Brittany"})
	move("carol", map[string]any{"type": "no_attack", "country_id": "Castile"})
	data = advance() // War

	battle := findBattle(data)
	if battle == nil {
		t.Fatalf("the war should have a battle: %+v", data.LastPhase.Beats)
	}
	if battle.AttackerID != "Avalon" || battle.AttackerStrength != 8 || battle.DefenderStrength != 3 {
		t.Errorf("the board should show the real armies now, got %+v", battle)
	}
	if battle.WinnerID != "Avalon" || battle.LoserID != "Brittany" {
		t.Errorf("Avalon should have won, got %+v", battle)
	}
	if len(data.Chronicle) != 1 || data.Chronicle[0].Turn != 1 {
		t.Fatalf("the finished war should unseal round 1, got %+v", data.Chronicle)
	}
	chronicle := chronicleText(data)
	for _, want := range []string{"Avalon spent 8 gold on its army", "Brittany spent 3 gold on its army"} {
		if !strings.Contains(chronicle, want) {
			t.Errorf("the unsealed round should record %q, got:\n%s", want, chronicle)
		}
	}
	// anna paid 3 gold in tax, which is hers, and stays sealed for good
	if strings.Contains(chronicle, "anna") {
		t.Errorf("no merchant's gold belongs in the chronicle, got:\n%s", chronicle)
	}

	// bram gives up on Brittany
	move("bram", map[string]any{"type": "flee", "merchant_id": "bram", "target_id": "Castile"})
	move("anna", map[string]any{"type": "remain", "merchant_id": "anna"})
	data = advance() // Assessment

	if !hasBeat(data, "Merchant bram flees Brittany for Castile.") {
		t.Errorf("a merchant fleeing is public and should be on the board: %+v", data.LastPhase.Beats)
	}
	// The Assessment phase rolls the round over, so bram is already there
	if !hasBeat(data, "Merchant bram reaches Castile.") {
		t.Errorf("bram's arrival should be on the board: %+v", data.LastPhase.Beats)
	}
	if !realmHasMerchant(data, "Castile", "bram") {
		t.Errorf("bram should now stand in Castile, got %+v", data.Realms)
	}

	// ---- Round 2: Avalon beats Brittany down to its last breath ----

	move("alice", map[string]any{"type": "tax_peasants_low", "country_id": "Avalon"})
	advance() // Taxation
	advance() // Negotiation
	move("alice", map[string]any{"type": "build_army", "country_id": "Avalon", "amount": 9})
	advance() // Spending
	move("alice", map[string]any{"type": "attack", "country_id": "Avalon", "target_id": "Brittany"})
	data = advance() // War

	if !hasBeatContaining(data, "Brittany") {
		t.Errorf("Brittany's defeat should be on the board: %+v", data.LastPhase.Beats)
	}
	if len(data.Chronicle) != 2 {
		t.Errorf("round 2 should be unsealed too, got %+v", data.Chronicle)
	}

	if !hasBeatContaining(data, "should have fallen") {
		t.Errorf("Brittany being spared its first defeat should be on the board: %+v", data.LastPhase.Beats)
	}
	advance() // Assessment

	// ---- Round 3: Brittany falls for good and is annexed ----

	move("alice", map[string]any{"type": "tax_peasants_low", "country_id": "Avalon"})
	advance() // Taxation
	advance() // Negotiation
	move("alice", map[string]any{"type": "build_army", "country_id": "Avalon", "amount": 4})
	advance() // Spending
	move("alice", map[string]any{"type": "attack", "country_id": "Avalon", "target_id": "Brittany"})
	data = advance() // War

	if !hasBeat(data, "Brittany falls.") {
		t.Errorf("Brittany's second defeat is final and should be on the board: %+v", data.LastPhase.Beats)
	}
	if !hasBeatContaining(data, "annexes Brittany") {
		t.Errorf("the conquest should be on the board: %+v", data.LastPhase.Beats)
	}
	if !hasBeatContaining(data, "bob of Brittany is conquered") {
		t.Errorf("bob losing the throne should be on the board: %+v", data.LastPhase.Beats)
	}
	// A deposed monarch starts again as a merchant, and arrives next round
	if len(data.Travellers) != 1 || data.Travellers[0].PlayerID != "bob" {
		t.Errorf("bob should be shown on the road to his new home, got %+v", data.Travellers)
	}

	data = advance() // Assessment

	// Whatever became of Brittany, the board and the game agree on it
	state := server.api.GetEngine().GetState()
	for _, realm := range data.Realms {
		if realm.Alive != state.GetCountry(realm.CountryID).IsAlive() {
			t.Errorf("%s: the board disagrees with the game about who is still standing", realm.CountryID)
		}
	}
	if len(data.Chronicle) != 3 {
		t.Errorf("three rounds should be unsealed by now, got %+v", data.Chronicle)
	}

	// A new game wipes the board, as the game leader would expect
	if resp := admin.ask(map[string]any{"type": "new_game"}); resp == nil || resp["success"] != true {
		t.Fatalf("the game leader could not start a new game: %v", resp)
	}
	data, loose = fetchBoard(t, url)
	if data.LastPhase != nil || len(data.Chronicle) != 0 || len(data.Realms) != 0 {
		t.Errorf("a new game should leave an empty board, got %+v", data)
	}
	checkNoSecretFields(t, "board", loose)
}

func hasBeat(data *BoardData, text string) bool {
	if data.LastPhase == nil {
		return false
	}
	for _, beat := range data.LastPhase.Beats {
		if beat.Text == text {
			return true
		}
	}
	return false
}

func hasBeatContaining(data *BoardData, text string) bool {
	if data.LastPhase == nil {
		return false
	}
	for _, beat := range data.LastPhase.Beats {
		if strings.Contains(beat.Text, text) {
			return true
		}
	}
	return false
}

func realmHasMerchant(data *BoardData, countryID, merchantID string) bool {
	for _, realm := range data.Realms {
		if realm.CountryID != countryID {
			continue
		}
		for _, id := range realm.Merchants {
			if id == merchantID {
				return true
			}
		}
	}
	return false
}

func findBattle(data *BoardData) *BoardBattle {
	if data.LastPhase == nil {
		return nil
	}
	for _, beat := range data.LastPhase.Beats {
		if beat.Battle != nil {
			return beat.Battle
		}
	}
	return nil
}

func chronicleText(data *BoardData) string {
	var sb strings.Builder
	for _, round := range data.Chronicle {
		for _, phase := range round.Phases {
			for _, line := range phase.Lines {
				sb.WriteString(line + "\n")
			}
		}
	}
	return sb.String()
}
