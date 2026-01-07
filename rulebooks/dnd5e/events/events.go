// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package events provides D&D 5e event system implementation
package events

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// ConditionType represents D&D 5e conditions
type ConditionType string

const (
	// ConditionBlinded is a condition that blinds a character
	ConditionBlinded ConditionType = "blinded"
	// ConditionCharmed is a condition that charms a character
	ConditionCharmed ConditionType = "charmed"
	// ConditionDeafened is a condition that deafens a character
	ConditionDeafened ConditionType = "deafened"
	// ConditionFrightened is a condition that frightens a character
	ConditionFrightened ConditionType = "frightened"
	// ConditionGrappled is a condition that grapples a character
	ConditionGrappled ConditionType = "grappled"
	// ConditionIncapacitated is a condition that incapacitates a character
	ConditionIncapacitated ConditionType = "incapacitated"
	// ConditionInvisible is a condition that makes a character invisible
	ConditionInvisible ConditionType = "invisible"
	// ConditionParalyzed is a condition that paralyzes a character
	ConditionParalyzed ConditionType = "paralyzed"
	// ConditionPetrified is a condition that petrifies a character
	ConditionPetrified ConditionType = "petrified"
	// ConditionPoisoned is a condition that poisons a character
	ConditionPoisoned ConditionType = "poisoned"
	// ConditionProne is a condition that makes a character prone
	ConditionProne ConditionType = "prone"
	// ConditionRestrained is a condition that restrains a character
	ConditionRestrained ConditionType = "restrained"
	// ConditionStunned is a condition that stuns a character
	ConditionStunned ConditionType = "stunned"
	// ConditionUnconscious is a condition that makes a character unconscious
	ConditionUnconscious ConditionType = "unconscious"

	// ConditionExhaustion1 is a condition that exhausts a character
	ConditionExhaustion1 ConditionType = "exhaustion_1"
	// ConditionExhaustion2 is a condition that exhausts a character
	ConditionExhaustion2 ConditionType = "exhaustion_2"
	// ConditionExhaustion3 is a condition that exhausts a character
	ConditionExhaustion3 ConditionType = "exhaustion_3"
	// ConditionExhaustion4 is a condition that exhausts a character
	ConditionExhaustion4 ConditionType = "exhaustion_4"
	// ConditionExhaustion5 is a condition that exhausts a character
	ConditionExhaustion5 ConditionType = "exhaustion_5"
	// ConditionExhaustion6 is a condition that exhausts a character
	ConditionExhaustion6 ConditionType = "exhaustion_6"

	// ConditionRaging is a class-specific condition for barbarians
	ConditionRaging ConditionType = "raging"

	// ConditionFightingStyle represents an active fighting style
	ConditionFightingStyle ConditionType = "fighting_style"
)

// ConditionSource identifies where a condition originated
type ConditionSource string

const (
	// ConditionSourceClass indicates condition from class choice (e.g., fighting style)
	ConditionSourceClass ConditionSource = "class"
	// ConditionSourceFeature indicates condition from feature activation (e.g., rage)
	ConditionSourceFeature ConditionSource = "feature"
)

// ConditionBehavior represents the behavior of an active condition.
// Conditions subscribe to events to modify game mechanics.
type ConditionBehavior interface {
	// IsApplied returns true if this condition is currently applied.
	// Note: Some conditions may allow stacking (multiple applies), others may not.
	IsApplied() bool

	// Apply subscribes this condition to relevant events on the bus
	Apply(ctx context.Context, bus events.EventBus) error

	// Remove unsubscribes this condition from events
	Remove(ctx context.Context, bus events.EventBus) error

	// ToJSON converts the condition to JSON for persistence
	ToJSON() (json.RawMessage, error)
}

// =============================================================================
// Damage Source Types
// =============================================================================

// DamageSourceType categorizes where damage bonuses come from.
// This is the category only - use SourceRef for the specific reference.
type DamageSourceType string

// Damage source category constants
const (
	DamageSourceWeapon       DamageSourceType = "weapon"        // Damage from a weapon
	DamageSourceAbility      DamageSourceType = "ability"       // Damage from ability modifier
	DamageSourceCondition    DamageSourceType = "condition"     // Damage from an active condition (rage, etc.)
	DamageSourceFeature      DamageSourceType = "feature"       // Damage from a class/racial feature
	DamageSourceSpell        DamageSourceType = "spell"         // Damage from a spell
	DamageSourceItem         DamageSourceType = "item"          // Damage from a magic item
	DamageSourceMonsterTrait DamageSourceType = "monster_trait" // Modifier from monster trait (vulnerability, etc.)
)

