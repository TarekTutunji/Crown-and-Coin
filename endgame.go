package main

// The end of the game.
//
// The rules give the game no winning condition: it runs until the game leader
// declares it over. When they do, the board stops being a live scoreboard and
// becomes a summary of the whole game - the standings as the rules track them,
// every merchant's gold at last, and the story of what happened, round by
// round.
//
// This is the one moment the third tier of the board's visibility rules opens
// up. Merchant gold - purses, hidden savings, investments - stays secret for the
// whole game precisely because no rule ever reveals it, and the board's payload
// carries no field for it while the game is running (see the comment at the top
// of board.go). Declaring the game over is what unseals it, which is why it
// takes a deliberate act by the game leader and not, say, a country falling.
//
// Nothing here changes the game. The engine is untouched, the phases go on
// working, and the game leader can dismiss the summary and carry on playing -
// useful if they hit it early, or want to show the standings and then play one
// more round.

import (
	"sort"

	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
)

// FinalSummary is the end-game screen: everything the room gets to know once
// the game leader has called the game
type FinalSummary struct {
	GameName  string          `json:"game_name"`
	Turn      int             `json:"turn"`  // The round the game stopped in
	Phase     string          `json:"phase"` // The phase it stopped in
	Tally     FinalTally      `json:"tally"`
	Realms    []FinalRealm    `json:"realms"`
	Merchants []FinalMerchant `json:"merchants"`
	Timeline  []FinalRound    `json:"timeline"`
}

// FinalTally is the size of the game in a few numbers, for the strip across the
// top of the summary
type FinalTally struct {
	Rounds         int `json:"rounds"`
	Battles        int `json:"battles"`
	RealmsStanding int `json:"realms_standing"`
	RealmsFallen   int `json:"realms_fallen"`
	MerchantGold   int `json:"merchant_gold"`
	// HiddenGold is the part of that gold nobody but its owner had seen until
	// this screen
	HiddenGold int `json:"hidden_gold"`
}

// FinalRealm is one country with nothing held back: the army it really has,
// rather than the one it was last seen with, and its treasury
type FinalRealm struct {
	CountryID  string   `json:"country_id"`
	HP         int      `json:"hp"`
	MaxHP      int      `json:"max_hp"`
	Alive      bool     `json:"alive"`
	DiedOnce   bool     `json:"died_once"`
	IsRepublic bool     `json:"is_republic"`
	Ruler      string   `json:"ruler"`
	Army       int      `json:"army"`
	Treasury   int      `json:"treasury"`
	Peasants   int      `json:"peasants"`
	Merchants  []string `json:"merchants"`
}

// FinalMerchant is one merchant's whole fortune, the hidden part included
type FinalMerchant struct {
	PlayerID  string `json:"player_id"`
	CountryID string `json:"country_id"`
	Arriving  bool   `json:"arriving"`
	Purse     int    `json:"purse"`
	Hidden    int    `json:"hidden"`
	Invested  int    `json:"invested"`
	Total     int    `json:"total"`
	// Rank counts from 1, and merchants who tie share a rank
	Rank int `json:"rank"`
}

// FinalRound is one round of the game's story
type FinalRound struct {
	Turn  int         `json:"turn"`
	Beats []FinalBeat `json:"beats"`
}

// FinalBeat is one thing worth remembering about a round
type FinalBeat struct {
	Kind  string `json:"kind"`
	Phase string `json:"phase"`
	Text  string `json:"text"`
	// Biggest marks the largest clash of the game, by the armies both sides
	// brought to it
	Biggest bool `json:"biggest,omitempty"`
}

// majorBeatKinds are the events the game's story is told with, and the kind
// each one is drawn as. Everything else that happened - income, taxes,
// investments, a phase starting - is detail the summary leaves out.
var majorBeatKinds = map[string]string{
	"battle_resolved":    "battle",
	"annexation":         "conquest",
	"country_collapsed":  "collapse",
	"republic_fallen":    "collapse",
	"republic_abandoned": "collapse",
	"republic_formed":    "government",
	"peasant_revolt":     "revolt",
	"revolt_success":     "revolt",
	"revolt_failed":      "revolt-failed",
	"monarch_deposed":    "exile",
}

// finalSummary builds the end-game screen from the game as it stands and the
// history it has written. The caller holds gameMu.
func (s *Server) finalSummary() *FinalSummary {
	state := s.api.GetEngine().GetState()

	s.historyMu.RLock()
	summary := &FinalSummary{
		GameName: s.history.GameName,
		Turn:     state.Turn,
		Phase:    state.Phase.String(),
		Timeline: gameTimeline(s.history.StateSnapshots),
	}
	battles := countBattles(s.history.StateSnapshots)
	rounds := roundsPlayed(s.history.StateSnapshots)
	s.historyMu.RUnlock()

	summary.Realms = finalRealms(state)
	summary.Merchants = finalMerchants(state)

	summary.Tally = FinalTally{Rounds: rounds, Battles: battles}
	for _, realm := range summary.Realms {
		if realm.Alive {
			summary.Tally.RealmsStanding++
		} else {
			summary.Tally.RealmsFallen++
		}
	}
	for _, merchant := range summary.Merchants {
		summary.Tally.MerchantGold += merchant.Total
		summary.Tally.HiddenGold += merchant.Hidden
	}
	return summary
}

