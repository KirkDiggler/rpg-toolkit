// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// ContestInput describes what somebody may save against: a condition, damage,
// or both. At least one of the two must be declared.
type ContestInput struct {
	Gate        *saves.SaveGate
	SaverID     string
	Application combatActions.ConditionApplication

	// Damage is what a failed save costs the saver, declared the same way an
	// attack declares its pools. It is the contest's SECOND consequence rather
	// than a second machine: the sequence is already Request(save) -> outcome
	// policy -> deliver -> Done, and damage is a delivery this step could not
	// express. Empty is the common case and the whole of today's behaviour.
	//
	// A zero-value ContestInput therefore still says what it always said: a
	// contest with no damage declared deals none.
	Damage []damage.Damage

	// SourceName is the display name of whatever [ContestInput.Cause] names as
	// the effect — "Vicious Mockery". REQUIRED whenever damage is declared, and
	// unread otherwise.
	//
	// Supplied rather than looked up, because the lookup would be a spell table
	// and this package does not have one and must not grow one (ADR-0045). It
	// is the provenance PAIR a roll trace is built from — ref and name — and a
	// trace missing either is refused by the rulebook rather than recorded
	// half-named, so a contest that deals damage without one is refused here
	// instead of producing a record nobody can validate.
	SourceName string

	// Removal is the contest's THIRD consequence: conditions that come off
	// sheets when the save fails, and the owner that ends with them.
	//
	// It is how a check produces a strip rather than an application, and it is
	// the shape a concentration break takes. Nil is the common case and the
	// whole of today's behaviour.
	Removal *ConditionRemoval

	// Move is the contest's FOURTH consequence: the saver is moved against
	// their will, which is the only consequence this package describes rather
	// than delivers.
	//
	// Nil is the common case. The anchor is the caller's to name — the cast
	// door names the caster — because a contest knows who saved and what they
	// saved against, and not who is doing the pushing.
	//
	// It RIDES a contest that also delivers something: a contest whose only
	// consequence is a move is refused with the empty one, because what is at
	// stake in a save would have nothing to name. No content declares one —
	// a cast must declare damage or a condition to exist at all.
	Move *MoveDirective

	Cause       dnd5eEvents.SaveCause
	DamageTaken int
	Roller      dice.Roller
	prepared    *preparedCondition
}

// ImposedEffectKind names WHICH consequence one [ImposedEffect] records.
//
// It is stated rather than inferred from which fields are populated: a
// condition with a nil ref and damage that landed for zero are both legal, and
// neither reading of the other fields could tell them apart.
type ImposedEffectKind string

const (
	// ImposedCondition is a condition the contest put on the saver.
	ImposedCondition ImposedEffectKind = "condition"

	// ImposedDamage is damage the contest landed on the saver.
	ImposedDamage ImposedEffectKind = "damage"

	// ImposedHealing is an immediate, post-clamp HP restoration.
	ImposedHealing ImposedEffectKind = "healing"

	// ImposedConditionRemoved is a condition the contest took OFF somebody.
	//
	// A kind of its own rather than an application with an empty payload,
	// because "this landed" and "this came off" are opposite facts and a reader
	// that had to infer which from a nil field would be inferring a rule.
	ImposedConditionRemoved ImposedEffectKind = "condition-removed"

	// ImposedMove is a move the contest forced on the saver: a shove, a slide,
	// a creature sent running.
	//
	// It is the consequence that is not finished when this package is done
	// with it. A condition is APPLIED here and damage is DEALT here, but a move
	// is only described — the cells it crosses are read from the map under its
	// own fold, which is encounter's, so what leaves here is a directive and
	// whoever holds the board walks it (rpg-project#431 §2).
	ImposedMove ImposedEffectKind = "move"
)

// MoveDirective is a move DESCRIBED: how the creature is moved, what it is
// measured from, how far, and what being moved costs them.
//
// NOTHING HERE IS GEOMETRY. No cells, no directions, no map. It mirrors
// [combatActions.CastMove], which is how content declares the same thing, plus
// the anchor — the creature the policy is measured from, which content cannot
// name because it does not know who cast it.
//
// The division is the point: a spell holding a path search of its own is what
// this shape exists to prevent, and a rules layer that worked out which cells a
// push crosses would be a second author of passability the day it disagreed
// with the fold.
type MoveDirective struct {
	// Policy is how the mover is moved. One value exists and it is the one
	// something can walk (see [combatActions.MovePolicy]).
	Policy combatActions.MovePolicy

	// AnchorID is the creature the policy is measured from — the caster, for
	// everything that pushes today.
	//
	// AN ID RATHER THAN A POSITION, because a position read here would be read
	// at the moment the save resolved and walked at the moment the push runs,
	// and the two are not the same moment. Whoever executes the move asks the
	// board where the anchor is.
	AnchorID string

	// Cells is a fixed budget, Speed says the budget is the mover's own speed
	// instead, and Turn says it is whatever movement the mover has left on its
	// own turn. Exactly one of the three, as the declaration requires.
	Cells int
	Speed bool

	// Turn is the third budget, and it is only meaningful when the move IS the
	// mover's turn — which is what [Obey] produces and what no cast declares.
	// Whoever owns the board reads the remaining movement off the turn it is
	// already tracking.
	Turn bool

	// Pays is what being moved costs the creature that is moved. Zero is
	// nothing, which is the push.
	Pays combatActions.MovePays

	// Provokes says whether the move offers opportunity attacks as it goes.
	// Zero is false: being thrown is not walking out of a reach.
	Provokes bool
}

