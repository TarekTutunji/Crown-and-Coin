package jsonapi

import (
	"encoding/json"
	"fmt"

	"crown_and_coin/actions"
	"crown_and_coin/engine"
	"crown_and_coin/events"
	"crown_and_coin/phases"
)

// GameAPI provides a JSON message-based interface to the game engine
type GameAPI struct {
	engine *engine.Engine
	phases map[engine.PhaseType]phases.Phase
	dice   engine.DiceRoller
}

// NewGameAPI creates a new JSON API wrapper
func NewGameAPI() *GameAPI {
	return NewGameAPIWithDice(engine.NewRandomDice())
}

// NewGameAPIWithDice creates a new JSON API with a specific dice roller (for testing)
func NewGameAPIWithDice(dice engine.DiceRoller) *GameAPI {
	api := &GameAPI{
		engine: engine.NewEngine(dice),
		phases: make(map[engine.PhaseType]phases.Phase),
		dice:   dice,
	}

	// Register all phases
	api.phases[engine.PhaseTaxation] = phases.NewTaxationPhase(dice)
	api.phases[engine.PhaseSpending] = phases.NewSpendingPhase(dice)
	api.phases[engine.PhaseWar] = phases.NewWarPhase(dice)
	api.phases[engine.PhaseAssessment] = phases.NewAssessmentPhase(dice)

	return api
}

// ProcessMessage handles a JSON message and returns a JSON response
func (api *GameAPI) ProcessMessage(data []byte) ([]byte, error) {
	reqType, req, err := ParseRequest(data)
	if err != nil {
		return api.errorResponse(fmt.Sprintf("invalid request: %v", err))
	}

	var response interface{}

	switch reqType {
	case RequestGetPlayers:
		response = api.handleGetPlayers()
	case RequestAddMerchant:
		response = api.handleAddMerchant(req.(*AddMerchantRequest))
	case RequestAddCountry:
		response = api.handleAddCountry(req.(*AddCountryRequest))
	case RequestGetState:
		response = api.handleGetState()
	case RequestGetActions:
		response = api.handleGetActions(req.(*GetActionsRequest))
	case RequestSubmit:
		response = api.handleSubmit(req.(*SubmitRequest))
	case RequestGetQueued:
		response = api.handleGetQueued(req.(*GetQueuedRequest))
	case RequestPendingActions:
		response = api.handleGetPendingActions(req.(*GetPendingActionsRequest))
	case RequestCancelActions:
		response = api.handleCancelActions(req.(*CancelActionsRequest))
	case RequestAdvance:
		response = api.handleAdvance()
	default:
		return api.errorResponse(fmt.Sprintf("unknown request type: %s", reqType))
	}

	return json.Marshal(response)
}

func (api *GameAPI) handleGetPlayers() *GetPlayersResponse {
	state := api.engine.GetState()
	players := make(map[string]*PlayerInfo)

	// Add monarchs
	for _, country := range state.Countries {
		if !country.IsRepublic && country.MonarchID != "" {
			players[country.MonarchID] = &PlayerInfo{
				CountryID: country.ID,
				Role:      "monarch",
			}
		}
	}

	// Add merchants
	for _, merchant := range state.Merchants {
		players[merchant.ID] = &PlayerInfo{
			CountryID: merchant.CountryID,
			Role:      "merchant",
		}
	}

	return &GetPlayersResponse{
		Success: true,
		Players: players,
	}
}

func (api *GameAPI) handleAddMerchant(req *AddMerchantRequest) *AddMerchantResponse {
	state := api.engine.GetState()

	// Check if player_id is already in use (as merchant or monarch)
	if state.GetMerchant(req.PlayerID) != nil {
		return &AddMerchantResponse{
			Success: false,
			Error:   fmt.Sprintf("player_id '%s' already exists as a merchant", req.PlayerID),
		}
	}
	for _, country := range state.Countries {
		if country.MonarchID == req.PlayerID {
			return &AddMerchantResponse{
				Success: false,
				Error:   fmt.Sprintf("player_id '%s' already exists as a monarch", req.PlayerID),
			}
		}
	}

	// Check if country exists
	if state.GetCountry(req.CountryID) == nil {
		return &AddMerchantResponse{
			Success: false,
			Error:   fmt.Sprintf("country_id '%s' not found", req.CountryID),
		}
	}

	merchant := engine.NewMerchant(req.PlayerID, req.CountryID)
	state.AddMerchant(merchant)

	return &AddMerchantResponse{Success: true}
}

