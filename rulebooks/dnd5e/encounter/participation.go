// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"
)

// TurnParticipation tells the encounter what to do when initiative reaches a
// member. The rulebook owns the reason; the encounter owns only the clock
// consequence.
type TurnParticipation string

const (
	// TurnParticipationWait retains the initiative slot and waits for its
	// player or driver. Conscious and dying members can both wait for different
	// rulebook reasons the encounter does not interpret.
	TurnParticipationWait TurnParticipation = "wait"

	// TurnParticipationAutoPass retains the initiative slot but advances it
	// without client input whenever the slot becomes active.
	TurnParticipationAutoPass TurnParticipation = "auto_pass"

	// TurnParticipationRemove transfers the member out of a turn bubble. The
	// member remains on the map, in the encounter roster, and on the world
	// clock. A Remove member cannot also report Contact; that incoherent answer
	// is refused as invalid participation data.
	TurnParticipationRemove TurnParticipation = "remove"
)

// MemberParticipation is the rulebook-neutral participation answer for one
// encounter member.
type MemberParticipation struct {
	Member    MemberID
	Down      bool
	Contact   bool
	Conscious bool
	Turn      TurnParticipation
}

// ParticipationAssessment is one complete answer about the roster supplied to
// [Participation.Assess]. PartyDefeated and KeepTurnOrder are group policy
// supplied by the rulebook; the encounter never derives either from member
// counts. PartyDefeated takes precedence over KeepTurnOrder.
type ParticipationAssessment struct {
	Members       []MemberParticipation
	PartyDefeated bool
	// KeepTurnOrder retains a one-sided bubble so members such as dying
	// characters can finish ordered turns after their opposition is removed.
	// A later false answer reconciles and dissolves that bubble.
	KeepTurnOrder bool
}

// Participation assesses encounter membership for contact, narration,
// initiative, and party-defeat policy.
type Participation interface {
	// Assess returns exactly one answer for every member in the question.
	Assess(members []MemberID) (*ParticipationAssessment, error)
}

// StandingWithParticipation is the migration bridge for stored-data and
// resolution constructors. Their Standing field remains source-compatible,
// while its concrete value must also provide the richer participation answer.
type StandingWithParticipation interface {
	Standing
	Participation
}

// participationState is one validated assessment indexed for all of the
// consequences a pass applies. It is deliberately ephemeral: every pass asks
// again, and nothing about participation is persisted or cached.
type participationState struct {
	assessment       *ParticipationAssessment
	members          map[MemberID]MemberParticipation
	down             map[MemberID]bool
	contact          map[MemberID]bool
	scheduledWrapped bool
	scheduledLastSeq uint64
}

// participationNow asks one stable, complete question and validates the
// capability's answer before any caller acts on it.
func (e *Encounter) participationNow() (*participationState, error) {
	if len(e.members) == 0 {
		return &participationState{
			assessment: &ParticipationAssessment{},
			members:    map[MemberID]MemberParticipation{},
			down:       map[MemberID]bool{},
			contact:    map[MemberID]bool{},
		}, nil
	}

	roster := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		roster = append(roster, id)
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i] < roster[j] })

	assessment, err := e.participation.Assess(roster)
	if err != nil {
		return nil, fmt.Errorf("participation: %w", err)
	}
	if assessment == nil {
		return nil, fmt.Errorf("participation: capability returned no assessment: %w", ErrInvalidData)
	}

	asked := make(map[MemberID]bool, len(roster))
	for _, id := range roster {
		asked[id] = true
	}

	state := &participationState{
		assessment: &ParticipationAssessment{
			PartyDefeated: assessment.PartyDefeated,
			KeepTurnOrder: assessment.KeepTurnOrder,
		},
		members: make(map[MemberID]MemberParticipation, len(roster)),
		down:    make(map[MemberID]bool),
		contact: make(map[MemberID]bool),
	}
	for _, member := range assessment.Members {
		if !asked[member.Member] {
			return nil, fmt.Errorf("participation: reported %q, who is not a member: %w", member.Member, ErrNotMember)
		}
		if _, duplicate := state.members[member.Member]; duplicate {
			return nil, fmt.Errorf("participation: reported %q twice: %w", member.Member, ErrInvalidData)
		}
		switch member.Turn {
		case TurnParticipationWait, TurnParticipationAutoPass:
		case TurnParticipationRemove:
			if member.Contact {
				return nil, fmt.Errorf(
					"participation: member %q cannot be removed while remaining in contact: %w",
					member.Member, ErrInvalidData)
			}
		default:
			return nil, fmt.Errorf("participation: member %q has unknown turn participation %q: %w",
				member.Member, member.Turn, ErrInvalidData)
		}

		state.assessment.Members = append(state.assessment.Members, member)
		state.members[member.Member] = member
		if member.Down {
			state.down[member.Member] = true
		}
		if member.Contact {
			state.contact[member.Member] = true
		}
	}

	for _, id := range roster {
		if _, ok := state.members[id]; !ok {
			return nil, fmt.Errorf("participation: assessment omitted member %q: %w", id, ErrInvalidData)
		}
	}

	return state, nil
}
