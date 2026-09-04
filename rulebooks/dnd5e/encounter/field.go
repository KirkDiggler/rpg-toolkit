// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MemberKind categorizes whether a member is a player, a monster, or a
// placed world NPC.
type MemberKind string

const (
	// KindPlayer indicates a player-controlled member.
	KindPlayer MemberKind = "player"

	// KindMonster indicates a monster member.
	KindMonster MemberKind = "monster"

	// KindWorld indicates a placed, non-combatant world NPC (rpg-toolkit#1404).
	//
	// Carries no content of its own — no ref, no capabilities, no policy.
	// Placed with exactly the same bare facts any member is (ID, Name,
	// Position, SpeedFeet, SightFeet, Actions, Targeting); the actual NPC
	// content a KindWorld member was spawned from lives at the session
	// layer, keyed by member ID, exactly parallel to how a monster's sheet
	// never crosses into this package either.
	//
	// Being non-combatant is not a policy this package reads — it is
	// structural. sidesInContactOrder's switch (trigger.go) has no default
	// case, so a member whose Kind is neither KindPlayer nor KindMonster
	// already falls into neither side and never enters classify's engaged
	// set; Pump's monster loop is filtered to KindMonster explicitly. A
	// KindWorld member needs no case added anywhere in either to be
	// excluded — it already is, by construction.
	KindWorld MemberKind = "world"
)

// MemberID is an alias for core.EntityID used for clarity in member contexts.
type MemberID = core.EntityID

// Lighting is a region's light level, carried through the composition unread
// (rpg-project#256).
//
// ONE FIELD TODAY, a block so later fields land beside it without reshaping
// anything. What an intensity MEANS for sight — bright, dim, dark, darkvision
// — is a rule, and rules live in the rulebook: this composition stores the
// number, reports it on the [Atlas], and never branches on it, exactly as it
// carries a monster's Targeting word or a prop's Ref. It lands on the REGION
// rather than the canvas because light is a fact about an area, not about the
// space between areas (closes rpg-toolkit#1113 by relocation).
type Lighting struct {
	// Intensity is the light level in [0,1]: 0 is no light, 1 is full light.
	// Refused outside that range at construction (ErrNoField).
	Intensity float64
}

// RegionInput is one named set of cells — the only thing that makes OWNED
// floor (rpg-project#256).
//
// # The regions are the floor that belongs to somebody
//
// There is no room, no rectangle, no anchor and no connection any more. A
// region lists the cells it owns, absolute, and the field's floor is the union
// of every region's cells plus [FieldInput.Scenery], the cells that belong to
// nobody (rpg-project#360); every other cell on the canvas is void. Only a
// region's cells are STANDABLE, and only a region's cells carry lighting, an
// archetype and concealment — those are what having an owner means. A cell in
// two regions is refused (W2, ErrRegionOverlap), a region with no cells is
// refused (ErrRegionEmpty), and there is no other floor.
//
// This replaces [RoomInput]'s Width x Height rectangle plus Origin, and
// [ConnectionInput] with it. A rectangle-plus-anchor was the room CHAIN's
// vocabulary: it could only say "a box, here", and the seam between two boxes
// had to be walled by generating edges the author never wrote. A connection
// was the chain's way of naming the one edge it left out. With the floor
// painted cell by cell, a wall is an edge somebody drew and a doorway is two
// adjacent floor cells with a [DoorInput] on the edge between them — which is
// what they always were on the canvas (rpg-toolkit#1106).
//
// # Every cell is an absolute offset pair
//
// Cells are authored as offset [col,row] under [CanvasInput.Orientation] and
// converted ONCE, at construction, through [HexCellAt] — the one conversion in
// this package. No caller ever adds an origin, and the room-local → absolute
// seam (rpg-toolkit#1139) ceases to exist rather than getting fixed. Every cell
// a VERB reports or accepts is absolute and axial.
//
// # What a region carries, and what it may not decide
//
// Archetype and Lighting are per-area world facts, carried unread. An
// ARCHETYPE NEVER DECIDES MECHANICS — not the start, not blocking, not sight,
// not intensity. v1's archetype chose where the party stood, which is the
// rpg-toolkit#1033 trap (a word about what a room is FOR silently deciding
// geometry), and it is the reason that dialect was deleted. Both are REQUIRED
// and neither has a default, for #1033's reason in the other direction: a
// region the assets cannot dress and a region whose light nobody stated are
// both facts this module may not invent.
type RegionInput struct {
	// ID is the region's unique identifier, and the name [Encounter.RegionAt]
	// answers with.
	ID string

	// Name is the region's display name — "The Hall" — carried verbatim and
	// never read here. Optional.
	Name string

	// Cells is every cell this region owns, as authored offset [col,row]
	// pairs under the field's orientation. REQUIRED non-empty. Integral,
	// within ±maxAnchorCoord, and no cell may appear twice here or in any
	// other region.
	Cells []spatial.Position

	// Archetype is the presentation profile the assets resolve for this
	// region — "crypt" — REQUIRED non-empty, carried unread, and NEVER a
	// mechanic. See the type's doc comment.
	Archetype string

	// Lighting is the region's light level. REQUIRED: a nil pointer is
	// refused at construction (ErrRegionLightingMissing), and there is no
	// default.
	Lighting *Lighting

	// Concealed is whether this region is authored as hidden space — the
	// room that "appears to be a wall unless it is found" (rpg-project#351:
	// the room hides with its door). DECLARED, never cascaded from a
	// concealed door: the two are separate authored facts, moved together
	// by choice, and the content compiler refuses the incoherent
	// combinations before they reach this seam.
	//
	// CARRIED, NOT INTERPRETED — [DoorInput.Concealed]'s law: nothing in
	// this composition's geometry or projection reads it; withholding a
	// concealed region from a non-knower's atlas is the world layer's work
	// (wave 1b). False is the zero value telling the truth: a region that
	// said nothing was never concealed, which is every region authored
	// before concealment existed.
	Concealed bool
}

