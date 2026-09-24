package events

import "fmt"

// MerchantIncomeEvent - merchant receives automatic income
type MerchantIncomeEvent struct {
	*BaseEvent
	MerchantID string
	Amount     int
}

func NewMerchantIncomeEvent(merchantID string, amount int) *MerchantIncomeEvent {
	e := &MerchantIncomeEvent{
		BaseEvent:  NewBaseEvent(EventMerchantIncome),
		MerchantID: merchantID,
		Amount:     amount,
	}
	e.Set("merchant_id", merchantID)
	e.Set("amount", amount)
	return e
}

func (e *MerchantIncomeEvent) String() string {
	return fmt.Sprintf("Merchant %s received %d gold income", e.MerchantID, e.Amount)
}

// PeasantTaxEvent - monarch taxes peasants
type PeasantTaxEvent struct {
	*BaseEvent
	CountryID string
	Amount    int
	HighTax   bool
}

func NewPeasantTaxEvent(countryID string, amount int, highTax bool) *PeasantTaxEvent {
	e := &PeasantTaxEvent{
		BaseEvent: NewBaseEvent(EventPeasantTax),
		CountryID: countryID,
		Amount:    amount,
		HighTax:   highTax,
	}
	e.Set("country_id", countryID)
	e.Set("amount", amount)
	e.Set("high_tax", highTax)
	return e
}

func (e *PeasantTaxEvent) String() string {
	taxType := "low"
	if e.HighTax {
		taxType = "high"
	}
	return fmt.Sprintf("Country %s collected %d gold from peasants (%s tax)", e.CountryID, e.Amount, taxType)
}

// PeasantRevoltEvent - peasants revolt due to high taxes
type PeasantRevoltEvent struct {
	*BaseEvent
	CountryID string
	Damage    int
}

func NewPeasantRevoltEvent(countryID string, damage int) *PeasantRevoltEvent {
	e := &PeasantRevoltEvent{
		BaseEvent: NewBaseEvent(EventPeasantRevolt),
		CountryID: countryID,
		Damage:    damage,
	}
	e.Set("country_id", countryID)
	e.Set("damage", damage)
	return e
}

func (e *PeasantRevoltEvent) String() string {
	return fmt.Sprintf("Peasants revolted in %s! Country took %d damage", e.CountryID, e.Damage)
}

// MerchantTaxEvent - monarch taxes merchants
type MerchantTaxEvent struct {
	*BaseEvent
	CountryID  string
	MerchantID string
	Amount     int
}

func NewMerchantTaxEvent(countryID, merchantID string, amount int) *MerchantTaxEvent {
	e := &MerchantTaxEvent{
		BaseEvent:  NewBaseEvent(EventMerchantTax),
		CountryID:  countryID,
		MerchantID: merchantID,
		Amount:     amount,
	}
	e.Set("country_id", countryID)
	e.Set("merchant_id", merchantID)
	e.Set("amount", amount)
	return e
}

func (e *MerchantTaxEvent) String() string {
	return fmt.Sprintf("Merchant %s paid %d gold tax to %s", e.MerchantID, e.Amount, e.CountryID)
}

// ArmyBuiltEvent - army strength increased
type ArmyBuiltEvent struct {
	*BaseEvent
	CountryID string
	Amount    int
	NewTotal  int
}

func NewArmyBuiltEvent(countryID string, amount, newTotal int) *ArmyBuiltEvent {
	e := &ArmyBuiltEvent{
		BaseEvent: NewBaseEvent(EventArmyBuilt),
		CountryID: countryID,
		Amount:    amount,
		NewTotal:  newTotal,
	}
	e.Set("country_id", countryID)
	e.Set("amount", amount)
	e.Set("new_total", newTotal)
	return e
}

func (e *ArmyBuiltEvent) String() string {
	return fmt.Sprintf("Country %s built %d army (total: %d)", e.CountryID, e.Amount, e.NewTotal)
}

// InvestmentPayoutEvent - merchant receives investment payout
type InvestmentPayoutEvent struct {
	*BaseEvent
	MerchantID string
	Amount     int
}

func NewInvestmentPayoutEvent(merchantID string, amount int) *InvestmentPayoutEvent {
	e := &InvestmentPayoutEvent{
		BaseEvent:  NewBaseEvent(EventInvestmentPayout),
		MerchantID: merchantID,
		Amount:     amount,
	}
	e.Set("merchant_id", merchantID)
	e.Set("amount", amount)
	return e
}

