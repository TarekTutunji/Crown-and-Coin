package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"

	"github.com/gorilla/websocket"
)

type User struct {
	Name   string
	Secret string
}

type ClientConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *ClientConn) send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *ClientConn) sendError(message string) {
	resp, _ := json.Marshal(ErrorResponse{Success: false, Error: message})
	c.send(resp)
}

type ActionEntry struct {
	PlayerID  string                 `json:"player_id"`
	Action    jsonapi.ActionJSON     `json:"action"`
	Turn      int                    `json:"turn"`
	Phase     string                 `json:"phase"`
	Timestamp time.Time              `json:"timestamp"`
}

type StateSnapshot struct {
	Turn      int                    `json:"turn"`
	Phase     string                 `json:"phase"`
	State     *jsonapi.StateJSON     `json:"state"`
	Events    []jsonapi.EventJSON    `json:"events,omitempty"` // What happened when the phase was resolved (admin only)
	Timestamp time.Time              `json:"timestamp"`
}

type GameHistory struct {
	GameName       string           `json:"game_name"`
	Actions        []ActionEntry    `json:"actions"`
	StateSnapshots []StateSnapshot  `json:"state_snapshots"`
	PhaseStartIdx  int              `json:"phase_start_idx"` // Index in Actions where current phase started
}

type Server struct {
	users    map[string]*User // name -> user
	mu       sync.RWMutex
	api      *jsonapi.GameAPI
	upgrader websocket.Upgrader

	clients   map[*ClientConn]string // connection -> username
	clientsMu sync.RWMutex

	history   *GameHistory
	historyMu sync.RWMutex

	// gameMu makes players take turns changing or reading the game
	gameMu sync.Mutex

	// board is what the public projector board is allowed to show, guarded by
	// gameMu like the game itself
	board *BoardProjection

	// ready holds the players who have said they are done with this phase
	ready   map[string]bool
	readyMu sync.Mutex
}

type ClientMessage struct {
	User    string          `json:"user"`
	Secret  string          `json:"secret"`
	Payload json.RawMessage `json:"payload"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// newGameName names a game after the time it started. The history is written
// to <game name>.md, so a name already used by an earlier game gets a number
// added rather than overwriting that game's record.
func newGameName(previous string) string {
	base := time.Now().Format("2006-01-02_15-04-05")
	name := base
	for i := 2; name == previous || fileExists(name+".md"); i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func NewServer() *Server {
	gameName := newGameName("")
	return &Server{
		users:   make(map[string]*User),
		clients: make(map[*ClientConn]string),
		ready:   make(map[string]bool),
		board:   NewBoardProjection(),
		api:    jsonapi.NewGameAPIWithDice(engine.NewRandomDice()),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		history: &GameHistory{
			GameName:       gameName,
			Actions:        make([]ActionEntry, 0),
			StateSnapshots: make([]StateSnapshot, 0),
			PhaseStartIdx:  0,
		},
	}
}

func generateSecret() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (s *Server) registerUser(name, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[name]; exists {
		return fmt.Errorf("user already exists")
	}

	s.users[name] = &User{Name: name, Secret: secret}
	return nil
}

func (s *Server) authenticateUser(name, secret string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[name]
	if !exists {
		return false
	}
	return user.Secret == secret
}

func (s *Server) isAdmin(name string) bool {
	return name == "admin"
}

func (s *Server) canSendMessage(user string, payload json.RawMessage) bool {
	if s.isAdmin(user) {
		return true
	}

	var msg struct {
		Type     string `json:"type"`
		PlayerID string `json:"player_id"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		return false
	}

	switch msg.Type {
	case "get_state", "get_players", "get_connected_players", "get_history", "get_settings":
		return true
	case "get_actions", "get_queued", "cancel_actions", "set_ready":
		return msg.PlayerID == user
	case "submit":
		// Check if action belongs to this user
		var submitMsg struct {
			Action struct {
				PlayerID string `json:"player_id"`
			} `json:"action"`
		}
		if err := json.Unmarshal(payload, &submitMsg); err != nil {
			return false
		}
		return submitMsg.Action.PlayerID == user
	case "add_country", "add_merchant", "advance", "assign_role", "set_settings", "new_game", "end_game":
		return false // admin only
	default:
		return false
	}
}

