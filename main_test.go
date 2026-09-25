package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// The real server with 30 players connected at once, all acting at the same
// moment, as they will in a live game.

type wsPlayer struct {
	t      *testing.T
	name   string
	conn   *websocket.Conn
	inbox  chan map[string]any
	secret string
}

func startServer(t *testing.T) (*Server, string) {
	t.Chdir(t.TempDir()) // The server writes the game history next to itself
	server := NewServer()
	server.users["admin"] = &User{Name: "admin", Secret: "crown"}
	mux := http.NewServeMux()
	mux.HandleFunc("/register", server.handleRegister)
	mux.HandleFunc("/login", server.handleLogin)
	mux.HandleFunc("/ws", server.handleWebSocket)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return server, ts.URL
}

func connect(t *testing.T, url, name, secret string, register bool) *wsPlayer {
	t.Helper()
	if register {
		body, _ := json.Marshal(map[string]string{"name": name, "secret": secret})
		resp, err := http.Post(url+"/register", "application/json", bytes.NewReader(body))
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("registering %s failed: %v %v", name, err, resp)
		}
		resp.Body.Close()
	}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(url, "http")+"/ws", nil)
	if err != nil {
		t.Fatalf("%s cannot connect: %v", name, err)
	}
	t.Cleanup(func() { conn.Close() })
	p := &wsPlayer{t: t, name: name, conn: conn, inbox: make(chan map[string]any, 1000), secret: secret}
	go func() {
		for {
			var msg map[string]any
			if err := conn.ReadJSON(&msg); err != nil {
				close(p.inbox)
				return
			}
			p.inbox <- msg
		}
	}()
	return p
}

// ask sends a request and waits for its answer, skipping the updates the
// server broadcasts to everyone
func (p *wsPlayer) ask(payload map[string]any) map[string]any {
	if err := p.conn.WriteJSON(map[string]any{"user": p.name, "secret": p.secret, "payload": payload}); err != nil {
		p.t.Errorf("%s cannot send: %v", p.name, err)
		return nil
	}
	timeout := time.After(10 * time.Second)
	for {
		select {
		case msg, ok := <-p.inbox:
			if !ok {
				p.t.Errorf("the server closed %s's connection", p.name)
				return nil
			}
			if msg["type"] == "connected_players" || msg["type"] == "history_update" {
				continue
			}
			return msg
		case <-timeout:
			p.t.Errorf("%s got no answer to %v", p.name, payload)
			return nil
		}
	}
}

