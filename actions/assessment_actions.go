package actions

import (
	"errors"
	"sort"

	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// RemainAction - Merchant stays with current country
type RemainAction struct {
	BaseAction
	MerchantID string
}

func NewRemainAction(playerID, merchantID string) *RemainAction {
	return &RemainAction{
		BaseAction: BaseAction{actionType: ActionRemain, playerID: playerID},
		MerchantID: merchantID,
	}
}

func (a *RemainAction) Validate(state *engine.GameState) error {
	merchant := state.GetMerchant(a.MerchantID)
	if merchant == nil {
		return errors.New("merchant not found")
	}
	if merchant.ID != a.playerID {
		return errors.New("can only control your own merchant")
	}
	if merchant.Arriving {
		return errArriving
	}
	country := state.GetCountry(merchant.CountryID)
	if country == nil || !country.IsAlive() {
		return errors.New("country is not alive")
	}
	return nil
}

func (a *RemainAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	// Remaining is a no-op
	return state.Clone(), nil
}

// FleeAction - Merchant flees to a different country
type FleeAction struct {
	BaseAction
	MerchantID   string
	ToCountryID  string
}

func NewFleeAction(playerID, merchantID, toCountryID string) *FleeAction {
	return &FleeAction{
		BaseAction:  BaseAction{actionType: ActionFlee, playerID: playerID},
		MerchantID:  merchantID,
		ToCountryID: toCountryID,
	}
}

func (a *FleeAction) Validate(state *engine.GameState) error {
	merchant := state.GetMerchant(a.MerchantID)
	if merchant == nil {
		return errors.New("merchant not found")
	}
	if merchant.ID != a.playerID {
		return errors.New("can only control your own merchant")
	}
	if merchant.Arriving {
		return errArriving
	}
	if merchant.CountryID == a.ToCountryID {
		return errors.New("already in that country")
	}
	toCountry := state.GetCountry(a.ToCountryID)
	if toCountry == nil {
		return errors.New("destination country not found")
	}
	if !toCountry.IsAlive() {
		return errors.New("cannot flee to a defeated country")
	}
	return nil
}

func (a *FleeAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	merchant := newState.GetMerchant(a.MerchantID)
	var evts []events.Event

	fromCountry := merchant.CountryID
	goldTaken := merchant.SpendableGold()
	goldLost := merchant.InvestedGold

	merchant.FleeToCountry(a.ToCountryID)

	evts = append(evts, events.NewMerchantFledEvent(
		a.MerchantID, fromCountry, a.ToCountryID, goldTaken, goldLost,
	))

	return newState, evts
}

// RevoltAction - Merchant participates in revolt against monarch
type RevoltAction struct {
	BaseAction
	MerchantID string
	CountryID  string
}

func NewRevoltAction(playerID, merchantID, countryID string) *RevoltAction {
	return &RevoltAction{
		BaseAction: BaseAction{actionType: ActionRevolt, playerID: playerID},
		MerchantID: merchantID,
		CountryID:  countryID,
	}
}

func (a *RevoltAction) Validate(state *engine.GameState) error {
	merchant := state.GetMerchant(a.MerchantID)
	if merchant == nil {
		return errors.New("merchant not found")
	}
	if merchant.ID != a.playerID {
		return errors.New("can only control your own merchant")
	}
	if merchant.Arriving {
		return errArriving
	}
	if merchant.CountryID != a.CountryID {
		return errors.New("merchant not in this country")
	}
	country := state.GetCountry(a.CountryID)
	if country == nil {
		return errors.New("country not found")
	}
	if country.IsRepublic {
		return errors.New("cannot revolt against a republic")
	}
	if !country.IsAlive() {
		return errors.New("country is not alive")
	}
	return nil
}

func (a *RevoltAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	// Individual revolt action just marks intention
	// The actual revolt resolution happens in the phase
	return state.Clone(), nil
}

// ResolveRevolt handles the actual revolt mechanics
// This is called by the phase after collecting all revolt actions.
// loyalistIDs are the merchants of this country who chose to remain; their gold
// backs the monarch. Only purse and hidden gold count, never investments. A
// tie goes to the monarch.
func ResolveRevolt(state *engine.GameState, countryID string, participantIDs, loyalistIDs []string, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	country := newState.GetCountry(countryID)
	var evts []events.Event

	// Calculate total merchant gold
	var merchantGold int
	for _, mID := range participantIDs {
		merchant := newState.GetMerchant(mID)
		if merchant != nil {
			merchantGold += merchant.SpendableGold()
		}
	}

	var loyalistGold int
	for _, mID := range loyalistIDs {
		merchant := newState.GetMerchant(mID)
		if merchant != nil {
			loyalistGold += merchant.SpendableGold()
		}
	}

	defenseGold := country.Gold + loyalistGold

	if merchantGold > defenseGold {
		// Revolt succeeds
		evts = append(evts, events.NewRevoltSuccessEvent(countryID, participantIDs, merchantGold, defenseGold))

		// Country loses 2 HP
		country.TakeDamage(2)

		// The monarch is exiled: they start over as a merchant in a randomly
		// chosen other country, never the one that just threw them out, with
		// gold from the bank. With nowhere left to go they drop out of the game.
		evts = append(evts, DeposeMonarch(newState, countryID, newState.GetAliveCountryIDsExcept(countryID), events.DeposedByRevolution, roller)...)

		// The rebels share out the whole treasury
		if treasury := country.EmptyTreasury(); treasury > 0 && len(participantIDs) > 0 {
			distributeGold(newState, participantIDs, treasury, roller)
			evts = append(evts, events.NewTreasurySplitEvent(countryID, participantIDs, treasury))
		}

		// If the 2 HP killed it, there is no republic to found: the country
		// falls apart just as if its peasants had destroyed it
		if !country.IsAlive() {
			evts = append(evts, CollapseCountry(newState, countryID, events.DeposedByRevolution, roller)...)
			return newState, evts
		}

		// Becomes a republic
		country.BecomeRepublic()

		evt := events.NewBaseEvent(events.EventRepublicFormed)
		evt.Set("country_id", countryID)
		evts = append(evts, evt)
	} else {
		// Revolt fails - the rebels lose their purse and hidden gold to the
		// treasury, and their investments are destroyed
		totalLost, totalDestroyed := 0, 0
		for _, mID := range participantIDs {
			merchant := newState.GetMerchant(mID)
			if merchant != nil {
				lost, destroyed := merchant.ForfeitRebellion()
				totalLost += lost
				totalDestroyed += destroyed
			}
		}
		country.AddGold(totalLost)

		evts = append(evts, events.NewRevoltFailedEvent(countryID, participantIDs, totalLost, totalDestroyed, merchantGold, defenseGold))
	}

	return newState, evts
}

// distributeGold splits gold evenly between the given merchants. When it does
// not divide evenly, the dice decide who gets the leftover coins, one each.
func distributeGold(state *engine.GameState, merchantIDs []string, gold int, roller engine.DiceRoller) {
	if len(merchantIDs) == 0 {
		return
	}

	recipients := append([]string(nil), merchantIDs...)
	sort.Strings(recipients)
	if gold%len(recipients) != 0 {
		recipients = engine.ShuffleIDs(recipients, roller)
	}

	for i, amount := range engine.SplitEvenly(gold, len(recipients)) {
		if merchant := state.GetMerchant(recipients[i]); merchant != nil {
			merchant.StoredGold += amount
		}
	}
}
