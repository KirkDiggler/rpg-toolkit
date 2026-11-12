// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"
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
	return Statement{Kind: kind, About: about, Sentence: s.Sentence, Path: s.Path, Connects: s.Connects, fold: s.fold}
}

// places counts places in words: "1 place", "3 places".
func places(n int) string {
	if n == 1 {
		return "1 place"
	}
	return fmt.Sprintf("%d places", n)
}
