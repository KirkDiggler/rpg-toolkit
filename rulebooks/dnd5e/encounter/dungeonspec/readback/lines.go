// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// lineTarget is who or what a set of lines is said to.
type lineTarget struct {
	name  string  // "Guard (gate-1)", "the prop Altar (altar)"
	short string  // the folded form's name: "the guard"
	about Subject // the statement's subject
	id    string  // the member id, when a creature
	prop  bool
}

// lineStatements says every line of one target, each facet its own
// statement at its own path: the line, its check, each requirement, its
// advance, each answer row, each grant. One more facet of a line (its
// loudness, when that lands) is one more statement here.
func (b *builder) lineStatements(opts []encounter.Option, base string, t lineTarget) []Statement {
	var out []Statement
	for i, o := range opts {
		p := fmt.Sprintf("%s[%d]", base, i)
		line := fmt.Sprintf("the line %s (%s)", said(o.Say), quote(o.ID))
		saidTo := Connection{Kind: t.about.Kind, ID: t.about.ID, Role: "said-to"}
		heading := fmt.Sprintf("%s (%s)", said(o.Say), quote(o.ID))
		head := fmt.Sprintf("The speaker can say %s to %s. %s %s", line, t.name, loudnessSentence(o.Loudness), perceivedSentence)
		short := "said to {self}; " + foldedLoudness(o.Loudness)
		if o.Checked() {
			head += " It is a checked line: it costs an action, one round of the world out of a fight."
			short += "checked, an action (a round of the world out of a fight)"
		} else {
			head += " It is a plain line: it rolls nothing, cannot be missed, and costs nothing on either clock."
			short += "plain: rolls nothing, cannot be missed, free"
		}
		out = append(out, b.fstmt(KindLine, t.about, p, head, foldInfo{text: short, line: heading}, saidTo))
		if o.Checked() {
			out = append(out, b.fstmt(KindLineCheck, t.about, p+".check", fmt.Sprintf(
				"To win %s, the speaker rolls %s; beating the DC wins the line and missing it misses.",
				line, checkWords(o.Check)), foldInfo{text: "rolls " + strings.ReplaceAll(checkWords(o.Check), " against ", " vs "), line: heading}))
		}
		ctx := lineContext{me: "the speaker", other: t.name}
		fctx := lineContext{me: "the speaker", other: "{self}"}
		for j, req := range o.Requires {
			w, conns := b.triggerWords(req, ctx)
			fw, _ := b.triggerWords(req, fctx)
			fw = shortClause(fw)
			out = append(out, b.fstmt(KindLineRequire, t.about, b.doc.item(p+".requires", j), fmt.Sprintf(
				"%s is offered to a speaker only while %s.", capitalize(line), w),
				foldInfo{text: fw, line: heading, site: quote(o.ID), who: t.about.ID}, conns...))
		}
		if o.Advance != nil {
			out = append(out, b.advanceStatement(o, line, heading, p, t))
		}
		out = append(out, b.rowStatements(o, line, heading, p, t)...)
	}
	return out
}

// advanceStatement says which beat a line's verdict sends its conversation,
// and where that beat moves it from each state.
func (b *builder) advanceStatement(o encounter.Option, line, heading, p string, t lineTarget) Statement {
	conv := string(o.Advance.Interaction)
	var parts []string
	f := foldInfo{line: heading}
	if o.Advance.Success != "" {
		parts = append(parts, fmt.Sprintf("winning it steps the conversation %s with the beat %s, %s",
			quote(conv), quote(string(o.Advance.Success)), b.moveWords(conv, o.Advance.Success, b.offeredIn(o, conv))))
		f.text = fmt.Sprintf("steps %s with %s: %s", quote(conv), quote(string(o.Advance.Success)), b.foldMoveWords(conv, o.Advance.Success, b.offeredIn(o, conv)))
	}
	if o.Advance.Failure != "" {
		parts = append(parts, fmt.Sprintf("missing it steps the conversation %s with the beat %s, %s",
			quote(conv), quote(string(o.Advance.Failure)), b.moveWords(conv, o.Advance.Failure, b.offeredIn(o, conv))))
		f.miss = fmt.Sprintf("steps %s with %s: %s", quote(conv), quote(string(o.Advance.Failure)), b.foldMoveWords(conv, o.Advance.Failure, b.offeredIn(o, conv)))
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("it names the conversation %s and steps it with no beat", quote(conv)))
	}
	return b.fstmt(KindLineAdvance, t.about, p+".advance",
		fmt.Sprintf("Said to %s, %s: %s%s.", t.name, line, strings.Join(parts, "; "), b.offeredWords(o, conv)), f,
		Connection{Kind: "conversation", ID: conv, Role: "advances"})
}