// PropInput is one thing standing in a room that is not a creature: a pillar,
// a coffin, an altar, a bank of candles.
//
// IT SAYS WHAT IT IS AND WHAT IT DOES, and until rpg-toolkit#1128 it said
// neither. A room's contents were bare cells called occluders, and the module
// decided their behaviour for them: every one blocked line of sight, none
// blocked movement, both hardcoded. That is the inverse of nearly everything a
// dungeon contains. Measured before the fix, on a chamber with a coffin across
// the middle: a member stood INSIDE the coffin, stepped out through its far
// side, and could not see the wight beyond it. A pillar you can walk through is
// not a pillar.
//
// # The ref is opaque, and that is the point
//
// [PropInput.Ref] is content's word for what this is — "dnd5e:props:pillar" —
// and nothing in this module ever reads it. It exists so the map can say WHICH
// thing is where: the multi-room census (rpg-project#227) recorded that "a
// pillar and a statue are the same cell", and a client that cannot tell them
// apart cannot draw the room. Behaviour comes from the two flags, never from
// the ref; a module that switched on ref strings would be holding a fact about
// a world, which is the overreach [Void]'s doc comment is about.
//
// # Both answers are required, neither is defaulted
//
// The flags are pointers because a prop must SAY what it does. This is
// rpg-toolkit#1033's capabilities-supplied-never-defaulted law, and [Void]'s
// own argument applied one type over: a nil pointer is obviously absent, where
// a false bool is a legal-looking answer nobody wrote. All four combinations
// are real content, so there is no combination the zero value could safely
// stand for:
//
//   - both — a pillar, a statue: go round it, cannot see past it.
//   - movement only — the reference tomb's coffin, a low altar: seen over,
//     not walked through. Authored `blocks_los: false`.
//   - sight only — a curtain, a fog bank: walked through blind.
//   - neither — candles, a rug: present, drawn, in nobody's way.
//
// The old top-level encounter module's authoring dialect carries the same two
// flags as pointers (dungeonspec's ObstacleEntry) with a documented nil-means-
// true default. The default is what this drops: a blocker nobody declared is a
// wall nobody drew.
//
// # What a movement-blocking prop denies
//
// A PLACE TO STAND, not a path. [Encounter.Step] deliberately does not check
// adjacency — that is a rule about walking and it lives with the walk
// ([StepOutput.Doors]' doc comment) — so a step is a placement question, and a
// solid prop refuses to be stood on (ErrBadPlacement). A seam that walks a path
// cell by cell asks once per cell and gets the wall it expects. A prop is not a
// [spatial.Boundary]: a wall is an edge BETWEEN cells and stops a crossing, a
// prop occupies a cell and stops an arrival.
//
// # Sight, and why one prop is not a wall
//
// A LONE PROP BLOCKS NOBODY even with BlocksLineOfSight set, and that is
// spatial v0.9.1's rule rather than this module's: sight is a LANE, blocked
// only when the direct lane and every lane from a neighbour closer to the
// target are obstructed, so a viewer leans around a single occupied cell
// exactly as they would at the table (testwalls_test.go's own finding, and
// rpg-toolkit#1022 behind it). Occluding ENTITIES can be leaned around;
// boundary EDGES cannot. So a prop that must genuinely stop a sightline needs
// width — or it wants to be a wall.
type PropInput struct {
	// ID is the author's name for this placement (rpg-project#368, design
	// P2). Optional and, like Ref, never interpreted: a prop nothing binds
	// to and nobody can pick up needs no name, and the empty string carries
	// forward to [AtlasProp.ID] as the fact that it has none.
	//
	// REQUIRED WHEN Holdable, refused at construction otherwise (ErrNoField).
	// Two things name a prop by id and both say it out loud: a scenario
	// binds an artifact by id, and the `held` beat says which thing was
	// picked up. A holdable prop with no id would be advertised to every client
	// as something anybody can pick up while no [HoldInput] could ever name
	// it — an offer with nothing behind it. dungeonspec refuses it at the
	// file and [compileField] refuses it again here, so a host assembling a
	// field by hand cannot produce one either.
	ID PropID

	// Holdable is whether a member can pick this prop up (design §5).
	// Optional, and FALSE IS THE HONEST ZERO VALUE here — unlike the two
	// blocking flags below, which are pointers because all four of their
	// combinations are real content. There is only one thing "said nothing
	// about holdable" can mean: a thing nobody declared holdable stays
	// scenery, which is what every prop that existed before this field did.
	Holdable bool

	// Ref is content's identifier for this thing, e.g.
	// "dnd5e:props:pillar". REQUIRED and never inspected here — see the
	// type's doc comment.
	Ref string

	// At is where it stands: an ABSOLUTE authored offset [col,row] pair under
	// the field's orientation, converted once at construction. Must be an
	// integral cell that some region owns — a prop is a cell OF the floor
	// (ErrBadPlacement otherwise), and two props may not share one.
	At spatial.Position

	// BlocksMovement is whether a member can end a step on this cell.
	// REQUIRED — a nil pointer is refused at construction (ErrNoField), and
	// there is no default. See the type's doc comment.
	BlocksMovement *bool

	// BlocksLineOfSight is whether a sightline is obstructed by it — subject
	// to the lane rule, so one cell of it obstructs nothing on its own.
	// REQUIRED, for the same reason and with the same refusal.
	BlocksLineOfSight *bool

	// Facing is the authored direction this prop faces, one of the EIGHT
	// true-compass names — n|ne|e|se|s|sw|w|nw — valid under BOTH hex
	// orientations (rpg-project#272 superseded #261's orientation-scoped
	// six-name sets: compass directions live in world space). Optional: ""
	// means the asset's own default facing. dungeonspec validates the word;
	// THIS MODULE NEVER DOES, and never turns it into an angle — angle math
	// is a render concern, not this module's (ADR-0040's spirit that the
	// wire/model names facts and the client derives pixels). Not a pointer,
	// unlike the two flags above: a prop's facing is a presentational fact
	// rather than a required answer, so "said nothing" and "said the
	// default" are the same by design.
	Facing string

	// Offset is an authored visual displacement: [x,y] within-cell nudge
	// fractions of the cell size, each in [-0.5, 0.5], plus a THIRD
	// component — height above the floor in the same cell-size unit, in
	// [0, 3] and deliberately not bound to its siblings' planar clamp
	// (rpg-project#272: "height should be able to gun higher than the 5
	// ticks we allow on x and y"). Optional: the zero value means centered
	// on the floor, which omitting the field also means — the two are the
	// same fact by design, for Facing's reason. VISUAL ONLY (Kirk,
	// rpg-project#261: "offset is visual only, agreed") — a prop still
	// occupies its whole cell for [Encounter.Step] and for sight; this
	// field never reaches Sight, Standing, or the turn loop, the same law
	// Facing follows.
	Offset [3]float64
}

// WallInput is one authored wall: the crossing it blocks, plus the
// presentational facts the crossing carries. The mechanics half is the
// embedded [spatial.Boundary] — spatial stays the layer that knows what a
// boundary DOES, and deliberately never learns what one looks like
// (rpg-project#273: height is visual-only by ruling, so it lives here at the
// composition, the same layering call that put Facing and Offset on
// [PropInput] rather than on a spatial type).
type WallInput struct {
	// Boundary is the crossing itself: endpoints in the AUTHORED offset
	// frame, plus what it blocks. See [FieldInput.Walls] for the frame and
	// adjacency contract.
	spatial.Boundary

	// Height is the authored wall-height MULTIPLIER of the standard
	// rendered wall height, in [1, 3] when authored — raise-only by ruling
	// (rpg-project#273: "I am looking to raise the walls not lower them").
	// 0 means not authored: a reader renders the standard height, exactly
	// as if 1.0 were written — the two are the same fact by design, and
	// nothing may multiply by the raw value. dungeonspec validates the
	// bounds; this module carries the number unread, VISUAL ONLY: height
	// never reaches Sight or movement — a wall cannot be seen past at any
	// height (Kirk's ruling, rpg-project#274).
	Height float64
}

