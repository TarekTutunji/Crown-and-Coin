package actions

import (
	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// ShareGoldAmongMerchants splits gold earned by a republic evenly among all of
// its merchants, with the dice deciding who gets any leftovers. A republic
// without merchants keeps the gold in its treasury. The state is changed in
// place.
func ShareGoldAmongMerchants(state *engine.GameState, countryID string, gold int, source string, roller engine.DiceRoller) []events.Event {
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

	distributeGold(state, merchantIDs, gold, roller)
	return []events.Event{events.NewRepublicGoldSharedEvent(countryID, merchantIDs, gold, source)}
}

// GiveGoldToCountry hands gold to a country: a monarchy puts it in its
// treasury, a republic shares it among its merchants. The state is changed
// in place.
func GiveGoldToCountry(state *engine.GameState, countryID string, gold int, source string, roller engine.DiceRoller) []events.Event {
	country := state.GetCountry(countryID)
	if country == nil || gold <= 0 {
		return nil
	}
	if country.IsRepublic {
		return ShareGoldAmongMerchants(state, countryID, gold, source, roller)
	}
	country.AddGold(gold)
	return nil
}

// DeposeMonarch removes the monarch of a country. They start over as a
// merchant with engine.FallenMonarchPurse gold from the bank, in one of the
// destinations picked at random, and keep nothing of the treasury. With no
// destination left they drop out of the game. The state is changed in place.
func DeposeMonarch(state *engine.GameState, countryID string, destinations []string, reason string, roller engine.DiceRoller) []events.Event {
	country := state.GetCountry(countryID)
	if country == nil || country.IsRepublic || country.MonarchID == "" {
		return nil
	}

	monarchID := country.MonarchID
	country.RemoveMonarch()

	destination := engine.PickRandomID(destinations, roller)
	if destination != "" {
		state.ResettleMonarchAsMerchant(monarchID, destination)
	}
	return []events.Event{events.NewMonarchDeposedEvent(monarchID, countryID, destination, engine.FallenMonarchPurse, reason)}
}

// CollapseCountry handles a country destroyed from within, by a peasant or
// merchant revolt (reason says which, as for MonarchDeposedEvent). Nobody
// conquered it, so it falls apart: the monarch (if any) starts over as a
// merchant in a random surviving country, the treasury and the peasants are
// destroyed, and the merchants are spread over the surviving countries,
// keeping their purse and hidden gold but losing their investments. The state
// is changed in place.
func CollapseCountry(state *engine.GameState, countryID, reason string, roller engine.DiceRoller) []events.Event {
	country := state.GetCountry(countryID)
	survivors := state.GetAliveCountryIDsExcept(countryID)

	evts := DeposeMonarch(state, countryID, survivors, reason, roller)
	treasury := country.EmptyTreasury()
	country.Peasants = 0

	// With several survivors the dice decide who comes first, and so who gets
	// any merchant left over after an even split
	if len(survivors) > 1 {
		survivors = engine.ShuffleIDs(survivors, roller)
	}
	merchantIDs, forfeited := state.ScatterMerchants(countryID, survivors, true, roller)
	return append(evts, events.NewCountryCollapsedEvent(countryID, merchantIDs, forfeited, treasury, reason))
}
