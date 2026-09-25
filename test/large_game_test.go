package test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
)

// The large game: 30 players split into 7 teams (one monarch and three or
// four merchants each), played through the JSON API the way the web page
// does. Every player picks legal moves at random, and after every phase the
// state is checked against the rules.

var bigCountries = []string{"Avalon", "Brittany", "Castile", "Dalmatia", "Estonia", "Flanders", "Galicia"}

const (
	bigPlayers   = 30
	bigMaxRounds = 40
)

type bigGame struct {
	t    *testing.T
	seed int64
	api  *jsonapi.GameAPI
	rng  *rand.Rand

	players []string
	gone    map[string]bool // Players who legitimately left the game
	stats   map[string]int

	declared map[[2]string]bool // Attacks declared this War phase
	choices  map[string]string  // Assessment choices this round
	fleeTo   map[string]string
	failures int
}

func (b *bigGame) state() *engine.GameState { return b.api.GetEngine().GetState() }

func (b *bigGame) where() string {
	s := b.state()
	return fmt.Sprintf("seed %d, round %d, %s", b.seed, s.Turn, s.Phase)
}

// fail records a problem, keeping the log short if something breaks badly
func (b *bigGame) fail(format string, args ...any) {
	b.t.Helper()
	b.failures++
	if b.failures <= 25 {
		b.t.Errorf("%s: %s", b.where(), fmt.Sprintf(format, args...))
	}
	if b.failures == 25 {
		b.t.Errorf("%s: too many problems, not reporting any more for this game", b.where())
	}
}

func (b *bigGame) request(req any) []byte {
	b.t.Helper()
	data, _ := json.Marshal(req)
	resp, err := b.api.ProcessMessage(data)
	if err != nil {
		b.t.Fatalf("%s: ProcessMessage error: %v", b.where(), err)
	}
	return resp
}

// newBigGame sets up 7 countries and 30 players through the API. The first
// two countries get 4 merchants and the rest 3, so the teams are 5, 5, 4, 4,
// 4, 4 and 4 players strong.
func newBigGame(t *testing.T, seed int64) *bigGame {
	b := &bigGame{
		t:     t,
		seed:  seed,
		api:   jsonapi.NewGameAPIWithDice(engine.NewSeededDice(seed)),
		rng:   rand.New(rand.NewSource(seed)),
		gone:  map[string]bool{},
		stats: map[string]int{},
	}

	ok := func(resp []byte, what string) {
		var r struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		json.Unmarshal(resp, &r)
		if !r.Success {
			t.Fatalf("setup: %s failed: %s", what, r.Error)
		}
	}

	merchant := 0
	for i, country := range bigCountries {
		monarch := fmt.Sprintf("monarch-%d", i+1)
		ok(b.request(map[string]any{"type": "add_country", "country_id": country, "monarch_id": monarch}), "adding "+country)
		b.players = append(b.players, monarch)
		count := 3
		if i < 2 {
			count = 4
		}
		for j := 0; j < count; j++ {
			merchant++
			id := fmt.Sprintf("merchant-%02d", merchant)
			ok(b.request(map[string]any{"type": "add_merchant", "player_id": id, "country_id": country}), "adding "+id)
			b.players = append(b.players, id)
		}
	}
	if len(b.players) != bigPlayers {
		t.Fatalf("setup made %d players, expected %d", len(b.players), bigPlayers)
	}
	return b
}

// ---------------------------------------------------------------------------
// Talking to the API

func (b *bigGame) menu(playerID string) []jsonapi.ActionJSON {
	b.t.Helper()
	var resp jsonapi.ActionsResponse
	json.Unmarshal(b.request(map[string]any{"type": "get_actions", "player_id": playerID}), &resp)
	if !resp.Success {
		b.fail("get_actions failed for %s", playerID)
	}
	if resp.Phase != b.state().Phase.String() {
		b.fail("get_actions reports phase %q, the game is in %s", resp.Phase, b.state().Phase)
	}
	// The game lists some moves in no particular order; sort them so a seed
	// always replays the same game
	sort.SliceStable(resp.Actions, func(i, j int) bool {
		return fmt.Sprint(resp.Actions[i]) < fmt.Sprint(resp.Actions[j])
	})
	return resp.Actions
}

func (b *bigGame) submitRaw(action jsonapi.ActionJSON) (bool, string) {
	var resp jsonapi.SubmitResponse
	json.Unmarshal(b.request(map[string]any{"type": "submit", "action": action}), &resp)
	return resp.Success, resp.RejectionReason
}

// submit sends a move that must be accepted
func (b *bigGame) submit(action jsonapi.ActionJSON) {
	b.t.Helper()
	if ok, why := b.submitRaw(action); !ok {
		b.fail("legal move %+v by %s was refused: %s", action, action.PlayerID, why)
	}
}

// refuse sends a move that must be refused
func (b *bigGame) refuse(action jsonapi.ActionJSON, what string) {
	b.t.Helper()
	b.stats["illegal moves tried"]++
	if ok, _ := b.submitRaw(action); ok {
		b.fail("illegal move was accepted (%s): %+v", what, action)
		b.request(map[string]any{"type": "cancel_actions", "player_id": action.PlayerID})
	}
}

// maxAmount reads N out of a placeholder amount like "<AMOUNT:0-N>"
func maxAmount(a jsonapi.ActionJSON) int {
	s, _ := a.Amount.(string)
	i := strings.LastIndex(s, "-")
	n, err := strconv.Atoi(strings.TrimSuffix(s[i+1:], ">"))
	if i < 0 || err != nil {
		return -1
	}
	return n
}

func withAmount(a jsonapi.ActionJSON, amount int) jsonapi.ActionJSON {
	a.Amount = amount
	return a
}

func menuTypes(menu []jsonapi.ActionJSON) map[string]int {
	types := map[string]int{}
	for _, a := range menu {
		types[a.Type]++
	}
	return types
}

func ofType(menu []jsonapi.ActionJSON, actionType string) []jsonapi.ActionJSON {
	var found []jsonapi.ActionJSON
	for _, a := range menu {
		if a.Type == actionType {
			found = append(found, a)
		}
	}
	return found
}

// ---------------------------------------------------------------------------
// Who is who

type role int

const (
	roleNone role = iota
	roleMonarch
	roleMerchant
)

func (b *bigGame) roleOf(playerID string) (role, *engine.Country, *engine.Merchant) {
	s := b.state()
	if m := s.GetMerchant(playerID); m != nil {
		return roleMerchant, s.GetCountry(m.CountryID), m
	}
	for _, c := range s.Countries {
		if c.MonarchID == playerID && !c.IsRepublic && c.IsAlive() {
			return roleMonarch, c, nil
		}
	}
	return roleNone, nil, nil
}

func (b *bigGame) aliveOthers(countryID string) []string {
	ids := b.state().GetAliveCountryIDsExcept(countryID)
	sort.Strings(ids)
	return ids
}

func (b *bigGame) shuffledPlayers() []string {
	order := append([]string(nil), b.players...)
	b.rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	return order
}

func (b *bigGame) chance(percent int) bool { return b.rng.Intn(100) < percent }

// ---------------------------------------------------------------------------
// Playing the phases

