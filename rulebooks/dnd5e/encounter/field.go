// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MemberKind categorizes whether a member is a player or monster.
type MemberKind string

const (
	// KindPlayer indicates a player-controlled member.
	KindPlayer MemberKind = "player"

	// KindMonster indicates a monster member.
	KindMonster MemberKind = "monster"
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

// RegionInput is one named set of cells — the ONLY thing that makes floor
// (rpg-project#256).
//
// # The regions are the floor
//
// There is no room, no rectangle, no anchor and no connection any more. A
// region lists the cells it owns, absolute, and the field's floor is the union
// of every region's cells; every other cell on the canvas is void. A cell in
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

	// Facing is the authored direction this prop faces, in the orientation's
	// own six-name vocabulary — flat-top: n|s|ne|nw|se|sw; pointy-top:
	// e|w|ne|nw|se|sw. Optional: "" means the asset's own default facing.
	// dungeonspec validates the word against the field's declared
	// [Orientation]; THIS MODULE NEVER DOES, and never turns it into an
	// angle — angle math is a render concern, not this module's (rpg-project
	// #261 ruling; ADR-0040's spirit that the wire/model names facts and the
	// client derives pixels). Not a pointer, unlike the two flags above: a
	// prop's facing is a presentational fact rather than a required answer,
	// so "said nothing" and "said the default" are the same by design.
	Facing string

	// Offset is a within-cell visual nudge: [x,y] fractions of the cell
	// size, each in [-0.5, 0.5]. Optional: {0,0} means centered, which
	// omitting the field also means — the two are the same fact by design,
	// for Facing's reason. VISUAL ONLY (Kirk, rpg-project#261: "offset is
	// visual only, agreed") — a prop still occupies its whole cell for
	// [Encounter.Step] and for sight; this field never reaches Sight,
	// Standing, or the turn loop, the same law Facing follows.
	Offset [2]float64
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

	// Regions are the named cell sets whose union is the floor. REQUIRED
	// non-empty. See [RegionInput].
	Regions []RegionInput

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
	Walls []spatial.Boundary

	// Doors are the doors standing in this field's crossings — each a set of
	// edges sharing one state (rpg-toolkit#1123), with edges in ABSOLUTE
	// AXIAL cells (the shape [HexCellAt] produces — a door's edge is the one
	// input here a caller converts, and [DoorEdge]'s doc says why). Optional:
	// a field with no doors is an ordinary field, and every opening in it is
	// simply a gap nobody can shut. A door edge need not sit on a region
	// seam; a door inside a region is legal.
	Doors []DoorInput
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
	// strike when no weapon is equipped), or a monster's authored
	// [monster.ActionData]. Static join-time facts, the same species as
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
}

// ActionView is a static fact about one action a member can take — an
// encounter-owned primitive, the same species as [Member.Name] and
// [AttackIdentity.DamageType]: this composition carries it and never
// interprets it, per C1 (this module's go.mod cannot import the rulebook,
// so it cannot know what a Ref like "dnd5e:monster_actions:melee" means, or
// what a Kind string like "melee" means).
type ActionView struct {
	// Ref identifies this action's implementation to whoever compiles an
	// attack from it later — resolution's AttackFromMonsterAction, or a
	// character's equipped-weapon compiler. Opaque here.
	Ref core.Ref

	// Name is this action's display name — "Shortsword", "Bite" — carried
	// forward from the rulebook's own authoring, exactly as [Member.Name]
	// is.
	Name string

	// ReachFeet is how far this action reaches, in FEET (Kirk,
	// rpg-project#254 review — a cell is 5 feet; see [CellsFromFeet]). The
	// ONE place that compares this against a grid Distance converts once,
	// via CellsFromFeet, rather than this record guessing at a conversion.
	ReachFeet int

	// Kind is this action's category, in the rulebook's own words —
	// "melee", "ranged", "unarmed" — opaque to this composition and never
	// branched on here, the same [AttackIdentity.DamageType] precedent
	// Name already follows.
	Kind string
}

// validateMemberFacts rejects a negative SpeedFeet, SightFeet, or any
// action's ReachFeet — the three feet-denominated member facts
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
		if a.ReachFeet < 0 {
			return fmt.Errorf("member %s: action %q reach %d feet is negative: %w", id, a.Ref, a.ReachFeet, ErrNoMember)
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

// TriggerExternal fires when the external caller requests it.
type TriggerExternal struct{}

// isTrigger marks TriggerExternal as a Trigger.
func (t TriggerExternal) isTrigger() {}

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

	// Standing reports which members are down (rpg-toolkit#1075). REQUIRED,
	// for the same reason Initiative is: the consult runs from first light, so
	// an encounter that cannot ask who is standing would start fights with
	// bodies and walk them around the map. Setup refuses without it
	// (ErrNoStanding). There is no default — a nil meaning "everyone is
	// standing" would be this module deciding a rule it is not allowed to know.
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

	// Retention is how many story beats the encounter keeps. Older beats are
	// trimmed after each append, so an encounter's blob does not grow without
	// bound and a save does not rewrite the whole history.
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
	ID        MemberID
	Kind      MemberKind
	Name      string
	SpeedFeet int
	SightFeet int
	Actions   []ActionView
	Targeting string
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
	// (SurveilOutput deltas from the refreshSight cycle).
	IntelDeltas map[MemberID]*intel.SurveilOutput

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
	// (SurveilOutput deltas from the single refreshSight cycle).
	IntelDeltas map[MemberID]*intel.SurveilOutput

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
}

// JoinOutput reports the results of a successful join.
type JoinOutput struct {
	// Member is the joined member's read-side data.
	Member Member

	// Formed is set when arriving in sight of the other side started a fight.
	// A joiner walks into a scene like anybody else.
	Formed *FormedBubble

	// IntelDeltas maps member IDs to their updated percepts after the join
	// (SurveilOutput deltas from the refreshSight cycle).
	IntelDeltas map[MemberID]*intel.SurveilOutput

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
