// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// Empty-world stand-ins (rpg-toolkit#1956).
//
// A host that compiles a world before anybody is in it — rpg-api's
// sessionworld builds one per dungeon launch — still has to hand
// [NewEncounter] every capability it refuses to default. The rulebook's real
// capabilities read sheets, and there are no sheets yet; the session package
// installs them when it loads the world to play it, and these are replaced at
// that load.
//
// They live HERE, beside [PassDriver] and [RefusingStriker], so that the list
// of capabilities an encounter requires is stated once, by the module that
// requires it. A host that hand-wrote these broke the moment a capability was
// added ([Conditions] made every launch fail with ErrNoConditions); a host
// that uses these does not, and the assertions below make a new capability
// fail THIS module's build until its stand-in answers it.
//
// Each answers the question it is asked without inventing an observation:
// nothing is seen, nothing is held, nobody is down. Where a capability's
// vocabulary has no "not observed" answer (standing, initiative) the stand-in
// says so in its own doc and names the choice it makes.

var (
	_ StandingWithParticipation = NobodyDown{}
	_ Sight                     = ZeroSight{}
	_ InitiativeRoller          = InitiativeAsGiven{}
	_ EquipmentWithConditions   = UnobservedEquipment{}
)

// NobodyDown is a Standing and Participation for a world being constructed
// before a host installs real capabilities; a session replaces it at load.
//
// Standing reports who is DOWN, and nobody has been hurt in a world this new,
// so it answers nobody — as an empty list, never nil with a nil error.
//
// Participation has no "unknown" answer, so Assess makes the one statement
// consistent with that: every member is up, conscious, in contact and waiting
// for their player or driver — the session's own answer for an undowned
// member. Contact is true on purpose: false would dissolve every fight the
// moment it formed. PartyDefeated and KeepTurnOrder stay false: they are group
// policy the rulebook owns and this rules on neither.
type NobodyDown struct{}

// Standing answers that none of the given members is down.
func (NobodyDown) Standing([]MemberID) ([]MemberID, error) {
	return []MemberID{}, nil
}

// Assess answers every given member up, conscious, in contact and waiting.
func (NobodyDown) Assess(members []MemberID) (*ParticipationAssessment, error) {
	out := &ParticipationAssessment{Members: make([]MemberParticipation, 0, len(members))}
	for _, id := range members {
		out.Members = append(out.Members, MemberParticipation{
			Member: id, Contact: true, Conscious: true, Turn: TurnParticipationWait,
		})
	}
	return out, nil
}

// ZeroSight is a Sight for a world being constructed before a host installs
// real capabilities; a session replaces it at load. Every member asked about
// sees zero cells: nobody in a world this new has looked at anything, so no
// sighting is written.
type ZeroSight struct{}

// Sight answers a range of zero for every given member.
func (ZeroSight) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 0
	}
	return out, nil
}

// InitiativeAsGiven is an InitiativeRoller for a world being constructed
// before a host installs real capabilities; a session replaces it at load.
//
// IT IS A CHOICE, NOT AN ABSENCE. Initiative has no "not observed" answer: it
// returns the members in the order the encounter asked, which is the forming
// members sorted by ID, so whoever's ID sorts first acts first. That is never
// reached in an empty world — no fight can form with nobody in it — and it is
// honest only for that reason. A host that constructs members into a world with this
// installed and lets a fight form gets that order, not a roll.
type InitiativeAsGiven struct{}

// RollInitiative returns the given members unchanged, in the order given.
func (InitiativeAsGiven) RollInitiative(members []MemberID) ([]MemberID, error) {
	return append([]MemberID(nil), members...), nil
}

// UnobservedEquipment is an Equipment and Conditions for a world being
// constructed before a host installs real capabilities; a session replaces it
// at load.
//
// Every member asked about is answered nil for both: no hands and no
// conditions to observe. Never empty hands or an empty condition set — those
// are claims that somebody was looked at and found holding nothing, and nobody
// has been looked at here (see [Equipment] and [ConditionSet]).
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