// AxialPointF is a point in FRACTIONAL axial coordinates: the frame every
// cell on this map already lives in, with the halves a wall endpoint needs.
//
// The unit a wall's shape crosses the seam in (rpg-project#360, design §5.2).
// A client's axial-to-world formula already accepts fractions, so no second
// basis and no world unit ever leaves the compiler — the two hex bugs this
// workspace has paid for (rpg-toolkit#1141, #1150) were both a second reading
// of one basis, and there is deliberately no second reading here.
type AxialPointF struct {
	// Q and R are the axial coordinates, which may be halves: a wall ends at
	// a side midpoint or a centre, and a side midpoint is exactly half a step
	// from the centre it belongs to.
	//
	// Tagged lowercase to match [spatial.Position] and everything else that
	// crosses a beat payload, so a reveal's segments decode into the session
	// mirror's own types the way its props and boundaries already do.
	Q float64 `json:"q"`
	R float64 `json:"r"`
}

// SegmentInput is one authored wall AS THE LINE IT IS: two ends, a height, the
// floor it stands on, and the doors that open in it.
//
// PRESENTATION, AND THE COMPILER'S ANSWER TO IT. [FieldInput.Walls] is the
// mechanical truth — the crossings nobody may take — and this is the same wall
// as a thing to draw. Both are derived from one authored line by the compiler,
// which is what makes it impossible for them to disagree; before this the
// client chained crossings back into runs under a straightness tolerance, and
// a rendering constant decided where one wall ended and the next began.
//
// This module reads Height and Footprint and DRAWS NOTHING. It carries no
// geometry of its own and never will: a hex is embedded in the plane in
// exactly one place, the authoring compiler (design C9).
type SegmentInput struct {
	// Name is the author's word for the wall, carried unread.
	Name string

	// From and To are the wall's two ends, in fractional axial.
	From, To AxialPointF

	// Height is the authored wall-height multiplier, 0 for not authored —
	// [WallInput.Height]'s contract exactly, at the scale the author wrote it.
	Height float64

	// Footprint is every floor cell the wall passes through, in absolute
	// authored offset [col,row] cells, in order along the wall.
	//
	// WHAT IT IS FOR: a wall presented to somebody who cannot see the room
	// behind it still has to stand on something, or the floor ends in nothing
	// and the wall marks itself (design C18). These cells enter that
	// recipient's atlas as floor nobody owns.
	Footprint []spatial.Position

	// DoorIDs is every door standing in this wall, in crossing order.
	// Carried so a door's masquerade can take the height of the wall it hides
	// in rather than guess at a neighbour's.
	DoorIDs []DoorID
}

// FieldInput describes the map: what the canvas declares, the regions that
// make its floor, and the props, walls and doors standing on it
// (rpg-project#256).
type FieldInput struct {
	// Canvas is what this field DECLARES about the map its regions paint:
	// what the space between them does to a sightline, and which way its
	// hexes point. Both REQUIRED: see [Void] and [Orientation] for why this
	// module is not allowed to pick either (rpg-toolkit#1116, #1127).
	Canvas CanvasInput

	// Regions are the named cell sets whose union is the OWNED floor.
	// REQUIRED non-empty. See [RegionInput].
	Regions []RegionInput

	// Scenery is floor nobody stands on: absolute authored offset [col,row]
	// cells that belong to no region (rpg-project#360, wall-geometry design
	// §1.4). Optional; omitted means none.
	//
	// A CELL CARRIES TWO FACTS, AND THIS IS WHAT SPLITS THEM. Before scenery
	// the region mask answered everything at once — who owns this cell, is it
	// floor, may a wall stand on it, does a sightline stop here, may somebody
	// stand here. Scenery has the second, third and fourth without the first
	// and the fifth:
	//
	//   - FLOOR is [FieldInput.Regions] ∪ Scenery. A wall's endpoint may be
	//     scenery, a door's edge may cross it, a prop may sit on it, and it
	//     is in [Atlas.Cells].
	//   - STANDABLE is the regions alone. A member seat, a step and an
	//     ending's trigger cell are all refused on scenery exactly as they
	//     are refused in the void.
	//   - OWNER decides visibility and meaning — concealment, lighting,
	//     archetype — and scenery has none, so it is in EVERY member's
	//     [Encounter.AtlasFor] and carries no light of its own.
	//   - TRANSPARENT REGARDLESS of [CanvasInput.Void]: scenery is floor, and
	//     the void declaration is about the space BETWEEN the floor.
	//
	// Refused when a cell is not a representable integral cell, is listed
	// twice, or is already a region's (ErrRegionOverlap — ownership has to be
	// unique for [Encounter.RegionAt] to be an answer rather than a guess,
	// and "no owner" is an answer too).
	Scenery []spatial.Position

	// Props are the things standing on the floor that are not creatures, in
	// absolute authored cells. Optional. See [PropInput].
	Props []PropInput

	// Walls are the authored edges between adjacent floor cells that block
	// movement and sight, with both endpoints as absolute authored offset
	// [col,row] pairs (rpg-project#256 moved these up from the room, where
	// they were room-local and compiled through an origin).
	//
	// WHICH CELLS ARE ADJACENT IS THE GRID'S ANSWER under the declared
	// orientation, not a pair of offsets to hardcode: in the authored frame
	// the neighbours of a cell STAGGER with the row's or column's parity, so
	// the same pair is a crossing under one layout and not the other. Refused
	// when the endpoints are not adjacent (ErrEdgeNotAdjacent), when either is
	// not floor (ErrEdgeOffFloor — the envelope is implied, never written), or
	// when the same edge is listed twice.
	//
	// Endpoint ORDER is not carried: spatial normalizes an undirected pair on
	// registration, so From and To describe the same edge either way round.
	Walls []WallInput

	// Segments are the authored walls as LINES, one per wall the author drew,
	// for a reader that draws them. Optional and inert: nothing in this module
	// decides anything from a segment except what a masquerade's height is and
	// which floor a presented wall stands on. See [SegmentInput].
	Segments []SegmentInput

	// Sealed is every cell some wall leaves too little of to stand on: a cell
	// that KEEPS ITS OWNER and loses its feet (rpg-project#360, design C10).
	// Absolute authored offset [col,row] cells. Optional; omitted means none.
	//
	// DERIVED BY THE COMPILER AND CARRIED, never recomputed here. The rule is
	// an area fraction of a hex clipped by half-planes, which is geometry, and
	// geometry lives in exactly one place (design C9). What this module needs
	// is the answer, and this is the answer.
	//
	// Every cell listed must be a region's: scenery is unstandable already,
	// and a second list saying so would be a second thing to be wrong.
	Sealed []spatial.Position

	// Doors are the doors standing in this field's crossings — each a set of
	// edges sharing one state (rpg-toolkit#1123), with edges in ABSOLUTE
	// AXIAL cells (the shape [HexCellAt] produces — a door's edge is the one
	// input here a caller converts, and [DoorEdge]'s doc says why). Optional:
	// a field with no doors is an ordinary field, and every opening in it is
	// simply a gap nobody can shut. A door edge need not sit on a region
	// seam; a door inside a region is legal.
	Doors []DoorInput

	// Exits are the authored ways out of this field (rpg-project#368,
	// design §3.1). Optional; omitted means none, and a field with none
	// behaves exactly as every field did before this list existed —
	// [Encounter.Exit] is still a departure, it simply happens nowhere in
	// particular.
	//
	// STRUCTURE, NOT SCENARIO. A dungeon has ways out whatever the party is
	// there for, so this sits beside the regions and the doors rather than
	// inside an ending. What an exit MEANS is a scenario's business: an
	// ending naming one ([TriggerExitedHolding]) is how "leaving here counts
	// as winning" gets said, and nothing here decides that.
	//
	// The party's START is deliberately not one of these. Nothing is
	// defaulted (rpg-toolkit#1033), and a dungeon whose entrance is also its
	// way out authors that in one line.
	Exits []FieldExit
}

