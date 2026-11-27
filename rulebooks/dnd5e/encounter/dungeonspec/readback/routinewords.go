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
func whose(who string) string {
	if who == "it" {
		return "its"
	}
	return who + "'s"
}

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
// topics and facts it reads and one [ConnCondition] per condition. on is the
// transition's beat: who a `noted:<topic>` asks about is the one that beat
// names.
func guardWords(g *machine.Guard, who string, on machine.Beat) (string, []Connection) {
	return guardWordsAt(g, who, on, "when", false)
}

// guardWordsAt is [guardWords] for the node at at, under an odd number of
// `not`s when not is true.
func guardWordsAt(g *machine.Guard, who string, on machine.Beat, at string, not bool) (string, []Connection) {
	if g == nil {
		return "", nil
	}
	var parts []string
	var conns []Connection
	add := func(s string, c []Connection) { parts = append(parts, s); conns = append(conns, c...) }
	asks := func(id, is string, op machine.Op) Connection {
		c := Connection{Kind: ConnCondition, ID: id, Role: "asks", Is: is, Not: not, At: at}
		if op != machine.OpEq {
			c.Op = string(op)
		}
		return c
	}
	if g.Fact != nil {
		name := string(g.Fact.Name)
		val := fmt.Sprint(g.Fact.Value)
		word, id, known := encounter.SplitConditionWord(name)
		verb := "has noted"
		if !boolTestTrue(val) {
			verb = "has not noted"
		}
		condID := name
		if known {
			condID = word.Word + id
		}
		self := asks(condID, val, g.Fact.Op)
		say := func(c ...Connection) {
			add(fill(word.Long, "who", who, "whose", whose(who), "id", quote(id), "verb", verb,
				"about", aboutWords(on, who), "raised", raisedWords(id)), append([]Connection{self}, c...))
		}
		switch {
		case word.Open == encounter.SlotFact && g.Fact.Op == "eq" && val == "true":
			say(Connection{Kind: "fact", ID: id, Role: "wakes-on"})
		case word.Word == "heard.kind" && g.Fact.Op == "eq":
			add(fill(word.Long, "who", who, "value", quote(val)), []Connection{self})
		case word.Open == encounter.SlotLoudness && g.Fact.Op == "eq" && val == "true":
			say()
		case word.Word == "heard.claim" && g.Fact.Op == "eq":
			add(fill(word.Long, "who", who, "value", val), []Connection{self})
		case word.Word == "stance" && g.Fact.Op == "eq":
			// A STANCE ASKED (rpg-project#1125): the one the beat names, and
			// the value's own label from the words table (the value itself
			// when the row has none for it). A stance is not a note: no edges.
			label, ok := word.ValueLabels[val]
			if !ok {
				label = val
			}
			add(fill(word.Long, "about", aboutWords(on, who), "value", label), []Connection{self})
		case word.Word == "noted:" && isBoolTest(g.Fact.Op, val):
			// WHOM IT ASKS ABOUT AND ITS POLARITY (rpg-project#1001): a note
			// asked false, or under an odd number of nots, is negated.
			say(noteAskEdges(id, "", "asks", not != !boolTestTrue(val), encounter.WhoTheOneNamed)...)
		case word.Word == "noted.as:" && g.Fact.Op == "eq":
			// A NOTE ASKED BY ITS VALUE (rpg-project#1007): the topic and the
			// value it must carry, about the one the beat names, said as
			// `noted:` is and connected as an ask of that topic.
			add(fill(word.Long, "who", who, "verb", "has noted", "id", quote(id), "value", quote(val), "about", aboutWords(on, who)),
				append([]Connection{self}, noteAskEdges(id, val, "asks", not, encounter.WhoTheOneNamed)...))
		case word.Word == "noted.anyone:" && isBoolTest(g.Fact.Op, val):
			// `noted.anyone:<topic>` as authored, the engine's `topic:<id>`
			// once compiled (rpg-project#1005).
			say(noteAskEdges(id, "", "wakes-on", not != !boolTestTrue(val), encounter.WhoAnyone)...)
		default:
			add(fill(cond("enemy").Long, "whose", whose(who), "name", quote(name), "op", opWords(g.Fact.Op), "value", val),
				[]Connection{self})
		}
	}
	if g.Flag != "" {
		add(fill(cond("flag:").Long, "whose", whose(who), "id", quote(g.Flag)), []Connection{asks(cond("flag:").Word+g.Flag, "", machine.OpEq),
			{Kind: ConnFlag, ID: g.Flag, Role: "asks"}})
	}
	if g.Scope != nil {
		if g.Scope.Within > 0 {
			add(fill(cond("within").Long, "n", fmt.Sprint(g.Scope.Within), "who", who),
				[]Connection{asks("within", fmt.Sprint(g.Scope.Within), encounter.OpAtMost)})
		}
		if g.Scope.Regions > 0 {
			add(fill(cond("regions").Long, "n", fmt.Sprint(g.Scope.Regions), "who", who),
				[]Connection{asks("regions", fmt.Sprint(g.Scope.Regions), encounter.OpAtMost)})
		}
	}
	if g.Rounds != nil {
		said := fill(cond("rounds").Long, "op", opWords(g.Rounds.Op), "n", fmt.Sprint(g.Rounds.N), "who", who)
		if g.Rounds.Op == machine.OpEq && g.Rounds.N == 0 {
			// `rounds: { op: eq }` with no n (rpg-project#1001): the very
			// round it marked the moment, said so rather than as "exactly 0".
			said = "it is the very round " + who + " set its anchor"
		}
		add(said, []Connection{asks("rounds", fmt.Sprint(g.Rounds.N), g.Rounds.Op)})
	}
	for i := range g.All {
		s, c := guardWordsAt(&g.All[i], who, on, fmt.Sprintf("%s.all[%d]", at, i), not)
		add(s, c)
	}
	if len(g.Any) > 0 {
		var any []string
		for i := range g.Any {
			s, c := guardWordsAt(&g.Any[i], who, on, fmt.Sprintf("%s.any[%d]", at, i), not)
			any = append(any, s)
			conns = append(conns, c...)
		}
		parts = append(parts, fill(cond("any").Long, "parts", strings.Join(any, cond("any").Join)))
	}
	if g.Not != nil {
		s, c := guardWordsAt(g.Not, who, on, at+".not", !not)
		add(fill(cond("not").Long, "inner", s), c)
	}
	return fill(cond("all").Long, "parts", strings.Join(parts, cond("all").Join)), conns
}

