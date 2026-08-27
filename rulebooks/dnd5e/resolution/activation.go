// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
)

// ActivationInput names one member using something they already carry.
type ActivationInput struct {
	// MemberID is who is activating. Required, and must be a CHARACTER in the
	// cast: a monster's abilities are driven by its behaviour, not declared by
	// a player, and its economy belongs to whoever runs its turn rather than
	// to its sheet — the same line [ErrNoPayer] draws.
	MemberID string

	// Ability is which of the things they carry. Required. A ref rather than
	// an index or a name, because the sheet's own lookup is by ref and a
	// second addressing scheme would be a second place for the two to
	// disagree.
	Ability *core.Ref

	// TargetID is who it lands on, for the one ability of the seven that takes
	// somebody: Help. Empty for Dodge, Dash, Disengage, Hide, Rage and Second
	// Wind, and a populated ID for one of those is a caller defect rather than
	// a value quietly ignored.
	TargetID string

	// ObserverPassivePerceptions is the passive Perception of everyone who
	// could notice this, which Hide's Stealth check is rolled against.
	//
	// Carried rather than computed, for [ErrNoSight]'s reason: who can see
	// whom is the composition's question, and a package that answered it here
	// would be deciding a rule about light it cannot see. Empty for six of the
	// seven — and empty is a legitimate answer for Hide too (nobody is
	// watching), so this cannot be validated by presence.
	ObserverPassivePerceptions []int
}

// ActivationOutcome is what an activation produced.
//
// Deliberately thin, for [BoundaryOutcome]'s reason: an activation's real
// output is the DIRTY SHEETS that come back on [Output], because everything it
// does happens inside a condition being applied to its owner or an economy
// being spent. What is here is the part a caller cannot recover from a sheet.
type ActivationOutcome struct {
	// Ability is the ref that ran, echoed back. A caller that dispatched by
	// selector rather than by ref gets to learn what the selector meant
	// without parsing it.
	Ability string

	// GrantedCapacity is the ability's own description of what it banked —
	// "1 attack", "30ft movement" — or empty when it banked nothing. Display
	// text authored by the ability, never parsed: Dash's effect on the ledger
	// is already in the ledger.
	GrantedCapacity string
}

func (ActivationOutcome) isOutcome() {}

// NewActivation returns the machine that uses one thing a character carries.
//
// # It is a sibling of NewBoundary, not an arm of NewAction
//
// [NewAction] dispatches on a populated profile of an authored
// actions.Definition, and validates one before it does anything else. The
// abilities and features a character carries have no such definition and do
// not need one: each already declares its own ActionType and enforces its own
// resources, which is the price the door actually charges. Minting a
// Definition for Dodge purely so a dispatcher recognised it would be a
// compiled price nothing charges — the exact fiction Afford's "read the price
// off the same profile" rule exists to prevent (rpg-project#300 §4).
//
// What it shares with [NewBoundary] is the shape that matters: attach
// everyone, do ONE thing on the interaction's own bus, collect dirty sheets.
//
// # Why the bus is the whole point
//
// A feature does not attach its own condition. Rage builds a RagingCondition
// and PUBLISHES it on ConditionAppliedTopic; the owner's SheetKeeper is
// subscribed and applies it. Dodge, Disengage, Hide, Help and Reckless Attack
// all take the same path.
//
// So an activation run off the bus is not an error — it is worse. The publish
// succeeds, returns nil, and the sheet is persisted with no condition on it.
// EffectiveAC silently falls back to base and nothing in the call stack says
// anything happened (the shape that bit the equip path, rpg-api#842). Running
// here means the actor was attached by [attachAll] through
// [character.Attach], so the subscription that applies the condition is live
// for exactly as long as the activation is.
//
// # It charges nothing at the door
//
// [Input.Cost] must stay NIL for an activation, and that is a real constraint
// rather than an omission. The ability spends its own slot — activateFeature
// calls consumeActionType, and a combat ability spends through the toolkit
// economy it is handed — so a Cost passed alongside would charge the same
// ledger twice and the second charge would look exactly like the first.
// "Free action" in Cost's own doc means "this package charges nothing", not
// "this costs nothing".
//
// Refuses a nil input, a member with no ID, and a missing or invalid ability
// ref, all before the world is loaded.
func NewActivation(in *ActivationInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.MemberID == "" {
		return nil, fmt.Errorf("%w: no member is activating", ErrBadActivation)
	}
	if in.Ability == nil {
		return nil, fmt.Errorf("%w: member %q named no ability", ErrBadActivation, in.MemberID)
	}
	if err := in.Ability.IsValid(); err != nil {
		return nil, fmt.Errorf("%w: member %q named an unusable ability: %w",
			ErrBadActivation, in.MemberID, err)
	}

	// BOTH pieces of caller-owned material are copied, not just the obvious
	// one. core.Ref is a mutable struct, so keeping the caller's pointer would
	// let a reused ref change WHICH ABILITY this machine activates after it was
	// constructed — a worse version of the observer-slice hazard, and one that
	// is easy to miss precisely because the slice copy right beside it looks
	// like the defence was already made.
	ability := *in.Ability

	observers := make([]int, len(in.ObserverPassivePerceptions))
	copy(observers, in.ObserverPassivePerceptions)

	return &activationMachine{
		member:    in.MemberID,
		ability:   &ability,
		targetID:  in.TargetID,
		observers: observers,
	}, nil
}

type activationMachine struct {
	member    string
	ability   *core.Ref
	targetID  string
	observers []int
}

