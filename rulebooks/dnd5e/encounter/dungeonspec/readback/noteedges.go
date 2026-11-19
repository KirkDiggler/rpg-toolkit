// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// The connections that trace one note (rpg-project#1001): who takes it and
// how, whom it is about, what gave it, and whose note a read asks of.
const (
	// ConnWho is a connection to a who-word ([encounter.WhoWords]): role
	// "about" names whom a note is about (given or asked), role "takes" a
	// taker word with its way in is. Its at is the topic it belongs to.
	ConnWho = "who"
	// RoleNotes is a creature that comes to note a topic: is is the way
	// it comes by the note (the engine's word), at the topic.
	RoleNotes = "notes"
	// RoleWhose is the creature whose note a line's or a post's read asks
	// of; at is the topic.
	RoleWhose = "whose"
)

// noteTaker is one taker word of a give and the creatures it can resolve
// to.
type noteTaker struct {
	word      string
	creatures []string
}

// noteGive is one topic given: whom it is about, who takes it, and what
// gave it.
type noteGive struct {
	topic  string
	about  string
	takers []noteTaker
	cause  Connection
}

// way is how a taker word comes by a note, from the who-words list.
func way(word string) string {
	w, _ := encounter.WhoWordOf(word)
	return w.Way
}

// noteGiveEdges is every connection one topic given carries: the
// `{topic, grants}` edge the hubs group by; whom it is about; each taker
// word with its way; each creature that can come to note it, with its way;
// and what gave it. THE ONE PLACE a give's note edges are made.
func noteGiveEdges(g noteGive) []Connection {
	out := []Connection{
		{Kind: "topic", ID: g.topic, Role: "grants"},
		{Kind: ConnWho, ID: g.about, Role: "about", At: g.topic},
	}
	for _, t := range g.takers {
		out = append(out, Connection{Kind: ConnWho, ID: t.word, Role: "takes", Is: way(t.word), At: g.topic})
	}
	for _, t := range g.takers {
		for _, c := range t.creatures {
			out = append(out, Connection{Kind: "creature", ID: c, Role: RoleNotes, Is: way(t.word), At: g.topic})
		}
	}
	if g.cause.Kind != "" {
		out = append(out, g.cause)
	}
	return out
}

// noteAskEdges is every connection one read of a topic carries: the topic
// edge (role as the reader has always read it, negated under a not), whom
// it asks about, and whose note it is, when a creature is named. THE ONE
// PLACE a read's note edges are made.
func noteAskEdges(topic, role string, negated bool, about, whose string) []Connection {
	out := []Connection{
		{Kind: "topic", ID: topic, Role: role, Not: negated},
		{Kind: ConnWho, ID: about, Role: "about", At: topic},
	}
	if whose != "" {
		out = append(out, Connection{Kind: "creature", ID: whose, Role: RoleWhose, At: topic})
	}
	return out
}