func (b *bigGame) playRound() {
	for i := 0; i < 5; i++ {
		s := b.state()
		pre := s.Clone()
		phase := s.Phase
		switch phase {
		case engine.PhaseTaxation:
			b.playTaxation()
		case engine.PhaseNegotiation:
			for _, p := range b.players {
				if menu := b.menu(p); len(menu) != 0 {
					b.fail("%s is offered %d moves during negotiation", p, len(menu))
				}
			}
		case engine.PhaseSpending:
			b.playSpending()
		case engine.PhaseWar:
			b.playWar()
		case engine.PhaseAssessment:
			b.playAssessment()
		}
		b.tryIllegalMoves()

		var resp struct {
			Success       bool                `json:"success"`
			PreviousPhase string              `json:"previous_phase"`
			CurrentPhase  string              `json:"current_phase"`
			Turn          int                 `json:"turn"`
			Events        []jsonapi.EventJSON `json:"events"`
		}
		json.Unmarshal(b.request(map[string]any{"type": "advance"}), &resp)
		if !resp.Success || resp.PreviousPhase != phase.String() {
			b.fail("advance from %s failed: %+v", phase, resp)
		}
		for _, e := range resp.Events {
			b.stats["event: "+e.Type]++
		}
		b.check(pre, phase, resp.Events)
		if b.failures >= 25 {
			return
		}
		if len(b.state().GetAliveCountries()) == 0 {
			return // Every country is destroyed: the game is over
		}
	}
}

func (b *bigGame) playTaxation() {
	for _, p := range b.shuffledPlayers() {
		menu := b.menu(p)
		r, country, merchant := b.roleOf(p)
		switch {
		case r == roleMonarch:
			types := menuTypes(menu)
			present := b.state().GetMerchantsByCountry(country.ID)
			if types["tax_peasants_low"] != 1 || types["tax_peasants_high"] != 1 || types["tax_merchants"] != len(present) {
				b.fail("monarch %s of %s has a wrong tax menu %v for %d merchants", p, country.ID, types, len(present))
			}
			switch n := b.rng.Intn(10); {
			case n < 2: // Chooses nothing, which means low tax
			case n < 6:
				b.submit(ofType(menu, "tax_peasants_low")[0])
			default:
				b.submit(ofType(menu, "tax_peasants_high")[0])
			}
			for _, a := range ofType(menu, "tax_merchants") {
				max := maxAmount(a)
				if want := b.state().GetMerchant(a.MerchantID).StoredGold; max != want {
					b.fail("%s may tax %s up to %d, but the purse holds %d", p, a.MerchantID, max, want)
				}
				if b.chance(50) {
					b.submit(withAmount(a, b.rng.Intn(max+1)))
				}
			}
		case r == roleMerchant && !merchant.Arriving && country.IsRepublic && country.IsAlive():
			types := menuTypes(menu)
			if len(menu) != 2 || types["vote_tax_low"] != 1 || types["vote_tax_high"] != 1 {
				b.fail("republic merchant %s has a wrong tax menu %v", p, types)
			}
			if b.chance(80) {
				b.submit(menu[b.rng.Intn(2)])
			}
		default:
			if len(menu) != 0 {
				b.fail("%s should have nothing to do in taxation, is offered %v", p, menuTypes(menu))
			}
		}
	}
}

func (b *bigGame) playSpending() {
	for _, p := range b.shuffledPlayers() {
		menu := b.menu(p)
		r, country, merchant := b.roleOf(p)
		switch {
		case r == roleMonarch:
			gold := country.Gold
			types := menuTypes(menu)
			wantGifts := 0
			if gold > 0 {
				wantGifts = len(b.state().GetMerchantsByCountry(country.ID))
			}
			if (gold > 0) != (types["build_army"] == 1) || types["monarch_invest"] != wantGifts {
				b.fail("monarch %s with %d gold has a wrong spending menu %v", p, gold, types)
			}
			if gold == 0 {
				continue
			}
			army := 0
			switch n := b.rng.Intn(10); {
			case n < 3:
				army = gold
			case n < 8:
				army = b.rng.Intn(gold + 1)
			}
			if army > 0 {
				b.submit(withAmount(ofType(menu, "build_army")[0], army))
			}
			left := gold - army
			for _, gift := range ofType(menu, "monarch_invest") {
				if left > 0 && b.chance(25) {
					amount := 1 + b.rng.Intn(left)
					left -= amount
					b.submit(withAmount(gift, amount))
				}
			}
		case r == roleMerchant && !merchant.Arriving && country.IsAlive():
			b.merchantSpends(p, menu, country, merchant)
		default:
			if len(menu) != 0 {
				b.fail("%s should have nothing to do in spending, is offered %v", p, menuTypes(menu))
			}
		}
	}
}

// merchantSpends picks an affordable mix of hiding, investing, unhiding and
// (in a republic) paying for the army, and queues it in a random order: the
// game carries the moves out in its own fixed order. Gold unhidden this round
// cannot be invested until the next, and trying to must be refused.
func (b *bigGame) merchantSpends(p string, menu []jsonapi.ActionJSON, country *engine.Country, m *engine.Merchant) {
	purse, hidden := m.StoredGold, m.HiddenGold
	types := menuTypes(menu)
	wantTypes := map[string]int{}
	if purse > 0 {
		wantTypes["merchant_invest"] = 1
		wantTypes["merchant_hide"] = 1
	}
	if hidden > 0 {
		wantTypes["merchant_unhide"] = 1
	}
	if country.IsRepublic && purse+hidden > 0 {
		wantTypes["contribute_army"] = 1
	}
	if fmt.Sprint(types) != fmt.Sprint(wantTypes) {
		b.fail("merchant %s (purse %d, hidden %d, republic %v) is offered %v, expected %v", p, purse, hidden, country.IsRepublic, types, wantTypes)
		return
	}
	if purse > 0 && maxAmount(ofType(menu, "merchant_invest")[0]) != purse {
		b.fail("merchant %s may invest up to %s, but only %d is in the purse", p, ofType(menu, "merchant_invest")[0].Amount, purse)
	}

	var moves []jsonapi.ActionJSON
	unhide := 0
	if hidden > 0 && b.chance(30) {
		unhide = 1 + b.rng.Intn(hidden)
		moves = append(moves, withAmount(ofType(menu, "merchant_unhide")[0], unhide))
	}
	inPurse := purse
	if inPurse > 0 && b.chance(40) {
		hide := 1 + b.rng.Intn(inPurse)
		inPurse -= hide
		moves = append(moves, withAmount(ofType(menu, "merchant_hide")[0], hide))
	}
	invest := 0
	if inPurse > 0 && b.chance(50) {
		invest = 1 + b.rng.Intn(inPurse)
		inPurse -= invest
		moves = append(moves, withAmount(ofType(menu, "merchant_invest")[0], invest))
	}
	if country.IsRepublic {
		if left := purse + hidden - invest; left > 0 && b.chance(60) {
			moves = append(moves, withAmount(ofType(menu, "contribute_army")[0], 1+b.rng.Intn(left)))
		}
	}
	b.rng.Shuffle(len(moves), func(i, j int) { moves[i], moves[j] = moves[j], moves[i] })
	for _, move := range moves {
		b.submit(move)
	}
	if unhide > 0 {
		b.stats["merchants unhiding gold"]++
		b.refuse(jsonapi.ActionJSON{Type: "merchant_invest", PlayerID: p, MerchantID: p, Amount: inPurse + 1}, "investing gold unhidden this same round")
	}
}

