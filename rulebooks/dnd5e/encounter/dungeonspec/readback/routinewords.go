// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// routinewords.go says a routine's vocabulary — its beats, guards, acts,
// conditions and rows — always about a named actor (who), never "it".

// whose is the possessive of an actor's name.
func whose(who string) string { return who + "'s" }

// beatWords says when a routine hears a beat.
func beatWords(on machine.Beat, who string) string {
	switch on {
	case "time":
		return fmt.Sprintf("on %s own time (each round %s is due, out of a fight)", whose(who), who)
	case "turn":
		return fmt.Sprintf("on %s turn in a fight", whose(who))
	case "fact":
		return fmt.Sprintf("when %s learns a fact", who)
	case "topic":
		return fmt.Sprintf("when %s comes to hold a topic about someone, by its own judgement or by hearing it", who)
	case "approached":
		return fmt.Sprintf("when %s hails a character, which happens only under the conversation's hail rule: the "+
			"character's own step ended within the approach distance, %s sees them and they see %s, %s offers them a line "+
			"that steps the conversation, no nearer carrier qualifies, and the conversation is addressing no one", who, who, who, who)
	case "barred":
		return fmt.Sprintf("when someone walks into %s and its body stops them", who)
	case "heard":
		return fmt.Sprintf("when %s hears a sound", who)
	case "seen":
		return fmt.Sprintf("when an opposed creature comes into %s sight", whose(who))
	case "deed":
		return "when a deed is done"
	case "clock:fight":
		return fmt.Sprintf("on each round of a fight %s side is in", whose(who))
	case "clock:world":
		return "on each round of world time (out of a fight)"
	}
	if s, ok := strings.CutPrefix(string(on), "deed:"); ok {
		return fmt.Sprintf("when the deed %s is done", quote(s))
	}
	return fmt.Sprintf("on the beat %s", quote(string(on)))
}

// guardWords says a transition's guard as a condition clause, with the
// topics and facts it reads.
func guardWords(g *machine.Guard, who string) (string, []Connection) {
	if g == nil {
		return "", nil
	}
	var parts []string
	var conns []Connection
	add := func(s string, c []Connection) { parts = append(parts, s); conns = append(conns, c...) }
	if g.Fact != nil {
		name := string(g.Fact.Name)
		val := fmt.Sprint(g.Fact.Value)
		switch {
		case strings.HasPrefix(name, "fact:") && g.Fact.Op == "eq" && val == "true":
			id := strings.TrimPrefix(name, "fact:")
			add(fmt.Sprintf("%s knows the fact %s", who, quote(id)), []Connection{{Kind: "fact", ID: id, Role: "wakes-on"}})
		case name == "heard.kind" && g.Fact.Op == "eq":
			add(fmt.Sprintf("what %s heard was a %s", who, quote(val)), nil)
		case strings.HasPrefix(name, "heard.loudness:") && g.Fact.Op == "eq" && val == "true":
			add(fmt.Sprintf("what %s heard was a voice%s (a clang, a crash or a boom is at no loudness)",
				who, raisedWords(strings.TrimPrefix(name, "heard.loudness:"))), nil)
		case strings.HasPrefix(name, "topic:") && g.Fact.Op == "eq" && val == "true":
			id := strings.TrimPrefix(name, "topic:")
			add(fmt.Sprintf("%s holds the topic %s about anyone (%s recognises someone as %s)", who, quote(id), who, quote(id)),
				[]Connection{{Kind: "topic", ID: id, Role: "wakes-on"}})
		default:
			add(fmt.Sprintf("%s %s is %s %s", whose(who), quote(name), opWords(g.Fact.Op), val), nil)
		}
	}
	if g.Flag != "" {
		add(fmt.Sprintf("%s flag %s is set", whose(who), quote(g.Flag)), nil)
	}
	if g.Scope != nil {
		if g.Scope.Within > 0 {
			add(fmt.Sprintf("it happened within %d cells of %s", g.Scope.Within, who), nil)
		}
		if g.Scope.Regions > 0 {
			add(fmt.Sprintf("it happened within %d rooms of %s", g.Scope.Regions, who), nil)
		}
	}
	if g.Rounds != nil {
		add(fmt.Sprintf("%s rounds have passed since %s set its anchor", opWords(g.Rounds.Op)+" "+fmt.Sprint(g.Rounds.N), who), nil)
	}
	for i := range g.All {
		s, c := guardWords(&g.All[i], who)
		add(s, c)
	}
	if len(g.Any) > 0 {
		var any []string
		for i := range g.Any {
			s, c := guardWords(&g.Any[i], who)
			any = append(any, s)
			conns = append(conns, c...)
		}
		parts = append(parts, "("+strings.Join(any, ", or ")+")")
	}
	if g.Not != nil {
		s, c := guardWords(g.Not, who)
		add("NOT ("+s+")", c)
	}
	return strings.Join(parts, " and "), conns
}

