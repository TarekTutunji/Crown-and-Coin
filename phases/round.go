package phases

import (
	"sort"

	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// StartRound runs what happens at the start of every round, before
// Taxation: merchants who moved last round arrive in their new country, then
// investments pay out into the merchants' purses, where their monarch can
// tax them. The state is changed in place.
func StartRound(state *engine.GameState) []events.Event {
	var evts []events.Event

	for _, id := range state.ArriveMovers() {
		evts = append(evts, events.NewMerchantArrivedEvent(id, state.GetMerchant(id).CountryID))
	}

	merchantIDs := make([]string, 0, len(state.Merchants))
	for id := range state.Merchants {
		merchantIDs = append(merchantIDs, id)
	}
	sort.Strings(merchantIDs)
	for _, id := range merchantIDs {
		if merchant := state.GetMerchant(id); merchant.InvestedGold > 0 {
			payout := merchant.CollectInvestment(state.Settings.InvestmentReturnPercent)
			evts = append(evts, events.NewInvestmentPayoutEvent(merchant.ID, payout))
		}
	}

	return evts
}
