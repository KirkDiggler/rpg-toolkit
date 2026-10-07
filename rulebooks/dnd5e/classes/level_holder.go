// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package classes

// LevelHolder answers the named class-level question: how many levels a
// member holds in one class. A character answers it from its level record, at
// the moment it is asked; nothing below the sheet keeps a copy of the answer.
//
// A feature activated by its owner asks the owner through this interface, and
// a sheet hands itself as one to an override it calls during its own attack
// assembly. Zero means the member holds no levels in the class; an asker whose
// number scales with that class refuses it rather than reading it as level
// one.
type LevelHolder interface {
	// ClassLevel returns how many levels the member holds in class.
	ClassLevel(class Class) int
}