func (s *Server) getConnectedPlayerNames() []string {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, username := range s.clients {
		if username != "admin" && !seen[username] {
			seen[username] = true
			names = append(names, username)
		}
	}
	return names
}

func (s *Server) broadcastConnectedPlayers() {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, username := range s.clients {
		if username != "admin" && !seen[username] {
			seen[username] = true
			names = append(names, username)
		}
	}

	for client, username := range s.clients {
		client.send(s.connectedPlayersMessage(names, username))
	}
}

// connectedPlayersMessage lists the connected players, along with who is
// ready for the next phase. The admin sees everyone's checkmark; a player
// only learns whether they themselves are marked ready.
func (s *Server) connectedPlayersMessage(names []string, viewer string) []byte {
	s.readyMu.Lock()
	ready := make([]string, 0)
	for name := range s.ready {
		if s.isAdmin(viewer) || name == viewer {
			ready = append(ready, name)
		}
	}
	s.readyMu.Unlock()

	msg, _ := json.Marshal(map[string]interface{}{
		"type":    "connected_players",
		"success": true,
		"players": names,
		"ready":   ready,
	})
	return msg
}

// setReady marks a player as done with this phase, or not
func (s *Server) setReady(playerID string, ready bool) {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	if ready {
		s.ready[playerID] = true
	} else {
		delete(s.ready, playerID)
	}
}

// clearReady takes away every checkmark once a new phase begins
func (s *Server) clearReady() {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	s.ready = make(map[string]bool)
}

// startNewGame clears the table for a fresh game: the board, the moves waiting
// to be resolved, the history and the ready checkmarks all go together, and
// the new game gets its own name so it writes its own history file. The
// settings and the player accounts are kept, so nobody has to log in again.
// It returns the name of the new game.
func (s *Server) startNewGame() string {
	s.api.NewGame()

	s.historyMu.Lock()
	name := newGameName(s.history.GameName)
	s.history = &GameHistory{
		GameName:       name,
		Actions:        make([]ActionEntry, 0),
		StateSnapshots: make([]StateSnapshot, 0),
		PhaseStartIdx:  0,
	}
	s.historyMu.Unlock()

	s.board.Reset()
	s.clearReady()
	return name
}

func (s *Server) broadcastHistoryToAdmin() {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	msg, _ := json.Marshal(map[string]interface{}{
		"type":    "history_update",
		"success": true,
		"history": s.history,
	})

	for client, username := range s.clients {
		if username == "admin" {
			client.send(msg)
		}
	}
}

func (s *Server) broadcastHistoryToPlayers() {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for client, username := range s.clients {
		if username != "admin" {
			msg, _ := json.Marshal(map[string]interface{}{
				"type":    "history_update",
				"success": true,
				"history": s.getHistoryForPlayer(username),
			})
			client.send(msg)
		}
	}
}

// getHistoryForPlayer returns the part of the history a player may see, from
// the phases that are over: their own actions, their monarch's actions (so a
// merchant learns the tax choice once Taxation ends), and every country's war
// declarations, which are revealed to everyone at once. Other players'
// choices and votes stay secret, and the full state snapshots are left out.
// In an open game they see everything, like the admin.
func (s *Server) getHistoryForPlayer(playerID string) interface{} {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	if s.api.IsOpenGame() {
		return s.history
	}

	state := s.api.GetEngine().GetState()
	monarchID := ""
	if merchant := state.GetMerchant(playerID); merchant != nil && !merchant.Arriving {
		if country := state.GetCountry(merchant.CountryID); country != nil {
			monarchID = country.MonarchID
		}
	}

	// Players get actions only from before the current phase
	playerActions := make([]ActionEntry, 0)
	for _, entry := range s.history.Actions[:s.history.PhaseStartIdx] {
		isWarDeclaration := entry.Action.Type == "attack" || entry.Action.Type == "no_attack"
		if entry.PlayerID == playerID || (monarchID != "" && entry.PlayerID == monarchID) || isWarDeclaration {
			playerActions = append(playerActions, entry)
		}
	}

	return map[string]interface{}{
		"game_name":       s.history.GameName,
		"actions":         playerActions,
		"state_snapshots": []StateSnapshot{},
		"phase_start_idx": len(playerActions),
	}
}