// =============================================================================
// Damage Components
// =============================================================================

// RerollEvent tracks a single die reroll
type RerollEvent struct {
	DieIndex int    // Which die was rerolled (0-based in OriginalDiceRolls)
	Before   int    // Value before reroll
	After    int    // Value after reroll
	Reason   string // Feature that caused reroll (e.g., "great_weapon_fighting")
}

// DamageComponent represents damage from one source
type DamageComponent struct {
	Source            DamageSourceType // Category: weapon, ability, condition, etc.
	SourceRef         *core.Ref        // Specific reference (e.g., refs.Weapons.Longsword())
	OriginalDiceRolls []int            // As first rolled
	FinalDiceRolls    []int            // After all rerolls
	Rerolls           []RerollEvent    // History of rerolls
	FlatBonus         int              // Flat modifier (0 if none)
	DamageType        damage.Type      // damage.Slashing, damage.Fire, etc.
	IsCritical        bool             // Was this doubled for crit?
	// Multiplier for this component (0 means 1.0/no multiplier).
	// Used for vulnerability (2.0), resistance (0.5), or immunity (0.0 to negate).
	// When non-zero, this component represents a multiplier to apply to other
	// components of the same damage type, not additional damage itself.
	Multiplier float64
}

// Total returns the total damage for this component
func (dc *DamageComponent) Total() int {
	total := dc.FlatBonus
	for _, roll := range dc.FinalDiceRolls {
		total += roll
	}
	return total
}

// =============================================================================
// Attack Modifier Types
// =============================================================================

// AttackModifierSource tracks the source of an advantage or disadvantage modifier.
// Used by features like Protection fighting style to record what caused the modifier.
type AttackModifierSource struct {
	SourceRef *core.Ref // Reference to the feature/condition (e.g., refs.Conditions.FightingStyleProtection())
	SourceID  string    // ID of the entity that provided the modifier
	Reason    string    // Human-readable explanation
}

// ReactionConsumption tracks a reaction consumed during an attack chain.
// Processed after chain execution to update game state.
type ReactionConsumption struct {
	CharacterID string    // Who used their reaction
	FeatureRef  *core.Ref // What feature consumed it
	Reason      string    // Human-readable explanation
}

// =============================================================================
// Chain Events (modifier chains)
// =============================================================================

// AttackChainEvent represents an attack flowing through the modifier chain.
// This event fires BEFORE the d20 roll to allow advantage/disadvantage to be collected.
type AttackChainEvent struct {
	// Identity
	AttackerID string    // ID of the attacking character
	TargetID   string    // ID of the target
	WeaponRef  *core.Ref // Reference to the weapon used
	IsMelee    bool      // True for melee attacks, false for ranged

	// Advantage/Disadvantage (inputs to the roll)
	AdvantageSources    []AttackModifierSource // Sources granting advantage
	DisadvantageSources []AttackModifierSource // Sources imposing disadvantage

	// Modifiers (applied to attack roll)
	AttackBonus       int // Base bonus before modifiers (can be modified by chain)
	TargetAC          int // Target's armor class (for reference)
	CriticalThreshold int // Roll >= this value is a critical hit (default 20, can be lowered)

	// Side effects (processed after chain execution)
	ReactionsConsumed []ReactionConsumption // Reactions used during this attack
}

// DamageChainEvent represents damage flowing through the modifier chain
type DamageChainEvent struct {
	AttackerID      string
	TargetID        string
	Components      []DamageComponent // All damage sources
	DamageType      damage.Type       // Type of damage (slashing, piercing, etc.)
	IsCritical      bool              // Double damage dice on crit
	HasAdvantage    bool              // True if attacker had advantage on the attack roll
	WeaponDamage    string            // Weapon damage dice (e.g., "1d8")
	AbilityUsed     abilities.Ability // Which ability was used (str, dex, etc.)
	WeaponRef       *core.Ref         // Reference to the weapon used (for off-hand detection, etc.)
	IsOffHandAttack bool              // True for bonus action off-hand attacks (two-weapon fighting)
	AbilityModifier int               // The ability modifier (STR/DEX) for this attack
}

