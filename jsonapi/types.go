package jsonapi

import "encoding/json"

// RequestType identifies the type of API request
type RequestType string

const (
	RequestGetPlayers       RequestType = "get_players"
	RequestAddMerchant      RequestType = "add_merchant"
	RequestAddCountry       RequestType = "add_country"
	RequestGetState         RequestType = "get_state"
	RequestGetActions       RequestType = "get_actions"
	RequestSubmit           RequestType = "submit"
	RequestGetQueued        RequestType = "get_queued"
	RequestPendingActions   RequestType = "pending_actions"
	RequestCancelActions    RequestType = "cancel_actions"
	RequestAdvance          RequestType = "advance"
	RequestAssignRole       RequestType = "assign_role"
)

// Roles a player can be given with an AssignRoleRequest
const (
	RoleMonarch  = "monarch"
	RoleMerchant = "merchant"
	RoleNone     = "none"
)

// AssignRoleRequest moves a player into a new role and country mid-game
type AssignRoleRequest struct {
	Type      RequestType `json:"type"`
	PlayerID  string      `json:"player_id"`
	Role      string      `json:"role"`                 // "monarch", "merchant" or "none"
	CountryID string      `json:"country_id,omitempty"` // Not needed for "none"
}

// AssignRoleResponse confirms a role change
type AssignRoleResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// Request is the base request structure - use Type to determine specific request
type Request struct {
	Type RequestType `json:"type"`
}

// AddMerchantRequest adds a merchant to the game
type AddMerchantRequest struct {
	Type      RequestType `json:"type"`
	PlayerID  string      `json:"player_id"`
	CountryID string      `json:"country_id"`
}

// AddCountryRequest adds a country to the game
type AddCountryRequest struct {
	Type      RequestType `json:"type"`
	CountryID string      `json:"country_id"`
	MonarchID string      `json:"monarch_id"`
}

// GetActionsRequest requests valid actions for a player
type GetActionsRequest struct {
	Type     RequestType `json:"type"`
	PlayerID string      `json:"player_id"`
}

// GetQueuedRequest requests queued actions (optionally filtered by player)
type GetQueuedRequest struct {
	Type     RequestType `json:"type"`
	PlayerID string      `json:"player_id,omitempty"` // Optional: if empty, returns all actions
}

// GetPendingActionsRequest requests pending actions (optionally filtered by player)
type GetPendingActionsRequest struct {
	Type     RequestType `json:"type"`
	PlayerID string      `json:"player_id,omitempty"` // Optional: if empty, returns all actions (admin only)
}

// SubmitRequest submits a single action for the current phase
type SubmitRequest struct {
	Type   RequestType `json:"type"`
	Action ActionJSON  `json:"action"`
}

// CancelActionsRequest cancels all pending actions for a player
type CancelActionsRequest struct {
	Type     RequestType `json:"type"`
	PlayerID string      `json:"player_id"`
}

// CancelActionsResponse confirms cancellation
type CancelActionsResponse struct {
	Success bool   `json:"success"`
	Removed int    `json:"removed"`
	Error   string `json:"error,omitempty"`
}

// ActionJSON is a generic JSON representation of any action
type ActionJSON struct {
	Type       string `json:"type"`
	PlayerID   string `json:"player_id,omitempty"`
	CountryID  string `json:"country_id,omitempty"`
	MerchantID string `json:"merchant_id,omitempty"`
	TargetID   string `json:"target_id,omitempty"` // For attacks, flee destination
	Amount     any    `json:"amount,omitempty"`    // Can be int or placeholder string
}

// Response types

// ErrorResponse is returned when an error occurs
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// PlayerInfo describes a player's country and role
type PlayerInfo struct {
	CountryID string `json:"country_id"`
	Role      string `json:"role"` // "monarch" or "merchant"
}

// GetPlayersResponse returns all players
type GetPlayersResponse struct {
	Success bool                   `json:"success"`
	Players map[string]*PlayerInfo `json:"players"`
}

// AddMerchantResponse confirms merchant addition
type AddMerchantResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// AddCountryResponse confirms country addition
type AddCountryResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// StateResponse returns the current game state
type StateResponse struct {
	Success bool       `json:"success"`
	State   *StateJSON `json:"state"`
}