// aboutWords is the one a beat names, as a `noted:<topic>` asks about them
// (rpg-project#1000).
func aboutWords(on machine.Beat, who string) string {
	if b, ok := encounter.BeatWordOf(string(on)); ok && b.Names {
		return fill(b.About, "who", who)
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

// opWords says a comparison, in the machine words table's plain words.
func opWords(op machine.Op) string {
	if w, ok := encounter.OpWordsOf(string(op)); ok {
		return w
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
	case w.HitPoints != nil:
		return fmt.Sprintf("%s hit points are at most %d%% of %s maximum", whose(who), w.HitPoints.AtMost, whose(who)), nil
	case w.Deed != "":
		return b.deedWords(w, who), nil
	}
	return "a condition this build cannot say", nil
}

// selectorWords says where a move goes. actor is who `actor` names for the
// row's deed ([builder.actorWords]).
func selectorWords(s *behavior.Selector, who, actor string) string {
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
		return actor
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
		does = append(does, "attacks "+selectorWords(a.Attack, who, b.actorWords(a)))
	}
	if a.Toward != nil {
		does = append(does, "walks toward "+selectorWords(a.Toward, who, b.actorWords(a)))
	}
	if a.Dash != nil {
		does = append(does, "runs (dashes) toward "+selectorWords(a.Dash, who, b.actorWords(a)))
	}
	if a.Activate != "" {
		does = append(does, "activates "+said(a.Activate)+", when it has that ability and can pay for it")
		conns = append(conns, Connection{Kind: ConnAbility, ID: a.Activate, Role: RoleActivates})
	}
	if a.Away != nil {
		does = append(does, "moves away from "+selectorWords(a.Away, who, b.actorWords(a)))
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

// The words for each deed a row can ask: what doing it is ("attack") and
// what having done it is ("attacked"). A deed word with no words here is
// said by [builder.deedWords]'s fallback, and a test walks every `when`
// word so none ships without them.
var deedVerbs = map[string]struct{ do, did string }{
	"attacked":    {"attack", "attacked"},
	"intimidated": {"intimidate", "intimidated"},
	"persuaded":   {"persuade", "persuaded"},
	"fled":        {"flee", "fled"},
	"fell":        {"fall", "felled"},
}

// deedWords says a row's deed condition about who: what happened and how
// who knows it, from how that deed reaches a creature in the relation the
// condition reads ([encounter.DeedWayOf]). A deed is held as the creature
// saw it: it knows who did it, and to whom, only if it saw them. "learned"
// is the word for a deed that was told, which none is yet (word spreading,
// rpg-project#992).
func (b *builder) deedWords(w behavior.When, who string) string {
	way, known := encounter.DeedWayOf(w.Deed)
	v, said := deedVerbs[w.Deed]
	if !known || !said {
		return fmt.Sprintf("a deed condition on %s this build cannot say", quote(w.Deed))
	}
	if why := encounter.NeverTrue(w.Deed, w.Scope); why != "" {
		return "never true: " + why
	}
	n := w.Within
	k := ""
	if w.Who != "" {
		k = b.creature(string(w.Who))
	}
	switch w.Scope {
	case behavior.ScopeActor:
		switch {
		case w.Deed == "fell" && k != "":
			return fmt.Sprintf("%s felled %s and saw it fall within the last %d rounds", who, k, n)
		case w.Deed == "fell":
			return fmt.Sprintf("%s felled someone, or fell with no blow to explain it, within the last %d rounds", who, n)
		}
		return fmt.Sprintf("%s %s someone within the last %d rounds", who, v.did, n)
	case behavior.ScopeAlly:
		target := "a creature on " + whose(who) + " side"
		if k != "" {
			target = k + ", on " + whose(who) + " side,"
		}
		if w.Deed == "fell" {
			return fmt.Sprintf("%s saw %s fall within the last %d rounds", who, target, n)
		}
		return fmt.Sprintf("%s saw someone %s %s within the last %d rounds", who, v.do, target, n)
	}
	if way.Target == encounter.ReachOwn {
		switch {
		case w.Deed == "fled":
			return fmt.Sprintf("%s fled from someone within the last %d rounds", who, n)
		case k != "" && k != who:
			return fmt.Sprintf("%s fell within the last %d rounds; `who` names %s, so this is true only if that is %s", who, n, k, who)
		}
		return fmt.Sprintf("%s fell within the last %d rounds", who, n)
	}
	return fmt.Sprintf("%s saw someone %s %s within the last %d rounds", who, v.do, who, n)
}

// actorWords is who a row's `actor` selector names: the one who did the
// deed its condition reads, as the creature knows them; "the one who acted"
// for a row with no deed condition.
func (b *builder) actorWords(a behavior.Answer) string {
	var d *behavior.When
	for i := range a.When {
		if a.When[i].Deed != "" {
			d = &a.When[i]
			break
		}
	}
	if d == nil {
		return "the one who acted"
	}
	v, said := deedVerbs[d.Deed]
	if !said {
		return "the one who acted"
	}
	switch {
	case d.Scope == behavior.ScopeActor:
		return "itself"
	case d.Deed == "fled":
		return "the one it fled from"
	}
	object := "it"
	if d.Scope == behavior.ScopeAlly {
		object = "a creature on its side"
		if d.Who != "" {
			object = b.creature(string(d.Who))
		}
	}
	return "the one who " + v.did + " " + object
}
