// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// SneakAttackData is the JSON structure for persisting sneak attack condition state.
//
// UsedThisTurn is persisted (not just runtime) because each Encounter.TakeAction
// RPC call goes through LoadFromData → fresh condition instance from JSON. Without
// persisting the once-per-turn flag, a rogue would sneak-attack on every TakeAction
// call, breaking the once-per-turn semantic across separate RPCs in the same turn.
// See rpg-toolkit#654 for the broader sustainable-per-turn-state pattern.
type SneakAttackData struct {
	Ref          *core.Ref `json:"ref"`
	CharacterID  string    `json:"member_id"`
	Level        int       `json:"level"`
	DamageDice   int       `json:"damage_dice"`
	UsedThisTurn bool      `json:"used_this_turn"`
}

// SneakAttackCondition represents the rogue's sneak attack feature.
// It adds extra damage dice when the rogue has advantage or an ally adjacent to the target.
// It implements the ConditionBehavior interface.
type SneakAttackCondition struct {
	CharacterID     string
	Level           int
	DamageDice      int  // Number of d6s to roll
	UsedThisTurn    bool // Sneak attack can only be used once per turn
	subscriptionIDs []string
	bus             events.EventBus
	roller          dice.Roller
}

// stateChanged reports that the once-per-turn flag moved. See
// [publishStateChanged].
func (s *SneakAttackCondition) stateChanged(ctx context.Context) error {
	return publishStateChanged(ctx, s.bus, s.CharacterID, s.Ref())
}

// Ensure SneakAttackCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*SneakAttackCondition)(nil)

