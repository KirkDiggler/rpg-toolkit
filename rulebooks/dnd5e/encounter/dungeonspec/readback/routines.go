// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// routines says every behaviour definition, as compiled for the first
// creature that runs it: who runs it, its states, what wakes each and where
// it goes, and every row of its table.
func (b *builder) routines() []Group {
	var groups []Group
	for _, def := range sortedKeys(b.spec.BehaviorDefinitions) {
		about := Subject{Kind: "routine", ID: def}
		b.routineNow = def
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
		rc = append(rc, Connection{Kind: ConnState, ID: inst.Recipe.Initial, Role: RoleStarts})
		add(b.fstmt(KindRoutine, about, base, fmt.Sprintf(
			"The routine %s is run by %s; it starts in the state %s; its recipe is %s. When it acts from a set of rows, one row whose conditions are true is picked by weight.",
			quote(def), b.creatures(runners), quote(inst.Recipe.Initial), quote(inst.Recipe.Ref.String())),
			foldInfo{text: fmt.Sprintf("run by %s; starts in %s; recipe %s.", b.creatures(runners), quote(inst.Recipe.Initial),
				quote(inst.Recipe.Ref.String()))}, rc...))

		// WHAT THE MACHINE REMEMBERS (rpg-project#1001): each flag its bag
		// declares, its kind and the value it starts at.
		for _, flag := range sortedKeys(inst.Recipe.Bag) {
			start := inst.Recipe.Bag[flag]
			literal, _ := json.Marshal(start)
			add(b.fstmt(KindRoutineFlag, about, base+".recipe.bag."+flag, fmt.Sprintf(
				"The routine %s remembers the flag %s, a %s that starts as %s.", quote(def), quote(flag), flagKind(start), string(literal)),
				foldInfo{text: fmt.Sprintf("Remembers the flag %s, a %s that starts as %s.", quote(flag), flagKind(start), string(literal))},
				Connection{Kind: ConnFlag, ID: flag, Role: "declares", Is: string(literal)}))
		}

		for _, state := range statesInOrder(inst.Recipe) {
			st := inst.Recipe.States[state]
			spath := base + ".recipe.states." + state
			is := Connection{Kind: ConnState, ID: state, Role: RoleIs}
			if st.Terminal {
				add(b.fstmt(KindRoutineState, about, spath, fmt.Sprintf("The state %s is final.", quote(state)),
					foldInfo{state: state, text: "final"}, is, Connection{Kind: ConnState, ID: state, Role: RoleFinal}))
			} else {
				add(b.fstmt(KindRoutineState, about, spath, fmt.Sprintf(
					"In the state %s, %s listens for %s.", quote(state), who, count(len(st.Transitions), "beat", "beats")),
					foldInfo{state: state}, is))
			}
			for k, tr := range st.Transitions {
				s, conns := wakeWords(tr, who)
				conns = append(conns, stepShape(state, tr)...)
				add(b.fstmt(KindRoutineWake, about, fmt.Sprintf("%s.transitions[%d]", spath, k),
					fmt.Sprintf("In the state %s, %s.", quote(state), s),
					foldInfo{state: state, text: shortWake(tr) + ".", site: "the routine " + quote(def)}, conns...))
			}
			for _, st := range b.stateOutcomes(def, state, inst.Recipe, who, about, runners) {
				add(st)
			}
		}
		keys := make([]string, 0, len(inst.Table))
		for k := range inst.Table {
			keys = append(keys, string(k))
		}
		sort.Strings(keys)
		for _, key := range keys {
			for i, row := range inst.Table[behavior.AnswerKey(key)] {
				words, conns := b.rowWords(row, who)
				conns = append(conns, Connection{Kind: ConnTable, ID: key, Role: RoleRowOf})
				short := b.shortRow(row)
				add(b.fstmt(KindRoutineRow, about, fmt.Sprintf("%s.table.%s[%d]", base, key, i), fmt.Sprintf(
					"%s row %d of %s: %s", capitalize(quote(key)), i+1, quote(def), period(words)),
					foldInfo{state: key, text: period(short), site: "the routine " + quote(def)}, conns...))
			}
		}
		groups = append(groups, g)
	}
	return groups
}

// stepShape is one step's place in its machine, as connections: the state
// it is heard in, the state it moves to when it moves, the beat it wakes on
// (the authored word) and each table key it picks from.
func stepShape(from string, tr machine.Transition) []Connection {
	conns := []Connection{{Kind: ConnState, ID: from, Role: RoleFrom}}
	if tr.To != "" {
		conns = append(conns, Connection{Kind: ConnState, ID: tr.To, Role: RoleTo})
	}
	conns = append(conns, Connection{Kind: ConnBeat, ID: string(tr.On), Role: RoleOn})
	for _, a := range tr.Do {
		if a.Pick != "" {
			conns = append(conns, Connection{Kind: ConnTable, ID: string(a.Pick), Role: RolePicks})
		}
	}
	return append(conns, actConnections(tr)...)
}

