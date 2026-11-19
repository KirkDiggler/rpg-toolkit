// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// routinewords.go says a routine's vocabulary — its beats, guards, acts,
// conditions and rows — always about a named actor (who), never "it".

// whose is the possessive of an actor's name.
func whose(who string) string { return who + "'s" }

// beatWords says when a routine hears a beat: the table's Long
// ([encounter.MachineWords]), about who.
func beatWords(on machine.Beat, who string) string {
	b, ok := encounter.BeatWordOf(string(on))
	if !ok {
		return fmt.Sprintf("on the beat %s", quote(string(on)))
	}
	return beatFill(b.Long, b, on, who)
}

// beatFill fills a beat template's slots.
func beatFill(t string, b encounter.BeatWord, on machine.Beat, who string) string {
	return fill(t, "who", who, "whose", whose(who), "id", quote(strings.TrimPrefix(string(on), b.Word)), "beat", quote(string(on)))
}

// fill replaces a table template's {slots}, given as name, value pairs.
func fill(t string, pairs ...string) string {
	r := make([]string, 0, len(pairs))
	for i := 0; i+1 < len(pairs); i += 2 {
		r = append(r, "{"+pairs[i]+"}", pairs[i+1])
	}
	return strings.NewReplacer(r...).Replace(t)
}

// cond is the table's entry for a condition word that is certainly in it.
func cond(word string) encounter.ConditionWord {
	c, _ := encounter.ConditionWordOf(word)
	return c
}

// act is the table's entry for an act word that is certainly in it.
func act(word string) encounter.ActWord {
	a, _ := encounter.ActWordOf(word)
	return a
}

// guardWords says a transition's guard as a condition clause, with the
// topics and facts it reads. on is the transition's beat: who a
// `noted:<topic>` asks about is the one that beat names.
func guardWords(g *machine.Guard, who string, on machine.Beat) (string, []Connection) {
	if g == nil {
		return "", nil
	}
	var parts []string
	var conns []Connection
	add := func(s string, c []Connection) { parts = append(parts, s); conns = append(conns, c...) }
	if g.Fact != nil {
		name := string(g.Fact.Name)
		val := fmt.Sprint(g.Fact.Value)
		word, _ := encounter.ConditionWordOf(name)
		prefix := word.Word
		if word.Engine != "" && strings.HasPrefix(name, word.Engine) {
			prefix = word.Engine
		}
		id := strings.TrimPrefix(name, prefix)
		verb := "has noted"
		if !boolTestTrue(val) {
			verb = "has not noted"
		}
		say := func(c []Connection) {
			add(fill(word.Long, "who", who, "whose", whose(who), "id", quote(id), "verb", verb,
				"about", aboutWords(on, who), "raised", raisedWords(id)), c)
		}
		switch {
		case word.Open == encounter.SlotFact && g.Fact.Op == "eq" && val == "true":
			say([]Connection{{Kind: "fact", ID: id, Role: "wakes-on"}})
		case word.Word == "heard.kind" && g.Fact.Op == "eq":
			add(fill(word.Long, "who", who, "value", quote(val)), nil)
		case word.Open == encounter.SlotLoudness && g.Fact.Op == "eq" && val == "true":
			say(nil)
		case word.Word == "heard.claim" && g.Fact.Op == "eq":
			add(fill(word.Long, "who", who, "value", val), []Connection{{Kind: ConnClaim, ID: val, Role: "asks"}})
		case word.Word == "noted:" && isBoolTest(g.Fact.Op, val):
			say([]Connection{{Kind: "topic", ID: id, Role: "asks"}})
		case word.Word == "noted.anyone:" && isBoolTest(g.Fact.Op, val):
			// `noted.anyone:<topic>` as authored, the engine's `topic:<id>`
			// once compiled (rpg-project#1005).
			say([]Connection{{Kind: "topic", ID: id, Role: "wakes-on"}})
		default:
			add(fill(cond("enemy").Long, "whose", whose(who), "name", quote(name), "op", opWords(g.Fact.Op), "value", val), nil)
		}
	}
	if g.Flag != "" {
		add(fill(cond("flag").Long, "whose", whose(who), "id", quote(g.Flag)), nil)
	}
	if g.Scope != nil {
		if g.Scope.Within > 0 {
			add(fill(cond("within").Long, "n", fmt.Sprint(g.Scope.Within), "who", who), nil)
		}
		if g.Scope.Regions > 0 {
			add(fill(cond("regions").Long, "n", fmt.Sprint(g.Scope.Regions), "who", who), nil)
		}
	}
	if g.Rounds != nil {
		add(fill(cond("rounds").Long, "op", opWords(g.Rounds.Op), "n", fmt.Sprint(g.Rounds.N), "who", who), nil)
	}
	for i := range g.All {
		s, c := guardWords(&g.All[i], who, on)
		add(s, c)
	}
	if len(g.Any) > 0 {
		var any []string
		for i := range g.Any {
			s, c := guardWords(&g.Any[i], who, on)
			any = append(any, s)
			conns = append(conns, c...)
		}
		parts = append(parts, fill(cond("any").Long, "parts", strings.Join(any, cond("any").Join)))
	}
	if g.Not != nil {
		s, c := guardWords(g.Not, who, on)
		add(fill(cond("not").Long, "inner", s), c)
	}
	return fill(cond("all").Long, "parts", strings.Join(parts, cond("all").Join)), conns
}

// aboutWords is the one a beat names, as a `noted:<topic>` asks about them
// (rpg-project#1000).
func aboutWords(on machine.Beat, who string) string {
	switch on {
	case "heard":
		return "the one " + who + " heard (" + who + " knows who only by seeing them)"
	case "seen":
		return "the one " + who + " saw"
	case "barred":
		return "the one who walked into " + who
	case "approached":
		return "the one " + who + " hailed"
	}
	return "nobody (`" + string(on) + "` is about nobody)"
}

// isBoolTest reports whether a fact leaf compares a yes-or-no fact to a
// yes or a no. The machine has no `ne` (a negation is a `not`).
func isBoolTest(op machine.Op, val string) bool {
	return op == "eq" && (val == "true" || val == "false")
}

// boolTestTrue reports whether a yes-or-no test asks for the fact to be true.
func boolTestTrue(val string) bool {
	return val == "true"
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

// actWords says what a transition does, as verb phrases without a subject:
// each act's Long from the table, or its Short when short.
func actWords(acts []machine.Act, short bool) []string {
	say := func(w encounter.ActWord, pairs ...string) string {
		if short {
			return fill(w.Short, pairs...)
		}
		return fill(w.Long, pairs...)
	}
	var out []string
	for _, a := range acts {
		if a.Pick != "" {
			out = append(out, say(act("pick"), "id", quote(a.Pick)))
		}
		if a.Anchor {
			out = append(out, say(act("anchor")))
		}
		if a.WakeIn > 0 {
			out = append(out, say(act("wake_in"), "n", fmt.Sprint(a.WakeIn)))
		}
		if a.Set != nil {
			out = append(out, say(act("set"), "id", quote(a.Set.Key), "value", fmt.Sprint(a.Set.Value)))
		}
		for _, k := range a.Clear {
			out = append(out, say(act("clear"), "id", quote(k)))
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
		// SAW, because a deed lands only on whoever saw its doer (deed.go
		// audienceOf; a spoken deed on who caught the words and saw the
		// speaker). "learned" is the word for a deed that was told, when
		// word spreads (rpg-project#992).
		return fmt.Sprintf("%s saw the deed %s done to %s within the last %d rounds", who, quote(w.Deed), target, w.Within), nil
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
