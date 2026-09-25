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

	// Merchants of a republic vote on whom to attack
	if merchant := state.GetMerchant(playerID); merchant != nil && !merchant.Arriving {
		country := state.GetCountry(merchant.CountryID)
		if country != nil && country.IsRepublic && country.IsAlive() {
			validActions = append(validActions,
				actions.NewVoteNoAttackAction(playerID, merchant.ID),
			)
			for _, target := range state.GetAliveCountries() {
				if target.ID != country.ID {
					validActions = append(validActions,
						actions.NewVoteAttackAction(playerID, merchant.ID, target.ID),
					)
				}
			}
		}
	}

	return validActions
}

// VictoryGold is what a winning attacker earns from the bank
const VictoryGold = 5

type battleResult struct {
	attackerID string
	defenderID string
	winnerID   string
	damage     int
	mutual     bool // Both sides declared war on each other
}

func (p *WarPhase) Execute(state *engine.GameState, playerActions []actions.Action) (*engine.GameState, []events.Event) {
	snapshot := state.Clone()
	newState := state.Clone()
	var allEvents []events.Event

	// Pass 1: compute all battles against the snapshot state. When two
	// countries attack each other it is a single battle.
	var results []*battleResult
	battles := make(map[[2]string]*battleResult)
	fight := func(attackerID, defenderID string) {
		if r := battles[[2]string{defenderID, attackerID}]; r != nil {
			r.mutual = true
			return
		}
		if battles[[2]string{attackerID, defenderID}] != nil {
			return
		}

		attackerStr := snapshot.GetCountry(attackerID).ArmyStrength
		defenderStr := snapshot.GetCountry(defenderID).ArmyStrength

		r := &battleResult{attackerID: attackerID, defenderID: defenderID}
		if attackerStr > defenderStr {
			r.winnerID = attackerID
			r.damage = attackerStr - defenderStr
		} else if defenderStr > attackerStr {
			r.winnerID = defenderID
			r.damage = defenderStr - attackerStr
		}
		battles[[2]string{attackerID, defenderID}] = r
		results = append(results, r)

		allEvents = append(allEvents, events.NewBattleResolvedEvent(
			attackerID, defenderID,
			attackerStr, defenderStr,
			r.winnerID, r.damage,
		))
	}

	for _, action := range playerActions {
		attackAction, ok := action.(*actions.AttackAction)
		if !ok {
			continue
		}
		if err := attackAction.Validate(snapshot); err != nil {
			continue
		}
		fight(attackAction.AttackerID, attackAction.DefenderID)
	}

	// Republics attack whichever target a strict majority of their merchants
	// voted for, counting only the first vote of each merchant
	warVotes := make(map[string]map[string]int) // countryID -> targetID -> votes
	voted := make(map[string]bool)
	for _, action := range playerActions {
		var merchantID, targetID string
		switch a := action.(type) {
		case *actions.VoteAttackAction:
			merchantID, targetID = a.MerchantID, a.TargetID
		case *actions.VoteNoAttackAction:
			merchantID = a.MerchantID
		default:
			continue
		}
		if err := action.Validate(snapshot); err != nil || voted[merchantID] {
			continue
		}
		voted[merchantID] = true
		if targetID == "" {
			continue
		}
		countryID := snapshot.GetMerchant(merchantID).CountryID
		if warVotes[countryID] == nil {
			warVotes[countryID] = make(map[string]int)
		}
		warVotes[countryID][targetID]++
	}

	republicIDs := make([]string, 0, len(warVotes))
	for countryID := range warVotes {
		republicIDs = append(republicIDs, countryID)
	}
	sort.Strings(republicIDs)

	for _, countryID := range republicIDs {
		merchantCount := len(snapshot.GetMerchantsByCountry(countryID))
		targetID, voteEvent := actions.ResolveRepublicWarVote(countryID, warVotes[countryID], merchantCount)
		allEvents = append(allEvents, voteEvent)
		if targetID != "" {
			fight(countryID, targetID)
		}
	}

	// Pass 2: apply outcomes. A winning attacker earns the victory gold (a
	// republic shares it among its merchants); a winning defender gets
	// nothing. The loser, attacker or defender, takes the damage, which adds
	// up over several lost battles.
	damageTaken := make(map[string]int)
	beatenBy := make(map[string][]string)

	for _, r := range results {
		if r.winnerID == "" {
			continue
		}
		loserID := r.defenderID
		if r.winnerID == r.defenderID {
			loserID = r.attackerID
		}
		if r.winnerID == r.attackerID || r.mutual {
			allEvents = append(allEvents, actions.GiveGoldToCountry(newState, r.winnerID, VictoryGold, events.GoldFromVictory, p.dice)...)
		}
		damageTaken[loserID] += r.damage
		beatenBy[loserID] = append(beatenBy[loserID], r.winnerID)
	}

	// Deaths are only looked at once every battle has been fought, in a fixed
	// country order so the dice rolls behind conquest stay reproducible
	damagedIDs := make([]string, 0, len(damageTaken))
	for id := range damageTaken {
		damagedIDs = append(damagedIDs, id)
	}
	sort.Strings(damagedIDs)

	for _, id := range damagedIDs {
		newState.GetCountry(id).TakeDamage(damageTaken[id])
	}

	for _, id := range damagedIDs {
		if newState.GetCountry(id).IsAlive() {
			continue
		}
		// The countries that beat it and are still standing share it out. If
		// none of them survived, every surviving country does.
		winners := make([]string, 0)
		seen := make(map[string]bool)
		for _, winnerID := range beatenBy[id] {
			if !seen[winnerID] && newState.GetCountry(winnerID).IsAlive() {
				seen[winnerID] = true
				winners = append(winners, winnerID)
			}
		}
		if len(winners) == 0 {
			winners = newState.GetAliveCountryIDs()
		}
		allEvents = append(allEvents, p.annex(newState, id, winners)...)
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

	// Other players only learn army sizes once the war is over
	newState.PublishArmies()

	// Every merchant receives their income. Investments pay out later, at the
	// start of the next round.
	for _, merchant := range newState.Merchants {
		merchant.ReceiveIncome(5)
		allEvents = append(allEvents, events.NewMerchantIncomeEvent(merchant.ID, 5))
	}

	return newState, allEvents
}

// annex shares out a country conquered in war among the countries that beat
// it. Its merchants, its peasants and its treasury are split as evenly as
// possible, with the dice deciding who gets any leftovers. The fallen monarch
// becomes a merchant and is split up at random along with the other
// merchants, who keep their purse, hidden gold and investments.
func (p *WarPhase) annex(state *engine.GameState, defeatedID string, winners []string) []events.Event {
	defeated := state.GetCountry(defeatedID)
	if defeated == nil || len(winners) == 0 {
		return nil
	}

	// With several winners the dice decide who comes first, and so who gets
	// any merchant, peasant or gold left over after an even split
	if len(winners) > 1 {
		winners = engine.ShuffleIDs(winners, p.dice)
	}

	var evts []events.Event

	fallenMonarch := ""
	if !defeated.IsRepublic && defeated.MonarchID != "" {
		fallenMonarch = defeated.MonarchID
		defeated.RemoveMonarch()
		state.ResettleMonarchAsMerchant(fallenMonarch, defeatedID)
	}

	merchantIDs, _ := state.ScatterMerchants(defeatedID, winners, false, p.dice)
	if fallenMonarch != "" {
		evts = append(evts, events.NewMonarchDeposedEvent(
			fallenMonarch, defeatedID, state.GetMerchant(fallenMonarch).CountryID,
			engine.FallenMonarchPurse, events.DeposedByConquest,
		))
	}

	treasury := defeated.EmptyTreasury()
	goldShares := engine.SplitEvenly(treasury, len(winners))
	peasantShares := engine.SplitEvenly(defeated.Peasants, len(winners))
	defeated.Peasants = 0
	for i, id := range winners {
		state.GetCountry(id).Peasants += peasantShares[i]
		evts = append(evts, actions.GiveGoldToCountry(state, id, goldShares[i], events.GoldFromConquest, p.dice)...)
	}

	return append(evts, events.NewAnnexationEvent(winners, defeatedID, merchantIDs, treasury))
}
