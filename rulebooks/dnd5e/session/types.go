// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// The types below are this package's own twins of what the composition
// returns, and the functions translating into them are the price of S2.
//
// The translation is boring, and that is the point: it is what lets the
// encounter's own View, Story, Status and Atlas types change shape — or be
// replaced outright by a different implementation — without a single host
// source file changing. Re-exporting them would have been fewer lines and
// would have made every one of those modules permanently load-bearing on the
// wire.
//
// Everything here is also proto-shaped by construction: flat structs, string
// enums, no interfaces, nothing whose meaning varies by another field's value.

// GridKind names the map's coordinate family.
//
// A string rather than a mirror of spatial's iota. The composition already
// persists grid this way for the same reason: an iota is an in-process
// enumeration order, not a wire contract, and reordering it upstream would
// silently reinterpret every stored and transmitted value.
//
// One value today. The square family left with the room chain
// (rpg-project#256), so GridHex is the only kind a map can report — but the
// field stays on the wire, because a client doing grid arithmetic should
// learn which arithmetic to do from the map rather than assume it.
type GridKind string

const (
	// GridHex is a hex grid addressed in axial coordinates, where distance is
	// measured in cube space.
	GridHex GridKind = "hex"
)

// HexLayout names which way a hex map's cells point when drawn: the one fact
// about a hex atlas a client cannot recover from the cells (rpg-toolkit#1140).
//
// Axial coordinates fix the topology — the same six neighbours either way —
// and not the picture: the same cell set laid out pointy-top and flat-top
// gives two different images, roughly one rotated from the other. A client
// putting pixels on a screen has to pick one, and the first one to try guessed
// from the content and drew the reference tomb as a diagonal staircase.
//
// This is the RENDER word, deliberately not the authoring word. The composition
// holds an Orientation: the frame an author typed offset cells in, which this
// seam consumes when it enumerates the cells and never hands out. Same two
// values, different question — and the staircase happened because the two
// questions shared a word, so here they do not.
type HexLayout string

const (
	// HexLayoutPointyTop draws each hex with a vertex up: rows run straight
	// and alternate rows stagger.
	HexLayoutPointyTop HexLayout = "pointy_top"

	// HexLayoutFlatTop draws each hex with an edge up: columns run straight
	// and alternate columns stagger.
	HexLayoutFlatTop HexLayout = "flat_top"
)

// Atlas is the static world map in dungeon-absolute space.
//
// ONE MAP (rpg-project#227). What a client renders is a set of cells, the
// things standing on them, the walls between cells, and the doorways — not a
// list of chambers with anchors and spans it would have to reassemble. The
// regions beside them (rpg-project#256) are not that list coming back: a
// region is a NAMED SET of the same cells, in the same frame, carrying the
// facts that are true of an area — nothing a client has to project through.
//
// Construction truth: unchanged by movement, joins, exits, or endings. Cache
// it per encounter rather than fetching it per frame. The same map is
// answered for a world nobody has started by [Manager.AtlasOf].
//
// The INBOUND direction is a different shape, and worth saying out loud so
// the asymmetry is not read as an oversight: LaunchInput.Dungeon is
// authored content, whose cells are offset pairs under an orientation.
// Authoring is construction data, and the one-map rule governs what a
// session SEES while it plays.
type Atlas struct {
	// Grid is the coordinate family the whole map speaks — always GridHex
	// as of rpg-project#256. See [GridKind] for why it is still said.
	Grid GridKind `json:"grid"`

	// Layout is how to lay the cells out to draw them. Always present, since
	// every map is hex and a hex field must declare an orientation; omitempty
	// stays so that the absence of one — impossible today — would be visible
	// on the wire rather than defaulted.
	Layout HexLayout `json:"layout,omitempty"`

	// Cells is every cell of the map, sorted by coordinate. Occluded cells
	// are included: occlusion is walkability, not ownership.
	//
	// Sorted by coordinate rather than concatenated room by room, so the
	// flattening does not leak the grouping back through iteration order —
	// a map that still came out room-by-room would be the old shape wearing
	// a new type.
	Cells []spatial.Position `json:"cells,omitempty"`

	// Props is the things standing on the map — a pillar, a coffin, a pile of
	// bones — each naming WHAT it is and answering both blocking questions for
	// itself.
	//
	// This used to be `Occluders []spatial.Position`: the subset of cells that
	// blocked sight, as bare coordinates. That could not say a pillar from a
	// statue (rpg-project#227 recorded it as "a pillar and a statue are the
	// same cell"), and it hardcoded one answer to two independent questions —
	// wrong in both directions, since a coffin is walked around but seen over
	// and a pile of bones is neither. Carried through from the composition
	// rather than re-flattened here (rpg-toolkit#1130).
	Props []AtlasProp `json:"props,omitempty"`

	// Placed is every authored FOOTPRINT standing on the map — the rectangles
	// the World Builder drew — sorted by id (rpg-api-protos#351).
	//
	// A DIFFERENT KIND OF THING FROM [Atlas.Props], never an overlapping
	// list. A prop occupies A CELL and names the content a client draws it
	// as; a placement occupies AN AREA and names only itself — it is the
	// geometry a door, a table or a bookcase was drawn as, and what it looks
	// like is the World Builder's business. The two id spaces are ONE,
	// because the compiler refuses a placement whose name a cell prop already
	// took, so a client may key both lists together without collision.
	//
	// THIS IS WHETHER THE THING IS THERE, which is why it crosses at all. A
	// client drawing placed geometry out of the authored scene draws the
	// author's whole drawing, and the drawing says nothing about the run.
	//
	// # The absence law
	//
	// Three ways a placement fails to be on this list, and NOTHING MARKS ANY
	// OF THEM — [AtlasProp.ID]'s rule, on the other kind of thing: a flag
	// would be a second answer to a question absence already answers, and a
	// client drawing flagged entries would draw furniture nobody can reach.
	//
	//   - IT HAS NOT ARRIVED. A placement whose authored predicate has not
	//     held is in reserve: its rectangle blocks no step, closes no
	//     crossing, obstructs no lane, and [Manager.Hold] refuses its id as
	//     one that names nothing. It appears here once the predicate holds,
	//     standing where the author drew it.
	//   - SOMEBODY IS HOLDING IT, for everyone at once — a thing that left
	//     the floor left it for every member, the same truth-grain fold
	//     [AtlasProp.ID] describes for a cell prop. Dropped afterwards it
	//     appears again, with its origin on the cell it was dropped on.
	//   - THIS MEMBER CANNOT SEE WHERE IT STANDS. The PER-OBSERVER clause,
	//     and the only one that varies between two members of one party: a
	//     placement standing on any cell a concealment hides from this
	//     recipient is withheld from their answer WHOLE, byte-identical to an
	//     atlas in which the author never drew it. Never TRIMMED to the cells
	//     they are shown — a footing cut down to what somebody can see is
	//     geometry the author never drew.
	//
	// Empty for a dungeon whose author placed none, and for every encounter
	// stored before placements existed. A client that finds none behaves
	// exactly as it did before; nothing is defaulted in and no rectangle is
	// implied by [Atlas.Props].
	Placed []AtlasPlacedProp `json:"placed,omitempty"`

	// Boundaries is every wall and barrier on the map, sorted by endpoint.
	Boundaries []AtlasBoundary `json:"boundaries,omitempty"`

	// Doorways is every door's every edge, sorted by door ID then cell.
	Doorways []AtlasDoorway `json:"doorways,omitempty"`

	// Segments is every wall AS THE LINE IT IS, in the order the author drew
	// them: what a client DRAWS, instead of chaining Boundaries back into runs
	// under a straightness tolerance (rpg-project#360).
	//
	// Presentation, and the same walls. Boundaries stays the mechanical truth
	// — every crossing nobody may take — and these are the lines those
	// crossings came from, both derived from one authored line by the compiler
	// so they cannot disagree. A door's gap is the reader's own arithmetic:
	// the doorway's crossing, projected onto the segment it stands in.
	Segments []AtlasSegment `json:"segments,omitempty"`

	// StructuralWalls is every authored structural wall the recipient may know
	// (rpg-project#169), sorted by ID: its stable identity, opaque appearance
	// ref, canonical-feet line, assembled dimensions and permitted cut list.
	// See [AtlasStructuralWall].
	//
	// FIXED LAYOUT, FILTERED BY THE COMPOSITION on the same explicit-membership
	// answer as [Atlas.Placed]: a wall is presented only when its raw static
	// presence survives, and a bound opening only when its own door identity is
	// independently permitted. AN ABSENT WALL IS WITHHELD WHOLE — no trimmed
	// geometry, no tell — and this seam neither re-evaluates that answer nor
	// infers a state the composition did not send.
	StructuralWalls []AtlasStructuralWall `json:"structural_walls,omitempty"`

	// PropPresentations are permitted fixed renderer inputs, not colliders.
	PropPresentations []PropPresentation `json:"prop_presentations,omitempty"`

	// StructuralDoors is every independently permitted structural door
	// (rpg-project#169), sorted by canonical DoorID: the actual gameplay door
	// id the observation and verb paths use, its opaque ref, its resolved
	// opening endpoints and the assembled dimensions it fits. See
	// [AtlasStructuralDoor].
	//
	// ONE FLAT COLLECTION, NOT A PARENT-DEPENDENT SUBTYPE: a door stands on its
	// own identity, so a withheld parent never conceals an independently
	// permitted door, and no record here carries a parent id or mutable state.
	StructuralDoors []AtlasStructuralDoor `json:"structural_doors,omitempty"`

	// Sealed is every cell in Cells NOBODY CAN STAND ON, sorted by coordinate:
	// scenery, and the cells walls leave no room in.
	//
	// Needed because membership in a region stopped implying standable the
	// moment a wall could halve a cell (rpg-project#360). A sealed cell keeps
	// its region, its lighting and its archetype — a client draws it exactly
	// as it draws the floor beside it — and refusing a step onto it is the
	// engine's answer, never the client's guess.
	Sealed []spatial.Position `json:"sealed,omitempty"`

	// Regions is every named area of the map, sorted by ID, each listing the
	// cells it owns in the same frame and order Cells uses (rpg-project#256).
	// Every cell in Cells appears in exactly one region's Cells.
	//
	// Regions ride beside the cells rather than around them: a client still
	// draws ONE MAP from Cells, Props, Boundaries and Doorways, and reads the
	// regions for the facts that are true of an area — what it looks like and
	// how bright it is — which a client that could not read them off the wire
	// would re-derive by experiment.
	Regions []AtlasRegion `json:"regions,omitempty"`

	// Exits is every authored way out, sorted by id (rpg-project#368 §5).
	//
	// STRUCTURE ON THE TRUTH GRAIN, the same for every member — a way out is
	// a fact about the building, and a party that has not found the vault has
	// still walked in through the front gate. The composition carries it
	// unfiltered through its own per-member projection for that reason, and
	// this seam does no more than copy it across.
	//
	// WHAT AN EXIT MEANS IS NOT HERE. An ending that names one is a
	// scenario's business; a client that could read "this is the winning one"
	// off the map would be reading the scenario off the geometry. Leave is
	// offered wherever the client likes and the server decides what a
	// departure means (design §5's wire paragraph).
	Exits []AtlasExit `json:"exits,omitempty"`

	// Start is where the party came in and which way they were looking, or
	// nil when the dungeon declares none (rpg-project#374).
	//
	// PRESENTATION, AND IT GATES NOTHING. Kirk, walking a dungeon: "we
	// always start looking the wrong way." It exists so a client can open
	// the camera the way the author meant, and no rule anywhere reads it.
	// Where members ACTUALLY are is View's answer and always was; this is
	// where the dungeon says it begins.
	//
	// STRUCTURE ON THE TRUTH GRAIN, like [Atlas.Exits] beside it: a way in
	// is a fact about the building, identical for every member, so the
	// composition carries it unfiltered through its own per-member
	// projection and this seam does no more than copy it across.
	//
	// THREE CASES, NOT TWO, and a pointer is what makes the first two
	// distinguishable:
	//
	//   nil                              nobody authored a way in
	//   &{At: cell, Facing: ""}          a cell, and no direction stated
	//   &{At: cell, Facing: "e"}         a cell and a direction
	//
	// The zero value would collapse the first into the second by claiming
	// the party arrives at [0,0] looking nowhere — which is a real dungeon
	// somebody could author, not the absence of one. The composition spends
	// a pointer for exactly this reason and this seam keeps it, so a HOST
	// OMITS THE WIRE MESSAGE ENTIRELY when it is nil rather than sending a
	// start it invented (rpg-api-protos#292). Every encounter stored before
	// the field existed is the first case.
	//
	// The middle case is what the authoring dialect's bare `start: [c, r]`
	// produces, and it must not become the third on the way here: a client
	// told "n" for a dungeon whose author never chose a direction would open
	// the camera on a decision nobody made.
	Start *AtlasStart `json:"start,omitempty"`

	// DungeonKey is the content key the world under this map was loaded
	// under — what a host fetches the room's APPEARANCE by, from the same
	// registry entry the field was compiled from (rpg-project#479).
	//
	// NOT THE SCENE ITSELF, and that is the whole change. What stood here
	// before was the World Builder's authored document in full: validated by
	// the engine and carried whole through four layers so a play view could
	// draw it. What a room looks like is content and belongs to the World
	// Builder; what the engine reads from that document is three numbers per
	// prop, and those are already here as [AtlasProp]. So the map names the
	// file and a host reads the picture out of it with the codec that owns
	// one.
	//
	// CARRIED VERBATIM, AS THE HOST GAVE IT. This seam does not parse it,
	// namespace it or check that anything answers to it — a key that names
	// nothing is a content miss the host discovers when it fetches, and
	// inventing a refusal here would mean this package deciding what a
	// content registry holds.
	//
	// EMPTY IS THE HONEST ABSENCE, not a default. It means no key was ever
	// given: a host that launched a world it had assembled itself rather
	// than one loaded from a registry entry, which is what EVERY session
	// before this field was. A client seeing it empty fetches nothing and
	// draws what the map alone says, exactly as it did before.
	DungeonKey string `json:"dungeon_key,omitempty"`
}

