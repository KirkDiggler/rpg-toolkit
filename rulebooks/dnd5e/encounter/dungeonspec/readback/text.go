// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"strings"
)

// Text renders the read-back for a person: each section, each thing in it,
// and every sentence followed on the next line by where it is set.
func (r *ReadBack) Text() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "READ-BACK: %s (%s)\n", r.Name, r.Key)
	for _, s := range r.Sections {
		if len(s.Groups) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n%s\n", strings.ToUpper(s.Title))
		for _, g := range s.Groups {
			sb.WriteString(GroupText(g))
		}
	}
	sb.WriteString("\nNOT YET SAID\n")
	if len(r.NotYetSaid) == 0 {
		sb.WriteString("  Nothing: every authored thing above is said or summarised.\n")
	}
	for _, u := range r.NotYetSaid {
		fmt.Fprintf(&sb, "  - %s (%s): %s\n", u.Path, u.Kind, u.Why)
	}
	return sb.String()
}

// GroupText renders one group: its title, then each sentence and its path.
func GroupText(g Group) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  %s\n", g.Title)
	for _, st := range g.Statements {
		fmt.Fprintf(&sb, "    - %s\n        at %s\n", st.Sentence, st.Path)
	}
	return sb.String()
}

// Group is the group about a subject in a section, or false.
func (r *ReadBack) Group(section string, about Subject) (Group, bool) {
	for _, s := range r.Sections {
		if s.Title != section {
			continue
		}
		for _, g := range s.Groups {
			if g.About == about {
				return g, true
			}
		}
	}
	return Group{}, false
}