// opWords says a comparison.
func opWords(op machine.Op) string {
	switch op {
	case "gte":
		return "at least"
	case "gt":
		return "more than"
	case "lte":
		return "at most"
	case "lt":
		return "fewer than"
	case "eq":
		return "exactly"
	}
	return string(op)
}

// actWords says what a transition does, as verb phrases without a subject.
func actWords(acts []machine.Act, who string) []string {
	var out []string
	for _, a := range acts {
		if a.Pick != "" {
			out = append(out, fmt.Sprintf("acts from its own %s rows", quote(a.Pick)))
		}
		if a.Anchor {
			out = append(out, "sets its anchor to this moment")
		}
		if a.WakeIn > 0 {
			out = append(out, fmt.Sprintf("wakes again in %d rounds", a.WakeIn))
		}
		if a.Set != nil {
			out = append(out, fmt.Sprintf("sets %s to %v", quote(a.Set.Key), a.Set.Value))
		}
		for _, k := range a.Clear {
			out = append(out, fmt.Sprintf("clears %s", quote(k)))
		}
	}
	return out
}

// enemyWords says an enemy band.
func enemyWords(w behavior.EnemyWord, who string) string {
	switch w {
	case behavior.EnemyReach:
		return fmt.Sprintf("an enemy is in %s reach", whose(who))
	case behavior.EnemySeen:
		return fmt.Sprintf("%s sees an enemy and none is in its reach", who)
	case behavior.EnemyRemembered:
		return fmt.Sprintf("%s sees no enemy but remembers where one was", who)
	case behavior.EnemyLost:
		return fmt.Sprintf("%s has lost an enemy's trail (went where it last saw them and found nobody)", who)
	case behavior.EnemyNone:
		return fmt.Sprintf("%s knows of no enemy (none in reach, seen, remembered or lost)", who)
	case behavior.EnemyWalk:
		return fmt.Sprintf("a walk this turn puts an enemy in %s reach", whose(who))
	case behavior.EnemyDash:
		return fmt.Sprintf("only a run this turn puts an enemy in %s reach", whose(who))
	case behavior.EnemyFar:
		return fmt.Sprintf("not even a run this turn puts an enemy in %s reach", whose(who))
	}
	return fmt.Sprintf("%s enemy band is %s", whose(who), quote(string(w)))
}

// whenWords says one row condition.
func (b *builder) whenWords(w behavior.When, who string) (string, []Connection) {
	switch {
	case w.Not != nil:
		s, c := b.whenWords(*w.Not, who)
		return "NOT (" + s + ")", c
	case w.Enemy != "":
		return enemyWords(w.Enemy, who), nil
	case w.Heard != nil:
		return fmt.Sprintf("%s heard a %s sound%s within the last %d rounds", who, quote(w.Heard.Kind.String()),
			raisedWords(w.Heard.Loudness), w.Heard.Within), nil
	case !w.Fact.IsZero():
		id := w.Fact.String()
		return fmt.Sprintf("%s knows the fact %s", who, quote(id)), []Connection{{Kind: "fact", ID: id, Role: "asks"}}
	case w.Threat != "":
		return fmt.Sprintf("the threat at the cell %s move lands on is %s", whose(who), quote(string(w.Threat))), nil
	case w.Deed != "":
		target := who
		switch w.Scope {
		case behavior.ScopeAlly:
			target = "a creature on " + whose(who) + " side"
		case behavior.ScopeActor:
			return fmt.Sprintf("%s did the deed %s itself within the last %d rounds", who, quote(w.Deed), w.Within), nil
		}
		if w.Who != "" {
			target = b.creature(string(w.Who))
			if w.Scope == behavior.ScopeAlly {
				target += ", on " + whose(who) + " side,"
			}
		}
		return fmt.Sprintf("%s holds that the deed %s was done to %s within the last %d rounds", who, quote(w.Deed), target, w.Within), nil
	}
	return "a condition this build cannot say", nil
}

