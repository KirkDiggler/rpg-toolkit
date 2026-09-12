// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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
// remain loadable. An authoritative KindPlayer's fallback is the same final
// fact used by its assessment row, view, Attack targeting, and party policy;
// KindMonster and KindWorld facts never enter that policy.
func (s standingSeam) participation(
	members []encounter.MemberID,
) (*participationSnapshot, error) {
	characters, monsters, err := s.recordsFor(members)
	if err != nil {
		return nil, err
	}

	facts := make(map[string]resolution.ParticipantParticipation, len(members))

	// The same records, read for a second fact resolution's projection does not
	// carry: what each member is holding. A compulsion is a condition on a
	// sheet, and the sheet is already in hand — fetching it again would be a
	// second snapshot free to disagree with the one the life state came from.
	stored := storedConditionsOf(characters, monsters)

	characterParticipation, err := resolution.Participation(s.ctx, &resolution.ParticipationInput{
		Participants: characters,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadCharacter, err)
	}
	for _, member := range characterParticipation.Members {
		facts[member.Member] = member
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

	assessment := &encounter.ParticipationAssessment{}
	views := make(map[string]participantView, len(members))
	playerFacts := make([]combat.Participation, 0, len(characters))
	for _, id := range members {
		kind := s.kinds[string(id)]
		if kind == encounter.KindWorld {
			member, view := neutralWorldNPCParticipation(id)
			assessment.Members = append(assessment.Members, member)
			views[string(id)] = view
			continue
		}

		fact, ok := facts[string(id)]
		if !ok {
			// Existing authored worlds can contain members without a stored sheet.
			// Binary Standing historically answered those members as up. Preserve
			// that compatibility in the one final fact used by the assessment,
			// public view, Attack targeting, and KindPlayer party policy.
			fact = resolution.ParticipantParticipation{
				Member:        string(id),
				Participation: combat.ParticipationFor(combat.LifeStateConscious),
			}
		}

		mapped, err := encounterParticipation(id, fact.Participation, stored[string(id)])
		if err != nil {
			return nil, err
		}
		assessment.Members = append(assessment.Members, mapped)
		views[string(id)] = participantView{
			LifeState:    LifeState(fact.Participation.State),
			DeathSaves:   projectDeathSaveProgress(fact.DeathSaves),
			attackTarget: fact.Participation.AttackTarget,
		}
		if kind == encounter.KindPlayer {
			playerFacts = append(playerFacts, fact.Participation)
		}
	}

	assessment.PartyDefeated = combat.PartyDefeated(combat.PartyState{Members: playerFacts})
	var dyingPlayer, consciousPlayer bool
	for _, participation := range playerFacts {
		dyingPlayer = dyingPlayer || participation.NeedsDeathSave
		consciousPlayer = consciousPlayer || participation.Conscious
	}
	assessment.KeepTurnOrder = dyingPlayer && consciousPlayer

	return &participationSnapshot{assessment: assessment, views: views}, nil
}

// neutralWorldNPCParticipation projects the explicit KindWorld invariant into
// the two session-owned views. It reads no content and chooses no combat rule:
// a placed WorldNPC is already declared non-combatant, so every combat
// participation capability is false and encounter removes any accidental turn
// slot while retaining the required one-answer-per-request shape.
func neutralWorldNPCParticipation(
	id encounter.MemberID,
) (encounter.MemberParticipation, participantView) {
	return encounter.MemberParticipation{
		Member: id,
		Turn:   encounter.TurnParticipationRemove,
	}, participantView{LifeState: LifeStateUnknown}
}

// storedConditionsOf indexes the condition blobs the records already carry, by
// member. Both shapes hold the same field for the same reason, and neither is
// read here: a blob is opaque bytes until somebody who owns the condition
// decodes it.
func storedConditionsOf(characters, monsters []resolution.Participant) map[string][]json.RawMessage {
	out := make(map[string][]json.RawMessage, len(characters)+len(monsters))
	for _, participant := range characters {
		if participant.Character != nil {
			out[participant.Character.ID] = participant.Character.Conditions
		}
	}
	for _, participant := range monsters {
		if participant.Monster != nil {
			out[participant.Monster.ID] = participant.Monster.Conditions
		}
	}
	return out
}

// encounterParticipation maps the rulebook's participation fact onto the
// encounter's scheduling word, and adds the one thing the rulebook's fact
// cannot say: that somebody else is taking this turn.
//
// # Driven is a narrowing of Wait, and only of Wait
//
// A member who would have been waited for, and holds a compulsion, is Driven:
// the slot is kept and the turn is taken by the TurnDriver whoever the member
// is. AutoPass and Remove both OUTRANK it, and that ordering is the ruling
// rather than an implementation detail — a dying commanded fighter still dies
// on schedule, and a member the fight has removed is not brought back to be
// marched around. The design says so in §5.1; the switch below is where it is
// true.
//
// WHAT THIS FUNCTION DOES NOT DO is decide what the compulsion means. It asks
// the conditions package whether the sheet holds one ref, which is a lookup;
// the word inside it, the anchor it measures from and the walk it produces all
// belong to the layers that own rules.
//
// # A blob nobody can read is not a compulsion
//
// The plan asked for an error here, reasoning that answering Wait could hand a
// human a turn somebody else was taking. That reasoning does not survive
// contact with what the rest of this module already decided, twice: the
// character loader DROPS a condition it cannot parse and logs a warning
// (TestACorruptConditionIsDroppedRatherThanRejected), and a member whose sheet
// cannot be read refuses that member's OFFERS with ShortfallUnreadable rather
// than the whole read
// (TestUnreadableTargetAndParticipantBlockAffordBeforeUnchangedAttack).
//
// So a blob this build cannot read is already not on the sheet: it is never
// attached, never runs, and compels nobody. Answering Wait agrees with the
// sheet the fight is actually using. Erroring would make this function
// STRICTER than the record it is describing, which is the disagreement between
// two readers of one record that the snapshot above exists to prevent — and it
// would stop a whole fight over a byte the loader had already thrown away.
func encounterParticipation(
	id encounter.MemberID, participation combat.Participation, stored []json.RawMessage,
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
	if member.Turn == encounter.TurnParticipationWait && holdsCompulsion(stored) {
		member.Turn = encounter.TurnParticipationDriven
	}
	if member.Turn == encounter.TurnParticipationRemove && member.Contact {
		return encounter.MemberParticipation{}, fmt.Errorf(
			"participation: member %q cannot be removed while remaining in contact: %w",
			id, ErrInvalidSession)
	}
	return member, nil
}

// holdsCompulsion reports whether any blob on the sheet is a Commanded, asking
// the conditions package one blob at a time.
//
// ONE AT A TIME IS THE WHOLE POINT, and it is not a style choice. HoldsRef
// stops at the first blob whose ref it cannot read and answers with an error
// for the whole list, so a corrupt entry sitting ahead of a real compulsion
// would hide it — and the member it hid would be handed back a turn somebody
// else was taking. Asked per blob, an unreadable one costs exactly itself.
//
// Which is also the character loader's own rule: drop what cannot be parsed,
// keep what can, and warn. See [encounterParticipation] for why this seam must
// not be stricter about a record than the loader that produced it.
func holdsCompulsion(stored []json.RawMessage) bool {
	commanded := refs.Conditions.Commanded()
	for _, raw := range stored {
		if held, err := conditions.HoldsRef([]json.RawMessage{raw}, commanded); err == nil && held {
			return true
		}
	}
	return false
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
