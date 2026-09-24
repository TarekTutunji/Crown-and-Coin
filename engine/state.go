package engine

import (
	"encoding/json"
	"fmt"
	"sort"
)

// GameState represents the complete state of the game at any point
type GameState struct {
	Turn      int                  `json:"turn"`
	Phase     PhaseType            `json:"phase"`
	Countries map[string]*Country  `json:"countries"`
	Merchants map[string]*Merchant `json:"merchants"`
}

// PhaseType represents the different phases of a game turn
type PhaseType int

const (
	PhaseTaxation PhaseType = iota + 1
	PhaseNegotiation
	PhaseSpending
	PhaseWar
	PhaseAssessment
)

// String returns the name of the phase
func (p PhaseType) String() string {
	switch p {
	case PhaseTaxation:
		return "taxation"
	case PhaseNegotiation:
		return "negotiation"
	case PhaseSpending:
		return "spending"
	case PhaseWar:
		return "war"
	case PhaseAssessment:
		return "assessment"
	default:
		return "unknown"
	}
}

// MarshalJSON serializes PhaseType as a JSON string
func (p PhaseType) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// UnmarshalJSON deserializes PhaseType from a JSON string
func (p *PhaseType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "taxation":
		*p = PhaseTaxation
	case "negotiation":
		*p = PhaseNegotiation
	case "spending":
		*p = PhaseSpending
	case "war":
		*p = PhaseWar
	case "assessment":
		*p = PhaseAssessment
	default:
		return fmt.Errorf("unknown phase: %s", s)
	}
	return nil
}

// NewGameState creates a new game state
func NewGameState() *GameState {
	return &GameState{
		Turn:      1,
		Phase:     PhaseTaxation,
		Countries: make(map[string]*Country),
		Merchants: make(map[string]*Merchant),
	}
}

// AddCountry adds a country to the game
func (gs *GameState) AddCountry(country *Country) {
	gs.Countries[country.ID] = country
}

// AddMerchant adds a merchant to the game
func (gs *GameState) AddMerchant(merchant *Merchant) {
	gs.Merchants[merchant.ID] = merchant
}

// RemoveMerchant takes a merchant out of the game, along with their gold
func (gs *GameState) RemoveMerchant(id string) {
	delete(gs.Merchants, id)
}

// PlayerCountryID returns the country a player belongs to, as its monarch or
// as one of its merchants, or "" if the player has no role
func (gs *GameState) PlayerCountryID(playerID string) string {
	if merchant := gs.GetMerchant(playerID); merchant != nil {
		return merchant.CountryID
	}
	for id, c := range gs.Countries {
		if c.MonarchID == playerID {
			return id
		}
	}
	return ""
}

// PublishArmies makes every country's current army strength known to the
// other players
func (gs *GameState) PublishArmies() {
	for _, c := range gs.Countries {
		c.PublicArmy = c.ArmyStrength
	}
}

// GetCountry returns a country by ID
func (gs *GameState) GetCountry(id string) *Country {
	return gs.Countries[id]
}

// GetMerchant returns a merchant by ID
func (gs *GameState) GetMerchant(id string) *Merchant {
	return gs.Merchants[id]
}

// GetMerchantsByCountry returns all merchants belonging to a country, sorted by ID
func (gs *GameState) GetMerchantsByCountry(countryID string) []*Merchant {
	var merchants []*Merchant
	for _, m := range gs.Merchants {
		if m.CountryID == countryID {
			merchants = append(merchants, m)
		}
	}
	sort.Slice(merchants, func(i, j int) bool {
		return merchants[i].ID < merchants[j].ID
	})
	return merchants
}

// GetAliveCountries returns all countries that are still in the game
func (gs *GameState) GetAliveCountries() []*Country {
	var alive []*Country
	for _, c := range gs.Countries {
		if c.IsAlive() {
			alive = append(alive, c)
		}
	}
	return alive
}

// GetAliveCountryIDs returns the IDs of all countries still in the game
func (gs *GameState) GetAliveCountryIDs() []string {
	return gs.GetAliveCountryIDsExcept("")
}

// GetAliveCountryIDsExcept returns the IDs of all countries still in the game
// apart from excludeID
func (gs *GameState) GetAliveCountryIDsExcept(excludeID string) []string {
	ids := make([]string, 0, len(gs.Countries))
	for id, c := range gs.Countries {
		if c.IsAlive() && id != excludeID {
			ids = append(ids, id)
		}
	}
	return ids
}

// ResettleMonarchAsMerchant seats a deposed monarch as a merchant in
// destCountryID, with gold as their starting personal savings
func (gs *GameState) ResettleMonarchAsMerchant(monarchID, destCountryID string, gold int) *Merchant {
	merchant := NewMerchant(monarchID, destCountryID)
	merchant.StoredGold = gold
	gs.AddMerchant(merchant)
	return merchant
}

// ScatterMerchants moves every merchant of fromCountryID to the countries in
// toCountryIDs, dealt out round-robin in the order given, so the first
// destinations get any extra merchants. With forfeitInvestments
// the merchants arrive with only their hidden savings. It returns the moved
// merchants' IDs and the invested gold they lost.
func (gs *GameState) ScatterMerchants(fromCountryID string, toCountryIDs []string, forfeitInvestments bool) ([]string, int) {
	if len(toCountryIDs) == 0 {
		return nil, 0
	}
	destinations := toCountryIDs

	merchants := gs.GetMerchantsByCountry(fromCountryID)
	merchantIDs := make([]string, 0, len(merchants))
	forfeited := 0
	for i, m := range merchants {
		if forfeitInvestments {
			forfeited += m.InvestedGold
			m.InvestedGold = 0
		}
		m.CountryID = destinations[i%len(destinations)]
		merchantIDs = append(merchantIDs, m.ID)
	}
	return merchantIDs, forfeited
}

// PickRandomID chooses one ID at random. The candidates are sorted first so
// the outcome depends only on the dice roll, never on map iteration order.
func PickRandomID(ids []string, roller DiceRoller) string {
	if len(ids) == 0 {
		return ""
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	return sorted[roller.Roll(len(sorted))-1]
}

// ShuffleIDs returns the IDs in a random order decided by the dice. The
// candidates are sorted first so the outcome depends only on the dice rolls,
// never on the order they were passed in.
func ShuffleIDs(ids []string, roller DiceRoller) []string {
	shuffled := append([]string(nil), ids...)
	sort.Strings(shuffled)
	for i := len(shuffled) - 1; i > 0; i-- {
		j := roller.Roll(i+1) - 1
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	return shuffled
}

// Clone creates a deep copy of the game state
func (gs *GameState) Clone() *GameState {
	newState := &GameState{
		Turn:      gs.Turn,
		Phase:     gs.Phase,
		Countries: make(map[string]*Country),
		Merchants: make(map[string]*Merchant),
	}
	for id, c := range gs.Countries {
		newState.Countries[id] = c.Clone()
	}
	for id, m := range gs.Merchants {
		newState.Merchants[id] = m.Clone()
	}
	return newState
}

// NextPhase advances to the next phase, or next turn if at the end
func (gs *GameState) NextPhase() {
	if gs.Phase == PhaseAssessment {
		gs.Turn++
		gs.Phase = PhaseTaxation
	} else {
		gs.Phase++
	}
}