// Start is pure preflight and runs before payment: it finds the actor and the
// target in the cast and yields the step that does the work, without touching
// either.
//
// The cast lookups happen HERE rather than inside the step for the reason
// Start exists — a member who was never passed in is a caller defect, and
// discovering it after the door has been paid at would charge somebody for an
// interaction that cannot run. It is the same preflight [strikeMachine] does
// for a combatant it was never handed.
func (m *activationMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	actor, ok := cast.Character(m.member)
	if !ok {
		// Named separately from "not in the cast at all" because a monster IS
		// in the cast and still cannot be here: its abilities are driven, not
		// declared, and its economy is not on its sheet.
		if _, isMonster := cast.Monster(m.member); isMonster {
			return nil, fmt.Errorf("%w: %q is a monster; its abilities are driven, not declared",
				ErrBadActivation, m.member)
		}
		return nil, fmt.Errorf("%w: %q is not a participant", ErrBadActivation, m.member)
	}

	if err := m.checkTargetContract(actor); err != nil {
		return nil, err
	}

	var target core.Entity
	if m.targetID != "" {
		found, err := memberEntity(cast, m.targetID)
		if err != nil {
			return nil, err
		}
		target = found
	}

	return m.step(actor, target), nil
}

// checkTargetContract enforces what ActivationInput.TargetID promises: required
// when the ability takes somebody, empty when it does not.
//
// # The rulebook answers, this does not decide
//
// Which abilities take a target is rules knowledge, and re-stating it here
// would be a second table to disagree with the first. So it ASKS: the sheet's
// own AvailableAbilities carries each ability's TargetKind, authored by the
// same table Afford projects into a declaration. This reads that answer and
// enforces it; it does not have an opinion about Help.
//
// # It stays quiet when the sheet cannot answer
//
// AvailableAbilities is empty for a character who is not in combat, and an
// ability this character does not carry is simply absent. NEITHER is a
// malformed call — the first is an actor-state refusal and the second is
// "unknown ability", and both belong to ErrActivationRefused. Refusing them
// here would report a barbarian out of combat as a caller defect, which is the
// exact confusion the two sentinels exist to prevent.
func (m *activationMachine) checkTargetContract(actor *character.Character) error {
	var kind character.TargetKind
	found := false
	for _, available := range actor.AvailableAbilities() {
		if available.Ref != nil && available.Ref.ID == m.ability.ID {
			kind, found = available.TargetKind, true
			break
		}
	}
	if !found {
		return nil
	}

	wantsTarget := kind == character.TargetKindSingleEntity
	switch {
	case wantsTarget && m.targetID == "":
		return fmt.Errorf("%w: %s needs a target and none was named",
			ErrBadActivation, m.ability.String())
	case !wantsTarget && m.targetID != "":
		// Refused rather than ignored. A target quietly dropped is a client
		// that believes it aimed Dodge at somebody and a server that knows
		// better, which is a disagreement nobody finds until it matters.
		return fmt.Errorf("%w: %s takes no target, but %q was named",
			ErrBadActivation, m.ability.String(), m.targetID)
	}
	return nil
}

// step is the one thing this machine does.
//
// A Gather rather than a new Step kind: "do this on the bus and hand me the
// next step" is what Gather already means, and adding a case against one
// example is the mistake this package's own step vocabulary keeps refusing to
// make.
func (m *activationMachine) step(actor *character.Character, target core.Entity) Step {
	return Gather{
		name: fmt.Sprintf("activate %s for %s", m.ability.String(), m.member),
		run: func(ctx context.Context, _ events.EventBus) (Step, error) {
			// The bus is not passed in, and that is not an oversight. The
			// sheet was handed its own view of this interaction's bus when
			// attachAll applied its keeper, and every verb on it publishes
			// through that. Handing a second reference to the same bus would
			// invite a caller to believe there was a choice.
			out, err := actor.ActivateAbility(ctx, &character.ActivateAbilityInput{
				AbilityRef:                 m.ability,
				TargetID:                   m.targetID,
				Target:                     target,
				ObserverPassivePerceptions: m.observers,
			})
			if err != nil {
				return nil, fmt.Errorf("activate %s for %q: %w", m.ability.String(), m.member, err)
			}

			// THE TRANSLATION, and it happens here rather than three layers
			// up. The sheet answers a refusal as (output{Success:false}, nil):
			// "not in combat", "unknown ability", "no rage charges left", and
			// every feature's own precondition come back as a SUCCESSFUL call
			// carrying a false. A machine that returned Done on that would
			// hand its caller a finished interaction that did nothing, and the
			// dirty sheets would be saved to prove it.
			//
			// Distinct sentinel from ErrBadActivation for ErrCannotPay's
			// reason: one is an actor who cannot do this right now and wants a
			// different verb, the other is content or wiring being wrong and
			// wants a developer. A single sentinel would send the first one
			// looking at the wrong sheet.
			if !out.Success {
				return nil, fmt.Errorf("%w: %s", ErrActivationRefused, out.Error)
			}

			return Done{Outcome: ActivationOutcome{
				Ability:         m.ability.String(),
				GrantedCapacity: out.GrantedCapacity,
			}}, nil
		},
	}
}

// memberEntity finds one member of the cast as an entity, character or monster.
//
// Help is the only one of the seven that takes a target, and it aids an ally —
// which in a dungeon can be a summoned or allied monster as easily as another
// character, so this does not assume the party. An ID that is neither is a
// caller defect and says so rather than passing a nil Target into an ability
// that will dereference it.
func memberEntity(cast *Participants, id string) (core.Entity, error) {
	if ch, ok := cast.Character(id); ok {
		return ch, nil
	}
	if mon, ok := cast.Monster(id); ok {
		return mon, nil
	}
	return nil, fmt.Errorf("%w: target %q is not a participant", ErrBadActivation, id)
}