// AtlasStart is the authored way in: a cell, and the direction the party is
// looking when they arrive.
type AtlasStart struct {
	// At is the cell, in dungeon-absolute space.
	At spatial.Position `json:"at"`

	// Facing is one of the eight true-compass names — n|ne|e|se|s|sw|w|nw
	// (rpg-project#272, the same eight [AtlasProp.Facing] speaks) — or empty
	// when the author stated none.
	//
	// Carried verbatim: the wire names the fact, and turning a name into an
	// angle is the client's own calibrated table, never this seam's
	// arithmetic. Empty is a FACT, not a gap — it means open the camera
	// however it opened before — so it is omitempty rather than spelled out,
	// unlike a blocking flag whose false would otherwise be indistinguishable
	// from an older server.
	Facing string `json:"facing,omitempty"`
}

// AtlasExit is one authored way out: its id, and the cell somebody stands on
// to leave through it.
type AtlasExit struct {
	// ID is the author's name for this exit — the dungeon file's
	// `exits[].id`, what a scenario binds to and what [ExitedBody.Exit]
	// reports.
	ID string `json:"id"`

	// At is the floor cell, in dungeon-absolute space.
	At spatial.Position `json:"at"`
}

// AtlasRegion is one named set of cells with the world facts it carries.
type AtlasRegion struct {
	// ID is the region's identifier.
	ID string `json:"id"`

	// Name is the region's display name, carried verbatim. May be empty.
	Name string `json:"name,omitempty"`

	// Cells is every cell the region owns, in dungeon-absolute space, sorted
	// by coordinate — a subset of [Atlas.Cells], in the same frame.
	Cells []spatial.Position `json:"cells,omitempty"`

	// Archetype is the presentation profile the assets resolve for this
	// region — "crypt". Never a mechanic.
	Archetype string `json:"archetype"`

	// Lighting is the region's light level.
	Lighting Lighting `json:"lighting"`
}

// Lighting is an area's light level: the dimmer on top of the archetype.
type Lighting struct {
	// Intensity is the light level in [0,1]: 0 is no light, 1 is full light.
	//
	// No omitempty: zero is dark, which is an answer, not an absence.
	Intensity float64 `json:"intensity"`
}

// AtlasProp is one thing standing on the map, in dungeon-absolute space.
//
// The two blocking answers are independent and both are carried: a pillar
// blocks movement and sight, a coffin blocks movement and is seen over, a
// pile of bones is walked through and seen over. A host that wants the old
// "occluders" list filters on BlocksLineOfSight.
type AtlasProp struct {
	// ID is the author's name for this placement — the dungeon file's
	// `place[].id` (rpg-project#368 P2). Empty when the author named none,
	// which is most props: an id is required only by whatever binds to one —
	// a scenario binding, or [Manager.Hold], which names its target by this
	// id and nothing else.
	//
	// A HELD PROP IS ABSENT FROM EVERY MEMBER'S ATLAS rather than marked
	// held: an object that left the floor left it for everyone, so absence
	// already answers the question a flag would answer twice. A prop dropped
	// after being carried appears again at its drop cell, carrying this same
	// id.
	ID string `json:"id,omitempty"`

	// Holdable is whether a member can pick this up — the author's
	// `holdable` flag, verbatim.
	//
	// STRUCTURE ON THE TRUTH GRAIN (design §5's wire paragraph): a holdable
	// thing LOOKS holdable, and every member who can see the cell sees the
	// same thing. A CLIENT OFFERS HOLD ONLY WHERE THIS IS TRUE AND NEVER
	// GUESSES FROM AN ID — ids exist for anything a scenario binds to, so
	// inferring the verb from a name would put the button on the altar as
	// readily as on the reliquary.
	//
	// No omitempty, for its three neighbours' reason: false is the ANSWER —
	// a thing nobody declared holdable is scenery — not an absent one.
	// Offering is also not permission: [Manager.Hold] still refuses out of
	// range, already held, or off-turn in a fight.
	Holdable bool `json:"holdable"`

	// Ref names what this is, so a host can draw it.
	Ref string `json:"ref"`

	// At is the cell it stands on, in dungeon-absolute space.
	At spatial.Position `json:"at"`

	// BlocksMovement reports whether an entity may enter its cell.
	//
	// No omitempty, here or on any of these four: false is an ANSWER, not an
	// absence. A pile of bones is walked through and seen over, so both of its
	// flags are false, and with omitempty it would serialise carrying no
	// blocking information at all — indistinguishable on the wire from a prop
	// nobody declared one for. That is exactly the ambiguity the composition
	// spends a *bool to prevent one layer down (rpg-toolkit#1033), and it must
	// not come back at the seam that a non-Go client reads.
	BlocksMovement bool `json:"blocks_movement"`

	// BlocksLineOfSight reports whether sight passes through its cell.
	BlocksLineOfSight bool `json:"blocks_line_of_sight"`

	// Facing is the authored direction this prop faces, one of the eight
	// true-compass names — carried verbatim from [encounter.AtlasProp.Facing]
	// and never interpreted here (rpg-project#261, vocabulary redefined by
	// rpg-project#272: the SAME eight names under both orientations).
	// Optional: "" means the asset's own default facing. Angle math is a
	// render concern, not this seam's — the wire names the fact, the client
	// derives pixels.
	Facing string `json:"facing,omitempty"`

	// Offset is an authored VISUAL displacement, carried verbatim from
	// [encounter.AtlasProp.Offset]: [x,y] within-cell nudge fractions of
	// the cell size, plus a third component — height above the floor in the
	// same unit (rpg-project#272), not bound to the planar ±0.5 clamp. The
	// zero value means centered on the floor — the SAME fact a prop that
	// authored no offset carries, by design (rpg-project#261).
	//
	// No omitempty: Go's encoding/json cannot omit a fixed-size array
	// regardless of the tag (verified — the key is always written), so a
	// reading client checks the VALUE for the zero value rather than the
	// key for absence, the same way [Lighting.Intensity]'s own zero is an
	// answer rather than a gap.
	//
	// VISUAL ONLY — a prop still occupies its whole cell for movement and
	// line of sight; this never reaches a rule.
	Offset [3]float64 `json:"offset"`
}

// AtlasPlacedProp is one authored FOOTPRINT standing on the map: a rectangle
// drawn at an angle, the two blocking answers its author gave it, whether it
// can be picked up, and the cells the engine says it stands on. Mirrors the
// wire's AtlasPlacedProp (rpg-api-protos#351). Dungeon-absolute.
//
// [Atlas.Placed] carries the absence law that decides whether an entry is
// here at all, and why this list and [Atlas.Props] never share an id.
type AtlasPlacedProp struct {
	// ID is the AUTHOR'S NAME for this placement, verbatim from the authored
	// file — required and unique, unlike [AtlasProp.ID], which most props
	// leave empty. A rectangle carries no content ref, so this is the only
	// handle anything binds to, and it is what [Manager.Hold] names as its
	// target.
	ID string `json:"id"`

	// Placement is the canonical rectangle — its size in feet, where it sits,
	// which way it faces, and how far it is nudged inside its own axes.
	// Always present.
	//
	// WHAT A CLIENT DRAWS, AND NOTHING IT DECIDES WITH. Every mechanical
	// answer about this placement is a field beside this one; see
	// [AtlasPlacedProp.Cells] for the one a reader would be tempted to
	// re-derive from the geometry.
	Placement FootprintPlacement `json:"placement"`

	// BlocksMovement reports whether a member may stand on the cells it
	// covers and cross the segments it lies across.
	//
	// No omitempty, here or on the two below, for [AtlasProp]'s reason: false
	// is the ANSWER — a rectangle nobody declared blocking is walked through
	// — not an absent one, and with omitempty it would serialise carrying no
	// blocking information at all.
	BlocksMovement bool `json:"blocks_movement"`

	// BlocksLineOfSight reports whether it obstructs a sightline. Independent
	// of the field above: a bookcase is walked around and seen over, a
	// curtain is neither, and all four combinations are real content.
	BlocksLineOfSight bool `json:"blocks_line_of_sight"`

	// Holdable is whether a member can pick this placement up — the author's
	// flag, verbatim.
	//
	// [AtlasProp.Holdable]'s law on the other kind of thing: A CLIENT OFFERS
	// HOLD ONLY WHERE THIS IS TRUE AND NEVER GUESSES FROM AN ID. Every
	// placement carries an id, so inferring the verb from a name would put a
	// take button on every wall a door stands in. False is the truth for a
	// rectangle nobody declared holdable — it is scenery, which is what every
	// placed footprint was before the flag existed. OFFERING IS NOT
	// PERMISSION: [Manager.Hold] still refuses out of reach, already held, or
	// off-turn in a fight.
	Holdable bool `json:"holdable"`

	// Cells is THE CELLS THE ENGINE SAYS THIS PLACEMENT STANDS ON: every cell
	// its rectangle covers, union the one cell its own centre lies in, in the
	// same absolute frame and the same coordinate order [Atlas.Cells] uses.
	// Never empty for a placement on this list.
	//
	// IT IS HERE SO NOBODY RE-DERIVES IT, which is the whole reason the field
	// exists. A footprint has no anchor cell, so "what is this thing next
	// to?" is a geometry question — a trace of the rectangle against every
	// cell of the floor. A CLIENT NEVER DERIVES IT FROM
	// [AtlasPlacedProp.Placement]: that would run a second geometry beside
	// the engine's, needing the engine's plane, its cell list and its
	// tie-break to agree, and it would disagree at exactly the edges that
	// matter — where a table's corner clips a hex.
	//
	// THE SAME SET [Manager.Hold]'S REACH JUDGES, so an offer and a refusal
	// cannot disagree. Reach is measured from every cell in this list, and
	// ADJACENT INCLUDES ON TOP OF: standing on one is distance zero and in
	// reach, standing next to one is distance one and in reach. No second
	// threshold exists for a footprint and none is authored.
	//
	// The centre cell is here even when the rectangle covers no cell centre
	// at all, which is how a thin door lying along a wall still stands
	// somewhere: a rectangle needs no floor under it, but the verbs that name
	// a cell need one to name.
	Cells []spatial.Position `json:"cells,omitempty"`
}