// Ensure SneakAttackCondition can receive a runtime roller.
var _ RollerBinder = (*SneakAttackCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on. Sneak Attack's canonical ref
// lives under refs.Features (it is a rogue feature turned active condition),
// so that is what Ref reports.
func (s *SneakAttackCondition) Ref() *core.Ref { return refs.Features.SneakAttack() }

// BindRoller binds the roller this condition rolls its extra damage dice
// with, so a condition restored from persisted JSON — whose loader has no
// roller to give it — rolls the interaction's dice instead of a
// process-global default. A nil roller leaves the current one alone.
func (s *SneakAttackCondition) BindRoller(roller dice.Roller) {
	if roller == nil {
		return
	}
	s.roller = roller
}

// SneakAttackInput provides configuration for creating a sneak attack condition
type SneakAttackInput struct {
	MemberID string      // ID of the rogue
	Level    int         // Rogue level (determines number of dice)
	Roller   dice.Roller // Dice roller for rolling extra damage
}

// NewSneakAttackCondition creates a sneak attack condition from input
func NewSneakAttackCondition(input SneakAttackInput) *SneakAttackCondition {
	return &SneakAttackCondition{
		CharacterID: input.MemberID,
		Level:       input.Level,
		DamageDice:  calculateSneakAttackDice(input.Level),
		roller:      input.Roller,
	}
}

// calculateSneakAttackDice determines number of d6s based on rogue level
// Sneak Attack starts at 1d6 at level 1 and increases by 1d6 every odd level
func calculateSneakAttackDice(level int) int {
	if level < 1 {
		return 0
	}
	return (level + 1) / 2 // 1d6 at 1, 2d6 at 3, 3d6 at 5, etc.
}

// IsApplied returns true if this condition is currently applied
func (s *SneakAttackCondition) IsApplied() bool {
	return s.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (s *SneakAttackCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if s.bus != nil {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "sneak attack condition already applied")
	}

	s.bus = bus

	// Subscribe to damage chain to add sneak attack dice
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID, err := damageChain.SubscribeWithChain(ctx, s.onDamageChain)
	if err != nil {
		s.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	s.subscriptionIDs = append(s.subscriptionIDs, subID)

	// Subscribe to turn end to reset the once-per-turn flag
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(bus)
	turnSubID, err := turnEndTopic.Subscribe(ctx, s.onTurnEnd)
	if err != nil {
		_ = s.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn end")
	}
	s.subscriptionIDs = append(s.subscriptionIDs, turnSubID)

	restTopic := dnd5eEvents.RestTopic.On(bus)
	restSubID, err := restTopic.Subscribe(ctx, s.onRest)
	if err != nil {
		_ = s.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to long rest")
	}
	s.subscriptionIDs = append(s.subscriptionIDs, restSubID)

	return nil
}

// Remove unsubscribes this condition from events
func (s *SneakAttackCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if s.bus == nil {
		return nil
	}

	total := len(s.subscriptionIDs)
	var errs []error
	for _, id := range s.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", id, err))
		}
	}

	s.subscriptionIDs = nil
	s.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// onTurnEnd resets the once-per-turn flag
func (s *SneakAttackCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	// Only when the flag actually changes. Marking unconditionally would
	// flag every rogue dirty at the end of every turn they did not sneak
	// attack, and a turn boundary is about to become an interaction that
	// runs for every participant.
	if event.SubjectID == s.CharacterID && s.UsedThisTurn {
		s.UsedThisTurn = false

		return s.stateChanged(ctx)
	}
	return nil
}

// onRest clears a spent Sneak Attack on its owner's long rest. A short rest
// does not reset this turn-scoped meter, and an already-clear meter publishes
// no state change.
func (s *SneakAttackCondition) onRest(ctx context.Context, event dnd5eEvents.RestEvent) error {
	if event.CharacterID != s.CharacterID || event.RestType != coreResources.ResetLongRest || !s.UsedThisTurn {
		return nil
	}

	s.UsedThisTurn = false
	return s.stateChanged(ctx)
}

// onDamageChain adds sneak attack dice when sneakAttackRule applies to the
// event's frame — the same rule information asks. The handler keeps only what
// execution owns: rolling the dice and the once-per-turn transition. An
// invalid frame or a Depends answer fails the fold.
func (s *SneakAttackCondition) onDamageChain(
	ctx context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	executed, err := executeRule(&executeRuleInput{Name: "sneak attack", Rule: s.rule(), Frame: event.Frame})
	if err != nil {
		return c, err
	}
	if executed.Answer.Decision.Applicability != contributions.Applies {
		return c, nil
	}

	// Roll sneak attack dice (use default roller if none configured, e.g., after JSON load)
	roller := s.roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	rolls := 1
	if event.IsCritical {
		rolls++
	}
	var sneakDice []int
	for range rolls {
		rolled, err := roller.RollN(ctx, s.DamageDice, 6)
		if err != nil {
			return c, rpgerr.Wrap(err, "failed to roll sneak attack dice")
		}
		sneakDice = append(sneakDice, rolled...)
	}

	// Add sneak attack damage component using DamageSourceFeature
	modifyDamage := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		subtotal := 0
		for _, face := range sneakDice {
			subtotal += face
		}
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source: dnd5eEvents.DamageSourceFeature,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{
					Ref:      refs.Features.SneakAttack(),
					Name:     "Sneak Attack",
					SourceID: s.CharacterID,
				},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      dice.SimplePool(len(sneakDice), 6, 0).Notation(),
					DieSize:       6,
					OriginalRolls: sneakDice,
					FinalRolls:    slices.Clone(sneakDice),
					Subtotal:      subtotal,
				},
			},
			DamageType: e.WeaponDamageType, // Sneak attack uses the marked primary weapon type
			IsCritical: event.IsCritical,
		})
		return e, nil
	}

	// Mark as used this turn
	s.UsedThisTurn = true
	if err := s.stateChanged(ctx); err != nil {
		return c, err
	}

	err = c.Add(combat.StageFeatures, "sneak_attack", modifyDamage)
	if err != nil {
		return c, rpgerr.Wrap(err, "failed to add sneak attack modifier")
	}

	return c, nil
}

var _ contributions.ActionAssessor = (*SneakAttackCondition)(nil)

// AssessAction answers whether Sneak Attack applies to the framed attack. It
// reads the frame and this condition's own owner, once-per-turn flag and dice;
// it never rolls or spends.
func (s *SneakAttackCondition) AssessAction(
	in *contributions.AssessActionInput,
) (*contributions.AssessActionOutput, error) {
	return s.rule().AssessAction(in)
}

func (s *SneakAttackCondition) rule() sneakAttackRule {
	return sneakAttackRule{owner: s.CharacterID, usedThisTurn: s.UsedThisTurn, dice: s.DamageDice}
}