// ImposedEffect names one consequence a contest delivered, or would have
// delivered, to the saver.
//
// Kind says which of the two it is. Ref and Description are filled for both:
// a condition names the condition, damage names whatever the save's cause named
// as the thing that dealt it. Amount and Components carry the damage's
// arithmetic and are zero and nil on a condition, which is the honest reading —
// a condition has no amount to report.
type ImposedEffect struct {
	Kind        ImposedEffectKind
	Ref         *core.Ref
	Description string

	// RecipientID is who it landed on. Always the saver for a contest, and for
	// a cast either party — True Strike's condition goes on the CASTER — which
	// is why it travels with the effect rather than being inferred by whoever
	// reads it.
	RecipientID string

	// Address is the exact source-qualified identity for condition consequences.
	Address dnd5eEvents.ConditionAddress

	// Move is the directive for an [ImposedMove], and nil for every other
	// kind. It is the whole of what this package says about the move: the
	// cells are worked out by whoever owns the board.
	Move *MoveDirective

	// NotTaken is why an imposed move was not carried out, and empty when it
	// was. It is only ever set on an [ImposedMove].
	//
	// A MOVE NOBODY COULD AFFORD IS A REAL ANSWER, not a missing one. The
	// directive still rides beside this, so a reader can see what the creature
	// was asked to do as well as why it did not — and whoever owns the board
	// records a distance of zero that says so rather than silently walking
	// nobody. Today the only reason is the price: a creature with no reaction
	// left cannot buy the right to run.
	NotTaken string `json:"not_taken,omitempty"`

	// Amount is the damage the SHEET applied. Zero on a condition, and zero on
	// the AT-STAKE effect of a contest whose dice have not been rolled yet.
	Amount int

	// Requested is what the dice and the multipliers settled on, before the
	// sheet had a say. It equals [ImposedEffect.Calculation]'s total by
	// construction, which is the rule a record validates against.
	//
	// It is Amount's sibling for the same reason healing has one: the two
	// separate the moment somebody's hit points stop moving, and a record that
	// carried only the applied number could not show a hit that was bigger than
	// the target had left.
	Requested int

	// Before and After are the recipient's hit points either side of the
	// application, as the sheet reported them.
	Before int
	After  int

	// Components are the TYPED breakdown — how much of which damage type, with
	// the pool's declared properties. Calculation cannot say this: a roll
	// component carries faces and modifiers, not a damage type.
	Components []dnd5eEvents.DamageComponent

	// Calculation is the roll's authoritative arithmetic — every component's
	// dice trace and modifier, and the total they come to. It is the
	// representation a record replays, and its total is [ImposedEffect.Requested].
	Calculation *dnd5eEvents.RollCalculation
}

// ContestOutcome records the requested save and what its failure delivered.
//
// AtStake is what the save was against: the condition when one was declared,
// and otherwise the declared damage. Imposed is what the save actually
// delivered: on a failure, damage first, then the condition, then the move; on
// a success, nothing at all, unless the gate halves — a made save against a
// Half gate imposes the one halved damage and never a condition or a move.
type ContestOutcome struct {
	Save      SaveOutcome
	DC        int
	Ability   abilities.Ability
	Succeeded bool
	AtStake   ImposedEffect
	Imposed   []ImposedEffect

	// FollowUps are the checks this contest's own damage came back with, in
	// append order. Empty is the common case: only damage that landed on
	// somebody holding an ongoing rule produces one.
	FollowUps []FollowUpOutcome
}

func (ContestOutcome) isOutcome() {}

type preparedCondition struct {
	declaration combatActions.ConditionApplication
	behavior    dnd5eEvents.ConditionBehavior
}

// validateGate checks the subset of save gates resolution can execute.
//
// It was validateConditionGate and was never only about conditions: Start has
// always called it for every contest, damage-only ones included, so a name that
// promised less than it did was refusing damage gates with a message about
// conditions. The name now says what the function does.
//
// What is left to check here is what the gate cannot see for itself.
// saves.SaveGate.Validate already refuses any word that is neither Negated nor
// Half, so this adds the ownership question: HALF OF WHAT.
//
// Half a condition is not a smaller condition and half a removal is not a
// partial one, so Half is permitted only for a contest whose consequences are
// damage and, at most, the move that damage's failure imposes.
//
// It must also be half of ONE DAMAGE TYPE. The halving is a single component
// and combat.FinalDamage groups per type, so a reduction large enough to sink
// the type it sits on leaves the trace explaining a number FinalDamage never
// produced — measured at 1d4 psychic beside 4d6 fire, where the delivery
// refused with "dealt 20, and its roll trace explains 11". That refusal is
// correct and it is far too late: the cast is charged and the save is rolled
// by then, which is the very shape Start's damage preflight exists to prevent.
// A second type arrives with the spell that has one, and it brings a reduction
// per type with it.
//
// Recurrence is still refused outright: shared action data permits it and this
// module resolves only the immediate save.
// contestShape is what a gate is validated AGAINST: the consequences a contest
// declares, reduced to the two facts a Half gate's legality turns on.
//
// A struct rather than two bare bools because they are answers to the same
// question — half of WHAT — and a caller that had to remember their order
// would be one transposition away from permitting exactly what this refuses.
type contestShape struct {
	// damageOnly is true when damage is the whole of what a failure delivers.
	damageOnly bool

	// damageTypes is how many DISTINCT damage types the declared pools carry.
	damageTypes int
}

func validateGate(gate *saves.SaveGate, shape contestShape) error {
	if gate == nil {
		return fmt.Errorf("%w: contest has no save gate", ErrNilInput)
	}
	if err := gate.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrBadGate, err)
	}
	for _, ability := range gate.Abilities {
		if !supportedSaveAbility(ability) {
			return fmt.Errorf("%w: unsupported save ability %q", ErrBadGate, ability)
		}
	}
	if gate.OnSuccess == saves.Half {
		if !shape.damageOnly {
			return fmt.Errorf(
				"%w: half on a save is for damage only, and this contest delivers more than damage",
				ErrBadGate)
		}
		if shape.damageTypes != 1 {
			return fmt.Errorf(
				"%w: half on a save halves one damage type; a second type arrives with the spell "+
					"that has one", ErrBadGate)
		}
	}
	if gate.Recurrence != saves.RecurrenceNone {
		return fmt.Errorf("%w: %q", ErrRecurrenceUnsupported, gate.Recurrence)
	}

	return nil
}

func supportedSaveAbility(ability abilities.Ability) bool {
	switch ability {
	case abilities.STR, abilities.DEX, abilities.CON, abilities.INT, abilities.WIS, abilities.CHA:
		return true
	default:
		return false
	}
}