// FootprintPlacement is one rectangle on the map's continuous plane: how big
// it is, where it sits, which way it faces, and how far it is nudged inside
// its own axes. Mirrors the wire's FootprintPlacement (rpg-api-protos#351).
//
// SEAM-OWNED, AND FLAT, rather than the composition's own geometry type. The
// inner module says this as a nested footprint holding a POINTER to a box —
// a shape that must not cross this seam's exported surface (S2), both because
// an inner type crossing it makes a host recompile when it moves and because
// a shared pointer hands a caller a route into the very box the atlas is
// supposed to be a snapshot of. Four numbers and two points, copied.
//
// NOT THE SPELL-AREA [Footprint] FAMILY, which shares four letters and
// nothing else: that is a provider-authored OUTLINE for a cast — a shape
// word, a size in feet and an anchor rule — and it says where to draw a
// template. This is a resolved pose in the world: a real rectangle standing
// in a real place, which the engine traces cells from. The two never
// interconvert, and no reader should look for a [FootprintShape] here.
type FootprintPlacement struct {
	// Width is the rectangle's extent ACROSS its facing, in feet.
	//
	// ACROSS, not "the author's width". The authoring dialect's `width` lies
	// ALONG the facing and becomes Depth below — the adapter swaps the names
	// at the construction boundary, because an author measures a door by the
	// opening it fills and the plane measures a box by its bearing. What
	// reaches this seam is already canonical, and nothing here re-swaps it.
	Width float64 `json:"width"`

	// Depth is its extent ALONG its facing, in feet. See Width for the name
	// swap the authored dialect performs before this seam.
	Depth float64 `json:"depth"`

	// Origin is where the placement is anchored on the plane, in feet. The
	// rectangle is NOT centred here unless LocalOffset is zero — see it.
	Origin FootprintPoint `json:"origin"`

	// Facing is which way it faces, in DEGREES measured from east in the
	// plane's numeric frame: positive 90 points south. Any finite angle is
	// valid and nothing snaps to a hex edge — a placement is drawn at the
	// angle the author drew it at, which is the whole reason it is not a cell
	// prop.
	//
	// Zero is a REAL FACING (due east), never "not authored": every placement
	// that reaches this seam was resolved at the construction boundary, which
	// owns the conversion from the source frame.
	Facing float64 `json:"facing"`

	// LocalOffset is the box's centre offset from Origin, in feet, in the
	// PLACEMENT'S OWN AXES: X along the facing, Y across it. Applied in local
	// axes and then turned by Facing, so the centre is
	// Origin + along*X + across*Y, with along = (cos Facing, sin Facing) and
	// across = (-sin Facing, cos Facing).
	//
	// Zero — the pose most placements carry — means the rectangle is centred
	// on Origin. It is a REAL OFFSET, not a nudge for looks: the engine
	// traces cells from the offset rectangle, so a client that ignores it
	// draws the thing somewhere the engine does not have it.
	LocalOffset FootprintPoint `json:"local_offset"`
}

// FootprintPoint is one point in the map's CONTINUOUS PLANE, IN FEET.
//
// NOT A CELL, and not an [AxialPointF] — the difference is the FRAME rather
// than the precision. An axial point is a fractional q/r in the atlas's own
// cell frame, where 1 means one cell along an axis. A placement's geometry is
// measured in feet on the plane the engine traces rectangles in, the unit
// that plane's cell width was measured in. The same spot carries different
// numbers in the two frames and neither converts without the layout, so a
// reader that mistook one for the other would put a table five times too far
// out.
//
// A placed footprint has no anchor cell either: it is a rectangle somebody
// drew, and asking which cell it is "at" has no authored answer.
// [AtlasPlacedProp.Cells] is where a placement STANDS; this is where its
// rectangle SITS.
type FootprintPoint struct {
	// X is the eastward coordinate, in feet.
	X float64 `json:"x"`

	// Y is the coordinate across it in the plane's numeric frame, in feet —
	// positive southward, the same sense Facing's positive 90 points.
	Y float64 `json:"y"`
}

// AtlasBoundary is one wall or barrier crossing between adjacent cells.
type AtlasBoundary struct {
	// From is one endpoint of the crossing, in dungeon-absolute space.
	From spatial.Position `json:"from"`

	// To is the other endpoint, in dungeon-absolute space.
	To spatial.Position `json:"to"`

	// BlocksMovement reports whether an entity may cross.
	BlocksMovement bool `json:"blocks_movement"`

	// BlocksLineOfSight reports whether sight may cross.
	BlocksLineOfSight bool `json:"blocks_line_of_sight"`

	// Height is the authored wall-height MULTIPLIER of the standard
	// rendered wall height, carried verbatim from
	// [encounter.AtlasBoundary.Height] and never interpreted here
	// (rpg-project#273). 0 means not authored: a reader renders the
	// STANDARD height — exactly as if 1.0 were written — and never
	// multiplies by the raw value; the authored bounds are [1, 3]
	// (raise-only, by ruling), so 0 cannot be an authored fact. omitempty
	// is safe for exactly that reason. VISUAL ONLY: a wall blocks movement
	// and sight identically at every height — a wall cannot be seen past
	// at ANY height (Kirk's ruling, rpg-project#274).
	Height float64 `json:"height,omitempty"`
}

// AtlasDoorway is one crossable pair of cells. The two are adjacent in
// absolute space, which is what makes crossing one an ordinary step rather
// than a jump between coordinate systems.
//
// It names its door and nothing else, because a doorway is a thing with
// identity — a door can be closed or locked, and the verbs that do so name
// one — while which regions lie on either side is readable from the regions
// themselves.
type AtlasDoorway struct {
	// Door is the identifier of the door standing in this crossing. A door
	// with several edges appears once per edge under the same ID.
	Door string `json:"door"`

	// From is one of the two cells, in dungeon-absolute space.
	From spatial.Position `json:"from"`

	// To is the other, adjacent to From.
	To spatial.Position `json:"to"`
}

// AtlasSegment is one authored wall as a line: two ends, and the height it is
// drawn at.
//
// NO NAME, NO FOOTPRINT AND NO DOOR IDS, mirroring the composition's own
// (rpg-toolkit#1477). What a recipient may know about a door is the doorway
// list's business and is withheld from a non-knower there; a segment that
// carried its doors, or the cells it stood on, would say through the back door
// what the front one refuses.
type AtlasSegment struct {
	// From is one end of the wall, in fractional axial.
	From AxialPointF `json:"from"`

	// To is the other end.
	To AxialPointF `json:"to"`

	// Height is the authored wall-height multiplier, carried verbatim.
	// 0 = not authored = standard height.
	Height float64 `json:"height,omitempty"`
}

// AtlasStructuralWall is one authored structural wall as this seam reports it:
// its stable identity, its opaque appearance reference, its line in canonical
// feet, the assembled dimensions it is drawn at, and its permitted cut list
// (rpg-project#169). It is the composition's [encounter.AtlasStructuralWall] as
// a type this package owns (S2).
//
// CANONICAL FEET, AND THE SAME FRAME [AtlasPlacedProp] IS MEASURED IN. From and
// To are copied into [FootprintPoint], this package's local two-number point,
// rather than carried as their own type: a caller reads feet on the plane the
// engine traces rectangles in, with no second frame to convert between.
//
// NO NESTED DOOR METADATA. An opening carries only its identity, position and
// width. A door bound to it, when independently permitted, is its own
// [AtlasStructuralDoor] — so a hidden parent cannot leak through a nested child
// and a hidden door leaves no tell behind.
//
// NO STATE, NO PARENT ID, NO PLACED ID. Every field here is a fixed layout
// fact; where a door is and what state it is in are other verbs' answers.
type AtlasStructuralWall struct {
	// ID is the raw placed presence identity the wall's contributors are keyed
	// by, carried verbatim.
	ID string `json:"id"`

	// Ref is content's identifier for the wall's appearance, carried verbatim
	// and never inspected.
	Ref string `json:"ref"`

	// From and To are the wall's two ends, in canonical feet.
	From FootprintPoint `json:"from"`
	To   FootprintPoint `json:"to"`

	// Height, Thickness and Elevation are the assembled dimensions in canonical
	// feet, carried verbatim.
	Height    float64 `json:"height"`
	Thickness float64 `json:"thickness"`
	Elevation float64 `json:"elevation"`

	// FootprintCells is every floor cell the wall's presence footprint
	// overlaps by positive area, in order along the wall from From to To,
	// carried verbatim from [encounter.AtlasStructuralWall.FootprintCells]
	// (fog-of-war R9). It is the wall's geometry, known or not, and identical
	// in every recipient's atlas: which of these cells a recipient knows is
	// [Atlas.Cells], and a stretch of the wall is known only where one of them
	// is. A client grades the wall by these cells and never re-derives them
	// from the line; this seam never derives them either.
	FootprintCells []spatial.Position `json:"footprint_cells"`

	// Openings is the permitted cut list, in authored order: every bare
	// opening, and a bound opening only when its door is independently
	// permitted. A withheld bound opening is omitted WHOLE, so the visible wall
	// does not disclose the secret.
	Openings []AtlasStructuralOpening `json:"openings"`
}

// AtlasStructuralOpening is one permitted gap in an [AtlasStructuralWall]: its
// identity, its centre along the line and its width, all canonical feet. It
// deliberately carries no door id, no state and no hidden association — the
// door, when permitted, is its own [AtlasStructuralDoor].
type AtlasStructuralOpening struct {
	// ID names this opening.
	ID string `json:"id"`

	// Position is the gap's centre, measured along From->To from the wall's
	// start, in canonical feet.
	Position float64 `json:"position"`

	// Width is the gap's width along the line, in canonical feet.
	Width float64 `json:"width"`
}

// StructuralWallOpeningsReplacement replaces a known wall's complete permitted
// cut list without repeating its fixed layout. A present record with empty or
// default Openings clears that component; absence of a record is a no-op. Apply
// with sibling structural introductions atomically after the snapshot cutoff.
// A missing local wall requires Knowledge recovery, never a partial wall.
type StructuralWallOpeningsReplacement struct {
	// WallID is the known AtlasStructuralWall identity, not a withheld parent.
	WallID string `json:"wall_id"`
	// Openings is the complete permitted list in snapshot order and feet.
	Openings []AtlasStructuralOpening `json:"openings"`
}

// AtlasStructuralDoor is one independently permitted structural door: its
// canonical gameplay door id, its opaque appearance reference, its resolved
// visual opening endpoints in canonical feet, and the assembled dimensions it
// fits (rpg-project#169). It is the composition's
// [encounter.AtlasStructuralDoor] as a type this package owns (S2).
//
// SELF-CONTAINED, WITH NO PARENT. It carries no parent wall id and no opening
// association, so a client places it without a withheld parent's identity and
// one collection holds every permitted attached door. No state is carried: an
// unknown door state is an unknown door state, never a vanished doorway.
type AtlasStructuralDoor struct {
	// ID is the actual canonical gameplay door id the observation and verb
	// paths use, carried verbatim.
	ID string `json:"id"`

	// Ref is content's identifier for the door's appearance, carried verbatim
	// and never inspected.
	Ref string `json:"ref"`

	// From and To are the resolved visual opening endpoints, in canonical feet;
	// their nonzero distance is the door's width.
	From FootprintPoint `json:"from"`
	To   FootprintPoint `json:"to"`

	// Height, Thickness and Elevation are the assembled dimensions the door
	// fits, in canonical feet.
	Height    float64 `json:"height"`
	Thickness float64 `json:"thickness"`
	Elevation float64 `json:"elevation"`
}

// AxialPointF is a point in FRACTIONAL axial coordinates: the frame every cell
// on this map already lives in, with the halves a wall endpoint needs.
//
// A wall ends at a hex's side midpoint or its centre, and a side midpoint is
// exactly half a step from the centre it belongs to — so the halves are exact
// and no second basis or world unit crosses this seam. A client's
// axial-to-world formula already accepts fractions.
type AxialPointF struct {
	// Q and R are the axial coordinates, which may be halves.
	Q float64 `json:"q"`
	R float64 `json:"r"`
}

// WhereOutput is a member's own placement.
type WhereOutput struct {
	// Position is the cell they stand on, in dungeon-absolute space — the same
	// coordinates the Atlas draws and every verb takes.
	Position spatial.Position `json:"position"`
}

// Status reports whether an encounter is still running.
type Status struct {
	// Open reports whether the encounter is active.
	Open bool `json:"open"`

	// Outcome is the ending that fired, present only once closed.
	Outcome *Outcome `json:"outcome,omitempty"`
}

// Outcome describes how an encounter ended. It carries no result: the
// ending's declared success or failure reaches a host on the ended clock
// entry ([ClockEntry.Outcome]), and this projection drops encounter's
// Outcome.Result (rpg-project#733).
type Outcome struct {
	// Ending is the key of the ending that fired.
	Ending string `json:"ending"`

	// At is the clock reading when it fired.
	At uint64 `json:"at,omitempty"`

	// Members is where everyone stood when it closed.
	Members []MemberOutcome `json:"members,omitempty"`
}

// Formed reports that a fight started, and who is in it.
//
// It is NEWS, not a decision anyone made. Nothing in this package chooses when
// an encounter begins: the composition detects contact wherever sight changes
// and starts the fight itself (rpg-toolkit#964), and the verb that happened to
// cause it carries the report back. A client renders "roll for initiative"
// from this; it never asks for one.
//
// It appears on every verb that can put two sides in sight of each other — a
// walk, a doorway, an arrival — because a verb that started a fight and said
// nothing would leave the caller to discover it from the NEXT verb's refusal.
type Formed struct {
	// Order is the fight's initiative order, first to act first.
	Order []string `json:"order"`

	// Surprised names the members who entered unaware, a subset of Order.
	//
	// EMPTY TODAY, and the round trip is worth recording. The composition's
	// notes originally predicted it would stay empty until asymmetric
	// perception arrives (rpg-toolkit#1020) — then this doc said the opposite,
	// because occlusion really was producing one-sided contact: a wall's
	// line-of-sight ray was direction-dependent on a square grid, so a monster
	// could hold a player who did not hold it back. That was a defect, not a
	// rule (rpg-toolkit#1022), and spatial v0.9.1 fixed it by making symmetry
	// a law. Under symmetric sight nobody can be unaware of somebody who is
	// aware of them, so this cannot fill.
	//
	// The original prediction is therefore true again: it fills when #1020
	// brings asymmetry DELIBERATELY, through stealth against perception, and
	// never through geometry. See TestSightIsSymmetric.
	//
	// Surprise is a fact about the MOMENT the fight started — a member
	// surprised at formation stays surprised through their first turn however
	// the fight then develops — which is why it is carried here rather than
	// re-derived by whoever asks later.
	Surprised []string `json:"surprised,omitempty"`
}

