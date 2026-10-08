// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions

import (
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// ClassLevel is one class and the number of levels a member holds in it.
type ClassLevel struct {
	Class  classes.Class
	Levels int
}

// ClassLevels is a member's class levels as a frame fact: the levels it holds
// in each class, or unknown. Its zero value is unknown.
//
// Known with no entries is a real answer — a monster's stat block holds no
// class levels — and is distinct from unknown, which means the producer did
// not establish the fact. A class the known list does not name is held at zero
// levels.
//
// Resolution fills it from the acting sheet, one type for both kinds of
// member: a character's level record gives one [ClassLevel] per class it has
// levels in, and a monster gives KnownClassLevels() with no entries. A rule
// computes a class-scaled number from the level in that class, never from the
// member's total level.
type ClassLevels struct {
	levels []ClassLevel
	known  bool
}

// KnownClassLevels records the class levels a member holds, in the order the
// producer supplies them. Called with nothing, it records a member known to
// hold no class levels. The list is copied, and nothing hands out the held
// slice, so a ClassLevels never changes once made. Malformed entries are
// refused by [ClassLevels.Validate], not here.
func KnownClassLevels(levels ...ClassLevel) ClassLevels {
	return ClassLevels{levels: slices.Clone(levels), known: true}
}

// UnknownClassLevels records that the producer did not establish the member's
// class levels.
func UnknownClassLevels() ClassLevels { return ClassLevels{} }

// Get returns a copy of the class levels and whether they are known. A known
// empty list is a member holding no class levels.
func (c ClassLevels) Get() ([]ClassLevel, bool) {
	return slices.Clone(c.levels), c.known
}

// Of returns the levels held in class and whether the class levels are known.
// A known list that does not name class answers zero, known: the member holds
// no levels in it.
func (c ClassLevels) Of(class classes.Class) (int, bool) {
	if !c.known {
		return 0, false
	}
	for _, entry := range c.levels {
		if entry.Class == class {
			return entry.Levels, true
		}
	}
	return 0, true
}

// Validate refuses a known list no rule can read: an entry with no class, a
// class listed twice, or levels below one. A class held at zero levels is
// omitted, never listed. Unknown class levels are valid.
func (c ClassLevels) Validate() error {
	if !c.known {
		return nil
	}
	seen := make(map[classes.Class]bool, len(c.levels))
	for _, entry := range c.levels {
		if entry.Class == "" {
			return fmt.Errorf("class levels name an empty class")
		}
		if seen[entry.Class] {
			return fmt.Errorf("class levels list %q twice", entry.Class)
		}
		seen[entry.Class] = true
		if entry.Levels < 1 {
			return fmt.Errorf("class levels hold %d levels of %q; a held class has at least one", entry.Levels, entry.Class)
		}
	}
	return nil
}
