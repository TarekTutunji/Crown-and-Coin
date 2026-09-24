package phases

import (
	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// SpendingPhase handles Phase 3: Spending & Investment
type SpendingPhase struct {
	BasePhase
	dice engine.DiceRoller
}

func NewSpendingPhase(dice engine.DiceRoller) *SpendingPhase {
	return &SpendingPhase{
		BasePhase: BasePhase{
			name:      "Spending & Investment",
			phaseType: engine.PhaseSpending,
		},
		dice: dice,
	}
}

func (p *SpendingPhase) ValidActions(state *engine.GameState, playerID string) []actions.Action {
	var validActions []actions.Action

	// Check if player is a monarch
	for _, country := range state.Countries {
		if country.MonarchID == playerID && !country.IsRepublic && country.IsAlive() {
			// Monarch can build army (any amount up to their gold)
			if country.Gold > 0 {
				validActions = append(validActions,
					actions.NewBuildArmyAction(playerID, country.ID, country.Gold),
				)
			}

			// Monarch can invest in merchants
			for _, merchant := range state.GetMerchantsByCountry(country.ID) {
				if country.Gold > 0 {
					validActions = append(validActions,
						actions.NewMonarchInvestAction(playerID, country.ID, merchant.ID, country.Gold),
					)
				}
			}
		}
	}

	// Check if player is a merchant
	for _, merchant := range state.Merchants {
		if merchant.ID == playerID {
			// Merchant can invest
			if merchant.StoredGold > 0 {
				validActions = append(validActions,
					actions.NewMerchantInvestAction(playerID, merchant.ID, merchant.StoredGold),
				)
			}

			// Merchant can hide (keep in savings)
			validActions = append(validActions,
				actions.NewMerchantHideAction(playerID, merchant.ID, merchant.StoredGold),
			)

			// In a republic the merchants pay for the army themselves
			country := state.GetCountry(merchant.CountryID)
			if country != nil && country.IsRepublic && country.IsAlive() && merchant.StoredGold > 0 {
				validActions = append(validActions,
					actions.NewContributeArmyAction(playerID, merchant.ID, merchant.StoredGold),
				)
			}
		}
	}

	return validActions
}

func (p *SpendingPhase) Execute(state *engine.GameState, playerActions []actions.Action) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	var allEvents []events.Event

	// Process all spending actions
	for _, action := range playerActions {
		if err := action.Validate(newState); err != nil {
			continue // Skip invalid actions
		}

		var actionEvents []events.Event
		newState, actionEvents = action.Apply(newState, p.dice)
		allEvents = append(allEvents, actionEvents...)
	}

	return newState, allEvents
}
