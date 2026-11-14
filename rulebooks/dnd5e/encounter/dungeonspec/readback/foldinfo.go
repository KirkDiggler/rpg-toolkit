// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

// foldInfo is what the folded rendering needs of a statement beyond its
// sentence: the short clause it is folded as, and the keys it is grouped by.
// It is set where the statement is made, from the same compiled values, and
// never serialised: the structured statements are unchanged by it.
type foldInfo struct {
	// text is the statement's short clause. "{self}" and "{Self}" stand for
	// the short name of the thing the statement is about ("the guard"), so
	// two creatures' clauses compare equal when they differ only by name.
	text string
	// miss is an advance's miss half; text is its win half.
	miss string
	// line is a line facet's heading: “say” (`id`).
	line string
	// done marks a line statement whose option is something done, not said
	// (rpg-project#959): its plain rows fold under "When done".
	done bool
	// state is the routine state a post or a wake belongs to.
	state string
	// verdict is a row's or a grant's: "win", "miss" or "said".
	verdict string
	// site names the place a hub lists this statement under ("a win of
	// `straggler`"), and who the creature it belongs to, when any.
	site string
	who  string
}

// fstmt is stmt with the statement's fold.
func (b *builder) fstmt(kind Kind, about Subject, path, sentence string, f foldInfo, conns ...Connection) Statement {
	s := Statement{Kind: kind, About: about, Sentence: sentence, Path: path, Connects: dedupe(conns), fold: f}
	for _, c := range s.Connects {
		if c.Kind == "topic" || c.Kind == "fact" {
			b.hubs = append(b.hubs, s)
			break
		}
	}
	return s
}