// interaction is a compiled conversation by id.
func (b *builder) interaction(id string) *encounter.InteractionInput {
	for i := range b.c.Field.Interactions {
		if string(b.c.Field.Interactions[i].ID) == id {
			return &b.c.Field.Interactions[i]
		}
	}
	return nil
}

// moveWords says where a beat moves a conversation, from each state that
// listens for it.
func (b *builder) moveWords(conv string, beat machine.Beat, offered map[string]bool) string {
	in := b.interaction(conv)
	if in == nil {
		return "a conversation this file does not compile"
	}
	var moves []string
	for _, s := range statesInOrder(in.Recipe) {
		for _, tr := range in.Recipe.States[s].Transitions {
			if tr.On == beat && tr.To != "" && offered[s] {
				moves = append(moves, fmt.Sprintf("from %s to %s", quote(s), quote(tr.To)))
			}
		}
	}
	if len(moves) == 0 {
		return "which no state the line is offered in listens for, so it moves nothing"
	}
	return "which moves it " + joinOr(moves)
}

// rowStatements says a line's answer rows and the grant each carries.
func (b *builder) rowStatements(o encounter.Option, line, heading, p string, t lineTarget) []Statement {
	var out []Statement
	for _, verdict := range []struct {
		failure bool
		rows    []encounter.Answer
		key     string
	}{{false, o.Success, "success"}, {true, o.Failure, "failure"}} {
		lead, verdictWord, site := "On a win of "+line, "win", "a win of "+quote(o.ID)
		switch {
		case verdict.failure:
			lead, verdictWord, site = "On a miss of "+line, "miss", "a miss of "+quote(o.ID)
		case !o.Checked():
			lead, verdictWord, site = "When "+line+" is said", "said", "saying "+quote(o.ID)
		}
		total := 0
		for _, r := range verdict.rows {
			total += r.Weight
		}
		for k, row := range verdict.rows {
			path := fmt.Sprintf("%s.%s[%d]", p, verdict.key, k)
			does, conns := rowDoes(row, t, t.name)
			fdoes, _ := rowDoes(row, t, "{self}")
			// A ROW THAT ONLY GRANTS (rpg-project#958): no words and no fact,
			// its whole effect is the grant said next — "nothing is said"
			// would read as a row that does nothing.
			if row.Say == "" && row.Activity == nil && row.Fact == "" && grantsOnRow(o.Grants, verdict.failure, k) {
				does = []string{"nothing is answered in words, and the row does what the next sentence says"}
				fdoes = []string{"no words"}
			}
			// THE PICK IS TOLD BY HEARING (rpg-project#948): a creature's
			// answer — its die and the rows it chose among — goes to whoever
			// caught the line; one who only sees it answer is told that and
			// nothing more. Said in full; the folded form leaves it to the
			// line's own perception sentence.
			if !t.prop {
				does = append(does, fmt.Sprintf("whoever sees %s answer and caught none of the line is told only that %s answered "+
					"and what %s is seen doing, and, if they see the speaker, sees the line said without its words", t.name, t.name, t.name))
			}
			s := lead + ", " + joinAnd(does)
			short := joinAnd(fdoes)
			if len(verdict.rows) > 1 {
				s += fmt.Sprintf(" (weight %d of %d: one row is picked by weight)", row.Weight, total)
				short += fmt.Sprintf(" (weight %d of %d)", row.Weight, total)
			}
			out = append(out, b.fstmt(KindLineRow, t.about, path, period(s),
				foldInfo{text: short, line: heading, verdict: verdictWord, site: site, who: t.about.ID}, conns...))
			for _, g := range o.Grants {
				if g.Failure == verdict.failure && g.Row == k && g.Consequence.Recognize != nil {
					out = append(out, b.grantStatement(o, *g.Consequence.Recognize, lead, path+".recognize", t,
						foldInfo{line: heading, verdict: verdictWord, site: site, who: t.about.ID}))
				}
				if g.Failure == verdict.failure && g.Row == k && (g.Consequence.Tells != "" || g.Consequence.Relocate != nil) {
					out = append(out, b.passageStatement(g.Consequence, row, lead, path, t,
						foldInfo{line: heading, verdict: verdictWord, site: site, who: t.about.ID}))
				}
			}
		}
	}
	return out
}

