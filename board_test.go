package main

import (
	"encoding/json"
	"strings"
	"testing"

	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
)

// The projector board hangs where everyone can see it, so these tests are
// mostly about what it must not show: army sizes before the war that reveals
// them, and merchant gold at any point in the game.

// ask sends one message to the game and fails the test if the game refuses it
func ask(t *testing.T, api *jsonapi.GameAPI, msg map[string]any) map[string]any {
	t.Helper()
	payload, _ := json.Marshal(msg)
	raw, err := api.ProcessMessage(payload)
	if err != nil {
		t.Fatalf("%v failed: %v", msg, err)
	}
	var resp map[string]any
	json.Unmarshal(raw, &resp)
	if resp["success"] != true {
		t.Fatalf("%v was refused: %v", msg, resp)
	}
	return resp
}

// advanceAndRecord resolves the phase in progress and hands the result to the
// board, the way the server does when the game leader presses Advance Phase
func advanceAndRecord(t *testing.T, api *jsonapi.GameAPI, board *BoardProjection) {
	t.Helper()
	state := api.GetEngine().GetState()
	turn, phase, before := state.Turn, state.Phase.String(), state.Clone()

	raw, err := api.ProcessMessage([]byte(`{"type":"advance"}`))
	if err != nil {
		t.Fatalf("advancing %s failed: %v", phase, err)
	}
	var resp struct {
		Success bool                `json:"success"`
		Events  []jsonapi.EventJSON `json:"events"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || !resp.Success {
		t.Fatalf("advancing %s failed: %v %s", phase, err, raw)
	}
	board.RecordPhase(turn, phase, resp.Events, before, api.GetEngine().GetState())
}

// twoRealms starts a game of Avalon (alice, with merchant anna) against
// Britannia (bob, with merchant ben)
func twoRealms(t *testing.T) (*jsonapi.GameAPI, *BoardProjection) {
	t.Helper()
	// A 6 never triggers a peasant revolt, so a test only sees the revolts it
	// asks for
	api := jsonapi.NewGameAPIWithDice(engine.NewFixedDice(6))
	ask(t, api, map[string]any{"type": "add_country", "country_id": "Avalon", "monarch_id": "alice"})
	ask(t, api, map[string]any{"type": "add_country", "country_id": "Britannia", "monarch_id": "bob"})
	ask(t, api, map[string]any{"type": "add_merchant", "player_id": "anna", "country_id": "Avalon"})
	ask(t, api, map[string]any{"type": "add_merchant", "player_id": "ben", "country_id": "Britannia"})
	return api, NewBoardProjection()
}

func submit(t *testing.T, api *jsonapi.GameAPI, action map[string]any) {
	t.Helper()
	ask(t, api, map[string]any{"type": "submit", "action": action})
}

// allText is every word the board would put on screen, from the beats of the
// last phase and from the chronicle
func allText(data *BoardData) string {
	var sb strings.Builder
	if data.LastPhase != nil {
		for _, beat := range data.LastPhase.Beats {
			sb.WriteString(beat.Text + "\n")
		}
	}
	for _, round := range data.Chronicle {
		for _, phase := range round.Phases {
			sb.WriteString(strings.Join(phase.Lines, "\n") + "\n")
		}
	}
	return sb.String()
}

func projected(board *BoardProjection) *BoardData {
	return &BoardData{LastPhase: board.lastPhase, Chronicle: board.chronicle}
}

// An army built in the Spending phase is nobody's business until the War phase
// publishes every army. Until then the board says nothing about it at all - not
// even that a country spent nothing, which would give the game away by itself.
func TestBoardHoldsArmyNumbersBackUntilTheWarIsOver(t *testing.T) {
	api, board := twoRealms(t)

	advanceAndRecord(t, api, board) // Taxation: both realms collect 5 gold
	advanceAndRecord(t, api, board) // Negotiation

	submit(t, api, map[string]any{"type": "build_army", "player_id": "alice", "country_id": "Avalon", "amount": 6})
	submit(t, api, map[string]any{"type": "build_army", "player_id": "bob", "country_id": "Britannia", "amount": 3})
	advanceAndRecord(t, api, board)

	if len(board.chronicle) != 0 {
		t.Errorf("no war has finished yet, so nothing should be unsealed: %+v", board.chronicle)
	}
	if beats := board.lastPhase.Beats; len(beats) != 1 || beats[0].Kind != "quiet" {
		t.Errorf("the Spending phase is entirely secret, so the board should only say so, got %+v", beats)
	}
	if text := allText(projected(board)); strings.Contains(text, "6") || strings.Contains(text, "3") {
		t.Errorf("the board must not hint at what was built: %q", text)
	}

	// The war reveals the armies, and with them the round that built them
	submit(t, api, map[string]any{"type": "attack", "player_id": "alice", "country_id": "Avalon", "target_id": "Britannia"})
	advanceAndRecord(t, api, board)

	if len(board.chronicle) != 1 || board.chronicle[0].Turn != 1 {
		t.Fatalf("the finished war should unseal round 1, got %+v", board.chronicle)
	}
	text := allText(projected(board))
	for _, want := range []string{
		"Avalon spent 6 gold on its army",
		"Britannia spent 3 gold on its army",
		"Avalon took the low tax and collected 5 gold",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the unsealed round should record %q, got:\n%s", want, text)
		}
	}
	if len(board.sealed) != 0 {
		t.Errorf("nothing should still be sealed after the war, got %+v", board.sealed)
	}
}

// The war phase is the one moment armies become public, so its beats carry the
// real strengths, who won, and the health the loser gave up.
func TestBoardWarBeatsShowTheBattleAndTheDamage(t *testing.T) {
	api, board := twoRealms(t)

	advanceAndRecord(t, api, board) // Taxation
	advanceAndRecord(t, api, board) // Negotiation
	submit(t, api, map[string]any{"type": "build_army", "player_id": "alice", "country_id": "Avalon", "amount": 6})
	submit(t, api, map[string]any{"type": "build_army", "player_id": "bob", "country_id": "Britannia", "amount": 3})
	advanceAndRecord(t, api, board) // Spending
	submit(t, api, map[string]any{"type": "attack", "player_id": "alice", "country_id": "Avalon", "target_id": "Britannia"})
	advanceAndRecord(t, api, board) // War

	var battle *BoardBattle
	var damage *BoardChange
	for _, beat := range board.lastPhase.Beats {
		if beat.Battle != nil {
			battle = beat.Battle
		}
		if beat.Kind == "damage" {
			damage = beat.Change
		}
	}

	if battle == nil {
		t.Fatalf("the war should have a battle to draw, got %+v", board.lastPhase.Beats)
	}
	if battle.AttackerID != "Avalon" || battle.AttackerStrength != 6 {
		t.Errorf("Avalon should attack with 6, got %+v", battle)
	}
	if battle.DefenderID != "Britannia" || battle.DefenderStrength != 3 {
		t.Errorf("Britannia should defend with 3, got %+v", battle)
	}
	if battle.WinnerID != "Avalon" || battle.LoserID != "Britannia" || battle.Damage != 3 {
		t.Errorf("Avalon should win by 3, got %+v", battle)
	}
	if battle.AttackerRepublic || battle.DefenderRepublic {
		t.Errorf("both realms are monarchies, got %+v", battle)
	}

	if damage == nil {
		t.Fatalf("the board should count Britannia's health down, got %+v", board.lastPhase.Beats)
	}
	if damage.Subject != "Britannia" || damage.From != 10 || damage.To != 7 || damage.Max != boardMaxHP {
		t.Errorf("Britannia should go from 10 to 7 health, got %+v", damage)
	}

	// Upkeep names every army, which is the moment they all become public
	var armies []BoardChange
	for _, beat := range board.lastPhase.Beats {
		if beat.Kind == "upkeep" {
			armies = beat.Armies
		}
	}
	if len(armies) != 2 {
		t.Fatalf("upkeep should name both armies, got %+v", armies)
	}
	if armies[0].Subject != "Avalon" || armies[0].From != 6 || armies[0].To != 3 {
		t.Errorf("Avalon's army should be halved from 6 to 3, got %+v", armies[0])
	}
	if armies[1].Subject != "Britannia" || armies[1].From != 3 || armies[1].To != 1 {
		t.Errorf("Britannia's army should be halved from 3 to 1, got %+v", armies[1])
	}
}

// Merchant gold is secret for the whole game: no rule ever reveals a purse, a
// hiding place or an investment, so none of it reaches the board - not in the
// live beats, and not in the chronicle a finished war unseals.
func TestBoardNeverShowsMerchantGold(t *testing.T) {
	api, board := twoRealms(t)
	// Numbers chosen to be unmistakable if they ever show up on screen
	const (
		taxed    = 4242
		hidden   = 3131
		invested = 2727
	)
	api.GetEngine().GetState().GetMerchant("anna").StoredGold = 12000

	submit(t, api, map[string]any{"type": "tax_merchants", "player_id": "alice",
		"country_id": "Avalon", "merchant_id": "anna", "amount": taxed})
	advanceAndRecord(t, api, board) // Taxation
	advanceAndRecord(t, api, board) // Negotiation

	submit(t, api, map[string]any{"type": "merchant_hide", "player_id": "anna", "merchant_id": "anna", "amount": hidden})
	submit(t, api, map[string]any{"type": "merchant_invest", "player_id": "anna", "merchant_id": "anna", "amount": invested})
	advanceAndRecord(t, api, board) // Spending
	advanceAndRecord(t, api, board) // War, which unseals the round
	advanceAndRecord(t, api, board) // Assessment, where the investment pays out

	// Whatever the board sends the page, the secrets must not be in it
	payload, err := json.Marshal(projected(board))
	if err != nil {
		t.Fatalf("cannot read the board: %v", err)
	}
	for name, secret := range map[string]string{
		"the tax anna paid": "4242",
		"the gold anna hid": "3131",
		"anna's investment": "2727",
		"anna's payout":     "5454",
		"anna's purse":      "12000",
	} {
		if strings.Contains(string(payload), secret) {
			t.Errorf("%s (%s) must not reach the board:\n%s", name, secret, payload)
		}
	}

	// It is the gold that is secret, not that anna exists
	if text := allText(projected(board)); strings.Contains(strings.ToLower(text), "gold tax") {
		t.Errorf("no merchant tax should be reported: %q", text)
	}
}

// A country spared its first death is a moment worth its own beat, and it is
// public: everyone can see the country still standing on 1 health.
func TestBoardTellsARealmSparedItsFirstDeath(t *testing.T) {
	before := engine.NewGameState()
	before.AddCountry(engine.NewCountry("Avalon", "alice"))
	before.GetCountry("Avalon").HP = 4

	after := before.Clone()
	after.GetCountry("Avalon").TakeDamage(9) // Spared: left on 1 HP, died_once set

	beats := damageBeats([]string{"Avalon"}, before, after)
	if len(beats) != 1 {
		t.Fatalf("one realm was hurt, so there should be one beat, got %+v", beats)
	}
	if !strings.Contains(beats[0].Text, "should have fallen") {
		t.Errorf("the beat should say Avalon was spared, got %q", beats[0].Text)
	}
	if c := beats[0].Change; c == nil || c.From != 4 || c.To != 1 {
		t.Errorf("the health should count from 4 down to 1, got %+v", beats[0].Change)
	}

	// And the second time, it really falls
	fallen := after.Clone()
	fallen.GetCountry("Avalon").TakeDamage(3)
	beats = damageBeats([]string{"Avalon"}, after, fallen)
	if len(beats) != 1 || !strings.Contains(beats[0].Text, "falls") {
		t.Errorf("the second defeat should be final, got %+v", beats)
	}
}

// The standing beside the beats is public facts only: health, ruler,
// government, where the merchants are, and the army as the last war left it -
// never the army a country has been building since.
func TestBoardStandingShowsOnlyThePublicArmy(t *testing.T) {
	t.Chdir(t.TempDir()) // NewServer names the game after a free history file
	s := NewServer()

	state := s.api.GetEngine().GetState()
	state.AddCountry(engine.NewCountry("Avalon", "alice"))
	state.AddCountry(engine.NewCountry("Britannia", "bob"))
	state.AddMerchant(engine.NewMerchant("anna", "Avalon"))
	state.AddMerchant(engine.NewMerchant("ben", "Britannia"))

	avalon := state.GetCountry("Avalon")
	// Unmistakable numbers, so the check below cannot match them by accident
	avalon.ArmyStrength = 3131 // Built since the last war: still secret
	avalon.PublicArmy = 2      // What the last war revealed
	avalon.Gold = 4242
	britannia := state.GetCountry("Britannia")
	britannia.BecomeRepublic()
	// ben has moved on and only arrives next round
	state.GetMerchant("ben").MoveTo("Avalon")

	data := s.boardData()

	if len(data.Realms) != 2 {
		t.Fatalf("both realms should be on the board, got %+v", data.Realms)
	}
	realm := data.Realms[0]
	if realm.CountryID != "Avalon" {
		t.Fatalf("the realms should be in name order, got %+v", data.Realms)
	}
	if realm.ArmyLastWar != 2 {
		t.Errorf("the board should show the army as of the last war (2), got %d", realm.ArmyLastWar)
	}
	if realm.Ruler != "alice" || realm.IsRepublic || realm.HP != 10 || !realm.Alive {
		t.Errorf("Avalon's public standing is wrong: %+v", realm)
	}
	if len(realm.Merchants) != 1 || realm.Merchants[0] != "anna" {
		t.Errorf("only anna is in Avalon so far, got %+v", realm.Merchants)
	}
	if data.Realms[1].Ruler != "" || !data.Realms[1].IsRepublic {
		t.Errorf("Britannia is a republic with no ruler, got %+v", data.Realms[1])
	}

	if len(data.Travellers) != 1 || data.Travellers[0].PlayerID != "ben" || data.Travellers[0].ToCountry != "Avalon" {
		t.Errorf("ben should be shown on the road to Avalon, got %+v", data.Travellers)
	}

	// The treasury and the army being built are not on the board at all
	payload, _ := json.Marshal(data)
	if strings.Contains(string(payload), "4242") {
		t.Errorf("the treasury (4242) does not belong on the board:\n%s", payload)
	}
	if strings.Contains(string(payload), "3131") {
		t.Errorf("the army built since the last war (3131) does not belong on the board:\n%s", payload)
	}
}

// Starting a new game clears the board, and the revision keeps climbing so the
// board page notices rather than mistaking the new game for the old one.
func TestNewGameClearsTheBoard(t *testing.T) {
	t.Chdir(t.TempDir())
	s := NewServer()
	state := s.api.GetEngine().GetState()
	state.AddCountry(engine.NewCountry("Avalon", "alice"))

	s.board.RecordPhase(1, "war", []jsonapi.EventJSON{
		{Type: "army_maintenance", Data: map[string]interface{}{
			"country_id": "Avalon", "old_strength": 4, "new_strength": 2}},
	}, state.Clone(), state)

	was := s.board.revision
	if s.board.lastPhase == nil || len(s.board.chronicle) != 1 {
		t.Fatalf("the war should be on the board, got %+v", s.board)
	}

	s.startNewGame()

	if s.board.lastPhase != nil || len(s.board.chronicle) != 0 || len(s.board.sealed) != 0 {
		t.Errorf("a new game should leave an empty board, got %+v", s.board)
	}
	if s.board.revision <= was {
		t.Errorf("the revision should keep climbing, got %d after %d", s.board.revision, was)
	}
	if data := s.boardData(); data.LastPhase != nil || len(data.Chronicle) != 0 {
		t.Errorf("the board should have nothing to replay, got %+v", data)
	}
}

// Every phase gets a beat even when nothing public happened, and the wording
// never changes with what did happen, so a quiet phase is not a tell.
func TestBoardAlwaysHasSomethingToSay(t *testing.T) {
	state := engine.NewGameState()
	state.AddCountry(engine.NewCountry("Avalon", "alice"))

	for _, phase := range []string{"taxation", "negotiation", "spending", "assessment"} {
		beats := publicBeats(phase, nil, state, state)
		if len(beats) != 1 || beats[0].Kind != "quiet" || beats[0].Text == "" {
			t.Errorf("%s with nothing public should have one quiet beat, got %+v", phase, beats)
		}
	}

	// A war where nobody marched says so, rather than showing an empty screen
	beats := publicBeats("war", nil, state, state)
	if len(beats) != 1 || !strings.Contains(beats[0].Text, "No realm marches") {
		t.Errorf("a war with no battles should say so, got %+v", beats)
	}
}
