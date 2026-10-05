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
// capability has no "not observed" answer; those two make a choice, named
// here so it is never mistaken for an absence:
//
//   - Standing/Participation: nobody is down, and every member is up,
//     conscious, IN CONTACT and waiting for their player or driver — the
//     session's own answer for an undowned member. Contact is a claim, made
//     because false would dissolve every fight the moment it formed.
//     PartyDefeated and KeepTurnOrder are false.
//   - Initiative: the members in the order the encounter asks, which is the
//     forming members sorted by ID, so whoever's ID sorts first acts first.
//     Honest only because a world with nobody in it forms no fight.
//   - Sight: zero cells for every member; no sighting is written.
//   - Equipment and Conditions: nil for every member — nothing to observe,
//     never empty hands or an empty set ([UnobservedEquipment]).
//   - Witness: nobody perceives any door. It answers rather than refuses,
//     because first light asks it for an authored concealed door that stands
//     open even with no members (legal content, rpg-api#887).
//   - CheckResolver: refuses with ErrRefusingCheckResolver; a check is only
//     rolled through an explicit Search, which nothing compiling a world does.
//   - TurnDriver, Striker, Mover, Announcer: [PassDriver], [RefusingStriker],
//     [RefusingMover], [RefusingAnnouncer].
func CompileOnlySetup(field FieldInput, endings []EndingInput) *SetupInput {
	return &SetupInput{
		Field:         field,
		Endings:       endings,
		Initiative:    initiativeAsGiven{},
		Standing:      nobodyDown{},
		Sight:         zeroSight{},
		Equipment:     UnobservedEquipment{},
		TurnDriver:    PassDriver{},
		Striker:       RefusingStriker{},
		Mover:         RefusingMover{},
		Announcer:     RefusingAnnouncer{},
		CheckResolver: refusingCheckResolver{},
		Witness:       nobodyPerceives{},
	}
}

// Every capability a stand-in must answer, asserted here so that a capability
// added to this module fails this module's build until its stand-in answers it.
var (
	_ InitiativeRoller          = initiativeAsGiven{}
	_ StandingWithParticipation = nobodyDown{}
	_ Sight                     = zeroSight{}
	_ EquipmentWithConditions   = UnobservedEquipment{}
	_ Driver                    = PassDriver{}
	_ Striker                   = RefusingStriker{}
	_ Mover                     = RefusingMover{}
	_ Announcer                 = RefusingAnnouncer{}
	_ CheckResolver             = refusingCheckResolver{}
	_ Witness                   = nobodyPerceives{}
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

// nobodyDown answers standing and participation for a world being compiled;
// see [CompileOnlySetup] for the claim Assess makes.
type nobodyDown struct{}

func (nobodyDown) Standing([]MemberID) ([]MemberID, error) {
	return []MemberID{}, nil
}

func (nobodyDown) Assess(members []MemberID) (*ParticipationAssessment, error) {
	out := &ParticipationAssessment{Members: make([]MemberParticipation, 0, len(members))}
	for _, id := range members {
		out.Members = append(out.Members, MemberParticipation{
			Member: id, Contact: true, Conscious: true, Turn: TurnParticipationWait,
		})
	}
	return out, nil
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

// initiativeAsGiven returns the members in the order asked; a choice, not an
// absence — see [CompileOnlySetup].
type initiativeAsGiven struct{}

func (initiativeAsGiven) RollInitiative(members []MemberID) ([]MemberID, error) {
	return append([]MemberID(nil), members...), nil
}

// refusingCheckResolver fails every check with ErrRefusingCheckResolver.
type refusingCheckResolver struct{}

func (refusingCheckResolver) ResolveCheck(*ResolveCheckInput) (*ResolveCheckOutput, error) {
	return nil, fmt.Errorf("resolve check: %w", ErrRefusingCheckResolver)
}

// nobodyPerceives answers that nobody perceives the door, as an empty list.
type nobodyPerceives struct{}

func (nobodyPerceives) Perceivers(*PerceiversInput) ([]MemberID, error) {
	return []MemberID{}, nil
}
