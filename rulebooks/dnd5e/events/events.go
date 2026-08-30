// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package events provides D&D 5e event system implementation
package events

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
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
	// ConditionRecklessAttack is a class-specific condition for barbarians using Reckless Attack
	ConditionRecklessAttack ConditionType = "reckless_attack"

	// ConditionFightingStyle represents an active fighting style
	ConditionFightingStyle ConditionType = "fighting_style"

	// ConditionDodging is applied when a character uses the Dodge combat ability.
	// Attackers have disadvantage against the character and the character has
	// advantage on DEX saves, until the start of the character's next turn.
	ConditionDodging ConditionType = "dodging"

	// ConditionDisengaging is applied when a character takes the Disengage
	// action, or a monk spends Step of the Wind on it. It arrives late
	// (rpg-toolkit#1272) because Disengage applied its condition directly to
	// the bus instead of publishing one, so there was never an event to
	// name — which is exactly why the condition never reached a sheet.
	ConditionDisengaging ConditionType = "disengaging"
	// ConditionHidden is applied when a character successfully hides
	// (a Stealth check beating observers' passive Perception). Grants
	// advantage on the hidden character's attacks and disadvantage on
	// attacks against them; removed when the hider makes their own attack.
	ConditionHidden ConditionType = "hidden"
	// ConditionHelped is applied to the ally targeted by the Help combat
	// ability. Grants advantage on the ally's next attack roll; removed
	// when consumed or at the helper's next turn if unused.
	ConditionHelped ConditionType = "helped"
)

// ConditionSource identifies where a condition originated
type ConditionSource string

const (
	// ConditionSourceClass indicates condition from class choice (e.g., fighting style)
	ConditionSourceClass ConditionSource = "class"
	// ConditionSourceFeature indicates condition from feature activation (e.g., rage)
	ConditionSourceFeature ConditionSource = "feature"
	// ConditionSourceCombatAbility indicates condition from a universal combat
	// ability activation (e.g., Dodge, Hide, Help)
	ConditionSourceCombatAbility ConditionSource = "combat_ability"
	// ConditionSourceDamage indicates a condition applied as a direct
	// consequence of taking damage (e.g., Unconscious at 0 HP) rather than
	// an actor's own ability activation.
	ConditionSourceDamage ConditionSource = "damage"
)

// ConditionBehavior represents the behavior of an active condition.
// Conditions subscribe to events to modify game mechanics.
type ConditionBehavior interface {
	// Ref returns the canonical ref this condition names itself by — the same
	// ref its ToJSON embeds and its loader routes on (rpg-toolkit#971). A live
	// condition can now name itself honestly, so a composition never has to
	// serialize it back to JSON to recover the pairing. Must never return nil.
	Ref() *core.Ref

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
	Source            DamageSourceType  // Category: weapon, ability, condition, etc.
	SourceRef         *core.Ref         // Specific reference (e.g., refs.Weapons.Longsword())
	Dice              string            // Pure notation for this component's declared dice pool.
	OriginalDiceRolls []int             // As first rolled
	FinalDiceRolls    []int             // After all rerolls
	Rerolls           []RerollEvent     // History of rerolls
	FlatBonus         int               // Flat modifier (0 if none)
	DamageType        damage.Type       // damage.Slashing, damage.Fire, etc.
	Properties        []damage.Property // Behavior belonging to this component's declared pool.
	IsCritical        bool              // Was this doubled for crit?
	// Multiplier scales the other components of the same damage type rather
	// than adding damage of its own: vulnerability (2.0), resistance (0.5),
	// immunity (0.0).
	//
	// A pointer because ZERO IS A LEGAL FACTOR. This was a plain float64 whose
	// doc read "0 means 1.0/no multiplier" and, two lines later, "immunity (0.0
	// to negate)" — one value carrying both meanings. The dispatch had to pick
	// one, picked "absent", and immunity silently stopped working: an immune
	// target took full damage and the immunity branch of the stacking rules was
	// unreachable (rpg-toolkit#1012). Nil now means "this component is damage",
	// and any float — zero included — means "this component modifies damage".
	//
	// Build one with [Multiply]; &0.0 is not expressible inline.
	Multiplier *float64
}

// HasProperty reports whether this component carries a declared damage-pool
// property.
func (dc DamageComponent) HasProperty(property damage.Property) bool {
	for _, got := range dc.Properties {
		if got == property {
			return true
		}
	}

	return false
}

