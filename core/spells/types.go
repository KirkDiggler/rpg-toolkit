// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spells provides type definitions for spell-related constants.
// These types are used by rulebooks to define spell identifiers, schools,
// components, and range types for magical abilities.
package spells

// ID identifies a specific spell.
// Rulebooks define constants of this type for their spells.
// Example: const Fireball ID = "fireball"
type ID string

// School identifies the school of magic a spell belongs to.
// Example: const SchoolEvocation School = "evocation"
type School string

// ComponentType identifies spell components required.
// Example: const ComponentVerbal ComponentType = "verbal"
type ComponentType string

// Common component types used in many systems
const (
	ComponentVerbal   ComponentType = "verbal"
	ComponentSomatic  ComponentType = "somatic"
	ComponentMaterial ComponentType = "material"
	ComponentFocus    ComponentType = "focus"
)

// TargetType identifies what a spell can target.
// Example: const TargetCreature TargetType = "creature"
type TargetType string

// RangeType identifies the range category of a spell.
// Example: const RangeTouch RangeType = "touch"
type RangeType string

// Common range types
const (
	RangeSelf    RangeType = "self"
	RangeTouch   RangeType = "touch"
	RangeShort   RangeType = "short"
	RangeMedium  RangeType = "medium"
	RangeLong    RangeType = "long"
	RangeSight   RangeType = "sight"
	RangeSpecial RangeType = "special"
)