// republicAttacks turns the attacks republics decided on by vote into history
// entries, so they are revealed along with the monarchs' war declarations
func republicAttacks(evts []jsonapi.EventJSON, turn int) []ActionEntry {
	var entries []ActionEntry
	for _, evt := range evts {
		if evt.Type != "republic_war_vote" {
			continue
		}
		countryID, _ := evt.Data["country_id"].(string)
		targetID, _ := evt.Data["target_id"].(string)
		if targetID == "" {
			continue
		}
		entries = append(entries, ActionEntry{
			PlayerID:  countryID,
			Action:    jsonapi.ActionJSON{Type: "attack", PlayerID: countryID, CountryID: countryID, TargetID: targetID},
			Turn:      turn,
			Phase:     "war",
			Timestamp: time.Now(),
		})
	}
	return entries
}

func (s *Server) saveHistoryToMarkdown() {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	filename := s.history.GameName + ".md"
	f, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create history file: %v", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# Game History: %s\n\n", s.history.GameName)

	for _, snapshot := range s.history.StateSnapshots {
		fmt.Fprintf(f, "## Turn %d - %s\n\n", snapshot.Turn, snapshot.Phase)

		// State
		fmt.Fprintf(f, "### State\n\n")
		if snapshot.State != nil {
			fmt.Fprintf(f, "#### Countries\n")
			for _, country := range snapshot.State.Countries {
				status := "Alive"
				if country.HP <= 0 {
					status = "Defeated"
				}
				fmt.Fprintf(f, "- **%s** (%s): HP=%d, Gold=%d, Army=%d, Peasants=%d\n",
					country.CountryID, status, country.HP, country.Gold,
					country.ArmyStrength, country.Peasants)
			}
			fmt.Fprintf(f, "\n#### Merchants\n")
			for _, merchant := range snapshot.State.Merchants {
				fmt.Fprintf(f, "- **%s** in %s: Purse=%d, Hidden=%d, Invested=%d\n",
					merchant.PlayerID, merchant.CountryID,
					merchant.StoredGold, merchant.HiddenGold, merchant.InvestedGold)
			}
			fmt.Fprintf(f, "\n")
		}

		// What happened when the phase was resolved
		if len(snapshot.Events) > 0 {
			fmt.Fprintf(f, "### Events\n\n")
			for _, event := range snapshot.Events {
				fmt.Fprintf(f, "- %s\n", event.Message)
			}
			fmt.Fprintf(f, "\n")
		}

		// Actions for this phase
		fmt.Fprintf(f, "### Actions\n\n")
		phaseKey := fmt.Sprintf("%d-%s", snapshot.Turn, snapshot.Phase)
		hasActions := false
		for _, entry := range s.history.Actions {
			if fmt.Sprintf("%d-%s", entry.Turn, entry.Phase) == phaseKey {
				fmt.Fprintf(f, "- **%s**: %s (%s)\n", entry.PlayerID,
					formatActionForMarkdown(entry.Action),
					entry.Timestamp.Format("15:04:05"))
				hasActions = true
			}
		}
		if !hasActions {
			fmt.Fprintf(f, "*No actions this phase.*\n")
		}
		fmt.Fprintf(f, "\n---\n\n")
	}

	log.Printf("Game history saved to %s", filename)
}

