// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Display is the rulebook-owned, display-ready projection of one condition
// ref: a never-empty display name and optional server-authored detail. It is
// the value a character StatusView looks up by ref so the projection never has
// to serialize a condition back to JSON to recover a human-readable label, and
// so the set of conditions a status view can name is bounded by the registry
// rather than by whatever a live condition happens to carry.
type Display struct {
	// Name is the condition's display name. Never empty for a known ref.
	Name string

	// Detail is optional, server/toolkit-composed display text. May be empty.
	Detail string
}

// DisplayFor returns the display descriptor for the given condition ref, or
// (Display{}, false) when the ref is not registered. The character projection
// turns a false result into a hard error rather than silently dropping the
// condition, so an unknown ref fails loudly.
func DisplayFor(ref core.Ref) (Display, bool) {
	entry, err := lookup(ref.String())
	if err != nil {
		return Display{}, false
	}
	return entry.display, true
}
