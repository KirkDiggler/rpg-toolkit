// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// AttackContext carries the complete state of a phase-1 (hit determination)
// attack through the RPC boundary to phase 2 (damage application).
//
// This is the value returned by ResolveAttackHit and consumed by
// ApplyAttackOutcome. The orchestrator (rpg-api) stores it in the encounter
// snapshot while awaiting player reaction choices.
type AttackContext struct {
	// Identity
	AttackerID string
	TargetID   string
	Weapon     *weapons.Weapon

	// Original state (before any reactions)
	OriginalAC int // Target AC before any reaction modifiers
	WouldHit   bool // Whether roll hits against originalAC

	// Roll details (needed by phase 2 to re-evaluate hit)
	AttackRoll      int   // The d20 result
	AttackBonus     int   // Total bonus applied
	TotalAttack     int   // Roll + bonus
	IsNaturalTwenty bool  // Natural 20 always hits regardless of AC
	IsNaturalOne    bool  // Natural 1 always misses regardless of AC
	AllRolls        []int // All d20 rolls (2 for adv/disadv, 1 otherwise)

	// Advantage/disadvantage (carried to phase 2 for damage chain context)
	HasAdvantage    bool
	HasDisadvantage bool

	// Chain outputs (carried to phase 2 for damage chain)
	CriticalThreshold int // Roll >= this value is a critical hit (default 20)

	// Side effects from phase 1
	ReactionsConsumed []dnd5eEvents.ReactionConsumption

	// Internal fields needed by ApplyAttackOutcome to reconstruct damage context.
	// These are unexported from the package perspective but accessed within the same package.
	abilityMod      int
	abilityUsed     abilities.Ability
	isOffHandAttack bool
	eventBus        events.EventBus
	roller          dice.Roller
}

// ReactionModifier represents an AC or roll modification chosen by a player
// reactor between phase 1 and phase 2.
//
// Each modifier maps to one player reaction decision (e.g. the Shield spell
// adding +5 AC). The orchestrator (rpg-api) translates the player's choice
// into one or more ReactionModifiers and passes them to ApplyAttackOutcome.
type ReactionModifier struct {
	// ConditionRef identifies the reaction that produced this modifier.
	// Matches the ref used to seed gamectx.ReactionReadinessMap.
	ConditionRef string

	// ACBonus is the AC increase to apply to the target (typically +5 for Shield).
	// Zero means no AC modification.
	ACBonus int
}

// ResolveAttackHitInput provides parameters for the first discrete attack phase.
// It carries the same fields as AttackInput; it exists as a separate type so the
// API is unambiguous and the two-phase split is visible at call sites.
type ResolveAttackHitInput struct {
	// AttackerID is the combatant performing the attack.
	AttackerID string

	// TargetID is the combatant being attacked.
	TargetID string

	// Weapon is the weapon being used for the attack.
	Weapon *weapons.Weapon

	// EventBus is required for publishing attack/damage events.
	EventBus events.EventBus

	// Roller is the dice roller for attack rolls.
	// If nil, a default roller is used.
	Roller dice.Roller

	// AttackHand indicates which hand is making the attack.
	AttackHand AttackHand

	// AttackType indicates whether this is a standard or opportunity attack.
	AttackType dnd5eEvents.AttackType
}

// Validate validates the input fields.
func (r *ResolveAttackHitInput) Validate() error {
	if r == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "ResolveAttackHitInput is nil")
	}
	if r.AttackerID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "AttackerID is required")
	}
	if r.TargetID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "TargetID is required")
	}
	if r.Weapon == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "Weapon is nil")
	}
	if r.EventBus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "EventBus is nil")
	}
	return nil
}

// ApplyAttackOutcomeInput provides parameters for the second discrete attack phase.
type ApplyAttackOutcomeInput struct {
	// HitResult is the AttackContext returned by ResolveAttackHit.
	HitResult *AttackContext

	// Reactions is the list of modifier decisions made by reactors between
	// phase 1 and phase 2. May be empty (no reactions, or all auto-resolved).
	Reactions []ReactionModifier
}

