package actions

import (
	"errors"
	"sort"

	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// validateRepublicMerchant checks that playerID controls merchantID and that
// the merchant belongs to a living merchant republic, which it returns
func validateRepublicMerchant(state *engine.GameState, playerID, merchantID string) (*engine.Country, error) {
	merchant := state.GetMerchant(merchantID)
	if merchant == nil {
		return nil, errors.New("merchant not found")
	}
	if merchant.ID != playerID {
		return nil, errors.New("can only control your own merchant")
	}
	country := state.GetCountry(merchant.CountryID)
	if country == nil {
		return nil, errors.New("country not found")
	}
	if !country.IsRepublic {
		return nil, errors.New("only merchants of a republic can vote")
	}
	if !country.IsAlive() {
		return nil, errors.New("country is not alive")
	}
	return country, nil
}

// VoteTaxAction - Republic merchant votes for low or high peasant tax
type VoteTaxAction struct {
	BaseAction
	MerchantID string
	HighTax    bool
}

func NewVoteTaxAction(playerID, merchantID string, highTax bool) *VoteTaxAction {
	actionType := ActionVoteTaxLow
	if highTax {
		actionType = ActionVoteTaxHigh
	}
	return &VoteTaxAction{
		BaseAction: BaseAction{actionType: actionType, playerID: playerID},
		MerchantID: merchantID,
		HighTax:    highTax,
	}
}

func (a *VoteTaxAction) Validate(state *engine.GameState) error {
	_, err := validateRepublicMerchant(state, a.playerID, a.MerchantID)
	return err
}

func (a *VoteTaxAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	// A single vote only marks intention; the phase counts them all
	return state.Clone(), nil
}

// ResolveRepublicTax taxes the peasants of a republic at the rate its
// merchants voted for and shares the gold evenly among all of its merchants,
// however each of them voted. A tied vote (including no votes at all) means
// low tax.
func ResolveRepublicTax(state *engine.GameState, countryID string, highVotes, lowVotes int, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	country := newState.GetCountry(countryID)

	highTax := highVotes > lowVotes
	evts := []events.Event{events.NewRepublicTaxVoteEvent(countryID, highVotes, lowVotes, highTax)}

	gold, taxEvents := CollectPeasantTax(country, highTax, roller)
	evts = append(evts, taxEvents...)
	evts = append(evts, ShareGoldAmongMerchants(newState, countryID, gold, events.GoldFromPeasantTax)...)
	return newState, evts
}

// ContributeArmyAction - Republic merchant pays into the communal army
type ContributeArmyAction struct {
	BaseAction
	MerchantID string
	Amount     int // Gold to give (1 gold = 1 army strength)
}

func NewContributeArmyAction(playerID, merchantID string, amount int) *ContributeArmyAction {
	return &ContributeArmyAction{
		BaseAction: BaseAction{actionType: ActionContributeArmy, playerID: playerID},
		MerchantID: merchantID,
		Amount:     amount,
	}
}

func (a *ContributeArmyAction) Validate(state *engine.GameState) error {
	if _, err := validateRepublicMerchant(state, a.playerID, a.MerchantID); err != nil {
		return err
	}
	if a.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if state.GetMerchant(a.MerchantID).StoredGold < a.Amount {
		return errors.New("insufficient stored gold")
	}
	return nil
}

func (a *ContributeArmyAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	merchant := newState.GetMerchant(a.MerchantID)
	country := newState.GetCountry(merchant.CountryID)

	merchant.StoredGold -= a.Amount
	country.AddArmy(a.Amount)

	return newState, []events.Event{
		events.NewArmyContributedEvent(country.ID, a.MerchantID, a.Amount, country.ArmyStrength),
	}
}

// VoteAttackAction - Republic merchant votes to attack a rival kingdom
type VoteAttackAction struct {
	BaseAction
	MerchantID string
	TargetID   string
}

func NewVoteAttackAction(playerID, merchantID, targetID string) *VoteAttackAction {
	return &VoteAttackAction{
		BaseAction: BaseAction{actionType: ActionVoteAttack, playerID: playerID},
		MerchantID: merchantID,
		TargetID:   targetID,
	}
}

func (a *VoteAttackAction) Validate(state *engine.GameState) error {
	country, err := validateRepublicMerchant(state, a.playerID, a.MerchantID)
	if err != nil {
		return err
	}
	target := state.GetCountry(a.TargetID)
	if target == nil {
		return errors.New("target country not found")
	}
	if !target.IsAlive() {
		return errors.New("cannot attack a defeated country")
	}
	if target.ID == country.ID {
		return errors.New("cannot attack yourself")
	}
	return nil
}

func (a *VoteAttackAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	// A single vote only marks intention; the phase counts them all
	return state.Clone(), nil
}

// VoteNoAttackAction - Republic merchant votes against attacking anyone
type VoteNoAttackAction struct {
	BaseAction
	MerchantID string
}

func NewVoteNoAttackAction(playerID, merchantID string) *VoteNoAttackAction {
	return &VoteNoAttackAction{
		BaseAction: BaseAction{actionType: ActionVoteNoAttack, playerID: playerID},
		MerchantID: merchantID,
	}
}

func (a *VoteNoAttackAction) Validate(state *engine.GameState) error {
	_, err := validateRepublicMerchant(state, a.playerID, a.MerchantID)
	return err
}

func (a *VoteNoAttackAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	return state.Clone(), nil
}

// ResolveRepublicWarVote picks the target a republic attacks this round. A
// target needs a strict majority of all the republic's merchants, not just of
// those who voted; otherwise (including exact ties) nobody is attacked and the
// returned target is empty. votes maps target country IDs to vote counts.
func ResolveRepublicWarVote(countryID string, votes map[string]int, merchantCount int) (string, events.Event) {
	targets := make([]string, 0, len(votes))
	for target := range votes {
		targets = append(targets, target)
	}
	sort.Strings(targets)

	winner := ""
	for _, target := range targets {
		if votes[target]*2 > merchantCount {
			winner = target
			break
		}
	}

	return winner, events.NewRepublicWarVoteEvent(countryID, votes, merchantCount, winner)
}
