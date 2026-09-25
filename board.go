package main

// The public projector board.
//
// The board is the screen at the front of the room. Everybody sees it at once,
// including players whose own screens are only allowed a small part of the
// truth, so everything on it has to be something the room is already entitled
// to know. Three tiers decide that, and every event in the game belongs to
// exactly one of them:
//
//   - Public. Things that happen in the open: battles, conquests, revolts,
//     monarchs losing their thrones, merchants fleeing, war declarations,
//     damage taken. These go on the board as soon as the phase is resolved.
//     Nothing leaks by their absence either, because whether they happened is
//     public in the room anyway.
//
//   - Sealed until the next war. Army sizes and a country's own money: how
//     much a country built, what its peasants paid. The war phase publishes
//     every army (see engine.GameState.PublishArmies), so a finished war is
//     the moment these numbers stop being secret. Until then the board says
//     nothing about them at all - not even that a country spent nothing,
//     which would be a tell in itself.
//
//   - Secret to the end. Merchant gold: purses, hidden gold, investments,
//     merchant taxes, payouts, shares of a treasury. Nothing in the rules ever
//     reveals these mid-game, so the board never shows them. They come out on
//     the end-game screen.
//
// What the board shows live, then, is each realm's public standing (health,
// ruler, government, the army as of the last war, which merchants are where)
// plus the beats of the phase that just finished. Rounds unsealed by a
// finished war are also kept, with their real numbers, as a chronicle.

import (
	"encoding/json"
	"net/http"
	"sort"

	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
)

// boardMaxHP is the health every country starts with, used to size the health
// bars on the board
const boardMaxHP = 10

// BoardData is everything the board page asks for in one poll. It is
// deliberately small: the board needs the public standing and the beats of the
// last phase, not the whole game history the game leader's screen loads.
type BoardData struct {
	GameName   string           `json:"game_name"`
	Turn       int              `json:"turn"`
	Phase      string           `json:"phase"`
	Revision   int              `json:"revision"`
	Realms     []BoardRealm     `json:"realms"`
	Travellers []BoardTraveller `json:"travellers"`
	LastPhase  *BoardPhase      `json:"last_phase"`
	Chronicle  []BoardRound     `json:"chronicle"`
}

// BoardRealm is one country as the whole room may see it
type BoardRealm struct {
	CountryID   string   `json:"country_id"`
	HP          int      `json:"hp"`
	MaxHP       int      `json:"max_hp"`
	Alive       bool     `json:"alive"`
	DiedOnce    bool     `json:"died_once"`
	IsRepublic  bool     `json:"is_republic"`
	Ruler       string   `json:"ruler"` // Empty for a republic, or a monarchy with no monarch
	ArmyLastWar int      `json:"army_last_war"`
	Merchants   []string `json:"merchants"`
}

// BoardTraveller is a merchant on the road, who joins their new country at the
// start of the next round
type BoardTraveller struct {
	PlayerID  string `json:"player_id"`
	ToCountry string `json:"to_country"`
}

// BoardPhase is the phase that finished most recently, broken into the beats
// the game leader steps through one at a time
type BoardPhase struct {
	Turn  int         `json:"turn"`
	Phase string      `json:"phase"`
	Beats []BoardBeat `json:"beats"`
}

// BoardBeat is one moment of a phase: a line to read out, whom it is about,
// and optionally a battle to draw or a number to count from and to.
type BoardBeat struct {
	Kind   string       `json:"kind"`
	Text   string       `json:"text"`
	Focus  []string     `json:"focus,omitempty"`
	Battle *BoardBattle `json:"battle,omitempty"`
	Change *BoardChange `json:"change,omitempty"`
	// Armies is set on the upkeep beat that ends a war, which is where every
	// army becomes public at once
	Armies []BoardChange `json:"armies,omitempty"`
}

// BoardBattle is the two sides of one battle, for the crests-facing-off
// picture the board draws
type BoardBattle struct {
	AttackerID       string `json:"attacker_id"`
	DefenderID       string `json:"defender_id"`
	AttackerStrength int    `json:"attacker_strength"`
	DefenderStrength int    `json:"defender_strength"`
	AttackerRepublic bool   `json:"attacker_republic"`
	DefenderRepublic bool   `json:"defender_republic"`
	WinnerID         string `json:"winner_id"` // Empty for a draw
	LoserID          string `json:"loser_id"`  // Empty for a draw
	Damage           int    `json:"damage"`
}

// BoardChange is a number before and after, so the board can animate it
type BoardChange struct {
	Subject string `json:"subject"`
	Label   string `json:"label"`
	From    int    `json:"from"`
	To      int    `json:"to"`
	Max     int    `json:"max"`
}

// BoardRound is one round of the chronicle, kept once a finished war has
// unsealed it
type BoardRound struct {
	Turn   int               `json:"turn"`
	Phases []BoardRoundPhase `json:"phases"`
}

