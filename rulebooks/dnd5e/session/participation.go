// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// LifeState is the provider-derived combat life state projected by session.
// It is explicit even when DeathSaves is absent: consumers never infer state
// from optional progress.
type LifeState string

const (
	LifeStateUnknown    LifeState = ""
	LifeStateConscious  LifeState = "conscious"
	LifeStateDying      LifeState = "dying"
	LifeStateStabilized LifeState = "stabilized"
	LifeStateDead       LifeState = "dead"
	LifeStateDefeated   LifeState = "defeated"
)

// DeathSaveProgress is the provider-owned progress projected across the
// session boundary. The remaining fields are provider answers, not arithmetic
// session derives from successes or failures.
type DeathSaveProgress struct {
	Successes         int  `json:"successes"`
	Failures          int  `json:"failures"`
	SuccessesNeeded   int  `json:"successes_needed"`
	FailuresRemaining int  `json:"failures_remaining"`
	Stabilized        bool `json:"stabilized"`
	Dead              bool `json:"dead"`
}

// participantView is the rich session projection retained beside encounter's
// neutral scheduling answer. attackTarget is deliberately private: it is a
// provider fact used to filter Attack offers, not a new public rule surface.
type participantView struct {
	LifeState    LifeState
	DeathSaves   *DeathSaveProgress
	attackTarget bool
}

type participationSnapshot struct {
	assessment *encounter.ParticipationAssessment
	views      map[string]participantView
}

// participation asks resolution for the root rulebook's narrow participation
// projection for every available stored record, then maps that answer without
// inspecting hit points or recomputing thresholds. Missing authored sheets keep
// the legacy Standing answer (conscious/up) so authored encounter fixtures
// remain loadable; they are not counted as player characters for party policy.
func (s standingSeam) participation(
	members []encounter.MemberID,
) (*participationSnapshot, error) {
	characters, monsters, err := s.recordsFor(members)
	if err != nil {
		return nil, err
	}

	facts := make(map[string]resolution.ParticipantParticipation, len(members))
	playerFacts := make([]combat.Participation, 0, len(characters))

	characterParticipation, err := resolution.Participation(s.ctx, &resolution.ParticipationInput{
		Participants: characters,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadCharacter, err)
	}
	for _, member := range characterParticipation.Members {
		facts[member.Member] = member
		playerFacts = append(playerFacts, member.Participation)
	}

	monsterParticipation, err := resolution.Participation(s.ctx, &resolution.ParticipationInput{
		Participants: monsters,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	for _, member := range monsterParticipation.Members {
		facts[member.Member] = member
	}

	assessment := &encounter.ParticipationAssessment{
		PartyDefeated: combat.PartyDefeated(combat.PartyState{Members: playerFacts}),
	}
	var dyingPlayer, consciousPlayer bool
	for _, participation := range playerFacts {
		dyingPlayer = dyingPlayer || participation.NeedsDeathSave
		consciousPlayer = consciousPlayer || participation.Conscious
	}
	assessment.KeepTurnOrder = dyingPlayer && consciousPlayer

	views := make(map[string]participantView, len(members))
	for _, id := range members {
		fact, ok := facts[string(id)]
		if !ok {
			// Existing authored worlds can contain members without a stored sheet.
			// Binary Standing historically answered those members as up. Preserve
			// that compatibility while still returning the complete assessment
			// encounter requires.
			fact = resolution.ParticipantParticipation{
				Member:        string(id),
				Participation: combat.ParticipationFor(combat.LifeStateConscious),
			}
		}

		mapped, err := encounterParticipation(id, fact.Participation)
		if err != nil {
			return nil, err
		}
		assessment.Members = append(assessment.Members, mapped)
		views[string(id)] = participantView{
			LifeState:    LifeState(fact.Participation.State),
			DeathSaves:   projectDeathSaveProgress(fact.DeathSaves),
			attackTarget: fact.Participation.AttackTarget,
		}
	}

	return &participationSnapshot{assessment: assessment, views: views}, nil
}

func encounterParticipation(
	id encounter.MemberID, participation combat.Participation,
) (encounter.MemberParticipation, error) {
	member := encounter.MemberParticipation{
		Member: id, Down: participation.Down,
		Contact:   participation.CanActNormally,
		Conscious: participation.Conscious,
	}
	switch {
	case participation.AutoPassesTurn:
		member.Turn = encounter.TurnParticipationAutoPass
	case participation.RetainsInitiative:
		member.Turn = encounter.TurnParticipationWait
	default:
		member.Turn = encounter.TurnParticipationRemove
	}
	if member.Turn == encounter.TurnParticipationRemove && member.Contact {
		return encounter.MemberParticipation{}, fmt.Errorf(
			"participation: member %q cannot be removed while remaining in contact: %w",
			id, ErrInvalidSession)
	}
	return member, nil
}

func projectDeathSaveProgress(in *character.DeathSaveProgress) *DeathSaveProgress {
	if in == nil {
		return nil
	}
	return &DeathSaveProgress{
		Successes: in.Successes, Failures: in.Failures,
		SuccessesNeeded: in.SuccessesNeeded, FailuresRemaining: in.FailuresRemaining,
		Stabilized: in.Stabilized, Dead: in.Dead,
	}
}
