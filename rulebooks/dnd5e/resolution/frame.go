// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// frame.go is where resolution builds the frame a rule answers from, once per
// action and target (rpg-project#520, R2). Two builders, one shared set of
// action facts: the INFORMATION frame is what the acting character knows —
// its own assembled attack and its current sightings — and the EXECUTION
// frame is authoritative state at the moment of the swing. A rule cannot tell
// which one it was handed, which is the point: the tooltip and the swing ask
// the same rule function, and only the facts differ.

// attackActionFacts is the one derivation of an attack's action facts, shared
// by both frames so the tooltip and the swing cannot read the assembled
// profile two ways.
//
// Every fact comes from the ASSEMBLED profile — the ability and dice the
// swing will use were settled before any rule is asked (R10), so a monk's
// unarmed strike already names Dexterity here. Ability is Known("") when the
// profile declares none, which is a stat block's honest answer. WeaponPool
// matches what a damage-chain rule finds as the primary weapon pool: a weapon
// attack with exactly one pool that adds the attack ability modifier.
// Advantage is left unknown: it is a fold result, not an assembly fact, and
// only the execution frame has a fold to read it from.
func attackActionFacts(p *combatActions.AttackProfile) contributions.ActionFacts {
	var ability abilities.Ability
	if p.Ability != nil {
		ability = p.Ability.Ability
	}
	primaryPools := 0
	for i := range p.Damage {
		if p.Damage[i].HasProperty(damage.AddsAttackAbilityModifier) {
			primaryPools++
		}
	}

	return contributions.ActionFacts{
		Roll:       contributions.Known(contributions.RollKindAttack),
		Ability:    contributions.Known(ability),
		Melee:      contributions.Known(p.Delivery.IsMelee()),
		WeaponPool: contributions.Known(p.Category == combatActions.AttackCategoryWeapon && primaryPools == 1),
		Advantage:  contributions.Unknown[bool](),
	}
}

// informationFrameInput is what an information frame is built from: the
// observer's own detached knowledge, the assembled attack, and the target
// being asked about. Target is empty for the ask about no target in
// particular.
type informationFrameInput struct {
	Observed *encounter.ObservedContextOutput
	Attack   *combatActions.AttackProfile
	Target   string
}

// informationFrameOutput carries the validated information frame.
type informationFrameOutput struct {
	Frame contributions.Frame
}

// informationFrame builds the frame from what the acting character knows and
// nothing else (K1): its observed context and its own assembled attack.
//
// A pair's distance is known because the observer measured it. A nil observed
// stance is UNKNOWN — the observer holds no belief about that pair — never
// [contributions.StanceNone], which is a known fact this read cannot prove.
// Complete is false: a set of sightings never proves no unseen creature
// exists (K5). Advantage stays unknown, because advantage is a fold result and
// information does not fold.
//
// Errors: a nil observed context or attack, or a frame that fails
// [contributions.Frame.Validate].
func informationFrame(in *informationFrameInput) (*informationFrameOutput, error) {
	if in == nil || in.Observed == nil || in.Attack == nil {
		return nil, fmt.Errorf("%w: an information frame needs an observed context and an attack", ErrNilInput)
	}
	frame := contributions.Frame{
		Actor:  string(in.Observed.Observer),
		Target: contributions.Unknown[string](),
		Action: attackActionFacts(in.Attack),
		Pairs:  make([]contributions.PairFacts, 0, len(in.Observed.Pairs)),
	}
	if in.Target != "" {
		frame.Target = contributions.Known(in.Target)
	}
	for _, observed := range in.Observed.Pairs {
		pair := contributions.PairFacts{
			From:          string(observed.From),
			To:            string(observed.To),
			DistanceCells: contributions.Known(observed.DistanceCells),
			Stance:        contributions.Unknown[contributions.Stance](),
		}
		if observed.Stance != nil {
			pair.Stance = contributions.Known(contributions.Stance(*observed.Stance))
		}
		frame.Pairs = append(frame.Pairs, pair)
	}
	if err := frame.Validate(); err != nil {
		return nil, fmt.Errorf("information frame: %w", err)
	}

	return &informationFrameOutput{Frame: frame}, nil
}

// executionFrame is the strike's execution frame, built from authoritative
// state ONCE per machine and handed to every rule the strike asks: the
// post-roll offers and the damage fold read the same frame (O5).
//
// Built on first use, after the attack chain has folded, so Advantage is the
// fold's own effective answer: granted and not imposed, the same reading the
// d20 was rolled under. A resumed machine builds it afresh from current truth
// plus the frozen fold; rows a client saw are never consulted (S3).
//
// Pairs range over every cast member the installed room places — the
// participants are this interaction's declared universe (R3) — measured with
// the room's own grid, the metric [encounter.Encounter.Distance] uses. The
// stance is the installed cast's authoritative [gamectx.Cast.StanceBetween];
// two placed members with no stance are a KNOWN no side
// ([contributions.StanceNone]), never unknown. Complete is true because the
// pairs cover every placed participant.
//
// Errors: no installed room or cast ([ErrBadWorld]), or a frame that fails
// [contributions.Frame.Validate]. The caller gets a detached copy.
func (m *strikeMachine) executionFrame(ctx context.Context) (contributions.Frame, error) {
	if m.frame != nil {
		return m.frame.Clone(), nil
	}
	room, err := gamectx.RequireRoom(ctx)
	if err != nil {
		return contributions.Frame{}, fmt.Errorf("%w: execution frame: %w", ErrBadWorld, err)
	}
	cast, ok := gamectx.CastOf(ctx)
	if !ok {
		return contributions.Frame{}, fmt.Errorf("%w: execution frame: no cast installed", ErrBadWorld)
	}

	action := attackActionFacts(m.attack)
	folded := m.outcome.Folded
	action.Advantage = contributions.Known(len(folded.AdvantageSources) > 0 && len(folded.DisadvantageSources) == 0)

	members := cast.Members()
	placed := make([]string, 0, len(members))
	for _, id := range members {
		if _, ok := room.GetEntityPosition(id); ok {
			placed = append(placed, id)
		}
	}
	pairs := make([]contributions.PairFacts, 0, len(placed)*(len(placed)-1))
	for _, from := range placed {
		fromAt, _ := room.GetEntityPosition(from)
		for _, to := range placed {
			if from == to {
				continue
			}
			toAt, _ := room.GetEntityPosition(to)
			stance, exists := cast.StanceBetween(from, to)
			if !exists {
				stance = contributions.StanceNone
			}
			pairs = append(pairs, contributions.PairFacts{
				From:          from,
				To:            to,
				DistanceCells: contributions.Known(room.GetGrid().Distance(fromAt, toAt)),
				Stance:        contributions.Known(stance),
			})
		}
	}

	frame := contributions.Frame{
		Actor:    m.in.AttackerID,
		Target:   contributions.Known(m.in.TargetID),
		Action:   action,
		Pairs:    pairs,
		Complete: true,
	}
	if err := frame.Validate(); err != nil {
		return contributions.Frame{}, fmt.Errorf("execution frame: %w", err)
	}
	m.frame = &frame

	return frame.Clone(), nil
}