// BoardRoundPhase is the full record of one phase, with the real numbers
type BoardRoundPhase struct {
	Phase string   `json:"phase"`
	Lines []string `json:"lines"`
}

// BoardProjection remembers what the board is allowed to show. Its fields are
// guarded by Server.gameMu, like the game itself.
type BoardProjection struct {
	revision  int
	lastPhase *BoardPhase
	chronicle []BoardRound
	// sealed holds the phases whose real numbers are still waiting for a war
	// to finish and unseal them
	sealed []BoardRoundPhase
	// sealedTurns is the round each sealed phase belongs to
	sealedTurns []int
}

// NewBoardProjection starts an empty board, as for a game that has not begun
func NewBoardProjection() *BoardProjection {
	return &BoardProjection{}
}

// Reset clears the board for a new game. The revision keeps counting up, so
// the board page still sees a change and does not mistake the new game for the
// one it was already showing.
func (b *BoardProjection) Reset() {
	*b = BoardProjection{revision: b.revision + 1}
}

// RecordPhase notes a phase the game leader has just resolved. before and
// after are the game state on either side of it, which is how a beat knows the
// health a country went from and to.
func (b *BoardProjection) RecordPhase(turn int, phase string, evts []jsonapi.EventJSON, before, after *engine.GameState) {
	b.revision++
	b.lastPhase = &BoardPhase{Turn: turn, Phase: phase, Beats: publicBeats(phase, evts, before, after)}

	if lines := sealedLines(evts); len(lines) > 0 {
		b.sealed = append(b.sealed, BoardRoundPhase{Phase: phase, Lines: lines})
		b.sealedTurns = append(b.sealedTurns, turn)
	}

	// A finished war publishes every army, and with it everything the round
	// was holding back
	if phase == "war" {
		b.release()
	}
}

// release moves the phases waiting on a war into the chronicle, each under its
// own round
func (b *BoardProjection) release() {
	for i, sealed := range b.sealed {
		turn := b.sealedTurns[i]
		at := -1
		for j := range b.chronicle {
			if b.chronicle[j].Turn == turn {
				at = j
				break
			}
		}
		if at < 0 {
			b.chronicle = append(b.chronicle, BoardRound{Turn: turn})
			at = len(b.chronicle) - 1
		}
		b.chronicle[at].Phases = append(b.chronicle[at].Phases, sealed)
	}
	b.sealed = nil
	b.sealedTurns = nil

	sort.SliceStable(b.chronicle, func(i, j int) bool {
		return b.chronicle[i].Turn < b.chronicle[j].Turn
	})
}

// boardData builds the board's answer from the live game. The caller holds
// gameMu.
func (s *Server) boardData() *BoardData {
	state := s.api.GetEngine().GetState()

	s.historyMu.RLock()
	gameName := s.history.GameName
	s.historyMu.RUnlock()

	data := &BoardData{
		GameName:  gameName,
		Turn:      state.Turn,
		Phase:     state.Phase.String(),
		Revision:  s.board.revision,
		LastPhase: s.board.lastPhase,
		Chronicle: s.board.chronicle,
	}
	if data.Chronicle == nil {
		data.Chronicle = []BoardRound{}
	}

	countryIDs := make([]string, 0, len(state.Countries))
	for id := range state.Countries {
		countryIDs = append(countryIDs, id)
	}
	sort.Strings(countryIDs)

	data.Realms = make([]BoardRealm, 0, len(countryIDs))
	for _, id := range countryIDs {
		c := state.GetCountry(id)
		merchants := make([]string, 0)
		for _, m := range state.GetMerchantsByCountry(id) {
			merchants = append(merchants, m.ID)
		}
		data.Realms = append(data.Realms, BoardRealm{
			CountryID:   c.ID,
			HP:          max(0, c.HP),
			MaxHP:       boardMaxHP,
			Alive:       c.IsAlive(),
			DiedOnce:    c.DiedOnce,
			IsRepublic:  c.IsRepublic,
			Ruler:       c.MonarchID,
			ArmyLastWar: c.PublicArmy,
			Merchants:   merchants,
		})
	}

	travellers := make([]BoardTraveller, 0)
	for _, m := range state.Merchants {
		if m.Arriving {
			travellers = append(travellers, BoardTraveller{PlayerID: m.ID, ToCountry: m.CountryID})
		}
	}
	sort.Slice(travellers, func(i, j int) bool { return travellers[i].PlayerID < travellers[j].PlayerID })
	data.Travellers = travellers

	return data
}

// handleBoard answers the projector board. It needs no login: the board hangs
// on the wall, and by the rules above it only ever carries what the room may
// already see.
func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	s.gameMu.Lock()
	data := s.boardData()
	s.gameMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(data)
}