// FieldExit is one authored way out: an id and the cell a member has to be
// standing on for [Encounter.Exit] to count as leaving through it.
//
// # Why this is not called ExitInput
//
// Every other member of [FieldInput] is an `Input` — [RegionInput],
// [PropInput], [WallInput], [DoorInput]. This one is not, because
// [ExitInput] is already the Exit VERB's input and has been since long
// before a field had exits. Two unrelated things under one name is the
// collision this package refuses everywhere else (see [doorEntityID] on
// prefixing two id namespaces rather than trusting them not to collide), so
// the authored fact takes a different word rather than the verb giving up
// its own.
type FieldExit struct {
	// ID names this exit. REQUIRED non-empty and unique within the field:
	// an ending names an exit by id, and an ambiguous one has no answer.
	// Refused at construction (ErrNoExit).
	ID ExitID

	// At is the cell somebody stands on to leave through it: an ABSOLUTE
	// authored offset [col,row] pair, converted once at construction through
	// the same [HexCellAt] every other authored cell goes through.
	//
	// Must be STANDABLE — a cell some region owns, not scenery and not
	// sealed. An exit nobody can stand on is a run nobody can leave, which
	// is [ErrNoEnding]'s own liveness argument about a trigger cell applied
	// to the cell the trigger is about. Refused at construction (ErrNoExit).
	At spatial.Position
}

// MemberInput describes a member being placed into the encounter AT
// CONSTRUCTION — Setup's roster, and Load's restored one.
//
// Authored, so its cell is an authored one: an absolute offset [col,row] pair
// under the field's orientation, converted once at construction through the
// same [HexCellAt] every region cell goes through. There is no room to name
// (rpg-project#256): which region holds the member is derived from the cell.
//
// ENTRY MID-SCENE IS A DIFFERENT SHAPE. A member walking in during play is not
// being authored; they arrive at a cell on the map, and [JoinInput] takes that
// cell directly, absolute and axial (rpg-toolkit#1101). Construction data and
// play data have different shapes for a reason, and this is the construction
// one.
type MemberInput struct {
	// ID is the member's unique identifier.
	ID MemberID

	// Kind is the member's category (player or monster).
	Kind MemberKind

	// Name is the member's display name — "skeleton-1", not a ref or an id
	// (rpg-toolkit#1137). Optional: a caller that leaves it empty gets a
	// member this composition can still place and reference exactly as
	// before this field existed; the empty string simply carries forward to
	// [Member.Name]. Supplying one is the author's business, not this
	// composition's to invent.
	Name string

	// Position is the member's starting cell: an ABSOLUTE authored offset
	// [col,row] pair, which must be floor — a cell some region owns
	// (ErrBadPlacement otherwise).
	Position spatial.Position

	// Decider is the monster's decision-making engine (monsters only).
	// Players must not have a Decider; passing one for a player will fail validation.
	// Deciders are NOT persisted; they are re-registered at load.
	Decider Decider

	// SpeedFeet is how far this member can move on their own turn, in FEET
	// (Kirk, rpg-project#254 review) — a character's walking speed, or a
	// monster's SpeedData.Walk. Filled for every kind, the same MEMBER fact
	// [Name] already is, not a monster-only field: a future driver for a
	// disconnected player shares this exact seam. Zero is legal and means
	// this member never moves on its own turn — true of every player today,
	// whose movement is driven by [Encounter.Step] under a live hand, not by
	// a [TurnDriver]'s Move intent.
	SpeedFeet int

	// SightFeet is how far this member can see, in FEET, before light and
	// line-of-sight are applied — 120 for a character (this rulebook's
	// stated default absent a stated number) or a monster's
	// SensesData.Darkvision when the stat block sets one. A STATIC base
	// fact, filled once at Join/Spawn — [Sight] still runs the full
	// light-and-LOS-aware answer at every percept refresh; this is what a
	// [Sight] implementation reads instead of reloading the sheet or stat
	// block on every single refresh, the same static/dynamic split
	// [Sight]'s own doc draws between "what changed" and "what this
	// composition was told."
	SightFeet int

	// Actions are this member's own static facts about what it can do on
	// its turn: a character's equipped weapon's swing (and the unarmed
	// strike when no weapon is equipped), or a monster's authored action
	// definitions. Static join-time facts, the same species as
	// Name (rpg-toolkit#1137) — this module cannot import the rulebook
	// (C1), so every field on [ActionView] is carried and never
	// interpreted, exactly as [Member.Name] already is.
	Actions []ActionView

	// Targeting is a monster's target-selection strategy, in the
	// rulebook's own words — "closest", "lowest-health", "lowest-ac" — and
	// the only field on this member fact that is NOT filled for every
	// kind: empty for a player, who is never asked to choose a target
	// autonomously (today; a future disconnected-player driver would read
	// this the same way a monster's does). Opaque here (C1): this
	// composition carries the string and never branches on it.
	Targeting string

	// BlocksMovement says whether this member refuses a later arrival on
	// its cell (rpg-toolkit#1434) — a bare fact, the same species as
	// SpeedFeet and SightFeet: this composition carries it and never asks
	// what kind of member set it or why. False (the zero value) is legal
	// and means what it always has for a player or monster: no member kind
	// blocked movement before this field existed, and none does now unless
	// its caller opts in.
	BlocksMovement bool

	// Knows is the doors this member carries the way to, by door ID
	// (rpg-project#368, design P1) — the author's knowledge links, seeded
	// at construction as holdings facts nobody but the engine ever sees.
	// Optional; omitted means they carry nothing.
	//
	// SETUP ONLY, AND DELIBERATELY. This is a SEED, not a running answer:
	// [Encounter.Loot] moves intel off a body, so re-seeding it at Load
	// would hand the captain back what somebody already looted. After
	// construction the holdings journal is the one answer to "who has what"
	// (design P5), it persists with the encounter, and [LoadEncounter]
	// replays it rather than re-reading this field — which is why
	// [MemberData] has no counterpart to it.
	//
	// NEVER PROJECTED, ANYWHERE (design P3). No atlas, no percept, no beat
	// and no output says who carries intel, because the affordance must not
	// answer the question it exists to ask: Loot is offered on EVERY body,
	// and a body with nothing to give has to be indistinguishable from the
	// one that has everything. See holdings.go, which is the only file that
	// reads these facts.
	//
	// A door that does not exist is refused at construction (ErrNoDoor). A
	// door that exists and is NOT concealed is legal and INERT: knowing
	// where an ordinary door is tells you nothing you could not already
	// see, and refusing it would make this declaration depend on a fact
	// about a different one.
	Knows []DoorID
}

