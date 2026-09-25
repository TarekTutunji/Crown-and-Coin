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

// PeasantTaxEvent - a country taxed its peasants. Revolted is true when a
// high tax failed because the peasants rose up, and nothing was collected.
type PeasantTaxEvent struct {
	*BaseEvent
	CountryID string
	Amount    int
	HighTax   bool
	Revolted  bool
}

func NewPeasantTaxEvent(countryID string, amount int, highTax, revolted bool) *PeasantTaxEvent {
	e := &PeasantTaxEvent{
		BaseEvent: NewBaseEvent(EventPeasantTax),
		CountryID: countryID,
		Amount:    amount,
		HighTax:   highTax,
		Revolted:  revolted,
	}
	e.Set("country_id", countryID)
	e.Set("amount", amount)
	e.Set("high_tax", highTax)
	e.Set("revolted", revolted)
	return e
}

func (e *PeasantTaxEvent) String() string {
	if e.Revolted {
		return fmt.Sprintf("The high tax in %s failed: the peasants revolted and paid nothing", e.CountryID)
	}
	taxType := "low"
	if e.HighTax {
		taxType = "high"
	}
	return fmt.Sprintf("The %s tax in %s succeeded: %d gold collected from the peasants", taxType, e.CountryID, e.Amount)
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
	Treasury       int
}

func NewAnnexationEvent(winnerIDs []string, defeatedID string, merchants []string, treasury int) *AnnexationEvent {
	e := &AnnexationEvent{
		BaseEvent:      NewBaseEvent(EventAnnexation),
		WinnerIDs:      winnerIDs,
		DefeatedID:     defeatedID,
		MerchantsTaken: merchants,
		Treasury:       treasury,
	}
	e.Set("winner_ids", winnerIDs)
	e.Set("defeated_id", defeatedID)
	e.Set("merchants", merchants)
	e.Set("treasury", treasury)
	return e
}

func (e *AnnexationEvent) String() string {
	return fmt.Sprintf("Countries %v annexed %s, taking %d merchants and %d gold of treasury",
		e.WinnerIDs, e.DefeatedID, len(e.MerchantsTaken), e.Treasury)
}