func (b *bigGame) playWar() {
	b.declared = map[[2]string]bool{}
	favourite := map[string]string{} // What each republic's merchants mostly vote for
	for _, p := range b.shuffledPlayers() {
		menu := b.menu(p)
		r, country, merchant := b.roleOf(p)
		switch {
		case r == roleMonarch:
			var targets []string
			for _, a := range ofType(menu, "attack") {
				targets = append(targets, a.TargetID)
			}
			sort.Strings(targets)
			if len(ofType(menu, "no_attack")) != 1 || strings.Join(targets, ",") != strings.Join(b.aliveOthers(country.ID), ",") {
				b.fail("monarch %s of %s may attack %v, expected %v", p, country.ID, targets, b.aliveOthers(country.ID))
			}
			switch n := b.rng.Intn(20); {
			case n < 9 && len(targets) > 0:
				a := ofType(menu, "attack")[b.rng.Intn(len(targets))]
				b.submit(a)
				b.declared[[2]string{country.ID, a.TargetID}] = true
			case n < 15:
				b.submit(ofType(menu, "no_attack")[0])
			}
		case r == roleMerchant && !merchant.Arriving && country.IsRepublic && country.IsAlive():
			attacks := ofType(menu, "vote_attack")
			if len(ofType(menu, "vote_no_attack")) != 1 || len(attacks) != len(b.aliveOthers(country.ID)) {
				b.fail("republic merchant %s has a wrong war menu %v", p, menuTypes(menu))
			}
			if len(attacks) == 0 {
				continue
			}
			if _, ok := favourite[country.ID]; !ok {
				favourite[country.ID] = attacks[b.rng.Intn(len(attacks))].TargetID
			}
			switch n := b.rng.Intn(20); {
			case n < 13:
				b.submit(jsonapi.ActionJSON{Type: "vote_attack", PlayerID: p, MerchantID: p, TargetID: favourite[country.ID]})
			case n < 16:
				b.submit(ofType(menu, "vote_no_attack")[0])
			case n < 18:
				b.submit(attacks[b.rng.Intn(len(attacks))])
			}
		default:
			if len(menu) != 0 {
				b.fail("%s should have nothing to do in war, is offered %v", p, menuTypes(menu))
			}
		}
	}
}

func (b *bigGame) playAssessment() {
	b.choices = map[string]string{}
	b.fleeTo = map[string]string{}
	mood := map[string]string{}
	for _, id := range b.aliveOthers("") {
		c := b.state().GetCountry(id)
		n := b.rng.Intn(100)
		switch {
		case !c.IsRepublic && n < 25:
			mood[c.ID] = "revolt"
		case n > 91:
			mood[c.ID] = "exodus"
		default:
			mood[c.ID] = "calm"
		}
	}

	for _, p := range b.shuffledPlayers() {
		menu := b.menu(p)
		r, country, merchant := b.roleOf(p)
		if r != roleMerchant || merchant.Arriving || !country.IsAlive() {
			if len(menu) != 0 {
				b.fail("%s should have nothing to do in the assessment, is offered %v", p, menuTypes(menu))
			}
			continue
		}
		types := menuTypes(menu)
		wantRevolt := 1
		if country.IsRepublic {
			wantRevolt = 0
		}
		flees := ofType(menu, "flee")
		if types["remain"] != 1 || types["revolt"] != wantRevolt || len(flees) != len(b.aliveOthers(country.ID)) {
			b.fail("merchant %s of %s has a wrong assessment menu %v", p, country.ID, types)
		}

		choice := "remain"
		n := b.rng.Intn(100)
		switch mood[country.ID] {
		case "calm":
			if n >= 85 {
				choice = ""
			}
			if n >= 95 {
				choice = "flee"
			}
		case "revolt":
			if n < 70 {
				choice = "revolt"
			} else if n >= 90 {
				choice = "flee"
			}
		case "exodus":
			if n < 85 {
				choice = "flee"
			}
		}
		if choice == "flee" && len(flees) == 0 {
			choice = "remain"
		}
		b.choices[p] = choice
		switch choice {
		case "remain":
			b.submit(ofType(menu, "remain")[0])
		case "revolt":
			b.submit(ofType(menu, "revolt")[0])
		case "flee":
			a := flees[b.rng.Intn(len(flees))]
			b.fleeTo[p] = a.TargetID
			b.submit(a)
		}
	}
}