// MemberOutcome is one member's final placement.
type MemberOutcome struct {
	// ID is the member's identifier.
	ID string `json:"id"`

	// Position is the cell they stood on when it ended, in dungeon-absolute
	// space — the same frame every other position on this seam speaks.
	Position spatial.Position `json:"position"`
}

// Seen is what the sight channel knows about a subject: the cell it occupies,
// dungeon-absolute — the same frame every other position on this seam speaks.
//
// On [Sighting], present when BOTH hold: the sighting was produced by sight
// (gated on Holding.Channel) AND the composition's payload actually decodes
// as a known location payload (encounter.DecodeSightTestimony succeeds). Nil
// for every other channel — that is the ordinary, expected case. Nil on a
// sight-channel holding whose payload fails to decode is NOT a legal state a
// caller should plan for; it means the composition wrote something projectSeen
// cannot read, which is a defect in that layer, not a case this seam is
// choosing to represent as "no sighting". A known memory (CurrentVia empty)
// keeps the Seen it last had — the last-known cell a client draws a faded
// marker on. An explicit unknown location has no Seen and is represented by
// Sighting.LocationState. Legacy untagged coordinates remain readable as
// known; malformed or current-unknown sight testimony is rejected while the
// encounter loads, before this package projects it.
//
// On [Report] this is weaker (Copilot review, PR #1159): a first-contact
// presence carries no Channel of its own, so a Report's Seen is inferred by
// decoding its payload rather than gated on provenance — see
// projectReportSeen's own comment in convert.go. It is not yet an
// authoritative "this was sight" discriminator on a Report the way it is on
// a Sighting; do not treat it as one until a second channel exists to prove
// the distinction matters.
//
// ADR-0041: channel-keyed typed sub-structs on Sighting/Report. A future
// channel (hearing, tremorsense) gets its own sub-struct with the facts that
// channel actually conveys; a sight-specific fact that arrives later
// (lighting, distance band) belongs inside Seen, not as a new field on
// Sighting.
type Seen struct {
	// Position is the sighted subject's cell, dungeon-absolute.
	Position spatial.Position `json:"position"`

	// Standing is whether the sighted subject was observed to be on their
	// feet, read out of the observer's own snapshot rather than from a live
	// roster consult. SIGHT-CHANNEL KNOWLEDGE, not roster truth: an observer
	// who can see a member can see whether they are standing, which is why
	// it belongs inside Seen and not on a roster read this seam deliberately
	// lacks. Lands with rpg-toolkit#1137.
	//
	// A memory keeps the standing it last saw and cannot disclose a change it
	// never witnessed — and because it is a snapshot rather than a live read
	// it can be WRONG, which is what illusion and enchantment need
	// (rpg-toolkit#1615).
	//
	// NIL MEANS NOT OBSERVED: testimony older than this field, or a channel
	// that does not report it. It is not the same claim as "seen standing" —
	// collapsing it to StandingUp would assert something nobody observed,
	// the exact defect this field closes (rpg-toolkit#1697, #1702). Observed
	// carries a present value naming StandingUp or StandingDowned.
	Standing *Standing `json:"standing,omitempty"`

	// Equipment is what the sighted subject was observed holding, read out of
	// the observer's own snapshot rather than from the subject. A memory keeps
	// the hands it last saw and cannot disclose a swap it never witnessed —
	// and because it is a snapshot rather than a live read it can be WRONG,
	// which is what illusion and enchantment need (rpg-toolkit#1615).
	//
	// NIL IS NOT EMPTY HANDS. Nil means the hands were not observed: nothing
	// with a sheet/presentation weapon behind it, or testimony older than this
	// field. Observed-empty arrives as a present
	// value whose strings are empty.
	Equipment *SeenEquipment `json:"equipment,omitempty"`

	// Facing is which way the sighted subject was observed to face: a
	// bearing in degrees, 0 is map north (-Y of the planar frame), 90 is
	// east (+X), clockwise, in [0, 360) (rpg-project#582, sneaking R13).
	// Read out of the observer's own snapshot, as Standing and Equipment
	// are: a memory keeps the facing it last saw. The session passes the
	// composition's number through unchanged.
	//
	// NIL MEANS NOT OBSERVED: testimony written before sightings carried a
	// facing. It is not the same claim as "facing south" — an unauthored
	// member faces 180 and its sighting says so as a present value.
	Facing *float64 `json:"facing,omitempty"`
}

// SeenEquipment is what a subject was observed holding, as one observer saw
// it. Mirrors encounter.HeldEquipment across this seam. For monsters, MainHand
// carries the rulebook's temporary first-weapon presentation answer, not tracked
// equipped state or permission to switch; OffHand carries no monster visual.
//
// The strings are BARE ITEM IDS — "longsword", "shield" — exactly as the
// rulebook holds them. The full ref a client keys a model off lives in the
// asset pipeline's namespace and is minted by the host, which is the side
// that knows both the rulebook id and the provider manifest. This seam does
// not learn that namespace.
//
// An empty string is a hand seen holding nothing, which is a POSITIVE fact
// and not an absence: the rulebook turns an empty main hand into an unarmed
// strike with its own ref and its own name.
type SeenEquipment struct {
	// MainHand is the item id in the subject's main hand, or "" for a hand
	// seen holding nothing.
	MainHand string `json:"main_hand,omitempty"`

	// OffHand is the item id in the subject's off hand, or "" for a hand seen
	// holding nothing.
	OffHand string `json:"off_hand,omitempty"`
}

// Standing says whether a member is still on their feet. Lands with
// rpg-toolkit#1137.
//
// A READ, NOT A REPLAY DEPENDENCY: the composition has known the answer all
// along — it asks the rulebook who is standing at every sight refresh — so
// this is that answer projected onto Participant and Seen, the same
// treatment Where gives position.
//
// TWO VALUES, AND NOT "DOWN". Downed is at zero hit points; it does not by
// itself say whether initiative retains the member. Dying retains a waiting
// slot, Stabilized retains an auto-pass slot, and only Dead/Defeated removes
// one. [Participant.LifeState] carries that distinction. A bare "down" also
// reads as PRONE, a posture condition on a member still acting (Kirk's ruling,
// rpg-toolkit#1084 — see ErrDowned).
type Standing string

const (
	// StandingUp is on their feet and able to act normally.
	StandingUp Standing = "up"
	// StandingDowned is at zero hit points and still on the map and roster.
	// LifeState, not Standing, says whether initiative retains the member.
	StandingDowned Standing = "downed"
)

// Passage is a provider-owned occupancy contribution for the viewing mover.
type Passage string

const (
	PassageBlocked     Passage = "blocked"
	PassagePassThrough Passage = "pass-through"
	PassageStandable   Passage = "standable"
)

// Sighting is one thing an observer knows through Intel. Status and CurrentVia
// distinguish current perception from held memory.
type Sighting struct {
	// Passage is present only for current, located sight testimony.
	Passage Passage `json:"passage,omitempty"`

	// Subject names what is perceived.
	Subject string `json:"subject"`

	// Name is the sighted subject's display name — "skeleton-1", not a ref
	// or an id. Beside Subject so a client labels what it draws and
	// narrates what it sees without a second lookup. Names are not a
	// perception question: anything an observer can sight, they can name.
	// Lands with rpg-toolkit#1137.
	Name string `json:"name,omitempty"`

	// Kind is the sighted subject's member kind — player or monster. Beside
	// Name for the same reason: kind is not a perception question either,
	// anything an observer can sight, they can classify at a glance. A
	// memory (CurrentVia empty) keeps its kind exactly as it keeps its
	// name. Lands with rpg-toolkit#1230.
	Kind MemberKind `json:"kind,omitempty"`

	// Stance is what THIS VIEWER BELIEVES the subject's stance toward them to
	// be — for a player, the creature's stance toward the party
	// (rpg-project#458, "The ring: what a player believes about a creature").
	// One of "hostile", "neutral" or "allied", and EMPTY when the run cannot
	// answer: a subject who is not a member, or one in no faction at all,
	// which a world NPC is.
	//
	// BESIDE Name AND Kind, NOT INSIDE [Seen], and the placement is the
	// contract. Seen is the SIGHT CHANNEL's facts — where the subject is and
	// what its body is doing, gone when the channel is. A stance is a fact
	// about the viewer's relationship to the subject, held the way a name and
	// a kind are: a memory keeps it exactly as it keeps those.
	//
	// IT EQUALS THE DERIVED STANCE TODAY, so every viewer gets the same
	// answer and the ring under a token does not change colour. It is carried
	// PER VIEWER anyway, because a stance read live off the graph can only
	// ever be true, and a game with no way to lie can never have illusion in
	// it. `pretend` — a creature showing one stance and holding another — is
	// where belief and truth will diverge, and when it lands it changes what
	// [encounter.Encounter.BelievedStance] answers and nothing else.
	//
	// PRESENTATION ONLY. It decides nothing about who may be attacked,
	// threatened or talked to; the composition's own fold does that, and this
	// is what a client draws.
	Stance string `json:"stance,omitempty"`

	// Seen is the sight channel's own typed knowledge; see [Seen]. Decoded by
	// the composition, not by this package — session never unmarshals
	// Payload itself.
	Seen *Seen `json:"seen,omitempty"`

	// LocationState distinguishes a known coordinate from explicit unknown
	// location testimony. It is set for sight-channel holdings only. Held
	// unknown testimony remains a sighting with nil Seen rather than being
	// deleted or treated as malformed.
	LocationState LocationState `json:"location_state,omitempty"`

	// Payload is what the observer knows about it, encoded by the composition.
	// Retained for channels the SDK has not typed; sight itself is typed
	// through Seen above rather than asking a client to decode this.
	Payload []byte `json:"payload,omitempty"`

	// Channel is how it was perceived.
	Channel string `json:"channel,omitempty"`

	// At is the clock reading when this knowledge was last refreshed.
	At uint64 `json:"at,omitempty"`

	// CurrentVia lists the channels currently carrying it. Empty means the
	// observer holds a memory rather than a live sighting.
	CurrentVia []string `json:"current_via,omitempty"`

	// Status distinguishes a live sighting from a stale memory.
	Status string `json:"status,omitempty"`
}

// LocationState is the encounter-authored state of location knowledge mirrored
// by this host seam. Session does not decide whether that knowledge is
// actionable or decode the composition's payload itself.
type LocationState string

const (
	// LocationKnown says the sight channel carries a lawful coordinate.
	LocationKnown LocationState = "known"
	// LocationUnknown says the subject remains known without an actionable
	// coordinate.
	LocationUnknown LocationState = "unknown"
)

// ExperienceGrant is one character's share of one grant.
type ExperienceGrant struct {
	// Character is who was paid.
	Character string `json:"character"`

	// Amount is what this fall paid them: the monster's worth divided among
	// the players on the roster, rounded down. Always positive.
	Amount int `json:"amount"`

	// Total is their cumulative experience AFTER the grant — the number the
	// character store now holds, written before this beat was recorded. A
	// client can answer "how close am I to the next level" from this and the
	// threshold table without summing every beat it has ever seen.
	Total int `json:"total"`
}

// RollSource identifies and describes the rulebook-owned source of a roll
// fact. Ref is the canonical module:type:id string of the content that
// produced the fact; Name is its display name. Label optionally describes the
// source's role within its calculation ("Fighter level").
//
// This package's own string-ref twin of the root rulebook's roll-trace
// primitives: the composition persists every *core.Ref as its canonical
// string, so the wire carries no type this seam has to parse (S2).
type RollSource struct {
	// Ref is the canonical module:type:id string of the source content.
	Ref string `json:"ref"`

	// Name is the source's display name.
	Name string `json:"name"`

	// Label is the source's role within its calculation, when it has one.
	Label string `json:"label,omitempty"`

	// SourceID is the entity responsible for this contribution, when the
	// provider qualifies it. Subtractive dice require one.
	SourceID string `json:"source_id,omitempty"`
}

// DiceReroll records one ordered replacement of a die face and its source.
// Before must equal the die's current face under the rerolls that precede it.
type DiceReroll struct {
	// DieIndex is the die's position in the pool, 0-based.
	DieIndex int `json:"die_index"`

	// Before is the face the die showed before this replacement.
	Before int `json:"before"`

	// After is the face the replacement produced.
	After int `json:"after"`

	// Source is the rulebook-owned cause of the replacement.
	Source RollSource `json:"source"`
}