func prepareCondition(
	application combatActions.ConditionApplication, targetID, sourceRef string,
) (preparedCondition, error) {
	if err := application.Validate(); err != nil {
		return preparedCondition{}, fmt.Errorf("%w: %w", ErrBadAction, err)
	}
	if application.Ref.Equals(refs.Conditions.Blessed()) {
		var config struct {
			SourceID string `json:"source_id"`
		}
		if err := json.Unmarshal(application.Parameters, &config); err != nil {
			return preparedCondition{}, fmt.Errorf("build condition blessed for %q: %w", targetID, err)
		}
		source, err := core.ParseString(sourceRef)
		if err != nil {
			return preparedCondition{}, fmt.Errorf("build condition blessed for %q: source: %w", targetID, err)
		}
		condition, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
			MemberID: targetID, SourceID: config.SourceID, SourceRef: source,
		})
		if err != nil {
			return preparedCondition{}, fmt.Errorf("build condition blessed for %q: %w", targetID, err)
		}
		return preparedCondition{declaration: application.Clone(), behavior: condition}, nil
	}
	if application.Ref.Equals(refs.Conditions.Baned()) {
		var config struct {
			SourceID string `json:"source_id"`
		}
		if err := json.Unmarshal(application.Parameters, &config); err != nil {
			return preparedCondition{}, fmt.Errorf("build condition baned for %q: %w", targetID, err)
		}
		source, err := core.ParseString(sourceRef)
		if err != nil {
			return preparedCondition{}, fmt.Errorf("build condition baned for %q: source: %w", targetID, err)
		}
		condition, err := conditions.NewBanedCondition(conditions.NewBanedConditionInput{
			MemberID: targetID, SourceID: config.SourceID, SourceRef: source,
		})
		if err != nil {
			return preparedCondition{}, fmt.Errorf("build condition baned for %q: %w", targetID, err)
		}
		return preparedCondition{declaration: application.Clone(), behavior: condition}, nil
	}
	built, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref:       application.Ref.String(),
		Config:    application.Parameters,
		MemberID:  targetID,
		SourceRef: sourceRef,
	})
	if err != nil {
		return preparedCondition{}, fmt.Errorf("build condition %s for %q: %w", application.Ref.ID, targetID, err)
	}
	return preparedCondition{declaration: application.Clone(), behavior: built.Condition}, nil
}

func (p preparedCondition) atStake(recipientID string) ImposedEffect {
	ref := p.declaration.Ref
	return ImposedEffect{
		Kind:        ImposedCondition,
		Ref:         &ref,
		Description: conditionDescription(ref),
		RecipientID: recipientID,
		Address:     conditions.ConditionAddressOf(recipientID, p.behavior),
	}
}

// conditionDescription is how a delivered condition reads in a step log and in
// a record. One spelling, so the contest's imposition and a cast's delivery
// describe the same condition the same way.
func conditionDescription(ref core.Ref) string {
	return fmt.Sprintf("the %s condition", ref.ID)
}

func publishPreparedCondition(
	prepared preparedCondition, cast *Participants, targetID string,
	source dnd5eEvents.ConditionSource, next func(replaced []ImposedEffect) (Step, error),
) Gather {
	return Gather{
		name: "impose " + conditionDescription(prepared.declaration.Ref),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			target, err := cast.entity(targetID)
			if err != nil {
				return nil, err
			}
			replaced, err := publishCondition(ctx, bus, cast, prepared, target, source)
			if err != nil {
				return nil, err
			}
			return next(replaced)
		},
	}
}

// replaceSameAddress takes off whatever the recipient already holds at the
// ADDRESS about to land, and hands back the effects that say so.
//
// # One instance per address per member
//
// A condition's address is member, ref and source (dnd5eEvents.ConditionAddress).
// A second instance arriving at an address already occupied replaces what is
// there: two Commands on one creature is not a state anything can obey, and
// Command's second word replacing the first is the use case that brought this
// rule.
//
// # The address is the key, and the ref is NOT
//
// THIS PARAGRAPH RECORDS A MISTAKE, because the mistake is the reason the key
// is what it is. The design and the plan both claimed that Bane from two
// casters stacked two −1d4 and that the first to end took both off, and offered
// that as the general stacking bug this rule would fix. Both halves were false,
// and a reviewer proved it against the tree:
//
//   - conditions.BanedContributionGroup already makes equal-potency Banes
//     non-stacking — DescribeSelectedRollContributions takes the oldest
//     provider in the group and skips the rest, so two Banes contributed one
//     −1d4 before this rule existed.
//   - Removal already compares the FULL address (character.onConditionRemoved),
//     and BanedCondition carries its caster as the source, so the first Bane to
//     end took off only itself.
//
// Keying this rule on the ref instead would have done real damage: a second
// caster's Bane would strip the first caster's instance, whose owner would find
// its child list empty and END THAT CASTER'S CONCENTRATION. One player's cast
// would silently free another player's spell. RAW 2014 says the opposite — the
// same spell cast twice keeps both durations and applies the most potent — which
// is exactly what the stacking group already implements.
//
// So the key is the address. For every condition with no source of its own —
// Commanded, Prone, Vicious Mockery — the two keys are the same thing, and
// those are the conditions this rule exists for: two entries sharing one
// address is what makes a single removal strip both.
//
// # It governs less than "every condition"
//
// Every delivery through publishCondition uses this rule, including gated
// impositions and gateless casts. Source-qualified addresses keep independent
// casters' durations intact; only another instance at the exact address is replaced.
//
// # Removed first, applied second, and the order is the rule
//
// The keeper strips by address off the same list it is about to append to. A
// removal published after the new condition landed would find two entries at
// one address and take off whichever it reached first, which is a coin toss
// between replacing the old one and undoing the new one.
//
// It ranges over the RECIPIENT's own sheet and nobody else's: this is a pointed
// question about one member, not a set derived from what happened to be loaded.
// The slice it walks is the sheet's LIVE one, and the keeper reassigns that
// field while this loop runs — safe only because onConditionRemoved builds a
// fresh filtered slice rather than compacting in place. A future keeper that
// compacted in place would corrupt this iteration, so it would have to hand
// back a copy here.
func replaceSameAddress(
	ctx context.Context, bus events.EventBus, cast *Participants, targetID string,
	landing dnd5eEvents.ConditionAddress,
) ([]ImposedEffect, error) {
	var replaced []ImposedEffect
	for _, held := range heldConditions(cast, targetID) {
		address := conditions.ConditionAddressOf(targetID, held)
		if address != landing {
			continue
		}
		if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(
			ctx, dnd5eEvents.ConditionRemovedEvent{
				MemberID:     address.MemberID,
				ConditionRef: address.ConditionRef,
				SourceID:     address.SourceID,
				Reason:       ConditionReplacedReason,
			}); err != nil {
			return nil, fmt.Errorf("replace %s on %q: %w", address.ConditionRef, targetID, err)
		}
		effect := removalEffect(address, ConditionReplacedReason)
		effect.Description = fmt.Sprintf("%s replaced by a newer instance", address.ConditionRef)
		replaced = append(replaced, effect)
	}

	return replaced, nil
}

// ConditionReplacedReason is why a condition came off when a second instance
// landed at its address. It travels on the removal so a listener can tell a
// replacement from an expiry, a dispel, or a concentration break.
const ConditionReplacedReason = "replaced"

