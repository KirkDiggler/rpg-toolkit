// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dungeonspec compiles an authored dungeon into a world the
// composition can run (rpg-toolkit#1127).
//
// # Where this lives, and why it is here rather than in the host
//
// The yaml is content and the target is [encounter.FieldInput], so the compiler
// could plausibly sit on either side of that seam. It sits here because the
// toolkit owns geometry: rpg-api's own words for the arrangement it already has
// with the old stack are that "rpg-api never computes dungeon geometry itself —
// a key maps to a builder, and only the toolkit turns that spec into an actual
// Space". The old stack's compiler is a subpackage of the module whose runtime
// it feeds; this is that arrangement, one stack over.
//
// # What it may not know
//
// This package compiles GEOMETRY and carries everything else. It resolves no
// content: "dnd5e:monsters:skeleton" comes out the far end as the same string
// that went in, and so does "lowest-health". That is not squeamishness — the
// composition may not import a rulebook (design law C1, the reason Standing,
// Sight and Decider are injected at all), and a compiler that resolved refs
// here would drag one in. Refs become sheets one layer up, where a package may
// import both.
//
// So the output is deliberately in two halves: a [encounter.FieldInput] that is
// ready to hand to [encounter.NewEncounter] as it stands, and a roster of
// placements that still need somebody who knows what a skeleton is.
package dungeonspec

// Spec is the authored dungeon, decoded and not yet checked. Every field is
// exactly what the file said; nothing is defaulted, derived, or repaired.
//
// # Pointers mean "the author may leave this out"
//
// Where a field is a pointer, an omitted value and an explicitly written one
// are different facts and the compiler must be able to tell them apart. The
// coffin is the reason: `blocks_los: false` is an authored exception, and a
// plain bool would make it indistinguishable from a placement that said nothing
// — which is how the reference tomb's one see-over-able prop would silently
// become a wall.
type Spec struct {
	// Version is the dialect this file is written in. Exactly one is
	// understood; an unknown one is refused rather than read hopefully,
	// because a file from a dialect this build does not speak is not a file
	// it can guess at.
	Version int `yaml:"version"`

	// Key is the dungeon's identifier, used to name the doors this compile
	// mints so two dungeons in one process cannot collide.
	Key string `yaml:"key"`

	// Void is what the space between the chambers does to a sightline —
	// "opaque" or "transparent", the words [encounter.Void] carries.
	//
	// REQUIRED, and this is the field most likely to be mistaken for
	// decoration. [encounter.CanvasInput.Void] has no default by ruling, so
	// somebody has to say it, and the only honest somebody is the author:
	// a tomb cut from rock and an open-air ruin are the same rooms with
	// different walls between them, and nothing in the room list can tell
	// them apart.
	Void string `yaml:"void"`

	// Orientation is which way the hexes point — "pointy" or "flat".
	//
	// REQUIRED, because every `at:` pair in this file is an OFFSET
	// coordinate, and an offset coordinate means nothing until the
	// orientation is known: the same [col,row] is a different hex under each.
	// Kirk's ruling: "flat and pointy top are both valid and should be
	// settable."
	Orientation string `yaml:"orientation"`

	// Height is how tall every chamber is, in cells. One number for the
	// dungeon rather than one per room, which is what makes a chain of
	// chambers share a seam at all.
	Height int `yaml:"height"`

	// Start is where the party comes in: an absolute [col,row] cell.
	//
	// REQUIRED and explicit. The dialect this replaces derived it from a
	// chamber's `archetype: entrance`, which is the shape of defaulting
	// rpg-toolkit#1033 forbids — a word describing what a room is FOR,
	// silently deciding where people stand. An author who wants the party in
	// the entrance says which cell of it.
	//
	// The compiler resolves which chamber holds this cell rather than being
	// told, because it already knows: the chambers' footprints are what it
	// just computed, and asking the author to name the room as well would be
	// a second fact that can disagree with the first.
	//
	// A POINTER because [0,0] is a legal cell — the north-west corner of the
	// first chamber — so a plain value could not tell "the author said corner"
	// apart from "the author said nothing", and the second would silently
	// become the first. Same argument as a prop's blocking flags, one field up.
	Start *[2]int `yaml:"start"`

	// Rooms are the chambers, IN LAYOUT ORDER: each one sits immediately
	// east of the one before it. Declaration order is geometry here, not
	// presentation.
	Rooms []RoomSpec `yaml:"rooms"`

	// Connectors are the openings between neighbouring chambers.
	Connectors []ConnectorSpec `yaml:"connectors"`
}

