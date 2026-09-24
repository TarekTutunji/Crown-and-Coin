package engine

// Country represents a nation in the game
type Country struct {
	ID           string `json:"country_id"`
	HP           int    `json:"hp"`             // Health points, starts at 10
	ArmyStrength int    `json:"army_strength"`  // Military power, starts at 0
	Gold         int    `json:"gold"`           // Treasury
	Peasants     int    `json:"peasants"`       // Tax base, starts at 5
	RevoltRisk   int    `json:"revolt_risk"`    // N in N/6 chance of revolt on high tax (2-5)
	IsRepublic   bool   `json:"is_republic"`    // false = monarchy, true = merchant republic
	MonarchID    string `json:"monarch_id"`     // Player controlling the country (if monarchy)
	DiedOnce     bool   `json:"died_once"`      // Tracks if country already used its "revival"
	PublicArmy   int    `json:"public_army"`    // Army strength other players know about, updated after each war
}

// NewCountry creates a new country with default starting values
func NewCountry(id string, monarchID string) *Country {
	return &Country{
		ID:           id,
		HP:           10,
		ArmyStrength: 0,
		Gold:         10,
		Peasants:     5,
		RevoltRisk:   2,
		IsRepublic:   false,
		MonarchID:    monarchID,
		DiedOnce:     false,
	}
}

// IsAlive returns true if the country still has HP
func (c *Country) IsAlive() bool {
	return c.HP > 0
}

// TakeDamage reduces HP by the given amount, handling the first-death revival
func (c *Country) TakeDamage(damage int) {
	c.HP -= damage
	if c.HP <= 0 && !c.DiedOnce {
		c.HP = 1
		c.DiedOnce = true
	}
}

// Eliminate removes the country from the game for good, skipping any revival
func (c *Country) Eliminate() {
	c.HP = 0
	c.DiedOnce = true
}

// AddArmy increases army strength
func (c *Country) AddArmy(amount int) {
	c.ArmyStrength += amount
}

// HalveArmy reduces army strength by half (maintenance cost)
func (c *Country) HalveArmy() {
	c.ArmyStrength = c.ArmyStrength / 2
}

// AddGold adds gold to the treasury
func (c *Country) AddGold(amount int) {
	c.Gold += amount
}

// SpendGold removes gold from the treasury, returns false if insufficient
func (c *Country) SpendGold(amount int) bool {
	if c.Gold < amount {
		return false
	}
	c.Gold -= amount
	return true
}

// AddPeasant increases the peasant count
func (c *Country) AddPeasant() {
	c.Peasants++
}

// BecomeRepublic converts the country to a merchant republic
func (c *Country) BecomeRepublic() {
	c.IsRepublic = true
	c.MonarchID = ""
}

// RemoveMonarch clears the monarch without changing the form of government
// (used when the country is conquered outright rather than overthrown)
func (c *Country) RemoveMonarch() {
	c.MonarchID = ""
}

// EmptyTreasury takes all gold out of the treasury and returns it
func (c *Country) EmptyTreasury() int {
	gold := c.Gold
	c.Gold = 0
	return gold
}

// Clone creates a deep copy of the country
func (c *Country) Clone() *Country {
	return &Country{
		ID:           c.ID,
		HP:           c.HP,
		ArmyStrength: c.ArmyStrength,
		Gold:         c.Gold,
		Peasants:     c.Peasants,
		RevoltRisk:   c.RevoltRisk,
		IsRepublic:   c.IsRepublic,
		MonarchID:    c.MonarchID,
		DiedOnce:     c.DiedOnce,
		PublicArmy:   c.PublicArmy,
	}
}
