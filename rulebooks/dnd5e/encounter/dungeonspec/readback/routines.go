// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

// routines says every behaviour definition, as compiled for the first
// creature that runs it: who runs it, its states, what wakes each and where
// it goes, and every row of its table.
func (b *builder) routines() []Group {
	var groups []Group
	for _, def := range sortedKeys(b.spec.BehaviorDefinitions) {
		about := Subject{Kind: "routine", ID: def}
		base := "behaviorDefinitions." + def
		g := Group{Title: "Routine " + quote(def), About: about}
		add := func(s Statement) { g.Statements = append(g.Statements, s) }

		var runners []string
		var inst *behavior.InstanceSpec
		for _, d := range b.spec.Room.Gameplay.MonsterDeclarations {
			bind := b.spec.Room.Gameplay.MonsterBindings[d.ID]
			m := b.monsters[d.ID]
			if bind.Behaviors == nil || m == nil || m.Behaviors == nil {
				continue
			}
			for k, use := range *bind.Behaviors {
				if use.Use != def || k >= len(m.Behaviors.Instances) {
					continue
				}
				runners = append(runners, d.ID)
				if inst == nil {
					inst = &m.Behaviors.Instances[k].Instance
				}
			}
		}
		if inst == nil {
			b.unsaid = append(b.unsaid, Unsaid{Path: base, Kind: "routine",
				Why: "no creature runs it, so it is not in the compiled encounter to be said"})
			continue
		}
		who := b.creatures(runners)
		if len(runners) > 1 {
			who = "the creature running it"
		}
		var rc []Connection
		for _, r := range runners {
			rc = append(rc, Connection{Kind: "creature", ID: r, Role: "runs"})
		}
		add(b.stmt(KindRoutine, about, base, fmt.Sprintf(
			"The routine %s is run by %s; it starts in the state %s. When it acts from a set of rows, one row whose conditions hold is picked by weight.",
			quote(def), b.creatures(runners), quote(inst.Recipe.Initial)), rc...))

		for _, state := range statesInOrder(inst.Recipe) {
			st := inst.Recipe.States[state]
			spath := base + ".recipe.states." + state
			if st.Terminal {
				add(b.stmt(KindRoutineState, about, spath, fmt.Sprintf("The state %s is final.", quote(state))))
			} else {
				add(b.stmt(KindRoutineState, about, spath, fmt.Sprintf(
					"In the state %s, %s listens for %s.", quote(state), who, count(len(st.Transitions), "beat", "beats"))))
			}
			for k, tr := range st.Transitions {
				s := fmt.Sprintf("In the state %s, %s", quote(state), beatWords(tr.On, who))
				cond, conns := guardWords(tr.When, who)
				if cond != "" {
					s += ", if " + cond
				}
				s += ", " + who
				var does []string
				if tr.To != "" {
					does = append(does, "moves to the state "+quote(tr.To))
				}
				does = append(does, actWords(tr.Do, who)...)
				if len(does) == 0 {
					does = append(does, "hears it and does nothing")
				}
				s += " " + joinAnd(does)
				if tr.Reason != "" {
					s += fmt.Sprintf(" (%s)", said(tr.Reason))
				}
				add(b.stmt(KindRoutineWake, about, fmt.Sprintf("%s.transitions[%d]", spath, k), s+".", conns...))
			}
		}
		keys := make([]string, 0, len(inst.Table))
		for k := range inst.Table {
			keys = append(keys, string(k))
		}
		sort.Strings(keys)
		for _, key := range keys {
			for i, row := range inst.Table[behavior.AnswerKey(key)] {
				words, conns := b.rowWords(row, who, "everyone who sees "+who+" or hears the voice")
				add(b.stmt(KindRoutineRow, about, fmt.Sprintf("%s.table.%s[%d]", base, key, i), fmt.Sprintf(
					"%s row %d of %s: %s", capitalize(quote(key)), i+1, quote(def), period(words)), conns...))
			}
		}
		groups = append(groups, g)
	}
	return groups
}
