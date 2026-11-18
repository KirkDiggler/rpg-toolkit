// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// hubGroups says each topic (or fact) as a hub: what it means, if the file
// says, then every place that grants or teaches it and every place that
// asks for it or wakes on it — each the same sentence, at the same path,
// as where it was first said.
func (b *builder) hubGroups(kind string) []Group {
	type site struct {
		role string
		s    Statement
	}
	byID := map[string][]site{}
	for _, s := range b.hubs {
		for _, c := range s.Connects {
			if c.Kind == kind {
				byID[c.ID] = append(byID[c.ID], site{role: c.Role, s: s})
			}
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	giveKind, askKind, hubKind := KindTopicGrant, KindTopicAsk, KindTopic
	giveWord, word := "granted", "topic"
	if kind == "fact" {
		giveKind, askKind, hubKind = KindFactTeach, KindFactAsk, KindFact
		giveWord, word = "taught", "fact"
	}
	var groups []Group
	for _, id := range ids {
		about := Subject{Kind: kind, ID: id}
		g := Group{Title: capitalize(word) + " " + quote(id), About: about}
		var gives, asks []Statement
		for _, st := range byID[id] {
			switch st.role {
			case "grants", "teaches":
				gives = append(gives, b.hubStatement(giveKind, about, st.s))
			default:
				asks = append(asks, b.hubStatement(askKind, about, st.s))
			}
		}
		path := "(declared nowhere)"
		switch {
		case len(gives) > 0:
			path = gives[0].Path
		case len(asks) > 0:
			path = asks[0].Path
		}
		head := fmt.Sprintf("The file does not say what the %s %s means: %ss carry no declared meaning yet. "+
			"It is %s in %s and asked for or woken on in %s.", word, quote(id), word, giveWord, places(len(gives)), places(len(asks)))
		if len(gives) == 0 {
			verb := "grants"
			if kind == "fact" {
				verb = "teaches"
			}
			head += fmt.Sprintf(" Nothing in the file %s it.", verb)
		}
		g.Statements = append(g.Statements, Statement{Kind: hubKind, About: about, Sentence: head, Path: path})
		g.Statements = append(g.Statements, gives...)
		g.Statements = append(g.Statements, asks...)
		groups = append(groups, g)
	}
	return groups
}

// hubStatement repeats a statement under its hub, keeping its sentence,
// path and connections.
func (b *builder) hubStatement(kind Kind, about Subject, s Statement) Statement {
	f := s.fold
	f.from, f.fromKind = s.About, s.Kind
	return Statement{Kind: kind, About: about, Sentence: s.Sentence, Path: s.Path, Connects: s.Connects, fold: f}
}

// places counts places in words: "1 place", "3 places".
func places(n int) string {
	if n == 1 {
		return "1 place"
	}
	return fmt.Sprintf("%d places", n)
}

// threadKinds is, for a record or a secret hub, the order its statements are
// said in: its own statement first, then each kind of place that reaches it.
var threadKinds = map[string][]Kind{
	"record":      {KindRecordReveal, KindRecordHold, KindRecordTell, KindRecordLift},
	"concealment": {KindSecretHide, KindSecretReveal, KindSecretFind, KindSecretBar, KindSecretNeed},
}

// threadKind is the kind a statement takes under the record or secret hub
// {kind, id}, or "" when it does not reach the hub. The hub's own statement
// is what it reveals (a record) or hides (a secret).
func threadKind(st Statement, kind, id string) Kind {
	if kind == "record" {
		if st.Kind == KindIntel && st.About.ID == id {
			return KindRecordReveal
		}
		for _, c := range st.Connects {
			if c.Kind != "record" || c.ID != id {
				continue
			}
			switch {
			case c.Role == "holds":
				return KindRecordHold
			case c.Role == "tells":
				return KindRecordTell
			case c.Role == "by" && st.Kind == KindWayTold:
				return KindRecordLift
			}
		}
		return ""
	}
	if st.Kind == KindConcealment && st.About.ID == id {
		return KindSecretHide
	}
	for _, c := range st.Connects {
		if c.Kind != "concealment" || c.ID != id {
			continue
		}
		switch c.Role {
		case "reveals":
			return KindSecretReveal
		case "hidden-by":
			return KindSecretBar
		case "needs":
			return KindSecretNeed
		}
	}
	return ""
}

// threadGroups says each record (or secret) as a hub (rpg-project#973):
// its own statement, then every statement that reaches it — who holds it,
// who tells it, the ways it lifts; what reveals it, what finds it, the joins
// it bars, the ways that need it — each repeated with its path, in reading
// order within its kind; how it is found is said once, from its own checks. Every record and every secret has a hub, reached
// by nothing or not. sts is every statement said before the hubs.
func (b *builder) threadGroups(kind string, sts []Statement) []Group {
	var ids []string
	if kind == "record" {
		for _, in := range b.c.Intel {
			ids = append(ids, b.unqualify(string(in.ID)))
		}
	} else {
		for _, c := range b.c.Concealments {
			ids = append(ids, b.unqualify(string(c.ID)))
		}
	}
	sort.Strings(ids)
	title := "Record "
	if kind == "concealment" {
		title = "Secret "
	}
	var groups []Group
	for _, id := range ids {
		about := Subject{Kind: kind, ID: id}
		g := Group{Title: title + quote(id), About: about}
		for _, k := range threadKinds[kind] {
			if k == KindSecretFind {
				g.Statements = append(g.Statements, b.secretFind(id))
				continue
			}
			for _, st := range sts {
				if threadKind(st, kind, id) == k {
					g.Statements = append(g.Statements, b.threadStatement(k, about, st))
				}
			}
		}
		groups = append(groups, g)
	}
	return groups
}

// threadStatement repeats a statement under a record or secret hub. A WAY
// repeated there is a pointer, never a way (rpg-project#973): it keeps its
// sentence and path and carries the one connection {way, <id>, is}, so a
// consumer that finds ways by their `through` connection or totals their
// methods meets each way once, at its join.
func (b *builder) threadStatement(kind Kind, about Subject, s Statement) Statement {
	st := b.hubStatement(kind, about, s)
	if wayKind(s.Kind) {
		st.Connects = []Connection{{Kind: "way", ID: s.About.ID, Role: "is"}}
		for _, c := range s.Connects {
			if c.Kind == "join" && c.Role == "through" {
				st.fold.site = fmt.Sprintf("the %s way through %s", strings.TrimPrefix(string(s.Kind), "place.way."), quote(c.ID))
			}
		}
	}
	return st
}

// secretFind says once how a secret is found: its own search checks, the
// same however many joins it bars. It connects to the secret alone; the
// search's methods are on each search way, at its join.
func (b *builder) secretFind(id string) Statement {
	var checks []encounter.CheckApproach
	for _, c := range b.c.Concealments {
		if b.unqualify(string(c.ID)) == id {
			checks = c.Checks
		}
	}
	return Statement{Kind: KindSecretFind, About: Subject{Kind: "concealment", ID: id},
		Sentence: fmt.Sprintf("A search with %s finds the secret %s.", checkWords(checks), quote(id)),
		Path:     "concealments." + id + ".checks",
		Connects: []Connection{{Kind: "concealment", ID: id, Role: "finds"}}}
}