func (api *GameAPI) handleAddCountry(req *AddCountryRequest) *AddCountryResponse {
	state := api.engine.GetState()

	// Check if country_id is already in use
	if state.GetCountry(req.CountryID) != nil {
		return &AddCountryResponse{
			Success: false,
			Error:   fmt.Sprintf("country_id '%s' already exists", req.CountryID),
		}
	}

	// Check if monarch_id is already in use
	for _, country := range state.Countries {
		if country.MonarchID == req.MonarchID {
			return &AddCountryResponse{
				Success: false,
				Error:   fmt.Sprintf("monarch_id '%s' already exists as a monarch", req.MonarchID),
			}
		}
	}
	if state.GetMerchant(req.MonarchID) != nil {
		return &AddCountryResponse{
			Success: false,
			Error:   fmt.Sprintf("monarch_id '%s' already exists as a merchant", req.MonarchID),
		}
	}

	country := engine.NewCountry(req.CountryID, req.MonarchID)
	state.AddCountry(country)

	return &AddCountryResponse{Success: true}
}

func (api *GameAPI) handleGetState() *StateResponse {
	return &StateResponse{
		Success: true,
		State:   SerializeState(api.engine.GetState()),
	}
}

func (api *GameAPI) handleGetActions(req *GetActionsRequest) *ActionsResponse {
	state := api.engine.GetState()
	currentPhase := state.Phase

	// Get the phase handler
	phase, ok := api.phases[currentPhase]
	if !ok {
		// No actions for this phase (e.g., Negotiation)
		return &ActionsResponse{
			Success:  true,
			PlayerID: req.PlayerID,
			Phase:    currentPhase.String(),
			Actions:  []ActionJSON{},
		}
	}

	// Get valid actions for this player
	validActions := phase.ValidActions(state, req.PlayerID)

	// Convert to JSON format
	actionJSONs := make([]ActionJSON, len(validActions))
	for i, action := range validActions {
		actionJSONs[i] = SerializeAction(action, state, true) // Use placeholders for valid actions
	}

	return &ActionsResponse{
		Success:  true,
		PlayerID: req.PlayerID,
		Phase:    currentPhase.String(),
		Actions:  actionJSONs,
	}
}

func (api *GameAPI) handleSubmit(req *SubmitRequest) *SubmitResponse {
	state := api.engine.GetState()

	// Get existing pending actions for validation
	pendingActions := make([]actions.Action, 0)
	for _, act := range api.engine.GetPendingActions() {
		if action, ok := act.(actions.Action); ok {
			pendingActions = append(pendingActions, action)
		}
	}

	// Deserialize the action
	action, err := DeserializeAction(req.Action)
	if err != nil {
		return &SubmitResponse{
			Success:         false,
			Action:          req.Action,
			RejectionReason: fmt.Sprintf("invalid action: %v", err),
		}
	}

	// Validate against current state
	if err := action.Validate(state); err != nil {
		return &SubmitResponse{
			Success:         false,
			Action:          req.Action,
			RejectionReason: fmt.Sprintf("validation failed: %v", err),
		}
	}

	// Validate against pending actions
	if reason := api.validateAgainstPending(action, pendingActions, state); reason != "" {
		return &SubmitResponse{
			Success:         false,
			Action:          req.Action,
			RejectionReason: reason,
		}
	}

	// Action is valid, add to pending
	api.engine.SubmitAction(action)

	return &SubmitResponse{
		Success: true,
		Action:  req.Action,
	}
}

// validateAgainstPending checks if an action conflicts with already pending actions
func (api *GameAPI) validateAgainstPending(action actions.Action, pendingActions []actions.Action, state *engine.GameState) string {
	playerID := action.PlayerID()

	// Filter pending actions to only those from the same player
	playerPending := make([]actions.Action, 0)
	for _, pa := range pendingActions {
		if pa.PlayerID() == playerID {
			playerPending = append(playerPending, pa)
		}
	}

	if reason := api.validateRepublicChoice(action, playerPending, state); reason != "" {
		return reason
	}

	switch a := action.(type) {
	case *actions.TaxPeasantsAction:
		return api.validatePeasantTaxation(a.CountryID, playerPending)

	case *actions.BuildArmyAction:
		return api.validateGoldSpending(a.CountryID, a.Amount, playerPending, state)

	case *actions.MonarchInvestAction:
		return api.validateGoldSpending(a.CountryID, a.Amount, playerPending, state)

	case *actions.TaxMerchantsAction:
		return api.validateMerchantTaxation(a.MerchantID, a.Amount, playerPending, state)

	case *actions.MerchantInvestAction:
		return api.validateMerchantGoldSpending(a.MerchantID, a.Amount, playerPending, state)

	case *actions.ContributeArmyAction:
		return api.validateMerchantGoldSpending(a.MerchantID, a.Amount, playerPending, state)

	case *actions.AttackAction:
		return api.validateWarAction(a.AttackerID, true, a.DefenderID, playerPending)

	case *actions.NoAttackAction:
		return api.validateWarAction(a.CountryID, false, "", playerPending)

	case *actions.RemainAction, *actions.FleeAction, *actions.RevoltAction:
		return api.validateMerchantAssessment(action, playerPending)
	}

	return ""
}

