// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// lineSite is one line of one creature, with where it is set.
type lineSite struct {
	member string
	option encounter.Option
	path   string
}

// lineSites is every creature line in declaration order, each at its own
// path (a creature's own, else its faction's).
func (b *builder) lineSites() []lineSite {
	var out []lineSite
	for _, d := range b.spec.Room.Gameplay.MonsterDeclarations {
		m := b.monsters[d.ID]
		if m == nil {
			continue
		}
		base := "room.room.monsterBindings." + d.ID + ".options"
		if len(b.spec.Room.Gameplay.MonsterBindings[d.ID].Options) == 0 {
			fi, ok := b.factions[m.Faction]
			if !ok {
				continue
			}
			base = fmt.Sprintf("factions[%d].options", fi)
		}
		for i, o := range m.Options {
			out = append(out, lineSite{member: d.ID, option: o, path: fmt.Sprintf("%s[%d]", base, i)})
		}
	}
	return out
}

// conversations says every root interaction: its carriers, its hail, its
// states and what moves each, what entering a state grants, and which lines
// read it.
func (b *builder) conversations() []Group {
	var groups []Group
	for _, in := range b.c.Field.Interactions {
		id := string(in.ID)
		about := Subject{Kind: "conversation", ID: id}
		base := "interactions." + id
		g := Group{Title: "Conversation " + quote(id), About: about}
		add := func(s Statement) { g.Statements = append(g.Statements, s) }
		carriers := b.carriers[id]
		var cc []Connection
		for _, c := range carriers {
			cc = append(cc, Connection{Kind: "creature", ID: c, Role: "carries"})
		}
		who := "no creature"
		if len(carriers) > 0 {
			who = b.creatures(carriers)
		}
		add(b.fstmt(KindConversation, about, base, fmt.Sprintf(
			"The conversation %s has one state for the whole run, shared by every speaker and every carrier; it starts in %s. "+
				"Its carriers, the creatures whose lines read or step it, are %s.", quote(id), quote(in.Recipe.Initial), who),
			foldInfo{text: fmt.Sprintf("one state for the whole run, shared by every speaker; starts in %s. "+
				"Carried by %s (the creatures whose lines read or step it).", quote(in.Recipe.Initial), who)}, cc...))
		if in.Approach != nil {
			add(b.fstmt(KindApproach, about, base+".approach", fmt.Sprintf(
				"Each carrier (%s) hails a character who comes within %d cells of it in its sight: that carrier hears the beat %s "+
					"and answers from its routine.", who, in.Approach.Within, quote("approached")),
				foldInfo{text: fmt.Sprintf("Each carrier hails a character who comes within %d cells in its sight "+
					"(it hears %s and answers from its routine).", in.Approach.Within, quote("approached"))}))
		}
		for _, state := range statesInOrder(in.Recipe) {
			st := in.Recipe.States[state]
			spath := base + ".recipe.states." + state
			switch {
			case st.Terminal:
				add(b.fstmt(KindConvState, about, spath, fmt.Sprintf(
					"The state %s of %s is final: once entered, nothing moves the conversation again.", quote(state), quote(id)),
					foldInfo{state: state, text: "final"}))
			default:
				add(b.fstmt(KindConvState, about, spath, fmt.Sprintf(
					"In the state %s, the conversation %s listens for %s.", quote(state), quote(id), count(len(st.Transitions), "beat", "beats")),
					foldInfo{state: state}))
			}
			for k, tr := range st.Transitions {
				add(b.fstmt(KindConvStep, about, fmt.Sprintf("%s.transitions[%d]", spath, k), b.convStepWords(id, state, tr),
					foldInfo{state: state, text: b.convStepShort(id, tr)}))
			}
		}
		for _, state := range sortedKeys(in.Outcomes) {
			for k, c := range in.Outcomes[state] {
				if c.Recognize == nil {
					continue
				}
				add(b.outcomeStatement(id, state, *c.Recognize, fmt.Sprintf("%s.outcomes.%s[%d].recognize", base, state, k)))
			}
		}
		for _, site := range b.lineSites() {
			for j, t := range site.option.Requires {
				ti, ok := t.(encounter.TriggerInteraction)
				if !ok || string(ti.Interaction) != id {
					continue
				}
				add(b.fstmt(KindConvAsked, about, b.doc.item(site.path+".requires", j), fmt.Sprintf(
					"The line %s said to %s is offered only while %s is in %s.",
					quote(site.option.ID), b.creature(site.member), quote(id), joinOr(quoteAll(ti.In))),
					foldInfo{text: quote(site.option.ID), state: joinOr(quoteAll(ti.In)), who: site.member},
					Connection{Kind: "creature", ID: site.member, Role: "carries"}))
			}
		}
		groups = append(groups, g)
	}
	return groups
}