// StateJSON is the JSON representation of GameState
type StateJSON struct {
	Turn      int                      `json:"turn"`
	Phase     string                   `json:"phase"`
	Countries map[string]*CountryJSON  `json:"countries"`
	Merchants map[string]*MerchantJSON `json:"merchants"`
}

// CountryJSON is the JSON representation of Country
type CountryJSON struct {
	CountryID    string `json:"country_id"`
	HP           int    `json:"hp"`
	ArmyStrength int    `json:"army_strength"`
	Gold         int    `json:"gold"`
	Peasants     int    `json:"peasants"`
	RevoltRisk   int    `json:"revolt_risk"`
	IsRepublic   bool   `json:"is_republic"`
	MonarchID    string `json:"monarch_id"`
	DiedOnce     bool   `json:"died_once"`
	Hidden       bool   `json:"hidden,omitempty"` // Gold, peasants and revolt risk are secret from this viewer
}

// MerchantJSON is the JSON representation of Merchant
type MerchantJSON struct {
	PlayerID     string `json:"player_id"`
	CountryID    string `json:"country_id"`
	StoredGold   int    `json:"stored_gold"`           // Purse, the monarch can tax it
	HiddenGold   int    `json:"hidden_gold,omitempty"` // Safe from tax
	InvestedGold int    `json:"invested_gold"`
	Hidden       bool   `json:"hidden,omitempty"`       // Hidden and invested gold are secret from this viewer
	PurseHidden  bool   `json:"purse_hidden,omitempty"` // The purse is secret from this viewer too
}

// ActionsResponse returns valid actions for a player
type ActionsResponse struct {
	Success  bool         `json:"success"`
	PlayerID string       `json:"player_id"`
	Phase    string       `json:"phase"`
	Actions  []ActionJSON `json:"actions"`
}

// SubmitResponse confirms action was queued or rejected
type SubmitResponse struct {
	Success         bool       `json:"success"`
	Action          ActionJSON `json:"action"`
	RejectionReason string     `json:"rejection_reason,omitempty"`
}

// QueuedResponse returns currently queued actions
type QueuedResponse struct {
	Success bool         `json:"success"`
	Phase   string       `json:"phase"`
	Actions []ActionJSON `json:"actions"`
}

// PendingActionsResponse returns currently pending actions
type PendingActionsResponse struct {
	Success bool         `json:"success"`
	Phase   string       `json:"phase"`
	Actions []ActionJSON `json:"actions"`
}

// AdvanceResponse returns results of advancing to next phase
type AdvanceResponse struct {
	Success       bool        `json:"success"`
	PreviousPhase string      `json:"previous_phase"`
	CurrentPhase  string      `json:"current_phase"`
	Turn          int         `json:"turn"`
	Events        []EventJSON `json:"events"`
	State         *StateJSON  `json:"state"`
}

// EventJSON is the JSON representation of a game event
type EventJSON struct {
	Type    string                 `json:"type"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// ParseRequest parses a JSON message into a typed request
func ParseRequest(data []byte) (RequestType, interface{}, error) {
	var base Request
	if err := json.Unmarshal(data, &base); err != nil {
		return "", nil, err
	}

	switch base.Type {
	case RequestGetPlayers:
		return base.Type, &base, nil

	case RequestAddMerchant:
		var req AddMerchantRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return base.Type, nil, err
		}
		return base.Type, &req, nil

	case RequestAddCountry:
		var req AddCountryRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return base.Type, nil, err
		}
		return base.Type, &req, nil

	case RequestGetState:
		return base.Type, &base, nil

	case RequestGetActions:
		var req GetActionsRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return base.Type, nil, err
		}
		return base.Type, &req, nil

	case RequestSubmit:
		var req SubmitRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return base.Type, nil, err
		}
		return base.Type, &req, nil

	case RequestGetQueued:
		var req GetQueuedRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return base.Type, nil, err
		}
		return base.Type, &req, nil

	case RequestPendingActions:
		var req GetPendingActionsRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return base.Type, nil, err
		}
		return base.Type, &req, nil

	case RequestCancelActions:
		var req CancelActionsRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return base.Type, nil, err
		}
		return base.Type, &req, nil

	case RequestAdvance:
		return base.Type, &base, nil

	case RequestAssignRole:
		var req AssignRoleRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return base.Type, nil, err
		}
		return base.Type, &req, nil

	default:
		return base.Type, nil, nil
	}
}
