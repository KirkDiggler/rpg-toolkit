// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"math"
)

// template_lets.go says what a stat block's creature can use
// (rpg-project#1131, slice 2c): one statement per stat block that writes
// `lets`, with a connection to each ability it names.
//
// IT SAYS THE WORD AND THE COUNT, NEVER THE MEANING. What `second wind` is and
// how `uses` is counted are the rules' to judge, so the sentence names each
// entry's `use` and, when the entry writes a whole number at `uses.max`, that
// count.

// Kinds, connections and roles of the stat-block `lets` statement.
const (
	// KindTemplateLets is what a stat block's creature can use, about
	// {template, <id>}, at `templates.<id>.lets`.
	KindTemplateLets Kind = "place.template.lets"
	// ConnAbility is the kind of a connection to a named ability: {ability, <the use word>}.
	ConnAbility = "ability"
	// RoleLets is the role of a stat block's connection to an ability it lets
	// its creature use.
	RoleLets = "lets"
	// RoleActivates is the role of a table row's connection to the ability it
	// has its creature use: the same word a stat block's `lets` writes, so the
	// two connect.
	RoleActivates = "activates"
)

// templateLetsStatement says the abilities a stat block's creature can use,
// or reports false when the block writes no `lets`.
func (b *builder) templateLetsStatement(id string, t templateLetsSource) (Statement, bool) {
	var phrases []string
	var conns []Connection
	for _, entry := range t.lets {
		use, _ := entry["use"].(string)
		if use == "" {
			continue
		}
		phrase := said(use) + ", "
		if max, ok := wholeNumber(entry["uses"]); ok {
			if max == 1 {
				phrase += "1 use"
			} else {
				phrase += fmt.Sprintf("%d uses", max)
			}
		} else {
			phrase += "with no count of its own"
		}
		phrases = append(phrases, phrase)
		conns = append(conns, Connection{Kind: ConnAbility, ID: use, Role: RoleLets})
	}
	if len(phrases) == 0 {
		return Statement{}, false
	}
	block := quote(id)
	if t.name != "" {
		block += " (" + t.name + ")"
	}
	at := "templates." + id + ".lets"
	sentence := fmt.Sprintf("The stat block %s can use %s. What that is and how it is counted is written in its own words "+
		"under %s, and the rules judge them. A behaviour's row uses one with `activate`.", block, joinAnd(phrases), quote(at))
	return b.stmt(KindTemplateLets, Subject{Kind: "template", ID: id}, at, sentence, conns...), true
}

// templateLetsSource is the part of a stat block this statement reads.
type templateLetsSource struct {
	name string
	lets []map[string]any
}

// wholeNumber reads the `max` of a `uses` mapping when it is a whole number.
func wholeNumber(uses any) (int, bool) {
	m, ok := uses.(map[string]any)
	if !ok {
		return 0, false
	}
	switch n := m["max"].(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint64:
		return int(n), true
	case float64:
		if n == math.Trunc(n) && !math.IsInf(n, 0) {
			return int(n), true
		}
	}
	return 0, false
}