// KeepRule names the rule that decided which faces of a dice pool count.
// The wire words, mirrored from the rulebook's own spelling.
type KeepRule string

const (
	// KeepAdvantage keeps the highest face of a pool rolled with advantage.
	KeepAdvantage KeepRule = "advantage"

	// KeepDisadvantage keeps the lowest face of a pool rolled with disadvantage.
	KeepDisadvantage KeepRule = "disadvantage"

	// KeepCancelled records granted and imposed sources meeting: the pool was
	// rolled straight and every face counts, and the record says why.
	KeepCancelled KeepRule = "cancelled"
)

// DiceKeep records the rule that decided KeptIndices, and who brought it.
// Nil when nothing touched the pool: a straight roll keeps every face.
//
// IT IS WHY THE PARALLEL SOURCE LISTS ARE GONE from this seam's bodies. A list
// beside the dice can disagree with the dice; a record on the trace that
// decided them cannot (rpg-project#462 R1). A client draws the discarded face
// struck through and prints the rule's name from here — never a word it
// invented, and never a style guessed from the beat's actor.
type DiceKeep struct {
	// Rule is "advantage", "disadvantage" or "cancelled".
	Rule KeepRule `json:"rule"`

	// Granted are the sources that granted advantage on this pool.
	Granted []RollSource `json:"granted,omitempty"`

	// Imposed are the sources that imposed disadvantage on this pool.
	Imposed []RollSource `json:"imposed,omitempty"`
}

// DiceTrace records the original and final faces of one homogeneous dice
// pool. An empty KeptIndices means every final face contributes to Subtotal.
type DiceTrace struct {
	// Notation is the pool's notation in the dice package's normalized form
	// ("d8", "2d6"), describing exactly the recorded faces.
	Notation string `json:"notation"`

	// DieSize is the size of every die in the pool.
	DieSize int `json:"die_size"`

	// OriginalRolls are the faces as first rolled.
	OriginalRolls []int `json:"original_rolls"`

	// Rerolls are the ordered sourced replacements applied to the pool.
	Rerolls []DiceReroll `json:"rerolls,omitempty"`

	// FinalRolls are the faces after every reroll, same order as
	// OriginalRolls.
	FinalRolls []int `json:"final_rolls"`

	// KeptIndices are the FinalRolls positions that contribute to Subtotal;
	// empty means every final face contributes.
	KeptIndices []int `json:"kept_indices,omitempty"`

	// Keep records the rule that decided KeptIndices, and who brought it.
	// Nil when nothing touched the pool — a straight roll, and the zero value
	// says exactly that.
	Keep *DiceKeep `json:"keep,omitempty"`

	// Subtotal is the dice' authoritative contribution — faces kept, never
	// resummed by a reader.
	Subtotal int `json:"subtotal"`
}

// RollComponent records dice, a modifier, or both from one source.
// A non-nil Modifier participates even when its value is zero.
type RollComponent struct {
	// Source is who provided this component's roll facts.
	Source RollSource `json:"source"`

	// Dice is the trace of the pool this component rolled, when it did.
	Dice *DiceTrace `json:"dice,omitempty"`

	// Modifier is the component's flat contribution, present whenever it
	// contributed one — even when that value is zero. Nil means the modifier
	// did not participate; a present zero is a real zero.
	Modifier *int `json:"modifier,omitempty"`

	// SubtractDice subtracts Dice.Subtotal while retaining positive die faces.
	SubtractDice bool `json:"subtract_dice,omitempty"`
}

// RollCalculation records the sourced components and authoritative total of
// a roll, in the order the rulebook produced them.
type RollCalculation struct {
	// Components are the sourced dice and modifiers, in production order.
	Components []RollComponent `json:"components"`

	// Total is the roll's authoritative result.
	Total int `json:"total"`
}

// DamageComponent is the replayable subset of one resolved damage component.
// Its order and values come from resolution; consumers display rather than
// recalculate it.
//
// Everything roll-shaped lives in [Roll]: the sourced identity, the dice
// trace when the component rolled dice, and the modifier when it contributed
// one — present even when that value is zero. The remaining fields are damage
// facts, not roll facts: the category, the damage type, and the multiplier. A
// component that exists only to carry a multiplier (resistance,
// vulnerability, immunity) has a sourced Roll with neither dice nor a
// modifier.
//
// The four scalar fields below it are the legacy READ representation for
// payloads this package last persisted before roll traces: the strict decoder
// maps exactly one representation into exactly one of the two field groups,
// and never fabricates a Roll from the legacy scalars. New bodies carry only
// Roll.
type DamageComponent struct {
	Source     string        `json:"source"`
	Roll       RollComponent `json:"roll"`
	DamageType DamageType    `json:"damage_type"`
	Multiplier *float64      `json:"multiplier,omitempty"`

	// SourceRef is the legacy scalar provider identity, readable only from a
	// pre-trace payload.
	SourceRef string `json:"source_ref,omitempty"`

	// Dice is the legacy scalar dice notation, readable only from a pre-trace
	// payload.
	Dice string `json:"dice,omitempty"`

	// FinalRolls are the legacy scalar final faces, readable only from a
	// pre-trace payload.
	FinalRolls []int `json:"final_rolls,omitempty"`

	// FlatBonus is the legacy scalar flat modifier, readable only from a
	// pre-trace payload.
	FlatBonus int `json:"flat_bonus,omitempty"`
}

// RevealedDoor is one door a concealment hid, as the recipient's own door
// list and atlas now carry it — the patch for both cached reads.
//
// A LIST ELEMENT RATHER THAN A BODY, because a concealment may hide several
// doors and reveals all of them at once. This is what the retired
// "door_revealed" body carried for exactly one.
type RevealedDoor struct {
	// Door is the door's identifier.
	Door string `json:"door"`

	// State is the door's LIVE state at reveal time — "open", "closed" or
	// "locked". A found door is usually shut (that is what concealment
	// looks like), but a door revealed by being perceived open arrives
	// open.
	State string `json:"state"`

	// Doorways is every edge of the door, ready to append to the cached
	// atlas's doorway list — a wide door's edges arrive together, Door
	// filled on each.
	//
	// EMPTY FOR A FOOTPRINT DOOR, which stands in no crossing and has no
	// doorway to append (rpg-project#485). Its rectangle reaches the
	// recipient on the atlas re-read this beat asks for, where [Atlas.Placed]
	// carries it, not through this list — see [ConcealmentRevealedBody]'s own
	// doc on what the beat does not carry.
	Doorways []AtlasDoorway `json:"doorways,omitempty"`

	// Approaches is the lock's authored routes, present only while the
	// door is locked — what the recipient's Doors read would now list.
	Approaches []DoorApproach `json:"approaches,omitempty"`
}

// Door is one door's identity and live state — the dynamic half of
// [AtlasDoorway], which carries the same door's edges and never changes.
// Read via [Manager.Doors] once, then updated from `door` beats.
type Door struct {
	// ID is the same identifier AtlasDoorway.Door carries.
	ID string `json:"id"`

	// State is "open", "closed" or "locked" — the composition's own kinds,
	// projected as strings for the same reason MemberKind is.
	State string `json:"state"`

	// Lock is set only while State is "locked".
	Lock *DoorLock `json:"lock,omitempty"`
}

// DoorLock is an authored lock, carried not interpreted — the DCs are public
// down to the number (full data until v1.0), and what beats one is
// [Manager.Unlock]'s ruling, never a comparison the composition makes.
//
// A lock is a LIST of accepted routes now, each priced separately (the
// multi-approach ruling, rpg-project#350) — forced with Strength or picked
// with Dexterity and tools. An attempt resolves through exactly one of them.
type DoorLock struct {
	// Approaches are the accepted ways through, at least one, in authored
	// order.
	Approaches []DoorApproach `json:"approaches"`
}

// DoorApproach is one accepted route through an authored check: an ability
// or skill ref, maybe a tool, and the DC that route must beat.
type DoorApproach struct {
	// Ability is the opaque ability or skill ref this route rolls, e.g.
	// "dex" or "perception".
	Ability string `json:"ability,omitempty"`

	// Tool is the opaque item ref the route may name; empty when none.
	Tool string `json:"tool,omitempty"`

	// DC is this route's authored difficulty. No omitempty: zero would be
	// an answer if an author wrote one (TestFalseIsAnAnswerOnTheWire's law).
	DC int `json:"dc"`
}

// SaveReport names which aggregates were persisted by a verb and which were
// not.
//
// Every verb returns one, and a partial write returns an error carrying a
// populated report rather than a bare failure (S6). The distinction is
// load-bearing for the host: "nothing was written" is safe to retry, while
// "the encounter advanced but the character did not" is a repair, and an
// unqualified error cannot tell them apart.
type SaveReport struct {
	// Written names the aggregates that were persisted successfully.
	Written []string `json:"written,omitempty"`

	// Failed names the aggregates that could not be persisted.
	Failed []string `json:"failed,omitempty"`
}

// Partial reports whether this save landed some aggregates but not all — the
// state that needs repair rather than retry.
func (r SaveReport) Partial() bool {
	return len(r.Written) > 0 && len(r.Failed) > 0
}

// MemberKind categorises a member.
//
// A string enum rather than a mirror of the composition's, for the same reason
// GridKind is: it maps onto a proto enum, and adding a kind must be a
// compatible change rather than a renumbering.
type MemberKind string

const (
	// KindPlayer is a player-controlled member.
	KindPlayer MemberKind = "player"

	// KindMonster is a member driven by the game rather than a person.
	KindMonster MemberKind = "monster"

	// KindWorld is a placed, non-combatant NPC — the host seam's twin of
	// encounter.KindWorld (rpg-toolkit#1404). Carries no content of its own;
	// the placed npc.Data lives in SessionData.WorldNPCs, keyed by member ID.
	KindWorld MemberKind = "world"
)

// RosterInput asks for the public identity of every current encounter member.
// Player is the authenticated host principal, not a client-supplied authority
// token; the manager uses it only to verify that the caller owns a seat.
type RosterInput struct {
	// Member is the controlled observer whose known identities are requested.
	Member string
	// Session is the session whose encounter roster to read.
	Session string

	// Player is the authenticated host principal requesting the roster.
	Player string
}

// RosterOutput contains the current public encounter roster in composition
// order. World NPC members are intentionally omitted.
type RosterOutput struct {
	// Members is the public identity of each player and monster in the roster.
	Members []PublicMember `json:"members,omitempty"`
}

// PublicMember is the identity and cosmetic projection visible in a public
// roster. It deliberately contains no position, hit points, inventory,
// player principal, or other private sheet facts.
type PublicMember struct {
	// ID is the member's encounter identifier.
	ID string `json:"id"`

	// Kind identifies whether the member is player-controlled or a monster.
	Kind MemberKind `json:"kind"`

	// Name is the member's current public display name.
	Name string `json:"name"`

	// ClassRef is the player's class identifier. It is empty for monsters.
	ClassRef string `json:"class_ref,omitempty"`

	// RaceRef is the player's race identifier. It is empty for monsters.
	RaceRef string `json:"race_ref,omitempty"`

	// MonsterRef is the stored catalog reference. It is empty for players.
	MonsterRef string `json:"monster_ref,omitempty"`

	// AppearanceRef is the observer's captured opaque body appearance, not a
	// live-member lookup. Empty retains legacy model resolution.
	AppearanceRef string `json:"appearance_ref,omitempty"`

	// DefaultActivityRef is the observer's captured authored presentation
	// default, not the current animation phase or a live-member lookup.
	// Empty means no explicit captured override, including legacy observations.
	DefaultActivityRef string `json:"default_activity_ref,omitempty"`

	// Customization is the player's exact cosmetic selection, or an empty
	// customization for monsters.
	Customization Customization `json:"customization"`

	// Faction is the side this member fights on (rpg-project#375, the
	// hold-out design §5): the dungeon file's `factions[].id`, or one of
	// the two the composition reserves and no author may declare — players
	// are `party`, and a monster spawned with no faction is `monsters`.
	//
	// A FREE-FORM ID, never an enum. Factions are content, declared per
	// dungeon, so a client groups or colours by the word without knowing
	// what a goblin is. Who fights whom is NOT derived from it: the stance
	// between two factions is the run's own fold, and a change in it
	// reaches a client as a `stance` beat.
	//
	// ON THE ROSTER because the roster is the only per-member row on the
	// wire; a placement (Member) answers a cell and nothing about sides.
	// Always written, never omitted: this row lists players and monsters
	// only, and every one of them is on a side, so an empty value here is a
	// defect upstream rather than a member in no faction.
	Faction string `json:"faction"`
}

// StyleSelectionKind identifies whether a style slot selects a provider-owned
// style or explicitly contains no style.
type StyleSelectionKind string