// MerchantFledEvent - merchant fled to another country
type MerchantFledEvent struct {
	*BaseEvent
	MerchantID  string
	FromCountry string
	ToCountry   string
	GoldTaken   int
	GoldLost    int
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

func NewRevoltFailedEvent(countryID string, participants []string, goldLost, investmentsDestroyed, totalGold, defenseGold int) *RevoltFailedEvent {
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
	e.Set("investments_destroyed", investmentsDestroyed)
	e.Set("total_gold", totalGold)
	e.Set("defense_gold", defenseGold)
	return e
}

func (e *RevoltFailedEvent) String() string {
	return fmt.Sprintf("Failed revolt in %s! %d merchants raised %d gold against %d defending gold and lost %d gold to the monarch; their investments were destroyed",
		e.CountryID, len(e.Participants), e.TotalGold, e.DefenseGold, e.GoldLost)
}

// Reasons a monarch lost their throne, used by MonarchDeposedEvent
const (
	DeposedByConquest   = "conquest"
	DeposedByRevolution = "revolution"
	DeposedByPeasants   = "peasant revolt"
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
	switch e.Reason {
	case DeposedByConquest:
		cause = "was conquered"
	case DeposedByPeasants:
		cause = "lost their country to a peasant revolt"
	}
	if e.ToCountry == "" {
		return fmt.Sprintf("Monarch %s of %s %s and left the game with %d gold",
			e.MonarchID, e.FromCountry, cause, e.GoldKept)
	}
	return fmt.Sprintf("Monarch %s of %s %s and became a merchant in %s with %d gold",
		e.MonarchID, e.FromCountry, cause, e.ToCountry, e.GoldKept)
}

// ExileRelocatedEvent - a monarch deposed this phase could not settle where
// MonarchDeposedEvent sent them, because that country fell in the same
// phase, and became a merchant in ToCountry instead
type ExileRelocatedEvent struct {
	*BaseEvent
	MonarchID   string
	FromCountry string // The country they were deposed from
	Intended    string // Where MonarchDeposedEvent first sent them
	ToCountry   string // Where they actually ended up
}

func NewExileRelocatedEvent(monarchID, fromCountry, intended, toCountry string) *ExileRelocatedEvent {
	e := &ExileRelocatedEvent{
		BaseEvent:   NewBaseEvent(EventExileRelocated),
		MonarchID:   monarchID,
		FromCountry: fromCountry,
		Intended:    intended,
		ToCountry:   toCountry,
	}
	e.Set("monarch_id", monarchID)
	e.Set("from_country", fromCountry)
	e.Set("intended", intended)
	e.Set("to_country", toCountry)
	return e
}

func (e *ExileRelocatedEvent) String() string {
	return fmt.Sprintf("Former monarch %s of %s could not settle in %s, which fell the same phase, and became a merchant in %s instead",
		e.MonarchID, e.FromCountry, e.Intended, e.ToCountry)
}

// TreasurySplitEvent - the treasury of an overthrown monarch was shared out
// among the rebels
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

// RepublicTaxVoteEvent - the merchants of a republic voted on the peasant
// tax. Abstained counts the merchants who did not vote, which count as votes
// for low tax.
type RepublicTaxVoteEvent struct {
	*BaseEvent
	CountryID string
	HighVotes int
	LowVotes  int
	Abstained int
	HighTax   bool
}

func NewRepublicTaxVoteEvent(countryID string, highVotes, lowVotes, abstained int, highTax bool) *RepublicTaxVoteEvent {
	e := &RepublicTaxVoteEvent{
		BaseEvent: NewBaseEvent(EventRepublicTaxVote),
		CountryID: countryID,
		HighVotes: highVotes,
		LowVotes:  lowVotes,
		Abstained: abstained,
		HighTax:   highTax,
	}
	e.Set("country_id", countryID)
	e.Set("high_votes", highVotes)
	e.Set("low_votes", lowVotes)
	e.Set("abstained", abstained)
	e.Set("high_tax", highTax)
	return e
}

func (e *RepublicTaxVoteEvent) String() string {
	result := "low"
	if e.HighTax {
		result = "high"
	}
	if e.Abstained > 0 {
		return fmt.Sprintf("The merchants of %s voted %d high / %d low, and %d did not vote (counted as low): %s tax",
			e.CountryID, e.HighVotes, e.LowVotes, e.Abstained, result)
	}
	return fmt.Sprintf("The merchants of %s voted %d high / %d low: %s tax",
		e.CountryID, e.HighVotes, e.LowVotes, result)
}

// Sources of gold a republic shares among its merchants, used by RepublicGoldSharedEvent
const (
	GoldFromPeasantTax = "peasant tax"
	GoldFromVictory    = "victory"
	GoldFromConquest   = "conquest"
)

// RepublicGoldSharedEvent - gold earned by a republic was shared among its merchants
type RepublicGoldSharedEvent struct {
	*BaseEvent
	CountryID  string
	Recipients []string
	TotalGold  int
	Source     string
}

func NewRepublicGoldSharedEvent(countryID string, recipients []string, totalGold int, source string) *RepublicGoldSharedEvent {
	e := &RepublicGoldSharedEvent{
		BaseEvent:  NewBaseEvent(EventRepublicGoldShared),
		CountryID:  countryID,
		Recipients: recipients,
		TotalGold:  totalGold,
		Source:     source,
	}
	e.Set("country_id", countryID)
	e.Set("recipients", recipients)
	e.Set("total_gold", totalGold)
	e.Set("source", source)
	return e
}

func (e *RepublicGoldSharedEvent) String() string {
	return fmt.Sprintf("The %s gold of %s (%d gold) was shared among %d merchants",
		e.Source, e.CountryID, e.TotalGold, len(e.Recipients))
}

// ArmyContributedEvent - a republic merchant paid into the communal army
type ArmyContributedEvent struct {
	*BaseEvent
	CountryID  string
	MerchantID string
	Amount     int
	NewTotal   int
}

func NewArmyContributedEvent(countryID, merchantID string, amount, newTotal int) *ArmyContributedEvent {
	e := &ArmyContributedEvent{
		BaseEvent:  NewBaseEvent(EventArmyContributed),
		CountryID:  countryID,
		MerchantID: merchantID,
		Amount:     amount,
		NewTotal:   newTotal,
	}
	e.Set("country_id", countryID)
	e.Set("merchant_id", merchantID)
	e.Set("amount", amount)
	e.Set("new_total", newTotal)
	return e
}

func (e *ArmyContributedEvent) String() string {
	return fmt.Sprintf("Merchant %s contributed %d gold to the army of %s (total: %d)",
		e.MerchantID, e.Amount, e.CountryID, e.NewTotal)
}

// RepublicWarVoteEvent - the merchants of a republic voted on whom to attack.
// TargetID is empty when no target reached a strict majority.
type RepublicWarVoteEvent struct {
	*BaseEvent
	CountryID string
	Votes     map[string]int
	Merchants int
	TargetID  string
}

func NewRepublicWarVoteEvent(countryID string, votes map[string]int, merchants int, targetID string) *RepublicWarVoteEvent {
	e := &RepublicWarVoteEvent{
		BaseEvent: NewBaseEvent(EventRepublicWarVote),
		CountryID: countryID,
		Votes:     votes,
		Merchants: merchants,
		TargetID:  targetID,
	}
	e.Set("country_id", countryID)
	e.Set("votes", votes)
	e.Set("merchants", merchants)
	e.Set("target_id", targetID)
	return e
}

func (e *RepublicWarVoteEvent) String() string {
	if e.TargetID == "" {
		return fmt.Sprintf("The merchants of %s reached no majority, so there is no attack", e.CountryID)
	}
	return fmt.Sprintf("The merchants of %s voted to attack %s (%d of %d votes)",
		e.CountryID, e.TargetID, e.Votes[e.TargetID], e.Merchants)
}

// RepublicFallenEvent - a republic was eliminated and its merchants lost their investments
type RepublicFallenEvent struct {
	*BaseEvent
	CountryID     string
	ForfeitedGold int
}

func NewRepublicFallenEvent(countryID string, forfeitedGold int) *RepublicFallenEvent {
	e := &RepublicFallenEvent{
		BaseEvent:     NewBaseEvent(EventRepublicFallen),
		CountryID:     countryID,
		ForfeitedGold: forfeitedGold,
	}
	e.Set("country_id", countryID)
	e.Set("forfeited_gold", forfeitedGold)
	return e
}

func (e *RepublicFallenEvent) String() string {
	return fmt.Sprintf("The republic of %s has fallen; its merchants forfeited %d invested gold",
		e.CountryID, e.ForfeitedGold)
}

// CountryCollapsedEvent - a country was destroyed from within by a peasant or
// merchant revolt and its merchants scattered to the surviving countries,
// losing their investments
type CountryCollapsedEvent struct {
	*BaseEvent
	CountryID     string
	Merchants     []string
	ForfeitedGold int
	TreasuryLost  int
	Reason        string
}

func NewCountryCollapsedEvent(countryID string, merchants []string, forfeitedGold, treasuryLost int, reason string) *CountryCollapsedEvent {
	e := &CountryCollapsedEvent{
		BaseEvent:     NewBaseEvent(EventCountryCollapsed),
		CountryID:     countryID,
		Merchants:     merchants,
		ForfeitedGold: forfeitedGold,
		TreasuryLost:  treasuryLost,
		Reason:        reason,
	}
	e.Set("country_id", countryID)
	e.Set("merchants", merchants)
	e.Set("forfeited_gold", forfeitedGold)
	e.Set("treasury_lost", treasuryLost)
	e.Set("reason", reason)
	return e
}

func (e *CountryCollapsedEvent) String() string {
	cause := "a merchant revolt"
	if e.Reason == DeposedByPeasants {
		cause = "a peasant revolt"
	}
	return fmt.Sprintf("%s collapsed in %s; its treasury of %d gold was lost and %d merchants fled elsewhere, forfeiting %d invested gold",
		e.CountryID, cause, e.TreasuryLost, len(e.Merchants), e.ForfeitedGold)
}

// RepublicAbandonedEvent - every merchant left a republic, so it died
type RepublicAbandonedEvent struct {
	*BaseEvent
	CountryID string
}

func NewRepublicAbandonedEvent(countryID string) *RepublicAbandonedEvent {
	e := &RepublicAbandonedEvent{
		BaseEvent: NewBaseEvent(EventRepublicAbandoned),
		CountryID: countryID,
	}
	e.Set("country_id", countryID)
	return e
}

func (e *RepublicAbandonedEvent) String() string {
	return fmt.Sprintf("Every merchant has left %s, and the republic is no more", e.CountryID)
}

// MerchantArrivedEvent - a merchant who moved last round joined their new
// country at the start of this round
type MerchantArrivedEvent struct {
	*BaseEvent
	MerchantID string
	CountryID  string
}

func NewMerchantArrivedEvent(merchantID, countryID string) *MerchantArrivedEvent {
	e := &MerchantArrivedEvent{
		BaseEvent:  NewBaseEvent(EventMerchantArrived),
		MerchantID: merchantID,
		CountryID:  countryID,
	}
	e.Set("merchant_id", merchantID)
	e.Set("country_id", countryID)
	return e
}

func (e *MerchantArrivedEvent) String() string {
	return fmt.Sprintf("Merchant %s arrived in %s", e.MerchantID, e.CountryID)
}
