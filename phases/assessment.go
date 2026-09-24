package phases

import (
	"sort"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// AssessmentPhase handles Phase 5: Internal Assessment
type AssessmentPhase struct {
	BasePhase
	dice engine.DiceRoller
}

func NewAssessmentPhase(dice engine.DiceRoller) *AssessmentPhase {
	return &AssessmentPhase{
		BasePhase: BasePhase{
			name:      "Internal Assessment",
			phaseType: engine.PhaseAssessment,
		},
		dice: dice,
	}
}

func (p *AssessmentPhase) ValidActions(state *engine.GameState, playerID string) []actions.Action {
	var validActions []actions.Action

	// Check if player is a merchant
	for _, merchant := range state.Merchants {
		if merchant.ID == playerID {
			country := state.GetCountry(merchant.CountryID)
			if country == nil || !country.IsAlive() {
				continue
			}

			// Can always remain
			validActions = append(validActions,
				actions.NewRemainAction(playerID, merchant.ID),
			)

			// Can flee to any other alive country
			for _, target := range state.GetAliveCountries() {
				if target.ID != merchant.CountryID {
					validActions = append(validActions,
						actions.NewFleeAction(playerID, merchant.ID, target.ID),
					)
				}
			}

			// Can revolt if country is a monarchy
			if !country.IsRepublic {
				validActions = append(validActions,
					actions.NewRevoltAction(playerID, merchant.ID, country.ID),
				)
			}
		}
	}

	return validActions
}

func (p *AssessmentPhase) Execute(state *engine.GameState, playerActions []actions.Action) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	var allEvents []events.Event

	// First, collect all revolt intentions by country
	revoltsByCountry := make(map[string][]string) // countryID -> list of merchant IDs
	loyalistsByCountry := make(map[string][]string)

	for _, action := range playerActions {
		switch a := action.(type) {
		case *actions.RevoltAction:
			if err := a.Validate(newState); err == nil {
				revoltsByCountry[a.CountryID] = append(
					revoltsByCountry[a.CountryID],
					a.MerchantID,
				)
			}
		case *actions.RemainAction:
			if err := a.Validate(newState); err == nil {
				merchant := newState.GetMerchant(a.MerchantID)
				loyalistsByCountry[merchant.CountryID] = append(
					loyalistsByCountry[merchant.CountryID],
					a.MerchantID,
				)
			}
		}
	}

	// Resolve revolts first (before merchants can flee), in a fixed country
	// order so the dice rolls behind them stay reproducible
	revoltingCountries := make([]string, 0, len(revoltsByCountry))
	for countryID := range revoltsByCountry {
		revoltingCountries = append(revoltingCountries, countryID)
	}
	sort.Strings(revoltingCountries)

	for _, countryID := range revoltingCountries {
		var revoltEvents []events.Event
		newState, revoltEvents = actions.ResolveRevolt(newState, countryID, revoltsByCountry[countryID], loyalistsByCountry[countryID], p.dice)
		allEvents = append(allEvents, revoltEvents...)
	}

	// Then process flee actions (only for merchants who didn't revolt)
	revolters := make(map[string]bool)
	for _, participants := range revoltsByCountry {
		for _, mID := range participants {
			revolters[mID] = true
		}
	}

	for _, action := range playerActions {
		if action.Type() == actions.ActionFlee {
			fleeAction := action.(*actions.FleeAction)
			// Skip if this merchant participated in a revolt
			if revolters[fleeAction.MerchantID] {
				continue
			}
			if err := fleeAction.Validate(newState); err == nil {
				var actionEvents []events.Event
				newState, actionEvents = fleeAction.Apply(newState, p.dice)
				allEvents = append(allEvents, actionEvents...)
			}
		}
	}

	return newState, allEvents
}
