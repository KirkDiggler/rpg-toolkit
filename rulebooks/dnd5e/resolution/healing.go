// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type preparedHealing struct {
	declaration healing.Declaration
	targetID    string
	source      dnd5eEvents.RollSource
	excludes    []string
	roller      dice.Roller
}

func (h *preparedHealing) prepare(cast *Participants) (string, error) {
	var state combat.LifeState
	creatureType := "humanoid"
	if ch, ok := cast.Character(h.targetID); ok {
		state = ch.ParticipationView().LifeState
	} else if monster, ok := cast.Monster(h.targetID); ok {
		state = combat.ClassifyLifeState(combat.LifeStateInput{Kind: combat.CombatantKindMonster, Down: combat.IsDown(monster)})
		creatureType = monster.CreatureType()
	} else {
		return "", fmt.Errorf("%w: no healing recipient %q", ErrBadAction, h.targetID)
	}
	if !combat.CanReceiveHealing(state) {
		return "", fmt.Errorf("%w: target cannot receive healing", ErrBadAction)
	}
	if len(h.excludes) > 0 && creatureType == "" {
		return "", fmt.Errorf("%w: healing recipient has no creature type", ErrBadAction)
	}
	if slices.Contains(h.excludes, creatureType) {
		return "No effect on " + creatureType, nil
	}
	return "", nil
}

func (h *preparedHealing) deliver(ctx context.Context, bus events.EventBus, noEffect string) error {
	var calculation *dnd5eEvents.RollCalculation
	if noEffect != "" {
		zero := 0
		source := h.source
		source.Label = noEffect
		calculation = &dnd5eEvents.RollCalculation{Components: []dnd5eEvents.RollComponent{{Source: source, Modifier: &zero}}}
	} else {
		var err error
		calculation, err = healing.Resolve(ctx, h.declaration, h.source, h.roller)
		if err != nil {
			return fmt.Errorf("resolve healing: %w", err)
		}
	}
	ref := *h.source.Ref
	return dnd5eEvents.HealingReceivedTopic.On(bus).Publish(ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: h.targetID, Amount: calculation.Total, Calculation: calculation,
		Source: ref.ID, SourceRef: &ref, SourceName: h.source.Name,
	})
}

// TouchReach answers physical reach using the encounter's live positional facts.
// It does not test illumination or a sighting. Blocking boundaries obstruct
// touch even when transparent; other occupants are not movement destinations.
func TouchReach(room spatial.Room, casterID, targetID string) (bool, error) {
	if room == nil {
		return false, fmt.Errorf("%w: touch requires a room", ErrBadWorld)
	}
	from, ok := room.GetEntityPosition(casterID)
	if !ok {
		return false, fmt.Errorf("%w: caster has no position", ErrBadWorld)
	}
	to, ok := room.GetEntityPosition(targetID)
	if !ok {
		return false, fmt.Errorf("%w: target has no position", ErrBadWorld)
	}
	if room.GetGrid().Distance(from, to) > 1 {
		return false, nil
	}
	if from == to {
		return true, nil
	}
	boundaries, ok := room.(interface {
		GetBoundary(spatial.Position, spatial.Position) (spatial.Boundary, bool)
	})
	if !ok {
		return false, fmt.Errorf("%w: touch requires boundary facts", ErrBadWorld)
	}
	if boundary, exists := boundaries.GetBoundary(from, to); exists && (boundary.BlocksMovement || boundary.BlocksLineOfSight) {
		return false, nil
	}
	return !room.IsLineOfSightBlocked(from, to), nil
}

func validateTouchTarget(ctx context.Context, casterID, targetID string) error {
	room, err := gamectx.RequireRoom(ctx)
	if err != nil {
		return err
	}
	reachable, err := TouchReach(room, casterID, targetID)
	if err != nil {
		return err
	}
	if !reachable {
		return fmt.Errorf("%w: target is not within touch", ErrOutOfRange)
	}
	return nil
}

// HealingTargets projects living-recipient and touch answers for known candidate
// identities. Candidates need not currently be seen; discovery remains with the
// caller's knowledge store. No payment, dice or HP mutation occurs here.
func HealingTargets(ctx context.Context, input *HealingTargetsInput) (map[string]bool, error) {
	if input == nil {
		return nil, ErrNilInput
	}
	view, err := Participation(ctx, &ParticipationInput{Participants: input.Participants})
	if err != nil {
		return nil, err
	}
	states := make(map[string]combat.LifeState, len(view.Members))
	for _, member := range view.Members {
		states[member.Member] = member.Participation.State
	}
	if len(input.Excludes) > 0 {
		for _, participant := range input.Participants {
			if participant.Monster == nil {
				continue
			}
			sheet, err := monster.Load(ctx, participant.Monster)
			if err != nil {
				return nil, err
			}
			if sheet.CreatureType() == "" {
				delete(states, participant.ID())
			}
		}
	}
	out := map[string]bool{}
	for _, id := range input.Candidates {
		if !combat.CanReceiveHealing(states[id]) {
			continue
		}
		reach, err := TouchReach(input.Room, input.CasterID, id)
		if err != nil {
			return nil, err
		}
		out[id] = reach
	}
	return out, nil
}

// HealingTargetsInput supplies encounter facts, known identities and the
// content's type exclusions. An unknown family cannot satisfy those exclusions.
type HealingTargetsInput struct {
	Room         spatial.Room
	CasterID     string
	Candidates   []string
	Participants []Participant
	Excludes     []string
}