const (
	// StyleSelectionStyle selects the provider-owned style named by StyleRef.
	StyleSelectionStyle StyleSelectionKind = "style"

	// StyleSelectionNone explicitly removes the style from the slot.
	StyleSelectionNone StyleSelectionKind = "none"
)

// StyleSelection is the Session-owned projection of one opaque style choice.
type StyleSelection struct {
	// Kind identifies a style selection or an explicit no-style selection.
	Kind StyleSelectionKind `json:"kind"`

	// StyleRef is the opaque provider-owned style reference, when selected.
	StyleRef string `json:"style_ref,omitempty"`
}

// HairCustomization is the Session-owned projection of optional hair choices.
type HairCustomization struct {
	// Scalp is the selected scalp style, when present.
	Scalp *StyleSelection `json:"scalp,omitempty"`

	// FacialHair is the selected facial-hair style, when present.
	FacialHair *StyleSelection `json:"facial_hair,omitempty"`

	// ColorSRGB is the packed sRGB hair color, when present.
	ColorSRGB *uint32 `json:"color_srgb,omitempty"`

	// Roughness is the provider-neutral hair roughness, when present.
	Roughness *float32 `json:"roughness,omitempty"`
}

// OutfitCustomization is the Session-owned projection of optional outfit
// colors.
type OutfitCustomization struct {
	// PrimaryColorSRGB is the packed sRGB primary outfit color, when present.
	PrimaryColorSRGB *uint32 `json:"primary_color_srgb,omitempty"`

	// SecondaryColorSRGB is the packed sRGB secondary outfit color, when present.
	SecondaryColorSRGB *uint32 `json:"secondary_color_srgb,omitempty"`
}

// Customization is the Session-owned projection of a player's appearance.
// Nil nested values preserve the absence of a selected customization slot.
type Customization struct {
	// Hair contains the selected hair values, when present.
	Hair *HairCustomization `json:"hair,omitempty"`

	// Outfit contains the selected outfit colors, when present.
	Outfit *OutfitCustomization `json:"outfit,omitempty"`
}

// Member is a participant's placement in the world.
type Member struct {
	// ID is the member's identifier.
	ID string `json:"id"`

	// Kind categorises the member.
	Kind MemberKind `json:"kind"`

	// Name is the member's display name — "skeleton-1", not a ref or an id.
	// Never empty for a member the composition can name, which is every
	// member it can place (rpg-toolkit#1137).
	Name string `json:"name,omitempty"`

	// Position is the cell they stand on, in dungeon-absolute space.
	//
	// It replaced a room id, and the swap is the reshape in one field: which
	// chamber somebody is in is the composition's own decomposition, while
	// where they STAND is the thing a client renders, a rule measures, and
	// another member walks toward.
	Position spatial.Position `json:"position"`
}

// CharacterState is what a loaded character reports about itself.
//
// This package's own twin, not character.Data: the stored shape is what the
// host persists, while this is what one call observed after reconstitution.
// Read through the character's accessors rather than by serialising it, so this
// stays a cheap read.
//
// Most of these fields are reported as loaded. SPEED is the exception and the
// interesting one: it is not stored on a character at all, but derived from
// race when asked, which is why it is the value the tests use to prove a sheet
// genuinely reconstituted rather than that a repository returned some bytes.
//
// Deliberately small. It carries what a client needs to render a member and
// what proves the load happened; the sheet in full is the host's own copy to
// read, and re-exporting it here would make character.Data's every field a wire
// commitment of this package.
type CharacterState struct {
	// ID is the character's identifier.
	ID string `json:"id"`

	// Name is the character's name.
	Name string `json:"name"`

	// Level is the character's level.
	Level int `json:"level"`

	// Speed is the character's BASE walking speed in feet, from race.
	//
	// Base, not effective: condition-driven modifiers (Unarmored Movement and
	// the like) are applied during resolution through the movement chain, not
	// folded in here. A client rendering this is showing the sheet's speed, not
	// what the next step will cost.
	Speed int `json:"speed"`

	// HitPoints is the character's current hit points.
	HitPoints int `json:"hit_points"`

	// MaxHitPoints is the character's hit point maximum.
	MaxHitPoints int `json:"max_hit_points"`

	// ArmorClass is the character's current armour class.
	ArmorClass int `json:"armor_class"`

	// ProficiencyBonus is the character's proficiency bonus.
	ProficiencyBonus int `json:"proficiency_bonus"`
}

// MonsterState is what a spawned NPC reports about itself.
//
// A sibling of CharacterState rather than a merger with it. The overlap is
// large — a monster reports name, hit points, armour class, speed and
// proficiency bonus through its own accessors, just as a character does — and
// merging them was considered and rejected: the two genuinely differ (a
// character has a Level, a monster has a challenge rating), and one type with
// fields that are meaningful for only half its values reintroduces exactly the
// guessing that giving characters and monsters separate content paths removed.
//
// Ref is here and has no counterpart on CharacterState, which is the same
// asymmetry stated at the seam: a monster is content built from a named recipe,
// and a character is an instance the host owns. Reporting it lets a client
// render a skeleton as a skeleton without inferring anything from the name.
type MonsterState struct {
	// ID is the member's identifier in this encounter, not the catalog entry's.
	ID string `json:"id"`

	// Ref is the catalog entry this was built from — "dnd5e:monsters:skeleton".
	Ref string `json:"ref"`

	// Name is the monster's name.
	Name string `json:"name"`

	// HitPoints is the monster's current hit points.
	HitPoints int `json:"hit_points"`

	// MaxHitPoints is the monster's hit point maximum.
	MaxHitPoints int `json:"max_hit_points"`

	// ArmorClass is the monster's armour class.
	ArmorClass int `json:"armor_class"`

	// Speed is the monster's walking speed in feet.
	Speed int `json:"speed"`

	// ProficiencyBonus is the monster's proficiency bonus, derived from its
	// challenge rating.
	ProficiencyBonus int `json:"proficiency_bonus"`
}

// DamageType names the kind of damage an attack deals: the rulebook's own
// thirteen. Lands with rpg-toolkit#866, carried on AttackRef.
//
// A CLOSED SET, SO A GO TYPE A CLIENT-FACING ENUM CAN MIRROR (Kirk,
// rpg-project#249 §6): a UI branches on it — a colour, a sound, a word in
// the beat line ("6 slashing") — and thirteen is the whole of 5e's list,
// sealed by the rulebook rather than open to a catalog. Compare AttackRef.Ref,
// which is an OPEN set and stays a string.
//
// The string values match damage.Type's own exactly, on purpose: this type
// exists so a host mapping onto a proto enum has something to switch on
// without importing the rulebook's own damage package across the boundary
// (S2), not to invent a second vocabulary for the same thirteen words.
type DamageType string

// The rulebook's thirteen damage types, mirroring damage.Type.
const (
	DamageAcid        DamageType = "acid"
	DamageBludgeoning DamageType = "bludgeoning"
	DamageCold        DamageType = "cold"
	DamageFire        DamageType = "fire"
	DamageForce       DamageType = "force"
	DamageLightning   DamageType = "lightning"
	DamageNecrotic    DamageType = "necrotic"
	DamagePiercing    DamageType = "piercing"
	DamagePoison      DamageType = "poison"
	DamagePsychic     DamageType = "psychic"
	DamageRadiant     DamageType = "radiant"
	DamageSlashing    DamageType = "slashing"
	DamageThunder     DamageType = "thunder"
)

// AbilityRef identifies WHAT is being activated — the combat ability or
// feature behind a [VerbActivate] declaration.
//
// It exists for the reason [AttackRef] does: a declaration a client renders
// verbatim needs an identity, and "Rage" is not derivable from the verb the
// way "Move" is from [VerbMove]. Attack has one identity per weapon; Activate
// has one per thing the character carries.
type AbilityRef struct {
	// Ref is the ability's full core.Ref.String() —
	// "dnd5e:combat_abilities:dodge", "dnd5e:features:rage". An OPEN set, so a
	// string, for [AttackRef.Ref]'s reason: the catalog grows without this
	// type changing.
	//
	// It is also the SELECTOR MATERIAL for an Activate declaration. One verb
	// compiles many offers and this is what makes their IDs differ, the way a
	// serialized definition makes two Attack offers differ.
	Ref string `json:"ref"`

	// Name is the ability's own display name — "Dodge", "Rage", "Second
	// Wind". Authored by the ability, never derived from the ref by a reader.
	Name string `json:"name"`
}

// SpellRef identifies WHICH SPELL a cast row offers and which spell a cast
// beat narrates — the seam's own word for the composition's SpellIdentity.
//
// It is not [AbilityRef] with a different ref string, and the separation is the
// same one the composition makes: a spell ref and an ability ref are different
// catalog namespaces ("dnd5e:spells:vicious-mockery" against
// "dnd5e:features:rage"), they reach a client as different bodies, and a
// declaration that read Spell: &SpellRef{...} says what it is doing.
type SpellRef struct {
	// Ref is the spell's full core.Ref.String() —
	// "dnd5e:spells:vicious-mockery". An OPEN set, so a string, for
	// [AttackRef.Ref]'s reason: the catalog grows without this type changing.
	Ref string `json:"ref"`

	// Name is the spell's own display name — "Vicious Mockery", "True
	// Strike". Authored by the content, never derived from the ref by a
	// reader.
	Name string `json:"name"`
}

// SpeakOptionRow is one line on the Speak row's menu: who offers it, the id
// the request echoes, and the words the character says (rpg-project#569).
//
// EXACTLY ONE of Target and Prop is set. A request brings both the option id
// and that same target back in [SpeakInput]; an option id alone names
// nothing, because two creatures may each author a line called "greet".
type SpeakOptionRow struct {
	// Target is the creature that offers this line, as it appears in the
	// roster. Empty when the line is a prop's.
	Target string `json:"target,omitempty"`

	// Prop is the prop that offers this line — the dungeon file's
	// `place[].id`, the id [AtlasProp.ID] carries. Empty when the line is a
	// creature's.
	Prop string `json:"prop,omitempty"`

	// OptionID is the author's id for the line, unique on its target. The
	// client echoes it in [SpeakInput.OptionID] and never parses it.
	OptionID string `json:"option_id"`

	// Line is what the character says, VERBATIM as authored. The engine
	// never composes one.
	Line string `json:"line"`

	// Available is whether this speaker may pick this line right now,
	// independent of the declaration's turn and economy gate. PLAIN bool:
	// false is an answer. Every row listed today is available — a line whose
	// `requires` the speaker does not meet is not listed at all (R9, the
	// probe law), and a creature that cannot see the speaker is not a target.
	Available bool `json:"available"`

	// Why is present if and only if Available is false — the line's own
	// reason, never the declaration's.
	Why *Shortfall `json:"why,omitempty"`
}

// CastOption is one word on a cast's menu — Command's "Approach", "Flee",
// "Grovel" — the seam's own word for the content's authored choice.
//
// IT IS A CAST-TIME INPUT AND NOT A ROW. The alternative considered and
// rejected was one declaration per word, which would have made the selector
// carry the choice; it puts the branching in the dock, and a spell with two
// choices becomes a grid of rows. So a spell with a menu is ONE offer that
// lists what may be chosen, exactly as a [TargetCell] offer is one row that
// says a cell is wanted, and the request brings one id back.
//
// A client draws what it was sent. Nothing here is grouped, inferred, or
// looked up: the label is authored beside the id by the spell.
type CastOption struct {
	// ID is the id the request echoes in CastInput.Option — "approach". An
	// OPEN set, so a string, for [AttackRef.Ref]'s reason: content adds words
	// without this type changing.
	ID string `json:"id"`

	// Label is the word a person reads — "Approach". Authored by the content
	// beside the id, never derived from it by a reader, which is the rule
	// [SpellRef.Name] and [AbilityRef.Name] already keep.
	Label string `json:"label"`

	// Description explains the choice before commitment, authored beside the
	// id by whoever declares the option — the spell, or the producer of a
	// reaction's choice (provider-design R12). Prose: never selector material
	// and never read on execution. Empty when the content authored none.
	Description string `json:"description,omitempty"`
}

// AttackRef identifies WHAT was swung — weapon identity, which this seam
// dropped on the floor since the first swing (rpg-toolkit#866). Carried on a
// compiled Declaration, AttackOutput, and the Struck/Missed event bodies, so
// selection and outcome name the same authored attack for every witness.
type AttackRef struct {
	// Ref is the full catalog ref the attack compiled from —
	// "dnd5e:weapons:longsword", "dnd5e:weapons:unarmed-strike". An OPEN set,
	// so a string: the client already maps
	// refs to models and icons, and the catalog grows without this type
	// changing.
	Ref string `json:"ref"`

	// Name is the catalog's display name — "Longsword", "Unarmed Strike".
	Name string `json:"name"`

	// DamageType is the kind of damage it deals. Closed set — see DamageType.
	DamageType DamageType `json:"damage_type"`
}

