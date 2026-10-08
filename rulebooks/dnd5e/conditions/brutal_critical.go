// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// diceNotationRegex matches simple dice notation like "1d8", "2d6", etc.
var diceNotationRegex = regexp.MustCompile(`^(\d*)[dD](\d+)`)

// BrutalCriticalData is the JSON structure for persisting brutal critical
// condition state. No barbarian level and no dice count is stored; a blob saved
// with the old "level" or "extra_dice" keys loads and the copy is ignored.
type BrutalCriticalData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
}

// BrutalCriticalCondition represents the barbarian's brutal critical feature.
// It adds extra weapon damage dice on critical hits based on the attacker's
// barbarian levels, read from the frame each time an attack asks.
// It implements the ConditionBehavior interface.
type BrutalCriticalCondition struct {
	MemberID        string
	subscriptionIDs []string
	bus             events.EventBus
	roller          dice.Roller
}

// Ensure BrutalCriticalCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*BrutalCriticalCondition)(nil)

// Ensure BrutalCriticalCondition can receive a runtime roller.
var _ RollerBinder = (*BrutalCriticalCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (b *BrutalCriticalCondition) Ref() *core.Ref { return refs.Conditions.BrutalCritical() }

// BrutalCriticalInput provides configuration for creating a brutal critical
// condition. It takes no level: the dice are read from the attacker's
// barbarian levels in the frame when an attack asks.
type BrutalCriticalInput struct {
	MemberID string      // ID of the barbarian
	Roller   dice.Roller // Dice roller for rolling extra damage
}

// NewBrutalCriticalCondition creates a brutal critical condition from input
func NewBrutalCriticalCondition(input BrutalCriticalInput) *BrutalCriticalCondition {
	return &BrutalCriticalCondition{
		MemberID: input.MemberID,
		roller:   input.Roller,
	}
}

// brutalCriticalDice is the number of extra weapon dice the framed attacker's
// Brutal Critical adds: one at barbarian 9, two at 13, three at 17, none
// below 9. Unknown class levels or zero barbarian levels are refused.
func brutalCriticalDice(frame contributions.Frame) (int, error) {
	level, err := actorClassLevel(frame, classes.Barbarian, "brutal critical")
	if err != nil {
		return 0, err
	}
	return calculateExtraDice(level), nil
}

// calculateExtraDice determines extra weapon dice based on barbarian level
func calculateExtraDice(level int) int {
	switch {
	case level >= 17:
		return 3
	case level >= 13:
		return 2
	case level >= 9:
		return 1
	default:
		return 0
	}
}

// BindRoller binds the roller this condition rolls its extra critical dice
// with, so a condition restored from persisted JSON — whose loader has no
// roller to give it — rolls the interaction's dice instead of a
// process-global default. A nil roller leaves the current one alone.
func (b *BrutalCriticalCondition) BindRoller(roller dice.Roller) {
	if roller == nil {
		return
	}
	b.roller = roller
}

// IsApplied returns true if this condition is currently applied
func (b *BrutalCriticalCondition) IsApplied() bool {
	return b.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (b *BrutalCriticalCondition) Apply(ctx context.Context, bus events.EventBus) error {
	b.bus = bus

	// Subscribe to damage chain to add extra dice on crits
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID, err := damageChain.SubscribeWithChain(ctx, b.onDamageChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	b.subscriptionIDs = append(b.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events
func (b *BrutalCriticalCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if b.bus == nil {
		return nil // Not applied, nothing to remove
	}

	total := len(b.subscriptionIDs)
	var errs []error
	for _, subID := range b.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	b.subscriptionIDs = nil
	b.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (b *BrutalCriticalCondition) ToJSON() (json.RawMessage, error) {
	data := BrutalCriticalData{
		Ref:      refs.Conditions.BrutalCritical(),
		MemberID: b.MemberID,
	}
	return json.Marshal(data)
}

// loadJSON loads brutal critical condition state from JSON
func (b *BrutalCriticalCondition) loadJSON(data json.RawMessage) error {
	var bcData BrutalCriticalData
	if err := json.Unmarshal(data, &bcData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal brutal critical data")
	}

	b.MemberID = bcData.MemberID

	return nil
}

var _ contributions.ActionAssessor = (*BrutalCriticalCondition)(nil)

// AssessAction answers whether Brutal Critical bears on the framed attack. It
// applies to its holder's weapon attacks and states that its dice join only a
// critical hit; it never predicts whether the roll will be one.
func (b *BrutalCriticalCondition) AssessAction(
	in *contributions.AssessActionInput,
) (*contributions.AssessActionOutput, error) {
	return b.rule().AssessAction(in)
}

func (b *BrutalCriticalCondition) rule() brutalCriticalRule {
	return brutalCriticalRule{owner: b.MemberID}
}

// brutalCriticalRule holds only the facts Brutal Critical's predicate uses.
// Whether the hit is critical is the swing's outcome, not a fact of the
// action, so it is execution's moment to add the dice — the way Sneak Attack
// doubles its own on a critical — and not part of this answer.
type brutalCriticalRule struct {
	owner string
}

func (r brutalCriticalRule) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "brutal critical")
	if err != nil {
		return nil, err
	}
	if frame.Actor != r.owner {
		return assessed(contributions.DoesNotApply, "Brutal Critical affects only its holder's attacks"), nil
	}
	extraDice, err := brutalCriticalDice(frame)
	if err != nil {
		return nil, err
	}
	if extraDice == 0 {
		return assessed(contributions.DoesNotApply, "Brutal Critical adds dice from 9th level"), nil
	}
	if roll, _ := frame.Action.Roll.Get(); roll != contributions.RollKindAttack {
		return assessed(contributions.DoesNotApply, "Brutal Critical adds only to weapon attacks"), nil
	}
	weapon, known := frame.Action.WeaponPool.Get()
	if !known {
		return assessed(contributions.Depends, "Depends on the attack's weapon"), nil
	}
	if !weapon {
		return assessed(contributions.DoesNotApply, "Brutal Critical requires a weapon damage die"), nil
	}
	out := assessed(contributions.Applies, "The attack has a weapon damage die")
	noun := "die"
	if extraDice != 1 {
		noun = "dice"
	}
	out.Answer.Benefit = fmt.Sprintf("+%d weapon damage %s on a critical hit", extraDice, noun)
	return out, nil
}

// onDamageChain adds extra weapon damage dice on a critical hit when
// brutalCriticalRule applies to the event's frame — the same rule information
// asks. An invalid frame or a Depends answer fails the fold.
func (b *BrutalCriticalCondition) onDamageChain(
	_ context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	executed, err := executeRule(&executeRuleInput{Name: "brutal critical", Rule: b.rule(), Frame: event.Frame})
	if err != nil {
		return c, err
	}
	if executed.Answer.Decision.Applicability != contributions.Applies || !event.IsCritical {
		return c, nil
	}
	extraDice, err := brutalCriticalDice(event.Frame)
	if err != nil {
		return c, err
	}

	// Parse marked weapon damage notation to get die size (e.g., "1d8" -> 8)
	dieSize, err := parseDieSize(event.WeaponDamageDice)
	if err != nil {
		return c, rpgerr.Wrapf(err, "failed to parse weapon damage notation: %s", event.WeaponDamageDice)
	}

	if dieSize == 0 {
		return c, nil // No dice to roll (shouldn't happen with valid weapons)
	}

	// Add brutal critical modifier at StageFeatures
	modifyDamage := func(modCtx context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		// Roll extra dice
		roller := b.roller
		if roller == nil {
			roller = dice.NewRoller()
		}

		extraRolls, rollErr := roller.RollN(modCtx, extraDice, dieSize)
		if rollErr != nil {
			return e, rpgerr.Wrap(rollErr, "failed to roll brutal critical dice")
		}

		subtotal := 0
		for _, face := range extraRolls {
			subtotal += face
		}

		// Append brutal critical damage component. The trace records the dice
		// actually rolled — a critical's extra dice — so the notation describes
		// the physical pool, not the weapon's printed expression.
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source: dnd5eEvents.DamageSourceFeature,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{
					Ref:      refs.Features.BrutalCritical(),
					Name:     "Brutal Critical",
					SourceID: b.MemberID,
				},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      dice.SimplePool(len(extraRolls), dieSize, 0).Notation(),
					DieSize:       dieSize,
					OriginalRolls: extraRolls,
					FinalRolls:    slices.Clone(extraRolls),
					Subtotal:      subtotal,
				},
			},
			DamageType: e.WeaponDamageType,
			IsCritical: false,
		})
		return e, nil
	}

	err = c.Add(combat.StageFeatures, "brutal_critical", modifyDamage)
	if err != nil {
		return c, rpgerr.Wrapf(err, "failed to add brutal critical modifier for character %s", b.MemberID)
	}

	return c, nil
}

// parseDieSize extracts the die size from a dice notation string (e.g., "1d8" -> 8)
func parseDieSize(notation string) (int, error) {
	matches := diceNotationRegex.FindStringSubmatch(notation)
	if len(matches) < 3 {
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid dice notation: %s", notation)
	}

	dieSize, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, rpgerr.Wrapf(err, "invalid die size in notation: %s", notation)
	}

	return dieSize, nil
}