// Multiply returns a factor for [DamageComponent.Multiplier].
//
// It exists because the zero factor — immunity — cannot be written inline as
// a pointer, and because a named constructor makes "this component is a
// modifier" the visible act at every call site rather than a consequence of
// which field happens to be set.
func Multiply(factor float64) *float64 {
	return &factor
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
// Attack Type
// =============================================================================

// AttackType categorizes the type of attack being made.
// This is used to distinguish standard attacks from opportunity attacks,
// which affects how certain conditions (like Disengaging) can respond.
type AttackType string

const (
	// AttackTypeStandard is a normal attack made during combat (default)
	AttackTypeStandard AttackType = "standard"

	// AttackTypeOpportunity is a reaction attack triggered by movement
	AttackTypeOpportunity AttackType = "opportunity"
)

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

// =============================================================================
// Chain Events (modifier chains)
// =============================================================================

// AttackChainEvent represents an attack flowing through the modifier chain.
// This event fires BEFORE the d20 roll to allow advantage/disadvantage to be collected.
type AttackChainEvent struct {
	// Identity
	AttackerID string     // ID of the attacking character
	TargetID   string     // ID of the target
	WeaponRef  *core.Ref  // Reference to the weapon used
	IsMelee    bool       // True for melee attacks, false for ranged
	AttackType AttackType // Type of attack (standard or opportunity)

	// Advantage/Disadvantage (inputs to the roll)
	AdvantageSources    []AttackModifierSource // Sources granting advantage
	DisadvantageSources []AttackModifierSource // Sources imposing disadvantage

	// Cancellation (attack can be cancelled by conditions like Disengaging)
	CancellationSources []AttackModifierSource // Sources that cancelled this attack

	// Modifiers (applied to attack roll)
	AttackBonus       int // Base bonus before modifiers (can be modified by chain)
	TargetAC          int // Target's armor class (for reference)
	CriticalThreshold int // Roll >= this value is a critical hit (default 20, can be lowered)
}

// IsCancelled returns true if this attack has been cancelled.
// An attack is cancelled if any cancellation sources have been added to the event.
// When cancelled, the attack should not proceed (no roll, no damage).
func (e *AttackChainEvent) IsCancelled() bool {
	return len(e.CancellationSources) > 0
}

// DamageChainEvent represents damage flowing through the modifier chain
type DamageChainEvent struct {
	AttackerID       string
	TargetID         string
	Components       []DamageComponent // All damage sources; each component owns its damage type.
	WeaponDamageDice string            // Marked primary weapon dice (e.g., "1d8").
	WeaponDamageType damage.Type       // Marked primary weapon damage type.
	IsCritical       bool              // Double damage dice on crit
	HasAdvantage     bool              // True if attacker had advantage on the attack roll
	AbilityUsed      abilities.Ability // Which ability was used (str, dex, etc.)
	WeaponRef        *core.Ref         // Reference to the weapon used (for off-hand detection, etc.)
	IsOffHandAttack  bool              // True for bonus action off-hand attacks (two-weapon fighting)
	AbilityModifier  int               // The ability modifier (STR/DEX) for this attack
	IsMelee          bool              // True for melee attacks, false for ranged (mirrors AttackChainEvent.IsMelee)

	// TwoHanded says this swing is being made with both hands on the
	// weapon — either the weapon itself requires it, or a versatile weapon
	// was gripped that way. A STATIC fact of the swing, compiled once by
	// the attack compiler rather than read live off a character registry
	// (rpg-toolkit#1178, docs/ideas/session-sdk/attack-profile-seam.md):
	// it cannot change between the attack roll and the damage roll of the
	// same swing.
	TwoHanded bool

	// OffHandWeaponRef names the weapon, if any, occupying the attacker's
	// OTHER hand from the one that just swung — nil when that hand is
	// empty or holds something that is not a weapon (a shield, most
	// often). Like TwoHanded, this is a static equipment fact the compiler
	// already knows, carried onto the event the same way WeaponRef already
	// is, so a predicate like Dueling's decides eligibility from the event
	// alone rather than a live gamectx lookup.
	OffHandWeaponRef *core.Ref
}

// DamageChainInput contains the facts used to construct a DamageChainEvent.
// Components remain the authoritative typed damage; the primary fields exist
// only for rules that explicitly inherit a marked weapon pool's dice or type.
type DamageChainInput struct {
	AttackerID       string
	TargetID         string
	Components       []DamageComponent
	WeaponDamageDice string
	WeaponDamageType damage.Type
	IsCritical       bool
	HasAdvantage     bool
	AbilityUsed      abilities.Ability
	WeaponRef        *core.Ref
	IsOffHandAttack  bool
	AbilityModifier  int
	IsMelee          bool
	TwoHanded        bool
	OffHandWeaponRef *core.Ref
}

// NewDamageChainEvent constructs a damage-chain event with explicit primary
// weapon metadata. It does not derive a damage type from Components because a
// multi-pool attack has no event-wide damage type.
func NewDamageChainEvent(input DamageChainInput) *DamageChainEvent {
	return &DamageChainEvent{
		AttackerID:       input.AttackerID,
		TargetID:         input.TargetID,
		Components:       input.Components,
		WeaponDamageDice: input.WeaponDamageDice,
		WeaponDamageType: input.WeaponDamageType,
		IsCritical:       input.IsCritical,
		HasAdvantage:     input.HasAdvantage,
		AbilityUsed:      input.AbilityUsed,
		WeaponRef:        input.WeaponRef,
		IsOffHandAttack:  input.IsOffHandAttack,
		AbilityModifier:  input.AbilityModifier,
		IsMelee:          input.IsMelee,
		TwoHanded:        input.TwoHanded,
		OffHandWeaponRef: input.OffHandWeaponRef,
	}
}

// =============================================================================
// Saving Throw Chain Types
// =============================================================================

// SaveTrigger identifies what caused the saving throw
type SaveTrigger string

const (
	// SaveTriggerSpell indicates the saving throw was caused by a spell
	SaveTriggerSpell SaveTrigger = "spell"
	// SaveTriggerTrap indicates the saving throw was caused by a trap
	SaveTriggerTrap SaveTrigger = "trap"
	// SaveTriggerConcentration indicates the saving throw was for maintaining concentration
	SaveTriggerConcentration SaveTrigger = "concentration"
	// SaveTriggerFeature indicates the saving throw was caused by a class/racial feature
	SaveTriggerFeature SaveTrigger = "feature"
	// SaveTriggerEnvironment indicates the saving throw was caused by environmental effects
	SaveTriggerEnvironment SaveTrigger = "environment"
)

// SaveCause provides context about what caused the saving throw
type SaveCause struct {
	Trigger        SaveTrigger // What type of effect triggered this save
	EffectRef      *core.Ref   // Reference to the spell/trap/feature causing the save
	InstigatorID   string      // ID of entity that caused the save (caster, trap placer, etc)
	InstigatorType string      // Type of instigator ("character", "monster", "trap", etc)
}

// SaveModifierSource tracks the source of a saving throw modifier
type SaveModifierSource struct {
	Name       string    // Display name (e.g., "Dodging", "Bless")
	SourceType string    // Type of source ("condition", "feature", "spell", etc)
	SourceRef  *core.Ref // Reference to the source
	EntityID   string    // ID of entity providing the modifier
}

// SaveBonusSource tracks a bonus to the saving throw
type SaveBonusSource struct {
	SaveModifierSource     // Embedded modifier source
	Bonus              int // The bonus amount
}

// SavingThrowChainEvent represents a saving throw flowing through the modifier chain.
// This event fires BEFORE the d20 roll to allow advantage/disadvantage/bonuses to be collected.
type SavingThrowChainEvent struct {
	SaverID string            // ID of the entity making the save
	Ability abilities.Ability // The ability being used (DEX, CON, etc)
	DC      int               // Difficulty class
	Cause   SaveCause         // What caused this saving throw

	AdvantageSources    []SaveModifierSource // Sources granting advantage
	DisadvantageSources []SaveModifierSource // Sources imposing disadvantage
	BonusSources        []SaveBonusSource    // Sources adding bonuses to the roll
}

// HasAdvantage returns true if any advantage sources have been added to this event
func (e *SavingThrowChainEvent) HasAdvantage() bool {
	return len(e.AdvantageSources) > 0
}

// HasDisadvantage returns true if any disadvantage sources have been added to this event
func (e *SavingThrowChainEvent) HasDisadvantage() bool {
	return len(e.DisadvantageSources) > 0
}

// TotalBonus returns the sum of all bonus sources
func (e *SavingThrowChainEvent) TotalBonus() int {
	total := 0
	for _, source := range e.BonusSources {
		total += source.Bonus
	}
	return total
}

// =============================================================================
// Ability Check Chain Types
// =============================================================================

// CheckModifierSource tracks the source of an ability check modifier.
// Mirrors SaveModifierSource.
type CheckModifierSource struct {
	Name       string    // Display name (e.g., "Hidden", "Guidance")
	SourceType string    // Type of source ("condition", "feature", "spell", etc)
	SourceRef  *core.Ref // Reference to the source
	EntityID   string    // ID of entity providing the modifier
}

// CheckBonusSource tracks a bonus to an ability check.
// Mirrors SaveBonusSource.
type CheckBonusSource struct {
	CheckModifierSource     // Embedded modifier source
	Bonus               int // The bonus amount
}

// AbilityCheckChainEvent represents an ability check flowing through the
// modifier chain. This event fires BEFORE the d20 roll to allow
// advantage/disadvantage/bonuses to be collected. Mirrors SavingThrowChainEvent.
type AbilityCheckChainEvent struct {
	CheckerID string       // ID of the entity making the check
	Skill     skills.Skill // The skill being checked (Stealth, Perception, etc)
	DC        int          // Difficulty class (or the value being beaten)

	AdvantageSources    []CheckModifierSource // Sources granting advantage
	DisadvantageSources []CheckModifierSource // Sources imposing disadvantage
	BonusSources        []CheckBonusSource    // Sources adding bonuses to the roll
}

// HasAdvantage returns true if any advantage sources have been added to this event
func (e *AbilityCheckChainEvent) HasAdvantage() bool {
	return len(e.AdvantageSources) > 0
}

// HasDisadvantage returns true if any disadvantage sources have been added to this event
func (e *AbilityCheckChainEvent) HasDisadvantage() bool {
	return len(e.DisadvantageSources) > 0
}

// TotalBonus returns the sum of all bonus sources
func (e *AbilityCheckChainEvent) TotalBonus() int {
	total := 0
	for _, source := range e.BonusSources {
		total += source.Bonus
	}
	return total
}

// =============================================================================
// Movement Chain Types
// =============================================================================

// MovementModifierSource tracks the source of a movement modifier.
// This is used by conditions like Disengaging to prevent opportunity attacks,
// or by features like Sentinel to stop movement.
type MovementModifierSource struct {
	Name       string    // Display name (e.g., "Disengaging", "Sentinel")
	SourceType string    // Type of source ("condition", "feature", etc)
	SourceRef  *core.Ref // Reference to the source
	EntityID   string    // ID of entity providing the modifier
}

// MovementChainEvent represents movement flowing through the modifier chain.
// This event fires BEFORE movement completes to allow OA prevention and other
// movement-related effects to be processed.
type MovementChainEvent struct {
	// Identity
	EntityID   string // ID of the moving entity
	EntityType string // Type ("character", "monster")

	// Movement details
	FromPosition Position // Starting position (grid coordinates)
	ToPosition   Position // Ending position (single step)

	// OA Prevention - conditions can add sources here to prevent OA
	OAPreventionSources []MovementModifierSource

	// Movement control - can stop movement entirely
	MovementPrevented bool   // If true, movement is blocked
	PreventionReason  string // Why movement was prevented
}

// IsOAPrevented returns true if opportunity attacks are prevented for this movement.
// A condition like Disengaging adds itself to OAPreventionSources to indicate
// that the moving entity should not provoke opportunity attacks.
func (e *MovementChainEvent) IsOAPrevented() bool {
	return len(e.OAPreventionSources) > 0
}

// Position represents a 2D grid position for movement tracking.
// This mirrors spatial.Position but avoids import cycles.
// Note: This uses float64 for compatibility with spatial.Position, but grid-based
// positions are typically exact integers. Direct equality comparison is safe for
// grid positions that come from exact assignments (not mathematical calculations).
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Equals checks if two positions are equal using direct comparison.
// This is safe for grid-based positions which are typically exact integer values.
// For positions derived from complex calculations, consider epsilon-based comparison.
func (p Position) Equals(other Position) bool {
	return p.X == other.X && p.Y == other.Y
}

// =============================================================================
// Simple Events (pub/sub notifications)
// =============================================================================

// TurnStartEvent is published when a turn begins.
//
// SubjectID is whoever is taking that turn — a player's character or a monster
// the composition drove. It is named for what it denotes rather than for what
// happens to be plugged into it today: monsters take turns, and a fight's
// unplayed members have their turns ended several at a time inside a single
// EndTurn (rpg-toolkit#1162), so a field called CharacterID here would be the
// same character-shaped mistake that got gamectx.CharacterRegistry deleted —
// "character-shaped, so it could not describe a monster at all".
//
// Round is which round of the fight this turn belongs to. It is a COORDINATE,
// not a trigger: 5e measures durations in rounds but resolves every one of them
// on a turn ("until the start of your next turn", "at the end of its turn"),
// which is why there is no round topic beside this one and no publisher wanting
// one. See rpg-project's ideas/session-combat/clock/design.md §3.1.
type TurnStartEvent struct {
	SubjectID string // whoever is taking the turn that is starting
	Round     int    // which round of the fight it belongs to
}

// TurnEndEvent is published when a turn ends. See [TurnStartEvent] for what
// SubjectID and Round mean and why they are named that way.
type TurnEndEvent struct {
	SubjectID string // whoever is taking the turn that is ending
	Round     int    // which round of the fight it belongs to
}

// DamageReceivedEvent is published when a member takes damage
type DamageReceivedEvent struct {
	TargetID   string      // ID of the member taking damage — character or monster
	SourceID   string      // ID of the attacker/source entity
	SourceRef  *core.Ref   // What caused the damage (weapon, spell, condition ref)
	Amount     int         // Amount of damage
	DamageType damage.Type // Type of damage (slashing, fire, etc)
	IsCritical bool        // True if this was a critical hit (unconscious characters take 2 death save failures)
}

// HealingReceivedEvent is published when a member receives healing
type HealingReceivedEvent struct {
	TargetID string // ID of the member receiving healing — character or monster
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

// ConditionRemovedEvent is published when a condition ends.
//
// MemberID rather than CharacterID, because both keepers subscribe: every
// condition in this package can hang on a monster as readily as on a
// character, and monstertraits.LoadJSON routes any conditions-typed ref
// straight into conditions.LoadJSON to prove it. Naming the field for one of
// the two kinds is the mistake [CombatEndEvent] documents at length, and the
// reason [ConditionStateChangedEvent] below spells its own choice out. This
// event was the last holdout in this file.
//
// A FACT, NOT A COMMAND, like its neighbours. What the publisher knows is
// that its condition ended; what a keeper does about that — drop it from the
// list, mark the sheet dirty — is the keeper's rule about its own sheet.
type ConditionRemovedEvent struct {
	MemberID     string // whose sheet carried the condition that ended
	ConditionRef string // which condition ended, as the ref its Ref() returns
	Reason       string // why, for a log that wants to say so
}

// ConditionStateChangedEvent is published by a condition whose OWN persisted
// fields changed — "my slice of your sheet changed where you cannot see it."
//
// A condition that keeps turn-scoped memory — was I hit this turn, did I
// already spend my sneak attack, have I used my reaction — stores that memory
// in its own fields, and those fields are serialized as part of the sheet it
// hangs on. Nothing about the sheet itself moves when one changes: the hit
// points are untouched, the economy is untouched, and resolution hands back
// only participants that report IsDirty. So a condition that updates itself in
// silence has its update discarded, and the only party who can see the change
// is the one making it.
//
// A FACT, NOT A COMMAND, which is what the name is for. What the condition
// knows is that its state changed. That this should mark a sheet dirty is the
// KEEPER's rule about its own sheet, not an instruction the condition gets to
// give — the same shape as [ConditionAppliedEvent] and [HealingReceivedEvent],
// which report what happened and leave the response to whoever owns the thing
// responding. A command-shaped name here would have put the keeper's policy in
// the caller, and bound every future keeper to it.
//
// MemberID rather than CharacterID, because both keepers subscribe: a monster
// carrying an opportunity attack publishes this exactly as a character does.
// Naming the field for one of the two kinds is the mistake [CombatEndEvent]
// documents at length. [ConditionRemovedEvent] above used to be the standing
// counter-example in this file and now makes the same choice.
type ConditionStateChangedEvent struct {
	MemberID     string    // whose sheet carries the condition that changed
	ConditionRef *core.Ref // which condition changed, so a log can say so
}

// AttackEvent is published when a character makes an attack (before rolls)
type AttackEvent struct {
	AttackerID string // ID of the attacking character
	TargetID   string // ID of the target
	WeaponRef  string // Reference to the weapon used
	IsMelee    bool   // True for melee attacks, false for ranged
}

// SpendRequestedEvent asks a member's keeper to debit that member's action
// economy.
//
// A REQUEST, unlike its neighbours in this file, and named for it. Everything
// else here reports something that happened; this one asks for something to
// happen, because the asker genuinely cannot do it itself. An effect reads the
// world through the cast, and what the cast hands out
// ([github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat.Member]) has no
// method that writes to a sheet. The member's own keeper holds the ledger.
//
// It is APPLIED, not adjudicated. Affordability is checked before the request
// is published — combat.Pay's contract is that a debit past a passed check
// cannot fail - so a keeper receiving this does not re-run the gate. Being
// request-shaped is only about who performs the write.
//
// A KEEPER MAY HAVE NO ROW FOR IT AT ALL, and the monster keeper does not.
// Monsters carry no action economy in this rulebook, so a monster has no
// ledger to debit and nothing to refuse; the request passes its keeper by,
// truthfully. That is the D&D asymmetry stated as which subscriptions each
// keeper's table has, rather than as a nil check inside the condition. A
// reaction-metering condition on a monster still meters itself through its own
// state — see conditions.OpportunityAttackCondition's UsedThisTurn.
//
// It supersedes a ReactionUsedEvent that had no publisher and no subscriber:
// that one named a single action type in its own topic, carried no amount, and
// identified its subject as a CharacterID, so it could serve neither a second
// currency nor the monster half. Zero users made reshaping it pointless and
// deleting it free.
type SpendRequestedEvent struct {
	MemberID   string                // whose economy is asked to pay
	ActionType coreCombat.ActionType // which slot: reaction, bonus, standard
	Amount     int                   // how many slots to debit
	SourceRef  *core.Ref             // what is asking, so a log can say so
}

// RestEvent is published when a character takes a rest
type RestEvent struct {
	RestType    resources.ResetType // Type of rest (short_rest, long_rest, etc)
	CharacterID string              // ID of the character resting
}

// CombatEndEvent is published when a fight a member was in has ended.
//
// ONE PER MEMBER, never one for the fight. Every attached effect hears every
// publish on the interaction's single bus (R1), so a subscriber answers "is
// this about me?" by comparing the subject against its own owner — which is
// exactly what conditions.RagingCondition.onCombatEnd does. A subject-less
// "the fight ended" would match nobody and expire nothing.
//
// SubjectID is whoever that fight ended for: a player's character, or a monster
// that was in it. Named for what it denotes rather than for what happens to be
// plugged into it today, for the same reason [TurnStartEvent] is — the
// composition ends a fight for everyone it held, and it holds monsters.
//
// NO ROUND, unlike the turn events. A fight ending is not a coordinate in a
// clock that no longer exists: play/clock's Turn.Dissolve sets the round back
// to zero, because round numbers are per-fight and always were. That fact is
// also why this event has to exist at all — see rpg-project's
// ideas/session-combat/combat-end/design.md §0.
//
// Combat-scoped conditions subscribe to CombatEndTopic in their Apply to remove
// themselves — RAW: rage ends when combat ends. This is an opt-in,
// per-condition lifetime: a condition that should outlive combat (e.g. a curse)
// simply does not subscribe. Mirrors the RestEvent self-termination pattern.
type CombatEndEvent struct {
	SubjectID string // whoever the fight just ended for
}

// =============================================================================
// Death Save Events
// =============================================================================

// DeathSaveRolledEvent is published when a death save is rolled for an unconscious character
type DeathSaveRolledEvent struct {
	CharacterID           string // ID of the character who rolled
	Roll                  int    // The d20 roll result (0 if from damage, not a roll)
	IsSuccess             bool   // True if the roll was 10+
	IsCriticalFail        bool   // True if the roll was 1
	IsCriticalSuccess     bool   // True if the roll was 20
	Successes             int    // Total successes after this event
	Failures              int    // Total failures after this event
	Stabilized            bool   // True if character stabilized (3 successes)
	Dead                  bool   // True if character died (3 failures)
	RegainedConsciousness bool   // True if nat 20 regained consciousness
	HPRestored            int    // HP restored (1 on nat 20, 0 otherwise)
}

// CharacterDiedEvent is published when a character accumulates 3 death save failures
type CharacterDiedEvent struct {
	CharacterID string // ID of the dead character
}

// CharacterStabilizedEvent is published when a character accumulates 3 death save successes
type CharacterStabilizedEvent struct {
	CharacterID string // ID of the stabilized character
}

// =============================================================================
// Reaction Trigger Events (Wave 2.11c)
// =============================================================================

// TriggerKind identifies which reaction window is firing.
// Used by the orchestrator (rpg-api) to decide which prompt to show.
type TriggerKind string

const (
	// TriggerKindPostHit is published after the attack chain resolves
	// (roll + hit determination against originalAC) — the Shield window.
	// The reactor may apply an AC bonus that retroactively turns a hit into a miss.
	TriggerKindPostHit TriggerKind = "post_hit"

	// TriggerKindMovementOA is published when a combatant leaves a threatened
	// square — the Opportunity Attack window.
	TriggerKindMovementOA TriggerKind = "movement_oa"

	// TriggerKindPostDamage is published after damage has been applied —
	// the Hellish Rebuke window.
	TriggerKindPostDamage TriggerKind = "post_damage"
)

// ReactionTriggerEvent is published by condition handlers when their predicate
// matches AND gamectx.IsReactionReady(reactorID, conditionRef) returns true.
//
// The chain itself runs to completion regardless; this event is additional
// output that the orchestrator (rpg-api) reads AFTER the chain returns.
//
// Wave 2.11d: per Director ruling, BOTH player AND NPC reactors publish this
// event. The orchestrator (encounter SDK wrapper) iterates the buffered
// triggers, partitions by reactor type, and either resolves NPC reactions
// inline or surfaces player reactions for prompt-driven response. Re-entrant
// chain calls from inside condition handlers are explicitly NOT used.
type ReactionTriggerEvent struct {
	// ReactorID is the character ID of the entity that can react.
	ReactorID string

	// ConditionRef identifies the reaction (e.g. "dnd5e:conditions:shield",
	// "dnd5e:conditions:opportunity_attack"). Matches the ref used to seed
	// gamectx.ReactionReadinessMap.
	ConditionRef string

	// TriggerKind identifies which reaction window fired.
	TriggerKind TriggerKind

	// SourceEntity is the entity that triggered this reaction opportunity
	// (the attacker for post-hit, the moving entity for OA, etc.).
	SourceEntity string

	// Payload carries window-specific context. The concrete type depends on
	// TriggerKind and is documented per condition:
	//   - TriggerKindPostHit: PostAttackRollEvent (the post-roll snapshot;
	//     the AttackContext lives on the resolver side and is correlated by
	//     attacker+target)
	//   - TriggerKindMovementOA: MovementChainEvent (read-only copy of the
	//     move event so the orchestrator knows mover, from/to positions)
	Payload any
}

// PostAttackRollEvent is published by Strike resolution AFTER the d20 has been
// rolled and wouldHit has been determined against the original AC, but BEFORE
// the AttackContext is returned to the caller.
//
// This is the subscription point for reactions whose predicate depends on the
// roll value and the would-hit determination — most notably Shield, which
// only fires when the attack would hit AND a +5 AC bonus would deflect it.
//
// Subscribers receive a read-only snapshot. They cannot mutate the in-flight
// roll — the AC modifier is applied in phase 2 via
// ReactionModifier when the reactor takes the reaction. Subscribers may
// publish ReactionTriggerEvents to surface a player prompt or signal the
// orchestrator that an NPC auto-resolve modifier should be applied.
type PostAttackRollEvent struct {
	// AttackerID is the entity making the attack.
	AttackerID string

	// TargetID is the entity being attacked.
	TargetID string

	// OriginalAC is the target's effective AC against this attack BEFORE any
	// reaction modifier (Shield etc.) is applied.
	OriginalAC int

	// AttackRoll is the d20 result (after advantage/disadvantage resolution).
	AttackRoll int

	// AttackBonus is the total bonus added to the roll.
	AttackBonus int

	// TotalAttack is AttackRoll + AttackBonus.
	TotalAttack int

	// WouldHit is true if TotalAttack >= OriginalAC (with natural 1/20 rules).
	WouldHit bool

	// IsNaturalTwenty is true if the natural d20 was 20 (always hits).
	IsNaturalTwenty bool

	// IsNaturalOne is true if the natural d20 was 1 (always misses).
	IsNaturalOne bool

	// HasAdvantage is true if the roll was made with advantage.
	HasAdvantage bool

	// HasDisadvantage is true if the roll was made with disadvantage.
	HasDisadvantage bool

	// AdvantageSources/DisadvantageSources name the condition(s)/feature(s)
	// that granted advantage or imposed disadvantage, for narration.
	AdvantageSources    []*core.Ref
	DisadvantageSources []*core.Ref
}

// =============================================================================
// Monk Feature Events
// =============================================================================

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

// HelpActivatedEvent is published when a character uses the Help action.
// The helper aids an ally: the next ability check or attack roll the ally makes
// (against the helped target, for attacks) gains advantage. The mechanical
// effect (granting advantage) is applied by a subscriber in a later beat; this
// event is the activation signal.
type HelpActivatedEvent struct {
	CharacterID string // ID of the character taking the Help action
	AllyID      string // ID of the ally being helped (the action's target)
}

// HideActivatedEvent is published when a character uses the Hide action.
// The character attempts to become hidden (a Stealth check vs observers' passive
// Perception). The check + the resulting Hidden condition are resolved by a
// subscriber in a later beat; this event is the activation signal.
type HideActivatedEvent struct {
	CharacterID string // ID of the character taking the Hide action
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

	// ConditionStateChangedTopic provides typed pub/sub for a condition
	// reporting a change to its own persisted state
	ConditionStateChangedTopic = events.DefineTypedTopic[ConditionStateChangedEvent](
		"dnd5e.condition.state.changed")

	// AttackTopic provides typed pub/sub for attack events
	AttackTopic = events.DefineTypedTopic[AttackEvent]("dnd5e.combat.attack")

	// SpendRequestedTopic provides typed pub/sub for action-economy debit requests
	SpendRequestedTopic = events.DefineTypedTopic[SpendRequestedEvent]("dnd5e.economy.spend.requested")

	// RestTopic provides typed pub/sub for rest events
	RestTopic = events.DefineTypedTopic[RestEvent]("dnd5e.rest")

	// CombatEndTopic provides typed pub/sub for combat-end events
	CombatEndTopic = events.DefineTypedTopic[CombatEndEvent]("dnd5e.combat.end")

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

	// HelpActivatedTopic provides typed pub/sub for Help ability activation
	HelpActivatedTopic = events.DefineTypedTopic[HelpActivatedEvent]("dnd5e.ability.help.activated")

	// HideActivatedTopic provides typed pub/sub for Hide ability activation
	HideActivatedTopic = events.DefineTypedTopic[HideActivatedEvent]("dnd5e.ability.hide.activated")

	// DeathSaveRolledTopic provides typed pub/sub for death save roll events
	DeathSaveRolledTopic = events.DefineTypedTopic[DeathSaveRolledEvent]("dnd5e.death_save.rolled")

	// CharacterDiedTopic provides typed pub/sub for character death events
	CharacterDiedTopic = events.DefineTypedTopic[CharacterDiedEvent]("dnd5e.death_save.died")

	// CharacterStabilizedTopic provides typed pub/sub for character stabilization events
	CharacterStabilizedTopic = events.DefineTypedTopic[CharacterStabilizedEvent]("dnd5e.death_save.stabilized")

	// ReactionTriggerTopic provides typed pub/sub for reaction trigger events.
	// Published by condition handlers when a reactor has a readied reaction
	// whose predicate matched. The orchestrator (encounter SDK wrapper) reads
	// these after the chain returns and either resolves NPC reactions inline
	// or surfaces player reactions for prompt-driven response (Wave 2.11d).
	ReactionTriggerTopic = events.DefineTypedTopic[ReactionTriggerEvent]("dnd5e.combat.reaction.trigger")
)

// PostAttackRollChain is a chained topic published by resolution.Strike
// AFTER the d20 has been rolled and wouldHit has been computed, BEFORE the
// AttackContext is returned. The Shield spell condition subscribes here to
// publish a ReactionTriggerEvent when a hit-but-deflectable attack lands on
// a wizard with Shield readied.
//
// Why a chain topic? The chained-topic pattern propagates the publish-time
// context to subscribers (carrying gamectx values like reaction-readiness
// and room). Typed topics in rpg-toolkit do not propagate context, which
// would cause IsReactionReady to read a stale empty context. Subscribers
// here typically do NOT modify the chain — they inspect the event and
// publish side-effect ReactionTriggerEvents. The chain stage is unused
// (ModifierStages provides the slot machinery; subscribers return the chain
// unchanged).
var PostAttackRollChain = events.DefineChainedTopic[*PostAttackRollEvent]("dnd5e.combat.attack.post_roll")

// Chain topics (for modifier chains)
var (
	// AttackChain provides typed chained topic for attack roll modifiers
	AttackChain = events.DefineChainedTopic[AttackChainEvent]("dnd5e.combat.attack.chain")

	// DamageChain provides typed chained topic for damage modifiers
	DamageChain = events.DefineChainedTopic[*DamageChainEvent]("dnd5e.combat.damage.chain")

	// SavingThrowChain provides typed chained topic for saving throw modifiers
	SavingThrowChain = events.DefineChainedTopic[*SavingThrowChainEvent]("dnd5e.saves.chain")

	// MovementChain provides typed chained topic for movement modifiers.
	// This chain fires BEFORE each step of movement to allow conditions like
	// Disengaging to prevent opportunity attacks, or features like Sentinel
	// to stop movement entirely.
	MovementChain = events.DefineChainedTopic[*MovementChainEvent]("dnd5e.combat.movement.chain")

	// AbilityCheckChain provides typed chained topic for ability check modifiers
	// (advantage/disadvantage/bonuses). Mirrors SavingThrowChain. Fired by
	// checks.MakeAbilityCheck whenever an EventBus is provided — the same
	// pattern SavingThrowChain uses, whether or not any subscriber exists.
	AbilityCheckChain = events.DefineChainedTopic[*AbilityCheckChainEvent]("dnd5e.checks.chain")
)
