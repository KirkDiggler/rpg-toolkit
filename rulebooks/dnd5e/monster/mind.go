// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import "fmt"

// Mind names which mind a monster thinks with — the behaviour a driver
// gives it when nobody is playing it. It is authored on the definition,
// crosses to the encounter as an opaque string beside Targeting, and is
// looked up by the driver (mind/behavior adoption, rpg-toolkit#1725, rule
// A5). The vocabulary is sealed here because a mind is a choice a designer
// names, not a rule the toolkit hardcodes.
type Mind string

// Mind constants. MindUnspecified is the zero value: the definition named
// nothing, and the driver answers with its basic behaviour. It is not an
// authorable choice; ParseMind never returns it.
const (
	// MindUnspecified means no mind was named — the basic driver.
	MindUnspecified Mind = ""
	// MindRetaliator turns on whoever attacked it while the deed is fresh,
	// and otherwise goes for the closest. The bow skeleton's mind.
	MindRetaliator Mind = "retaliator"
)

// SetMind names the mind this monster thinks with.
func (m *Monster) SetMind(mind Mind) {
	m.mind = mind
}

// Mind returns the mind this monster thinks with; MindUnspecified when the
// definition named none.
func (m *Monster) Mind() Mind {
	return m.mind
}

// ParseMind parses the author-facing mind vocabulary ("retaliator") into a
// Mind. Any other value, including the empty string, is rejected — a caller
// that wants MindUnspecified for an absent value must not call this on it.
func ParseMind(s string) (Mind, error) {
	switch Mind(s) {
	case MindRetaliator:
		return MindRetaliator, nil
	default:
		return MindUnspecified, fmt.Errorf("invalid mind %q (must be %q)", s, MindRetaliator)
	}
}

// String is the author-facing label, the inverse of ParseMind for every
// value it can produce. MindUnspecified is "" — it names an absence.
func (m Mind) String() string {
	return string(m)
}