// RoomSpec is one chamber.
type RoomSpec struct {
	// ID names the chamber, and survives the compile as the region's ID.
	ID string `yaml:"id"`

	// Width is the chamber's horizontal extent in cells. Its height is the
	// dungeon's.
	Width int `yaml:"width"`

	// Place is everything standing in the chamber — props, monsters, and the
	// boss alike, routed by the ref's type segment rather than by which list
	// they were written in.
	Place []PlaceSpec `yaml:"place,omitempty"`
}

// PlaceSpec is one authored placement, room-local.
type PlaceSpec struct {
	// Ref is content's identifier, "module:type:id". Its TYPE segment routes
	// the placement: "props" becomes a prop, "monsters" becomes a member.
	// Nothing else about it is read.
	Ref string `yaml:"ref"`

	// At is the room-local cell, offset [col,row].
	At [2]int `yaml:"at"`

	// BlocksMovement is whether a prop stops somebody standing there.
	//
	// REQUIRED on every prop, and REFUSED on anything else. The dialect this
	// replaces read a missing value as `true`, and [encounter.PropInput]'s
	// own doc comment names that default as the thing it drops: "a blocker
	// nobody declared is a wall nobody drew". The composition refuses a nil
	// pointer one layer down, so a default here would be exactly the move
	// rpg-toolkit#1033 forbids, relocated to where it is harder to see.
	//
	// It costs an author two lines per prop. All four combinations are real
	// content — a pillar blocks both, this tomb's coffin is seen over but not
	// walked through, a curtain is the reverse, candles block neither — so
	// there is no combination the omission could have safely stood for.
	BlocksMovement *bool `yaml:"blocks_movement,omitempty"`

	// BlocksLoS is whether a prop stops a sightline. REQUIRED on every prop
	// and refused on anything else, for BlocksMovement's reason.
	BlocksLoS *bool `yaml:"blocks_los,omitempty"`

	// Targeting is the author's word for how a monster picks a target.
	// Monsters only. Carried opaquely and never interpreted here: what it
	// MEANS is a rule, and rules live in the rulebook.
	Targeting *string `yaml:"targeting,omitempty"`

	// Boss marks the monster whose death ends things. Monsters only, and at
	// most one per chamber.
	//
	// A FLAG RATHER THAN ITS OWN KEY, which is Kirk's question answered: the
	// dialect this replaces gave a chamber a `boss:` entry beside its
	// `place:` list, and the two then needed the same rules about cells,
	// refs, targeting and collisions written twice. A boss is a monster with
	// a flag. What it is FOR — which member a host watches — is a fact worth
	// carrying, and [MonsterPlacement.Boss] carries it.
	Boss bool `yaml:"boss,omitempty"`
}

// ConnectorSpec is an opening between two neighbouring chambers.
type ConnectorSpec struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`

	// Locked, when present, makes the opening a locked door rather than a
	// gap. Nil is a gap nobody can shut.
	Locked *LockSpec `yaml:"locked,omitempty"`
}

// LockSpec is the check that opens a locked connector.
//
// Both fields are carried to the composition opaquely — [encounter.Lock] never
// inspects an ability either, because "does a DEX check of 12 succeed" is a
// rule and the composition holds none.
type LockSpec struct {
	DC      int    `yaml:"dc"`
	Ability string `yaml:"ability"`
	Tool    string `yaml:"tool,omitempty"`
}