func formatActionForMarkdown(action jsonapi.ActionJSON) string {
	switch action.Type {
	case "tax_peasants_low":
		return "Tax Peasants (Low)"
	case "tax_peasants_high":
		return "Tax Peasants (High)"
	case "tax_merchants":
		return fmt.Sprintf("Tax %s (%v)", action.MerchantID, action.Amount)
	case "build_army":
		return fmt.Sprintf("Build Army (%v)", action.Amount)
	case "merchant_invest":
		return fmt.Sprintf("Invest %v", action.Amount)
	case "merchant_hide":
		if action.Amount == nil {
			return "Hide 0"
		}
		return fmt.Sprintf("Hide %v", action.Amount)
	case "attack":
		return fmt.Sprintf("Attack %s", action.TargetID)
	case "no_attack":
		return "No Attack"
	case "remain":
		return "Remain"
	case "flee":
		return fmt.Sprintf("Flee to %s", action.TargetID)
	case "revolt":
		return "Revolt"
	case "vote_tax_low":
		return "Vote: Low Tax"
	case "vote_tax_high":
		return "Vote: High Tax"
	case "contribute_army":
		return fmt.Sprintf("Contribute %v to Army", action.Amount)
	case "vote_attack":
		return fmt.Sprintf("Vote: Attack %s", action.TargetID)
	case "vote_no_attack":
		return "Vote: No Attack"
	default:
		return action.Type
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("New WebSocket connection from %s", r.RemoteAddr)

	client := &ClientConn{conn: conn}
	var connUser string

	defer func() {
		if connUser != "" {
			s.clientsMu.Lock()
			delete(s.clients, client)
			s.clientsMu.Unlock()
			log.Printf("Player disconnected: %s", connUser)
			s.broadcastConnectedPlayers()
		}
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			log.Printf("Invalid message format: %v", err)
			client.sendError("invalid message format")
			continue
		}

		if !s.authenticateUser(clientMsg.User, clientMsg.Secret) {
			log.Printf("Authentication failed for user: %s", clientMsg.User)
			client.sendError("authentication failed")
			continue
		}

		// Register connection on first authenticated message
		if connUser == "" {
			connUser = clientMsg.User
			s.clientsMu.Lock()
			s.clients[client] = connUser
			s.clientsMu.Unlock()
			log.Printf("Player connected: %s", connUser)
			s.broadcastConnectedPlayers()
		}

		if !s.canSendMessage(clientMsg.User, clientMsg.Payload) {
			log.Printf("Permission denied for user %s", clientMsg.User)
			client.sendError("permission denied")
			continue
		}

		if !s.handleMessage(client, clientMsg) {
			break
		}
	}
}

