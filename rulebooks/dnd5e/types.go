// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dnd5e

import (
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Entity type constants for D&D 5e
const (
	Module = "dnd5e"

	EntityTypeCondition core.EntityType = "condition"
	EntityTypeCharacter core.EntityType = "character"
	EntityTypeMonster   core.EntityType = "monster"
	EntityTypeNPC       core.EntityType = "npc"
	EntityTypeFeature   core.EntityType = "feature"
	EntityTypeItem      core.EntityType = "item"
	EntityTypeSpell     core.EntityType = "spell"
)

// Size identifies a D&D 5e creature's combat size category. Size describes
// the space a creature controls, not its attack reach or its exact body shape.
// All sizes currently occupy one hex in the toolkit.
type Size string

const (
	SizeTiny       Size = "tiny"
	SizeSmall      Size = "small"
	SizeMedium     Size = "medium"
	SizeLarge      Size = "large"
	SizeHuge       Size = "huge"
	SizeGargantuan Size = "gargantuan"
)

// NormalizeSize returns a supported size. An omitted or unknown value safely
// falls back to Medium, preserving compatibility with existing saved data.
func NormalizeSize(size Size) Size {
	switch Size(strings.ToLower(string(size))) {
	case SizeTiny:
		return SizeTiny
	case SizeSmall:
		return SizeSmall
	case SizeMedium:
		return SizeMedium
	case SizeLarge:
		return SizeLarge
	case SizeHuge:
		return SizeHuge
	case SizeGargantuan:
		return SizeGargantuan
	default:
		return SizeMedium
	}
}

// Sized is implemented by D&D 5e creatures that expose a combat size category.
// It is separate from core.Entity so rulebook-neutral entities such as items,
// doors, and walls do not need a D&D-specific size.
type Sized interface {
	Size() Size
}
