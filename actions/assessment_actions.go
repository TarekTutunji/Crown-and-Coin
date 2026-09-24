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
	goldTaken := merchant.StoredGold
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

// DeposedMonarchSeverance is the flat amount of gold a monarch keeps when
// overthrown by a merchant revolt, regardless of how large the treasury was.
const DeposedMonarchSeverance = 5

// ResolveRevolt handles the actual revolt mechanics
// This is called by the phase after collecting all revolt actions.
// loyalistIDs are the merchants of this country who chose to remain; their gold
// backs the monarch. A tie goes to the monarch.
func ResolveRevolt(state *engine.GameState, countryID string, participantIDs, loyalistIDs []string, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	country := newState.GetCountry(countryID)
	var evts []events.Event

	// Calculate total merchant gold
	var merchantGold int
	for _, mID := range participantIDs {
		merchant := newState.GetMerchant(mID)
		if merchant != nil {
			merchantGold += merchant.TotalGold()
		}
	}

	var loyalistGold int
	for _, mID := range loyalistIDs {
		merchant := newState.GetMerchant(mID)
		if merchant != nil {
			loyalistGold += merchant.TotalGold()
		}
	}

	defenseGold := country.Gold + loyalistGold

	if merchantGold > defenseGold {
		// Revolt succeeds
		evts = append(evts, events.NewRevoltSuccessEvent(countryID, participantIDs, merchantGold, defenseGold))

		monarchID := country.MonarchID
		treasury := country.EmptyTreasury()

		// Country loses 2 HP
		country.TakeDamage(2)

		// Becomes a republic
		country.BecomeRepublic()

		evt := events.NewBaseEvent(events.EventRepublicFormed)
		evt.Set("country_id", countryID)
		evts = append(evts, evt)

		// The deposed monarch keeps a flat severance and restarts as a merchant
		// in a randomly chosen surviving kingdom. They are exiled: the country
		// that just threw them out is never a destination, even if it survived
		// the revolt. With nowhere left to go they drop out of the game.
		if monarchID != "" {
			destination := engine.PickRandomID(newState.GetAliveCountryIDsExcept(countryID), roller)
			if destination != "" {
				newState.ResettleMonarchAsMerchant(monarchID, destination, DeposedMonarchSeverance)
			}
			evts = append(evts, events.NewMonarchDeposedEvent(
				monarchID, countryID, destination,
				DeposedMonarchSeverance, events.DeposedByRevolution,
			))
		}

		// Whatever the monarch did not take is shared out among the revolters
		spoils := treasury - DeposedMonarchSeverance
		if spoils > 0 && len(participantIDs) > 0 {
			distributeGold(newState, participantIDs, spoils)
			evts = append(evts, events.NewTreasurySplitEvent(countryID, participantIDs, spoils))
		}
	} else {
		// Revolt fails - all participating merchants lose their gold to the king
		totalLost := 0
		for _, mID := range participantIDs {
			merchant := newState.GetMerchant(mID)
			if merchant != nil {
				lost := merchant.LoseAllGold()
				totalLost += lost
			}
		}
		country.AddGold(totalLost)

		evts = append(evts, events.NewRevoltFailedEvent(countryID, participantIDs, totalLost, merchantGold, defenseGold))
	}

	return newState, evts
}

// distributeGold splits gold evenly between the given merchants. Any remainder
// goes one coin at a time to the lowest merchant IDs, so nothing is lost and
// the split does not depend on map iteration order.
func distributeGold(state *engine.GameState, merchantIDs []string, gold int) {
	if len(merchantIDs) == 0 {
		return
	}

	recipients := append([]string(nil), merchantIDs...)
	sort.Strings(recipients)

	share := gold / len(recipients)
	remainder := gold % len(recipients)

	for i, mID := range recipients {
		merchant := state.GetMerchant(mID)
		if merchant == nil {
			continue
		}
		amount := share
		if i < remainder {
			amount++
		}
		merchant.StoredGold += amount
	}
}