// ActionView is a static fact about one action a member can take — an
// encounter-owned primitive, the same species as [Member.Name] and
// [AttackIdentity.DamageType]: this composition carries it and never
// interprets it, per C1 (this module's go.mod cannot import the rulebook,
// so it cannot know what a Ref like "dnd5e:monster_actions:skeleton-shortbow" means, or
// what a Kind string like "melee" means).
type ActionView struct {
	// Ref identifies this authored action to its eventual consumer. Opaque here.
	Ref core.Ref

	// Name is this action's display name — "Shortsword", "Bite" — carried
	// forward from the rulebook's own authoring, exactly as [Member.Name]
	// is.
	Name string

	// RangeFeet is the action delivery's maximum legal distance in FEET:
	// melee reach or a ranged delivery's long/maximum range. A cell is 5 feet;
	// the one comparison against grid distance converts once via [CellsFromFeet].
	RangeFeet int

	// Kind is this action's category, in the rulebook's own words —
	// "melee", "ranged", "unarmed" — opaque to this composition and never
	// branched on here, the same [AttackIdentity.DamageType] precedent
	// Name already follows.
	Kind string
}

// validateMemberFacts rejects a negative SpeedFeet, SightFeet, or any
// action's RangeFeet — the three feet-denominated member facts
// [CellsFromFeet] divides by [FeetPerCell] (Copilot, PR #1187 review). A
// negative one is not a shorter distance; it is a caller defect that would
// otherwise produce a nonsense movement budget or reach the moment a
// monster's turn asks [MonsterView.Budget] or [SeenMember.InReach] for one.
// Callers ask this BEFORE any mutation — see [Encounter.Join]'s own call for
// why that ordering matters there specifically.
func validateMemberFacts(id MemberID, speedFeet, sightFeet int, actions []ActionView) error {
	if speedFeet < 0 {
		return fmt.Errorf("member %s: speed %d feet is negative: %w", id, speedFeet, ErrNoMember)
	}
	if sightFeet < 0 {
		return fmt.Errorf("member %s: sight %d feet is negative: %w", id, sightFeet, ErrNoMember)
	}
	for _, a := range actions {
		if a.RangeFeet < 0 {
			return fmt.Errorf("member %s: action %q range %d feet is negative: %w", id, a.Ref, a.RangeFeet, ErrNoMember)
		}
	}
	return nil
}

// Trigger is an interface for ending conditions.
type Trigger interface {
	isTrigger()
}

// TriggerReachedPosition fires when a member reaches a specific cell.
//
// Authored alongside the regions (see [MemberInput]): the pair is an absolute
// authored offset cell, compiled at construction to the one absolute axial
// cell an arrival is compared against.
type TriggerReachedPosition struct {
	// Position is the target cell, an ABSOLUTE authored offset [col,row]
	// pair. Must be floor (ErrNoEnding otherwise — an ending nobody can reach
	// is a liveness hole).
	Position spatial.Position

	// Member is the target member ID (empty = any player member).
	// This v1 resolution means: if empty, the ending fires when ANY player
	// member (not monster) reaches the position.
	Member MemberID
}

// isTrigger marks TriggerReachedPosition as a Trigger.
func (t TriggerReachedPosition) isTrigger() {}

// TriggerMemberDown fires when a named member's standing reaches down.
//
// Declared like every ending and evaluated at the one place the composition
// notices a body — [Encounter.noticeDown] — AFTER the bubble logic there: a
// boss death both dissolves the fight (rpg-toolkit#959 fork (c), untouched)
// and closes the encounter, in that order, so the run ends on the world
// clock and the beat order a client reads is down → bubble-dissolved →
// ended (Kirk's ruling, rpg-project#269 §6.6).
//
// "Boss" is a content word and it is not here: dungeonspec authors the flag,
// the host turns it into one of these naming the member it spawns, and this
// composition only knows "this member's death ends things"
// (rpg-project#269 §6.1).
type TriggerMemberDown struct {
	// Member is the member whose fall fires the ending. REQUIRED — an empty
	// filter is refused at construction (ErrNoEnding) rather than given
	// latent semantics. "Any player member down" and "every player member
	// down" are different endings — first blood versus a party wipe — and
	// choosing between them belongs to the wave that brings death saves,
	// not to a default nobody declares today.
	Member MemberID
}

// isTrigger marks TriggerMemberDown as a Trigger.
func (t TriggerMemberDown) isTrigger() {}

// TriggerExternal fires when the external caller requests it.
type TriggerExternal struct{}

// isTrigger marks TriggerExternal as a Trigger.
func (t TriggerExternal) isTrigger() {}

// TriggerExitedHolding fires when a member standing on Exit's cell declares
// [Encounter.Exit] while holding Item (rpg-project#368, design §6, ruled R6
// and R7).
//
// # Explicit, never on arrival
//
// The player says when. An arrival filter on [TriggerReachedPosition] would
// have been the smaller change — one field, evaluated where arrivals already
// are — and it was rejected for what it does at the table: nobody's run
// should end because the carrier stepped on a cell while retreating to help
// a friend. Kirk, ruling it: "explicit."
//
// # Evaluated after the departure beat
//
// In the one place a departure is noticed, AFTER the beat that narrates it,
// so the record reads "left through the front gate with the heirloom" and
// then "ended" — the same cause-then-effect ordering [Encounter.refreshSight]
// states for every verb.
//
// # What a departure from anywhere else does
//
// It DROPS (design R9). A carrier who leaves from any cell but this one drops
// the holding where they stood, and it reappears there as a holdable prop for
// everybody. Otherwise a carrier who quits through the lobby — or simply
// disconnects — takes the only win out of the run with them. That rule lives
// with [Encounter.Exit] rather than here, because it holds whether or not any
// ending names the item.
type TriggerExitedHolding struct {
	// Exit is the authored exit whose cell the member must be standing on.
	// REQUIRED, and must name a [FieldExit] this field declares — refused at
	// construction (ErrNoEnding), for the reason a ReachedPosition ending
	// naming an unreachable cell is: an ending that can never fire is a
	// liveness hole.
	Exit ExitID

	// Item is the prop that must be held. REQUIRED, and must name a prop
	// this field declares as [PropInput.Holdable] — refused at construction
	// (ErrNoEnding). A prop nobody can pick up can never be held, so an
	// ending waiting for somebody to hold one is the same dead ending.
	Item PropID
}

