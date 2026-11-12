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

// shortwords.go is a routine's vocabulary in the folded form: the same
// beats, guards, acts, conditions and rows as routinewords.go, said tersely
// under a heading that names the routine's runner, so "it" is the runner.

// shortWake says one transition: "on a fact, if it knows `x` → `turned`
// (“the garrison turned”)".
func shortWake(tr machine.Transition) string {
	s := shortBeat(tr.On)
	if cond, _ := guardWords(tr.When, "it"); cond != "" {
		s += ", if " + cond
	}
	var does []string
	if tr.To != "" {
		does = append(does, "to "+quote(tr.To))
	}
	for _, a := range tr.Do {
		if a.Pick != "" {
			does = append(does, "acts from its "+quote(a.Pick)+" rows")
		}
		if a.Anchor {
			does = append(does, "sets its anchor")
		}
		if a.WakeIn > 0 {
			does = append(does, fmt.Sprintf("wakes again in %d rounds", a.WakeIn))
		}
		if a.Set != nil {
			does = append(does, fmt.Sprintf("sets %s to %v", quote(a.Set.Key), a.Set.Value))
		}
		for _, k := range a.Clear {
			does = append(does, "clears "+quote(k))
		}
	}
	if len(does) == 0 {
		does = append(does, "nothing")
	}
	s += " → " + joinAnd(does)
	if tr.Reason != "" {
		s += fmt.Sprintf(" (%s)", said(tr.Reason))
	}
	return s
}

// shortBeat says a beat tersely.
func shortBeat(on machine.Beat) string {
	switch on {
	case "time":
		return "on its time (out of a fight)"
	case "turn":
		return "on its turn in a fight"
	case "fact":
		return "on learning a fact"
	case "topic":
		return "on coming to hold a topic about someone (judged or heard)"
	case "approached":
		return "when it is the carrier that hails a character (only under the conversation's hail rule)"
	case "barred":
		return "when someone walks into it and its body stops them"
	case "heard":
		return "on hearing a sound"
	case "seen":
		return "when an opposed creature comes into its sight"
	case "clock:fight":
		return "on each round of a fight its side is in"
	case "clock:world":
		return "on each round of world time"
	}
	return beatWords(on, "it")
}

// shortEnemy says an enemy band tersely.
func shortEnemy(w behavior.EnemyWord) string {
	switch w {
	case behavior.EnemyReach:
		return "an enemy is in reach"
	case behavior.EnemySeen:
		return "it sees an enemy, none in reach"
	case behavior.EnemyRemembered:
		return "it remembers an enemy it cannot see"
	case behavior.EnemyLost:
		return "it has lost the trail"
	case behavior.EnemyNone:
		return "it knows of no enemy"
	case behavior.EnemyWalk:
		return "a walk reaches an enemy"
	case behavior.EnemyDash:
		return "only a run reaches an enemy"
	case behavior.EnemyFar:
		return "not even a run reaches an enemy"
	}
	return enemyWords(w, "it")
}

// shortWhen says a row condition tersely.
func (b *builder) shortWhen(w behavior.When) string {
	switch {
	case w.Not != nil:
		return "NOT (" + b.shortWhen(*w.Not) + ")"
	case w.Enemy != "":
		return shortEnemy(w.Enemy)
	case w.Heard != nil:
		return fmt.Sprintf("it heard a %s%s within %d rounds", quote(w.Heard.Kind.String()), raisedWords(w.Heard.Loudness), w.Heard.Within)
	case !w.Fact.IsZero():
		return "it knows " + quote(w.Fact.String())
	}
	s, _ := b.whenWords(w, "it")
	return s
}

// shortSelector says where a move goes, tersely.
func shortSelector(s *behavior.Selector) string {
	if s.At != nil {
		return posWords(*s.At)
	}
	if len(s.Path) > 0 {
		return selectorWords(s, "it")
	}
	return selectorWords(s, "it")
}

// shortRow says one table row tersely: "if it knows of no enemy → walks to
// q 3, r -9".
func (b *builder) shortRow(a behavior.Answer) string {
	var conds []string
	for _, w := range a.When {
		conds = append(conds, b.shortWhen(w))
	}
	var does []string
	if a.Say != "" {
		does = append(does, speechVerb(a.Loudness)+" "+said(a.Say))
	}
	if a.Activity != nil {
		does = append(does, fmt.Sprintf("plays %s %d times", quote(a.Activity.Ref), a.Activity.Cycles))
	}
	if a.Attack != nil {
		does = append(does, "attacks "+shortSelector(a.Attack))
	}
	if a.Toward != nil {
		does = append(does, "walks to "+shortSelector(a.Toward))
	}
	if a.Dash != nil {
		does = append(does, "runs to "+shortSelector(a.Dash))
	}
	if a.Away != nil {
		does = append(does, "moves away from "+shortSelector(a.Away))
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
			inner = append(inner, b.shortRow(r))
		}
		group := "one of these by weight"
		if a.From != "" {
			group = "one of the bundle " + quote(a.From) + "'s rows by weight"
			if len(a.Params) > 0 {
				keys := make([]string, 0, len(a.Params))
				for k := range a.Params {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var ps []string
				for _, k := range keys {
					ps = append(ps, k+" = "+a.Params[k])
				}
				group += " (" + strings.Join(ps, ", ") + ")"
			}
		}
		// A bundle's rows are said in full the first time; a later use with
		// the same rows names where they were said.
		key := group + ": [" + strings.Join(inner, "; ") + "]"
		if where, said := b.bundleSaid[key]; said && where != b.routineNow && a.From != "" {
			does = append(does, group+", its rows as said under the routine "+quote(where))
		} else {
			if !said && a.From != "" {
				b.bundleSaid[key] = b.routineNow
			}
			does = append(does, key)
		}
	}
	if len(does) == 0 {
		does = append(does, "does nothing")
	}
	out := joinAnd(does)
	if a.Fact != "" {
		out += ", and everyone its words reach learns " + quote(a.Fact)
	}
	if a.As != "" {
		out += " (counted as " + quote(a.As) + ")"
	}
	if len(conds) > 0 {
		out = "if " + strings.Join(conds, " and ") + " → " + out
	}
	if a.Weight > 1 {
		out += fmt.Sprintf(" (weight %d)", a.Weight)
	}
	return out
}