// rowDoes is what a line's answer row does, said about name.
func rowDoes(row encounter.Answer, t lineTarget, name string) ([]string, []Connection) {
	var does []string
	var conns []Connection
	if row.Say != "" {
		if t.prop {
			does = append(does, "the speaker is told "+said(row.Say))
		} else {
			does = append(does, fmt.Sprintf("%s answers %s, at %s: its words reach %s", name, said(row.Say),
				quote(loudnessOf(row.Loudness)), reachWords(row.Loudness, name)))
		}
	}
	if row.Activity != nil {
		does = append(does, fmt.Sprintf("%s plays the activity %s %d times", name, quote(row.Activity.Ref), row.Activity.Cycles))
	}
	if row.Fact != "" {
		if t.prop {
			// A PROP'S ANSWER IS NARRATION, not a voice (rpg-project#933): its
			// fact goes to those who saw the line said.
			does = append(does, fmt.Sprintf("everyone who can see the speaker, the speaker included, learns the fact %s", quote(row.Fact)))
		} else {
			does = append(does, fmt.Sprintf("%s learns the fact %s", learnersWords(row.Loudness, name), quote(row.Fact)))
		}
		conns = append(conns, Connection{Kind: "fact", ID: row.Fact, Role: "teaches"})
	}
	if len(does) == 0 {
		does = append(does, "nothing is said")
	}
	return does, conns
}

// passageStatement says a row's told record or relocate (rpg-project#958):
// who comes to hold the record and what it reveals, or where the speaker is
// set down and what the others are told of it.
func (b *builder) passageStatement(c encounter.Consequence, row encounter.Answer, lead, path string, t lineTarget, f foldInfo) Statement {
	if c.Tells != "" {
		id := b.unqualify(string(c.Tells))
		who, short := "everyone who can see the speaker, the speaker included,", "everyone who sees the speaker"
		if !t.prop {
			who, short = learnersWords(row.Loudness, t.name), "every member {self}'s words reach"
		}
		s := fmt.Sprintf("%s, %s comes to hold the record %s, as one who carried it would, and learns what it reveals (%s). "+
			"One who holds it already is told nothing again.", lead, who, quote(id), b.recordReveals(c.Tells))
		f.text = short + " comes to hold the record " + quote(id)
		return b.fstmt(KindLineGrant, t.about, path+".tells", s, f, Connection{Kind: "record", ID: id, Role: "tells"})
	}
	r := c.Relocate
	rounds := "No world time passes with it."
	switch {
	case r.Rounds == 1:
		rounds = "One round passes on the world clock with it, and creatures due in it act."
	case r.Rounds > 1:
		rounds = fmt.Sprintf("%d rounds pass on the world clock with it, and creatures due in them act.", r.Rounds)
	}
	s := fmt.Sprintf("%s, the speaker is set down on the first free cell of %s, without walking the way between, and is told %s. %s "+
		"Whoever saw the speaker leave is told they went, never where; whoever sees them arrive is told they arrived, never from "+
		"where. A speaker in a fight, held, or with no free cell to land on is refused the line whole; nobody else goes with them.",
		lead, b.regionWords(string(r.To)), said(r.Words), rounds)
	f.text = fmt.Sprintf("the speaker is set down in %s, told %s", b.regionName(string(r.To)), said(r.Words))
	return b.fstmt(KindLineGrant, t.about, path+".relocate", s, f, Connection{Kind: "region", ID: string(r.To), Role: "sends-to"})
}

