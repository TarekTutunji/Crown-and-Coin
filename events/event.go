package events

// EventType identifies the type of event
type EventType string

const (
	EventMerchantIncome    EventType = "merchant_income"
	EventPeasantTax        EventType = "peasant_tax"
	EventPeasantRevolt     EventType = "peasant_revolt"
	EventMerchantTax       EventType = "merchant_tax"
	EventArmyBuilt         EventType = "army_built"
	EventInvestmentMade    EventType = "investment_made"
	EventInvestmentPayout  EventType = "investment_payout"
	EventBattleStarted     EventType = "battle_started"
	EventBattleResolved    EventType = "battle_resolved"
	EventCountryDefeated   EventType = "country_defeated"
	EventAnnexation        EventType = "annexation"
	EventArmyMaintenance   EventType = "army_maintenance"
	EventMerchantArrived   EventType = "merchant_arrived"
	EventMerchantFled      EventType = "merchant_fled"
	EventMerchantRevolt    EventType = "merchant_revolt"
	EventRevoltSuccess     EventType = "revolt_success"
	EventRevoltFailed      EventType = "revolt_failed"
	EventRepublicFormed    EventType = "republic_formed"
	EventMonarchDeposed    EventType = "monarch_deposed"
	EventTreasurySplit     EventType = "treasury_split"
	EventRepublicTaxVote   EventType = "republic_tax_vote"
	EventRepublicGoldShared EventType = "republic_gold_shared"
	EventCountryCollapsed  EventType = "country_collapsed"
	EventRepublicAbandoned EventType = "republic_abandoned"
	EventArmyContributed   EventType = "army_contributed"
	EventRepublicWarVote   EventType = "republic_war_vote"
	EventRepublicFallen    EventType = "republic_fallen"
	EventPhaseStarted      EventType = "phase_started"
	EventPhaseEnded        EventType = "phase_ended"
	EventTurnStarted       EventType = "turn_started"
	EventTurnEnded         EventType = "turn_ended"
	EventGameOver          EventType = "game_over"
)

// Event represents something that happened in the game
type Event interface {
	// Type returns the event type
	Type() EventType

	// Data returns event-specific data
	Data() map[string]interface{}

	// String returns a human-readable description
	String() string
}

// BaseEvent provides common event functionality
type BaseEvent struct {
	eventType EventType
	data      map[string]interface{}
}

func NewBaseEvent(eventType EventType) *BaseEvent {
	return &BaseEvent{
		eventType: eventType,
		data:      make(map[string]interface{}),
	}
}

func (e *BaseEvent) Type() EventType {
	return e.eventType
}

func (e *BaseEvent) Data() map[string]interface{} {
	return e.data
}

func (e *BaseEvent) Set(key string, value interface{}) {
	e.data[key] = value
}

func (e *BaseEvent) Get(key string) interface{} {
	return e.data[key]
}

func (e *BaseEvent) String() string {
	return string(e.eventType)
}

// EventBus allows subscribing to and publishing events
type EventBus struct {
	subscribers []func(Event)
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make([]func(Event), 0),
	}
}

func (eb *EventBus) Subscribe(handler func(Event)) {
	eb.subscribers = append(eb.subscribers, handler)
}

func (eb *EventBus) Publish(events ...Event) {
	for _, event := range events {
		for _, handler := range eb.subscribers {
			handler(event)
		}
	}
}
