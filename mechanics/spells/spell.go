// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// TargetType defines how a spell targets entities.
type TargetType string

const (
	// TargetSelf indicates the spell targets the caster.
	TargetSelf TargetType = "self"
	// TargetCreature indicates the spell targets a creature.
	TargetCreature TargetType = "creature"
	// TargetObject indicates the spell targets an object.
	TargetObject TargetType = "object"
	// TargetPoint indicates the spell targets a point in space.
	TargetPoint TargetType = "point"
	// TargetArea indicates the spell targets an area.
	TargetArea TargetType = "area"
)

// AreaShape defines the shape of an area of effect.
type AreaShape string

const (
	// AreaCone represents a cone-shaped area of effect.
	AreaCone AreaShape = "cone"
	// AreaCube represents a cube-shaped area of effect.
	AreaCube AreaShape = "cube"
	// AreaLine represents a line-shaped area of effect.
	AreaLine AreaShape = "line"
	// AreaSphere represents a sphere-shaped area of effect.
	AreaSphere AreaShape = "sphere"
)

// AreaOfEffect describes the area a spell affects.
type AreaOfEffect struct {
	Shape  AreaShape
	Size   int    // radius, side length, etc. in feet
	Origin string // "self", "point", "target"
}

// CastingComponents describes what components a spell requires.
type CastingComponents struct {
	Verbal    bool
	Somatic   bool
	Material  bool
	Materials string // Description of material components
	Consumed  bool   // Whether materials are consumed
	Cost      int    // GP value of materials
}

// Spell represents a magical spell that can be cast.
type Spell interface {
	core.Entity

	// Basic Properties
	Level() int
	School() string
	CastingTime() time.Duration
	Range() int // in feet, -1 for self, 0 for touch
	Duration() events.Duration
	Description() string

	// Components
	Components() CastingComponents

	// Casting Properties
	IsRitual() bool
	RequiresConcentration() bool
	CanBeUpcast() bool

	// Targeting
	TargetType() TargetType
	AreaOfEffect() *AreaOfEffect // nil if not an area spell
	MaxTargets() int             // -1 for unlimited

	// Cast the spell
	Cast(context CastContext) error
}

// CastContext provides context for casting a spell.
type CastContext struct {
	Caster    core.Entity
	Targets   []core.Entity
	Point     *Point // For area spells cast at a point
	SlotLevel int    // Level of slot used (for upcasting)
	Bus       events.EventBus
	Metadata  map[string]interface{} // Additional casting data
}

// Point represents a location in 3D space.
type Point struct {
	X, Y, Z int
}

// SpellSave describes a saving throw against a spell.
type SpellSave struct {
	Ability string // "strength", "dexterity", etc.
	DC      int
	Effect  string // "half", "negates", etc.
}

// SpellAttack describes a spell attack.
type SpellAttack struct {
	Bonus     int
	Advantage bool
	CritRange int // Natural roll to crit (usually 20)
}
