// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// testconceal_test.go is the scripted stand-ins for the two concealment
// capabilities (rpg-toolkit#1371), in the same spirit as everyoneStanding
// and everyoneSeesTheWholeMap: the rule lives in the fixture, standing in
// for the session seam that will supply it for real, and each scene picks
// the answer it is about.

// findsNothing is a CheckResolver whose every check fails — the fixture for
// scenes that are not about the roll. The applied route is the first listed
// one, which is as good a "best" as a resolver with no character sheet has.
type findsNothing struct{}

// ResolveCheck fails the check.
func (findsNothing) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return &encounter.ResolveCheckOutput{Beaten: false, Applied: in.Approaches[0]}, nil
}

// ResolveDiscoveryCheck fails the automatic check the same way.
func (f findsNothing) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return f.ResolveCheck(in)
}

// nobodyPerceives is a Witness whose answer is always nobody — the fixture
// for scenes where perception never fires.
type nobodyPerceives struct{}

// Perceivers answers that nobody perceives the door.
func (nobodyPerceives) Perceivers(*encounter.PerceiversInput) ([]encounter.MemberID, error) {
	return nil, nil
}

// scriptedWitness answers with a fixed per-door cast, standing in for the
// host's light-and-sight truth. The zero value perceives nothing.
type scriptedWitness struct {
	// perceivers maps door ID to who perceives it right now. Scenes
	// reassign entries mid-test to script somebody walking up.
	perceivers map[encounter.DoorID][]encounter.MemberID
}

// Perceivers answers the script.
func (w *scriptedWitness) Perceivers(in *encounter.PerceiversInput) ([]encounter.MemberID, error) {
	return append([]encounter.MemberID(nil), w.perceivers[in.Door]...), nil
}