// heldConditions is what one member's sheet is currently carrying, and an
// absent member carries nothing.
//
// Nil for somebody who is not in the cast, rather than an error, because every
// caller is asking a question that has the same answer either way: a member
// with no sheet here holds no condition this interaction can see. The callers
// that need a member to EXIST ask cast.entity, which refuses.
func heldConditions(cast *Participants, memberID string) []dnd5eEvents.ConditionBehavior {
	if character, ok := cast.Character(memberID); ok {
		return character.GetConditions()
	}
	if monster, ok := cast.Monster(memberID); ok {
		return monster.GetConditions()
	}

	return nil
}

// publishCondition is the one place a built condition reaches the bus.
//
// The publish is the whole application: an effect never attaches itself, it
// publishes and the owning keeper applies. Two callers share this — the
// contest's imposition and the gateless cast's delivery — so there is one
// implementation of what "the condition landed" means rather than a copy per
// door.
//
// The source is the caller's to state because the callers mean different things
// by it, and none may guess for another.
func publishCondition(
	ctx context.Context, bus events.EventBus, cast *Participants, prepared preparedCondition,
	target core.Entity, source dnd5eEvents.ConditionSource,
) ([]ImposedEffect, error) {
	landing := conditions.ConditionAddressOf(target.GetID(), prepared.behavior)
	replaced, err := replaceSameAddress(ctx, bus, cast, target.GetID(), landing)
	if err != nil {
		return nil, err
	}
	// Removal precedes application. If publication fails, the interaction
	// fails and the host reloads persisted data; re-applying the old instance
	// here would compound the subscribers' partially updated state.
	err = dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    target,
		Type:      dnd5eEvents.ConditionType(prepared.declaration.Ref.ID),
		Source:    source,
		Condition: prepared.behavior,
	})
	if err != nil {
		return nil, fmt.Errorf("apply %s to %q: %w", prepared.declaration.Ref.ID, target.GetID(), err)
	}

	return replaced, nil
}

// halvedBySaveLabel is what the halving component calls itself on the roll
// trace. A player reading the damage breakdown sees the dice, then this line,
// then the total they came to.
const halvedBySaveLabel = "halved by a successful save"

// describeDamage names the declared pools the way a step log should read them:
// "1d4 psychic damage", and "1d4 psychic and 1d6 fire damage" for two.
func describeDamage(pools []damage.Damage) string {
	if len(pools) == 0 {
		return "no damage"
	}
	parts := make([]string, 0, len(pools))
	for _, pool := range pools {
		parts = append(parts, fmt.Sprintf("%s %s", pool.Dice, pool.Type))
	}

	return strings.Join(parts, " and ") + " damage"
}

// applyPreparedDamage is publishPreparedCondition's sibling: the other thing a
// failed save can deliver, in the same shape, chained through the same next().
//
// Bus-free on the write, exactly as a strike's damage phase is. The dice are
// rolled HERE rather than in Start, so a contest refused at the door — an
// invalid gate, an unpayable price — rolls nothing.
//
// # No damage-chain fold, and it is named rather than hidden
//
// A strike folds DamageChainEvent before it applies, which is how a condition
// adds to or resists the number. This does not: the fold's input is an
// attacker, a weapon, an ability and a criticality, and a contest has none of
// them — the machine knows only who saved and what the save was against.
// combat.FinalDamage still runs, so a multiplier that reaches these components
// applies; what is missing is the bus round that could put one there. The cost
// is real and bounded: a target resistant to a spell's damage type takes full
// damage this slice. It is bought back when a cast carries a caster the fold
// can name, not by inventing one here.
func applyPreparedDamage(
	pools []damage.Damage, roller dice.Roller, cause dnd5eEvents.SaveCause, sourceName string,
	cast *Participants, targetID string, halved bool, next func(ImposedEffect) (Step, error),
) Gather {
	described := describeDamage(pools)
	if halved {
		described += " (halved)"
	}

	return Gather{
		name: "deal " + described,
		run: func(ctx context.Context, _ events.EventBus) (Step, error) {
			target, err := combatantFor(cast, targetID)
			if err != nil {
				return nil, err
			}
			components, err := rollContestDamage(ctx, pools, roller, cause, sourceName)
			if err != nil {
				return nil, err
			}
			if halved {
				components, err = halveDamage(components, cause, sourceName)
				if err != nil {
					return nil, err
				}
			}

			calculation, err := damageCalculation(components)
			if err != nil {
				return nil, err
			}

			final, total := combat.FinalDamage(components)
			if total != calculation.Total {
				// The trace no longer explains the number. It cannot happen
				// while nothing folds this damage — every component here came
				// from a declared pool and none carries a multiplier — and if
				// something ever does, a record built from this would show
				// faces that do not add up to the damage the player took.
				return nil, fmt.Errorf(
					"%w: %s dealt %d, and its roll trace explains %d",
					ErrBadAction, described, total, calculation.Total)
			}

			instances := make([]combat.DamageInstance, 0, len(final))
			for _, instance := range final {
				instances = append(instances, combat.DamageInstance{
					Amount: instance.Amount,
					Type:   string(instance.Type),
				})
			}

			// The sheet's own business, and the same call a strike makes, so a
			// cantrip that drops somebody to zero flows through the death-save
			// and downed transitions Character.ApplyDamage already owns.
			applied := target.ApplyDamage(ctx, &combat.ApplyDamageInput{Instances: instances})

			return next(ImposedEffect{
				Kind:        ImposedDamage,
				Ref:         cloneCoreRef(cause.EffectRef),
				Description: described,
				RecipientID: targetID,
				Amount:      applied.TotalDamage,
				Requested:   calculation.Total,
				Before:      applied.PreviousHP,
				After:       applied.CurrentHP,
				Components:  cloneDamageComponents(components),
				Calculation: calculation,
			})
		},
	}
}

// validateMove refuses a directive this stack cannot execute, at the door,
// before the save is rolled or the price is charged.
//
// The policy, the budget and the price are CONTENT's rules, so they are checked
// by content's own validator rather than restated here — one owner for what a
// move may declare, and no second copy to drift. What this layer adds is the
// anchor, which content never names.
//
// # The two declarations this used to refuse are now executable
//
// A move paid for with a reaction, and a budget that is the mover's own speed,
// were both refused here for the same reason: they validated in content and had
// no executor anywhere in this stack, so describing one would have handed the
// board a directive it walked for free or for a distance nobody worked out.
//
// Dissonant Whispers brought both, and the arms are gone rather than worked
// around. The price is charged by payForMove before the move is described, and
// the speed is read into cells by whoever owns the board, from the roster row
// it already holds — which is the same reason the cells were never worked out
// here either.
func validateMove(directive *MoveDirective) error {
	declared := combatActions.CastMove{
		Policy: directive.Policy, Cells: directive.Cells, Speed: directive.Speed,
		Turn: directive.Turn, Pays: directive.Pays, Provokes: directive.Provokes,
	}
	if err := declared.Validate(); err != nil {
		return fmt.Errorf("%w: contest move: %w", ErrBadAction, err)
	}
	if directive.AnchorID == "" {
		return fmt.Errorf("%w: a move must name the anchor it is measured from", ErrBadAction)
	}

	return nil
}