// handleMessage answers one message from an authenticated player. Messages
// are handled one at a time: with many players connected, two moves arriving
// at the same moment would otherwise overwrite each other. It returns false
// once the connection has failed.
func (s *Server) handleMessage(client *ClientConn, clientMsg ClientMessage) bool {
	s.gameMu.Lock()
	defer s.gameMu.Unlock()

	// Handle non-engine messages separately
	var msgType struct {
		Type string `json:"type"`
	}
	json.Unmarshal(clientMsg.Payload, &msgType)

	if msgType.Type == "get_connected_players" {
		client.send(s.connectedPlayersMessage(s.getConnectedPlayerNames(), clientMsg.User))
		return true
	}

	// A player says they are done with this phase, or takes it back
	if msgType.Type == "set_ready" {
		var readyMsg struct {
			PlayerID string `json:"player_id"`
			Ready    bool   `json:"ready"`
		}
		json.Unmarshal(clientMsg.Payload, &readyMsg)
		s.setReady(readyMsg.PlayerID, readyMsg.Ready)
		s.broadcastConnectedPlayers()
		return true
	}

	// The game leader wipes the table and starts a new game
	if msgType.Type == "new_game" {
		name := s.startNewGame()
		resp, _ := json.Marshal(map[string]interface{}{
			"type":      "new_game",
			"success":   true,
			"game_name": name,
		})
		log.Printf("New game started: %s", name)
		s.broadcastConnectedPlayers()
		s.broadcastHistoryToAdmin()
		s.broadcastHistoryToPlayers()
		if err := client.send(resp); err != nil {
			log.Printf("Write error: %v", err)
			return false
		}
		return true
	}

	// The game leader calls the game over, or takes it back. The engine is not
	// touched either way: this only decides whether the projector board shows
	// the live game or the end-game summary.
	if msgType.Type == "end_game" {
		var endMsg struct {
			Ended *bool `json:"ended"`
		}
		json.Unmarshal(clientMsg.Payload, &endMsg)
		ended := endMsg.Ended == nil || *endMsg.Ended // Saying nothing means end it
		s.board.SetEnded(ended)
		if ended {
			log.Printf("The game was called over")
		} else {
			log.Printf("The game leader dismissed the summary")
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"type":    "end_game",
			"success": true,
			"ended":   ended,
		})
		if err := client.send(resp); err != nil {
			log.Printf("Write error: %v", err)
			return false
		}
		return true
	}

	if msgType.Type == "get_history" {
		var history interface{}
		if clientMsg.User == "admin" {
			s.historyMu.RLock()
			history = s.history
			s.historyMu.RUnlock()
		} else {
			history = s.getHistoryForPlayer(clientMsg.User)
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"type":    "history",
			"success": true,
			"history": history,
		})
		client.send(resp)
		return true
	}

	// Players only see their own share of the game state; the admin sees
	// everything, and so does everyone in an open game
	if msgType.Type == "get_state" && !s.isAdmin(clientMsg.User) && !s.api.IsOpenGame() {
		response, err := s.api.GetStateForPlayer(clientMsg.User)
		if err != nil {
			log.Printf("Engine error: %v", err)
			client.sendError(err.Error())
			return true
		}
		if err := client.send(response); err != nil {
			log.Printf("Write error: %v", err)
			return false
		}
		return true
	}

	// Handle advance separately to record state snapshot
	if msgType.Type == "advance" {
		// Capture the old phase/turn before advancing so the snapshot matches the actions
		oldEngineState := s.api.GetEngine().GetState()
		oldPhase := oldEngineState.Phase.String()
		oldTurn := oldEngineState.Turn
		// Kept whole, not just the phase and turn: the projector board tells
		// each beat as a before -> after change, so it needs the board as it
		// stood before this phase was resolved
		boardBefore := oldEngineState.Clone()

		response, err := s.api.ProcessMessage(clientMsg.Payload)
		if err != nil {
			log.Printf("Engine error: %v", err)
			client.sendError(err.Error())
			return true
		}

		// Parse the response to get the new state
		var advanceResp struct {
			Success bool                `json:"success"`
			State   *jsonapi.StateJSON  `json:"state"`
			Events  []jsonapi.EventJSON `json:"events"`
		}
		if err := json.Unmarshal(response, &advanceResp); err == nil && advanceResp.Success {
			// Record snapshot keyed to the phase that just ended
			s.historyMu.Lock()
			snapshot := StateSnapshot{
				Turn:      oldTurn,
				Phase:     oldPhase,
				State:     advanceResp.State,
				Events:    advanceResp.Events,
				Timestamp: time.Now(),
			}
			s.history.StateSnapshots = append(s.history.StateSnapshots, snapshot)
			if oldPhase == "war" {
				s.history.Actions = append(s.history.Actions, republicAttacks(advanceResp.Events, oldTurn)...)
			}

			// Update phase start index to current length (new phase begins)
			s.history.PhaseStartIdx = len(s.history.Actions)
			s.historyMu.Unlock()

			// Give the projector board the beats of the phase that just ended
			s.board.RecordPhase(oldTurn, oldPhase, advanceResp.Events, boardBefore, s.api.GetEngine().GetState())

			// Nobody is ready for the new phase yet
			s.clearReady()
			s.broadcastConnectedPlayers()

			// Broadcast updated history to admin
			s.broadcastHistoryToAdmin()

			// Broadcast updated history to players (they get old version)
			s.broadcastHistoryToPlayers()

			// Save to markdown file
			go s.saveHistoryToMarkdown()
		}

		if err := client.send(response); err != nil {
			log.Printf("Write error: %v", err)
			return false
		}
		return true
	}

	// Handle submit separately to record action
	if msgType.Type == "submit" {
		response, err := s.api.ProcessMessage(clientMsg.Payload)
		if err != nil {
			log.Printf("Engine error: %v", err)
			client.sendError(err.Error())
			return true
		}

		// Parse the response to check if action was successful
		var submitResp struct {
			Success bool               `json:"success"`
			Action  jsonapi.ActionJSON `json:"action"`
		}
		if err := json.Unmarshal(response, &submitResp); err == nil && submitResp.Success {
			// Record the successful action
			state := s.api.GetEngine().GetState()
			s.historyMu.Lock()
			entry := ActionEntry{
				PlayerID:  submitResp.Action.PlayerID,
				Action:    submitResp.Action,
				Turn:      state.Turn,
				Phase:     state.Phase.String(),
				Timestamp: time.Now(),
			}
			s.history.Actions = append(s.history.Actions, entry)
			s.historyMu.Unlock()

			// A player who changes a move is no longer done
			s.setReady(entry.PlayerID, false)
			s.broadcastConnectedPlayers()

			// Broadcast updated history to admin, and in an open game to everyone
			s.broadcastHistoryToAdmin()
			if s.api.IsOpenGame() {
				s.broadcastHistoryToPlayers()
			}
		}

		if err := client.send(response); err != nil {
			log.Printf("Write error: %v", err)
			return false
		}
		return true
	}

	// Process all other engine messages normally
	response, err := s.api.ProcessMessage(clientMsg.Payload)
	if err != nil {
		log.Printf("Engine error: %v", err)
		client.sendError(err.Error())
		return true
	}

	// Opening or closing the game changes what every player may see
	if msgType.Type == "set_settings" {
		s.broadcastHistoryToPlayers()
	}

	// A player who cancels their moves is no longer done
	if msgType.Type == "cancel_actions" {
		s.setReady(clientMsg.User, false)
		s.broadcastConnectedPlayers()
	}

	if err := client.send(response); err != nil {
		log.Printf("Write error: %v", err)
		return false
	}
	return true
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name   string `json:"name"`
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Secret == "" {
		http.Error(w, "Name and secret required", http.StatusBadRequest)
		return
	}

	if req.Name == "admin" {
		http.Error(w, "Cannot register as admin", http.StatusForbidden)
		return
	}

	if err := s.registerUser(req.Name, req.Secret); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	log.Printf("User registered: %s", req.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name   string `json:"name"`
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Secret == "" {
		http.Error(w, "Name and secret required", http.StatusBadRequest)
		return
	}

	if !s.authenticateUser(req.Name, req.Secret) {
		http.Error(w, "Invalid username or secret", http.StatusUnauthorized)
		return
	}

	log.Printf("User logged in: %s", req.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func main() {
	server := NewServer()

	// Create admin user with fixed secret
	adminSecret := "crown"
	server.users["admin"] = &User{Name: "admin", Secret: adminSecret}
	fmt.Printf("Admin secret: %s\n", adminSecret)

	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	// API endpoints
	http.HandleFunc("/register", server.handleRegister)
	http.HandleFunc("/login", server.handleLogin)
	http.HandleFunc("/ws", server.handleWebSocket)

	// What the public projector board at static/board.html polls for
	http.HandleFunc("/board.json", server.handleBoard)

	addr := ":8080"
	log.Printf("Server starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