// tryIllegalMoves sends moves the game must refuse: acting for somebody else,
// acting out of turn, spending gold nobody has, choosing twice, and so on
func (b *bigGame) tryIllegalMoves() {
	s := b.state()
	var monarchs, merchants, arriving []string
	for _, p := range b.players {
		switch r, _, m := b.roleOf(p); {
		case r == roleMonarch:
			monarchs = append(monarchs, p)
		case r == roleMerchant && m.Arriving:
			arriving = append(arriving, p)
		case r == roleMerchant:
			merchants = append(merchants, p)
		}
	}
	var dead []string
	for id, c := range s.Countries {
		if !c.IsAlive() {
			dead = append(dead, id)
		}
	}
	sort.Strings(dead)
	pick := func(ids []string) string { return ids[b.rng.Intn(len(ids))] }

	if len(merchants) > 0 {
		m := s.GetMerchant(pick(merchants))
		b.refuse(jsonapi.ActionJSON{Type: "tax_peasants_high", PlayerID: m.ID, CountryID: m.CountryID}, "a merchant setting the peasant tax")
		b.refuse(jsonapi.ActionJSON{Type: "build_army", PlayerID: m.ID, CountryID: m.CountryID, Amount: 1}, "a merchant building the army")
		b.refuse(jsonapi.ActionJSON{Type: "attack", PlayerID: m.ID, CountryID: m.CountryID, TargetID: m.CountryID}, "a merchant declaring war")
		if other := pick(merchants); other != m.ID {
			b.refuse(jsonapi.ActionJSON{Type: "merchant_hide", PlayerID: m.ID, MerchantID: other, Amount: 1}, "hiding someone else's gold")
			b.refuse(jsonapi.ActionJSON{Type: "flee", PlayerID: m.ID, MerchantID: other, TargetID: m.CountryID}, "making someone else flee")
		}
		b.refuse(jsonapi.ActionJSON{Type: "merchant_invest", PlayerID: m.ID, MerchantID: m.ID, Amount: m.SpendableGold() + 1}, "investing more than they have")
		b.refuse(jsonapi.ActionJSON{Type: "merchant_invest", PlayerID: m.ID, MerchantID: m.ID, Amount: 0}, "investing nothing")
		b.refuse(jsonapi.ActionJSON{Type: "merchant_hide", PlayerID: m.ID, MerchantID: m.ID, Amount: -1}, "hiding a negative amount")
		b.refuse(jsonapi.ActionJSON{Type: "merchant_unhide", PlayerID: m.ID, MerchantID: m.ID, Amount: m.HiddenGold + 1}, "unhiding more than is hidden")
		b.refuse(jsonapi.ActionJSON{Type: "flee", PlayerID: m.ID, MerchantID: m.ID, TargetID: m.CountryID}, "fleeing to their own country")
		if len(dead) > 0 {
			b.refuse(jsonapi.ActionJSON{Type: "flee", PlayerID: m.ID, MerchantID: m.ID, TargetID: pick(dead)}, "fleeing to a destroyed country")
		}
		if c := s.GetCountry(m.CountryID); c.IsRepublic {
			b.refuse(jsonapi.ActionJSON{Type: "revolt", PlayerID: m.ID, MerchantID: m.ID, CountryID: m.CountryID}, "revolting in a republic")
		} else {
			b.refuse(jsonapi.ActionJSON{Type: "vote_tax_high", PlayerID: m.ID, MerchantID: m.ID}, "voting in a monarchy")
			b.refuse(jsonapi.ActionJSON{Type: "contribute_army", PlayerID: m.ID, MerchantID: m.ID, Amount: 1}, "paying for a monarch's army")
		}
	}
	if len(arriving) > 0 {
		m := s.GetMerchant(pick(arriving))
		b.refuse(jsonapi.ActionJSON{Type: "remain", PlayerID: m.ID, MerchantID: m.ID}, "a merchant on the move acting")
		b.refuse(jsonapi.ActionJSON{Type: "merchant_hide", PlayerID: m.ID, MerchantID: m.ID, Amount: 0}, "a merchant on the move hiding gold")
		c := s.GetCountry(m.CountryID)
		if c != nil && !c.IsRepublic && c.MonarchID != "" {
			b.refuse(jsonapi.ActionJSON{Type: "tax_merchants", PlayerID: c.MonarchID, CountryID: c.ID, MerchantID: m.ID, Amount: 0}, "taxing a merchant still on the move")
		}
	}
	if len(monarchs) > 0 {
		king := pick(monarchs)
		_, c, _ := b.roleOf(king)
		b.refuse(jsonapi.ActionJSON{Type: "build_army", PlayerID: king, CountryID: c.ID, Amount: c.Gold + 1}, "building more army than the treasury pays for")
		b.refuse(jsonapi.ActionJSON{Type: "build_army", PlayerID: king, CountryID: c.ID, Amount: -1}, "building a negative army")
		b.refuse(jsonapi.ActionJSON{Type: "attack", PlayerID: king, CountryID: c.ID, TargetID: c.ID}, "attacking yourself")
		if len(dead) > 0 {
			b.refuse(jsonapi.ActionJSON{Type: "attack", PlayerID: king, CountryID: c.ID, TargetID: pick(dead)}, "attacking a destroyed country")
		}
		b.refuse(jsonapi.ActionJSON{Type: "tax_merchants", PlayerID: king, CountryID: c.ID, MerchantID: "merchant-99", Amount: 1}, "taxing a merchant who does not exist")
		for _, other := range b.players {
			if m := s.GetMerchant(other); m != nil && m.CountryID != c.ID {
				b.refuse(jsonapi.ActionJSON{Type: "tax_merchants", PlayerID: king, CountryID: m.CountryID, MerchantID: m.ID, Amount: 0}, "taxing another country's merchant")
				b.refuse(jsonapi.ActionJSON{Type: "monarch_invest", PlayerID: king, CountryID: c.ID, MerchantID: m.ID, Amount: 1}, "a gift to another country's merchant")
				break
			}
		}
		for _, m := range s.GetMerchantsByCountry(c.ID) {
			b.refuse(jsonapi.ActionJSON{Type: "tax_merchants", PlayerID: king, CountryID: c.ID, MerchantID: m.ID, Amount: -1}, "a negative merchant tax")
			b.refuse(jsonapi.ActionJSON{Type: "monarch_invest", PlayerID: king, CountryID: c.ID, MerchantID: m.ID, Amount: 0}, "an empty gift")
			break
		}
		if other := pick(monarchs); other != king {
			b.refuse(jsonapi.ActionJSON{Type: "tax_peasants_high", PlayerID: king, CountryID: s.PlayerCountryID(other)}, "taxing another country's peasants")
		}
	}

	// Choosing twice
	pending := map[string][]jsonapi.ActionJSON{}
	var queued jsonapi.QueuedResponse
	json.Unmarshal(b.request(map[string]any{"type": "get_queued"}), &queued)
	for _, a := range queued.Actions {
		pending[a.Type] = append(pending[a.Type], a)
	}
	for _, kind := range []string{"tax_peasants_low", "tax_peasants_high", "attack", "no_attack", "remain", "revolt", "flee", "vote_tax_low", "vote_tax_high", "vote_attack", "vote_no_attack"} {
		if len(pending[kind]) > 0 {
			b.refuse(pending[kind][0], "choosing "+kind+" twice")
			if kind == "tax_peasants_low" {
				changed := pending[kind][0]
				changed.Type = "tax_peasants_high"
				b.refuse(changed, "a second, different peasant tax")
			}
		}
	}
	for _, a := range pending["tax_merchants"] {
		purse := s.GetMerchant(a.MerchantID).StoredGold
		already := 0
		for _, other := range pending["tax_merchants"] {
			if other.MerchantID == a.MerchantID {
				n, _ := other.Amount.(float64)
				already += int(n)
			}
		}
		b.refuse(withAmount(a, purse-already+1), "taxing more than is left in the purse")
		break
	}
}

// ---------------------------------------------------------------------------
// Checking the rules after each phase

func eventsOf(evts []jsonapi.EventJSON, eventType string) []jsonapi.EventJSON {
	var found []jsonapi.EventJSON
	for _, e := range evts {
		if e.Type == eventType {
			found = append(found, e)
		}
	}
	return found
}

