package engine

// Merchant represents a merchant player in the game
type Merchant struct {
	ID           string `json:"player_id"`
	CountryID    string `json:"country_id"`    // Which country this merchant belongs to
	StoredGold   int    `json:"stored_gold"`   // Purse: income and payouts, the monarch can see and tax it
	HiddenGold   int    `json:"hidden_gold"`   // Hidden savings, safe from tax and secret from the monarch
	InvestedGold int    `json:"invested_gold"` // Gold invested (doubles next turn, lost if fleeing)
}

// NewMerchant creates a new merchant with default values
func NewMerchant(id string, countryID string) *Merchant {
	return &Merchant{
		ID:           id,
		CountryID:    countryID,
		StoredGold:   5,
		InvestedGold: 0,
	}
}

// SpendableGold returns the gold a merchant can spend: purse and hidden savings
func (m *Merchant) SpendableGold() int {
	return m.StoredGold + m.HiddenGold
}

// TotalGold returns all of a merchant's gold: purse, hidden and invested
func (m *Merchant) TotalGold() int {
	return m.StoredGold + m.HiddenGold + m.InvestedGold
}

// ReceiveIncome adds gold to stored gold (automatic 5 gold per turn)
func (m *Merchant) ReceiveIncome(amount int) {
	m.StoredGold += amount
}

// PayTax removes gold from the purse, returns actual amount paid. Hidden
// gold is never taxed.
func (m *Merchant) PayTax(amount int) int {
	if m.StoredGold >= amount {
		m.StoredGold -= amount
		return amount
	}
	paid := m.StoredGold
	m.StoredGold = 0
	return paid
}

// Spend takes gold from the purse first and then from hidden savings,
// returns false if the merchant does not have enough
func (m *Merchant) Spend(amount int) bool {
	if m.SpendableGold() < amount {
		return false
	}
	fromPurse := min(amount, m.StoredGold)
	m.StoredGold -= fromPurse
	m.HiddenGold -= amount - fromPurse
	return true
}

// Invest moves gold (purse first, then hidden) into investments
func (m *Merchant) Invest(amount int) bool {
	if !m.Spend(amount) {
		return false
	}
	m.InvestedGold += amount
	return true
}

// CollectInvestment doubles invested gold and moves it to the purse, where
// it can be taxed until the merchant hides it
func (m *Merchant) CollectInvestment() int {
	payout := m.InvestedGold * 2
	m.StoredGold += payout
	m.InvestedGold = 0
	return payout
}

// Hide moves gold from the purse into hidden savings, returns false if the
// purse does not hold enough
func (m *Merchant) Hide(amount int) bool {
	if m.StoredGold < amount {
		return false
	}
	m.StoredGold -= amount
	m.HiddenGold += amount
	return true
}

// FleeToCountry moves merchant to a new country, losing invested gold
func (m *Merchant) FleeToCountry(newCountryID string) {
	m.CountryID = newCountryID
	m.InvestedGold = 0 // Lose investments when fleeing
}

// LoseAllGold transfers all gold away (used in failed revolt)
func (m *Merchant) LoseAllGold() int {
	total := m.TotalGold()
	m.StoredGold = 0
	m.HiddenGold = 0
	m.InvestedGold = 0
	return total
}

// Clone creates a deep copy of the merchant
func (m *Merchant) Clone() *Merchant {
	return &Merchant{
		ID:           m.ID,
		CountryID:    m.CountryID,
		StoredGold:   m.StoredGold,
		HiddenGold:   m.HiddenGold,
		InvestedGold: m.InvestedGold,
	}
}
