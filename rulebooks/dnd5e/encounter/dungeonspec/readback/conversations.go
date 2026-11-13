// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
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
				"Its carriers, the creatures whose lines read or step it, are %s. Its recipe is %s.", quote(id), quote(in.Recipe.Initial), who,
			quote(in.Recipe.Ref.String())),
			foldInfo{text: fmt.Sprintf("one state for the whole run, shared by every speaker; starts in %s; recipe %s. "+
				"Carried by %s (the creatures whose lines read or step it).", quote(in.Recipe.Initial), quote(in.Recipe.Ref.String()), who)}, cc...))
		if in.Approach != nil {
			hail := fmt.Sprintf("When a character's own step ends within %d cells of a carrier who sees them and whom they see, "+
				"both up and out of a fight, and that carrier offers them a line that steps %s, the nearest such carrier "+
				"(then by id) hails them and hears the beat %s, unless %s is already addressing someone. "+
				"A line that only reads %s does not hail. It is opened within %d cells, and stands while the other conditions "+
				"hold and the character is within Speak reach of that carrier (never less than %d cells); it ends when any fails.",
				in.Approach.Within, quote(id), quote("approached"), quote(id), quote(id), in.Approach.Within, in.Approach.Within)
			add(b.fstmt(KindApproach, about, base+".approach", hail, foldInfo{text: hail}))
			hailers, short := b.hailersWords(id, &in)
			add(b.fstmt(KindHailers, about, base+".approach", hailers, foldInfo{text: short}, cc...))
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
					foldInfo{state: state, text: b.convStepShort(id, state, tr)}))
			}
		}
		for _, state := range sortedKeys(in.Outcomes) {
			for k, c := range in.Outcomes[state] {
				if c.Arrest != nil {
					add(b.arrestStatement(id, state, fmt.Sprintf("%s.outcomes.%s[%d].arrest", base, state, k)))
					continue
				}
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
		checked := false
		for _, site := range b.lineSites() {
			if conv, ok := optionInteraction(site.option); ok && conv == id && site.option.Checked() {
				checked = true
			}
		}
		rules := []string{
			fmt.Sprintf("while a checked line that reads or steps %s waits on its speaker's decision about a held benefit, "+
				"every line that reads or steps %s is pending for every speaker until that roll is settled or released.", quote(id), quote(id)),
			"if the speaker and the creature spoken to lose reach or sight of each other while that roll waits, the " +
				"speaker keeps the roll they made: it is settled as rolled, judged where the words were spoken.",
			fmt.Sprintf("a reply sent from a menu offered before %s last changed (a state change, or a roll beginning or "+
				"ending its wait on it) is refused with nothing spent; the speaker must be offered the menu again.", quote(id)),
		}
		if !checked {
			// No checked line touches it: nothing can wait on a roll.
			rules = rules[2:]
		}
		for _, rule := range rules {
			add(b.fstmt(KindEngineRule, about, EngineRule, "Engine rule, not authored: "+rule,
				foldInfo{text: "Engine rule, not authored: " + rule}))
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
		if !b.offeredIn(site.option, id)[state] {
			continue
		}
		if a.Success == tr.On {
			form := "winning %s said to %s"
			if !site.option.Checked() {
				form = "saying %s to %s"
			}
			senders = append(senders, fmt.Sprintf(form, quote(site.option.ID), b.creature(site.member)))
		}
		if a.Failure == tr.On {
			senders = append(senders, fmt.Sprintf("missing %s said to %s", quote(site.option.ID), b.creature(site.member)))
		}
	}
	if len(senders) > 0 {
		s += ". The beat is sent by " + joinOr(senders)
	} else {
		s += fmt.Sprintf(". No line offered in %s sends this beat", quote(state))
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
	f.text = strings.ReplaceAll(fmt.Sprintf("On entering %s: the creature the deciding line was said to comes to hold %s about %s",
		quote(state), topic, shortSubjects(r.About))+shortHeard(r, id)+".", "{self}", "that creature")
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
func (b *builder) convStepShort(id, state string, tr machine.Transition) string {
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
		if a == nil || string(a.Interaction) != id || !b.offeredIn(site.option, id)[state] {
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
		return s + "; no line offered here sends it"
	}
	var parts []string
	for _, k := range order {
		parts = append(parts, k+" ("+strings.Join(who[k], ", ")+")")
	}
	return s + "; sent by " + joinOr(parts)
}

// hailersWords says which carriers can hail in which state of a
// conversation: those offering, in that state, a line that steps it, with
// what else the line asks of the speaker.
func (b *builder) hailersWords(id string, in *encounter.InteractionInput) (string, string) {
	var parts []string
	var nobody []string
	for _, state := range statesInOrder(in.Recipe) {
		var order []string
		who := map[string][]string{}
		for _, site := range b.lineSites() {
			o := site.option
			if o.Advance == nil || string(o.Advance.Interaction) != id || !b.offeredIn(o, id)[state] {
				continue
			}
			desc := quote(o.ID)
			var also []string
			for _, t := range o.Requires {
				if inner, ok := t.(encounter.TriggerNot); ok {
					if _, conv := inner.Inner.(encounter.TriggerInteraction); conv {
						continue
					}
				}
				if _, conv := t.(encounter.TriggerInteraction); conv {
					continue
				}
				w, _ := b.triggerWords(t, lineContext{me: "the speaker", other: "the carrier"})
				also = append(also, w)
			}
			if len(also) > 0 {
				desc += ", if " + strings.Join(also, " and ")
			}
			name := b.creature(site.member)
			if _, ok := who[name]; !ok {
				order = append(order, name)
			}
			who[name] = append(who[name], desc)
		}
		if len(order) == 0 {
			nobody = append(nobody, quote(state))
			continue
		}
		// Carriers with the same lines are named together.
		var groups []string
		seen := map[string]bool{}
		for _, n := range order {
			key := strings.Join(who[n], "; ")
			if seen[key] {
				continue
			}
			seen[key] = true
			var names []string
			for _, m := range order {
				if strings.Join(who[m], "; ") == key {
					names = append(names, m)
				}
			}
			groups = append(groups, fmt.Sprintf("%s (by %s)", joinAnd(names), strings.Join(who[n], "; by ")))
		}
		parts = append(parts, fmt.Sprintf("in %s, %s", quote(state), strings.Join(groups, ", and ")))
	}
	if len(nobody) > 0 {
		parts = append(parts, fmt.Sprintf("in %s, nobody", joinAnd(nobody)))
	}
	s := fmt.Sprintf("Who can hail for %s, by its state: %s.", quote(id), strings.Join(parts, "; "))
	return s, s
}

// arrestStatement says what an arrest decides on entering a conversation's
// state (rpg-project#925): who is taken and why, who becomes wanted, what
// the guards come to hold, where the arrested are to be held — and that,
// for now, nobody is moved.
func (b *builder) arrestStatement(id, state, path string) Statement {
	spec := b.spec.Interactions[id].Outcomes[state]
	var a *dungeonspec.ArrestSpec
	for _, c := range spec {
		if c.Arrest != nil {
			a = c.Arrest
		}
	}
	var carriers []string
	for _, site := range b.lineSites() {
		if which, ok := optionInteraction(site.option); ok && which == id && !contains(carriers, site.member) {
			carriers = append(carriers, site.member)
		}
	}
	var area []string
	for _, r := range a.Area.Regions {
		area = append(area, b.regionWords(r))
	}
	if n := len(dungeonspec.ExpandRows(a.Area.Cells, path+".area.cells", func(string, string) {})); n > 0 {
		area = append(area, count(n, "cell", "cells")+" besides")
	}
	conns := []Connection{
		{Kind: "topic", ID: string(encounter.TopicArrested), Role: "grants"},
		{Kind: "topic", ID: string(encounter.TopicWanted), Role: "grants"},
	}
	for _, c := range carriers {
		conns = append(conns, Connection{Kind: "creature", ID: c, Role: "holds"})
	}
	s := fmt.Sprintf("When the conversation %s enters %s, it arrests the speaker's party, to be held in %s. "+
		"Taken: every player of the speaker's party standing in the arrest area (%s), or seen by any of its carriers "+
		"who is up (%s), as the deciding line is said; a roll kept and settled after its speaker walked away is "+
		"decided where everyone stood and whom the carriers saw when its words were said. Wanted: a player of the "+
		"party who is not taken and whom one of those carriers has seen before. A creature of the party's faction is "+
		"neither. The creature the deciding line was said to comes to hold %s about the taken and %s about the "+
		"wanted, and the conversation's other carriers who catch the deciding line hold the same, heard. Who is wanted "+
		"is told to those carriers alone, never to a player. The conversation's address ends. Who is taken and who is "+
		"wanted is decided once and kept. The taken are held from that moment: they cannot walk, dash, sneak or cast "+
		"themselves away, and nothing moves them but the march (a shove or a flee on them is refused), while "+
		"speaking, looking, looting and fighting stay open. A fight with one of those carriers in it ends the hold for "+
		"all of them, and the record stays; a fight without them frees nobody, and the held stay held inside it. A "+
		"taken player who leaves the run stays taken. Nobody is moved yet: the taken are not walked to %s.",
		quote(id), quote(state), b.regionWords(a.Cell), joinAnd(area), b.creatures(carriers),
		quote(string(encounter.TopicArrested)), quote(string(encounter.TopicWanted)), b.regionName(a.Cell))
	f := foldInfo{state: state, site: fmt.Sprintf("the conversation %s on entering %s", quote(id), quote(state))}
	f.text = fmt.Sprintf("On entering %s: arrests the players in %s or seen by its carriers, to %s; the rest they have seen become wanted, known to the carriers alone. The taken are held until a fight with the carriers in it. Nobody is moved yet.",
		quote(state), joinAnd(area), b.regionName(a.Cell))
	return b.fstmt(KindOutcome, Subject{Kind: "conversation", ID: id}, path, s, f, conns...)
}