// isTrigger marks TriggerExitedHolding as a Trigger.
func (t TriggerExitedHolding) isTrigger() {}

// EndingInput describes a declared encounter ending.
type EndingInput struct {
	// Key is the unique ending identifier.
	Key string

	// Trigger defines when this ending fires.
	Trigger Trigger
}

// SetupInput contains all information needed to construct an encounter.
type SetupInput struct {
	// Field describes the spatial layout.
	Field FieldInput

	// Members are the initial roster.
	Members []MemberInput

	// Endings are the declared ways the encounter can close.
	Endings []EndingInput

	// Initiative rolls the order a bubble forms in when trigger detection
	// starts a fight (rpg-toolkit#964). REQUIRED — trigger detection runs from
	// first light, so a fight can start before the caller does anything, and
	// an encounter that cannot order one is a misconfiguration. Setup refuses
	// without it (ErrNoInitiative).
	Initiative InitiativeRoller

	// Standing retains the constructor's source-compatible binary shape. It is
	// REQUIRED and its concrete value must also implement Participation
	// (StandingWithParticipation); otherwise Setup returns ErrNoParticipation.
	// Play consults the richer assessment only. Neither a nil nor a
	// Standing-only value defaults to everyone active.
	Standing Standing

	// Sight reports how far each member can see, in cells (rpg-toolkit#1111).
	// REQUIRED, for the same reason Standing is: the consult runs at every
	// sight refresh including first light, so an encounter that cannot ask
	// cannot build a percept. Refused at construction (ErrNoSight). There is no
	// default — a number meaning "everyone sees this far" would be this module
	// inventing a rule 5e does not have, since sight is per-creature and
	// per-light-source.
	Sight Sight

	// TurnDriver decides what a member with no player does when the fight's
	// clock lands on their turn (rpg-toolkit#1162). REQUIRED — a fight can
	// form at first light with an unplayed member first in initiative, so an
	// encounter that cannot answer this would stall before its caller does
	// anything. Refused at construction (ErrNoTurnDriver). There is no default
	// — see ADR-0043 for why this capability, unlike Decider, may not be
	// silently absent.
	TurnDriver TurnDriver

	// Striker resolves and records a member's attack when a [TurnDriver]
	// returns an [Attack] intent (rpg-project#254). REQUIRED, for the same
	// reason TurnDriver is and at the same door: a fight can form with an
	// unplayed member ready to swing the moment it forms, so an encounter
	// that cannot resolve that swing would stall or silently drop it.
	// Refused at construction (ErrNoStriker). There is no default — see
	// [Striker]'s own doc.
	Striker Striker

	// Announcer publishes the temporal boundaries a clock advance crossed —
	// a turn ending, a fight forming. REQUIRED, refused at construction
	// (ErrNoAnnouncer). There is no default, and the reason it cannot have
	// one is in [Announcer]'s own doc: a silent Announcer and a missing one
	// look identical, and one of them is the bug.
	Announcer Announcer

	// CheckResolver resolves an authored find check when a member searches
	// (rpg-toolkit#1371). REQUIRED exactly when the field carries concealed
	// structure — a concealed door or region — and refused there at
	// construction (ErrNoCheckResolver): a concealed door exists to be
	// searched for, and this module may not roll the find itself
	// (rpg-toolkit#1033). Unread, and legally nil, for a field with none.
	CheckResolver CheckResolver

	// Witness answers who currently perceives a concealed door standing
	// open (rpg-toolkit#1371). REQUIRED under exactly the same rule as
	// CheckResolver, refused at the same door (ErrNoWitness): perception's
	// reach is the host's light-and-sight truth, never this module's guess.
	Witness Witness

	// Retention is how many story beats the encounter keeps. Older beats are
	// trimmed at the storage boundary — when ToData snapshots the encounter —
	// so a blob does not grow without bound, a save does not rewrite the
	// whole history, and a verb's own beats always survive the verb that
	// minted them however many it minted (#1381).
	//
	// Zero selects DefaultRetention. RetentionUnbounded disables trimming —
	// appropriate for verified-transcript scenes, which are asserting on the
	// story itself rather than on the retention policy.
	//
	// The default is deliberately small, and that is a test strategy rather
	// than a storage economy: a generous window means the full-resync path
	// almost never runs and stays unexercised until a real player's
	// connection drops. A small one makes resync the common path, so the
	// expensive branch is the well-trodden one (#937).
	//
	// Retention persists with the encounter, so a reloaded encounter keeps
	// the policy it was built with.
	Retention int
}

// ViewInput is used to query a member's current percepts.
type ViewInput struct {
	// Member is the observer's ID.
	Member MemberID
}

// StoryInput is used to query a member's story entries from a given sequence
// number onward.
type StoryInput struct {
	// Audience is the member requesting their story.
	Audience MemberID

	// AfterSeq is the INCLUSIVE lower bound: the returned entries begin AT this
	// sequence, not after it. Zero means "from the beginning of what is
	// retained" and is always answerable.
	//
	// The name predates the behaviour and is now misleading — it is passed
	// straight through as record.SliceFor's FromSeq, which is inclusive. A
	// caller who read the old wording and passed its last-seen sequence to
	// resume would receive that entry a second time, and under retention could
	// also draw an unexpected ErrTrimmed at the boundary. Documented rather
	// than renamed because the rename is a breaking change to a shipped field;
	// it belongs in the next deliberate break (Copilot, PR #939).
	//
	// To resume after entry N, pass N+1.
	AfterSeq uint64
}

// Member represents a member's public read-side data: who they are, and
// where they stand on the dungeon map.
//
// A READ SHAPE, not the stored record. What this composition keeps about a
// member is [memberRecord]; a member's cell is the spatial room's to know,
// and this type is built by asking it. The two were one type until #1040,
// which put a position on the record that the record did not own — the dual
// state this module has paid for before.
type Member struct {
	ID   MemberID
	Kind MemberKind

	// Name is the member's display name — "skeleton-1", not a ref or an id.
	// Carried forward from [JoinInput.Name]/[MemberInput.Name] verbatim;
	// this composition neither invents nor validates it. Empty for a member
	// whose caller never supplied one (rpg-toolkit#1137).
	Name string

	// Region is the named cell set that holds Position — DERIVED, not stored.
	// A member's cell is the canvas's to know and the authored footprints
	// decide which region that cell falls in, so keeping a region label on the
	// record would be a second truth every verb had to move in step with it
	// (rpg-toolkit#1106; the dual state [memberRecord] already warns about, one
	// field over).
	//
	// A member standing in a doorway is in the region whose cell is under their
	// feet, and the member facing them one cell away is in the other — see
	// [Encounter.RegionAt] for why this composition has no unnamed doorway cell
	// to report instead (rpg-toolkit#1108).
	Region RegionID

	// Position is where the member stands, in DUNGEON-ABSOLUTE axial space —
	// the same frame the Atlas draws and every verb takes, so it can be
	// compared with any other coordinate this composition reports without
	// the caller redoing any arithmetic.
	//
	// There is no room-local counterpart, and no bridge to one: the field IS
	// this frame (rpg-toolkit#1106). Region above names the authored region
	// whose cells hold this one.
	Position spatial.Position

	// SpeedFeet, SightFeet, Actions and Targeting are this member's static
	// facts, carried forward verbatim from [MemberInput]/[JoinInput] — see
	// those fields' own docs. Read by a [TurnDriver] through [MonsterView],
	// which projects the same record plus the turn's own dynamic parts
	// (Seen, Budget).
	SpeedFeet int
	SightFeet int
	Actions   []ActionView
	Targeting string

	// BlocksMovement carries forward [MemberInput.BlocksMovement]/
	// [JoinInput.BlocksMovement] verbatim — see that field's own doc.
	BlocksMovement bool
}