// describeMove names the directive the way a step log should read it: "a line
// move of 2 cells", "an away move of their own speed".
//
// The article agrees with the policy word, which is a detail only because the
// policy is content's own vocabulary rather than a closed list this function
// may switch on: the second policy to arrive started with a vowel, and the
// third might too.
func describeMove(directive MoveDirective) string {
	budget := fmt.Sprintf("%d cells", directive.Cells)
	switch {
	case directive.Speed:
		budget = "their own speed"
	case directive.Turn:
		budget = "their own turn's movement"
	}

	return fmt.Sprintf("%s %s move of %s", articleFor(string(directive.Policy)), directive.Policy, budget)
}

// articleFor is "an" before a vowel sound and "a" otherwise, judged on the
// spelling because every word it sees is a policy an author wrote in English.
func articleFor(word string) string {
	if word == "" {
		return "a"
	}
	switch word[0] {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return "an"
	default:
		return "a"
	}
}

// payForMove charges the directive's price, and it runs BEFORE the move is
// described rather than after.
//
// PAID FIRST, WALKED AFTER. A reaction buys the right to run, not the distance
// run: a creature that pays and then finds a wall two cells out does not get a
// refund, exactly as a creature that Dashes into a dead end does not get its
// action back. Charging after the walk would make the price depend on the map,
// which is a rule nobody wrote and a seam this package does not own.
//
// Three answers, and each of them ends the price:
//
//   - The target is DOWN. The dropped are not asked. `stayed` finishes the
//     contest with no move at all, which is imposeMove's own reading of
//     rpg-project#432 §5 applied one step earlier — the creature that is not
//     going to be pushed is also not going to be billed for it.
//   - The target CANNOT REACT. The move is described anyway, carrying
//     [ImposedEffect.NotTaken], because a price nobody could pay is a real
//     answer: whoever owns the board records a distance of zero that says why,
//     rather than a missing result somebody downstream has to interpret.
//   - The target CAN. The same [dnd5eEvents.SpendRequestedEvent] an
//     opportunity attack publishes goes out, attributed to whatever raised the
//     save, and the move is described.
//
// The bus is the Gather's own — the driver's, which is where every sheet in
// this interaction attached its keeper. A bus captured out of an earlier step
// would publish onto whatever bus that step happened to run on, which is the
// rule movementMachine.bill states and this obeys.
func payForMove(
	directive MoveDirective, cause dnd5eEvents.SaveCause, cast *Participants, targetID string,
	paid func() (Step, error), unpaid func(ImposedEffect) (Step, error), stayed func() (Step, error),
) Gather {
	return Gather{
		name: "pay for " + describeMove(directive),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			target, err := combatantFor(cast, targetID)
			if err != nil {
				return nil, err
			}
			if combat.IsDown(target) {
				return stayed()
			}
			if !target.CanReact() {
				asked := directive
				return unpaid(ImposedEffect{
					Kind:        ImposedMove,
					Ref:         cloneCoreRef(cause.EffectRef),
					Description: describeMove(asked),
					RecipientID: targetID,
					Move:        &asked,
					NotTaken:    noReactionToSpend,
				})
			}

			if err := dnd5eEvents.SpendRequestedTopic.On(bus).Publish(ctx,
				dnd5eEvents.SpendRequestedEvent{
					MemberID:   targetID,
					ActionType: coreCombat.ActionReaction,
					Amount:     1,
					SourceRef:  cloneCoreRef(cause.EffectRef),
				}); err != nil {
				return nil, fmt.Errorf("charge %q for a move: %w", targetID, err)
			}

			return paid()
		},
	}
}

// noReactionToSpend is the one reason a move goes untaken today.
const noReactionToSpend = "has no reaction to spend"

// imposeMove is applyPreparedDamage's other sibling: the third thing a failed
// save can cost, in the same shape, chained through the same continuation.
//
// IT MOVES NOBODY. What it produces is the directive, handed on as an
// [ImposedMove] for whoever owns the board to route and walk. There is no
// publish and no write here, which is why it is the one consequence that is not
// finished when this package is: a rule that worked out the cells would be
// reading a map it does not own, and would disagree with the fold the day the
// two were written by different hands.
//
// # The dropped are not pushed
//
// Read here rather than decided in resolve, because resolve builds the whole
// chain before the damage step has run: the hit points that matter are the ones
// the saver has at the moment the push would happen, and this is that moment.
// A creature the damage dropped stays where it fell (rpg-project#432 §5), and
// it is `stayed` that says so — the effect is never produced, rather than
// produced and filtered by somebody downstream.
func imposeMove(
	directive MoveDirective, cause dnd5eEvents.SaveCause, cast *Participants, targetID string,
	pushed func(ImposedEffect) (Step, error), stayed func() (Step, error),
) Gather {
	return Gather{
		name: "impose " + describeMove(directive),
		run: func(_ context.Context, _ events.EventBus) (Step, error) {
			target, err := combatantFor(cast, targetID)
			if err != nil {
				return nil, err
			}
			if combat.IsDown(target) {
				return stayed()
			}

			moved := directive
			return pushed(ImposedEffect{
				Kind:        ImposedMove,
				Ref:         cloneCoreRef(cause.EffectRef),
				Description: describeMove(moved),
				RecipientID: targetID,
				Move:        &moved,
			})
		},
	}
}