// =============================================================================
// Simple Events (pub/sub notifications)
// =============================================================================

// TurnStartEvent is published when a character's turn begins
type TurnStartEvent struct {
	CharacterID string // ID of the character whose turn is starting
	Round       int    // Current round number
}

// TurnEndEvent is published when a character's turn ends
type TurnEndEvent struct {
	CharacterID string // ID of the character whose turn is ending
	Round       int    // Current round number
}

// DamageReceivedEvent is published when a character takes damage
type DamageReceivedEvent struct {
	TargetID   string      // ID of the character taking damage
	SourceID   string      // ID of the attacker/source entity
	SourceRef  *core.Ref   // What caused the damage (weapon, spell, condition ref)
	Amount     int         // Amount of damage
	DamageType damage.Type // Type of damage (slashing, fire, etc)
}

// HealingReceivedEvent is published when a character receives healing
type HealingReceivedEvent struct {
	TargetID string // ID of the character receiving healing
	Amount   int    // Amount of healing
	Roll     int    // The dice roll result (before modifiers)
	Modifier int    // Any modifier added to the roll (e.g., fighter level)
	Source   string // What caused this healing (e.g., "second_wind")
}

// ConditionAppliedEvent is published when a condition is applied to an entity
type ConditionAppliedEvent struct {
	Target    core.Entity       // Entity receiving the condition
	Type      ConditionType     // Type of condition being applied
	Source    ConditionSource   // What caused this condition
	Condition ConditionBehavior // The condition behavior to apply
}

// ConditionRemovedEvent is published when a condition ends
type ConditionRemovedEvent struct {
	CharacterID  string
	ConditionRef string
	Reason       string
}

// AttackEvent is published when a character makes an attack (before rolls)
type AttackEvent struct {
	AttackerID string // ID of the attacking character
	TargetID   string // ID of the target
	WeaponRef  string // Reference to the weapon used
	IsMelee    bool   // True for melee attacks, false for ranged
}

// ReactionUsedEvent is published when a character uses their reaction.
// Game server listens to update ActionEconomy.
type ReactionUsedEvent struct {
	CharacterID string    // ID of the character who used their reaction
	FeatureRef  *core.Ref // What feature consumed the reaction
	Reason      string    // Human-readable explanation
}

// RestEvent is published when a character takes a rest
type RestEvent struct {
	RestType    resources.ResetType // Type of rest (short_rest, long_rest, etc)
	CharacterID string              // ID of the character resting
}

// ResourceConsumedEvent is published when a character uses a resource
type ResourceConsumedEvent struct {
	CharacterID string                // ID of the character consuming the resource
	ResourceKey resources.ResourceKey // Which resource was consumed
	Amount      int                   // How much was consumed
	Remaining   int                   // How much is left after consumption
}

// =============================================================================
// Monk Feature Events
// =============================================================================

// FlurryOfBlowsActivatedEvent is published when a monk activates Flurry of Blows
// DEPRECATED: Use FlurryStrike actions instead. This event will be removed
// once all consumers migrate to the action-based pattern.
type FlurryOfBlowsActivatedEvent struct {
	CharacterID    string // ID of the monk activating the feature
	UnarmedStrikes int    // Number of unarmed strikes granted (always 2)
	Source         string // Feature that triggered this (refs.Features.FlurryOfBlows().ID)
}

// FlurryStrikeRequestedEvent is published when a FlurryStrike action is activated.
// The game server should resolve an unarmed strike attack from attacker to target.
type FlurryStrikeRequestedEvent struct {
	AttackerID string // ID of the monk making the strike
	TargetID   string // ID of the target being struck
	ActionID   string // ID of the FlurryStrike action (for tracking)
}

// ActionRemovedEvent is published when an action removes itself from a character.
// The character should listen for this event and remove the action from their list.
type ActionRemovedEvent struct {
	ActionID string // ID of the action being removed
	OwnerID  string // ID of the character who owns the action
}

// FlurryStrikeActivatedEvent is published after a FlurryStrike action is successfully used.
// This is a notification event for UI/logging - the attack itself is resolved via FlurryStrikeRequestedEvent.
type FlurryStrikeActivatedEvent struct {
	AttackerID    string // ID of the monk who used the strike
	TargetID      string // ID of the target that was struck
	ActionID      string // ID of the FlurryStrike action
	UsesRemaining int    // Uses remaining after this activation (0 = action will be removed)
}

