// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// StaleTargetPolicy decides the consequence of aiming at a remembered location
// that no longer contains the named creature. This is a host policy, not a
// spell option or a replacement for the observer's testimony. There is no default.
type StaleTargetPolicy string

const (
	// StaleTargetRefuse refuses the entire cast before payment or randomness.
	StaleTargetRefuse StaleTargetPolicy = "refuse"
	// StaleTargetAttempt pays once and misses only the displaced recipients.
	StaleTargetAttempt StaleTargetPolicy = "attempt"
)

func (p StaleTargetPolicy) validate() error {
	if p != StaleTargetRefuse && p != StaleTargetAttempt {
		return fmt.Errorf("%w: known-creature casting requires an explicit stale-target policy", ErrBadAction)
	}
	return nil
}

// KnownCreatureTargetsInput supplies an explicit candidate universe and the
// same policy used for execution. Where the caster believes each candidate
// stands is the encounter's answer ([encounter.Encounter.BelievedAim]); the
// candidate list alone does not establish a known location.
type KnownCreatureTargetsInput struct {
	Encounter         *encounter.Encounter
	CasterID          string
	Candidates        []string
	Participants      []Participant
	RangeFeet         int
	StaleTargetPolicy StaleTargetPolicy
}

// KnownCreatureTargets projects declaration eligibility without spending or
// rolling. Remembered locations remain usable without current sight. Under
// StaleTargetAttempt, availability never reveals whether a creature moved.
func KnownCreatureTargets(ctx context.Context, in *KnownCreatureTargetsInput) (map[string]bool, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if err := in.StaleTargetPolicy.validate(); err != nil {
		return nil, err
	}
	view, err := Participation(ctx, &ParticipationInput{Participants: in.Participants})
	if err != nil {
		return nil, err
	}
	states := make(map[string]combat.LifeState, len(view.Members))
	for _, member := range view.Members {
		states[member.Member] = member.Participation.State
	}
	out := make(map[string]bool, len(in.Candidates))
	for _, id := range in.Candidates {
		if !knownCreatureEligible(states[id]) {
			continue
		}
		available, _, err := knownCreatureReach(in.Encounter, in.CasterID, id, in.RangeFeet, in.StaleTargetPolicy)
		if err != nil {
			return nil, err
		}
		out[id] = available
	}
	return out, nil
}

func knownCreatureEligible(state combat.LifeState) bool {
	switch state {
	case combat.LifeStateConscious, combat.LifeStateDying, combat.LifeStateStabilized:
		return true
	default:
		return false
	}
}

func validateKnownCreatureTarget(ctx context.Context, cast *Participants, casterID, targetID string, rangeFeet int, policy StaleTargetPolicy) (bool, error) {
	var state combat.LifeState
	if ch, ok := cast.Character(targetID); ok {
		state = ch.ParticipationView().LifeState
	} else if monster, ok := cast.Monster(targetID); ok {
		state = combat.ClassifyLifeState(combat.LifeStateInput{Kind: combat.CombatantKindMonster, Down: combat.IsDown(monster)})
	}
	if !knownCreatureEligible(state) {
		return false, fmt.Errorf("%w: ineligible creature recipient", ErrBadAction)
	}
	view, ok := gamectx.CastOf(ctx)
	if !ok {
		return false, fmt.Errorf("%w: known-creature casting requires the encounter cast", ErrBadWorld)
	}
	live, ok := view.(*castView)
	if !ok {
		return false, fmt.Errorf("%w: known-creature casting requires the encounter view", ErrBadWorld)
	}
	available, missed, err := knownCreatureReach(live.run, casterID, targetID, rangeFeet, policy)
	if err != nil {
		return false, err
	}
	if !available {
		return false, fmt.Errorf("%w: known target is unavailable", ErrOutOfRange)
	}
	return missed, nil
}

// knownCreatureReach applies the stale-target policy to the encounter's
// answer about where the caster believes the target stands
// ([encounter.Encounter.BelievedAim]). Everything measured — the believed
// point, its range on a clear line, whether the target moved off it — is the
// encounter's; this decides only what the answer means for the cast. A target
// the caster holds no belief about, one known without a point, or a believed
// point out of range or behind a wall is unavailable. A displaced target is
// unavailable under [StaleTargetRefuse] and an attempt that misses (missed)
// under [StaleTargetAttempt]. Never aim using a hidden live position.
//
// Errors: an invalid policy ([ErrBadAction]); no encounter or a range that is
// not positive ([ErrBadWorld]); the encounter refusing the aim, wrapped in
// [ErrBadWorld].
func knownCreatureReach(run *encounter.Encounter, casterID, targetID string, rangeFeet int, policy StaleTargetPolicy) (available, missed bool, err error) {
	if err := policy.validate(); err != nil {
		return false, false, err
	}
	if run == nil || rangeFeet <= 0 {
		return false, false, fmt.Errorf("%w: known-creature casting requires an encounter and positive range", ErrBadWorld)
	}
	aim, err := run.BelievedAim(&encounter.BelievedAimInput{
		Observer:  encounter.MemberID(casterID),
		Subject:   encounter.MemberID(targetID),
		RangeFeet: rangeFeet,
	})
	if err != nil {
		return false, false, fmt.Errorf("%w: %w", ErrBadWorld, err)
	}
	if !aim.Held || aim.State != encounter.LocationKnown || !aim.InRange {
		return false, false, nil
	}
	return !aim.Displaced || policy == StaleTargetAttempt, aim.Displaced, nil
}