// validateGoldSpending checks if a country has enough gold for the action
func (api *GameAPI) validateGoldSpending(countryID string, amount int, pending []actions.Action, state *engine.GameState) string {
	country := state.GetCountry(countryID)
	if country == nil {
		return "country not found"
	}

	totalSpent := amount
	for _, pa := range pending {
		switch a := pa.(type) {
		case *actions.BuildArmyAction:
			if a.CountryID == countryID {
				totalSpent += a.Amount
			}
		case *actions.MonarchInvestAction:
			if a.CountryID == countryID {
				totalSpent += a.Amount
			}
		}
	}

	if totalSpent > country.Gold {
		return fmt.Sprintf("insufficient gold: trying to spend %d but only have %d (including pending actions)", totalSpent, country.Gold)
	}

	return ""
}

// validateMerchantTaxation checks if a merchant is being over-taxed
func (api *GameAPI) validateMerchantTaxation(merchantID string, amount int, pending []actions.Action, state *engine.GameState) string {
	merchant := state.GetMerchant(merchantID)
	if merchant == nil {
		return "merchant not found"
	}

	totalTaxed := amount
	for _, pa := range pending {
		if a, ok := pa.(*actions.TaxMerchantsAction); ok {
			if a.MerchantID == merchantID {
				totalTaxed += a.Amount
			}
		}
	}

	if totalTaxed > merchant.StoredGold {
		return fmt.Sprintf("merchant has insufficient gold: trying to tax %d but merchant only has %d (including pending taxes)", totalTaxed, merchant.StoredGold)
	}

	return ""
}

// validateMerchantGoldSpending checks if a merchant has enough gold
func (api *GameAPI) validateMerchantGoldSpending(merchantID string, amount int, pending []actions.Action, state *engine.GameState) string {
	merchant := state.GetMerchant(merchantID)
	if merchant == nil {
		return "merchant not found"
	}

	totalSpent := amount
	totalTaxed := 0

	for _, pa := range pending {
		switch a := pa.(type) {
		case *actions.MerchantInvestAction:
			if a.MerchantID == merchantID {
				totalSpent += a.Amount
			}
		case *actions.ContributeArmyAction:
			if a.MerchantID == merchantID {
				totalSpent += a.Amount
			}
		case *actions.TaxMerchantsAction:
			if a.MerchantID == merchantID {
				totalTaxed += a.Amount
			}
		}
	}

	available := merchant.StoredGold - totalTaxed
	if totalSpent > available {
		return fmt.Sprintf("merchant has insufficient gold: trying to spend %d but only have %d after pending taxes", totalSpent, available)
	}

	return ""
}

// validateWarAction checks if a country already has a war action pending
func (api *GameAPI) validateWarAction(countryID string, isAttack bool, targetID string, pending []actions.Action) string {
	for _, pa := range pending {
		switch a := pa.(type) {
		case *actions.AttackAction:
			if a.AttackerID == countryID {
				if isAttack && a.DefenderID == targetID {
					return "already have this attack action pending"
				}
				return "already have an attack action pending, cannot submit another war action"
			}
		case *actions.NoAttackAction:
			if a.CountryID == countryID {
				return "already have a no-attack action pending, cannot submit another war action"
			}
		}
	}

	return ""
}

// validateMerchantAssessment checks if a merchant already has an assessment action pending
func (api *GameAPI) validateMerchantAssessment(action actions.Action, pending []actions.Action) string {
	var merchantID string

	switch a := action.(type) {
	case *actions.RemainAction:
		merchantID = a.MerchantID
	case *actions.FleeAction:
		merchantID = a.MerchantID
	case *actions.RevoltAction:
		merchantID = a.MerchantID
	default:
		return ""
	}

	for _, pa := range pending {
		var pendingMerchantID string
		switch a := pa.(type) {
		case *actions.RemainAction:
			pendingMerchantID = a.MerchantID
		case *actions.FleeAction:
			pendingMerchantID = a.MerchantID
		case *actions.RevoltAction:
			pendingMerchantID = a.MerchantID
		}

		if pendingMerchantID == merchantID {
			return "merchant already has an assessment action pending"
		}
	}

	return ""
}

// republicChoice returns the merchant behind a republic action and which
// decision it belongs to. A merchant of a republic gets one tax vote, one
// spending choice (invest, hide or contribute to the army) and one war vote
// per round.
func republicChoice(action actions.Action) (merchantID, decision string) {
	switch a := action.(type) {
	case *actions.VoteTaxAction:
		return a.MerchantID, "tax vote"
	case *actions.MerchantInvestAction:
		return a.MerchantID, "spending choice"
	case *actions.MerchantHideAction:
		return a.MerchantID, "spending choice"
	case *actions.ContributeArmyAction:
		return a.MerchantID, "spending choice"
	case *actions.VoteAttackAction:
		return a.MerchantID, "war vote"
	case *actions.VoteNoAttackAction:
		return a.MerchantID, "war vote"
	}
	return "", ""
}

