// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
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
// observer's own detached knowledge, the assembled attack, the target being
// asked about, and what the observer holds by its own sheet. Target is empty
// for the ask about no target in particular. ActorHeld is the observer's own
// holdings, always known: nil is read as holding none. ActorClassLevels is the
// observer's class levels from its own sheet ([sheetClassLevels]); it must be
// known, because a member always knows its own sheet.
type informationFrameInput struct {
	Observed         *encounter.ObservedContextOutput
	Attack           *combatActions.AttackProfile
	Target           string
	ActorHeld        []contributions.HeldCondition
	ActorClassLevels contributions.ClassLevels
}

// informationFrameOutput carries the validated information frame.
type informationFrameOutput struct {
	Frame contributions.Frame
}

// informationFrame builds the frame from what the acting character knows and
// nothing else (K1): its observed context and its own assembled attack.
//
// A pair's distance is known because the observer measured it, and its stance
// is known because every observed pair names one: a member in no faction is
// the known no side ([contributions.StanceNone]), exactly as the execution
// frame reads it (R5, rpg-toolkit#1958).
// Complete is false: a set of sightings never proves no unseen creature
// exists (K5). Advantage stays unknown, because advantage is a fold result and
// information does not fold. Opportunity is known false: information answers
// for an attack the actor declares, and an opportunity attack is a reaction
// that is never declared — rows outside the actor's own turn are not
// delivered (R12).
//
// What members hold is testimony, never a live read (R16, K3). The observer is
// listed first with its own sheet's holdings. Each sighted member whose
// sighting observed conditions follows, in sighting order, listed exactly as
// it was seen — an empty set is a known "holds nothing". A sighting that
// observed no conditions (nil) leaves that member out of Held, which is
// UNKNOWN, never "holds nothing". A member who is not sighted is unknown too.
//
// Sight is known only where the observer knows it: each observer→member pair
// is Known(true), because every member is a current SIGHT sighting by
// [encounter.ObservedContextOutput]'s contract. Every other direction — what
// a member sees, including whether it sees the observer — is unknown, because
// the observer's sightings say nothing about another creature's eyes.
//
// The observer's class levels are its own sheet's (rpg-project#538), the same
// answer [strikeMachine.attackRollFrame] reads from the cast, so a
// class-scaled row and the swing it describes compute from one level.
//
// Errors: a nil observed context or attack, unknown actor class levels, or a
// frame that fails [contributions.Frame.Validate].
func informationFrame(in *informationFrameInput) (*informationFrameOutput, error) {
	if in == nil || in.Observed == nil || in.Attack == nil {
		return nil, fmt.Errorf("%w: an information frame needs an observed context and an attack", ErrNilInput)
	}
	observer := string(in.Observed.Observer)
	if _, known := in.ActorClassLevels.Get(); !known {
		return nil, fmt.Errorf("information frame: %q's class levels are unknown; its own sheet answers them", observer)
	}
	frame := contributions.Frame{
		Actor:            observer,
		ActorClassLevels: in.ActorClassLevels,
		Target:           contributions.Unknown[string](),
		Action:           attackActionFacts(in.Attack, false),
		Pairs:            make([]contributions.PairFacts, 0, len(in.Observed.Pairs)),
		Held:             make([]contributions.MemberHeld, 0, len(in.Observed.Members)+1),
	}
	if in.Target != "" {
		frame.Target = contributions.Known(in.Target)
	}
	frame.Held = append(frame.Held, contributions.MemberHeld{
		Member: observer, Conditions: append([]contributions.HeldCondition{}, in.ActorHeld...),
	})
	sighted := make(map[string]bool, len(in.Observed.Members))
	for _, member := range in.Observed.Members {
		sighted[string(member.ID)] = true
		if member.Conditions == nil {
			continue
		}
		held := make([]contributions.HeldCondition, 0, len(member.Conditions.Conditions))
		for _, seen := range member.Conditions.Conditions {
			held = append(held, contributions.HeldCondition{Ref: seen.ConditionRef, SourceID: seen.SourceID})
		}
		frame.Held = append(frame.Held, contributions.MemberHeld{Member: string(member.ID), Conditions: held})
	}
	for _, observed := range in.Observed.Pairs {
		pair := contributions.PairFacts{
			From:          string(observed.From),
			To:            string(observed.To),
			DistanceCells: contributions.Known(observed.DistanceCells),
			Stance:        contributions.Known(contributions.Stance(observed.Stance)),
			Sees:          contributions.Unknown[bool](),
		}
		if pair.From == observer && sighted[pair.To] {
			pair.Sees = contributions.Known(true)
		}
		frame.Pairs = append(frame.Pairs, pair)
	}
	if err := frame.Validate(); err != nil {
		return nil, fmt.Errorf("information frame: %w", err)
	}

	return &informationFrameOutput{Frame: frame}, nil
}