// memberRecord is what the composition stores about a member: identity,
// kind, and the static member facts — never anything spatial.
//
// Deliberately NOT their cell — the canvas holds that, and duplicating it here
// would create a second truth that the verbs would have to keep in step. Not
// their room either, as of rpg-toolkit#1106: with one canvas, which authored
// chamber a member stands in is a question the field answers from their cell,
// and the copy this record used to carry had to be mutated by hand on every
// crossing.
//
// SpeedFeet, SightFeet, Actions and Targeting joined Name here for the same
// reason Name did (rpg-toolkit#1137): static join-time facts, the same
// species whether the member is a player or a monster (Kirk, rpg-project#254
// review — "MemberInput/memberRecord carries MEMBER facts, filled for every
// kind"), round-tripped through ToData/LoadEncounter as encounter-owned
// primitives (core.Ref, string, int) rather than imported rulebook types
// (C1). No parallel monster-only struct: Targeting is the only field of the
// four that is not filled for a player, and it is simply empty for one.
type memberRecord struct {
	ID             MemberID
	Kind           MemberKind
	Name           string
	SpeedFeet      int
	SightFeet      int
	Actions        []ActionView
	Targeting      string
	BlocksMovement bool
}

// Status represents the encounter's open/closed state.
type Status struct {
	// Open indicates whether the encounter is active.
	Open bool

	// Outcome is the ending, if closed.
	Outcome *Outcome
}

// Outcome is returned when an encounter closes.
type Outcome struct {
	// Ending is the key of the ending that fired.
	Ending string

	// At is the clock reading when the ending fired.
	At uint64

	// Members are the final positions of all members.
	Members []MemberOutcome
}

// MemberOutcome is where a member stood when the encounter closed.
type MemberOutcome struct {
	// ID is the member's identifier.
	ID MemberID

	// Region is the region they finished in — DERIVED from Position at the
	// moment the encounter closed, exactly as [Member.Region] is, and for the
	// same reason (rpg-toolkit#1108).
	Region RegionID

	// Position is the DUNGEON-ABSOLUTE cell they finished on
	// (rpg-toolkit#1068) — the same frame Member.Position and every beat
	// speak, and since rpg-toolkit#1106 the only frame there is.
	//
	// This was the last room-local report on the surface, and the worst place
	// for one to survive: an outcome is read AFTER the encounter is over, when
	// a host has no roster call and no further beats left to cross-check it
	// against. A party that finished in a room anchored anywhere but the
	// origin was reported at cells belonging to whatever room happens to sit
	// there.
	Position spatial.Position
}

// FormedBubble reports a fight that trigger detection started.
type FormedBubble struct {
	// Order is the initiative order the bubble formed with, first to act
	// first.
	Order []MemberID

	// Surprised names who entered unaware, sorted. A subset of Order.
	Surprised []MemberID

	// Seq is the story sequence of the formation beat.
	Seq uint64
}

// StepInput names who steps and which cell they step to.
type StepInput struct {
	// Member is the ID of the member stepping.
	Member MemberID

	// To is the destination cell, DUNGEON-ABSOLUTE — the same frame the Atlas
	// draws, a Member's Position reports, and every movement beat speaks.
	//
	// A cell, not a room and a cell. Whether this one is inside the stepper's
	// current room or through a doorway is decided here rather than by the
	// caller, which is the whole point of the verb (rpg-toolkit#1059).
	To spatial.Position
}

// CrossedDoor is one door a step went through, and the state it was in.
type CrossedDoor struct {
	// ID is the door's identifier.
	ID DoorID

	// State is the state it was in when the step passed through it.
	State DoorStateKind
}

// StepOutput reports what the step actually did.
type StepOutput struct {
	// Stepped is the movement, in dungeon-absolute cells at both ends —
	// read straight off the canvas, with no anchor involved at either end.
	Stepped struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}

	// Doors are the doors this step went through, IN TRAVEL ORDER, or empty
	// when it went through none (rpg-toolkit#1123).
	//
	// A LIST, because a step is not necessarily one cell. [Encounter.Step]
	// deliberately does not check adjacency — that is a rule about walking and
	// it lives with the walk — so a move can legitimately cross several
	// crossings, and a singular field would have picked one of them by
	// accident. The seam that walks a path visits each cell in turn, so in
	// practice this is empty or one long. (Found by Copilot on PR #1125.)
	//
	// EACH CARRIES ITS STATE rather than leaving it implied. Every door here is
	// one the step got through, so today the state reads "open" and could read
	// nothing else — but "the step succeeded, therefore the door was open" is
	// an inference that stops being true the moment a third state arrives
	// (ajar, broken, one-way), and a contract a caller has to re-derive from an
	// implication is one nobody can rely on. #1123 asks for the door's identity
	// AND state; this is both.
	//
	// A step names the DOORS it went through and nothing else. The
	// connection list that used to name an opening beside them was the room
	// chain's vocabulary; on this canvas a doorway is two adjacent floor
	// cells with a door on the edge between them (rpg-project#256).
	Doors []CrossedDoor

	// IntelDeltas maps member IDs to their updated percepts after the step
	// (IntelDelta values from the refreshSight cycle).
	IntelDeltas map[MemberID]*IntelDelta

	// Seq is the sequence number of the recorded movement beat.
	Seq uint64

	// Outcome is the encounter outcome if an ending fired underfoot; nil
	// otherwise.
	Outcome *Outcome

	// Formed is set when this step started a fight. Nil otherwise.
	Formed *FormedBubble
}

// PumpInput contains no parameters; the pump is parameterless in wave 1.
type PumpInput struct{}

