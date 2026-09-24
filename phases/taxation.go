package phases

import (
	"sort"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// TaxationPhase handles Phase 1: Taxation
type TaxationPhase struct {
	BasePhase
	dice engine.DiceRoller
}

func NewTaxationPhase(dice engine.DiceRoller) *TaxationPhase {
	return &TaxationPhase{
		BasePhase: BasePhase{
			name:      "Taxation",
			phaseType: engine.PhaseTaxation,
		},
		dice: dice,
	}
}

func (p *TaxationPhase) ValidActions(state *engine.GameState, playerID string) []actions.Action {
	var validActions []actions.Action

	// Check if player is a monarch
	for _, country := range state.Countries {
		if country.MonarchID == playerID && !country.IsRepublic && country.IsAlive() {
			// Monarch can choose low or high peasant tax
			validActions = append(validActions,
				actions.NewTaxPeasantsAction(playerID, country.ID, false), // Low tax
				actions.NewTaxPeasantsAction(playerID, country.ID, true),  // High tax
			)

			// Monarch can tax each merchant
			for _, merchant := range state.GetMerchantsByCountry(country.ID) {
				// Offer to tax any amount up to merchant's stored gold
				validActions = append(validActions,
					actions.NewTaxMerchantsAction(playerID, country.ID, merchant.ID, merchant.StoredGold),
				)
			}
		}
	}

	// Merchants of a republic vote on the peasant tax
	if merchant := state.GetMerchant(playerID); merchant != nil {
		country := state.GetCountry(merchant.CountryID)
		if country != nil && country.IsRepublic && country.IsAlive() {
			validActions = append(validActions,
				actions.NewVoteTaxAction(playerID, merchant.ID, false),
				actions.NewVoteTaxAction(playerID, merchant.ID, true),
			)
		}
	}

	return validActions
}

func (p *TaxationPhase) Execute(state *engine.GameState, playerActions []actions.Action) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	var allEvents []events.Event

	// Process monarch tax actions
	// Group actions by country to handle peasant revolt at end of phase
	revoltChecks := make(map[string]bool) // countryID -> had high tax

	// Republic tax votes, counting only the first vote of each merchant
	highVotes := make(map[string]int)
	lowVotes := make(map[string]int)
	voted := make(map[string]bool)

	// Monarchies whose monarch chose a peasant tax this round
	taxed := make(map[string]bool)

	for _, action := range playerActions {
		if err := action.Validate(newState); err != nil {
			continue // Skip invalid actions
		}

		if tax, ok := action.(*actions.TaxPeasantsAction); ok {
			if taxed[tax.CountryID] {
				continue // Peasants are only taxed once per round
			}
			taxed[tax.CountryID] = true
		}

		if vote, ok := action.(*actions.VoteTaxAction); ok {
			if !voted[vote.MerchantID] {
				voted[vote.MerchantID] = true
				countryID := newState.GetMerchant(vote.MerchantID).CountryID
				if vote.HighTax {
					highVotes[countryID]++
				} else {
					lowVotes[countryID]++
				}
			}
			continue
		}

		var actionEvents []events.Event
		newState, actionEvents = action.Apply(newState, p.dice)
		allEvents = append(allEvents, actionEvents...)

		// Track if any peasant revolt events occurred
		for _, evt := range actionEvents {
			if evt.Type() == events.EventPeasantRevolt {
				revoltChecks[evt.Data()["country_id"].(string)] = true
			}
		}
	}

	// A monarch who did not choose a peasant tax taxes low. Low tax never
	// rolls the dice, so the order does not matter here.
	for _, country := range newState.Countries {
		if !country.IsRepublic && country.IsAlive() && country.MonarchID != "" && !taxed[country.ID] {
			gold, taxEvents := actions.CollectPeasantTax(country, false, p.dice)
			country.AddGold(gold)
			allEvents = append(allEvents, taxEvents...)
		}
	}

	// Every living republic taxes its peasants, even if nobody voted (a tie
	// means low tax). Countries go in a fixed order so the dice stay reproducible.
	republicIDs := make([]string, 0)
	for id, country := range newState.Countries {
		if country.IsRepublic && country.IsAlive() {
			republicIDs = append(republicIDs, id)
		}
	}
	sort.Strings(republicIDs)

	for _, countryID := range republicIDs {
		var taxEvents []events.Event
		newState, taxEvents = actions.ResolveRepublicTax(newState, countryID, highVotes[countryID], lowVotes[countryID], p.dice)
		allEvents = append(allEvents, taxEvents...)
	}

	// Countries destroyed by a peasant revolt fall apart once all taxes are in,
	// so nobody scatters into a country that is about to collapse too
	var collapsedIDs []string
	for id, country := range state.Countries {
		if country.IsAlive() && !newState.GetCountry(id).IsAlive() {
			collapsedIDs = append(collapsedIDs, id)
		}
	}
	sort.Strings(collapsedIDs)

	for _, countryID := range collapsedIDs {
		allEvents = append(allEvents, actions.CollapseCountry(newState, countryID, events.DeposedByPeasants, p.dice)...)
	}

	return newState, allEvents
}