// OffHandStrikeRequestedEvent is published when an OffHandStrike action is activated.
// The game server should resolve an off-hand weapon attack from attacker to target.
// Note: Off-hand attacks don't add ability modifier to damage unless the character
// has the Two-Weapon Fighting fighting style.
type OffHandStrikeRequestedEvent struct {
	AttackerID string // ID of the character making the off-hand attack
	TargetID   string // ID of the target being struck
	WeaponID   string // ID of the off-hand weapon being used
	ActionID   string // ID of the OffHandStrike action (for tracking)
}

// OffHandStrikeActivatedEvent is published after an OffHandStrike action is successfully used.
// This is a notification event for UI/logging.
type OffHandStrikeActivatedEvent struct {
	AttackerID    string // ID of the character who used the strike
	TargetID      string // ID of the target that was struck
	WeaponID      string // ID of the off-hand weapon used
	ActionID      string // ID of the OffHandStrike action
	UsesRemaining int    // Uses remaining after this activation (0 = action will be removed)
}

// PatientDefenseActivatedEvent is published when a monk activates Patient Defense
type PatientDefenseActivatedEvent struct {
	CharacterID string // ID of the monk activating the feature
	Source      string // Feature that triggered this (refs.Features.PatientDefense().ID)
}

// StepOfTheWindActivatedEvent is published when a monk activates Step of the Wind
type StepOfTheWindActivatedEvent struct {
	CharacterID string // ID of the monk activating the feature
	Action      string // Action taken: "disengage" or "dash"
	Source      string // Feature that triggered this (refs.Features.StepOfTheWind().ID)
}

// DeflectMissilesTriggerEvent is published when a monk deflects a ranged weapon attack
type DeflectMissilesTriggerEvent struct {
	CharacterID      string // ID of the monk deflecting
	OriginalDamage   int    // Damage before reduction
	Reduction        int    // Amount reduced (1d10 + DEX + monk level)
	DamageReducedTo0 bool   // If true, monk can spend 1 Ki to throw it back
	Source           string // Feature that triggered this (refs.Features.DeflectMissiles().ID)
}

// DeflectMissilesThrowEvent is published when a monk throws a deflected missile back
type DeflectMissilesThrowEvent struct {
	CharacterID string // ID of the monk throwing the missile
	Source      string // Feature that triggered this (refs.Features.DeflectMissiles().ID)
}

// =============================================================================
// Combat Ability Events
// =============================================================================

// DodgeActivatedEvent is published when a character uses the Dodge action.
// Until the start of their next turn, attacks against them have disadvantage
// (if they can see the attacker), and they make DEX saves with advantage.
// The condition ends if they become incapacitated or their speed drops to 0.
type DodgeActivatedEvent struct {
	CharacterID string // ID of the character who is dodging
}

// DisengageActivatedEvent is published when a character uses the Disengage action.
// Their movement doesn't provoke opportunity attacks for the rest of the turn.
type DisengageActivatedEvent struct {
	CharacterID string // ID of the character who is disengaging
}

// =============================================================================
// Topic Definitions
// =============================================================================