// sneakAttackRule holds only the facts Sneak Attack's predicate uses.
//
// Note whose enemy. RAW is "another enemy of the target is within 5 feet of
// it" — the relation is measured from the TARGET's point of view, not the
// attacker's, which is why the rule reads target→X pairs. With three factions
// a hobgoblin beside the duergar you are stabbing is an enemy of your target
// and enables this, and it is nobody's ally. The attacker's own adjacency
// never counts.
//
// The Dexterity test is the known defect rpg-toolkit#1929 (RAW asks for a
// finesse or ranged weapon), kept as it is.
type sneakAttackRule struct {
	owner        string
	usedThisTurn bool
	dice         int
}

func (r sneakAttackRule) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "sneak attack")
	if err != nil {
		return nil, err
	}
	answer := func(state contributions.Applicability, reason string) *contributions.AssessActionOutput {
		out := &contributions.AssessActionOutput{Answer: contributions.Answer{
			Decision:      contributions.Decision{Applicability: state, Reason: reason},
			Participation: contributions.ContributesNow,
		}}
		if state == contributions.Applies {
			out.Answer.Benefit = fmt.Sprintf("+%dd6 damage", r.dice)
		}
		return out
	}

	if frame.Actor != r.owner {
		return answer(contributions.DoesNotApply, "Sneak Attack adds to its owner's attacks"), nil
	}
	if r.usedThisTurn {
		return answer(contributions.DoesNotApply, "Already used this turn"), nil
	}
	ability, abilityKnown := frame.Action.Ability.Get()
	if abilityKnown && ability != abilities.DEX {
		return answer(contributions.DoesNotApply, "Sneak Attack requires a Dexterity attack"), nil
	}
	weapon, weaponKnown := frame.Action.WeaponPool.Get()
	if weaponKnown && !weapon {
		return answer(contributions.DoesNotApply, "Sneak Attack requires a weapon attack"), nil
	}
	target, targetKnown := frame.Target.Get()
	if !targetKnown {
		return answer(contributions.Depends, "Depends on the target"), nil
	}
	if !abilityKnown || !weaponKnown {
		return answer(contributions.Depends, "Depends on the attack's weapon and ability"), nil
	}
	if advantage, known := frame.Action.Advantage.Get(); known && advantage {
		return answer(contributions.Applies, "The attack has advantage"), nil
	}

	// Look for a proven enemy of the target beside it. A pair that cannot be
	// ruled out — an unknown distance or stance that could still qualify —
	// keeps the answer open; only a complete frame can prove there is none.
	open := false
	for _, pair := range frame.Pairs {
		if pair.From != target || pair.To == frame.Actor || pair.To == target {
			continue
		}
		distance, distanceKnown := pair.DistanceCells.Get()
		stance, stanceKnown := pair.Stance.Get()
		if distanceKnown && distance > combat.AdjacentCells {
			continue
		}
		if stanceKnown && stance != contributions.StanceHostile {
			continue
		}
		if distanceKnown && stanceKnown {
			return answer(contributions.Applies, "Another enemy of the target is within 5 feet"), nil
		}
		open = true
	}
	if advantage, known := frame.Action.Advantage.Get(); frame.Complete && known && !advantage && !open {
		return answer(contributions.DoesNotApply, "No advantage and no other enemy of the target within 5 feet"), nil
	}
	return answer(contributions.Depends, "Needs advantage or another enemy of the target within 5 feet"), nil
}

// ToJSON converts the condition to JSON for persistence
func (s *SneakAttackCondition) ToJSON() (json.RawMessage, error) {
	data := SneakAttackData{
		Ref:          refs.Features.SneakAttack(),
		CharacterID:  s.CharacterID,
		Level:        s.Level,
		DamageDice:   s.DamageDice,
		UsedThisTurn: s.UsedThisTurn,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to marshal sneak attack data")
	}

	return bytes, nil
}

// loadJSON loads the condition from JSON
func (s *SneakAttackCondition) loadJSON(data json.RawMessage) error {
	var sneakData SneakAttackData
	if err := json.Unmarshal(data, &sneakData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal sneak attack data")
	}

	s.CharacterID = sneakData.CharacterID
	s.Level = sneakData.Level
	s.DamageDice = sneakData.DamageDice
	s.UsedThisTurn = sneakData.UsedThisTurn

	return nil
}
