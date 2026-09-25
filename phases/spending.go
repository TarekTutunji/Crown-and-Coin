package phases

import (
	"sort"

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
	if merchant := state.GetMerchant(playerID); merchant != nil && !merchant.Arriving {
		// Merchant can invest from their purse. Gold they unhide this round
		// can only be invested next round.
		if merchant.StoredGold > 0 {
			validActions = append(validActions,
				actions.NewMerchantInvestAction(playerID, merchant.ID, merchant.StoredGold),
			)
		}

		// Merchant can hide gold from their purse
		if merchant.StoredGold > 0 {
			validActions = append(validActions,
				actions.NewMerchantHideAction(playerID, merchant.ID, merchant.StoredGold),
			)
		}

		// ...and take hidden gold back out
		if merchant.HiddenGold > 0 {
			validActions = append(validActions,
				actions.NewMerchantUnhideAction(playerID, merchant.ID, merchant.HiddenGold),
			)
		}

		// In a republic the merchants pay for the army themselves, from the
		// purse only: hidden gold cannot go to the army
		country := state.GetCountry(merchant.CountryID)
		if country != nil && country.IsRepublic && country.IsAlive() && merchant.StoredGold > 0 {
			validActions = append(validActions,
				actions.NewContributeArmyAction(playerID, merchant.ID, merchant.StoredGold),
			)
		}
	}

	return validActions
}

// spendingOrder is the order spending actions are carried out in. Merchants
// go first: hiding, then investing and paying into a republic's army, then
// unhiding, so gold unhidden this round lands in the purse too late to be
// invested or paid into the army (it can be next round). The monarch acts
// last, so a gift arrives after the merchants have acted and cannot be
// hidden or invested that round.
func spendingOrder(action actions.Action) int {
	switch action.(type) {
	case *actions.MerchantHideAction:
		return 0
	case *actions.MerchantInvestAction:
		return 1
	case *actions.ContributeArmyAction:
		return 2
	case *actions.MerchantUnhideAction:
		return 3
	default:
		return 4
	}
}

func (p *SpendingPhase) Execute(state *engine.GameState, playerActions []actions.Action) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	var allEvents []events.Event

	ordered := append([]actions.Action(nil), playerActions...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return spendingOrder(ordered[i]) < spendingOrder(ordered[j])
	})

	for _, action := range ordered {
		if err := action.Validate(newState); err != nil {
			continue // Skip invalid actions
		}

		var actionEvents []events.Event
		newState, actionEvents = action.Apply(newState, p.dice)
		allEvents = append(allEvents, actionEvents...)
	}

	return newState, allEvents
}
