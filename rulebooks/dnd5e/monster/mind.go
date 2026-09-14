// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"fmt"
	"strings"
)

// Mind names which mind a monster thinks with — the behaviour a driver
// gives it when nobody is playing it. It is authored on the definition,
// crosses to the encounter as an opaque string beside Targeting, and is
// looked up by the driver (mind/behavior adoption, rpg-toolkit#1725, rule
// A5). The vocabulary is sealed here because a mind is a choice a designer
// names, not a rule the toolkit hardcodes.
//
// Each word is one preset of one behaviour, and the words are the only dial
// an author has: the driver holds the profile a word means, so a monster
// tuned differently needs a new word and a toolkit release
// (rpg-toolkit#1745). That cost is the argument for minds as authored data,
// and it is deliberately being paid a few times first, so the fields a
// profile has are known before any format is chosen.
type Mind string

// Mind constants. MindUnspecified is the zero value: the definition named
// nothing, and the driver answers with its basic behaviour. It is not an
// authorable choice; ParseMind never returns it.
const (
	// MindUnspecified means no mind was named — the basic driver.
	MindUnspecified Mind = ""
	// MindRetaliator turns on whoever attacked it while the deed is fresh
	// and they are still holding something to shoot back with, and
	// otherwise goes for the closest. The bow skeleton's mind.
	MindRetaliator Mind = "retaliator"
	// MindBerserker turns on whoever attacked it and does not care what
	// they are holding: nothing but the clock talks it off a grudge. The
	// thug's mind.
	MindBerserker Mind = "berserker"
	// MindCoward holds no grudge at all: it shoots whoever is nearest from
	// a distance and steps away from whatever closes on it. The goblin's
	// mind.
	MindCoward Mind = "coward"
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

// authorable is the vocabulary ParseMind accepts, in the order its error
// lists them. MindUnspecified is deliberately absent: it names an absence,
// and an author who wrote nothing never called this.
var authorable = []Mind{MindRetaliator, MindBerserker, MindCoward}

// ParseMind parses the author-facing mind vocabulary ("retaliator",
// "berserker", "coward") into a Mind. Any other value, including the empty
// string, is rejected — a caller that wants MindUnspecified for an absent
// value must not call this on it.
func ParseMind(s string) (Mind, error) {
	for _, mind := range authorable {
		if Mind(s) == mind {
			return mind, nil
		}
	}

	words := make([]string, len(authorable))
	for i, mind := range authorable {
		words[i] = fmt.Sprintf("%q", mind)
	}

	return MindUnspecified, fmt.Errorf("invalid mind %q (must be one of %s)", s, strings.Join(words, ", "))
}

// String is the author-facing label, the inverse of ParseMind for every
// value it can produce. MindUnspecified is "" — it names an absence.
func (m Mind) String() string {
	return string(m)
}