// halveDamage makes a made save's damage half of what was rolled, as ONE MORE
// COMPONENT rather than as a flag on the roll.
//
// The rounded half's complement rides on a modifier-only component of the same
// damage type: 3d6 that came to 13 gains a -7, and the trace totals 6. Nothing
// downstream has to learn a new word for it. The guard that FinalDamage and the
// trace agree holds by construction, the encounter record's identical guard
// holds for the same reason, and a client already rendering Bane's -1d4 renders
// this with no change — which a Halved flag on the calculation could not have
// claimed, since a trace showing dice that sum to 13 above a total of 6 is a
// trace that contradicts itself everywhere it is read.
//
// Rounding is the tabletop's: half, rounded DOWN. Integer division does that
// for the non-negative sums a declared pool produces, and a pool small enough
// to halve to nothing is an honest zero rather than a special case — the
// component cancels the die exactly, FinalDamage drops a group that nets zero,
// and both sides of the guard are zero.
//
// It refuses rather than no-ops on an empty component set: validateGate
// guarantees a Half gate has damage, so reaching here with nothing rolled means
// that guarantee broke, and halving nothing would hide it.
func halveDamage(
	components []dnd5eEvents.DamageComponent, cause dnd5eEvents.SaveCause, sourceName string,
) ([]dnd5eEvents.DamageComponent, error) {
	if len(components) == 0 {
		return nil, fmt.Errorf(
			"%w: a made save cannot halve damage that was never rolled", ErrBadAction)
	}

	sum := 0
	for _, component := range components {
		sum += component.Total()
	}
	reduction := sum/2 - sum

	return append(components, dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceSpell,
		Roll: dnd5eEvents.RollComponent{
			Source: dnd5eEvents.RollSource{
				Ref:   cloneCoreRef(cause.EffectRef),
				Name:  sourceName,
				Label: halvedBySaveLabel,
			},
			Modifier: &reduction,
		},
		// The first pool's type, which validateGate has already established is
		// the ONLY type: a Half gate over two of them is refused at the door,
		// because FinalDamage groups per type and one reduction can only
		// cancel against one group. The guard below still compares this
		// component's arithmetic against FinalDamage's, so if that door were
		// ever widened without a reduction per type, the delivery would say so
		// rather than quietly deal the wrong number.
		DamageType: components[0].DamageType,
	}), nil
}

// damageCalculation is the roll behind the damage, in the one shape a record
// replays: every component's own trace, and the total they come to.
//
// It is BUILT from the components rather than accumulated alongside them, so
// the total and the faces cannot drift apart — and it is validated here, at the
// only place that can still refuse, because the rulebook's validator is what a
// record will run and failing it there would be a beat nobody can write.
func damageCalculation(
	components []dnd5eEvents.DamageComponent,
) (*dnd5eEvents.RollCalculation, error) {
	calculation := &dnd5eEvents.RollCalculation{
		Components: make([]dnd5eEvents.RollComponent, 0, len(components)),
	}
	for _, component := range components {
		calculation.Components = append(calculation.Components, component.Roll)
		if component.Roll.Dice != nil {
			calculation.Total += component.Roll.Dice.Subtotal
		}
		if component.Roll.Modifier != nil {
			calculation.Total += *component.Roll.Modifier
		}
	}

	if err := dnd5eEvents.ValidateRollCalculation(calculation); err != nil {
		return nil, fmt.Errorf("%w: damage roll trace: %w", ErrBadAction, err)
	}

	// Owned from here on: the components it was built from are the machine's,
	// and an outcome handed outward must not alias them.
	return dnd5eEvents.CloneRollCalculation(calculation), nil
}

// rollContestDamage rolls each declared pool into the component a record can
// read the faces off.
//
// Provenance is the save's cause: the effect ref that raised the save is what
// dealt the damage, and a cause that named none leaves the source ref nil
// rather than borrowing the condition's.
func rollContestDamage(
	ctx context.Context, pools []damage.Damage, roller dice.Roller,
	cause dnd5eEvents.SaveCause, sourceName string,
) ([]dnd5eEvents.DamageComponent, error) {
	components := make([]dnd5eEvents.DamageComponent, 0, len(pools))
	for _, declared := range pools {
		pool, err := dice.ParseNotation(declared.Dice)
		if err != nil {
			return nil, fmt.Errorf("%w: damage dice %q: %w", ErrBadAction, declared.Dice, err)
		}
		result := pool.RollContext(ctx, roller)
		if result.Error() != nil {
			return nil, fmt.Errorf("roll damage: %w", result.Error())
		}
		dieSize, err := parseDieSize(declared.Dice)
		if err != nil {
			return nil, fmt.Errorf("%w: damage dice %q: %w", ErrBadAction, declared.Dice, err)
		}

		// Per-die, not summed, for the same reason a strike records them that
		// way: a downstream reroll addresses dice by index.
		rolls := flattenDice(result.Rolls())
		subtotal := 0
		for _, face := range rolls {
			subtotal += face
		}

		component := dnd5eEvents.DamageComponent{
			Source: dnd5eEvents.DamageSourceSpell,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: cloneCoreRef(cause.EffectRef), Name: sourceName},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      dice.SimplePool(len(rolls), dieSize, 0).Notation(),
					DieSize:       dieSize,
					OriginalRolls: append([]int(nil), rolls...),
					FinalRolls:    append([]int(nil), rolls...),
					Subtotal:      subtotal,
				},
			},
			DamageType: declared.Type,
			Properties: append([]damage.Property(nil), declared.Properties...),
		}
		if declared.FlatBonus != 0 {
			bonus := declared.FlatBonus
			component.Roll.Modifier = &bonus
		}
		components = append(components, component)
	}

	return components, nil
}

// conditionSourceFor answers WHAT SORT OF THING imposed a contested condition,
// read off the cause the caller already stated rather than from a second field
// that could disagree with it.
//
// A cast says so — its cause carries [dnd5eEvents.SaveTriggerSpell] and the
// spell's own ref — and everything else keeps the answer this package has
// always given. Damage is the honest default for the rest: the contest exists
// because something landed on the saver, and the wolf's knockdown is exactly
// that. The condition still names WHICH spell or weapon separately, through the
// source ref it was built with.
func conditionSourceFor(cause dnd5eEvents.SaveCause) dnd5eEvents.ConditionSource {
	if cause.Trigger == dnd5eEvents.SaveTriggerSpell {
		return dnd5eEvents.ConditionSourceSpell
	}

	return dnd5eEvents.ConditionSourceDamage
}

// NewContest returns the machine for one save-gated declaration: a condition,
// damage, or both.
func NewContest(in *ContestInput) Machine { return &contestMachine{in: in} }

type contestMachine struct {
	in       *ContestInput
	cast     *Participants
	prepared preparedCondition

	// hasCondition records whether a condition was declared at all. A contest
	// may be damage-only, and "the zero ConditionApplication" is not something
	// a later step should have to re-derive.
	hasCondition bool
}

