package phases

import (
	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/events"
	"sort"
)

// WarPhase handles Phase 4: War
type WarPhase struct {
	BasePhase
	dice engine.DiceRoller
}

func NewWarPhase(dice engine.DiceRoller) *WarPhase {
	return &WarPhase{
		BasePhase: BasePhase{
			name:      "War",
			phaseType: engine.PhaseWar,
		},
		dice: dice,
	}
}

func (p *WarPhase) ValidActions(state *engine.GameState, playerID string) []actions.Action {
	var validActions []actions.Action

	// Check if player is a monarch
	for _, country := range state.Countries {
		if country.MonarchID == playerID && !country.IsRepublic && country.IsAlive() {
			// Can choose not to attack
			validActions = append(validActions,
				actions.NewNoAttackAction(playerID, country.ID),
			)

			// Can attack any other alive country
			for _, target := range state.GetAliveCountries() {
				if target.ID != country.ID {
					validActions = append(validActions,
						actions.NewAttackAction(playerID, country.ID, target.ID),
					)
				}
			}
		}
	}

	return validActions
}

type battleResult struct {
	attackerID string
	defenderID string
	winnerID   string
	damage     int
	goldBonus  int
}

func (p *WarPhase) Execute(state *engine.GameState, playerActions []actions.Action) (*engine.GameState, []events.Event) {
	snapshot := state.Clone()
	newState := state.Clone()
	var allEvents []events.Event

	// Pass 1: compute all battles against the snapshot state
	var results []battleResult
	for _, action := range playerActions {
		attackAction, ok := action.(*actions.AttackAction)
		if !ok {
			continue
		}
		if err := attackAction.Validate(snapshot); err != nil {
			continue
		}

		attacker := snapshot.GetCountry(attackAction.AttackerID)
		defender := snapshot.GetCountry(attackAction.DefenderID)

		attackerStr := attacker.ArmyStrength
		defenderStr := defender.ArmyStrength

		var winnerID string
		var damage, goldBonus int

		if attackerStr > defenderStr {
			winnerID = attackAction.AttackerID
			damage = attackerStr - defenderStr
			goldBonus = 5
		} else if defenderStr > attackerStr {
			winnerID = attackAction.DefenderID
			damage = defenderStr - attackerStr
			goldBonus = 5
		}

		results = append(results, battleResult{
			attackerID: attackAction.AttackerID,
			defenderID: attackAction.DefenderID,
			winnerID:   winnerID,
			damage:     damage,
			goldBonus:  goldBonus,
		})

		allEvents = append(allEvents, events.NewBattleResolvedEvent(
			attackAction.AttackerID, attackAction.DefenderID,
			attackerStr, defenderStr,
			winnerID, damage,
		))
	}

	// Pass 2: apply outcomes
	// Track accumulated damage per defender and which attackers targeted them
	defenderDamage := make(map[string]int)
	defenderAttackers := make(map[string][]string)

	for _, r := range results {
		// Apply gold bonus to winner
		if r.winnerID != "" {
			winner := newState.GetCountry(r.winnerID)
			if winner != nil {
				winner.AddGold(r.goldBonus)
			}
		}
		// Accumulate damage on defender (only when attacker won)
		if r.winnerID == r.attackerID {
			defenderDamage[r.defenderID] += r.damage
			defenderAttackers[r.defenderID] = append(defenderAttackers[r.defenderID], r.attackerID)
		}
		// Accumulate damage on attacker (only when defender won)
		if r.winnerID == r.defenderID {
			defenderDamage[r.attackerID] += r.damage
			defenderAttackers[r.attackerID] = append(defenderAttackers[r.attackerID], r.defenderID)
		}
	}

	// Apply accumulated damage and handle annexation, in a fixed country order
	// so the dice rolls behind annexation stay reproducible
	damagedIDs := make([]string, 0, len(defenderDamage))
	for defID := range defenderDamage {
		damagedIDs = append(damagedIDs, defID)
	}
	sort.Strings(damagedIDs)

	for _, defID := range damagedIDs {
		totalDamage := defenderDamage[defID]
		def := newState.GetCountry(defID)
		if def == nil {
			continue
		}
		def.TakeDamage(totalDamage)
		if !def.IsAlive() {
			attackerIDs := defenderAttackers[defID]
			allEvents = append(allEvents, p.annex(newState, defID, attackerIDs)...)
		}
	}

	// Handle NoAttack actions (pass-through for non-attack actions)
	for _, action := range playerActions {
		if _, ok := action.(*actions.AttackAction); ok {
			continue
		}
		if err := action.Validate(newState); err != nil {
			continue
		}
		newState, _ = action.Apply(newState, p.dice)
	}

	// After all battles, halve all armies (maintenance cost)
	for _, country := range newState.Countries {
		if country.IsAlive() && country.ArmyStrength > 0 {
			oldStrength := country.ArmyStrength
			country.HalveArmy()
			allEvents = append(allEvents, events.NewArmyMaintenanceEvent(
				country.ID, oldStrength, country.ArmyStrength,
			))
		}
	}

	// End of turn: pay out investments and give all merchants income
	for _, merchant := range newState.Merchants {
		if merchant.InvestedGold > 0 {
			payout := merchant.CollectInvestment()
			allEvents = append(allEvents, events.NewInvestmentPayoutEvent(merchant.ID, payout))
		}
	}
	for _, merchant := range newState.Merchants {
		merchant.ReceiveIncome(5)
		allEvents = append(allEvents, events.NewMerchantIncomeEvent(merchant.ID, 5))
	}

	return newState, allEvents
}

// annex distributes the spoils of a defeated country among its attackers.
func (p *WarPhase) annex(state *engine.GameState, defeatedID string, attackerIDs []string) []events.Event {
	if len(attackerIDs) == 0 {
		return nil
	}

	// Sort attackerIDs for deterministic distribution
	sort.Strings(attackerIDs)

	// Assign merchants round-robin
	merchants := state.GetMerchantsByCountry(defeatedID)
	merchantIDs := make([]string, 0, len(merchants))
	for i, m := range merchants {
		m.CountryID = attackerIDs[i%len(attackerIDs)]
		merchantIDs = append(merchantIDs, m.ID)
	}

	defeated := state.GetCountry(defeatedID)
	if defeated == nil {
		return nil
	}

	var evts []events.Event

	// The defeated monarch flees with the whole treasury as personal savings
	// and starts over as a merchant with one of the victors
	if !defeated.IsRepublic && defeated.MonarchID != "" {
		monarchID := defeated.MonarchID
		savings := defeated.EmptyTreasury()
		defeated.RemoveMonarch()

		destination := engine.PickRandomID(attackerIDs, p.dice)
		if destination != "" {
			state.ResettleMonarchAsMerchant(monarchID, destination, savings)
		}
		evts = append(evts, events.NewMonarchDeposedEvent(
			monarchID, defeatedID, destination, savings, events.DeposedByConquest,
		))
	}

	// Split peasants evenly
	peasants := defeated.Peasants
	share := peasants / len(attackerIDs)
	remainder := peasants % len(attackerIDs)
	for i, id := range attackerIDs {
		attacker := state.GetCountry(id)
		if attacker == nil {
			continue
		}
		extra := 0
		if i < remainder {
			extra = 1
		}
		for j := 0; j < share+extra; j++ {
			attacker.AddPeasant()
		}
	}

	return append(evts, events.NewAnnexationEvent(attackerIDs, defeatedID, merchantIDs))
}