// grantStatement says one recognition a line's row grants: who comes to hold
// the topic, about whom, and who else hears it — in that order.
func (b *builder) grantStatement(o encounter.Option, r encounter.Recognition, lead, path string, t lineTarget, f foldInfo) Statement {
	topic := string(r.Topic)
	holder := t.name
	s := fmt.Sprintf("%s, %s — the creature the line is said to — comes to hold %s about %s: %s recognises them as %s.",
		lead, holder, quote(topic), subjectsWords(r.About, holder), holder, quote(topic))
	conns := []Connection{{Kind: "topic", ID: topic, Role: "grants"}}
	if t.id != "" {
		conns = append(conns, Connection{Kind: "creature", ID: t.id, Role: "holds"})
	}
	conv, _ := optionInteraction(o)
	s += " " + b.heardWords(r, conv, []string{t.id}, &conns)
	f.text = fmt.Sprintf("{self} comes to hold %s about %s", quote(topic), shortSubjects(r.About)) + shortHeard(r, conv)
	return b.fstmt(KindLineGrant, t.about, path, s, f, conns...)
}

// heardWords says who else comes to hold a recognition, having heard it.
func (b *builder) heardWords(r encounter.Recognition, conv string, except []string, conns *[]Connection) string {
	topic := quote(string(r.Topic))
	if r.HeardBy != encounter.RecognitionCarriersWhoWitnessed {
		return "Nobody else comes to hold it."
	}
	var others []string
	for _, id := range b.carriers[conv] {
		skip := false
		for _, e := range except {
			skip = skip || e == id
		}
		if !skip {
			others = append(others, id)
			*conns = append(*conns, Connection{Kind: "creature", ID: id, Role: "holds"})
		}
	}
	who := "nobody: no other creature carries it"
	switch {
	case except == nil && len(others) > 0:
		who = "of " + b.creatures(others) + ", each but the one the line was said to"
	case len(others) > 0:
		who = b.creatures(others)
	}
	return fmt.Sprintf("Every other creature that carries the conversation %s (%s), was among those the line's words reached "+
		"when it was said, and is up, comes to hold %s about the same members too, having heard it, whether or not it sees "+
		"them itself; one the words did not reach holds nothing, even if it saw the line said. When the holder recognises "+
		"nobody, nobody else comes to hold anything either.",
		quote(conv), who, topic)
}

// shortSubjects is whom a recognition is about, in the folded form.
func shortSubjects(about encounter.RecognitionSubjects) string {
	switch about {
	case encounter.RecognitionPartyInSight:
		return "the speaker and every member of the speaker's party {self} sees then"
	case encounter.RecognitionSpeaker:
		return "the speaker alone, if {self} sees the speaker"
	}
	return fmt.Sprintf("the subjects %q", about)
}

// shortHeard is who else holds a recognition, in the folded form; the
// carriers themselves are named once, on the conversation.
func shortHeard(r encounter.Recognition, conv string) string {
	if r.HeardBy != encounter.RecognitionCarriersWhoWitnessed {
		return "; nobody else holds it"
	}
	return fmt.Sprintf("; so does each other carrier of %s the line's words reached and who is up, about the same members, "+
		"whether or not that carrier sees them (heard); if {self} recognises nobody, nobody else holds anything", quote(conv))
}