func (m *contestMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}
	m.cast = cast

	// Read before the gate is checked, because the gate's own question is what
	// this contest DELIVERS: a Half gate is legal here only when damage is the
	// whole of it. Nothing below this line depends on the order, and every
	// other refusal still fires in the order it always did.
	m.hasCondition = m.in.prepared != nil || m.in.Application.Ref != (core.Ref{})
	if err := validateGate(m.in.Gate, m.shape()); err != nil {
		return nil, err
	}

	if !m.hasCondition && len(m.in.Damage) == 0 && m.in.Removal == nil {
		// Fail closed: a contest that would deliver nothing is a save the
		// player is asked to roll for no reason, and it would look like it
		// worked.
		//
		// A removal counts, which is the same widening slice two made for
		// damage: Application may be nil when the contest declares one of the
		// other two consequences.
		return nil, fmt.Errorf(
			"%w: a contest must declare a condition, damage, or a removal", ErrBadAction)
	}
	if m.in.Removal != nil {
		if err := validateRemoval(m.in.Removal); err != nil {
			return nil, err
		}
	}
	if m.in.Move != nil {
		if err := validateMove(m.in.Move); err != nil {
			return nil, err
		}
	}

	if m.in.prepared != nil {
		m.prepared = *m.in.prepared
	} else if m.hasCondition {
		source := m.in.Application.Ref.String()
		if m.in.Cause.EffectRef != nil {
			source = m.in.Cause.EffectRef.String()
		}
		prepared, err := prepareCondition(m.in.Application, m.in.SaverID, source)
		if err != nil {
			return nil, err
		}
		m.prepared = prepared
	}

	if len(m.in.Damage) > 0 {
		// Preflight, so a malformed pool is refused before the door charges
		// anybody rather than mid-delivery with the save already rolled.
		if err := damage.Validate(m.in.Damage); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBadAction, err)
		}
		// A roll trace is a provenance PAIR, and the rulebook refuses a half-
		// named one. Refusing here means a contest that could not be recorded
		// never rolls, rather than dealing damage and failing to write it down.
		if m.in.Cause.EffectRef == nil || strings.TrimSpace(m.in.SourceName) == "" {
			return nil, fmt.Errorf(
				"%w: contest damage needs the ref and name of what dealt it", ErrBadAction)
		}
	}

	ability, err := m.chooseAbility(cast)
	if err != nil {
		return nil, err
	}
	dc := m.in.Gate.DC.DC(saves.DCInput{DamageTaken: m.in.DamageTaken})
	d20Source, err := m.saveSource()
	if err != nil {
		return nil, err
	}
	if m.in.Roller == nil {
		return nil, fmt.Errorf("%w: a contest rolls with no roller", ErrNoRoller)
	}
	return requestSave(&SaveInput{
		SaverID:   m.in.SaverID,
		Ability:   ability,
		DC:        dc,
		Cause:     m.in.Cause,
		D20Source: d20Source,
		Roller:    m.in.Roller,
	}, func(_ context.Context, save SaveOutcome) (Step, error) {
		return m.resolve(ability, dc, save)
	}), nil
}

func (m *contestMachine) saveSource() (dnd5eEvents.RollSource, error) {
	ref := cloneCoreRef(m.in.Cause.EffectRef)
	name := strings.TrimSpace(m.in.SourceName)
	if ref == nil && m.hasCondition {
		conditionRef := m.prepared.declaration.Ref
		ref = cloneCoreRef(&conditionRef)
	}
	if ref == nil && m.in.Removal != nil {
		parsed, err := core.ParseString(m.in.Removal.Owner.ConditionRef)
		if err != nil {
			return dnd5eEvents.RollSource{}, fmt.Errorf("%w: save source: %v", ErrBadAction, err)
		}
		ref = parsed
	}
	if ref == nil {
		return dnd5eEvents.RollSource{}, fmt.Errorf("%w: contest save needs a source ref", ErrBadAction)
	}
	if name == "" {
		name = ref.ID
	}
	return dnd5eEvents.RollSource{Ref: ref, Name: name, SourceID: m.in.Cause.InstigatorID}, nil
}

func (m *contestMachine) chooseAbility(cast *Participants) (abilities.Ability, error) {
	best := m.in.Gate.Abilities[0]
	bestModifier, err := savingThrowModifier(cast, m.in.SaverID, best)
	if err != nil {
		return "", err
	}
	for _, ability := range m.in.Gate.Abilities[1:] {
		modifier, modErr := savingThrowModifier(cast, m.in.SaverID, ability)
		if modErr != nil {
			return "", modErr
		}
		if modifier > bestModifier {
			best, bestModifier = ability, modifier
		}
	}
	return best, nil
}

// shape reduces what this contest declares to the facts a gate is judged
// against.
//
// A declared MOVE does not disqualify a damage-only contest. The move is what
// the failure costs and a made save never imposes one, so "half" still has
// exactly one thing to halve — which is how Dissonant Whispers saves for half
// AND sends a creature running when it does not.
func (m *contestMachine) shape() contestShape {
	types := make(map[damage.Type]struct{}, len(m.in.Damage))
	for _, pool := range m.in.Damage {
		types[pool.Type] = struct{}{}
	}

	return contestShape{
		damageOnly:  len(m.in.Damage) > 0 && !m.hasCondition && m.in.Removal == nil,
		damageTypes: len(types),
	}
}

// atStake names what this save is against, which is the condition whenever one
// was declared and the damage otherwise.
func (m *contestMachine) atStake() ImposedEffect {
	if m.hasCondition {
		return m.prepared.atStake(m.in.SaverID)
	}
	if len(m.in.Damage) == 0 && m.in.Removal != nil {
		// A removal-only contest is a check to KEEP something, so what is at
		// stake is the owner the failure would end.
		return removalEffect(m.in.Removal.Owner, m.in.Removal.Reason)
	}

	return ImposedEffect{
		Kind:        ImposedDamage,
		Ref:         cloneCoreRef(m.in.Cause.EffectRef),
		Description: describeDamage(m.in.Damage),
		RecipientID: m.in.SaverID,
	}
}