// actConnections is every act of a step as a [ConnAct], in the order the
// author wrote them (the move first), with a [ConnFlag] beside each act
// that writes a flag.
func actConnections(tr machine.Transition) []Connection {
	var out []Connection
	does := func(id, is, at string) {
		out = append(out, Connection{Kind: ConnAct, ID: id, Role: "does", Is: is, At: at})
	}
	if tr.To != "" {
		does(act("to").Word+":"+tr.To, "", "to")
	}
	for j, a := range tr.Do {
		at := fmt.Sprintf("do[%d]", j)
		if a.Pick != "" {
			does(act("pick").Word+":"+a.Pick, "", at)
		}
		if a.WakeIn != 0 {
			does(act("wake_in").Word, fmt.Sprint(a.WakeIn), at)
		}
		if a.Anchor {
			does(act("anchor").Word, "", at)
		}
		if a.Set != nil {
			value, _ := json.Marshal(a.Set.Value)
			does(act("set").Word+":"+a.Set.Key, string(value), at)
			out = append(out, Connection{Kind: ConnFlag, ID: a.Set.Key, Role: "sets"})
		}
		for k, key := range a.Clear {
			does(act("clear").Word+":"+key, "", fmt.Sprintf("%s.clear[%d]", at, k))
			out = append(out, Connection{Kind: ConnFlag, ID: key, Role: "clears"})
		}
	}
	return out
}

// wakeWords says one routine transition about who, without its state.
func wakeWords(tr machine.Transition, who string) (string, []Connection) {
	s := beatWords(tr.On, who)
	cond, conns := guardWords(tr.When, who, tr.On)
	if cond != "" {
		s += ", if " + cond
	}
	s += ", " + who
	var does []string
	if tr.To != "" {
		does = append(does, fill(act("to").Long, "id", quote(tr.To)))
	}
	does = append(does, actWords(tr.Do, false)...)
	if len(does) == 0 {
		does = append(does, "hears it and does nothing")
	}
	s += " " + joinAnd(does)
	if tr.Reason != "" {
		s += fmt.Sprintf(" (%s)", said(tr.Reason))
	}
	// A STATE CHANGE IS TOLD TO THE WHOLE RUN (rpg-project#948): the change,
	// its state names and its reason reach every member, wherever they are,
	// whether or not they saw or heard what caused it — the state beat's
	// audience today.
	if tr.To != "" {
		what := "the change"
		if tr.Reason != "" {
			what = "the change and its reason"
		}
		s += fmt.Sprintf("; %s reach every member of the run, wherever they are, whether or not they saw or heard what caused it", what)
	}
	return s, conns
}

// flagKind is the kind a flag's starting value declares it: "bool", "int"
// or "string", the kinds a machine's bag holds.
func flagKind(v any) string {
	switch v.(type) {
	case bool:
		return encounter.ArgKindBool
	case int, int64, float64:
		return encounter.ArgKindInt
	default:
		return encounter.ArgKindString
	}
}

// stateOutcomes says what a routine's machine gives on entering a state
// (rpg-project#989), one statement per consequence at
// `behaviorDefinitions.<routine>.outcomes.<state>[i]`: the topic it notes,
// about the one each beat entering the state names, connected as any give is
// (noteGiveEdges: the creatures that run THIS routine, `runners`, are the
// takers of `itself`) and to the state it is entered at.
func (b *builder) stateOutcomes(def, state string, recipe machine.Recipe, who string, about Subject, runners []string) []Statement {
	var out []Statement
	var subjects []string
	for _, name := range statesInOrder(recipe) {
		for _, tr := range recipe.States[name].Transitions {
			if tr.To != state {
				continue
			}
			words := aboutWords(tr.On, who)
			if !slices.Contains(subjects, words) {
				subjects = append(subjects, words)
			}
		}
	}
	for i, c := range b.spec.BehaviorDefinitions[def].Outcomes[state] {
		if c.Give == nil {
			continue
		}
		topic := c.Give.Topic
		conns := noteGiveEdges(noteGive{topic: topic, about: encounter.WhoTheOneNamed,
			takers: []noteTaker{{word: c.Give.To, creatures: runners}},
			cause:  Connection{Kind: ConnState, ID: state, Role: "enters"}})
		sentence := fmt.Sprintf("Entering %s, it notes %s about %s.", quote(state), quote(topic), strings.Join(subjects, " or "))
		out = append(out, b.fstmt(KindRoutineOutcome, about, fmt.Sprintf("behaviorDefinitions.%s.outcomes.%s[%d]", def, state, i), sentence,
			foldInfo{state: state, text: fmt.Sprintf("Entering %s, it notes %s about %s.", quote(state), quote(topic), strings.Join(subjects, " or ")), site: "the routine " + quote(def)}, conns...))
	}
	return out
}