func (e *InvestmentPayoutEvent) String() string {
	return fmt.Sprintf("Merchant %s received %d gold from investments", e.MerchantID, e.Amount)
}

// BattleResolvedEvent - battle between countries resolved
type BattleResolvedEvent struct {
	*BaseEvent
	AttackerID       string
	DefenderID       string
	AttackerStrength int
	DefenderStrength int
	WinnerID         string
	DamageDealt      int
}

func NewBattleResolvedEvent(attackerID, defenderID string, attackerStr, defenderStr int, winnerID string, damage int) *BattleResolvedEvent {
	e := &BattleResolvedEvent{
		BaseEvent:        NewBaseEvent(EventBattleResolved),
		AttackerID:       attackerID,
		DefenderID:       defenderID,
		AttackerStrength: attackerStr,
		DefenderStrength: defenderStr,
		WinnerID:         winnerID,
		DamageDealt:      damage,
	}
	e.Set("attacker_id", attackerID)
	e.Set("defender_id", defenderID)
	e.Set("attacker_strength", attackerStr)
	e.Set("defender_strength", defenderStr)
	e.Set("winner_id", winnerID)
	e.Set("damage", damage)
	return e
}

func (e *BattleResolvedEvent) String() string {
	return fmt.Sprintf("Battle: %s (%d) vs %s (%d) - Winner: %s, Damage: %d",
		e.AttackerID, e.AttackerStrength, e.DefenderID, e.DefenderStrength, e.WinnerID, e.DamageDealt)
}

// AnnexationEvent - country annexed after defeat
type AnnexationEvent struct {
	*BaseEvent
	WinnerIDs      []string
	DefeatedID     string
	MerchantsTaken []string
}

func NewAnnexationEvent(winnerIDs []string, defeatedID string, merchants []string) *AnnexationEvent {
	e := &AnnexationEvent{
		BaseEvent:      NewBaseEvent(EventAnnexation),
		WinnerIDs:      winnerIDs,
		DefeatedID:     defeatedID,
		MerchantsTaken: merchants,
	}
	e.Set("winner_ids", winnerIDs)
	e.Set("defeated_id", defeatedID)
	e.Set("merchants", merchants)
	return e
}

func (e *AnnexationEvent) String() string {
	return fmt.Sprintf("Countries %v annexed %s, taking %d merchants", e.WinnerIDs, e.DefeatedID, len(e.MerchantsTaken))
}

// MerchantFledEvent - merchant fled to another country
type MerchantFledEvent struct {
	*BaseEvent
	MerchantID   string
	FromCountry  string
	ToCountry    string
	GoldTaken    int
	GoldLost     int
}

func NewMerchantFledEvent(merchantID, from, to string, goldTaken, goldLost int) *MerchantFledEvent {
	e := &MerchantFledEvent{
		BaseEvent:   NewBaseEvent(EventMerchantFled),
		MerchantID:  merchantID,
		FromCountry: from,
		ToCountry:   to,
		GoldTaken:   goldTaken,
		GoldLost:    goldLost,
	}
	e.Set("merchant_id", merchantID)
	e.Set("from_country", from)
	e.Set("to_country", to)
	e.Set("gold_taken", goldTaken)
	e.Set("gold_lost", goldLost)
	return e
}

func (e *MerchantFledEvent) String() string {
	return fmt.Sprintf("Merchant %s fled from %s to %s (took %d gold, lost %d invested)",
		e.MerchantID, e.FromCountry, e.ToCountry, e.GoldTaken, e.GoldLost)
}

// RevoltSuccessEvent - merchants successfully overthrew the monarch
type RevoltSuccessEvent struct {
	*BaseEvent
	CountryID    string
	Participants []string
	TotalGold    int
	DefenseGold  int
}

func NewRevoltSuccessEvent(countryID string, participants []string, totalGold, defenseGold int) *RevoltSuccessEvent {
	e := &RevoltSuccessEvent{
		BaseEvent:    NewBaseEvent(EventRevoltSuccess),
		CountryID:    countryID,
		Participants: participants,
		TotalGold:    totalGold,
		DefenseGold:  defenseGold,
	}
	e.Set("country_id", countryID)
	e.Set("participants", participants)
	e.Set("total_gold", totalGold)
	e.Set("defense_gold", defenseGold)
	return e
}

func (e *RevoltSuccessEvent) String() string {
	return fmt.Sprintf("Successful revolt in %s! %d merchants overthrew the monarch with %d gold against %d defending gold",
		e.CountryID, len(e.Participants), e.TotalGold, e.DefenseGold)
}