func num(e jsonapi.EventJSON, key string) int {
	switch v := e.Data[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

func str(e jsonapi.EventJSON, key string) string {
	s, _ := e.Data[key].(string)
	return s
}

func strs(e jsonapi.EventJSON, key string) []string {
	var out []string
	switch v := e.Data[key].(type) {
	case []string:
		out = v
	case []any:
		for _, x := range v {
			s, _ := x.(string)
			out = append(out, s)
		}
	}
	return out
}

func sum(evts []jsonapi.EventJSON, key string) int {
	total := 0
	for _, e := range evts {
		total += num(e, key)
	}
	return total
}

// totalGold is every coin in the game: treasuries and merchants' purses,
// hidden gold and investments
func totalGold(s *engine.GameState) int {
	total := 0
	for _, c := range s.Countries {
		total += c.Gold
	}
	for _, m := range s.Merchants {
		total += m.TotalGold()
	}
	return total
}

func totalPeasants(s *engine.GameState) int {
	total := 0
	for _, c := range s.Countries {
		total += c.Peasants
	}
	return total
}

// afterDamage is what a country's health becomes after taking damage
func afterDamage(c *engine.Country, damage int) (hp int, diedOnce bool) {
	hp, diedOnce = c.HP-damage, c.DiedOnce
	if hp <= 0 && !diedOnce {
		hp, diedOnce = 1, true
	}
	return hp, diedOnce
}

func (b *bigGame) check(pre *engine.GameState, phase engine.PhaseType, evts []jsonapi.EventJSON) {
	post := b.state()

	// The round moves on one phase at a time
	wantTurn, wantPhase := pre.Turn, pre.Phase+1
	if pre.Phase == engine.PhaseAssessment {
		wantTurn, wantPhase = pre.Turn+1, engine.PhaseTaxation
	}
	if post.Turn != wantTurn || post.Phase != wantPhase {
		b.fail("after %s the game is in round %d %s, expected round %d %s", phase, post.Turn, post.Phase, wantTurn, wantPhase)
	}

	for _, e := range eventsOf(evts, "monarch_deposed") {
		if str(e, "to_country") == "" {
			b.gone[str(e, "monarch_id")] = true
		}
	}
	// Republics declare war by vote
	for _, e := range eventsOf(evts, "republic_war_vote") {
		if target := str(e, "target_id"); target != "" {
			b.declared[[2]string{str(e, "country_id"), target}] = true
		}
	}

	b.checkEveryone(post)
	b.checkGoldIsAccountedFor(pre, post, phase, evts)
	b.checkPeasants(pre, post, evts)
	b.checkSecrecy(post)

	switch phase {
	case engine.PhaseTaxation:
		b.checkTaxation(pre, post, evts)
	case engine.PhaseSpending:
		b.checkSpending(pre, post, evts)
	case engine.PhaseWar:
		b.checkWar(pre, post, evts)
	case engine.PhaseAssessment:
		b.checkAssessment(pre, post, evts)
	}
}

// checkEveryone makes sure no player is lost or counted twice, and that every
// country is in a sensible state
func (b *bigGame) checkEveryone(s *engine.GameState) {
	alive := s.GetAliveCountries()
	seen := map[string]string{}
	for _, c := range s.Countries {
		if c.MonarchID == "" {
			continue
		}
		if c.IsRepublic || !c.IsAlive() {
			b.fail("%s (republic %v, hp %d) still has monarch %s", c.ID, c.IsRepublic, c.HP, c.MonarchID)
		}
		if where, dup := seen[c.MonarchID]; dup {
			b.fail("%s rules both %s and %s", c.MonarchID, where, c.ID)
		}
		seen[c.MonarchID] = "the throne of " + c.ID
	}
	for id, m := range s.Merchants {
		if where, dup := seen[id]; dup {
			b.fail("%s is a merchant and also holds %s", id, where)
		}
		seen[id] = "a merchant"
		c := s.GetCountry(m.CountryID)
		if c == nil {
			b.fail("merchant %s lives in unknown country %q", id, m.CountryID)
		} else if !c.IsAlive() && len(alive) > 0 {
			b.fail("merchant %s is left behind in destroyed %s", id, c.ID)
		}
		if m.StoredGold < 0 || m.HiddenGold < 0 || m.InvestedGold < 0 {
			b.fail("merchant %s has negative gold %+v", id, m)
		}
	}
	for _, p := range b.players {
		if _, ok := seen[p]; !ok && !b.gone[p] {
			b.fail("player %s has disappeared from the game", p)
		}
		if _, ok := seen[p]; ok && b.gone[p] {
			b.fail("player %s left the game but is still in it", p)
		}
	}
	if len(seen) != len(b.players)-len(b.gone) {
		b.fail("%d players in the game, expected %d", len(seen), len(b.players)-len(b.gone))
	}

	for _, c := range s.Countries {
		if c.Gold < 0 || c.ArmyStrength < 0 || c.Peasants < 0 {
			b.fail("%s has a negative value: %+v", c.ID, c)
		}
		if c.HP > 10 {
			b.fail("%s has %d HP, more than it started with", c.ID, c.HP)
		}
		if c.IsAlive() {
			if c.RevoltRisk < 2 || c.RevoltRisk > 5 {
				b.fail("%s has a revolt risk of %d in 6", c.ID, c.RevoltRisk)
			}
			if !c.IsRepublic && c.MonarchID == "" {
				b.fail("the monarchy of %s has no monarch", c.ID)
			}
		} else {
			if !c.DiedOnce {
				b.fail("%s is dead without having died before", c.ID)
			}
			if c.Peasants != 0 {
				b.fail("destroyed %s still has %d peasants", c.ID, c.Peasants)
			}
			if c.Gold != 0 {
				b.fail("destroyed %s still has %d gold in its treasury", c.ID, c.Gold)
			}
		}
	}
}

// checkGoldIsAccountedFor makes sure no gold appears or vanishes except the
// way the rules say: the bank pays taxes, income, victories, investment
// returns and fallen monarchs; armies, destroyed investments and destroyed
// treasuries take gold out of the game.
func (b *bigGame) checkGoldIsAccountedFor(pre, post *engine.GameState, phase engine.PhaseType, evts []jsonapi.EventJSON) {
	fallenMonarchs := 0
	for _, e := range eventsOf(evts, "monarch_deposed") {
		if str(e, "to_country") != "" {
			fallenMonarchs += engine.FallenMonarchPurse
		}
	}
	collapsed := eventsOf(evts, "country_collapsed")

	want := totalGold(pre)
	switch phase {
	case engine.PhaseTaxation:
		want += sum(eventsOf(evts, "peasant_tax"), "amount") + fallenMonarchs -
			sum(collapsed, "treasury_lost") - sum(collapsed, "forfeited_gold")
	case engine.PhaseSpending:
		want -= sum(eventsOf(evts, "army_built"), "amount") + sum(eventsOf(evts, "army_contributed"), "amount")
	case engine.PhaseWar:
		victories := 0
		for _, e := range eventsOf(evts, "battle_resolved") {
			winner, attacker, defender := str(e, "winner_id"), str(e, "attacker_id"), str(e, "defender_id")
			if winner != "" && (winner == attacker || b.declared[[2]string{defender, attacker}]) {
				victories++
			}
		}
		b.stats["victory gold paid"] += victories
		want += victories*5 + sum(eventsOf(evts, "merchant_income"), "amount") + fallenMonarchs
	case engine.PhaseAssessment:
		// Every investment is either paid out or destroyed by the start of
		// the next round, so what was invested is replaced by the payouts
		invested := 0
		for _, m := range pre.Merchants {
			invested += m.InvestedGold
		}
		want += sum(eventsOf(evts, "investment_payout"), "amount") - invested + fallenMonarchs - sum(collapsed, "treasury_lost")
		// An exile with nowhere left to go takes their gold out of the game
		exiled := map[string]bool{}
		for _, e := range eventsOf(evts, "monarch_deposed") {
			if str(e, "to_country") != "" {
				exiled[str(e, "monarch_id")] = true
			} else if exiled[str(e, "monarch_id")] {
				want -= num(e, "gold_kept")
			}
		}
	}
	if got := totalGold(post); got != want {
		b.fail("gold does not add up after %s: %d in the game, expected %d", phase, got, want)
	}
}

// checkPeasants: peasants are only ever moved by conquest or destroyed with
// their country
func (b *bigGame) checkPeasants(pre, post *engine.GameState, evts []jsonapi.EventJSON) {
	want := totalPeasants(pre)
	for _, e := range append(eventsOf(evts, "country_collapsed"), eventsOf(evts, "republic_abandoned")...) {
		want -= pre.GetCountry(str(e, "country_id")).Peasants
	}
	if got := totalPeasants(post); got != want {
		b.fail("there are %d peasants, expected %d", got, want)
	}
}

// checkSecrecy looks at the game from every player's seat
func (b *bigGame) checkSecrecy(s *engine.GameState) {
	if s.Settings.OpenGame {
		return
	}
	for _, p := range b.players {
		if b.gone[p] {
			continue
		}
		view := jsonapi.SerializeStateForPlayer(s, p)
		own := s.PlayerCountryID(p)
		isMonarch := own != "" && s.GetCountry(own).MonarchID == p
		for id, c := range view.Countries {
			real := s.GetCountry(id)
			if id == own {
				if c.Gold != real.Gold || c.ArmyStrength != real.ArmyStrength || c.Hidden {
					b.fail("%s cannot see their own country %s properly", p, id)
				}
				continue
			}
			if c.Gold != 0 || c.Peasants != 0 || c.RevoltRisk != 0 || !c.Hidden {
				b.fail("%s can see inside %s: %+v", p, id, c)
			}
			if c.ArmyStrength != real.PublicArmy {
				b.fail("%s sees %s's army as %d, but only %d has been revealed", p, id, c.ArmyStrength, real.PublicArmy)
			}
			if c.HP != real.HP || c.IsRepublic != real.IsRepublic {
				b.fail("%s gets %s's public facts wrong", p, id)
			}
		}
		for id, m := range view.Merchants {
			real := s.GetMerchant(id)
			if id == p {
				if m.StoredGold != real.StoredGold || m.HiddenGold != real.HiddenGold || m.InvestedGold != real.InvestedGold {
					b.fail("%s cannot see their own gold", p)
				}
				continue
			}
			if m.HiddenGold != 0 || m.InvestedGold != 0 || !m.Hidden {
				b.fail("%s can see %s's hidden or invested gold", p, id)
			}
			ownMerchant := isMonarch && real.CountryID == own
			if ownMerchant && (m.StoredGold != real.StoredGold || m.PurseHidden) {
				b.fail("monarch %s cannot see the purse of their merchant %s", p, id)
			}
			if !ownMerchant && (m.StoredGold != 0 || !m.PurseHidden) {
				b.fail("%s can see the purse of %s", p, id)
			}
		}
	}
}

func (b *bigGame) checkTaxation(pre, post *engine.GameState, evts []jsonapi.EventJSON) {
	taxes := map[string][]jsonapi.EventJSON{}
	for _, e := range eventsOf(evts, "peasant_tax") {
		taxes[str(e, "country_id")] = append(taxes[str(e, "country_id")], e)
	}
	revolts := map[string]bool{}
	for _, e := range eventsOf(evts, "peasant_revolt") {
		revolts[str(e, "country_id")] = true
	}

	for id, before := range pre.Countries {
		after := post.GetCountry(id)
		if !before.IsAlive() {
			if len(taxes[id]) != 0 {
				b.fail("destroyed %s collected a peasant tax", id)
			}
			continue
		}
		if len(taxes[id]) != 1 {
			b.fail("%s collected its peasant tax %d times", id, len(taxes[id]))
			continue
		}
		e := taxes[id][0]
		high := e.Data["high_tax"] == true
		rate := 1
		if high {
			rate = 2
			b.stats["high taxes"]++
		}
		switch {
		case revolts[id]:
			if !high || num(e, "amount") != 0 {
				b.fail("%s had a peasant revolt on %v tax and still collected %d", id, high, num(e, "amount"))
			}
			if hp, _ := afterDamage(before, 2); (hp > 0 && after.HP != hp) || (hp <= 0 && after.IsAlive()) {
				b.fail("the peasant revolt should leave %s with %d HP, it has %d", id, hp, after.HP)
			}
			if after.IsAlive() && after.RevoltRisk != 2 {
				b.fail("a peasant revolt should reset %s's risk to 2, it is %d", id, after.RevoltRisk)
			}
		case num(e, "amount") != before.Peasants*rate:
			b.fail("%s collected %d from %d peasants at %d each", id, num(e, "amount"), before.Peasants, rate)
		case high && after.RevoltRisk != min(before.RevoltRisk+1, 5):
			b.fail("%s's risk went from %d to %d after a calm high tax", id, before.RevoltRisk, after.RevoltRisk)
		case !high && after.RevoltRisk != 2:
			b.fail("%s's risk should be 2 after a low tax, it is %d", id, after.RevoltRisk)
		}
		if !revolts[id] && after.HP != before.HP {
			b.fail("%s lost HP without a revolt", id)
		}
	}

	// Merchants lose exactly what they were taxed, and only from the purse
	taxed := map[string]int{}
	for _, e := range eventsOf(evts, "merchant_tax") {
		taxed[str(e, "merchant_id")] += num(e, "amount")
	}
	for id, before := range pre.Merchants {
		after := post.GetMerchant(id)
		if after.Arriving != before.Arriving {
			continue // Scattered by a collapse
		}
		if after.StoredGold != before.StoredGold-taxed[id] && !pre.GetCountry(before.CountryID).IsRepublic {
			b.fail("%s's purse went from %d to %d after a tax of %d", id, before.StoredGold, after.StoredGold, taxed[id])
		}
		if after.HiddenGold != before.HiddenGold {
			b.fail("%s's hidden gold changed during taxation", id)
		}
	}
}

func (b *bigGame) checkSpending(pre, post *engine.GameState, evts []jsonapi.EventJSON) {
	armyAdded := map[string]int{}
	for _, e := range append(eventsOf(evts, "army_built"), eventsOf(evts, "army_contributed")...) {
		armyAdded[str(e, "country_id")] += num(e, "amount")
	}
	gifts := map[string]int{}
	given := map[string]int{}
	for _, e := range eventsOf(evts, "investment_made") {
		if str(e, "from") == "monarch" {
			gifts[str(e, "merchant_id")] += num(e, "amount")
			given[str(e, "country_id")] += num(e, "amount")
		}
	}
	contributed := map[string]int{}
	for _, e := range eventsOf(evts, "army_contributed") {
		contributed[str(e, "merchant_id")] += num(e, "amount")
	}

	for id, before := range pre.Countries {
		after := post.GetCountry(id)
		if after.ArmyStrength != before.ArmyStrength+armyAdded[id] {
			b.fail("%s's army went from %d to %d, expected +%d", id, before.ArmyStrength, after.ArmyStrength, armyAdded[id])
		}
		spent := given[id]
		if !before.IsRepublic {
			spent += armyAdded[id]
		}
		if after.Gold != before.Gold-spent {
			b.fail("%s's treasury went from %d to %d, expected -%d", id, before.Gold, after.Gold, spent)
		}
		if after.HP != before.HP || after.IsRepublic != before.IsRepublic {
			b.fail("%s changed during spending", id)
		}
	}
	for id, before := range pre.Merchants {
		after := post.GetMerchant(id)
		if after.TotalGold() != before.TotalGold()+gifts[id]-contributed[id] {
			b.fail("%s had %d gold and has %d after a gift of %d and %d for the army", id, before.TotalGold(), after.TotalGold(), gifts[id], contributed[id])
		}
		if after.StoredGold < gifts[id] {
			b.fail("%s's gift of %d did not land in the purse", id, gifts[id])
		}
		if after.CountryID != before.CountryID {
			b.fail("%s moved during spending", id)
		}
	}
}

func (b *bigGame) checkWar(pre, post *engine.GameState, evts []jsonapi.EventJSON) {
	// One battle per pair of countries at war, however many declared it
	pairs := map[[2]string]bool{}
	for d := range b.declared {
		if d[0] > d[1] {
			d[0], d[1] = d[1], d[0]
		}
		pairs[d] = true
	}
	battles := eventsOf(evts, "battle_resolved")
	if len(battles) != len(pairs) {
		b.fail("%d battles were fought for %d pairs of countries at war", len(battles), len(pairs))
	}

	damage := map[string]int{}
	for _, e := range battles {
		a, d := str(e, "attacker_id"), str(e, "defender_id")
		as, ds := pre.GetCountry(a).ArmyStrength, pre.GetCountry(d).ArmyStrength
		if num(e, "attacker_strength") != as || num(e, "defender_strength") != ds {
			b.fail("battle %s vs %s used armies %d vs %d, they had %d vs %d", a, d, num(e, "attacker_strength"), num(e, "defender_strength"), as, ds)
		}
		wantWinner, loser := "", ""
		switch {
		case as > ds:
			wantWinner, loser = a, d
		case ds > as:
			wantWinner, loser = d, a
		}
		if str(e, "winner_id") != wantWinner {
			b.fail("battle %s (%d) vs %s (%d) was won by %q", a, as, d, ds, str(e, "winner_id"))
		}
		if loser != "" {
			damage[loser] += max(as, ds) - min(as, ds)
		}
		if b.declared[[2]string{a, d}] && b.declared[[2]string{d, a}] {
			b.stats["mutual battles"]++
		}
	}

	for id, before := range pre.Countries {
		if !before.IsAlive() {
			continue
		}
		after := post.GetCountry(id)
		hp, _ := afterDamage(before, damage[id])
		if hp <= 0 {
			if after.IsAlive() {
				b.fail("%s should have been destroyed in the war (hp %d)", id, hp)
			}
			continue
		}
		if after.HP != hp {
			b.fail("%s should have %d HP after the war, has %d", id, hp, after.HP)
		}
		if after.ArmyStrength != before.ArmyStrength/2 {
			b.fail("%s's army of %d should be halved to %d, it is %d", id, before.ArmyStrength, before.ArmyStrength/2, after.ArmyStrength)
		}
		if after.PublicArmy != after.ArmyStrength {
			b.fail("%s's army was not revealed after the war", id)
		}
	}

	if got := len(eventsOf(evts, "merchant_income")); got != len(post.Merchants) {
		b.fail("%d merchants received income, there are %d", got, len(post.Merchants))
	}

	// Conquest: the losers' merchants and fallen monarch are shared between
	// the winners as evenly as possible
	for _, e := range eventsOf(evts, "annexation") {
		defeated, winners := str(e, "defeated_id"), strs(e, "winner_ids")
		b.stats["conquests"]++
		if len(winners) > 1 {
			b.stats["conquests shared by several winners"]++
		}
		isWinner := map[string]bool{}
		for _, w := range winners {
			isWinner[w] = true
			if !post.GetCountry(w).IsAlive() {
				b.fail("%s was given to %s, which is destroyed", defeated, w)
			}
		}
		perWinner := map[string]int{}
		for _, id := range strs(e, "merchants") {
			m := post.GetMerchant(id)
			if !isWinner[m.CountryID] || !m.Arriving {
				b.fail("%s from conquered %s went to %s (arriving %v), winners %v", id, defeated, m.CountryID, m.Arriving, winners)
			}
			if before := pre.GetMerchant(id); before != nil && before.InvestedGold != m.InvestedGold {
				b.fail("%s's investments changed from %d to %d on conquest", id, before.InvestedGold, m.InvestedGold)
			}
			perWinner[m.CountryID]++
		}
		counts := []int{}
		for _, w := range winners {
			counts = append(counts, perWinner[w])
		}
		sort.Ints(counts)
		if len(counts) > 0 && counts[len(counts)-1]-counts[0] > 1 {
			b.fail("the merchants of %s were shared unevenly: %v", defeated, counts)
		}
	}
	for _, e := range eventsOf(evts, "monarch_deposed") {
		m := post.GetMerchant(str(e, "monarch_id"))
		if m == nil || m.StoredGold != engine.FallenMonarchPurse+5 || m.HiddenGold != 0 || m.InvestedGold != 0 {
			b.fail("fallen monarch %s should have 5 gold plus 5 income, has %+v", str(e, "monarch_id"), m)
		}
	}
}

func (b *bigGame) checkAssessment(pre, post *engine.GameState, evts []jsonapi.EventJSON) {
	percent := pre.Settings.InvestmentReturnPercent
	for id, m := range post.Merchants {
		if m.Arriving {
			b.fail("%s is still on the move when the new round starts", id)
		}
		if m.InvestedGold != 0 {
			b.fail("%s's investment of %d was not paid out", id, m.InvestedGold)
		}
	}

	// Work out each revolt from the choices that were made
	rebels := map[string][]string{}
	loyal := map[string]int{}
	for id, choice := range b.choices {
		m := pre.GetMerchant(id)
		switch choice {
		case "revolt":
			rebels[m.CountryID] = append(rebels[m.CountryID], id)
		case "remain":
			loyal[m.CountryID] += m.SpendableGold()
		}
	}
	succeeded := map[string]bool{}
	for _, e := range eventsOf(evts, "revolt_success") {
		succeeded[str(e, "country_id")] = true
	}
	failed := map[string]bool{}
	for _, e := range eventsOf(evts, "revolt_failed") {
		failed[str(e, "country_id")] = true
	}
	collapsed := map[string]bool{}
	for _, e := range append(eventsOf(evts, "country_collapsed"), eventsOf(evts, "republic_abandoned")...) {
		collapsed[str(e, "country_id")] = true
	}

	payout := map[string]int{}
	for _, e := range eventsOf(evts, "investment_payout") {
		payout[str(e, "merchant_id")] = num(e, "amount")
	}

	for id, before := range pre.Countries {
		if !before.IsAlive() {
			continue
		}
		after := post.GetCountry(id)
		group := rebels[id]
		if len(group) == 0 {
			if succeeded[id] || failed[id] {
				b.fail("%s had a revolt nobody joined", id)
			}
			if !collapsed[id] && (after.Gold != before.Gold || after.MonarchID != before.MonarchID || after.HP != before.HP) {
				b.fail("%s changed without a revolt: %+v -> %+v", id, before, after)
			}
			continue
		}
		strength := 0
		for _, r := range group {
			strength += pre.GetMerchant(r).SpendableGold()
		}
		defence := before.Gold + loyal[id]
		b.stats["merchant revolts"]++
		if strength > defence {
			b.stats["successful merchant revolts"]++
			if !succeeded[id] {
				b.fail("rebels with %d gold against %d should have taken %s", strength, defence, id)
				continue
			}
			if after.IsAlive() && (!after.IsRepublic || after.MonarchID != "") {
				b.fail("%s should be a republic after its revolt", id)
			}
			if hp, _ := afterDamage(before, 2); (hp > 0 && after.HP != hp) || (hp <= 0 && after.IsAlive()) {
				b.fail("%s should have %d HP after the revolt, has %d", id, hp, after.HP)
			}
			if exiled := post.GetMerchant(before.MonarchID); exiled != nil {
				for _, e := range eventsOf(evts, "monarch_deposed") {
					if str(e, "monarch_id") == before.MonarchID && collapsed[str(e, "to_country")] {
						b.stats["exiles whose country of exile fell the same phase"]++
					}
				}
				if exiled.CountryID == id || exiled.StoredGold != engine.FallenMonarchPurse || exiled.HiddenGold != 0 {
					b.fail("exiled monarch %s should start over elsewhere with 5 gold, got %+v", before.MonarchID, exiled)
				}
			} else if !b.gone[before.MonarchID] {
				b.fail("exiled monarch %s vanished", before.MonarchID)
			}
			// The rebels share out the whole treasury
			got, want := 0, strength+before.Gold
			for _, r := range group {
				m := post.GetMerchant(r)
				got += m.SpendableGold()
				want += payout[r]
			}
			if got != want {
				b.fail("the rebels of %s hold %d gold, expected %d", id, got, want)
			}
		} else {
			if !failed[id] {
				b.fail("rebels with %d gold against %d should have failed in %s", strength, defence, id)
				continue
			}
			for _, r := range group {
				if m := post.GetMerchant(r); m.TotalGold() != 0 {
					b.fail("failed rebel %s kept %d gold", r, m.TotalGold())
				}
			}
			if after.Gold != before.Gold+strength || after.MonarchID != before.MonarchID || after.HP != before.HP {
				b.fail("after a failed revolt %s should keep its monarch and gain %d gold: %+v -> %+v", id, strength, before, after)
			}
		}
	}

	// Everyone else keeps their gold; fleeing costs the investments,
	// staying pays them out
	for id, choice := range b.choices {
		before, after := pre.GetMerchant(id), post.GetMerchant(id)
		home := before.CountryID
		if choice == "revolt" || succeeded[home] || collapsed[home] {
			continue
		}
		fled := choice == "flee" && post.GetCountry(b.fleeTo[id]).IsAlive() && pre.GetCountry(b.fleeTo[id]).IsAlive()
		if fled && !collapsed[b.fleeTo[id]] {
			b.stats["merchants fled"]++
			if after.CountryID != b.fleeTo[id] || after.SpendableGold() != before.SpendableGold() {
				b.fail("%s fled to %s with %d gold, ended up in %s with %d", id, b.fleeTo[id], before.SpendableGold(), after.CountryID, after.SpendableGold())
			}
			continue
		}
		if choice == "flee" {
			continue // Their destination fell this same phase
		}
		if want := before.StoredGold + before.InvestedGold*percent/100; after.CountryID != home || after.StoredGold != want || after.HiddenGold != before.HiddenGold {
			b.fail("%s stayed in %s with purse %d and %d invested, now has purse %d in %s (expected %d)", id, home, before.StoredGold, before.InvestedGold, after.StoredGold, after.CountryID, want)
		}
	}
}

// ---------------------------------------------------------------------------
// The tests

// The game can be set up with 30 players in 7 teams, and everybody can see
// who they are and where they stand.
func TestThirtyPlayersInSevenTeamsSetUp(t *testing.T) {
	b := newBigGame(t, 1)

	var players jsonapi.GetPlayersResponse
	json.Unmarshal(b.request(map[string]any{"type": "get_players"}), &players)
	if len(players.Players) != bigPlayers {
		t.Fatalf("get_players lists %d players, expected %d", len(players.Players), bigPlayers)
	}
	teams := map[string]int{}
	monarchs := 0
	for id, info := range players.Players {
		teams[info.CountryID]++
		if info.Role == "monarch" {
			monarchs++
			if !strings.HasPrefix(id, "monarch-") {
				t.Errorf("%s is listed as a monarch", id)
			}
		}
	}
	if monarchs != 7 || len(teams) != 7 {
		t.Errorf("expected 7 monarchs in 7 countries, got %d monarchs and teams %v", monarchs, teams)
	}
	for i, c := range bigCountries {
		want := 4
		if i < 2 {
			want = 5
		}
		if teams[c] != want {
			t.Errorf("%s has %d players, expected %d", c, teams[c], want)
		}
	}

	// Nobody can join twice or take two roles
	for _, req := range []map[string]any{
		{"type": "add_merchant", "player_id": "merchant-01", "country_id": "Castile"},
		{"type": "add_merchant", "player_id": "monarch-3", "country_id": "Avalon"},
		{"type": "add_merchant", "player_id": "newcomer", "country_id": "Atlantis"},
		{"type": "add_country", "country_id": "Avalon", "monarch_id": "someone"},
		{"type": "add_country", "country_id": "Hungary", "monarch_id": "monarch-1"},
		{"type": "add_country", "country_id": "Hungary", "monarch_id": "merchant-05"},
	} {
		var resp struct{ Success bool }
		json.Unmarshal(b.request(req), &resp)
		if resp.Success {
			t.Errorf("the duplicate %v was accepted", req)
		}
	}

	var state jsonapi.StateResponse
	json.Unmarshal(b.request(map[string]any{"type": "get_state"}), &state)
	for _, c := range state.State.Countries {
		if c.HP != 10 || c.Gold != 10 || c.Peasants != 5 || c.RevoltRisk != 2 || c.ArmyStrength != 0 {
			t.Errorf("%s does not start as the rules say: %+v", c.CountryID, c)
		}
	}
	for _, m := range state.State.Merchants {
		if m.StoredGold != 5 || m.HiddenGold != 0 || m.InvestedGold != 0 {
			t.Errorf("%s does not start with 5 gold: %+v", m.PlayerID, m)
		}
	}
	b.checkEveryone(b.state())
	b.checkSecrecy(b.state())
}

// Many complete games of 30 players in 7 teams, each played with different
// dice and different random choices, checked against the rules after every
// phase.
func TestThirtyPlayersInSevenTeamsPlayFullGames(t *testing.T) {
	games := 150
	if testing.Short() {
		games = 20
	}
	total := map[string]int{}
	rounds := 0
	start := time.Now()

	for seed := int64(1); seed <= int64(games); seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			b := newBigGame(t, seed)
			switch seed % 5 {
			case 1:
				b.request(map[string]any{"type": "set_settings", "investment_return_percent": 150})
			case 2:
				b.request(map[string]any{"type": "set_settings", "investment_return_percent": 0})
			case 3:
				b.request(map[string]any{"type": "set_settings", "investment_return_percent": 500})
			}
			for round := 1; round <= bigMaxRounds; round++ {
				b.playRound()
				rounds++
				if b.failures > 0 || len(b.state().GetAliveCountries()) == 0 {
					break
				}
			}
			alive := len(b.state().GetAliveCountries())
			b.stats[fmt.Sprintf("games ending with %d countries left", alive)]++
			for k, v := range b.stats {
				total[k] += v
			}
		})
	}

	keys := make([]string, 0, len(total))
	for k := range total {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t.Logf("%d games, %d rounds in %v", games, rounds, time.Since(start).Round(time.Millisecond))
	for _, k := range keys {
		t.Logf("  %-45s %d", k, total[k])
	}

	// The random play must actually have reached every part of the rules
	for _, k := range []string{
		"event: peasant_revolt", "event: revolt_success", "event: revolt_failed", "event: republic_formed",
		"event: annexation", "event: country_collapsed", "event: republic_abandoned", "event: merchant_fled",
		"event: republic_war_vote", "event: army_contributed", "event: investment_payout", "event: merchant_arrived",
		"conquests shared by several winners", "mutual battles", "illegal moves tried",
	} {
		if total[k] == 0 {
			t.Errorf("the games never reached %q, so it went untested", k)
		}
	}
}

// A 30-player game plays quickly enough that the game leader never waits on
// the server: every request in a full round, including all 30 players
// looking up their moves, is answered in well under a second.
func TestThirtyPlayerRoundIsFast(t *testing.T) {
	b := newBigGame(t, 42)
	start := time.Now()
	for i := 0; i < 10; i++ {
		b.playRound()
	}
	perRound := time.Since(start) / 10
	t.Logf("one round of 30 players, with every rule check, takes %v", perRound)
	if perRound > time.Second {
		t.Errorf("a round takes %v", perRound)
	}
}