// Simple pub/sub topics
var (
	// TurnStartTopic provides typed pub/sub for turn start events
	TurnStartTopic = events.DefineTypedTopic[TurnStartEvent]("dnd5e.turn.start")

	// TurnEndTopic provides typed pub/sub for turn end events
	TurnEndTopic = events.DefineTypedTopic[TurnEndEvent]("dnd5e.turn.end")

	// DamageReceivedTopic provides typed pub/sub for damage received events
	DamageReceivedTopic = events.DefineTypedTopic[DamageReceivedEvent]("dnd5e.combat.damage.received")

	// HealingReceivedTopic provides typed pub/sub for healing received events
	HealingReceivedTopic = events.DefineTypedTopic[HealingReceivedEvent]("dnd5e.combat.healing.received")

	// ConditionAppliedTopic provides typed pub/sub for condition applied events
	ConditionAppliedTopic = events.DefineTypedTopic[ConditionAppliedEvent]("dnd5e.condition.applied")

	// ConditionRemovedTopic provides typed pub/sub for condition removed events
	ConditionRemovedTopic = events.DefineTypedTopic[ConditionRemovedEvent]("dnd5e.condition.removed")

	// AttackTopic provides typed pub/sub for attack events
	AttackTopic = events.DefineTypedTopic[AttackEvent]("dnd5e.combat.attack")

	// ReactionUsedTopic provides typed pub/sub for reaction used events
	ReactionUsedTopic = events.DefineTypedTopic[ReactionUsedEvent]("dnd5e.combat.reaction.used")

	// RestTopic provides typed pub/sub for rest events
	RestTopic = events.DefineTypedTopic[RestEvent]("dnd5e.rest")

	// ResourceConsumedTopic provides typed pub/sub for resource consumption events
	ResourceConsumedTopic = events.DefineTypedTopic[ResourceConsumedEvent]("dnd5e.resource.consumed")

	// FlurryOfBlowsActivatedTopic provides typed pub/sub for flurry of blows activation events
	// DEPRECATED: Use FlurryStrikeRequestedTopic instead.
	FlurryOfBlowsActivatedTopic = events.DefineTypedTopic[FlurryOfBlowsActivatedEvent](
		"dnd5e.feature.flurry_of_blows.activated")

	// FlurryStrikeRequestedTopic provides typed pub/sub for flurry strike action requests
	FlurryStrikeRequestedTopic = events.DefineTypedTopic[FlurryStrikeRequestedEvent](
		"dnd5e.action.flurry_strike.requested")

	// FlurryStrikeActivatedTopic provides typed pub/sub for flurry strike completion notifications
	FlurryStrikeActivatedTopic = events.DefineTypedTopic[FlurryStrikeActivatedEvent](
		"dnd5e.action.flurry_strike.activated")

	// OffHandStrikeRequestedTopic provides typed pub/sub for off-hand strike action requests
	OffHandStrikeRequestedTopic = events.DefineTypedTopic[OffHandStrikeRequestedEvent](
		"dnd5e.action.off_hand_strike.requested")

	// OffHandStrikeActivatedTopic provides typed pub/sub for off-hand strike completion notifications
	OffHandStrikeActivatedTopic = events.DefineTypedTopic[OffHandStrikeActivatedEvent](
		"dnd5e.action.off_hand_strike.activated")

	// ActionRemovedTopic provides typed pub/sub for action removed events
	ActionRemovedTopic = events.DefineTypedTopic[ActionRemovedEvent](
		"dnd5e.action.removed")

	// PatientDefenseActivatedTopic provides typed pub/sub for patient defense activation events
	PatientDefenseActivatedTopic = events.DefineTypedTopic[PatientDefenseActivatedEvent](
		"dnd5e.feature.patient_defense.activated")

	// StepOfTheWindActivatedTopic provides typed pub/sub for step of the wind activation events
	StepOfTheWindActivatedTopic = events.DefineTypedTopic[StepOfTheWindActivatedEvent](
		"dnd5e.feature.step_of_the_wind.activated")

	// DeflectMissilesTriggerTopic provides typed pub/sub for deflect missiles trigger events
	DeflectMissilesTriggerTopic = events.DefineTypedTopic[DeflectMissilesTriggerEvent](
		"dnd5e.feature.deflect_missiles.triggered")

	// DeflectMissilesThrowTopic provides typed pub/sub for deflect missiles throw events
	DeflectMissilesThrowTopic = events.DefineTypedTopic[DeflectMissilesThrowEvent]("dnd5e.feature.deflect_missiles.throw")

	// DodgeActivatedTopic provides typed pub/sub for Dodge ability activation
	DodgeActivatedTopic = events.DefineTypedTopic[DodgeActivatedEvent]("dnd5e.ability.dodge.activated")

	// DisengageActivatedTopic provides typed pub/sub for Disengage ability activation
	DisengageActivatedTopic = events.DefineTypedTopic[DisengageActivatedEvent]("dnd5e.ability.disengage.activated")
)

// Chain topics (for modifier chains)
var (
	// AttackChain provides typed chained topic for attack roll modifiers
	AttackChain = events.DefineChainedTopic[AttackChainEvent]("dnd5e.combat.attack.chain")

	// DamageChain provides typed chained topic for damage modifiers
	DamageChain = events.DefineChainedTopic[*DamageChainEvent]("dnd5e.combat.damage.chain")
)