// ReactionRef identifies WHAT a beat was taken as, when it was taken as a
// reaction rather than as a declared action — the seam's own word for the
// composition's ReactionIdentity.
//
// It answers the only question a struck beat on somebody else's turn raises:
// why did the fighter deal damage while the skeleton was walking. Carried on a
// REACT [Declaration], on [StruckBody] and [MissedBody], and on
// [WindowOpenedBody], so the offer, the swing and the story all name the same
// thing.
type ReactionRef struct {
	// Ref is the full ref of the condition or feature that reacted —
	// "dnd5e:conditions:opportunity_attack". An OPEN set, so a string, for
	// [AttackRef.Ref]'s reason.
	Ref string `json:"ref"`

	// Name is the display name for Ref — "Opportunity Attack". Authored
	// beside the ref, never derived from it by a reader.
	Name string `json:"name"`
}

// SequenceRef names the multiattack a swing was performed inside, as
// [ReactionRef] names what a swing was taken as. Carried on [StruckBody],
// [MissedBody] and [WardedBody]; nil on a lone swing.
type SequenceRef struct {
	// Ref is the full ref of the sequence action — the Multiattack's own,
	// not the component it swung. An OPEN set, so a string, for
	// [AttackRef.Ref]'s reason.
	Ref string `json:"ref"`

	// Name is the display name for Ref — "Multiattack". Authored beside the
	// ref, never derived from it by a reader.
	Name string `json:"name"`
}

// ShortfallReason names WHY a declaration is unaffordable, as a value a UI
// can act on rather than prose it can only repeat. Lands with
// rpg-toolkit#1010 (reach) and the structured Shortfall it carries.
//
// Each value is one of the refusals the matching verb would give, named
// here BEFORE the refusal — the same promise Afford has always made,
// extended from the economy to the turn and to reach. A client greys a
// target for NoTargetInReach and dims a shape for NoBudget; it never
// parses Text to tell them apart.
type ShortfallReason string

const (
	// ShortfallNoBudget is the currency running out. Attack exposes it before
	// execution; echoing that unavailable offer is ErrStaleDeclaration. Move's
	// path-specific overrun is ErrCannotAfford. Currency, Needed and Left are
	// populated.
	ShortfallNoBudget ShortfallReason = "no_budget"

	// ShortfallNotYourTurn is it not being this member's turn — what
	// Attack, Move, Activate and EndTurn all refuse as ErrNotYourTurn.
	ShortfallNotYourTurn ShortfallReason = "not_your_turn"

	// ShortfallNoTargetInReach is nothing to swing at within reach. Echoing
	// that unavailable offer is ErrStaleDeclaration; ErrOutOfReach remains the
	// final defensive resolution validation (rpg-toolkit#1010). This is the one
	// shortfall that arrives on a declaration with no Target: when no candidate
	// is in reach, Afford still says so, once, rather than saying nothing.
	ShortfallNoTargetInReach ShortfallReason = "no_target_in_reach"

	// ShortfallDowned is the member being downed and unable to act — what
	// the verbs refuse as ErrDowned.
	ShortfallDowned ShortfallReason = "downed"

	// ShortfallUnreadable is the rulebook being unable to compile this
	// member's price — the sheet is unreadable (ErrBadAttack for anything
	// but an empty hand, which compiles to the unarmed strike instead,
	// rpg-toolkit#1168). Afford reports it rather than failing the read,
	// so the rest of the declarations still arrive.
	ShortfallUnreadable ShortfallReason = "unreadable"

	// ShortfallTargetOutOfReach is the named target failing the attack's
	// reach gate — a CANDIDATE-level reason, never the declaration's. Each
	// out-of-reach candidate row carries it while the declaration-level
	// answer remains ShortfallNoTargetInReach when no candidate is in reach
	// at all (rpg-toolkit#1010, rpg-project#249 §6).
	ShortfallTargetOutOfReach ShortfallReason = "target_out_of_reach"

	// ShortfallWindowOpen is the freeze: somebody at this table is being
	// asked whether they react, and until they answer nothing else may
	// change the world (rpg-project#316 rung 3). The verbs refuse it as
	// ErrWindowOpen.
	//
	// NOT A BUDGET AND NOT A TURN. Nothing ran out and it is still this
	// member's turn — or still is not, and that is not why they are blocked.
	// A client greys the panel and says who the table is waiting for, which
	// it cannot do from either of the reasons this would otherwise be
	// collapsed into.
	//
	// It is also the one shortfall that can appear on a member whose turn it
	// IS NOT, because the freeze is not about whose turn it is.
	ShortfallWindowOpen ShortfallReason = "window_open"

	// ShortfallUnavailable is a provider precondition refusing, such as already
	// raging or an incompatible spell already cast this turn. Currency is empty:
	// this is not a resource shortage. The accompanying text describes the
	// provider's refusal; its resolution depends on that precondition.
	ShortfallUnavailable ShortfallReason = "unavailable"

	// ShortfallNoOption is a Speak row with nothing on its menu: somebody can
	// see the speaker, and nothing here — no creature that sees them, no prop
	// they can address — has a line for them (rpg-project#569). The verb
	// refuses it as ErrNoOption.
	//
	// NOT ShortfallNoTargetInReach, and the difference is the instruction a
	// client gives. "Nobody can see you to be spoken to" means move or make
	// yourself seen; this one means there is nothing here to say, however you
	// stand — or nothing YOU know to say yet, since a line can wait on a fact
	// the speaker has not learned (R9). The two are deliberately one reason:
	// telling them apart would confirm a line the author hid.
	//
	// THE NO-WITNESS CASE STILL WINS when no prop offers a line either: the
	// audience is the first thing missing, and naming the second would answer
	// a question the player has not reached.
	ShortfallNoOption ShortfallReason = "no_option"
)

// Currency names which of a turn's budgets a NO_BUDGET shortfall ran out
// of. Lands with the structured Shortfall (rpg-project#249).
//
// This is the economy's word for what ran out — it is NOT Slot, although
// three values coincide. Slot says which shape a declaration LIGHTS;
// Currency says which ledger a refusal DRAINED, and movement is a ledger
// with no shape. The two are kept separate so a client that lights shapes
// never has to treat feet as a fourth one.
type Currency string

const (
	// CurrencyAction is the standard action.
	CurrencyAction Currency = "action"
	// CurrencyBonus is the bonus action.
	CurrencyBonus Currency = "bonus"
	// CurrencyReaction is the reaction.
	CurrencyReaction Currency = "reaction"
	// CurrencyMovement is feet, not a count — Needed and Left on a MOVEMENT
	// shortfall are in feet, at the server's five per cell.
	CurrencyMovement Currency = "movement"

	// CurrencyCharges is charges of a named feature resource — rage uses,
	// Second Wind uses, ki points. A count, like the three slots.
	//
	// WHICH resource is named only in Text. This seam does not enumerate the
	// rulebook's resource keys, for the reason [Verb] does not enumerate the
	// rulebook's actions: the catalog grows without this contract changing,
	// and a client that branched on a specific pool would be deriving rules.
	CurrencyCharges Currency = "charges"
)

// Shortfall is the structured reason a declaration cannot be paid for.
// Lands with rpg-toolkit#1010 as the seam's own answer to "why not."
//
// STRUCTURED SO THE UI CAN ACT ON IT; TEXT STAYS FOR NARRATION. The string
// form alone ("action: 1 needed, 0 left") is enough to repeat a refusal in
// the player's own words and not enough to do anything else with it —
// Reason is what it branches on, Currency/Needed/Left are the figures it
// shows, and Text is what it says. The three agree by construction: Text is
// rendered from the other four, never the reverse.
type Shortfall struct {
	// Reason is what kind of refusal this is. Always set.
	Reason ShortfallReason `json:"reason"`

	// Currency is which ledger ran out. Set for NoBudget; empty for every
	// other reason, which has no currency to name.
	Currency Currency `json:"currency,omitempty"`

	// Needed is how much the verb costs, in that currency's unit (feet for
	// MOVEMENT, a count otherwise). Meaningful for NoBudget; zero
	// otherwise.
	Needed int `json:"needed,omitempty"`

	// Left is how much is left. Meaningful for NoBudget; zero otherwise —
	// and a real zero, which is the usual case.
	Left int `json:"left,omitempty"`

	// Text is the refusal in the SDK's own words — "action: 1 needed, 0
	// left", "movement: 20 ft needed, 15 ft left", "not your turn", "no
	// target in reach". The same text Declaration.Why carries and the
	// same text the verb's own refusal error would.
	Text string `json:"text"`
}

// TargetKind tells a client which selector shape a Declaration accepts. A
// closed set owned at the seam, mirroring the merged proto's TargetKind. The
// fixed verb mapping is Attack -> TargetMember, Move -> TargetPath, DeathSave
// -> TargetNone, and EndTurn -> TargetNone. Activate is ability-defined rather
// than globally fixed: Help uses TargetMember, while every other currently
// supported ability uses TargetNone. A new kind arrives only with a proven
// executor for it, never in advance.
//
// A blocked declaration keeps the shape known at its compilation point even
// with empty candidates, so a client knows which selector shape it carries.
type TargetKind string

const (
	// TargetNone is the no-target selector shape used by DeathSave, EndTurn,
	// and every currently supported Activate ability except Help.
	TargetNone TargetKind = "none"

	// TargetMember is the one-member selector shape used by Attack and the
	// Help Activate ability. Attack's compiled candidate universe contains
	// every live CurrentVia-nonempty holding except the actor exactly once.
	TargetMember TargetKind = "member"

	// TargetArea is an area cast's selector shape: the caster chooses nobody,
	// and the engine derives who is caught from the shape the spell declares.
	//
	// NOT TargetNone, though both mean "do not prompt". TargetNone already
	// carries "this lands on you" for a self cast; an area centred on the
	// caster lands on everyone BUT them, and one value meaning both is a
	// distinction a client cannot recover. A client that knows the shape can
	// also draw it, which no amount of "no selector" ever permits.
	TargetArea TargetKind = "area"

	// TargetPath is Move's selector shape: a walk along a path on the turn
	// clock. Move declarations carry no candidates; the path is chosen at
	// execution time and priced whole.
	TargetPath TargetKind = "path"

	// TargetCell is the selector shape of an area the caster has to POINT.
	// The caster names a cell the shape is aimed toward; it is a REFERENCE,
	// not a calculation, and it rides the verb the way a path does rather
	// than arriving as a candidate somebody chose.
	//
	// NOT TargetArea, though both derive their recipients. TargetArea's shape
	// is settled the moment the offer is compiled — Thunderclap's burst is
	// centred on the caster and a client can draw it from the profile alone.
	// A cube hanging off the caster's own edge cannot be drawn or resolved
	// until somebody says which way it faces, and a client told only "area"
	// would prompt for nothing and send a cast the door has to refuse.
	//
	// It carries no candidates for the same reason TargetArea carries none: a
	// candidate is somebody you may CHOOSE, and the cells of a room are not a
	// roster. Which cells are legal is the client's own reading of the map it
	// already holds; the door is what refuses a cell that was not.
	TargetCell TargetKind = "cell"
)

// TargetCandidate is one member in a compiled Attack's complete candidate
// universe: every current live-sight holding (CurrentVia non-empty) except the
// actor appears exactly once, sorted by member ID. Stale memories, undisclosed
// members, and the actor are excluded. A live holding whose position is
// missing fails Afford rather than being silently omitted.
//
// The candidate's availability is the TARGET-SPECIFIC gate, independent of
// the declaration's turn/economy gate: a declaration may be unavailable while
// an in-reach candidate remains available, and an out-of-reach candidate
// carries ShortfallTargetOutOfReach on itself while the declaration-level
// reason stays ShortfallNoTargetInReach when no candidate is in reach at all.
type TargetCandidate struct {
	// Member is the candidate member ID, as it appears in the encounter
	// roster and the observer's holdings.
	Member string `json:"member"`

	// Available is whether this candidate passes the attack's reach gate.
	// PLAIN bool, not optional: false is an answer (out of reach), not an
	// absence — the same false-vs-absent law every other bool at this seam
	// keeps.
	Available bool `json:"available"`

	// Why is present if and only if Available is false. Carries a
	// candidate-level reason (ShortfallTargetOutOfReach today); global
	// turn/economy reasons are never copied here — they live on the
	// declaration's Why.
	Why *Shortfall `json:"why,omitempty"`

	// Effects are this target's answers that differ from the declaration's
	// rows, each replacing the row with the same ID (rpg-project#520). Empty
	// when every row reads as declared for this target. They never change
	// Available.
	Effects []TargetEffect `json:"effects,omitempty"`

	// HeldEffects are the effects THIS TARGET holds that bear on the
	// declaration's action, as full rows (rpg-project#520, R18): Faerie Fire
	// on a goblin, Dodging on a duelist. They are never joined with the
	// declaration's rows or with Effects, and each ID ("target:" + ref, plus
	// "@" + source) never equals a declaration row's. Empty when the target
	// holds nothing that bears, or when what it holds is not known — the
	// actor has not sighted it holding anything. They never change
	// Available.
	HeldEffects []EffectRow `json:"held_effects,omitempty"`
}

