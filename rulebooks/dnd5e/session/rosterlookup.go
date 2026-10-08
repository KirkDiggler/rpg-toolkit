// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// rosterNames indexes a roster read by member id — the lookup Sighting.Name
// and Participant.Name both need (rpg-toolkit#1137), built once per verb
// from a roster read the caller already has rather than fetched again per
// subject.
func rosterNames(roster []encounter.Member) map[string]string {
	out := make(map[string]string, len(roster))
	for _, m := range roster {
		out[string(m.ID)] = m.Name
	}
	return out
}

// rosterKinds indexes a roster read by member id — the same batching
// rosterNames does, for Sighting.Kind (rpg-toolkit#1230). The roster is the
// one place a member's kind lives; this is the only lookup, reused wherever
// a Sighting is projected rather than invented a second time.
func rosterKinds(roster []encounter.Member) map[string]MemberKind {
	out := make(map[string]MemberKind, len(roster))
	for _, m := range roster {
		out[string(m.ID)] = MemberKind(m.Kind)
	}
	return out
}

// encounterRosterKinds snapshots authoritative kinds from one live roster read
// for a standalone session projection. The participation seam copies this map,
// so one assessment cannot observe a second roster snapshot.
func encounterRosterKinds(roster []encounter.Member) map[string]encounter.MemberKind {
	out := make(map[string]encounter.MemberKind, len(roster))
	for _, member := range roster {
		out[string(member.ID)] = member.Kind
	}
	return out
}

// encounterDataKinds snapshots authoritative kinds at a persisted-world load
// or setup boundary. It is the construction twin of encounterRosterKinds.
func encounterDataKinds(roster []encounter.MemberData) map[string]encounter.MemberKind {
	out := make(map[string]encounter.MemberKind, len(roster))
	for _, member := range roster {
		out[string(member.ID)] = member.Kind
	}
	return out
}

// standingSet batches a down-check over a set of member ids into a lookup —
// one Standing() call per verb rather than one per sighted subject or
// participant, and the same batching turn.go's own participantsFor uses.
func standingSet(standing encounter.Standing, ids []encounter.MemberID) (map[string]bool, error) {
	down, err := standing.Standing(ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(down))
	for _, id := range down {
		out[string(id)] = true
	}
	return out, nil
}

type richParticipationLookup struct {
	members map[string]encounter.MemberParticipation
	views   map[string]participantView
}

// richParticipationSet derives encounter scheduling, binary down, explicit
// life state, and Death Save progress from one provider snapshot.
func richParticipationSet(
	standing standingSeam, ids []encounter.MemberID,
) (*richParticipationLookup, error) {
	snapshot, err := standing.participation(ids)
	if err != nil {
		return nil, err
	}
	members := make(map[string]encounter.MemberParticipation, len(snapshot.assessment.Members))
	for _, member := range snapshot.assessment.Members {
		members[string(member.Member)] = member
	}
	return &richParticipationLookup{members: members, views: snapshot.views}, nil
}

// rosterIDs pulls the bare ids out of a roster read, for callers that need
// to ask Standing about everyone rather than a narrower set (View's own
// call: the observer might hold a sighting for anyone in the roster).
func rosterIDs(roster []encounter.Member) []encounter.MemberID {
	out := make([]encounter.MemberID, len(roster))
	for i, m := range roster {
		out[i] = m.ID
	}
	return out
}

// believedStances indexes what ONE VIEWER believes about every member of the
// roster, by member id — the same batching rosterNames and rosterKinds do, for
// [Sighting.Stance] (rpg-project#458).
//
// PER VIEWER, WHICH IS THE WHOLE POINT, and why this cannot be a roster read
// like the other two: a name and a kind are facts about the subject and the
// same for everybody, while a stance is a fact about the pair. The viewer is
// the member the View was taken for.
//
// NO SIDE IS EMPTY, not "neutral". The composition answers
// [encounter.StanceNone] for a member in no faction, which a world NPC is; the
// sighting carries that as the empty stance its own contract names, so a
// client never paints a vendor the colour of a truce. A viewer or subject the
// composition refuses (ErrNoMember, ErrNotMember) is an error, never a quiet
// gap: every subject here is a roster member, so a refusal means the viewer
// itself is not one.
func believedStances(
	enc *encounter.Encounter, viewer string, roster []encounter.Member,
) (map[string]string, error) {
	out := make(map[string]string, len(roster))
	for _, m := range roster {
		stance, err := enc.BelievedStance(encounter.MemberID(viewer), m.ID)
		if err != nil {
			return nil, translate(err)
		}
		if stance == encounter.StanceNone {
			continue
		}
		out[string(m.ID)] = string(stance)
	}

	return out, nil
}
