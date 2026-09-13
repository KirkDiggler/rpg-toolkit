// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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
// same policy used for execution. Knowledge comes from Encounter.View; the
// candidate list alone does not establish a known location.
type KnownCreatureTargetsInput struct {
	Room              spatial.Room
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
		available, _, err := knownCreatureReach(in.Room, in.Encounter, in.CasterID, id, in.RangeFeet, in.StaleTargetPolicy)
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
	room, err := gamectx.RequireRoom(ctx)
	if err != nil {
		return false, err
	}
	view, ok := gamectx.CastOf(ctx)
	if !ok {
		return false, fmt.Errorf("%w: known-creature casting requires the encounter cast", ErrBadWorld)
	}
	live, ok := view.(*castView)
	if !ok {
		return false, fmt.Errorf("%w: known-creature casting requires the encounter view", ErrBadWorld)
	}
	available, missed, err := knownCreatureReach(room, live.run, casterID, targetID, rangeFeet, policy)
	if err != nil {
		return false, err
	}
	if !available {
		return false, fmt.Errorf("%w: known target is unavailable", ErrOutOfRange)
	}
	return missed, nil
}

// Encounter supplies position and testimony facts; resolution interprets them
// for this pointed delivery. Never aim using a hidden live position.
func knownCreatureReach(room spatial.Room, run *encounter.Encounter, casterID, targetID string, rangeFeet int, policy StaleTargetPolicy) (available, missed bool, err error) {
	if err := policy.validate(); err != nil {
		return false, false, err
	}
	if room == nil || run == nil || rangeFeet <= 0 {
		return false, false, fmt.Errorf("%w: known-creature casting requires room, encounter and positive range", ErrBadWorld)
	}
	from, ok := room.GetEntityPosition(casterID)
	if !ok {
		return false, false, fmt.Errorf("%w: caster has no position", ErrBadWorld)
	}
	if casterID == targetID {
		return true, false, nil
	}
	holdings, err := run.View(&encounter.ViewInput{Member: encounter.MemberID(casterID)})
	if err != nil {
		return false, false, err
	}
	for _, holding := range holdings {
		if string(holding.Subject) != targetID {
			continue
		}
		testimony, valid := encounter.DecodeSightTestimony(holding.Payload)
		if !valid {
			return false, false, fmt.Errorf("%w: invalid location testimony", ErrBadWorld)
		}
		if testimony.State != encounter.LocationKnown {
			return false, false, nil
		}
		aim := testimony.Position
		if room.GetGrid().Distance(from, aim) > float64(encounter.CellsFromFeet(rangeFeet)) || room.IsLineOfSightBlocked(from, aim) {
			return false, false, nil
		}
		actual, placed := room.GetEntityPosition(targetID)
		if !placed {
			return false, false, fmt.Errorf("%w: target has no position", ErrBadWorld)
		}
		missed = actual != aim
		return !missed || policy == StaleTargetAttempt, missed, nil
	}
	return false, false, nil
}
