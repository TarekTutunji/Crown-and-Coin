package jsonapi

import (
	"fmt"
	"strconv"
	"strings"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// SerializeState converts a GameState to StateJSON
func SerializeState(state *engine.GameState) *StateJSON {
	countries := make(map[string]*CountryJSON)
	for id, c := range state.Countries {
		countries[id] = SerializeCountry(c)
	}

	merchants := make(map[string]*MerchantJSON)
	for id, m := range state.Merchants {
		merchants[id] = SerializeMerchant(m)
	}

	return &StateJSON{
		Turn:      state.Turn,
		Phase:     state.Phase.String(),
		Countries: countries,
		Merchants: merchants,
	}
}

// SerializeCountry converts a Country to CountryJSON
func SerializeCountry(c *engine.Country) *CountryJSON {
	return &CountryJSON{
		CountryID:    c.ID,
		HP:           c.HP,
		ArmyStrength: c.ArmyStrength,
		Gold:         c.Gold,
		Peasants:     c.Peasants,
		RevoltRisk:   c.RevoltRisk,
		IsRepublic:   c.IsRepublic,
		MonarchID:    c.MonarchID,
		DiedOnce:     c.DiedOnce,
	}
}

// SerializeMerchant converts a Merchant to MerchantJSON
func SerializeMerchant(m *engine.Merchant) *MerchantJSON {
	return &MerchantJSON{
		PlayerID:     m.ID,
		CountryID:    m.CountryID,
		StoredGold:   m.StoredGold,
		InvestedGold: m.InvestedGold,
	}
}

// SerializeAction converts an action to ActionJSON
// If usePlaceholders is true, amounts will be converted to placeholders like "<AMOUNT:0-10>"
// If false, the actual amount values from the action will be used
func SerializeAction(action actions.Action, state *engine.GameState, usePlaceholders bool) ActionJSON {
	aj := ActionJSON{
		Type:     string(action.Type()),
		PlayerID: action.PlayerID(),
	}

	switch a := action.(type) {
	case *actions.TaxPeasantsAction:
		aj.CountryID = a.CountryID
		// Note: high_tax is not included - it's redundant since it can be inferred from type
		// (tax_peasants_low vs tax_peasants_high)

	case *actions.TaxMerchantsAction:
		aj.CountryID = a.CountryID
		aj.MerchantID = a.MerchantID
		if usePlaceholders {
			aj.Amount = fmt.Sprintf("<AMOUNT:0-%d>", a.Amount)
		} else {
			aj.Amount = a.Amount
		}

	case *actions.BuildArmyAction:
		aj.CountryID = a.CountryID
		if usePlaceholders {
			aj.Amount = fmt.Sprintf("<AMOUNT:0-%d>", a.Amount)
		} else {
			aj.Amount = a.Amount
		}

	case *actions.MonarchInvestAction:
		aj.CountryID = a.CountryID
		aj.MerchantID = a.MerchantID
		if usePlaceholders {
			aj.Amount = fmt.Sprintf("<AMOUNT:0-%d>", a.Amount)
		} else {
			aj.Amount = a.Amount
		}

	case *actions.MerchantInvestAction:
		aj.MerchantID = a.MerchantID
		if usePlaceholders {
			aj.Amount = fmt.Sprintf("<AMOUNT:0-%d>", a.Amount)
		} else {
			aj.Amount = a.Amount
		}

	case *actions.MerchantHideAction:
		aj.MerchantID = a.MerchantID

	case *actions.AttackAction:
		aj.CountryID = a.AttackerID
		aj.TargetID = a.DefenderID

	case *actions.NoAttackAction:
		aj.CountryID = a.CountryID

	case *actions.RemainAction:
		aj.MerchantID = a.MerchantID

	case *actions.FleeAction:
		aj.MerchantID = a.MerchantID
		aj.TargetID = a.ToCountryID

	case *actions.RevoltAction:
		aj.MerchantID = a.MerchantID
		aj.CountryID = a.CountryID

	case *actions.VoteTaxAction:
		aj.MerchantID = a.MerchantID

	case *actions.ContributeArmyAction:
		aj.MerchantID = a.MerchantID
		if usePlaceholders {
			aj.Amount = fmt.Sprintf("<AMOUNT:0-%d>", a.Amount)
		} else {
			aj.Amount = a.Amount
		}

	case *actions.VoteAttackAction:
		aj.MerchantID = a.MerchantID
		aj.TargetID = a.TargetID

	case *actions.VoteNoAttackAction:
		aj.MerchantID = a.MerchantID
	}

	return aj
}

// DeserializeAction converts ActionJSON to an actions.Action
func DeserializeAction(aj ActionJSON) (actions.Action, error) {
	switch actions.ActionType(aj.Type) {
	case actions.ActionTaxPeasantsLow:
		return actions.NewTaxPeasantsAction(aj.PlayerID, aj.CountryID, false), nil

	case actions.ActionTaxPeasantsHigh:
		return actions.NewTaxPeasantsAction(aj.PlayerID, aj.CountryID, true), nil

	case actions.ActionTaxMerchants:
		amount, err := parseAmount(aj.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount for tax_merchants: %w", err)
		}
		return actions.NewTaxMerchantsAction(aj.PlayerID, aj.CountryID, aj.MerchantID, amount), nil

	case actions.ActionBuildArmy:
		amount, err := parseAmount(aj.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount for build_army: %w", err)
		}
		return actions.NewBuildArmyAction(aj.PlayerID, aj.CountryID, amount), nil

	case actions.ActionMonarchInvest:
		amount, err := parseAmount(aj.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount for monarch_invest: %w", err)
		}
		return actions.NewMonarchInvestAction(aj.PlayerID, aj.CountryID, aj.MerchantID, amount), nil

	case actions.ActionMerchantInvest:
		amount, err := parseAmount(aj.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount for merchant_invest: %w", err)
		}
		return actions.NewMerchantInvestAction(aj.PlayerID, aj.MerchantID, amount), nil

	case actions.ActionMerchantHide:
		amount, _ := parseAmount(aj.Amount) // Amount is optional for hide
		return actions.NewMerchantHideAction(aj.PlayerID, aj.MerchantID, amount), nil

	case actions.ActionAttack:
		return actions.NewAttackAction(aj.PlayerID, aj.CountryID, aj.TargetID), nil

	case actions.ActionNoAttack:
		return actions.NewNoAttackAction(aj.PlayerID, aj.CountryID), nil

	case actions.ActionRemain:
		return actions.NewRemainAction(aj.PlayerID, aj.MerchantID), nil

	case actions.ActionFlee:
		return actions.NewFleeAction(aj.PlayerID, aj.MerchantID, aj.TargetID), nil

	case actions.ActionRevolt:
		return actions.NewRevoltAction(aj.PlayerID, aj.MerchantID, aj.CountryID), nil

	case actions.ActionVoteTaxLow:
		return actions.NewVoteTaxAction(aj.PlayerID, aj.MerchantID, false), nil

	case actions.ActionVoteTaxHigh:
		return actions.NewVoteTaxAction(aj.PlayerID, aj.MerchantID, true), nil

	case actions.ActionContributeArmy:
		amount, err := parseAmount(aj.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount for contribute_army: %w", err)
		}
		return actions.NewContributeArmyAction(aj.PlayerID, aj.MerchantID, amount), nil

	case actions.ActionVoteAttack:
		return actions.NewVoteAttackAction(aj.PlayerID, aj.MerchantID, aj.TargetID), nil

	case actions.ActionVoteNoAttack:
		return actions.NewVoteNoAttackAction(aj.PlayerID, aj.MerchantID), nil

	default:
		return nil, fmt.Errorf("unknown action type: %s", aj.Type)
	}
}

// parseAmount extracts an integer from the Amount field (which may be int, float64, or string)
func parseAmount(amount any) (int, error) {
	if amount == nil {
		return 0, nil
	}

	switch v := amount.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		// Check if it's a placeholder like "<AMOUNT:0-10>"
		if strings.HasPrefix(v, "<") {
			return 0, fmt.Errorf("placeholder not replaced: %s", v)
		}
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("invalid amount type: %T", amount)
	}
}

// SerializeEvent converts an event to EventJSON
func SerializeEvent(event events.Event) EventJSON {
	return EventJSON{
		Type:    string(event.Type()),
		Message: event.String(),
		Data:    event.Data(),
	}
}

// SerializeEvents converts a slice of events to EventJSON slice
func SerializeEvents(evts []events.Event) []EventJSON {
	result := make([]EventJSON, len(evts))
	for i, e := range evts {
		result[i] = SerializeEvent(e)
	}
	return result
}
