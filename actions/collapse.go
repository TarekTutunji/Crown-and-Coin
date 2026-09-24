package actions

import (
	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// ShareGoldAmongMerchants splits gold earned by a republic evenly among all of
// its merchants. A republic without merchants keeps the gold in its treasury.
// The state is changed in place.
func ShareGoldAmongMerchants(state *engine.GameState, countryID string, gold int, source string) []events.Event {
	if gold <= 0 {
		return nil
	}
	merchantIDs := make([]string, 0)
	for _, m := range state.GetMerchantsByCountry(countryID) {
		merchantIDs = append(merchantIDs, m.ID)
	}
	if len(merchantIDs) == 0 {
		state.GetCountry(countryID).AddGold(gold)
		return nil
	}

	distributeGold(state, merchantIDs, gold)
	return []events.Event{events.NewRepublicGoldSharedEvent(countryID, merchantIDs, gold, source)}
}

// DeposeMonarchWithTreasury removes the monarch of a fallen country, who
// escapes with the whole treasury as personal savings and starts over as a
// merchant in one of the destinations, picked at random. With no destination
// left they drop out of the game. The state is changed in place.
func DeposeMonarchWithTreasury(state *engine.GameState, countryID string, destinations []string, reason string, roller engine.DiceRoller) []events.Event {
	country := state.GetCountry(countryID)
	if country == nil || country.IsRepublic || country.MonarchID == "" {
		return nil
	}

	monarchID := country.MonarchID
	savings := country.EmptyTreasury()
	country.RemoveMonarch()

	destination := engine.PickRandomID(destinations, roller)
	if destination != "" {
		state.ResettleMonarchAsMerchant(monarchID, destination, savings)
	}
	return []events.Event{events.NewMonarchDeposedEvent(monarchID, countryID, destination, savings, reason)}
}

// CollapseCountry handles a country destroyed from within, by a peasant or
// merchant revolt (reason says which, as for MonarchDeposedEvent). Nobody
// conquered it, so everyone scatters to the surviving countries: the monarch
// (if any) escapes with the treasury, and the merchants keep only their hidden
// gold. The state is changed in place.
func CollapseCountry(state *engine.GameState, countryID, reason string, roller engine.DiceRoller) []events.Event {
	survivors := state.GetAliveCountryIDsExcept(countryID)

	evts := DeposeMonarchWithTreasury(state, countryID, survivors, reason, roller)

	merchantIDs, forfeited := state.ScatterMerchants(countryID, survivors, true)
	return append(evts, events.NewCountryCollapsedEvent(countryID, merchantIDs, forfeited, reason))
}