// sideAnswerer is a cast that can say whether anyone answered its stance
// questions. [castView] is one: with no run loaded there is no disposition
// graph, so its silence about a pair proves nothing.
type sideAnswerer interface {
	answersSides() bool
}

// authoritativeStance is the execution answer for the stance from one member
// toward another: the installed cast's [gamectx.Cast.StanceBetween], with no
// stance between two members of the cast read as the known no side
// ([contributions.StanceNone], R5). Membership is proven from the cast's own
// Members, as StanceBetween's contract requires; a pair naming anyone the cast
// does not hold is UNKNOWN, never no side. So is every pair when the cast has
// no graph to ask ([sideAnswerer]), or cannot say whether it has one: silence
// from nobody is not "no side". Every execution read of a stance goes through
// here, so the attack frame and the cast's ward gate cannot disagree about one
// pair.
func authoritativeStance(cast gamectx.Cast, from, to string) contributions.Fact[contributions.Stance] {
	if stance, ok := cast.StanceBetween(from, to); ok {
		return contributions.Known(stance)
	}
	if answerer, ok := cast.(sideAnswerer); !ok || !answerer.answersSides() {
		return contributions.Unknown[contributions.Stance]()
	}
	members := cast.Members()
	if slices.Contains(members, from) && slices.Contains(members, to) {
		return contributions.Known(contributions.StanceNone)
	}
	return contributions.Unknown[contributions.Stance]()
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
// Sight is the installed visibility's live answer for each placed ordered
// pair, both directions, uncapped by range — the attack owns its range: known
// and visible is Known(true), known and not visible Known(false), and a pair
// visibility cannot answer stays unknown, never false. The strike's sight
// rule ([applySightAttackModifiers]) reads attacker→target and target→attacker
// from here and asks visibility nothing itself. Held lists every participant with
// a sheet, in the cast's order, each with its persisted conditions at their
// own addresses ([conditions.ConditionAddressOf]); a participant with
// nothing on its sheet is listed holding nothing, because the sheet is the
// authority. The first caller is the Sanctuary step, after the attacker's own
// ward has ended, so the frame the ward check, the attack chain and the
// damage fold read is one frame. ActorClassLevels is the attacker's own
// sheet's answer ([sheetClassLevels]), asked as the frame is built and never
// stored on an effect (rpg-project#538).
//
// A resumed machine builds it afresh from current truth (S3); rows a client
// saw are never consulted.
//
// Errors: no installed room, cast or visibility ([ErrBadWorld]), an attacker
// with no sheet in the cast ([ErrNoCombatant]), or a frame that fails
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
	sight, ok := gamectx.Visibility(ctx)
	if !ok {
		return contributions.Frame{}, fmt.Errorf("%w: attack frame: no visibility installed", ErrBadWorld)
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
			sees := contributions.Unknown[bool]()
			if visible, known := sight.SeesWithin(from, to, math.MaxInt); known {
				sees = contributions.Known(visible)
			}
			pairs = append(pairs, contributions.PairFacts{
				From:          from,
				To:            to,
				DistanceCells: contributions.Known(room.GetGrid().Distance(fromAt, toAt)),
				Stance:        authoritativeStance(cast, from, to),
				Sees:          sees,
			})
		}
	}

	held, err := castHeld(m.cast, members)
	if err != nil {
		return contributions.Frame{}, fmt.Errorf("%w: attack frame: %w", ErrBadWorld, err)
	}
	levels, err := sheetClassLevels(m.cast, m.in.AttackerID)
	if err != nil {
		return contributions.Frame{}, fmt.Errorf("attack frame: %w", err)
	}
	frame := contributions.Frame{
		Actor:            m.in.AttackerID,
		ActorClassLevels: levels,
		Target:           contributions.Known(m.in.TargetID),
		Action:           attackActionFacts(m.attack, m.in.Opportunity),
		Pairs:            pairs,
		Complete:         true,
		Held:             held,
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

// sheetClassLevels is a member's class levels as its own sheet answers them
// (rpg-project#538): a character's from its level record, a monster's known
// and empty. Every frame that names an actor with a sheet in the cast fills
// [contributions.Frame.ActorClassLevels] through here, so the information
// frame and the execution frames cannot read two different levels for one
// member. The levels are asked each time a frame is built; nothing below the
// sheet keeps a copy.
//
// Errors: a member with no sheet in the cast ([ErrNoCombatant]).
func sheetClassLevels(cast *Participants, id string) (contributions.ClassLevels, error) {
	if character, ok := cast.Character(id); ok {
		return character.ClassLevels(), nil
	}
	if monster, ok := cast.Monster(id); ok {
		return monster.ClassLevels(), nil
	}
	return contributions.UnknownClassLevels(), fmt.Errorf("%w: class levels of %q", ErrNoCombatant, id)
}

// castHeld lists what each member with a sheet holds, in the given order, each
// condition at its own address — the same address its handler asks its held
// rule about. A member with no sheet in the cast is left out, which a frame
// reads as unknown.
//
// Each sheet is read through [sheetHeld], the one derivation every frame
// uses, over the sheet's CURRENT record: a ward the strike has just ended is
// gone, and the free reactions every combatant carries are there although
// they are never stored.
//
// Errors: a sheet whose record does not read ([sheetHeld]) — authoritative
// state that cannot be read fails the strike rather than framing the member
// as holding less.
func castHeld(cast *Participants, members []string) ([]contributions.MemberHeld, error) {
	held := make([]contributions.MemberHeld, 0, len(members))
	for _, id := range members {
		var stored []json.RawMessage
		if character, ok := cast.Character(id); ok {
			data, err := character.ToData()
			if err != nil {
				return nil, fmt.Errorf("frame: %q: %w", id, err)
			}
			stored = data.Conditions
		} else if monster, ok := cast.Monster(id); ok {
			stored = monster.ToData().Conditions
		} else {
			continue
		}
		conditions, err := sheetHeld(id, stored)
		if err != nil {
			return nil, err
		}
		held = append(held, contributions.MemberHeld{Member: id, Conditions: conditions})
	}
	return held, nil
}

// sheetHeld is what a member's record says it holds, as a frame carries it:
// [conditions.HeldAddresses] — the stored conditions at their own addresses,
// in stored order, plus the free reactions every combatant carries from
// attach. It is the rulebook's own reader, so the information frame's actor
// entry, the execution frames and a seam answering sightings agree on what a
// member holds.
//
// Errors: a stored condition that does not load.
func sheetHeld(member string, stored []json.RawMessage) ([]contributions.HeldCondition, error) {
	addresses, err := conditions.HeldAddresses(member, stored)
	if err != nil {
		return nil, fmt.Errorf("what %q holds: %w", member, err)
	}
	held := make([]contributions.HeldCondition, 0, len(addresses))
	for _, address := range addresses {
		held = append(held, contributions.HeldCondition{Ref: address.ConditionRef, SourceID: address.SourceID})
	}
	return held, nil
}