// resolve turns the save into the outcome, and on a failure chains whatever was
// declared: damage first, then the condition, then the removal, then the price,
// then the move.
//
// THE MOVE IS LAST, and that is a rule rather than an ordering accident. Damage
// before the push is ruled by the design (rpg-project#432 §5) — what the damage
// drops is not pushed, and a body sliding across the floor is not a story we
// tell. The condition before the push follows from the same reading: whatever
// the failure put on the creature is on it before it is moved, so a rule that
// has something to say about being moved has already been applied when the
// board comes to walk it.
//
// THE SUCCESS BRANCH DELIVERS, for one gate. A Negated gate still negates
// everything, which is most of them. A Half gate — legal only for a contest
// whose whole consequence is damage — deals half of it, and it does so through
// the SAME continuation the failure branch runs: the apply, then the report,
// then whatever the report came back with. A creature that makes its save
// against a spell it was concentrating through still owes the Constitution
// check, and a success branch that shortcut straight to Done would silently owe
// nothing. There is no condition and no move on that branch: the save was made,
// and half is the only thing a made save costs.
func (m *contestMachine) resolve(ability abilities.Ability, dc int, save SaveOutcome) (Step, error) {
	outcome := ContestOutcome{
		Save:      save,
		DC:        dc,
		Ability:   ability,
		Succeeded: save.Result != nil && save.Result.Success,
		AtStake:   m.atStake(),
	}

	done := func() (Step, error) { return Done{Outcome: outcome}, nil }

	// deliverDamage is the damage phase both branches share: roll it, apply it,
	// say what landed, answer what came back, then tail-call `then`. Named once
	// for the reason reportDamage and runFollowUps are named once — two copies
	// of it would be two places a follow-up could go missing from.
	deliverDamage := func(halved bool, then func(context.Context) (Step, error)) (Step, error) {
		return applyPreparedDamage(
			m.in.Damage, m.in.Roller, m.in.Cause, m.in.SourceName, m.cast, m.in.SaverID, halved,
			func(applied ImposedEffect) (Step, error) {
				outcome.Imposed = append(outcome.Imposed, applied)

				// Say what landed, then answer what came back — the same two
				// steps the strike calls, in the same order, right after the
				// apply. A third damage source gets concentration by calling
				// them too, which is the whole reason they are named once.
				return reportDamage(reportDamageInput{
					MemberID:      m.in.SaverID,
					Amount:        applied.Amount,
					DamageType:    primaryComponentType(applied.Components),
					DroppedToZero: applied.Before > 0 && applied.After == 0,
					Cause:         m.in.Cause,
				}, func(ctx context.Context, ups []dnd5eEvents.FollowUp) (Step, error) {
					return runFollowUps(ctx, ups, 0, m.in.Roller,
						func(followUp FollowUpOutcome) {
							outcome.FollowUps = append(outcome.FollowUps, followUp)
						},
						then,
					)
				}), nil
			},
		), nil
	}

	if outcome.Succeeded {
		if m.in.Gate.OnSuccess != saves.Half {
			return done()
		}

		return deliverDamage(true, func(context.Context) (Step, error) { return done() })
	}

	deliverMove := func() (Step, error) {
		if m.in.Move == nil {
			return done()
		}

		return imposeMove(*m.in.Move, m.in.Cause, m.cast, m.in.SaverID,
			func(directed ImposedEffect) (Step, error) {
				outcome.Imposed = append(outcome.Imposed, directed)
				return done()
			}, done), nil
	}

	payTheMove := func() (Step, error) {
		if m.in.Move == nil || m.in.Move.Pays == combatActions.PaysNothing {
			return deliverMove()
		}

		return payForMove(*m.in.Move, m.in.Cause, m.cast, m.in.SaverID,
			deliverMove,
			func(unpaid ImposedEffect) (Step, error) {
				outcome.Imposed = append(outcome.Imposed, unpaid)
				return done()
			}, done), nil
	}

	deliverRemoval := func() (Step, error) {
		if m.in.Removal == nil {
			return payTheMove()
		}

		return publishRemoval(m.in.Removal, func(stripped ImposedEffect) (Step, error) {
			outcome.Imposed = append(outcome.Imposed, stripped)
			return payTheMove()
		}), nil
	}

	deliverCondition := func() (Step, error) {
		if !m.hasCondition {
			return deliverRemoval()
		}

		return publishPreparedCondition(
			m.prepared, m.cast, m.in.SaverID, conditionSourceFor(m.in.Cause),
			func(replaced []ImposedEffect) (Step, error) {
				// Replacement before application, in the trace as on the bus:
				// the record reads in the order the rules happened, and a
				// reader that met the new condition first would be reading an
				// instant where the creature held both.
				outcome.Imposed = append(outcome.Imposed, replaced...)
				outcome.Imposed = append(outcome.Imposed, m.prepared.atStake(m.in.SaverID))
				return deliverRemoval()
			},
		), nil
	}

	if len(m.in.Damage) == 0 {
		return deliverCondition()
	}

	return deliverDamage(false, func(context.Context) (Step, error) { return deliverCondition() })
}

// validateRemoval refuses a removal that names nothing to end.
//
// Fail closed: a contest whose failure strips an empty owner address would ask
// somebody to roll and then quietly do nothing, which is exactly the
// affordance-with-nothing-behind-it this stack keeps finding.
func validateRemoval(removal *ConditionRemoval) error {
	if removal.Owner.MemberID == "" || removal.Owner.ConditionRef == "" {
		return fmt.Errorf("%w: a contest removal must name the owner it ends", ErrBadAction)
	}
	if strings.TrimSpace(removal.Reason) == "" {
		return fmt.Errorf("%w: a contest removal must say why", ErrBadAction)
	}
	return nil
}

// removalEffect is one stripped address, as the record reads it.
func removalEffect(address dnd5eEvents.ChildRef, reason string) ImposedEffect {
	effect := ImposedEffect{
		Kind:        ImposedConditionRemoved,
		Description: fmt.Sprintf("%s ended (%s)", address.ConditionRef, reason),
		RecipientID: address.MemberID,
		Address:     address,
	}
	if ref, err := core.ParseString(address.ConditionRef); err == nil {
		effect.Ref = ref
	}

	return effect
}

func savingThrowModifier(cast *Participants, saverID string, ability abilities.Ability) (int, error) {
	if saver, ok := cast.Character(saverID); ok {
		return saver.GetSavingThrowModifier(ability), nil
	}
	if saver, ok := cast.Monster(saverID); ok {
		return saver.GetSavingThrowModifier(ability), nil
	}
	return 0, fmt.Errorf("%w: %q", ErrNoSaver, saverID)
}

func (p *Participants) entity(id string) (core.Entity, error) {
	if character, ok := p.Character(id); ok {
		return character, nil
	}
	if monster, ok := p.Monster(id); ok {
		return monster, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrNoSaver, id)
}

func requestSave(in *SaveInput, next func(context.Context, SaveOutcome) (Step, error)) Request {
	return Request{
		name:    "saving throw",
		machine: NewSave(in),
		next: func(ctx context.Context, out Outcome) (Step, error) {
			save, ok := out.(SaveOutcome)
			if !ok {
				return nil, fmt.Errorf("%w: saving throw returned %T", ErrBadStep, out)
			}
			return next(ctx, save)
		},
	}
}
