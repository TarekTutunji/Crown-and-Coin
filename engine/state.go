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
	Settings  Settings             `json:"settings"`
}

// Settings are the options the game leader can change during the game
type Settings struct {
	// What an investment pays back at the start of the next round, as a
	// percentage of the gold put in (200 = double, 150 = one and a half
	// times). Fractions of a gold coin are rounded down.
	InvestmentReturnPercent int `json:"investment_return_percent"`
	// Open game: every player sees every move the way the game leader does,
	// instead of only what the secrecy rules allow
	OpenGame bool `json:"open_game"`
}

// DefaultSettings are the settings a new game starts with
func DefaultSettings() Settings {
	return Settings{InvestmentReturnPercent: 200}
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
		Settings:  DefaultSettings(),
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

// GetMerchantsByCountry returns the merchants present in a country, sorted by
// ID. Merchants still on their way there (see Merchant.Arriving) are left
// out: they only join at the start of the next round.
func (gs *GameState) GetMerchantsByCountry(countryID string) []*Merchant {
	var merchants []*Merchant
	for _, m := range gs.GetMerchantsHeadingTo(countryID) {
		if !m.Arriving {
			merchants = append(merchants, m)
		}
	}
	return merchants
}

// GetMerchantsHeadingTo returns every merchant of a country, including those
// still on their way there, sorted by ID
func (gs *GameState) GetMerchantsHeadingTo(countryID string) []*Merchant {
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

// ArriveMovers lets every merchant who moved last round join their new
// country, and returns their IDs
func (gs *GameState) ArriveMovers() []string {
	var arrived []string
	for _, m := range gs.Merchants {
		if m.Arriving {
			m.Arriving = false
			arrived = append(arrived, m.ID)
		}
	}
	sort.Strings(arrived)
	return arrived
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

// FallenMonarchPurse is the gold a monarch who loses the throne (by conquest,
// revolt or collapse) starts over with as a merchant. The bank creates it;
// the monarch never keeps any of the treasury.
const FallenMonarchPurse = 5

// ResettleMonarchAsMerchant turns a fallen monarch into a merchant of
// destCountryID with FallenMonarchPurse gold in their purse. Like anyone who
// moves, they arrive at the start of the next round.
func (gs *GameState) ResettleMonarchAsMerchant(monarchID, destCountryID string) *Merchant {
	merchant := NewMerchant(monarchID, destCountryID)
	merchant.StoredGold = FallenMonarchPurse
	merchant.Arriving = true
	gs.AddMerchant(merchant)
	return merchant
}

// ScatterMerchants moves every merchant of fromCountryID, including any still
// on their way there, to the countries in toCountryIDs. With several
// destinations the dice shuffle the merchants, who are then dealt out
// round-robin in the order the destinations are given, so the first
// destinations get any extra merchants. With forfeitInvestments their
// investments are destroyed. The merchants arrive at the start of the next
// round. It returns the moved merchants' IDs and the invested gold destroyed.
func (gs *GameState) ScatterMerchants(fromCountryID string, toCountryIDs []string, forfeitInvestments bool, roller DiceRoller) ([]string, int) {
	if len(toCountryIDs) == 0 {
		return nil, 0
	}
	destinations := toCountryIDs

	merchantIDs := make([]string, 0)
	for _, m := range gs.GetMerchantsHeadingTo(fromCountryID) {
		merchantIDs = append(merchantIDs, m.ID)
	}
	if len(destinations) > 1 {
		merchantIDs = ShuffleIDs(merchantIDs, roller)
	}

	forfeited := 0
	for i, id := range merchantIDs {
		m := gs.GetMerchant(id)
		if forfeitInvestments {
			forfeited += m.InvestedGold
			m.InvestedGold = 0
		}
		m.MoveTo(destinations[i%len(destinations)])
	}
	return merchantIDs, forfeited
}

// SplitEvenly divides amount into n shares as evenly as possible. The first
// shares get the leftovers, one each.
func SplitEvenly(amount, n int) []int {
	shares := make([]int, n)
	for i := range shares {
		shares[i] = amount / n
		if i < amount%n {
			shares[i]++
		}
	}
	return shares
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
		Settings:  gs.Settings,
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