// RevoltFailedEvent - merchant revolt failed
type RevoltFailedEvent struct {
	*BaseEvent
	CountryID    string
	Participants []string
	GoldLost     int
	TotalGold    int
	DefenseGold  int
}

func NewRevoltFailedEvent(countryID string, participants []string, goldLost, totalGold, defenseGold int) *RevoltFailedEvent {
	e := &RevoltFailedEvent{
		BaseEvent:    NewBaseEvent(EventRevoltFailed),
		CountryID:    countryID,
		Participants: participants,
		GoldLost:     goldLost,
		TotalGold:    totalGold,
		DefenseGold:  defenseGold,
	}
	e.Set("country_id", countryID)
	e.Set("participants", participants)
	e.Set("gold_lost", goldLost)
	e.Set("total_gold", totalGold)
	e.Set("defense_gold", defenseGold)
	return e
}

func (e *RevoltFailedEvent) String() string {
	return fmt.Sprintf("Failed revolt in %s! %d merchants raised %d gold against %d defending gold and lost %d gold to the monarch",
		e.CountryID, len(e.Participants), e.TotalGold, e.DefenseGold, e.GoldLost)
}

// Reasons a monarch lost their throne, used by MonarchDeposedEvent
const (
	DeposedByConquest   = "conquest"
	DeposedByRevolution = "revolution"
)

// MonarchDeposedEvent - a monarch lost their throne and became a merchant elsewhere
type MonarchDeposedEvent struct {
	*BaseEvent
	MonarchID   string
	FromCountry string
	ToCountry   string
	GoldKept    int
	Reason      string
}

func NewMonarchDeposedEvent(monarchID, fromCountry, toCountry string, goldKept int, reason string) *MonarchDeposedEvent {
	e := &MonarchDeposedEvent{
		BaseEvent:   NewBaseEvent(EventMonarchDeposed),
		MonarchID:   monarchID,
		FromCountry: fromCountry,
		ToCountry:   toCountry,
		GoldKept:    goldKept,
		Reason:      reason,
	}
	e.Set("monarch_id", monarchID)
	e.Set("from_country", fromCountry)
	e.Set("to_country", toCountry)
	e.Set("gold_kept", goldKept)
	e.Set("reason", reason)
	return e
}

func (e *MonarchDeposedEvent) String() string {
	cause := "was overthrown"
	if e.Reason == DeposedByConquest {
		cause = "was conquered"
	}
	if e.ToCountry == "" {
		return fmt.Sprintf("Monarch %s of %s %s and left the game with %d gold",
			e.MonarchID, e.FromCountry, cause, e.GoldKept)
	}
	return fmt.Sprintf("Monarch %s of %s %s and became a merchant in %s with %d gold",
		e.MonarchID, e.FromCountry, cause, e.ToCountry, e.GoldKept)
}

// TreasurySplitEvent - a deposed monarch's remaining treasury was shared out
type TreasurySplitEvent struct {
	*BaseEvent
	CountryID  string
	Recipients []string
	TotalGold  int
}

func NewTreasurySplitEvent(countryID string, recipients []string, totalGold int) *TreasurySplitEvent {
	e := &TreasurySplitEvent{
		BaseEvent:  NewBaseEvent(EventTreasurySplit),
		CountryID:  countryID,
		Recipients: recipients,
		TotalGold:  totalGold,
	}
	e.Set("country_id", countryID)
	e.Set("recipients", recipients)
	e.Set("total_gold", totalGold)
	return e
}

func (e *TreasurySplitEvent) String() string {
	return fmt.Sprintf("The treasury of %s (%d gold) was split among %d revolting merchants",
		e.CountryID, e.TotalGold, len(e.Recipients))
}

// ArmyMaintenanceEvent - army halved due to maintenance
type ArmyMaintenanceEvent struct {
	*BaseEvent
	CountryID   string
	OldStrength int
	NewStrength int
}

func NewArmyMaintenanceEvent(countryID string, oldStr, newStr int) *ArmyMaintenanceEvent {
	e := &ArmyMaintenanceEvent{
		BaseEvent:   NewBaseEvent(EventArmyMaintenance),
		CountryID:   countryID,
		OldStrength: oldStr,
		NewStrength: newStr,
	}
	e.Set("country_id", countryID)
	e.Set("old_strength", oldStr)
	e.Set("new_strength", newStr)
	return e
}

func (e *ArmyMaintenanceEvent) String() string {
	return fmt.Sprintf("Army maintenance in %s: %d -> %d", e.CountryID, e.OldStrength, e.NewStrength)
}