// PumpOutput reports the results of a world tick.
type PumpOutput struct {
	// Tick is the exploration clock's reading after the advance.
	Tick uint64

	// MonsterMoves contains the steps monsters actually took this pump — all
	// of them, whether or not one went through a doorway. There is no second
	// list: a crossing is an ordinary step (rpg-toolkit#1106), and the
	// separate MonsterTraverses that used to sit beside this one described a
	// mechanism the composition no longer has.
	//
	// From and To are DUNGEON-ABSOLUTE — already projected through the room's
	// origin, so they can be compared with any other absolute coordinate this
	// composition reports (a [Member]'s Position, an Atlas cell, the position
	// on this move's own "moved" beat) without the caller redoing the
	// arithmetic. They are also the frame the decider named the step in: what
	// the pump reports back is the cell that was asked for.
	//
	// No room field, deliberately: an absolute cell does not need one, and
	// carrying a composition-internal room ID beside a position is the dialect
	// the seam reshape exists to remove (rpg-toolkit#1062).
	MonsterMoves []struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}

	// IntelDeltas maps member IDs to their updated percepts after all monster actions
	// (IntelDelta values from the single refreshSight cycle).
	IntelDeltas map[MemberID]*IntelDelta

	// Seqs contains the sequence numbers of the recorded beats (tick beat
	// first, then movement beats in decision order).
	Seqs []uint64

	// Outcome is the encounter outcome if an ending fired; nil otherwise.
	Outcome *Outcome

	// Formed is set when a monster's own movement started a fight — first
	// contact with nobody walking, the case a walk-only trigger seam misses.
	Formed *FormedBubble
}

// JoinInput names who is arriving and the cell on the map they arrive at.
//
// A CELL, NOT A ROOM AND A CELL (rpg-toolkit#1101). This input used to be a
// [MemberInput] — Setup's own type — so a seam holding an absolute cell had to
// resolve it to a room and a local coordinate before it could hand somebody
// through the door, which was the last room translation left in the session's
// reasoning after rpg-toolkit#1059 made the walk absolute.
//
// The two are separate types now rather than one shared one, because they are
// two different things: Setup AUTHORS a roster in authored cells, and Join
// takes somebody who is already on the map. A second entry VERB was the other
// way to close that gap, and was refused for the reason rpg-toolkit#1059 spent
// two PRs on: two ways in is two places for a rule to land, and eventually one
// of them misses.
type JoinInput struct {
	// Member is the joining member's unique identifier.
	Member MemberID

	// Kind is the member's category (player or monster).
	Kind MemberKind

	// Name is the member's display name — "skeleton-1", not a ref or an id
	// (rpg-toolkit#1137). Optional, for the same reason [MemberInput.Name]
	// is: the caller who loaded the sheet already knows it (a character's
	// CharacterState.Name, a spawned monster's MonsterState.Name), and this
	// composition only carries it forward for a cold read to project later.
	Name string

	// Cell is where they arrive, DUNGEON-ABSOLUTE — the same frame the Atlas
	// draws, [Encounter.Step] takes, and a [Member]'s Position reports.
	//
	// A cell no region owns is refused with ErrBadPlacement: void is
	// not floor, and arriving in it is not a placement this composition can
	// make sense of.
	Cell spatial.Position

	// Decider is the monster's decision-making engine (monsters only).
	// Players must not have a Decider; passing one for a player will fail
	// validation. Deciders are NOT persisted; they are re-registered at load.
	Decider Decider

	// SpeedFeet, SightFeet, Actions and Targeting are this member's static
	// facts — see [MemberInput]'s own fields of the same name for the full
	// doc. A joiner arriving mid-scene carries them exactly as an authored
	// one does; the caller who loaded the sheet or spawned the monster
	// already knows them (rpg-toolkit#1101's own argument for Name, one
	// field further).
	SpeedFeet int
	SightFeet int
	Actions   []ActionView
	Targeting string

	// BlocksMovement — see [MemberInput.BlocksMovement]'s own doc. A joiner
	// arriving mid-scene carries it exactly as an authored one does.
	BlocksMovement bool

	// Knows is the doors this member carries the way to, by door ID
	// (rpg-project#368, design §3.1) — [MemberInput.Knows] for a member who
	// arrives mid-scene instead of being authored into the roster.
	// Optional; omitted means they carry nothing.
	//
	// # Why this field has to exist
	//
	// THE AUTHORED ROSTER IS NOT HOW MONSTERS GET INTO A RUN. The host
	// builds the world empty of members and spawns each one through the
	// seam, which lands here — so a knowledge link authored on a placement
	// reached [MemberInput.Knows], which construction reads, and never
	// reached the captain who was spawned. The dungeon said the captain
	// knows the vault door and the captain did not, which makes design R2's
	// second way to win unwalkable: loot the body and learn nothing.
	//
	// # It is the same seed, not a second one
	//
	// Written through the SAME path [MemberInput.Knows] uses — one holdings
	// fact per door on the member — so there is one way intel enters a run
	// and one place it is read (design P5). A door this field does not
	// declare is refused (ErrNoDoor), and a door that exists but is not
	// concealed is legal and inert, both exactly as at construction.
	//
	// NEVER PROJECTED, ANYWHERE (design P3): no atlas, no percept, no beat
	// and no output says who carries intel, and the join beat is
	// byte-identical whether a joiner knows every secret in the dungeon or
	// none. Loot is offered on every body, so a body with nothing to give
	// has to be indistinguishable from the one that has everything.
	Knows []DoorID
}

// JoinOutput reports the results of a successful join.
type JoinOutput struct {
	// Member is the joined member's read-side data.
	Member Member

	// Formed is set when arriving in sight of the other side started a fight.
	// A joiner walks into a scene like anybody else.
	Formed *FormedBubble

	// IntelDeltas maps member IDs to their updated percepts after the join
	// (IntelDelta values from the refreshSight cycle).
	IntelDeltas map[MemberID]*IntelDelta

	// Seq is the sequence number of the recorded join beat.
	Seq uint64

	// Outcome is the encounter outcome if an ending fired during join; nil otherwise.
	Outcome *Outcome
}

// ExitInput contains the member ID of the member exiting.
type ExitInput struct {
	// Member is the ID of the member exiting.
	Member MemberID
}

// ExitOutput reports the results of a successful exit.
type ExitOutput struct {
	// Outcome is the exiting member's final placement and holdings.
	Outcome MemberOutcome

	// Carry contains the exiting member's holdings at the time of exit (copy-out).
	Carry []intel.Holding

	// IntelDeltas maps member IDs to their updated percepts after any driven
	// monster turns and remaining-member refresh caused by the exit.
	IntelDeltas map[MemberID]*IntelDelta

	// Seq is the sequence number of the recorded exit beat.
	Seq uint64

	// Closed is the encounter outcome if the encounter auto-closed due to
	// the last member exiting; nil otherwise.
	Closed *Outcome
}

// EndInput contains the ending key to fire.
type EndInput struct {
	// Ending is the key of the ending to fire (must be External).
	Ending string
}

// EndOutput reports the results of a successful ending.
type EndOutput struct {
	// Outcome is the final state of the encounter when closed.
	Outcome Outcome
}
