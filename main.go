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

func NewServer() *Server {
	gameName := time.Now().Format("2006-01-02_15-04-05")
	return &Server{
		users:   make(map[string]*User),
		clients: make(map[*ClientConn]string),
		api:     jsonapi.NewGameAPIWithDice(engine.NewRandomDice()),
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
	case "get_state", "get_players", "get_connected_players", "get_history":
		return true
	case "get_actions", "get_queued", "cancel_actions":
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
	case "add_country", "add_merchant", "advance":
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

	msg, _ := json.Marshal(map[string]interface{}{
		"type":    "connected_players",
		"success": true,
		"players": names,
	})

	for client := range s.clients {
		client.send(msg)
	}
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

	playerHistory := s.getHistoryForPlayer()
	msg, _ := json.Marshal(map[string]interface{}{
		"type":    "history_update",
		"success": true,
		"history": playerHistory,
	})

	for client, username := range s.clients {
		if username != "admin" {
			client.send(msg)
		}
	}
}

func (s *Server) getHistoryForPlayer() interface{} {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	// Players get actions only from the beginning of current phase
	playerActions := s.history.Actions[:s.history.PhaseStartIdx]

	// Players get state snapshots up to (but not including) the current phase
	playerSnapshots := s.history.StateSnapshots
	if len(playerSnapshots) > 0 {
		// Remove the most recent snapshot if it's from the current phase
		currentPhase := s.api.GetEngine().GetState().Phase.String()
		if playerSnapshots[len(playerSnapshots)-1].Phase == currentPhase {
			playerSnapshots = playerSnapshots[:len(playerSnapshots)-1]
		}
	}

	return map[string]interface{}{
		"game_name":       s.history.GameName,
		"actions":         playerActions,
		"state_snapshots": playerSnapshots,
		"phase_start_idx": s.history.PhaseStartIdx,
	}
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
				fmt.Fprintf(f, "- **%s** in %s: Stored=%d, Invested=%d\n",
					merchant.PlayerID, merchant.CountryID,
					merchant.StoredGold, merchant.InvestedGold)
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
		return "Hide Gold"
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

		// Handle non-engine messages separately
		var msgType struct {
			Type string `json:"type"`
		}
		json.Unmarshal(clientMsg.Payload, &msgType)

		if msgType.Type == "get_connected_players" {
			names := s.getConnectedPlayerNames()
			resp, _ := json.Marshal(map[string]interface{}{
				"type":    "connected_players",
				"success": true,
				"players": names,
			})
			client.send(resp)
			continue
		}

		if msgType.Type == "get_history" {
			var history interface{}
			if clientMsg.User == "admin" {
				s.historyMu.RLock()
				history = s.history
				s.historyMu.RUnlock()
			} else {
				history = s.getHistoryForPlayer()
			}
			resp, _ := json.Marshal(map[string]interface{}{
				"type":    "history",
				"success": true,
				"history": history,
			})
			client.send(resp)
			continue
		}

		// Handle advance separately to record state snapshot
		if msgType.Type == "advance" {
			// Capture the old phase/turn before advancing so the snapshot matches the actions
			oldEngineState := s.api.GetEngine().GetState()
			oldPhase := oldEngineState.Phase.String()
			oldTurn := oldEngineState.Turn

			response, err := s.api.ProcessMessage(clientMsg.Payload)
			if err != nil {
				log.Printf("Engine error: %v", err)
				client.sendError(err.Error())
				continue
			}

			// Parse the response to get the new state
			var advanceResp struct {
				Success bool               `json:"success"`
				State   *jsonapi.StateJSON `json:"state"`
			}
			if err := json.Unmarshal(response, &advanceResp); err == nil && advanceResp.Success {
				// Record snapshot keyed to the phase that just ended
				s.historyMu.Lock()
				snapshot := StateSnapshot{
					Turn:      oldTurn,
					Phase:     oldPhase,
					State:     advanceResp.State,
					Timestamp: time.Now(),
				}
				s.history.StateSnapshots = append(s.history.StateSnapshots, snapshot)

				// Update phase start index to current length (new phase begins)
				s.history.PhaseStartIdx = len(s.history.Actions)
				s.historyMu.Unlock()

				// Broadcast updated history to admin
				s.broadcastHistoryToAdmin()

				// Broadcast updated history to players (they get old version)
				s.broadcastHistoryToPlayers()

				// Save to markdown file
				go s.saveHistoryToMarkdown()
			}

			if err := client.send(response); err != nil {
				log.Printf("Write error: %v", err)
				break
			}
			continue
		}

		// Handle submit separately to record action
		if msgType.Type == "submit" {
			response, err := s.api.ProcessMessage(clientMsg.Payload)
			if err != nil {
				log.Printf("Engine error: %v", err)
				client.sendError(err.Error())
				continue
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

				// Broadcast updated history to admin
				s.broadcastHistoryToAdmin()
			}

			if err := client.send(response); err != nil {
				log.Printf("Write error: %v", err)
				break
			}
			continue
		}

		// Process all other engine messages normally
		response, err := s.api.ProcessMessage(clientMsg.Payload)
		if err != nil {
			log.Printf("Engine error: %v", err)
			client.sendError(err.Error())
			continue
		}

		if err := client.send(response); err != nil {
			log.Printf("Write error: %v", err)
			break
		}
	}
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

	addr := ":8080"
	log.Printf("Server starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