// finalRealms lists the countries with the ones still standing first, the
// strongest of those at the top. The rules name no victor, so this is an
// ordering of what they do track - health, then army, then treasury - and not a
// claim about who won.
func finalRealms(state *engine.GameState) []FinalRealm {
	realms := make([]FinalRealm, 0, len(state.Countries))
	for _, c := range state.Countries {
		merchants := make([]string, 0)
		for _, m := range state.GetMerchantsByCountry(c.ID) {
			merchants = append(merchants, m.ID)
		}
		sort.Strings(merchants)
		realms = append(realms, FinalRealm{
			CountryID:  c.ID,
			HP:         max(0, c.HP),
			MaxHP:      boardMaxHP,
			Alive:      c.IsAlive(),
			DiedOnce:   c.DiedOnce,
			IsRepublic: c.IsRepublic,
			Ruler:      c.MonarchID,
			Army:       c.ArmyStrength,
			Treasury:   c.Gold,
			Peasants:   c.Peasants,
			Merchants:  merchants,
		})
	}

	sort.Slice(realms, func(i, j int) bool {
		a, b := realms[i], realms[j]
		if a.Alive != b.Alive {
			return a.Alive
		}
		if a.HP != b.HP {
			return a.HP > b.HP
		}
		if a.Army != b.Army {
			return a.Army > b.Army
		}
		if a.Treasury != b.Treasury {
			return a.Treasury > b.Treasury
		}
		return a.CountryID < b.CountryID
	})
	return realms
}

// finalMerchants lists every merchant still in the game, richest first, with
// the gold they had been keeping to themselves
func finalMerchants(state *engine.GameState) []FinalMerchant {
	merchants := make([]FinalMerchant, 0, len(state.Merchants))
	for _, m := range state.Merchants {
		merchants = append(merchants, FinalMerchant{
			PlayerID:  m.ID,
			CountryID: m.CountryID,
			Arriving:  m.Arriving,
			Purse:     m.StoredGold,
			Hidden:    m.HiddenGold,
			Invested:  m.InvestedGold,
			Total:     m.TotalGold(),
		})
	}

	sort.Slice(merchants, func(i, j int) bool {
		if merchants[i].Total != merchants[j].Total {
			return merchants[i].Total > merchants[j].Total
		}
		return merchants[i].PlayerID < merchants[j].PlayerID
	})

	// Equal fortunes share a rank, so two merchants on 30 gold are both second
	for i := range merchants {
		if i > 0 && merchants[i].Total == merchants[i-1].Total {
			merchants[i].Rank = merchants[i-1].Rank
			continue
		}
		merchants[i].Rank = i + 1
	}
	return merchants
}

// roundsPlayed counts the rounds that were played out to the end. The game is
// usually called part way through a round - the summary says which round in its
// own right - and a round only counts here once its Assessment, the last phase,
// has been resolved.
func roundsPlayed(snapshots []StateSnapshot) int {
	rounds := 0
	for _, snapshot := range snapshots {
		if snapshot.Phase == "assessment" {
			rounds++
		}
	}
	return rounds
}

// countBattles counts the battles fought over the whole game
func countBattles(snapshots []StateSnapshot) int {
	battles := 0
	for _, snapshot := range snapshots {
		for _, e := range snapshot.Events {
			if e.Type == "battle_resolved" {
				battles++
			}
		}
	}
	return battles
}

// gameTimeline retells the game from the history, one group of beats per round.
// The wording is the chronicle's: by now every number in it is public, so a beat
// can say what a thing really cost.
func gameTimeline(snapshots []StateSnapshot) []FinalRound {
	rounds := make([]FinalRound, 0)
	at := map[int]int{} // Round number -> where that round sits in rounds

	// The largest clash of the game, measured by what both sides brought to it
	biggest := 0
	for _, snapshot := range snapshots {
		for _, e := range snapshot.Events {
			if e.Type != "battle_resolved" {
				continue
			}
			if strength := battleStrength(e); strength > biggest {
				biggest = strength
			}
		}
	}

	for _, snapshot := range snapshots {
		for _, e := range snapshot.Events {
			kind, major := majorBeatKinds[e.Type]
			if !major {
				continue
			}
			text := sealedLine(e)
			if text == "" {
				continue
			}
			beat := FinalBeat{Kind: kind, Phase: snapshot.Phase, Text: text}
			if e.Type == "battle_resolved" && biggest > 0 && battleStrength(e) == biggest {
				beat.Biggest = true
			}

			where, seen := at[snapshot.Turn]
			if !seen {
				rounds = append(rounds, FinalRound{Turn: snapshot.Turn})
				where = len(rounds) - 1
				at[snapshot.Turn] = where
			}
			rounds[where].Beats = append(rounds[where].Beats, beat)
		}
	}

	sort.SliceStable(rounds, func(i, j int) bool { return rounds[i].Turn < rounds[j].Turn })
	return rounds
}

// battleStrength is the size of a battle: everything both sides committed to it
func battleStrength(e jsonapi.EventJSON) int {
	return evtInt(e.Data, "attacker_strength") + evtInt(e.Data, "defender_strength")
}
