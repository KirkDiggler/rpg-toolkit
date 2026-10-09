// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

// Every capability a stand-in must answer, asserted here so that a capability
// added to this module fails this module's build until its stand-in answers it.
var (
	_ InitiativeRoller          = refusingInitiative{}
	_ StandingWithParticipation = nobodyDown{}
	_ Sight                     = zeroSight{}
	_ EquipmentWithConditions   = UnobservedEquipment{}
	_ Sheets                    = noSheets{}
	_ Driver                    = PassDriver{}
	_ Striker                   = RefusingStriker{}
	_ Mover                     = RefusingMover{}
	_ Announcer                 = RefusingAnnouncer{}
	_ CheckResolver             = RefusingCheckResolver{}
	_ Witness                   = NobodyPerceives{}
)

// UnobservedEquipment is an Equipment and Conditions for a world being
// constructed before a host installs real capabilities; a session replaces it
// at load. [RefusingCapabilities] installs it.
//
// Every member asked about is answered nil for both: no hands and no
// conditions to observe. Never empty hands or an empty condition set — those
// are claims that somebody was looked at and found holding nothing, and nobody
// has been looked at here (see [Equipment] and [ConditionSet]). It is also the
// honest fixture for any scene with no sheets behind its members.
type UnobservedEquipment struct{}

// Equipment answers nil — nothing to observe — for every given member.
func (UnobservedEquipment) Equipment(members []MemberID) (map[MemberID]*HeldEquipment, error) {
	out := make(map[MemberID]*HeldEquipment, len(members))
	for _, id := range members {
		out[id] = nil
	}
	return out, nil
}

// Conditions answers nil — nothing to observe — for every given member.
func (UnobservedEquipment) Conditions(members []MemberID) (map[MemberID]*ConditionSet, error) {
	out := make(map[MemberID]*ConditionSet, len(members))
	for _, id := range members {
		out[id] = nil
	}
	return out, nil
}

// nobodyDown answers standing and participation for a world being compiled
// with nobody in it; see [RefusingCapabilities] for why Assess refuses members.
type nobodyDown struct{}

func (nobodyDown) Standing([]MemberID) ([]MemberID, error) {
	return []MemberID{}, nil
}

func (nobodyDown) Assess(members []MemberID) (*ParticipationAssessment, error) {
	if len(members) > 0 {
		return nil, fmt.Errorf("assess %d members: %w", len(members), ErrRefusingParticipation)
	}
	return &ParticipationAssessment{Members: []MemberParticipation{}}, nil
}

// noSheets answers sheets for a world being compiled with nobody in it; see
// [RefusingCapabilities] for why it refuses members.
type noSheets struct{}

func (noSheets) Sheets(members []MemberID) (map[MemberID]SheetFacts, error) {
	if len(members) > 0 {
		return nil, fmt.Errorf("sheets for %d members: %w", len(members), ErrRefusingSheets)
	}
	return map[MemberID]SheetFacts{}, nil
}

// zeroSight gives every member a range of zero cells.
type zeroSight struct{}

func (zeroSight) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 0
	}
	return out, nil
}

// refusingInitiative fails every roll with ErrRefusingInitiative; see
// [RefusingCapabilities] for why it refuses rather than ordering.
type refusingInitiative struct{}

func (refusingInitiative) RollInitiative([]MemberID) ([]MemberID, error) {
	return nil, fmt.Errorf("roll initiative: %w", ErrRefusingInitiative)
}

// RefusingCheckResolver is a CheckResolver for a world compiled or loaded
// only to be inspected — [RefusingStriker]'s pattern one capability over. A
// find check is rolled only through an explicit Search, which nothing that
// compiles, previews or re-serializes a world does, so reaching it is a HOST
// BUG reported by name rather than answered with an invented roll.
// [RefusingCapabilities] install it.
type RefusingCheckResolver struct{}

// ResolveCheck always fails with ErrRefusingCheckResolver.
func (RefusingCheckResolver) ResolveCheck(*ResolveCheckInput) (*ResolveCheckOutput, error) {
	return nil, fmt.Errorf("resolve check: %w", ErrRefusingCheckResolver)
}

// NobodyPerceives is a Witness that answers nobody perceives the door, as an
// empty list. [RefusingCapabilities] install it.
//
// IT ANSWERS RATHER THAN REFUSES, unlike the other stand-ins, because the
// witness is asked by every sight refresh, not by a verb a compile-only host
// avoids: [NewEncounter]'s first light asks it for an authored concealed door
// that stands open even with no members (legal content, rpg-api#887). With
// zero sight nobody perceives anything, so "nobody" is the true answer, not an
// invented one. [LoadEncounter] itself never asks it — load re-derives
// presence (sweepOccupancy) but runs no sight refresh — so a refusing witness
// would only move the failure to the first verb that refreshes sight, on a
// world whose only fault is a door standing open.
type NobodyPerceives struct{}

// Perceivers answers an empty list: nobody perceives the door.
func (NobodyPerceives) Perceivers(*PerceiversInput) ([]MemberID, error) {
	return []MemberID{}, nil
}

// RefusingDriver is a [Driver] for a world loaded only to be inspected —
// [RefusingStriker]'s pattern one capability over. [RefusingCapabilities] installs
// it rather than [PassDriver] because a loaded world may hold members: a
// silent Pass would turn a host that drove a compile-only world into a board
// of idle monsters, where this names the bug.
type RefusingDriver struct{}

// Act always fails with ErrRefusingDriver.
func (RefusingDriver) Act(MonsterView) (Decision, error) {
	return Decision{}, ErrRefusingDriver
}
