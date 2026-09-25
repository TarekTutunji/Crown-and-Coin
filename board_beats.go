package main

// Turning a resolved phase into the beats the projector board plays. See the
// comment at the top of board.go for which of the three tiers each event
// belongs to.
//
// Live beats carry names, health and - in the war phase, where the engine
// publishes every army anyway - army strengths. They never carry an amount of
// gold, of either kind. Gold appears only in the chronicle, and merchant gold
// not even there.

import (
	"fmt"
	"sort"
	"strings"

	"crown_and_coin/engine"
	"crown_and_coin/jsonapi"
)

// Reading event data. The events have been through JSON by the time the board
// sees them, so a number arrives as a float64; tests build them by hand with
// plain ints, so both are accepted.

func evtStr(d map[string]interface{}, key string) string {
	s, _ := d[key].(string)
	return s
}

func evtInt(d map[string]interface{}, key string) int {
	switch v := d[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

func evtBool(d map[string]interface{}, key string) bool {
	b, _ := d[key].(bool)
	return b
}

func evtList(d map[string]interface{}, key string) []string {
	switch v := d[key].(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// eventsOfType picks out the events of one type, keeping the order the engine
// produced them in
func eventsOfType(evts []jsonapi.EventJSON, eventType string) []jsonapi.EventJSON {
	var out []jsonapi.EventJSON
	for _, e := range evts {
		if e.Type == eventType {
			out = append(out, e)
		}
	}
	return out
}

func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// countThings counts things there can be several of: merchants, rebels
func countThings(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// amount measures what the game never counts in plurals: gold, health, damage
func amount(n int, word string) string {
	return fmt.Sprintf("%d %s", n, word)
}

// howDeposed puts a MonarchDeposedEvent's reason into words
func howDeposed(reason string) string {
	switch reason {
	case "conquest":
		return "is conquered"
	case "peasant revolt":
		return "loses the throne to the peasants"
	default:
		return "is overthrown"
	}
}

// collapseCause puts a CountryCollapsedEvent's reason into words
func collapseCause(reason string) string {
	if reason == "peasant revolt" {
		return "a peasant revolt"
	}
	return "a merchant revolt"
}

// publicBeats turns a resolved phase into the beats the room may watch. They
// are ordered to be told as a story - declarations, then the fighting, then the
// damage, then who lost a country, a throne or a home - which is not
// necessarily the order the engine had to resolve them in.
func publicBeats(phase string, evts []jsonapi.EventJSON, before, after *engine.GameState) []BoardBeat {
	beats := make([]BoardBeat, 0)
	add := func(b *BoardBeat) {
		if b != nil {
			beats = append(beats, *b)
		}
	}

	// Who said they would march
	for _, e := range eventsOfType(evts, "republic_war_vote") {
		d := e.Data
		country, target := evtStr(d, "country_id"), evtStr(d, "target_id")
		if target == "" {
			add(&BoardBeat{Kind: "vote", Focus: []string{country},
				Text: fmt.Sprintf("The merchants of %s reach no majority. %s marches on nobody.", country, country)})
			continue
		}
		add(&BoardBeat{Kind: "vote", Focus: []string{country, target},
			Text: fmt.Sprintf("The merchants of %s vote to attack %s.", country, target)})
	}

	// Trouble at home
	for _, e := range eventsOfType(evts, "peasant_revolt") {
		country := evtStr(e.Data, "country_id")
		add(&BoardBeat{Kind: "revolt", Focus: []string{country},
			Text: fmt.Sprintf("The peasants of %s rise against the tax.", country)})
	}
	for _, e := range eventsOfType(evts, "revolt_success") {
		country := evtStr(e.Data, "country_id")
		rebels := len(evtList(e.Data, "participants"))
		add(&BoardBeat{Kind: "revolt", Focus: []string{country},
			Text: fmt.Sprintf("%s of %s rise against their monarch, and win.", countThings(rebels, "merchant"), country)})
	}
	for _, e := range eventsOfType(evts, "revolt_failed") {
		country := evtStr(e.Data, "country_id")
		rebels := len(evtList(e.Data, "participants"))
		add(&BoardBeat{Kind: "revolt-failed", Focus: []string{country},
			Text: fmt.Sprintf("%s of %s rise against their monarch, and fail.", countThings(rebels, "merchant"), country)})
	}

	// The battles, with the armies the war has just made public
	battles := eventsOfType(evts, "battle_resolved")
	isRepublic := func(id string) bool {
		if c := before.GetCountry(id); c != nil {
			return c.IsRepublic
		}
		if c := after.GetCountry(id); c != nil {
			return c.IsRepublic
		}
		return false
	}
	for _, e := range battles {
		d := e.Data
		attacker, defender := evtStr(d, "attacker_id"), evtStr(d, "defender_id")
		winner := evtStr(d, "winner_id")
		loser := ""
		if winner == attacker {
			loser = defender
		} else if winner == defender {
			loser = attacker
		}
		battle := &BoardBattle{
			AttackerID:       attacker,
			DefenderID:       defender,
			AttackerStrength: evtInt(d, "attacker_strength"),
			DefenderStrength: evtInt(d, "defender_strength"),
			AttackerRepublic: isRepublic(attacker),
			DefenderRepublic: isRepublic(defender),
			WinnerID:         winner,
			LoserID:          loser,
			Damage:           evtInt(d, "damage"),
		}
		text := fmt.Sprintf("%s and %s meet with equal strength. Neither gives ground.", attacker, defender)
		if winner != "" {
			text = fmt.Sprintf("%s marches on %s. %s carries the field.", attacker, defender, winner)
		}
		add(&BoardBeat{Kind: "battle", Text: text, Focus: []string{attacker, defender}, Battle: battle})
	}
	if phase == "war" && len(battles) == 0 {
		add(&BoardBeat{Kind: "quiet", Text: "No realm marches this round. The armies stay home."})
	}

	// What all of that cost in health
	loserOrder := make([]string, 0, len(battles))
	for _, e := range battles {
		if loser := lossSide(e.Data); loser != "" {
			loserOrder = append(loserOrder, loser)
		}
	}
	beats = append(beats, damageBeats(loserOrder, before, after)...)

	// Countries, thrones and homes lost
	for _, e := range eventsOfType(evts, "annexation") {
		d := e.Data
		defeated := evtStr(d, "defeated_id")
		winners := evtList(d, "winner_ids")
		taken := len(evtList(d, "merchants"))
		focus := append([]string{defeated}, winners...)
		text := fmt.Sprintf("%s annexes %s.", joinNames(winners), defeated)
		if taken > 0 {
			text = fmt.Sprintf("%s annexes %s and divides its %s.", joinNames(winners), defeated, countThings(taken, "merchant"))
		}
		add(&BoardBeat{Kind: "conquest", Text: text, Focus: focus})
	}
	for _, e := range eventsOfType(evts, "country_collapsed") {
		d := e.Data
		country := evtStr(d, "country_id")
		scattered := len(evtList(d, "merchants"))
		text := fmt.Sprintf("%s falls apart in %s.", country, collapseCause(evtStr(d, "reason")))
		if scattered > 0 {
			text = fmt.Sprintf("%s falls apart in %s. %s scatter to the surviving realms.",
				country, collapseCause(evtStr(d, "reason")), countThings(scattered, "merchant"))
		}
		add(&BoardBeat{Kind: "collapse", Text: text, Focus: []string{country}})
	}
	for _, e := range eventsOfType(evts, "republic_fallen") {
		country := evtStr(e.Data, "country_id")
		add(&BoardBeat{Kind: "collapse", Focus: []string{country},
			Text: fmt.Sprintf("The republic of %s has fallen.", country)})
	}
	for _, e := range eventsOfType(evts, "republic_abandoned") {
		country := evtStr(e.Data, "country_id")
		add(&BoardBeat{Kind: "collapse", Focus: []string{country},
			Text: fmt.Sprintf("Every merchant has left %s. The republic is no more.", country)})
	}
	for _, e := range eventsOfType(evts, "monarch_deposed") {
		d := e.Data
		monarch, from, to := evtStr(d, "monarch_id"), evtStr(d, "from_country"), evtStr(d, "to_country")
		if to == "" {
			add(&BoardBeat{Kind: "exile", Focus: []string{from},
				Text: fmt.Sprintf("%s of %s %s, and leaves the game.", monarch, from, howDeposed(evtStr(d, "reason")))})
			continue
		}
		add(&BoardBeat{Kind: "exile", Focus: []string{from, to},
			Text: fmt.Sprintf("%s of %s %s, and begins again as a merchant in %s.",
				monarch, from, howDeposed(evtStr(d, "reason")), to)})
	}
	for _, e := range eventsOfType(evts, "exile_relocated") {
		d := e.Data
		add(&BoardBeat{Kind: "exile", Focus: []string{evtStr(d, "to_country")},
			Text: fmt.Sprintf("%s cannot settle in %s, which fell the same phase, and goes to %s instead.",
				evtStr(d, "monarch_id"), evtStr(d, "intended"), evtStr(d, "to_country"))})
	}
	for _, e := range eventsOfType(evts, "republic_formed") {
		country := evtStr(e.Data, "country_id")
		add(&BoardBeat{Kind: "government", Focus: []string{country},
			Text: fmt.Sprintf("%s is a merchant republic now.", country)})
	}
	for _, e := range eventsOfType(evts, "merchant_fled") {
		d := e.Data
		from, to := evtStr(d, "from_country"), evtStr(d, "to_country")
		add(&BoardBeat{Kind: "flight", Focus: []string{from, to},
			Text: fmt.Sprintf("Merchant %s flees %s for %s.", evtStr(d, "merchant_id"), from, to)})
	}
	for _, e := range eventsOfType(evts, "merchant_arrived") {
		d := e.Data
		country := evtStr(d, "country_id")
		add(&BoardBeat{Kind: "arrival", Focus: []string{country},
			Text: fmt.Sprintf("Merchant %s reaches %s.", evtStr(d, "merchant_id"), country)})
	}

	// The war ends with every army halved for upkeep, and every army named:
	// this is the moment the rules make them public
	if upkeep := upkeepBeat(evts); upkeep != nil {
		add(upkeep)
	}

	if len(beats) == 0 {
		add(quietBeat(phase))
	}
	return beats
}

// lossSide returns which country a battle's damage falls on, or "" for a draw
func lossSide(d map[string]interface{}) string {
	winner := evtStr(d, "winner_id")
	if winner == "" {
		return ""
	}
	if winner == evtStr(d, "attacker_id") {
		return evtStr(d, "defender_id")
	}
	return evtStr(d, "attacker_id")
}

// damageBeats reports the health every country lost over the phase as a whole.
// Damage from several battles lands together by the rules, so one beat per
// country is also the honest picture. battleLosers puts the countries beaten in
// battle first, in the order they were beaten; anyone else hurt this phase
// follows in name order.
func damageBeats(battleLosers []string, before, after *engine.GameState) []BoardBeat {
	hurt := make([]string, 0)
	seen := make(map[string]bool)
	consider := func(id string) {
		if seen[id] {
			return
		}
		old, now := before.GetCountry(id), after.GetCountry(id)
		if old == nil || now == nil || now.HP >= old.HP {
			return
		}
		seen[id] = true
		hurt = append(hurt, id)
	}

	for _, id := range battleLosers {
		consider(id)
	}
	rest := make([]string, 0, len(after.Countries))
	for id := range after.Countries {
		rest = append(rest, id)
	}
	sort.Strings(rest)
	for _, id := range rest {
		consider(id)
	}

	beats := make([]BoardBeat, 0, len(hurt))
	for _, id := range hurt {
		old, now := before.GetCountry(id), after.GetCountry(id)
		change := &BoardChange{Subject: id, Label: "Health", From: max(0, old.HP), To: max(0, now.HP), Max: boardMaxHP}

		var text string
		switch {
		case now.HP <= 0:
			text = fmt.Sprintf("%s falls.", id)
		case !old.DiedOnce && now.DiedOnce:
			// TakeDamage spares a country its first death and leaves it on 1
			text = fmt.Sprintf("%s should have fallen, and rises once more with its last breath.", id)
		default:
			text = fmt.Sprintf("%s loses %s.", id, amount(old.HP-now.HP, "health"))
		}
		beats = append(beats, BoardBeat{Kind: "damage", Text: text, Focus: []string{id}, Change: change})
	}
	return beats
}

// upkeepBeat names every army halved at the end of a war. Army sizes are
// secret right up to here and public from here on, so this beat is where the
// room learns what everybody had been building.
func upkeepBeat(evts []jsonapi.EventJSON) *BoardBeat {
	kept := eventsOfType(evts, "army_maintenance")
	if len(kept) == 0 {
		return nil
	}
	sort.SliceStable(kept, func(i, j int) bool {
		return evtStr(kept[i].Data, "country_id") < evtStr(kept[j].Data, "country_id")
	})

	armies := make([]BoardChange, 0, len(kept))
	focus := make([]string, 0, len(kept))
	for _, e := range kept {
		d := e.Data
		country := evtStr(d, "country_id")
		focus = append(focus, country)
		from, to := evtInt(d, "old_strength"), evtInt(d, "new_strength")
		armies = append(armies, BoardChange{Subject: country, Label: "Army", From: from, To: to, Max: from})
	}
	return &BoardBeat{
		Kind:   "upkeep",
		Text:   "Every army stands revealed, and upkeep halves what is left of it.",
		Focus:  focus,
		Armies: armies,
	}
}

// quietBeat is what the board says for a phase with nothing public in it. The
// wording never changes with what actually happened, so a quiet phase gives
// nothing away.
func quietBeat(phase string) *BoardBeat {
	switch phase {
	case "taxation":
		return &BoardBeat{Kind: "quiet", Text: "The tax is gathered behind closed doors."}
	case "negotiation":
		return &BoardBeat{Kind: "quiet", Text: "The realms talk. Nothing is written down."}
	case "spending":
		return &BoardBeat{Kind: "quiet", Text: "Coin changes hands in private. What it bought will be plain when the armies next meet."}
	case "assessment":
		return &BoardBeat{Kind: "quiet", Text: "The realms hold. Nobody rises, nobody flees."}
	}
	return &BoardBeat{Kind: "quiet", Text: "Nothing the room may yet know."}
}

// sealedLines is the full record of a phase, with the real numbers, for the
// chronicle a finished war unseals. Merchant gold is left out even here: no
// rule ever reveals it in play, so it waits for the end of the game.
func sealedLines(evts []jsonapi.EventJSON) []string {
	lines := make([]string, 0)
	for _, e := range evts {
		if line := sealedLine(e); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func sealedLine(e jsonapi.EventJSON) string {
	d := e.Data
	switch e.Type {
	case "peasant_tax":
		country := evtStr(d, "country_id")
		if evtBool(d, "revolted") {
			return fmt.Sprintf("%s: the high tax failed, and the peasants paid nothing.", country)
		}
		rate := "low"
		if evtBool(d, "high_tax") {
			rate = "high"
		}
		return fmt.Sprintf("%s took the %s tax and collected %s.", country, rate, amount(evtInt(d, "amount"), "gold"))

	case "peasant_revolt":
		return fmt.Sprintf("The peasants of %s revolted, costing it %s.",
			evtStr(d, "country_id"), amount(evtInt(d, "damage"), "health"))

	case "republic_tax_vote", "republic_war_vote":
		return e.Message

	case "army_built":
		return fmt.Sprintf("%s spent %s on its army, bringing it to %d.",
			evtStr(d, "country_id"), amount(evtInt(d, "amount"), "gold"), evtInt(d, "new_total"))

	case "battle_resolved":
		attacker, defender := evtStr(d, "attacker_id"), evtStr(d, "defender_id")
		head := fmt.Sprintf("%s (%d) attacked %s (%d): ", attacker, evtInt(d, "attacker_strength"),
			defender, evtInt(d, "defender_strength"))
		winner := evtStr(d, "winner_id")
		if winner == "" {
			return head + "a draw, and no damage."
		}
		return head + fmt.Sprintf("%s won, and %s took %s.", winner, lossSide(d), amount(evtInt(d, "damage"), "damage"))

	case "army_maintenance":
		return fmt.Sprintf("Upkeep in %s: %d → %d.",
			evtStr(d, "country_id"), evtInt(d, "old_strength"), evtInt(d, "new_strength"))

	case "annexation":
		return fmt.Sprintf("%s annexed %s, taking %s of treasury and %s.",
			joinNames(evtList(d, "winner_ids")), evtStr(d, "defeated_id"),
			amount(evtInt(d, "treasury"), "gold"), countThings(len(evtList(d, "merchants")), "merchant"))

	case "country_collapsed":
		return fmt.Sprintf("%s collapsed in %s: %s of treasury was lost and %s scattered.",
			evtStr(d, "country_id"), collapseCause(evtStr(d, "reason")),
			amount(evtInt(d, "treasury_lost"), "gold"), countThings(len(evtList(d, "merchants")), "merchant"))

	case "revolt_success":
		return fmt.Sprintf("The revolt in %s succeeded: %s overcame %s of defending treasury.",
			evtStr(d, "country_id"), countThings(len(evtList(d, "participants")), "rebel"),
			amount(evtInt(d, "defense_gold"), "gold"))

	case "revolt_failed":
		return fmt.Sprintf("The revolt in %s failed: %s could not overcome %s of defending treasury.",
			evtStr(d, "country_id"), countThings(len(evtList(d, "participants")), "rebel"),
			amount(evtInt(d, "defense_gold"), "gold"))

	case "monarch_deposed":
		monarch, from := evtStr(d, "monarch_id"), evtStr(d, "from_country")
		how := howDeposed(evtStr(d, "reason"))
		if dest := evtStr(d, "to_country"); dest != "" {
			return fmt.Sprintf("%s of %s %s, and became a merchant in %s.", monarch, from, how, dest)
		}
		return fmt.Sprintf("%s of %s %s, and left the game.", monarch, from, how)

	case "exile_relocated":
		return fmt.Sprintf("%s could not settle in %s, which fell the same phase, and went to %s instead.",
			evtStr(d, "monarch_id"), evtStr(d, "intended"), evtStr(d, "to_country"))

	case "republic_formed":
		return fmt.Sprintf("%s became a merchant republic.", evtStr(d, "country_id"))

	case "republic_fallen":
		return fmt.Sprintf("The republic of %s fell.", evtStr(d, "country_id"))

	case "republic_abandoned":
		return fmt.Sprintf("Every merchant left %s, and the republic was no more.", evtStr(d, "country_id"))

	case "merchant_fled":
		return fmt.Sprintf("Merchant %s fled %s for %s.",
			evtStr(d, "merchant_id"), evtStr(d, "from_country"), evtStr(d, "to_country"))

	case "merchant_arrived":
		return fmt.Sprintf("Merchant %s arrived in %s.", evtStr(d, "merchant_id"), evtStr(d, "country_id"))
	}

	// Everything left over is merchant gold - taxes paid, gold hidden or
	// invested, payouts, a treasury shared among rebels, a republic's army
	// contributions - which no rule reveals while the game is running
	return ""
}
