package actions

import (
	"errors"
	"crown_and_coin/engine"
	"crown_and_coin/events"
)

// AttackAction - Monarch attacks another country
type AttackAction struct {
	BaseAction
	AttackerID string
	DefenderID string
}

func NewAttackAction(playerID, attackerID, defenderID string) *AttackAction {
	return &AttackAction{
		BaseAction: BaseAction{actionType: ActionAttack, playerID: playerID},
		AttackerID: attackerID,
		DefenderID: defenderID,
	}
}

func (a *AttackAction) Validate(state *engine.GameState) error {
	attacker := state.GetCountry(a.AttackerID)
	if attacker == nil {
		return errors.New("attacking country not found")
	}
	if !attacker.IsAlive() {
		return errors.New("attacking country is not alive")
	}
	if attacker.IsRepublic {
		return errors.New("republics vote on attacks")
	}
	if attacker.MonarchID != a.playerID {
		return errors.New("only the monarch can order attacks")
	}

	defender := state.GetCountry(a.DefenderID)
	if defender == nil {
		return errors.New("defending country not found")
	}
	if !defender.IsAlive() {
		return errors.New("cannot attack a defeated country")
	}
	if a.AttackerID == a.DefenderID {
		return errors.New("cannot attack yourself")
	}

	return nil
}

func (a *AttackAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	newState := state.Clone()
	attacker := newState.GetCountry(a.AttackerID)
	defender := newState.GetCountry(a.DefenderID)
	var evts []events.Event

	// Resolve battle
	attackerStr := attacker.ArmyStrength
	defenderStr := defender.ArmyStrength

	var winnerID string
	var damage int

	if attackerStr > defenderStr {
		// Attacker wins
		winnerID = a.AttackerID
		damage = attackerStr - defenderStr
		defender.TakeDamage(damage)
		attacker.AddGold(5) // Victory bonus
	} else if defenderStr > attackerStr {
		// Defender wins
		winnerID = a.DefenderID
		damage = defenderStr - attackerStr
		attacker.TakeDamage(damage) // A winning defender earns nothing
	} else {
		// Tie - no damage, no winner
		winnerID = ""
		damage = 0
	}

	evts = append(evts, events.NewBattleResolvedEvent(
		a.AttackerID, a.DefenderID,
		attackerStr, defenderStr,
		winnerID, damage,
	))

	return newState, evts
}

// NoAttackAction - Monarch chooses not to attack
type NoAttackAction struct {
	BaseAction
	CountryID string
}

func NewNoAttackAction(playerID, countryID string) *NoAttackAction {
	return &NoAttackAction{
		BaseAction: BaseAction{actionType: ActionNoAttack, playerID: playerID},
		CountryID:  countryID,
	}
}

func (a *NoAttackAction) Validate(state *engine.GameState) error {
	country := state.GetCountry(a.CountryID)
	if country == nil {
		return errors.New("country not found")
	}
	if country.IsRepublic {
		return errors.New("republics vote on attacks")
	}
	if country.MonarchID != a.playerID {
		return errors.New("only the monarch can decide on attacks")
	}
	return nil
}

func (a *NoAttackAction) Apply(state *engine.GameState, roller engine.DiceRoller) (*engine.GameState, []events.Event) {
	// No attack is a no-op
	return state.Clone(), nil
}
