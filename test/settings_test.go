package test

import (
	"testing"

	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
	"crown_and_coin/phases"
)

func settingsOf(t *testing.T, resp map[string]any) (percent float64, open bool) {
	t.Helper()
	settings, ok := resp["settings"].(map[string]any)
	if !ok {
		t.Fatalf("no settings in %v", resp)
	}
	return settings["investment_return_percent"].(float64), settings["open_game"].(bool)
}

// A new game pays out double and keeps the secrecy rules. The game leader can
// change both, and a return outside 0% to 500% is refused.
func TestGameLeaderChangesSettings(t *testing.T) {
	api := jsonapi.NewGameAPIWithDice(engine.NewFixedDice(1))

	if percent, open := settingsOf(t, send(t, api, map[string]any{"type": "get_settings"})); percent != 200 || open {
		t.Fatalf("a new game should pay 200%% with the limited view, got %v%% open=%v", percent, open)
	}

	resp := send(t, api, map[string]any{"type": "set_settings", "investment_return_percent": 150, "open_game": true})
	if percent, open := settingsOf(t, resp); resp["success"] != true || percent != 150 || !open {
		t.Fatalf("the settings should now be 150%% and open, got %v", resp)
	}
	if !api.IsOpenGame() {
		t.Error("the game should be open")
	}

	// Changing one setting leaves the other alone
	resp = send(t, api, map[string]any{"type": "set_settings", "open_game": false})
	if percent, open := settingsOf(t, resp); percent != 150 || open {
		t.Errorf("only the view should change, got %v", resp)
	}

	resp = send(t, api, map[string]any{"type": "set_settings", "investment_return_percent": 501})
	if percent, _ := settingsOf(t, resp); resp["success"] != false || percent != 150 {
		t.Errorf("a return of 501%% should be refused and leave 150%%, got %v", resp)
	}
}

// With a return of 150%, an investment of 5 pays back 7 (7.5 rounded down).
func TestInvestmentReturnSetting(t *testing.T) {
	state := twoKingdoms()
	state.Settings.InvestmentReturnPercent = 150
	anna := state.GetMerchant("anna")
	anna.StoredGold, anna.InvestedGold = 0, 5

	phases.StartRound(state)
	if anna.StoredGold != 7 || anna.InvestedGold != 0 {
		t.Errorf("anna should be paid 7 for her investment of 5, got purse %d invested %d", anna.StoredGold, anna.InvestedGold)
	}
}

// The settings survive the game moving from phase to phase.
func TestSettingsLastThroughTheRound(t *testing.T) {
	api := jsonapi.NewGameAPIWithDice(engine.NewFixedDice(1))
	send(t, api, map[string]any{"type": "set_settings", "investment_return_percent": 300, "open_game": true})

	for i := 0; i < 5; i++ {
		send(t, api, map[string]any{"type": "advance"})
	}
	if settings := api.GetEngine().GetState().Settings; settings.InvestmentReturnPercent != 300 || !settings.OpenGame {
		t.Errorf("the settings should still be 300%% and open after a full round, got %+v", settings)
	}
}
