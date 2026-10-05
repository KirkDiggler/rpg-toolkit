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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// frame.go is where resolution builds the frame a rule answers from, once per
// action and target (rpg-project#520, R2). Two builders, one shared set of
// action facts: the INFORMATION frame is what the acting character knows —
// its own assembled attack and its current sightings — and the EXECUTION
// frame is authoritative state at the moment of the swing. A rule cannot tell
// which one it was handed, which is the point: the tooltip and the swing ask
// the same rule function, and only the facts differ.

// attackActionFacts is the one derivation of an attack's action facts, shared
// by the information frame, the attack-roll frame and the post-fold frame so
// the tooltip and the swing cannot read the assembled profile two ways.
//
// Every fact comes from the ASSEMBLED profile — the ability and dice the
// swing will use were settled before any rule is asked (R10), so a monk's
// unarmed strike already names Dexterity here. Ability is Known("") when the
// profile declares none, which is a stat block's honest answer. WeaponPool
// matches what a damage-chain rule finds as the primary weapon pool: a weapon
// attack with exactly one pool that adds the attack ability modifier.
// Advantage is left unknown: it is a fold result, not an assembly fact, and
// only the post-fold frame has a fold to read it from.
//
// The weapon facts read the profile's weapon context, and what was not read
// stays unknown:
//   - No weapon context — a spell attack, or a stat-block attack that names no
//     weapon — honestly has no weapon: Weapon and WeaponSlot are Known(""),
//     Finesse, RangedWeapon and TwoHanded known false. Whether the other hand
//     holds a weapon was never read, so OffHandWeapon is unknown.
//   - A weapon context whose producer names no hand (Slot "", every monster
//     today: weaponattack never reads a second hand or asks a grip) leaves
//     WeaponSlot, TwoHanded and OffHandWeapon unknown.
//   - A weapon context whose ref is missing, or names a weapon the catalogue
//     does not hold, leaves the weapon's catalogue properties unknown: an
//     unreadable weapon is never read as a plain one.
//
// No rule that ships reads an unknown here for those attacks: Dueling and Great
// Weapon Fighting refuse a spell attack on its weapon pool first, and no
// monster holds a grip-reading rule. Opportunity is the caller's: the strike
// knows whether it is an opportunity attack, and the profile does not.
func attackActionFacts(p *combatActions.AttackProfile, opportunity bool) contributions.ActionFacts {
	var ability abilities.Ability
	modifier := 0
	if p.Ability != nil {
		ability = p.Ability.Ability
		modifier = p.Ability.Modifier
	}
	primaryPools := 0
	for i := range p.Damage {
		if p.Damage[i].HasProperty(damage.AddsAttackAbilityModifier) {
			primaryPools++
		}
	}

	facts := contributions.ActionFacts{
		Roll:            contributions.Known(contributions.RollKindAttack),
		Ability:         contributions.Known(ability),
		Melee:           contributions.Known(p.Delivery.IsMelee()),
		WeaponPool:      contributions.Known(p.Category == combatActions.AttackCategoryWeapon && primaryPools == 1),
		Advantage:       contributions.Unknown[bool](),
		AbilityModifier: contributions.Known(modifier),
		Weapon:          contributions.Known(""),
		WeaponSlot:      contributions.Known(""),
		Finesse:         contributions.Known(false),
		RangedWeapon:    contributions.Known(false),
		TwoHanded:       contributions.Known(false),
		OffHandWeapon:   contributions.Unknown[bool](),
		OffHandAttack:   contributions.Known(p.IsOffHandAttack),
		Opportunity:     contributions.Known(opportunity),
	}
	if p.Weapon == nil {
		return facts
	}
	if p.Weapon.Slot == "" {
		facts.WeaponSlot = contributions.Unknown[string]()
		facts.TwoHanded = contributions.Unknown[bool]()
	} else {
		facts.WeaponSlot = contributions.Known(p.Weapon.Slot)
		facts.TwoHanded = contributions.Known(p.Weapon.TwoHanded)
		facts.OffHandWeapon = contributions.Known(p.Weapon.OffHandWeaponRef != nil)
	}
	facts.Weapon = contributions.Unknown[string]()
	facts.Finesse = contributions.Unknown[bool]()
	facts.RangedWeapon = contributions.Unknown[bool]()
	if p.Weapon.Ref == nil {
		return facts
	}
	facts.Weapon = contributions.Known(p.Weapon.Ref.String())
	if weapon, err := weapons.GetByID(weapons.WeaponID(p.Weapon.Ref.ID)); err == nil {
		facts.Finesse = contributions.Known(weapon.HasProperty(weapons.PropertyFinesse))
		facts.RangedWeapon = contributions.Known(weapon.IsRanged())
	}
	return facts
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
// information does not fold. Opportunity is known false: information answers
// for an attack the actor declares, and an opportunity attack is a reaction
// that is never declared — rows outside the actor's own turn are not
// delivered (R12).
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
		Action: attackActionFacts(in.Attack, false),
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

// attackRollFrame is the strike's attack-roll frame, built from authoritative
// state ONCE per machine, before the attack chain folds, and handed to every
// rule the attack chain asks. Its Advantage is unknown: the fold that settles
// it has not run.
//
// Pairs range over every cast member the installed room places — the
// participants are this interaction's declared universe (R3) — measured with
// the room's own grid, the metric [encounter.Encounter.Distance] uses. The
// stance is the installed cast's authoritative [gamectx.Cast.StanceBetween];
// two placed members with no stance are a KNOWN no side
// ([contributions.StanceNone]), never unknown. Complete is true because the
// pairs cover every placed participant. Opportunity is the strike input's own.
//
// A resumed machine builds it afresh from current truth (S3); rows a client
// saw are never consulted.
//
// Errors: no installed room or cast ([ErrBadWorld]), or a frame that fails
// [contributions.Frame.Validate]. The caller gets a detached copy.
func (m *strikeMachine) attackRollFrame(ctx context.Context) (contributions.Frame, error) {
	if m.rollFrame != nil {
		return m.rollFrame.Clone(), nil
	}
	room, err := gamectx.RequireRoom(ctx)
	if err != nil {
		return contributions.Frame{}, fmt.Errorf("%w: attack frame: %w", ErrBadWorld, err)
	}
	cast, ok := gamectx.CastOf(ctx)
	if !ok {
		return contributions.Frame{}, fmt.Errorf("%w: attack frame: no cast installed", ErrBadWorld)
	}

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
		Action:   attackActionFacts(m.attack, m.in.Opportunity),
		Pairs:    pairs,
		Complete: true,
	}
	if err := frame.Validate(); err != nil {
		return contributions.Frame{}, fmt.Errorf("attack frame: %w", err)
	}
	m.rollFrame = &frame

	return frame.Clone(), nil
}

// executionFrame is the strike's post-fold frame: the attack-roll frame with
// Advantage settled to the fold's own effective answer — granted and not
// imposed, the same reading the d20 was rolled under. It only ADDS that
// knowledge; every other fact is the attack-roll frame's. The post-roll offers
// and the damage fold read it (O5). Built once, on first use after the attack
// chain has folded.
//
// That guarantee holds within one uninterrupted strike. A strike resumed after
// a freeze rebuilds its attack-roll frame from current state (S3), so its
// distances, stances and holdings may differ from the frame the attack chain
// folded under before the freeze; only the frozen fold's Advantage carries
// over.
//
// Errors: any error from [strikeMachine.attackRollFrame]. The caller gets a
// detached copy.
func (m *strikeMachine) executionFrame(ctx context.Context) (contributions.Frame, error) {
	if m.frame != nil {
		return m.frame.Clone(), nil
	}
	frame, err := m.attackRollFrame(ctx)
	if err != nil {
		return contributions.Frame{}, err
	}
	folded := m.outcome.Folded
	frame.Action.Advantage = contributions.Known(len(folded.AdvantageSources) > 0 && len(folded.DisadvantageSources) == 0)
	m.frame = &frame

	return frame.Clone(), nil
}