// selectorWords says where a move goes.
func selectorWords(s *behavior.Selector, who string) string {
	switch {
	case s.At != nil:
		return "the cell " + posWords(*s.At)
	case len(s.Path) > 0:
		stops := make([]string, len(s.Path))
		for i, p := range s.Path {
			stop := posWords(p)
			var extra []string
			if i < len(s.Waits) && s.Waits[i] > 0 {
				extra = append(extra, fmt.Sprintf("waits %d round", s.Waits[i]))
			}
			if i < len(s.Facings) && s.Facings[i] != nil {
				extra = append(extra, fmt.Sprintf("facing %g°", *s.Facings[i]))
			}
			if len(extra) > 0 {
				stop += " (" + strings.Join(extra, ", ") + ")"
			}
			stops[i] = stop
		}
		return "the route " + strings.Join(stops, " → ") + ", then round again"
	}
	switch s.Word {
	case behavior.SelectorEnemy:
		return "the enemy"
	case behavior.SelectorAttacker:
		return "whoever attacked " + who
	case behavior.SelectorActor:
		return "the one who acted"
	case behavior.SelectorSound:
		return "the sound " + who + " heard"
	case behavior.SelectorAlly:
		return "the nearest ally of " + who
	}
	return quote(string(s.Word))
}

// rowWords says one table row about who, recursing into a group's rows,
// with the facts it teaches and asks. teach says who learns a row's fact.
func (b *builder) rowWords(a behavior.Answer, who string) (string, []Connection) {
	var conds []string
	var conns []Connection
	for _, w := range a.When {
		s, c := b.whenWords(w, who)
		conds = append(conds, s)
		conns = append(conns, c...)
	}
	var does []string
	if a.Say != "" {
		does = append(does, fmt.Sprintf("%s %s (at %s: its words reach %s)", speechVerb(a.Loudness), said(a.Say),
			quote(loudnessOf(a.Loudness)), reachWords(a.Loudness, who)))
	}
	if a.Activity != nil {
		does = append(does, fmt.Sprintf("plays the activity %s %d times", quote(a.Activity.Ref), a.Activity.Cycles))
	}
	if a.Attack != nil {
		does = append(does, "attacks "+selectorWords(a.Attack, who))
	}
	if a.Toward != nil {
		does = append(does, "walks toward "+selectorWords(a.Toward, who))
	}
	if a.Dash != nil {
		does = append(does, "runs (dashes) toward "+selectorWords(a.Dash, who))
	}
	if a.Away != nil {
		does = append(does, "moves away from "+selectorWords(a.Away, who))
	}
	if a.Flee {
		does = append(does, "flees")
	}
	if a.Hold {
		does = append(does, "holds where it stands")
	}
	if a.Search != nil {
		does = append(does, fmt.Sprintf("searches %d cells on, heading %s", a.Search.Distance, quote(string(a.Search.Heading))))
	}
	if len(a.Rows) > 0 {
		var inner []string
		for _, r := range a.Rows {
			s, c := b.rowWords(r, who)
			inner = append(inner, s)
			conns = append(conns, c...)
		}
		group := "does one of these rows, by weight"
		if a.From != "" {
			group = fmt.Sprintf("does one of the bundle %s's rows, by weight", quote(a.From))
			conns = append(conns, Connection{Kind: "bundle", ID: a.From, Role: "uses"})
			if len(a.Params) > 0 {
				keys := make([]string, 0, len(a.Params))
				for k := range a.Params {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				ps := make([]string, len(keys))
				for i, k := range keys {
					ps[i] = k + " = " + a.Params[k]
				}
				group += " (with " + strings.Join(ps, ", ") + ")"
			}
		}
		does = append(does, group+": ["+strings.Join(inner, "; ")+"]")
	}
	if len(does) == 0 {
		does = append(does, "does nothing")
	}
	out := who + " " + joinAnd(does)
	if a.Fact != "" {
		out += fmt.Sprintf(", and %s learns the fact %s", learnersWords(a.Loudness, who), quote(a.Fact))
		conns = append(conns, Connection{Kind: "fact", ID: a.Fact, Role: "teaches"})
	}
	if a.As != "" {
		out += fmt.Sprintf(" (counted as %s)", quote(a.As))
	}
	if len(conds) > 0 {
		out = "if " + strings.Join(conds, " and ") + ", " + out
	}
	if a.Weight > 1 {
		out += fmt.Sprintf(" (weight %d)", a.Weight)
	}
	return out, conns
}