// Validate validates the input fields.
func (a *ApplyAttackOutcomeInput) Validate() error {
	if a == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "ApplyAttackOutcomeInput is nil")
	}
	if a.HitResult == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "HitResult is nil")
	}
	return nil
}

// ResolveAttackHit executes phase 1 of a discrete two-phase attack resolution.
//
// Phase 1 runs the full attack chain (collecting advantage/disadvantage, attack
// bonuses, and critical thresholds), rolls the d20, and evaluates hit/miss
// against the target's original AC. It returns an AttackContext that the
// orchestrator stores between the two phases.
//
// Conditions may publish a ReactionTriggerEvent to the EventBus during phase 1
// when their predicate matches AND gamectx.IsReactionReady returns true for the
// reactor. The orchestrator reads these events after this call returns to decide
// whether to push reaction prompts to players.
//
// The chain itself always runs to completion — there is no in-process pause.
// The "reaction window" lives at the RPC boundary between phase 1 and phase 2.
//
//nolint:gocyclo // Attack resolution requires orchestrating multiple game rules stages
func ResolveAttackHit(ctx context.Context, input *ResolveAttackHitInput) (*AttackContext, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	// Look up attacker from context
	attacker, err := GetCombatantFromContext(ctx, input.AttackerID)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to look up attacker %s", input.AttackerID)
	}

	// Look up defender from context
	defender, err := GetCombatantFromContext(ctx, input.TargetID)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to look up defender %s", input.TargetID)
	}

	attackerScores := attacker.AbilityScores()
	proficiencyBonus := attacker.ProficiencyBonus()
	defenderAC := GetEffectiveAC(ctx, defender)

	isOffHandAttack := input.AttackHand == AttackHandOff
	if isOffHandAttack {
		if err := validateOffHandAttack(ctx, &AttackInput{
			AttackerID: input.AttackerID,
			TargetID:   input.TargetID,
			Weapon:     input.Weapon,
			EventBus:   input.EventBus,
			Roller:     input.Roller,
			AttackHand: input.AttackHand,
			AttackType: input.AttackType,
		}); err != nil {
			return nil, err
		}
	}

	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	abilityMod := calculateAttackAbilityModifier(input.Weapon, attackerScores)
	baseBonus := abilityMod + proficiencyBonus

	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID:          input.AttackerID,
		TargetID:            input.TargetID,
		WeaponRef:           weaponToRef(input.Weapon),
		IsMelee:             !input.Weapon.IsRanged(),
		AttackType:          resolveAttackType(input.AttackType),
		AdvantageSources:    nil,
		DisadvantageSources: nil,
		CancellationSources: nil,
		AttackBonus:         baseBonus,
		TargetAC:            defenderAC,
		CriticalThreshold:   20,
		ReactionsConsumed:   nil,
	}

	// Build and execute attack chain (phase 1 chain runs end-to-end)
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(input.EventBus)

	modifiedAttackChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to publish attack chain")
	}

	finalAttackEvent, err := modifiedAttackChain.Execute(ctx, attackEvent)
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to execute attack chain")
	}

	// Determine advantage/disadvantage and roll
	hasAdvantage := len(finalAttackEvent.AdvantageSources) > 0
	hasDisadvantage := len(finalAttackEvent.DisadvantageSources) > 0

	var attackRoll int
	var allRolls []int

	switch {
	case hasAdvantage && hasDisadvantage:
		attackRoll, err = roller.Roll(ctx, 20)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to roll attack")
		}
		allRolls = []int{attackRoll}
		hasAdvantage = false
		hasDisadvantage = false
	case hasAdvantage:
		allRolls, err = roller.RollN(ctx, 2, 20)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to roll attack with advantage")
		}
		attackRoll = max(allRolls[0], allRolls[1])
	case hasDisadvantage:
		allRolls, err = roller.RollN(ctx, 2, 20)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to roll attack with disadvantage")
		}
		attackRoll = min(allRolls[0], allRolls[1])
	default:
		attackRoll, err = roller.Roll(ctx, 20)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to roll attack")
		}
		allRolls = []int{attackRoll}
	}

	totalAttack := attackRoll + finalAttackEvent.AttackBonus
	isNatural20 := attackRoll == 20
	isNatural1 := attackRoll == 1

	// Determine hit against originalAC (before any reactions modify it)
	var wouldHit bool
	switch {
	case isNatural1:
		wouldHit = false
	case isNatural20:
		wouldHit = true
	default:
		wouldHit = totalAttack >= defenderAC
	}

	// Publish ReactionUsedEvents for reactions consumed during phase 1 chain
	if len(finalAttackEvent.ReactionsConsumed) > 0 {
		reactionTopic := dnd5eEvents.ReactionUsedTopic.On(input.EventBus)
		for _, reaction := range finalAttackEvent.ReactionsConsumed {
			if pubErr := reactionTopic.Publish(ctx, dnd5eEvents.ReactionUsedEvent(reaction)); pubErr != nil {
				return nil, rpgerr.Wrap(pubErr, "failed to publish reaction used event")
			}
		}
	}

	return &AttackContext{
		AttackerID:        input.AttackerID,
		TargetID:          input.TargetID,
		Weapon:            input.Weapon,
		OriginalAC:        defenderAC,
		WouldHit:          wouldHit,
		AttackRoll:        attackRoll,
		AttackBonus:       finalAttackEvent.AttackBonus,
		TotalAttack:       totalAttack,
		IsNaturalTwenty:   isNatural20,
		IsNaturalOne:      isNatural1,
		AllRolls:          allRolls,
		HasAdvantage:      hasAdvantage,
		HasDisadvantage:   hasDisadvantage,
		CriticalThreshold: finalAttackEvent.CriticalThreshold,
		ReactionsConsumed: finalAttackEvent.ReactionsConsumed,
		abilityMod:        abilityMod,
		abilityUsed:       determineAbilityUsed(input.Weapon, attackerScores),
		isOffHandAttack:   isOffHandAttack,
		eventBus:          input.EventBus,
		roller:            roller,
	}, nil
}