// validateRepublicChoice rejects a second decision of the same kind from a
// merchant of a republic
func (api *GameAPI) validateRepublicChoice(action actions.Action, pending []actions.Action, state *engine.GameState) string {
	merchantID, decision := republicChoice(action)
	if merchantID == "" {
		return ""
	}
	// Merchants of a monarchy may still combine investing and hiding
	merchant := state.GetMerchant(merchantID)
	if merchant == nil {
		return ""
	}
	country := state.GetCountry(merchant.CountryID)
	if country == nil || !country.IsRepublic {
		return ""
	}

	for _, pa := range pending {
		if pendingMerchantID, pendingDecision := republicChoice(pa); pendingMerchantID == merchantID && pendingDecision == decision {
			return fmt.Sprintf("merchant already has a %s pending", decision)
		}
	}
	return ""
}

// validatePeasantTaxation checks if there's already a peasant tax action pending for the country
func (api *GameAPI) validatePeasantTaxation(countryID string, pending []actions.Action) string {
	for _, pa := range pending {
		if a, ok := pa.(*actions.TaxPeasantsAction); ok {
			if a.CountryID == countryID {
				return "country already has a peasant tax action pending (cannot tax peasants multiple times)"
			}
		}
	}
	return ""
}

func (api *GameAPI) handleGetQueued(req *GetQueuedRequest) *QueuedResponse {
	state := api.engine.GetState()

	var filteredActions []ActionJSON
	for _, act := range api.engine.GetPendingActions() {
		action, ok := act.(actions.Action)
		if !ok {
			continue
		}
		// If player_id is specified, only include actions from that player
		if req.PlayerID != "" && action.PlayerID() != req.PlayerID {
			continue
		}
		filteredActions = append(filteredActions, SerializeAction(action, state, false)) // Use actual values for queued actions
	}

	return &QueuedResponse{
		Success: true,
		Phase:   state.Phase.String(),
		Actions: filteredActions,
	}
}

func (api *GameAPI) handleGetPendingActions(req *GetPendingActionsRequest) *PendingActionsResponse {
	state := api.engine.GetState()

	var filteredActions []ActionJSON
	for _, act := range api.engine.GetPendingActions() {
		action, ok := act.(actions.Action)
		if !ok {
			continue
		}
		// If player_id is specified, only include actions from that player
		if req.PlayerID != "" && action.PlayerID() != req.PlayerID {
			continue
		}
		filteredActions = append(filteredActions, SerializeAction(action, state, false)) // Use actual values for pending actions
	}

	return &PendingActionsResponse{
		Success: true,
		Phase:   state.Phase.String(),
		Actions: filteredActions,
	}
}

func (api *GameAPI) handleCancelActions(req *CancelActionsRequest) *CancelActionsResponse {
	if req.PlayerID == "" {
		return &CancelActionsResponse{
			Success: false,
			Error:   "player_id is required",
		}
	}

	removed := api.engine.ClearPendingActionsByPlayer(req.PlayerID)
	return &CancelActionsResponse{
		Success: true,
		Removed: removed,
	}
}

func (api *GameAPI) handleAdvance() *AdvanceResponse {
	state := api.engine.GetState()
	previousPhase := state.Phase

	// Get the phase handler
	phase, ok := api.phases[previousPhase]
	var allEvents []events.Event

	if ok {
		// Convert pending actions from []interface{} to []actions.Action
		pendingActions := make([]actions.Action, 0)
		for _, act := range api.engine.GetPendingActions() {
			if action, ok := act.(actions.Action); ok {
				pendingActions = append(pendingActions, action)
			}
		}

		// Execute the phase with pending actions from engine
		newState, phaseEvents := phase.Execute(state, pendingActions)
		api.engine.SetState(newState)
		allEvents = phaseEvents
	}

	// Clear pending actions for next phase
	api.engine.ClearPendingActions()

	// Advance to next phase
	api.engine.GetState().NextPhase()
	newState := api.engine.GetState()

	return &AdvanceResponse{
		Success:       true,
		PreviousPhase: previousPhase.String(),
		CurrentPhase:  newState.Phase.String(),
		Turn:          newState.Turn,
		Events:        SerializeEvents(allEvents),
		State:         SerializeState(newState),
	}
}

func (api *GameAPI) errorResponse(message string) ([]byte, error) {
	return json.Marshal(&ErrorResponse{
		Success: false,
		Error:   message,
	})
}

// GetEngine returns the underlying engine (for testing)
func (api *GameAPI) GetEngine() *engine.Engine {
	return api.engine
}
