package actions

import (
	"errors"
	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// BuildArmyAction - Monarch spends gold to build army
type BuildArmyAction struct {
	BaseAction
	CountryID string
	Amount    int // Gold to spend (1 gold = 1 army strength)
}

func NewBuildArmyAction(playerID, countryID string, amount int) *BuildArmyAction {
	return &BuildArmyAction{
		BaseAction: BaseAction{actionType: ActionBuildArmy, playerID: playerID},
		CountryID:  countryID,
		Amount:     amount,
	}
}

func (a *BuildArmyAction) Validate(state *engine.GameState) error {
	country := state.GetCountry(a.CountryID)
	if country == nil {
		return errors.New("country not found")
	}
	if country.IsRepublic {
		return errors.New("republics vote on army building")
	}
	if country.MonarchID != a.playerID {
		return errors.New("only the monarch can build army")
	}
	if a.Amount < 0 {
		return errors.New("amount cannot be negative")
	}
	if country.Gold < a.Amount {
		return errors.New("insufficient gold")
	}
	return nil
}

func (a *BuildArmyAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	country := newState.GetCountry(a.CountryID)
	var evts []events.Event

	country.SpendGold(a.Amount)
	country.AddArmy(a.Amount)

	evts = append(evts, events.NewArmyBuiltEvent(a.CountryID, a.Amount, country.ArmyStrength))

	return newState, evts
}

// MonarchInvestAction - Monarch gives gold to a merchant
type MonarchInvestAction struct {
	BaseAction
	CountryID  string
	MerchantID string
	Amount     int
}

func NewMonarchInvestAction(playerID, countryID, merchantID string, amount int) *MonarchInvestAction {
	return &MonarchInvestAction{
		BaseAction: BaseAction{actionType: ActionMonarchInvest, playerID: playerID},
		CountryID:  countryID,
		MerchantID: merchantID,
		Amount:     amount,
	}
}

func (a *MonarchInvestAction) Validate(state *engine.GameState) error {
	country := state.GetCountry(a.CountryID)
	if country == nil {
		return errors.New("country not found")
	}
	if country.IsRepublic {
		return errors.New("republics handle investments differently")
	}
	if country.MonarchID != a.playerID {
		return errors.New("only the monarch can invest")
	}
	merchant := state.GetMerchant(a.MerchantID)
	if merchant == nil {
		return errors.New("merchant not found")
	}
	if merchant.CountryID != a.CountryID {
		return errors.New("merchant does not belong to this country")
	}
	if a.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if country.Gold < a.Amount {
		return errors.New("insufficient gold")
	}
	return nil
}

func (a *MonarchInvestAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	country := newState.GetCountry(a.CountryID)
	merchant := newState.GetMerchant(a.MerchantID)
	var evts []events.Event

	country.SpendGold(a.Amount)
	merchant.StoredGold += a.Amount

	evt := events.NewBaseEvent(events.EventInvestmentMade)
	evt.Set("from", "monarch")
	evt.Set("country_id", a.CountryID)
	evt.Set("merchant_id", a.MerchantID)
	evt.Set("amount", a.Amount)
	evts = append(evts, evt)

	return newState, evts
}

// MerchantInvestAction - Merchant invests gold (doubles next turn)
type MerchantInvestAction struct {
	BaseAction
	MerchantID string
	Amount     int
}

func NewMerchantInvestAction(playerID, merchantID string, amount int) *MerchantInvestAction {
	return &MerchantInvestAction{
		BaseAction: BaseAction{actionType: ActionMerchantInvest, playerID: playerID},
		MerchantID: merchantID,
		Amount:     amount,
	}
}

func (a *MerchantInvestAction) Validate(state *engine.GameState) error {
	merchant := state.GetMerchant(a.MerchantID)
	if merchant == nil {
		return errors.New("merchant not found")
	}
	if merchant.ID != a.playerID {
		return errors.New("can only invest your own gold")
	}
	if a.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if merchant.SpendableGold() < a.Amount {
		return errors.New("insufficient gold")
	}
	return nil
}

func (a *MerchantInvestAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	merchant := newState.GetMerchant(a.MerchantID)
	var evts []events.Event

	merchant.Invest(a.Amount)

	evt := events.NewBaseEvent(events.EventInvestmentMade)
	evt.Set("from", "merchant")
	evt.Set("merchant_id", a.MerchantID)
	evt.Set("amount", a.Amount)
	evts = append(evts, evt)

	return newState, evts
}

// MerchantHideAction - Merchant moves gold from their purse into hidden
// savings, where the monarch can neither see nor tax it
type MerchantHideAction struct {
	BaseAction
	MerchantID string
	Amount     int
}

func NewMerchantHideAction(playerID, merchantID string, amount int) *MerchantHideAction {
	return &MerchantHideAction{
		BaseAction: BaseAction{actionType: ActionMerchantHide, playerID: playerID},
		MerchantID: merchantID,
		Amount:     amount,
	}
}

func (a *MerchantHideAction) Validate(state *engine.GameState) error {
	merchant := state.GetMerchant(a.MerchantID)
	if merchant == nil {
		return errors.New("merchant not found")
	}
	if merchant.ID != a.playerID {
		return errors.New("can only hide your own gold")
	}
	if a.Amount < 0 {
		return errors.New("amount cannot be negative")
	}
	if merchant.StoredGold < a.Amount {
		return errors.New("not enough gold in the purse")
	}
	return nil
}

func (a *MerchantHideAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	newState.GetMerchant(a.MerchantID).Hide(a.Amount)
	return newState, nil
}
