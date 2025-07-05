// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

// Context key constants for common event data.
// Using constants prevents typos and ensures consistency across the toolkit.
const (
	// Combat context keys
	ContextKeyAttacker    = "attacker"     // Entity making the attack
	ContextKeyTarget      = "target"       // Entity being attacked
	ContextKeyWeapon      = "weapon"       // Weapon used in attack
	ContextKeySpell       = "spell"        // Spell being cast
	ContextKeyDamageType  = "damage_type"  // Type of damage (slashing, fire, etc.)
	ContextKeyDamageTypes = "damage_types" // Multiple damage types
	ContextKeyDamageRoll  = "damage_roll"  // Damage roll expression
	ContextKeyDamage      = "damage"       // Actual damage amount

	// Roll context keys
	ContextKeyRoll          = "roll"           // The die roll result
	ContextKeyRollModifiers = "roll_modifiers" // Modifiers applied to roll
	ContextKeyRollTotal     = "roll_total"     // Total after modifiers
	ContextKeyAdvantage     = "advantage"      // Has advantage on roll
	ContextKeyDisadvantage  = "disadvantage"   // Has disadvantage on roll
	ContextKeyCritical      = "critical"       // Is a critical hit/success
	ContextKeyFumble        = "fumble"         // Is a critical miss/failure

	// Save/Check context keys
	ContextKeyAbility         = "ability"          // Ability being used (STR, DEX, etc.)
	ContextKeyDC              = "dc"               // Difficulty class
	ContextKeySaveType        = "save_type"        // Type of saving throw
	ContextKeySkill           = "skill"            // Skill being used
	ContextKeyProficiency     = "proficiency"      // Proficiency bonus applies
	ContextKeyExpertise       = "expertise"        // Expertise (double prof) applies
	ContextKeyHalfProficiency = "half_proficiency" // Half proficiency applies

	// Effect/Condition context keys
	ContextKeyEffect        = "effect"        // Effect being applied
	ContextKeyCondition     = "condition"     // Condition being applied
	ContextKeyDuration      = "duration"      // Duration of effect
	ContextKeyConcentration = "concentration" // Requires concentration
	ContextKeySource        = "source"        // Source of effect/condition
	ContextKeyStacks        = "stacks"        // Number of stacks

	// Action context keys
	ContextKeyAction      = "action"       // Action being taken
	ContextKeyBonusAction = "bonus_action" // Is a bonus action
	ContextKeyReaction    = "reaction"     // Is a reaction
	ContextKeyMovement    = "movement"     // Movement action

	// Resource context keys
	ContextKeyResource     = "resource"      // Resource being consumed
	ContextKeyResourceCost = "resource_cost" // Amount of resource consumed
	ContextKeySlotLevel    = "slot_level"    // Spell slot level
	ContextKeySpellLevel   = "spell_level"   // Level spell is cast at

	// Turn/Time context keys
	ContextKeyRound      = "round"      // Current combat round
	ContextKeyTurn       = "turn"       // Whose turn it is
	ContextKeyInitiative = "initiative" // Initiative value
	ContextKeyTurnPhase  = "turn_phase" // Phase of turn (start, main, end)
	ContextKeyTimestamp  = "timestamp"  // When event occurred

	// Modifier context keys
	ContextKeyModifierType   = "modifier_type"   // Type of modifier
	ContextKeyModifierValue  = "modifier_value"  // Value of modifier
	ContextKeyModifierSource = "modifier_source" // Source of modifier

	// Range/Distance context keys
	ContextKeyRange     = "range"     // Range of attack/spell
	ContextKeyDistance  = "distance"  // Distance to target
	ContextKeyReach     = "reach"     // Reach of attacker
	ContextKeyCover     = "cover"     // Target has cover
	ContextKeyElevation = "elevation" // Elevation difference

	// Visibility context keys
	ContextKeyVisible     = "visible"      // Target is visible
	ContextKeyHidden      = "hidden"       // Attacker is hidden
	ContextKeyInvisible   = "invisible"    // Entity is invisible
	ContextKeyDarkness    = "darkness"     // Area is in darkness
	ContextKeyDimLight    = "dim_light"    // Area is in dim light
	ContextKeyBrightLight = "bright_light" // Area is in bright light
	ContextKeyBlind       = "blind"        // Entity is blind
	ContextKeyDarkvision  = "darkvision"   // Entity has darkvision
	ContextKeyTruesight   = "truesight"    // Entity has truesight

	// Status context keys
	ContextKeyProne         = "prone"         // Entity is prone
	ContextKeyGrappled      = "grappled"      // Entity is grappled
	ContextKeyRestrained    = "restrained"    // Entity is restrained
	ContextKeyIncapacitated = "incapacitated" // Entity is incapacitated
	ContextKeyStunned       = "stunned"       // Entity is stunned
	ContextKeyParalyzed     = "paralyzed"     // Entity is paralyzed
	ContextKeyPetrified     = "petrified"     // Entity is petrified
	ContextKeyUnconscious   = "unconscious"   // Entity is unconscious
	ContextKeyExhaustion    = "exhaustion"    // Exhaustion level

	// Equipment context keys
	ContextKeyArmor      = "armor"      // Armor worn
	ContextKeyShield     = "shield"     // Shield equipped
	ContextKeyMainHand   = "main_hand"  // Main hand item
	ContextKeyOffHand    = "off_hand"   // Off hand item
	ContextKeyTwoHanded  = "two_handed" // Using two-handed weapon
	ContextKeyAmmunition = "ammunition" // Ammunition type

	// Misc context keys
	ContextKeyDescription = "description"  // Human-readable description
	ContextKeyReason      = "reason"       // Reason for event
	ContextKeyOverride    = "override"     // Override normal rules
	ContextKeyAutoFail    = "auto_fail"    // Automatically fail
	ContextKeyAutoSuccess = "auto_success" // Automatically succeed
)