// ApplyAttackOutcome executes phase 2 of a discrete two-phase attack resolution.
//
// Phase 2 takes the AttackContext from phase 1 plus any ReactionModifiers
// chosen by players between the phases. It:
//  1. Recomputes effective AC = originalAC + sum(modifier.ACBonus)
//  2. Re-evaluates hit/miss against effective AC (natural 1/20 rules still apply)
//  3. If the attack still hits, runs the damage chain end-to-end and publishes
//     DamageReceivedEvent
//  4. If the attack misses (e.g., Shield turned a hit into a miss), returns a
//     result with Hit=false and no damage
//
// Passing an empty Reactions slice produces identical output to the original
// monolithic ResolveAttack for the same roll + AC combination.
//
//nolint:gocyclo // Attack resolution requires orchestrating multiple game rules stages
func ApplyAttackOutcome(ctx context.Context, input *ApplyAttackOutcomeInput) (*AttackResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	ac := input.HitResult

	// Recompute effective AC with any reaction modifiers
	effectiveAC := ac.OriginalAC
	for _, mod := range input.Reactions {
		effectiveAC += mod.ACBonus
	}

	// Re-evaluate hit with effective AC
	var hit bool
	switch {
	case ac.IsNaturalOne:
		hit = false
	case ac.IsNaturalTwenty:
		hit = true
	default:
		hit = ac.TotalAttack >= effectiveAC
	}

	// Critical requires BOTH: roll meets the threshold AND the attack still hits.
	// If a reaction (e.g. Shield) retroactively converts a would-be crit into a
	// miss, Critical must be false — an attack that misses cannot deal crit damage.
	isCritical := hit && ac.AttackRoll >= ac.CriticalThreshold

	result := &AttackResult{
		AttackRoll:      ac.AttackRoll,
		AttackBonus:     ac.AttackBonus,
		TotalAttack:     ac.TotalAttack,
		TargetAC:        effectiveAC,
		Hit:             hit,
		Critical:        isCritical,
		IsNaturalTwenty: ac.IsNaturalTwenty,
		IsNaturalOne:    ac.IsNaturalOne,
		AllRolls:        ac.AllRolls,
		HasAdvantage:    ac.HasAdvantage,
		HasDisadvantage: ac.HasDisadvantage,
		DamageType:      ac.Weapon.DamageType,
	}

	if !hit {
		return result, nil
	}

	// Phase 2: Roll and apply damage
	damagePool, err := dice.ParseNotation(ac.Weapon.Damage)
	if err != nil {
		return nil, rpgerr.Wrap(err, fmt.Sprintf("invalid weapon damage %s", ac.Weapon.Damage))
	}

	var damageRolls []int
	if isCritical {
		damageRolls, err = rollDamageDice(ctx, damagePool, ac.roller, 2)
	} else {
		damageRolls, err = rollDamageDice(ctx, damagePool, ac.roller, 1)
	}
	if err != nil {
		return nil, err
	}
	result.DamageRolls = damageRolls

	weaponComponent := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		SourceRef:         weaponToRef(ac.Weapon),
		OriginalDiceRolls: damageRolls,
		FinalDiceRolls:    damageRolls,
		DamageType:        ac.Weapon.DamageType,
		IsCritical:        isCritical,
	}

	abilityComponent := dnd5eEvents.DamageComponent{
		Source:     dnd5eEvents.DamageSourceAbility,
		SourceRef:  abilityToRef(ac.abilityUsed),
		FlatBonus:  ac.abilityMod,
		DamageType: ac.Weapon.DamageType,
		IsCritical: isCritical,
	}

	resolveOutput, err := ResolveDamage(ctx, &ResolveDamageInput{
		AttackerID:      ac.AttackerID,
		TargetID:        ac.TargetID,
		Components:      []dnd5eEvents.DamageComponent{weaponComponent, abilityComponent},
		IsCritical:      isCritical,
		HasAdvantage:    ac.HasAdvantage,
		IsOffHandAttack: ac.isOffHandAttack,
		AbilityModifier: ac.abilityMod,
		EventBus:        ac.eventBus,
		WeaponDamage:    ac.Weapon.Damage,
		AbilityUsed:     ac.abilityUsed,
		WeaponRef:       weaponToRef(ac.Weapon),
	})
	if err != nil {
		return nil, err
	}

	result.TotalDamage = resolveOutput.TotalDamage

	finalAbilityUsed := ac.abilityUsed
	if resolveOutput.AbilityUsed != "" {
		finalAbilityUsed = resolveOutput.AbilityUsed
	}

	// Look up attacker to get final ability modifier (may have been changed by chain)
	attacker, err := GetCombatantFromContext(ctx, ac.AttackerID)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to look up attacker %s for damage bonus", ac.AttackerID)
	}
	result.DamageBonus = attacker.AbilityScores().Modifier(finalAbilityUsed)

	if result.TotalDamage < 0 {
		result.TotalDamage = 0
	}

	result.Breakdown = &DamageBreakdown{
		Components:  resolveOutput.FinalComponents,
		AbilityUsed: finalAbilityUsed,
		TotalDamage: resolveOutput.TotalDamage,
	}

	damageTopic := dnd5eEvents.DamageReceivedTopic.On(ac.eventBus)
	if err := damageTopic.Publish(ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   ac.TargetID,
		SourceID:   ac.AttackerID,
		SourceRef:  weaponToRef(ac.Weapon),
		Amount:     result.TotalDamage,
		DamageType: ac.Weapon.DamageType,
	}); err != nil {
		return nil, rpgerr.Wrap(err, "failed to publish damage received event")
	}

	return result, nil
}
