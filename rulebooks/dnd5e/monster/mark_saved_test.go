// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

// markSaved clears the persistence flag so the next assertion is about the
// action under test alone. The twin of character's helper of the same name,
// and there for the same reason: Monster.MarkClean was deleted in
// rpg-project#319 Phase 6 as production code with no production caller, and
// these tests were always pinning the transition rather than the verb.
//
// Monster.MarkDirty is NOT its counterpart and stays — that one has a live
// caller and a pin of its own.
func markSaved(m *Monster) {
	m.dirty = false
}
