// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

// CompileOnlySetup returns a SetupInput for a world being compiled, not played:
// the given field and endings, and every capability [NewEncounter] requires
// stood in by this module (rpg-toolkit#1956).
//
// A host that builds a world before anybody is in it — rpg-api's sessionworld
// builds one per dungeon launch, keeps only [Encounter.ToData], and the session
// package loads that data with the real capabilities — has no sheets to answer
// from. This is the one place that knows which capabilities an encounter
// requires, so a capability added here is stood in here, and the host's call
// does not change. A host that hand-wrote its stand-ins broke the moment
// [Conditions] was added: every launch failed with ErrNoConditions.
//
// The returned value is the caller's to finish: Members, Retention and Roller
// are left zero because they are the host's choices, not capabilities. Each
// stand-in answers without inventing an observation, except where the
// capability has no "not observed" answer, and those say what they do:
//
//   - Standing: nobody is down, as an empty list — literally true of a world
//     with nobody in it.
//   - Participation: an empty ask gets an empty assessment. A non-empty ask
//     REFUSES with ErrRefusingParticipation: participation has no "not
//     observed" answer, and saying a member is conscious and in contact would
//     be a confident answer nobody observed. The documented caller places no
//     members, so only an empty ask reaches it; a host that places members in
//     a compiled world supplies its own Standing. Never asked in play: the
//     capabilities are not persisted and every load supplies its own.
//   - Initiative: REFUSES with ErrRefusingInitiative. Initiative has no "not
//     observed" answer, and any order this could give — the order asked is
//     the forming members sorted by ID — would be a confident wrong answer
//     deciding a fight. With zero sight no fight can form here, so reaching
//     it means a host changed the setup and tried to play a compiled world.
//   - Sight: zero cells for every member; no sighting is written.
//   - Equipment and Conditions: nil for every member — nothing to observe,
//     never empty hands or an empty set ([UnobservedEquipment]).
//   - Sheets: an empty ask gets an empty answer; a non-empty ask REFUSES with
//     ErrRefusingSheets, Participation's shape and for its reason. A speed or
//     a reach has no "not observed" answer — zero is a real speed — and a
//     compiled world has no sheets behind its members. Asked only when a walk
//     is paced, a turn budgeted or a driver's view built, none of which
//     compiling a world does.
//   - Witness: nobody perceives any door. It answers rather than refuses,
//     because first light asks it for an authored concealed door that stands
//     open even with no members (legal content, rpg-api#887).
//   - CheckResolver: refuses with ErrRefusingCheckResolver; a check is only
//     rolled through an explicit Search, which nothing compiling a world does.
//   - TurnDriver, Striker, Mover, Announcer: [PassDriver], [RefusingStriker],
//     [RefusingMover], [RefusingAnnouncer].
//
// [CompileOnlyLoad] is the same for [LoadEncounter].
func CompileOnlySetup(field FieldInput, endings []EndingInput) *SetupInput {
	return &SetupInput{
		Field:         field,
		Endings:       endings,
		Initiative:    refusingInitiative{},
		Standing:      nobodyDown{},
		Sight:         zeroSight{},
		Equipment:     UnobservedEquipment{},
		Sheets:        noSheets{},
		TurnDriver:    PassDriver{},
		Striker:       RefusingStriker{},
		Mover:         RefusingMover{},
		Announcer:     RefusingAnnouncer{},
		CheckResolver: RefusingCheckResolver{},
		Witness:       NobodyPerceives{},
	}
}