// Discovery is what changed in one observer's perception.
//
// Three disjoint lists rather than a single "here is everything you now
// perceive": a client rendering a first sighting wants to announce it, a
// refresh wants to update silently, and a fade wants to grey something out.
// Collapsing them would force the client to diff against its own previous
// state to recover the distinction the composition already knows.
type Discovery struct {
	// FirstContact is what came into view that the observer did not hold at
	// all. These are the moments worth announcing.
	FirstContact []Report `json:"first_contact,omitempty"`

	// Refreshed names subjects the observer already held whose knowledge was
	// renewed by a live channel.
	Refreshed []string `json:"refreshed,omitempty"`

	// Faded names subjects that stopped being sustained. The observer keeps a
	// memory; it is no longer a live sighting.
	Faded []string `json:"faded,omitempty"`
}

// Report is one newly perceived subject and what is known about it.
type Report struct {
	// Subject names what was perceived.
	Subject string `json:"subject"`

	// Seen is the sight channel's own typed knowledge, carried the same way
	// [Sighting.Seen] is — first contact and a held sighting are the same
	// fact in the same shape.
	//
	// UNLIKE Sighting.Seen, this is not gated on channel provenance: Report
	// (unlike Holding) carries no Channel field, so this is populated by
	// decoding the payload and checking whether it parses, not by asking
	// what channel produced it. See [Seen]'s own doc and projectReportSeen
	// in convert.go. Do not read a non-nil Seen here as proof the report
	// came from sight — today it always does, because sight is the only
	// channel this codebase surveils with, not because this field checked.
	Seen *Seen `json:"seen,omitempty"`

	// Payload is what the observer learned, encoded by the composition.
	// Retained for channels the SDK has not typed.
	Payload []byte `json:"payload,omitempty"`
}

// RecheckInput names the members whose observable facts changed outside the
// session.
type RecheckInput struct {
	// Session is the session to tell.
	Session string
	// Members is who changed. NOT what about them changed — see
	// [Manager.Recheck].
	Members []string
}

// RecheckOutput reports what the re-look saved and delivered.
type RecheckOutput struct {
	// Discovered is the perception movement the refresh produced, the same
	// shape every other verb reports it in. A re-look ordinarily produces
	// none: nobody moved.
	Discovered map[string]Discovery
	// Saved and Delivery are the persistence and stream reports every
	// changing verb returns.
	Saved    SaveReport
	Delivery DeliveryReport
}

// LevelChoiceKind names what a level is asking a player to pick.
//
// A string enum of this package's own, for the reason [MemberKind] is one: it
// maps onto a proto enum, so adding a kind must be a compatible change. It is
// deliberately NOT a mirror of the rulebook's [choices.Requirements] fields —
// that shape has eleven of them and this seam can translate two. A kind
// missing from this list is refused with [ErrLevelNotOffered] rather than
// dropped; see [Manager.NextLevel].
type LevelChoiceKind string

const (
	// LevelChoiceCantrip asks for cantrips. Options and selections are
	// canonical spell refs.
	LevelChoiceCantrip LevelChoiceKind = "cantrip"

	// LevelChoiceSpell asks for leveled spells. Options and selections are
	// canonical spell refs, and [LevelChoice.SpellLevel] says which spell
	// level they are.
	LevelChoiceSpell LevelChoiceKind = "spell"
)

// LevelChoice is one question the next level asks.
//
// Options is the WHOLE universe this choice ranges over — the class's row for
// that level with everything the character already knows removed, which is the
// character's own view of the row rather than the class's (rpg-toolkit#1781).
//
// Options is therefore never shorter than Count. A row this build cannot
// satisfy — the class table says two spells are learned and no list has been
// authored for their spell level — is refused by [Manager.NextLevel] with
// [ErrLevelNotOffered] rather than projected (R4.4f), because the write refuses
// it too: a question rendered with a count of two and nothing to pick from is a
// disabled confirm and no explanation, which is the confirmation-the-write-will-
// refuse shape this seam exists to close.
type LevelChoice struct {
	// ID is the requirement's own identifier, e.g. "bard-spells-2". A
	// submission names this back.
	ID string `json:"id"`

	// Label is the requirement's own words, e.g. "Choose 1 supported
	// 1st-level spell". Carried rather than composed: the rulebook already
	// writes the sentence, and a host that rebuilt it from Count and Kind
	// would be authoring content — and would lose the one the rulebook uses
	// when a count equals its whole option list ("Select all ...", which is
	// not a choice and says so).
	Label string `json:"label,omitempty"`

	// Kind is what is being chosen. CARRIED, NEVER INFERRED: a host reading
	// SpellLevel or the option strings to work out which question this was
	// would be guessing at something the row states.
	Kind LevelChoiceKind `json:"kind"`

	// Count is how many to choose. It is the CLASS's number and is not
	// reduced along with Options: how many spells a level teaches is a rule,
	// not a property of who is taking it.
	Count int `json:"count"`

	// SpellLevel is which spell level the options are, for
	// [LevelChoiceSpell]. Zero for a cantrip choice, where the question does
	// not arise.
	SpellLevel int `json:"spell_level,omitempty"`

	// Options are the canonical refs this choice may select from, e.g.
	// "dnd5e:spells:healing-word".
	Options []string `json:"options,omitempty"`
}

// NextLevelInput asks what the next level would be for one stored character.
//
// The character ID is the whole universe: this read ranges over one sheet and
// the class tables, and names both in its answer.
type NextLevelInput struct {
	// Character is the stored character to ask about.
	Character string
}

// NextLevelOutput describes the level this character would take next, and what
// its sheet says about whether it may.
//
// THIS VERB DECIDES NOTHING. "A level is available" is the gap between Level
// and EntitledLevel (R4.10) — both are projected and the comparison is the
// host's, because a flag here would be a rule in the seam.
type NextLevelOutput struct {
	// Level is the character level the sheet holds NOW. CharacterLevel below
	// is the one the next level would make it.
	Level int `json:"level"`

	// ClassLevel is the class level the next level would take.
	ClassLevel int `json:"class_level"`

	// CharacterLevel is the character level the next level WOULD TAKE, which
	// is Level plus one. Equal to ClassLevel until multiclassing opens, and
	// stated separately so the day it opens this surface does not change
	// shape.
	CharacterLevel int `json:"character_level"`

	// Class is the character's class as a CANONICAL REF,
	// "dnd5e:classes:bard" — the same vocabulary the rulebook already uses
	// for a class source ref, so a host maps the id after the second colon
	// and nothing here invents a second way to name a class. Multiclassing
	// is not open, so the next level is taken in this class (R2.4).
	Class string `json:"class"`

	// ClassName is the class's display name, "Bard". Projected here because
	// it is something the host wants to display and the rulebook is the only
	// thing that knows it; a host spelling it from the id would be authoring
	// content.
	ClassName string `json:"class_name"`

	// Experience is the character's cumulative total.
	Experience int `json:"experience"`

	// EntitledLevel is the highest level that total has earned. Greater than
	// Level means a level is waiting to be taken.
	EntitledLevel int `json:"entitled_level"`

	// NextLevelThreshold is the total needed before another level is earned,
	// or 0 at the top of the table — zero telling the truth rather than
	// hiding a level, since no reading of a table whose first threshold is 0
	// produces "0 more needed".
	NextLevelThreshold int `json:"next_level_threshold"`

	// HitDie is the class's hit die, e.g. 10 for a fighter. What the level
	// does with it is [Manager.LevelUp]'s HitPointMethod.
	HitDie int `json:"hit_die"`

	// Features are the canonical refs the level grants, carried verbatim
	// from the class's authored grant rows.
	Features []string `json:"features,omitempty"`

	// Choices are the questions the level asks THIS character. Empty for
	// most levels of most classes, which is a level and not an error.
	Choices []LevelChoice `json:"choices,omitempty"`
}

// LevelUpHitPointMethod selects how a level's hit point gain is produced.
//
// This package's own string type rather than the rulebook's, for the S2 reason
// every enum here is: a host wiring this verb must not have to name a toolkit
// type. It offers TWO of the rulebook's three — the level-1-only "max" is not
// a level-up answer and is refused before any I/O rather than carried down to
// be refused by the rulebook after a sheet has been read.
type LevelUpHitPointMethod string

const (
	// HitPointsRolled rolls the class hit die through the host's own Dice.
	HitPointsRolled LevelUpHitPointMethod = "rolled"

	// HitPointsAverage takes the class's fixed average, half the die plus
	// one (PHB p.15).
	HitPointsAverage LevelUpHitPointMethod = "average"
)

// LevelChoiceSubmission answers one [LevelChoice].
//
// IT CARRIES NO CATEGORY. The client names the choice and its selections; what
// KIND of question that id was is read back off the level's own requirement
// row, so a client cannot pick the wrong one and a level that changes what it
// asks does not need every client to change with it.
type LevelChoiceSubmission struct {
	// ChoiceID is the [LevelChoice.ID] being answered. An id the level did
	// not ask for is refused with [ErrBadLevelRequest].
	ChoiceID string

	// Selections are canonical refs, e.g. "dnd5e:spells:healing-word". A
	// string that is not a ref, or names no spell this build carries, is
	// refused before the rulebook is called.
	Selections []string
}

// LevelUpInput takes the level a character has earned.
type LevelUpInput struct {
	// Character is the stored character to advance.
	Character string

	// HitPointMethod is how the hit point gain is produced. The VALUE is the
	// rulebook's; a caller-supplied number would put a game rule in the host.
	HitPointMethod LevelUpHitPointMethod

	// Choices answers every question [Manager.NextLevel] reported. A level
	// that asks nothing takes none, and one supplied anyway is refused: the
	// level record is append-only, so a choice nobody asked for could never
	// be taken back.
	Choices []LevelChoiceSubmission
}

// ResourceMaximumChange is one pool whose maximum the level moved.
type ResourceMaximumChange struct {
	// Key is the resource key, e.g. "spell_slot_level_1".
	Key string `json:"key"`

	// Name is the display name for that key, or the key itself when the
	// rulebook's table has no name for it. A pool nobody named still ships,
	// named by its key, rather than being dropped from the account.
	Name string `json:"name"`

	// From is the maximum before this level. Zero for a pool the character
	// did not have.
	From int `json:"from"`

	// To is the maximum after it.
	To int `json:"to"`
}

// LevelGained is what one level added, as the sheet can describe it
// immediately afterwards.
//
// A PROJECTION FOR DISPLAY. Nothing reads it back: every field is recomputable
// from the level record and the current rules, which is the whole point of
// recording a level's inputs rather than its effects.
//
// Conditions granted at a level are deliberately absent. The rulebook reports
// them beside the features and no consumer has a place to put them yet; a
// field nobody fills is a worse answer than one that is not there, and the day
// one is needed it is a compatible addition.
type LevelGained struct {
	// CharacterLevel is the level the character is now.
	CharacterLevel int `json:"character_level"`

	// ClassLevel is the class level it is now.
	ClassLevel int `json:"class_level"`

	// HitPointGain is what the level added to the maximum, die plus
	// Constitution modifier.
	HitPointGain int `json:"hit_point_gain"`

	// ProficiencyBonus is the bonus AT the new level, not the change.
	ProficiencyBonus int `json:"proficiency_bonus"`

	// Class is the class the level was taken in, as a canonical ref —
	// "dnd5e:classes:bard", the same vocabulary [NextLevelOutput.Class]
	// carries. Stated here rather than left for the host to remember from
	// the read, because a host may write without having read and because
	// multiclassing will make "which class did this level go into" a real
	// question rather than a restatement.
	Class string `json:"class"`

	// ClassName is that class's display name.
	ClassName string `json:"class_name"`

	// Features are the canonical refs the level granted.
	Features []string `json:"features,omitempty"`

	// Resources are the pools whose maximum the level moved.
	Resources []ResourceMaximumChange `json:"resources,omitempty"`
}

// LevelUpOutput reports what the level persisted and what it added.
//
// IT DOES NOT RETURN THE SHEET. character.Data crosses this boundary on the
// repository alone (S2), so a host that wants the new sheet re-reads it
// through the repository it already implements — which is also the only read
// that can be trusted to show what actually landed.
type LevelUpOutput struct {
	// Saved names what was persisted: one character aggregate.
	Saved SaveReport `json:"saved"`

	// Gained is what the level added.
	Gained LevelGained `json:"gained"`
}