func TestThirtyPlayersConnectedAtOnce(t *testing.T) {
	server, url := startServer(t)
	admin := connect(t, url, "admin", "crown", false)

	// 7 countries, 30 players: teams of 5, 5, 4, 4, 4, 4 and 4
	countries := []string{"Avalon", "Brittany", "Castile", "Dalmatia", "Estonia", "Flanders", "Galicia"}
	var names []string
	home := map[string]string{}
	isMonarch := map[string]bool{}
	merchant := 0
	for i, c := range countries {
		monarch := fmt.Sprintf("monarch-%d", i+1)
		admin.ask(map[string]any{"type": "add_country", "country_id": c, "monarch_id": monarch})
		names = append(names, monarch)
		home[monarch], isMonarch[monarch] = c, true
		count := 3
		if i < 2 {
			count = 4
		}
		for j := 0; j < count; j++ {
			merchant++
			id := fmt.Sprintf("merchant-%02d", merchant)
			admin.ask(map[string]any{"type": "add_merchant", "player_id": id, "country_id": c})
			names = append(names, id)
			home[id] = c
		}
	}

	// All 30 register and connect at the same time
	players := make([]*wsPlayer, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			players[i] = connect(t, url, name, "secret-"+name, true)
		}()
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	everyone := func(do func(p *wsPlayer)) {
		var wg sync.WaitGroup
		for _, p := range players {
			wg.Add(1)
			go func() {
				defer wg.Done()
				do(p)
			}()
		}
		wg.Wait()
	}

	resp := admin.ask(map[string]any{"type": "get_players"})
	if list, _ := resp["players"].(map[string]any); len(list) != 30 {
		t.Fatalf("the server lists %d players, expected 30", len(list))
	}

	// Nobody but the admin may run the game or act for someone else
	everyone(func(p *wsPlayer) {
		other := "monarch-2"
		if p.name == other {
			other = "monarch-3"
		}
		for _, payload := range []map[string]any{
			{"type": "advance"},
			{"type": "set_settings", "open_game": true},
			{"type": "assign_role", "player_id": p.name, "role": "monarch", "country_id": "Avalon"},
			{"type": "get_actions", "player_id": other},
			{"type": "cancel_actions", "player_id": other},
			{"type": "submit", "action": map[string]any{"type": "tax_peasants_high", "player_id": other, "country_id": home[other]}},
		} {
			if r := p.ask(payload); r == nil || r["success"] != false {
				t.Errorf("%s was allowed %v: %v", p.name, payload, r)
			}
		}
	})
	if r := players[1].ask(map[string]any{"type": "get_state"}); r != nil {
		wrong := players[1]
		wrong.secret = "guess"
		if r := wrong.ask(map[string]any{"type": "get_state"}); r == nil || r["error"] != "authentication failed" {
			t.Errorf("a wrong password was accepted: %v", r)
		}
		wrong.secret = "secret-" + wrong.name
	}

	// Play several rounds. In every phase all 30 players look at the game
	// and send their moves at the same moment.
	for round := 1; round <= 5; round++ {
		for phase := 0; phase < 5; phase++ {
			state := admin.ask(map[string]any{"type": "get_state"})["state"].(map[string]any)
			phaseName := state["phase"].(string)

			var mu sync.Mutex
			accepted := 0
			everyone(func(p *wsPlayer) {
				// Everyone checks the board, then acts
				view := p.ask(map[string]any{"type": "get_state"})
				if view == nil {
					return
				}
				checkPlayerView(t, p.name, view)
				actions, _ := p.ask(map[string]any{"type": "get_actions", "player_id": p.name})["actions"].([]any)
				move := pickMove(p.name, phaseName, actions)
				if move == nil {
					return
				}
				r := p.ask(map[string]any{"type": "submit", "action": move})
				if r == nil || r["success"] != true {
					t.Errorf("round %d %s: %s's move %v was refused: %v", round, phaseName, p.name, move, r)
					return
				}
				mu.Lock()
				accepted++
				mu.Unlock()
				p.ask(map[string]any{"type": "get_history"})
			})

			// Every accepted move must be queued: none lost in the crowd
			queued, _ := admin.ask(map[string]any{"type": "get_queued"})["actions"].([]any)
			if len(queued) != accepted {
				t.Errorf("round %d %s: %d moves were accepted but %d are queued", round, phaseName, accepted, len(queued))
			}
			server.historyMu.RLock()
			recorded := len(server.history.Actions) - server.history.PhaseStartIdx
			server.historyMu.RUnlock()
			if recorded != accepted {
				t.Errorf("round %d %s: %d moves were accepted but %d are in the game history", round, phaseName, accepted, recorded)
			}

			if r := admin.ask(map[string]any{"type": "advance"}); r == nil || r["success"] != true {
				t.Fatalf("round %d: advancing from %s failed: %v", round, phaseName, r)
			}
		}
	}

	state := admin.ask(map[string]any{"type": "get_state"})["state"].(map[string]any)
	if state["turn"].(float64) != 6 {
		t.Errorf("after 5 rounds the game should be in round 6, it is in %v", state["turn"])
	}
}

// checkPlayerView makes sure a player's view of the board keeps other
// countries' treasuries and other merchants' hidden gold secret
func checkPlayerView(t *testing.T, name string, view map[string]any) {
	state, _ := view["state"].(map[string]any)
	if state == nil {
		t.Errorf("%s got no state: %v", name, view)
		return
	}
	merchants, _ := state["merchants"].(map[string]any)
	own := ""
	if me, ok := merchants[name].(map[string]any); ok {
		own, _ = me["country_id"].(string)
	}
	countries, _ := state["countries"].(map[string]any)
	for id, c := range countries {
		c := c.(map[string]any)
		if c["monarch_id"] == name {
			own = id
		}
	}
	for id, c := range countries {
		c := c.(map[string]any)
		if id != own && (c["gold"].(float64) != 0 || c["hidden"] != true) {
			t.Errorf("%s can see inside %s: %v", name, id, c)
		}
	}
	for id, m := range merchants {
		m := m.(map[string]any)
		if id != name && (m["hidden_gold"] != nil || m["invested_gold"].(float64) != 0) {
			t.Errorf("%s can see %s's secret gold: %v", name, id, m)
		}
	}
}

// pickMove chooses a simple legal move from a player's menu: monarchs tax
// high, build an army and attack their neighbour; merchants invest a coin
// and remain loyal
func pickMove(name, phase string, menu []any) map[string]any {
	find := func(actionType string) map[string]any {
		for _, a := range menu {
			if a := a.(map[string]any); a["type"] == actionType {
				return a
			}
		}
		return nil
	}
	var move map[string]any
	switch phase {
	case "taxation":
		move = find("tax_peasants_high")
		if move == nil {
			move = find("vote_tax_high")
		}
	case "spending":
		if move = find("build_army"); move == nil {
			move = find("merchant_invest")
		}
		if move != nil {
			move["amount"] = 1
		}
	case "war":
		move = find("attack")
		if move == nil {
			move = find("vote_no_attack")
		}
	case "assessment":
		move = find("remain")
	}
	return move
}