// shortClause drops a requirement's restatement for the folded form: the
// conversation block already says its state is the run's, and "recognises X
// as T" already says who holds what about whom.
func shortClause(w string) string {
	w = strings.Replace(w, " (one state for the whole run, not per speaker)", "", 1)
	w = strings.Replace(w, "the conversation ", "", 1)
	w = strings.Replace(w, " is in the state ", " is in ", 1)
	w = strings.Replace(w, " is in one of the states ", " is in ", 1)
	if i := strings.Index(w, " ({self} "); i > 0 && strings.HasSuffix(w, ")") {
		w = w[:i]
	}
	return w
}

// foldMoveWords is moveWords grouped by where the beat leads: "moving it to
// `admitted` from `fresh`, `warned-guest` or `warned-troupe`".
func (b *builder) foldMoveWords(conv string, beat machine.Beat, offered map[string]bool) string {
	in := b.interaction(conv)
	if in == nil {
		return "a conversation this file does not compile"
	}
	var order []string
	from := map[string][]string{}
	for _, s := range statesInOrder(in.Recipe) {
		for _, tr := range in.Recipe.States[s].Transitions {
			if tr.On == beat && tr.To != "" && offered[s] {
				if _, ok := from[tr.To]; !ok {
					order = append(order, tr.To)
				}
				from[tr.To] = append(from[tr.To], quote(s))
			}
		}
	}
	if len(order) == 0 {
		return "which moves nothing"
	}
	var parts []string
	for _, to := range order {
		parts = append(parts, fmt.Sprintf("to %s from %s", quote(to), joinOr(from[to])))
	}
	return strings.Join(parts, ", ")
}

// offeredIn is the states of a conversation a line is offered in: every
// state its requirements on that conversation admit (a `not` admits the
// rest), in the recipe's order. A line with no requirement on it is offered
// in every state.
func (b *builder) offeredIn(o encounter.Option, conv string) map[string]bool {
	in := b.interaction(conv)
	out := map[string]bool{}
	if in == nil {
		return out
	}
	for s := range in.Recipe.States {
		out[s] = true
	}
	for _, t := range o.Requires {
		negated := false
		if n, ok := t.(encounter.TriggerNot); ok {
			t, negated = n.Inner, true
		}
		ti, ok := t.(encounter.TriggerInteraction)
		if !ok || string(ti.Interaction) != conv {
			continue
		}
		named := map[string]bool{}
		for _, s := range ti.In {
			named[s] = true
		}
		for s := range out {
			if named[s] == negated {
				delete(out, s)
			}
		}
	}
	return out
}

// offeredWords says the states a line is offered in, when not all of them.
func (b *builder) offeredWords(o encounter.Option, conv string) string {
	in := b.interaction(conv)
	if in == nil {
		return ""
	}
	offered := b.offeredIn(o, conv)
	if len(offered) == len(in.Recipe.States) {
		return ""
	}
	var states []string
	for _, s := range statesInOrder(in.Recipe) {
		if offered[s] {
			states = append(states, quote(s))
		}
	}
	if len(states) == 0 {
		return " (the line is offered in no state of it)"
	}
	return " (the line is offered only in " + joinOr(states) + ")"
}

// recordReveals is what an intel record reveals, in words.
func (b *builder) recordReveals(id encounter.IntelID) string {
	for _, in := range b.c.Intel {
		if in.ID != id {
			continue
		}
		switch {
		case in.Reveals.Concealment != "":
			return "the secret " + quote(b.unqualify(string(in.Reveals.Concealment)))
		case in.Reveals.Fact != "":
			return "the fact " + quote(string(in.Reveals.Fact))
		}
	}
	return "nothing this build can say"
}

// grantsOnRow reports whether any grant hangs on a verdict's row k.
func grantsOnRow(grants []encounter.RowGrant, failure bool, k int) bool {
	for _, g := range grants {
		if g.Failure == failure && g.Row == k {
			return true
		}
	}
	return false
}