// CompileOnlyLoad returns a LoadEncounterInput for a persisted world being
// loaded only to be inspected or re-serialized — never played: the given data,
// and every capability [LoadEncounter] requires stood in by this module, the
// same stand-ins [CompileOnlySetup] installs (rpg-toolkit#1958).
//
// A host that loads an authored world to prove it loads, preview its atlas or
// re-serialize it — session's StartSession validation load and AtlasOf — has
// no turn to drive and no check to roll. This is the one place that knows
// which capabilities a load requires, so a capability added to LoadEncounter
// is stood in here and the host's call does not change.
//
// The returned value is the caller's to finish: Roller is left nil (a load
// that rolls is not compile-only), and any capability the host CAN answer —
// real Initiative, Standing, Sight, Equipment or Sheets from the sheets behind a
// world's members — it overwrites on the returned value. Left as returned:
//
//   - Standing: nobody down. Participation REFUSES a non-empty ask with
//     ErrRefusingParticipation, as at Setup; [LoadEncounter] itself never
//     asks it, so a world WITH members loads, and the refusal fires only if
//     the host goes on to play it.
//   - Initiative: REFUSES with ErrRefusingInitiative. Load forms no fight.
//   - Sight: zero cells for every member; Equipment and Conditions:
//     [UnobservedEquipment].
//   - Sheets: REFUSES a non-empty ask with ErrRefusingSheets, as at Setup.
//     [LoadEncounter] never asks it, so a world with members loads; a host
//     that walks or drives one supplies its own.
//   - TurnDriver: [RefusingDriver], not [PassDriver] — a loaded world may
//     hold members, and a silent pass would hide a driven turn.
//   - Striker, Mover, Announcer: [RefusingStriker], [RefusingMover],
//     [RefusingAnnouncer].
//   - CheckResolver: [RefusingCheckResolver]; Witness: [NobodyPerceives].
//     Both are installed whether or not the data declares a concealment;
//     LoadEncounter holds them only when it does. LoadEncounter never asks
//     the witness (see [NobodyPerceives]); a later sight refresh would, and
//     with zero sight nobody is the true answer.
func CompileOnlyLoad(data EncounterData) *LoadEncounterInput {
	return &LoadEncounterInput{
		Data:          data,
		Initiative:    refusingInitiative{},
		Standing:      nobodyDown{},
		Sight:         zeroSight{},
		Equipment:     UnobservedEquipment{},
		Sheets:        noSheets{},
		TurnDriver:    RefusingDriver{},
		Striker:       RefusingStriker{},
		Mover:         RefusingMover{},
		Announcer:     RefusingAnnouncer{},
		CheckResolver: RefusingCheckResolver{},
		Witness:       NobodyPerceives{},
	}
}

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
// at load. [CompileOnlySetup] installs it.
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
// with nobody in it; see [CompileOnlySetup] for why Assess refuses members.
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
// [CompileOnlySetup] for why it refuses members.
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
// [CompileOnlySetup] for why it refuses rather than ordering.
type refusingInitiative struct{}

func (refusingInitiative) RollInitiative([]MemberID) ([]MemberID, error) {
	return nil, fmt.Errorf("roll initiative: %w", ErrRefusingInitiative)
}

// RefusingCheckResolver is a CheckResolver for a world compiled or loaded
// only to be inspected — [RefusingStriker]'s pattern one capability over. A
// find check is rolled only through an explicit Search, which nothing that
// compiles, previews or re-serializes a world does, so reaching it is a HOST
// BUG reported by name rather than answered with an invented roll.
// [CompileOnlySetup] and [CompileOnlyLoad] install it.
type RefusingCheckResolver struct{}

// ResolveCheck always fails with ErrRefusingCheckResolver.
func (RefusingCheckResolver) ResolveCheck(*ResolveCheckInput) (*ResolveCheckOutput, error) {
	return nil, fmt.Errorf("resolve check: %w", ErrRefusingCheckResolver)
}

// NobodyPerceives is a Witness that answers nobody perceives the door, as an
// empty list. [CompileOnlySetup] and [CompileOnlyLoad] install it.
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
// [RefusingStriker]'s pattern one capability over. [CompileOnlyLoad] installs
// it rather than [PassDriver] because a loaded world may hold members: a
// silent Pass would turn a host that drove a compile-only world into a board
// of idle monsters, where this names the bug.
type RefusingDriver struct{}

// Act always fails with ErrRefusingDriver.
func (RefusingDriver) Act(MonsterView) (Decision, error) {
	return Decision{}, ErrRefusingDriver
}
