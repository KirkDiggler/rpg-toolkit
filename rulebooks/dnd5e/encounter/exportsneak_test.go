// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// ExportedRebuildPerceptsFor runs one sight pass for exactly the named
// observers — the shape of the single-observer pass a concealment reveal runs
// ([Encounter.revealConcealmentTo]) — for the external test package, so a
// scene can show that a pass judges only its own observers' pairs.
//
// A TEST-ONLY DOOR, in a _test.go file, for the reason [ExportedPlacedCentre]
// gives.
func ExportedRebuildPerceptsFor(e *Encounter, observers ...MemberID) error {
	_, err := e.rebuildPercepts(observers)
	return err
}
