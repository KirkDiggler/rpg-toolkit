// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package combat provides type definitions for combat-related constants.
// These types are used by rulebooks to define their combat mechanics.
package combat

// AttackType identifies the type of attack being made.
// Example: const MeleeWeapon AttackType = "melee_weapon"
type AttackType string

// Common attack types used in many systems
const (
	AttackMeleeWeapon  AttackType = "melee_weapon"
	AttackRangedWeapon AttackType = "ranged_weapon"
	AttackMeleeSpell   AttackType = "melee_spell"
	AttackRangedSpell  AttackType = "ranged_spell"
	AttackSpecial      AttackType = "special"
)

// WeaponProperty identifies special properties of weapons.
// Example: const PropertyFinesse WeaponProperty = "finesse"
type WeaponProperty string

// ArmorType identifies categories of armor.
// Example: const ArmorLight ArmorType = "light"
type ArmorType string

// Common armor types
const (
	ArmorNone    ArmorType = "none"
	ArmorLight   ArmorType = "light"
	ArmorMedium  ArmorType = "medium"
	ArmorHeavy   ArmorType = "heavy"
	ArmorShield  ArmorType = "shield"
	ArmorNatural ArmorType = "natural"
)

// ActionType identifies what kind of action something requires.
// Example: const ActionStandard ActionType = "action"
type ActionType string

// Common action types
const (
	ActionStandard ActionType = "action"
	ActionBonus    ActionType = "bonus_action"
	ActionReaction ActionType = "reaction"
	ActionFree     ActionType = "free"
	ActionMovement ActionType = "movement"
)
