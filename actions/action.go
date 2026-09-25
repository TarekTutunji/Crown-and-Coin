package actions

import (
	"errors"

	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// ActionType identifies the type of action
type ActionType string

const (
	// Taxation phase actions
	ActionTaxPeasantsLow  ActionType = "tax_peasants_low"  // 5 gold, no risk
	ActionTaxPeasantsHigh ActionType = "tax_peasants_high" // 10 gold, revolt risk
	ActionTaxMerchants    ActionType = "tax_merchants"

	// Spending phase actions
	ActionBuildArmy       ActionType = "build_army"
	ActionMonarchInvest   ActionType = "monarch_invest" // Gift gold to a merchant
	ActionMonarchSave     ActionType = "monarch_save"
	ActionMerchantInvest  ActionType = "merchant_invest"
	ActionMerchantHide    ActionType = "merchant_hide"
	ActionMerchantUnhide  ActionType = "merchant_unhide"

	// War phase actions
	ActionAttack ActionType = "attack"
	ActionNoAttack ActionType = "no_attack"

	// Assessment phase actions
	ActionRemain ActionType = "remain"
	ActionFlee   ActionType = "flee"
	ActionRevolt ActionType = "revolt"

	// Merchant republic actions
	ActionVoteTaxLow     ActionType = "vote_tax_low"
	ActionVoteTaxHigh    ActionType = "vote_tax_high"
	ActionContributeArmy ActionType = "contribute_army"
	ActionVoteAttack     ActionType = "vote_attack"
	ActionVoteNoAttack   ActionType = "vote_no_attack"
)

// Action defines the interface for a player action
type Action interface {
	// Type returns the action type
	Type() ActionType

	// PlayerID returns the ID of the player taking this action
	PlayerID() string

	// Validate checks if this action is valid given the current state
	Validate(state *engine.GameState) error

	// Apply executes the action and returns the new state and any events
	Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event)
}

// BaseAction provides common functionality for actions
type BaseAction struct {
	actionType ActionType
	playerID   string
}

func (a *BaseAction) Type() ActionType {
	return a.actionType
}

func (a *BaseAction) PlayerID() string {
	return a.playerID
}

// errArriving is returned for any action of a merchant who moved this round:
// they sit out the rest of the round and join their new country at the
// start of the next one
var errArriving = errors.New("merchant is still on the way and joins their new country at the start of the next round")