// convStepWords says one transition of a conversation and which lines send
// its beat.
func (b *builder) convStepWords(id, state string, tr machine.Transition) string {
	s := fmt.Sprintf("In the state %s, the beat %s", quote(state), quote(string(tr.On)))
	if tr.To != "" {
		s += " moves the conversation to " + quote(tr.To)
	} else {
		s += " moves nothing"
	}
	if tr.Reason != "" {
		s += fmt.Sprintf(" (%s)", said(tr.Reason))
	}
	var senders []string
	for _, site := range b.lineSites() {
		a := site.option.Advance
		if a == nil || string(a.Interaction) != id {
			continue
		}
		if a.Success == tr.On {
			senders = append(senders, fmt.Sprintf("winning %s said to %s", quote(site.option.ID), b.creature(site.member)))
		}
		if a.Failure == tr.On {
			senders = append(senders, fmt.Sprintf("missing %s said to %s", quote(site.option.ID), b.creature(site.member)))
		}
	}
	if len(senders) > 0 {
		s += ". The beat is sent by " + joinOr(senders)
	} else {
		s += ". No line sends this beat"
	}
	return s + "."
}

// outcomeStatement says what entering a conversation's state grants: who
// comes to hold the topic, about whom, and who else hears it.
func (b *builder) outcomeStatement(id, state string, r encounter.Recognition, path string) Statement {
	topic := quote(string(r.Topic))
	holder := "the creature the deciding line was said to"
	var holders []string
	for _, site := range b.lineSites() {
		if a := site.option.Advance; a != nil && string(a.Interaction) == id {
			if !contains(holders, site.member) {
				holders = append(holders, site.member)
			}
		}
	}
	conns := []Connection{{Kind: "topic", ID: string(r.Topic), Role: "grants"}}
	for _, h := range holders {
		conns = append(conns, Connection{Kind: "creature", ID: h, Role: "holds"})
	}
	s := fmt.Sprintf("When the conversation %s enters %s, %s (whichever of %s that was) comes to hold %s about %s: that creature recognises them as %s.",
		quote(id), quote(state), holder, b.creatures(holders), topic, subjectsWords(r.About, "the creature"), topic)
	s += " " + b.heardWords(r, id, nil, &conns)
	f := foldInfo{state: state, site: fmt.Sprintf("the conversation %s on entering %s", quote(id), quote(state))}
	f.text = fmt.Sprintf("On entering %s: the creature the deciding line was said to comes to hold %s about %s",
		quote(state), topic, strings.ReplaceAll(shortSubjects(r.About), "{self}", "it")) + shortHeard(r, id) + "."
	return b.fstmt(KindOutcome, Subject{Kind: "conversation", ID: id}, path, s, f, conns...)
}

// contains reports whether ids holds id.
func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// convStepShort is a transition in the folded form, with the lines that send
// its beat grouped by verdict and line: "`admitted` moves it to `admitted`
// (“let through”); sent by a win of `troupe` (gate-1, gate-2, captain)".
func (b *builder) convStepShort(id string, tr machine.Transition) string {
	s := quote(string(tr.On)) + " moves it to " + quote(tr.To)
	if tr.To == "" {
		s = quote(string(tr.On)) + " moves nothing"
	}
	if tr.Reason != "" {
		s += fmt.Sprintf(" (%s)", said(tr.Reason))
	}
	var order []string
	who := map[string][]string{}
	for _, site := range b.lineSites() {
		a := site.option.Advance
		if a == nil || string(a.Interaction) != id {
			continue
		}
		for _, v := range []struct {
			beat machine.Beat
			word string
		}{{a.Success, map[bool]string{true: "a win of ", false: "saying "}[site.option.Checked()]}, {a.Failure, "a miss of "}} {
			if v.beat != tr.On {
				continue
			}
			key := v.word + quote(site.option.ID)
			if _, ok := who[key]; !ok {
				order = append(order, key)
			}
			who[key] = append(who[key], site.member)
		}
	}
	if len(order) == 0 {
		return s + "; no line sends it"
	}
	var parts []string
	for _, k := range order {
		parts = append(parts, k+" ("+strings.Join(who[k], ", ")+")")
	}
	return s + "; sent by " + joinOr(parts)
}
