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
// the asymmetry is not read as an oversight: StartSessionInput.World is
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

// Outcome describes how an encounter ended.
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

	// Seq is the story sequence of the beat that recorded the fight starting.
	Seq uint64 `json:"seq"`
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
// as a known location payload (encounter.DecodeLocationPayload succeeds). Nil
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
// On [Report] this is weaker (Copilot review, PR #1159): intel.Report carries
// no Channel of its own, so a Report's Seen is inferred by decoding its
// payload rather than gated on provenance — see projectReportSeen's own
// comment in convert.go. It is not yet an authoritative "this was sight"
// discriminator on a Report the way it is on a Sighting; do not treat it as
// one until a second channel exists to prove the distinction matters.
//
// ADR-0041: channel-keyed typed sub-structs on Sighting/Report. A future
// channel (hearing, tremorsense) gets its own sub-struct with the facts that
// channel actually conveys; a sight-specific fact that arrives later
// (lighting, distance band) belongs inside Seen, not as a new field on
// Sighting.
type Seen struct {
	// Position is the sighted subject's cell, dungeon-absolute.
	Position spatial.Position `json:"position"`

	// Standing is whether the sighted subject is on their feet.
	// SIGHT-CHANNEL KNOWLEDGE, not roster truth: an observer who can see a
	// member can see whether they are standing, which is why it belongs
	// inside Seen and not on a roster read this seam deliberately lacks.
	// A memory (CurrentVia empty) keeps the standing it last saw, exactly
	// as it keeps the position. Lands with rpg-toolkit#1137.
	Standing Standing `json:"standing"`
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

// Sighting is one thing an observer knows through Intel. Status and CurrentVia
// distinguish current perception from held memory.
type Sighting struct {
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

// IntelCorrection reports that an observer corrected a subject's location
// knowledge. It mirrors encounter-owned correction deltas by identifier only,
// never exposes concealed truth, and does not ask session to decide behavior.
type IntelCorrection struct {
	// Observer is the member whose own location knowledge changed.
	Observer string `json:"observer"`
	// Subject is the member whose location is now explicitly unknown to the
	// observer.
	Subject string `json:"subject"`
}

// EventKind names what an event reports.
//
// A string enum rather than free-form prose, because this is the field clients
// branch on: it must be machine-readable and stable, and it maps directly onto
// a proto enum. Adding a kind is compatible; changing what an existing one
// means is not.
type EventKind string

const (
	// EventMoved reports that a member stepped to a new cell.
	EventMoved EventKind = "moved"

	// NOTE: there is no EventTraversed any more.
	//
	// It reported that a step carried a member through a doorway, and it was
	// kept distinct from EventMoved on the argument that ONE MAP DOES NOT MEAN
	// ONE NARRATION — a client renders a doorway differently from a corridor,
	// and making it re-derive that from geometry was the thing to avoid.
	//
	// The composition stopped emitting the beat. A crossing is written like any
	// other step now (rpg-toolkit#1048, #1059), which was the point: absolute
	// coordinates made a crossing expressible as an ordinary move. So nothing
	// upstream can distinguish the two, and a kind nothing can produce is worse
	// than no kind at all — it reads as a contract.
	//
	// The information is not lost, only moved: [Atlas.Doorways] lists every
	// crossable pair, so a step whose from/to matches one IS a traversal and a
	// client (or this package) can say so. Restoring the distinction means
	// deriving it here rather than waiting for a beat that is not coming.

	// EventJoined reports that a member entered the encounter.
	EventJoined EventKind = "joined"

	// EventExited reports that a member left the encounter.
	EventExited EventKind = "exited"

	// EventEnded reports that the encounter closed.
	EventEnded EventKind = "ended"

	// EventSceneOpened reports the encounter's opening beat.
	EventSceneOpened EventKind = "scene_opened"

	// EventTick reports that the clock advanced.
	EventTick EventKind = "tick"

	// EventTurnEnded reports that a member's turn in a fight ended and the
	// order moved on.
	//
	// It arrived at clients as EventUnknown until the turn verbs existed —
	// delivered but uninterpretable, which is correct by the delivery rule and
	// useless to a client rendering a turn tracker. The beat was always there;
	// nothing in this package could produce it.
	EventTurnEnded EventKind = "turn_ended"

	// EventFightStarted reports that two sides came into contact and a fight
	// began, with its initiative order.
	//
	// Every member of the encounter hears it, not only the ones in the fight:
	// a fight is localized (the rest of the party keeps free-roaming) but it is
	// not secret, and a client that learns about it only from the fighters'
	// own responses could not render the party's shared view of the scene.
	//
	// It is the reason this list needed extending at all. The beat existed as
	// soon as the composition started fights by itself (rpg-toolkit#964) and
	// arrived at clients as EventUnknown — technically fine, since unknown
	// beats are delivered rather than dropped, and useless in practice for the
	// single most important thing that can happen in a session.
	EventFightStarted EventKind = "fight_started"

	// EventFightEnded reports that a fight dissolved and its members returned
	// to free roam.
	EventFightEnded EventKind = "fight_ended"

	// EventStruck reports that an attack landed, and EventMissed that one did
	// not. The beat carries the numbers behind it: the roll, what it totalled,
	// and what it had to reach. A landed blow carries how much was done as
	// well; a miss OMITS that key rather than reporting zero, because a beat
	// saying "missed for 0" reads as a hit that did nothing.
	//
	// Two kinds rather than one with a hit flag, because a client branches on
	// them: a landed blow and a whiffed one are different animations, different
	// sounds, and different things to say. Reading a boolean out of the payload
	// to decide which is exactly the interpretation this enum exists to spare
	// whoever renders it.
	//
	// The same round trip EventTurnEnded and EventFightStarted went through,
	// and the third time it has happened: the beat existed as soon as a strike
	// could be recorded (rpg-toolkit#966), and until rpg-toolkit#1038 every
	// swing reached a client as EventUnknown — delivered, uninterpretable, and
	// useless for the most common thing in a fight.
	//
	// Everyone in the encounter hears both, for the reason EventFightStarted
	// spells out: an outcome is not secret, and a table that learned about a
	// blow only from the striker's own response could not narrate the scene it
	// is in.
	EventStruck EventKind = "struck"

	// EventMissed reports that an attack did not land. See EventStruck.
	EventMissed EventKind = "missed"

	// EventActivated reports the authored ability a member successfully used.
	// It precedes every EventActivationResult produced by that activation.
	EventActivated EventKind = "activated"

	// EventActivationResult reports one ordered rulebook effect produced by a
	// successful activation. Its body carries exactly one result pointer.
	EventActivationResult EventKind = "activation_result"

	// EventDeathSave reports one explicit authoritative Death Save result.
	// Its PresentationID is the same opaque token returned to the actor and to
	// every other witness; Seq remains recipient-local.
	EventDeathSave EventKind = "death_save"

	// EventDowned reports that a member is DOWNED: at zero hit points. It does
	// not claim initiative removed them: Dying and Stabilized retain provider-
	// defined slots, while Dead characters and Defeated monsters are removed.
	// The opposite standing is UP.
	//
	// Downed rather than "down" because a bare "down" also reads as PRONE, and
	// prone is a different thing entirely — a posture condition the rulebook
	// tracks, still in the fight and still acting. A client narrating these two
	// the same way would say somebody died every time they were knocked flat.
	// Kirk's ruling, rpg-toolkit#1084.
	//
	// There is no separate event for coming back UP. A natural-20 Death Save
	// reports recovery, restored hit points, and its continuation through
	// EventDeathSave, so a second event would duplicate the same transition.
	//
	// The world NOTICED it; nobody announced it. There is no downing verb and
	// no way to push this beat in — the composition asks the rulebook who is
	// standing at every sight refresh and narrates what it learns
	// (rpg-toolkit#1077), so this arrives on whatever verb happened to refresh
	// sight, which is frequently not the verb that dealt the damage.
	//
	// Kind and who, and nothing else. Hit points are not on the wire here: what
	// a client renders is somebody down, and how much damage produced it is a
	// separate question with its own answer (ruled fork (d) on
	// rpg-toolkit#959).
	//
	// It is in the same family as EventStruck and EventMissed, tagged the same
	// way and delivered to the same audience, because a client reads all three
	// side by side while narrating one exchange.
	EventDowned EventKind = "downed"

	// EventStanceChanged reports that the stance between two factions
	// turned (rpg-project#375, the hold-out design §3.4–§3.5, §5): a
	// disposition's `until` held — the faction's mind came to know the fact
	// the author named — and the pair's stance folded to neutral.
	//
	// Every member of the encounter hears it. A stance is TRUTH GRAIN, like
	// a door standing open: the same for everyone, however each of them came
	// to be standing where they are. What is per-member is the KNOWLEDGE
	// that led to it — which member carried what to whom — and that never
	// rides this beat; it reaches its holders on their own streams as the
	// reveals it always was.
	//
	// A fight formed between members of the two factions dissolves in the
	// same pass, so an EventFightEnded carrying DissolveByStance follows
	// this beat for everyone who could hear the fight; members of a third
	// faction still hostile keep theirs. And a declared ending waiting on
	// the stance — the hold-out scenario's own — fires after that, as
	// EventEnded naming its key, exactly as any ending is named.
	//
	// The composition says "stance" and this seam says "stance_changed":
	// the wire's word is the event, not the noun, matching FIGHT_ENDED and
	// DOOR_REVEALED beside it (rpg-api-protos EVENT_KIND_STANCE_CHANGED).
	EventStanceChanged EventKind = "stance_changed"

	// EventDoor reports a door changing — opened, closed, or an unlock
	// attempt, beaten or not. A failed attempt is as much fiction as a
	// beaten one, and the composition writes both through one path
	// (rpg-project#268). Every member of the encounter hears it: door
	// knowledge is roster-wide until asymmetric perception
	// (rpg-toolkit#1020) narrows it.
	EventDoor EventKind = "door"

	// EventDoorRevealed is a concealed door entering THIS RECIPIENT's
	// knowledge — their own search, a crossing, or perceiving it open. The
	// body is the patch for the recipient's cached atlas and door list:
	// the door's doorways and live state, plus the lock's approaches when
	// locked. Always recipient-scoped to exactly one member (detection
	// beats are per-player from birth).
	EventDoorRevealed EventKind = "door_revealed"

	// EventRegionRevealed is a concealed region entering THIS RECIPIENT's
	// knowledge — perceiving its door open, or standing inside it. The
	// body carries the region's whole atlas slice as the recipient's own
	// atlas now answers it: entry, props, and every boundary touching its
	// cells. Always recipient-scoped to exactly one member.
	EventRegionRevealed EventKind = "region_revealed"

	// EventLooted reports that a member looted a downed body — who looted
	// whom, and DELIBERATELY NOTHING ELSE (rpg-project#368 P3).
	//
	// The affordance must not say which body carries anything. Loot is
	// offered on every downed member, a body with nothing transfers nothing,
	// and this beat is identical either way: no list, no count, no flag. What
	// actually moved reaches the LOOTER ALONE as the kind that carries it —
	// intel arrives as EventDoorRevealed on their own stream, byte-identical
	// to the reveal a successful search produces. Everyone present hears
	// this one.
	EventLooted EventKind = "looted"

	// EventHeld reports that a member picked a holdable prop up.
	//
	// PHYSICAL STATE FOLDS ON THE TRUTH GRAIN: an object leaving the floor is
	// not a secret, so this goes to everyone present and every recipient's
	// atlas loses the prop. A client patches its cached Atlas by removing the
	// prop with this id — the load-once, beat-refreshed law running in the
	// subtractive direction, where EventDoorRevealed runs it in the additive
	// one — and a refetch agrees, because Atlas omits held props for
	// everyone.
	EventHeld EventKind = "held"

	// EventDropped reports that a holding landed back on the map.
	//
	// NOT A PLAYER VERB, and this slice adds none: a drop is what happens
	// when a carrier leaves from anywhere but a scenario's bound exit (design
	// R9). Without it a carrier who walks out through the lobby — or simply
	// disconnects — walks off with the only win in the run. The prop
	// reappears at [DroppedBody.At] for everyone present; the journal
	// underneath stays append-only, since a drop is a new fact rather than
	// the erasure of the hold.
	EventDropped EventKind = "dropped"

	// EventUnknown is a beat this version does not recognise.
	//
	// Delivered rather than dropped on purpose: a client that cannot interpret
	// an event still learns its sequence advanced, so gap-detection keeps
	// working. Dropping it would manufacture a hole and trigger a resync that
	// was never needed. It also means a newer composition can add beats without
	// older clients losing their place.
	EventUnknown EventKind = "unknown"
)

// Event is one thing that happened, addressed to one recipient.
//
// Deliberately flat and non-polymorphic: it maps onto a proto message, so no
// interface-valued fields, no type switches on the wire, and no payload shape
// that varies by kind in a way a generated client cannot express.
//
// An Event is per-recipient, not per-occurrence. The same underlying beat
// becomes several Events, one for each viewer who may know about it, and their
// payloads may differ — a player being offered a choice receives the choices,
// while everyone else receives only that the world is waiting. Projection is a
// rule, decided where perception lives; filtering a single shared payload would
// mean the difference between viewers was a delivery detail, and the first
// mistake would leak something unperceived.
type Event struct {
	// Session is the session this event belongs to.
	Session string `json:"session"`

	// Seq is this event's position in THE RECIPIENT'S OWN delivered stream:
	// dense from the recipient's point of view, monotonic, and never
	// renumbered (stream.go). A recipient that notices a gap in Seq has
	// missed an event and can re-query the story from its last known value
	// — and because the numbering is per-recipient, a beat delivered to
	// somebody else leaves NO hole here: the record's global sequence stays
	// internal to the seam (the gap-oracle ruling, rpg-project#351).
	Seq uint64 `json:"seq"`

	// At is the clock reading when the underlying beat was recorded.
	At uint64 `json:"at,omitempty"`

	// Correlation groups cause and effect across events. Empty is legal.
	Correlation string `json:"correlation,omitempty"`

	// Tags is queryable metadata describing the beat, carried unchanged from
	// the story-log entry this event was projected from. Before
	// rpg-api-protos#239 only a Story catch-up carried this field; a live
	// subscriber now gets the identical Tags too, so a client resolving a
	// gap by re-querying Story is never handed a thinner answer than what
	// arrived live.
	Tags map[string]string `json:"tags,omitempty"`

	// Recipient is the member this projection is addressed to.
	Recipient string `json:"recipient"`

	// Kind names what happened.
	Kind EventKind `json:"kind"`

	// Payload is the kind-specific body, encoded by this package. REMAINS
	// FOR KINDS NOT YET TYPED — a client reads Body and never decodes this,
	// not as a fallback, not for a field it wishes were typed (rpg-toolkit#941).
	Payload []byte `json:"payload,omitempty"`

	// Body is the beat, typed: a sealed interface with one struct per kind
	// that has one, decoded from the SAME payload this package already
	// wrote rather than a second encoding of it (rpg-toolkit#941). Nil in
	// three cases: a kind with no typed body member (SCENE_OPENED, TICK,
	// UNKNOWN); a beat this build's decoder does not recognise; and a
	// beat whose declared kind IS recognised but whose payload does not
	// match that kind's required shape — bodyFor's own refusal rather than
	// a zero-valued body (see TestBodyForRefusesAMissingRequiredField).
	Body EventBody `json:"-"`
}

// EventBody is the beat, typed — a sealed interface with one struct per
// kind Event.Body carries: TurnEndedBody, DownedBody, DeathSaveBody,
// StruckBody, MissedBody, ActivatedBody, ActivationResultBody, FightStartedBody,
// FightEndedBody, MovedBody, JoinedBody, ExitedBody, EndedBody, DoorBody,
// StanceChangedBody.
// Sealed the way
// DissolveCause is (dissolve.go) and for the same reason: a caller matches
// on it with a type switch, and a second implementation declared outside
// this package would be indistinguishable from these to anyone reading the
// switch.
//
// ONE-TO-ONE WITH EventKind where a body exists (rpg-toolkit#941). A kind
// with no body member here leaves Event.Body nil and is read from Kind
// alone.
type EventBody interface {
	// isEventBody seals the set.
	isEventBody()
}

// TurnEndedBody is EventTurnEnded's typed body: a member's turn ended and
// the order moved on. Also "X's turn" as a moment — one of these per driven
// member, so a monster's turn is a visible beat rather than a gap between
// two of a player's.
type TurnEndedBody struct {
	// Member is whose turn ended.
	Member string `json:"member"`
	// Next is whose turn it is now.
	Next string `json:"next"`
}

func (TurnEndedBody) isEventBody() {}

// DownedBody is EventDowned's typed body: who is at zero hit points. It says
// nothing about initiative retention or removal; provider-derived LifeState
// carries that distinction. Who, and nothing else — see EventDowned's own doc
// for why hit points are not here.
type DownedBody struct {
	Member string `json:"member"`
}

func (DownedBody) isEventBody() {}

// DeathSaveBody is EventDeathSave's typed game result. It contains the same
// projected provider facts as DeathSaveOutput, excluding persistence/delivery
// and recipient-local sequence.
type DeathSaveBody struct {
	Actor             string                `json:"actor"`
	Roll              int                   `json:"roll"`
	Outcome           DeathSaveOutcome      `json:"outcome"`
	SuccessesAdded    int                   `json:"successes_added"`
	FailuresAdded     int                   `json:"failures_added"`
	Successes         int                   `json:"successes"`
	Failures          int                   `json:"failures"`
	SuccessesNeeded   int                   `json:"successes_needed"`
	FailuresRemaining int                   `json:"failures_remaining"`
	Stabilized        bool                  `json:"stabilized"`
	Dead              bool                  `json:"dead"`
	Recovered         bool                  `json:"recovered"`
	HPRestored        int                   `json:"hp_restored"`
	Continuation      DeathSaveContinuation `json:"continuation"`
	PresentationID    string                `json:"presentation_id"`
}

func (DeathSaveBody) isEventBody() {}

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

// AttackModifierSource identifies one replayable advantage/disadvantage
// source. Human-readable rules-engine reasons deliberately do not cross this
// seam.
type AttackModifierSource struct {
	SourceRef string `json:"source_ref,omitempty"`
	SourceID  string `json:"source_id,omitempty"`
}

// StruckBody is EventStruck's typed body: an attack landed. The numbers
// AttackOutput gives the attacker, here for every witness, plus what was
// swung.
type StruckBody struct {
	Attacker string `json:"attacker"`
	Target   string `json:"target"`
	// Roll is the d20 as rolled.
	Roll int `json:"roll"`
	// Total is the roll plus everything the attack chain added.
	Total int `json:"total"`
	// Against is the number the total had to reach.
	Against int `json:"against"`
	// Damage is what was dealt. Never zero on a Struck; a miss is a Missed.
	Damage int `json:"damage"`
	// Attack is what was swung — ref, name, damage type.
	Attack   AttackRef `json:"attack"`
	Critical bool      `json:"critical"`
	// DamageComponents are ordered inputs to the authoritative aggregate Damage.
	DamageComponents []DamageComponent `json:"damage_components,omitempty"`
	// AdvantageSources and DisadvantageSources preserve the fold's attribution.
	AdvantageSources    []AttackModifierSource `json:"advantage_sources,omitempty"`
	DisadvantageSources []AttackModifierSource `json:"disadvantage_sources,omitempty"`
}

func (StruckBody) isEventBody() {}

// MissedBody is EventMissed's typed body: an attack did not land. A
// separate body from StruckBody rather than one with a Damage of zero — a
// whiff is a different animation, sound and sentence — and it carries no
// damage field at all so "missed for 0" cannot be said.
type MissedBody struct {
	Attacker string    `json:"attacker"`
	Target   string    `json:"target"`
	Roll     int       `json:"roll"`
	Total    int       `json:"total"`
	Against  int       `json:"against"`
	Attack   AttackRef `json:"attack"`
}

func (MissedBody) isEventBody() {}

// ActivatedBody is EventActivated's typed body. Ability is copied from the
// selected server-authored declaration; Session does not derive its name from
// its ref. Target is empty for abilities that select nobody.
type ActivatedBody struct {
	Actor   string     `json:"actor"`
	Ability AbilityRef `json:"ability"`
	Target  string     `json:"target,omitempty"`
}

func (ActivatedBody) isEventBody() {}

// ActivationResultBody is EventActivationResult's typed body. Exactly one of
// its result pointers is non-nil on a valid body, and effects remain in the
// order resolution published them.
type ActivationResultBody struct {
	Actor            string                `json:"actor"`
	HealingApplied   *HealingAppliedBody   `json:"healing_applied,omitempty"`
	ConditionApplied *ConditionAppliedBody `json:"condition_applied,omitempty"`
	ConditionRemoved *ConditionRemovedBody `json:"condition_removed,omitempty"`
	CapacityGranted  *CapacityGrantedBody  `json:"capacity_granted,omitempty"`
}

func (ActivationResultBody) isEventBody() {}

// HealingAppliedBody carries authoritative post-clamp healing facts from the
// rulebook, including the requested amount and the source that authored it.
//
// The roll behind the requested heal is carried by [Calculation] — the sourced
// dice trace and modifiers, in production order, exactly as the rulebook
// published them. Roll and Modifier below it are the legacy READ
// representation for payloads written before roll traces: the strict decoder
// accepts exactly one representation and never fabricates a Calculation from
// the legacy scalars.
type HealingAppliedBody struct {
	Target    string `json:"target"`
	Amount    int    `json:"amount"`
	Requested int    `json:"requested"`
	// Roll is the legacy scalar dice result, readable only from a pre-trace
	// payload.
	Roll int `json:"roll,omitempty"`
	// Modifier is the legacy scalar modifier, readable only from a pre-trace
	// payload.
	Modifier   int    `json:"modifier,omitempty"`
	SourceRef  string `json:"source_ref"`
	SourceName string `json:"source_name"`
	HPBefore   int    `json:"hp_before"`
	HPAfter    int    `json:"hp_after"`

	// Calculation is the sourced roll behind the requested heal.
	Calculation *RollCalculation `json:"calculation,omitempty"`
}

// ConditionAppliedBody identifies a condition attached to a target.
type ConditionAppliedBody struct {
	Target string `json:"target"`
	Ref    string `json:"ref"`
	Name   string `json:"name"`
}

// ConditionRemovedBody identifies a condition removed from a target and the
// provider-authored reason it ended.
type ConditionRemovedBody struct {
	Target string `json:"target"`
	Ref    string `json:"ref"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// CapacityGrantedBody carries the member and provider-authored description of
// capacity an activation banked.
type CapacityGrantedBody struct {
	Member      string `json:"member"`
	Description string `json:"description"`
}

// FightStartedBody is EventFightStarted's typed body: two sides came into
// contact and a fight began. Delivered to every member of the encounter, in
// or out of the fight.
type FightStartedBody struct {
	// Members is the fight's members in initiative order, first to act
	// first.
	Members []string `json:"members"`
}

func (FightStartedBody) isEventBody() {}

// FightEndedBody is EventFightEnded's typed body: a fight dissolved and its
// members returned to free roam. Cause says why, in the same enum
// DissolveOutput.Cause speaks.
type FightEndedBody struct {
	Cause DissolveKind `json:"cause"`
}

func (FightEndedBody) isEventBody() {}

// StanceChangedBody is EventStanceChanged's typed body: which pair of
// factions, and the stance they now hold.
//
// Between is UNORDERED and exactly two: a disposition is one per pair and
// has no direction (rpg-project#375, design §2), so [goblins, party] and
// [party, goblins] name the same edge — carried as the composition wrote
// it, in its own order. Stance is the dungeon file's closed vocabulary —
// hostile, neutral or allied — carried as the author's word rather than an
// enum, the way EndedBody.Ending carries the author's key: what a stance
// MEANS to a client (which colour, which sentence) is content.
type StanceChangedBody struct {
	Between []string `json:"between"`
	Stance  string   `json:"stance"`
}

func (StanceChangedBody) isEventBody() {}

// MovedBody is EventMoved's typed body: a member stepped to a new cell.
// One body per step — a walk of four cells is four of these, each with its
// own Event.Seq.
type MovedBody struct {
	Member string           `json:"member"`
	To     spatial.Position `json:"to"`
}

func (MovedBody) isEventBody() {}

// JoinedBody is EventJoined's typed body: who entered the encounter. The
// same member the beat's audience already includes (Join's own memberIDs),
// carried here so a client rendering the beat does not have to cross-reference
// Event.Recipient to learn who arrived.
type JoinedBody struct {
	Member string `json:"member"`
}

func (JoinedBody) isEventBody() {}

// ExitedBody is EventExited's typed body: who left the encounter, what they
// carried out, and the authored way they left by. See JoinedBody's own doc for
// why the departure names itself — the same reasoning, the opposite beat.
type ExitedBody struct {
	Member string `json:"member"`

	// Holding is the PROP placement ids the member carried out
	// (rpg-project#368 §5). Empty is the ordinary case and the honest one:
	// most departures carry nothing.
	//
	// SINGULAR ON PURPOSE, though the field is a list: it is the participle
	// in the statement this beat makes — "Aldric exited, holding the
	// heirloom" — which is the naming rule the whole slice runs on. The seam
	// already names repeated fields this way where the word is not a count
	// noun (ExitOutput.Carry, the atlas's Sealed).
	//
	// PROPS ONLY. Intel is a holding too, and the same one mechanism moves
	// it, but it NEVER appears here or anywhere else that leaves this seam
	// (design P3): a departure carrying nothing but knowledge is
	// indistinguishable from one carrying nothing at all, which is the
	// point.
	//
	// A departure from anywhere but a scenario's bound exit does not populate
	// this — the carrier DROPS what they hold where they stood (R9) and
	// EventDropped says so, so a holding is either carried out or left on the
	// floor, never silently deleted.
	Holding []string `json:"holding,omitempty"`

	// Exit is the authored exit id they left through — one of the dungeon
	// file's `exits[].id`.
	//
	// EMPTY FOR A DEPARTURE FROM ELSEWHERE, which is every departure today
	// that is not a scenario ending: the lobby's abandon, a disconnect, a
	// member walking out at a cell nobody authored as a way out. Empty is not
	// "unknown"; it is the truth that no authored exit was used.
	Exit string `json:"exit,omitempty"`
}

func (ExitedBody) isEventBody() {}

// LootedBody is EventLooted's typed body: who looted whom, and nothing of
// what moved. See EventLooted for the law this shape keeps.
type LootedBody struct {
	// Looter is who looted.
	Looter string `json:"looter"`

	// Body is the downed member whose body was looted — a member id, the
	// same vocabulary ExitedBody.Member and DownedBody.Member speak.
	Body string `json:"body"`
}

func (LootedBody) isEventBody() {}

// HeldBody is EventHeld's typed body: who picked up which prop.
type HeldBody struct {
	// Holder is who picked it up, and who has it until they carry it out
	// through a bound exit ([ExitedBody.Holding]) or drop it.
	Holder string `json:"holder"`

	// Prop is the placement id — the same id [AtlasProp.ID] carries.
	Prop string `json:"prop"`
}

func (HeldBody) isEventBody() {}

// DroppedBody is EventDropped's typed body: who dropped which prop, and where
// it landed.
type DroppedBody struct {
	// Member is the departing carrier who dropped it.
	Member string `json:"member"`

	// Prop is the placement id dropped.
	Prop string `json:"prop"`

	// At is the cell it landed on — where the carrier stood on the way out,
	// dungeon-absolute like every position on this seam.
	At spatial.Position `json:"at"`
}

func (DroppedBody) isEventBody() {}

// EndedBody is EventEnded's typed body: the declared ending that fired.
//
// The key is a content string ("boss-down", "withdrawn", "abandoned"), not
// an enum — what an ending MEANS is authored, and the client maps key to
// sentence (rpg-project#269 §6.3). The full outcome is Status's answer;
// this beat arrives on the WORLD clock, after the fight it may have grown
// out of has dissolved (ruling §6.6).
type EndedBody struct {
	Ending string `json:"ending"`
}

func (EndedBody) isEventBody() {}

// DoorBody is EventDoor's typed body: which door, what it is now, and — on
// an unlock attempt — the numbers behind it.
//
// DC, Total and Beaten are set only on attempt beats; a plain open or close
// carries DC zero. Actor is empty when the change has no author to narrate.
// The total is public down to the number — full data until v1.0.
type DoorBody struct {
	Door   string `json:"door"`
	State  string `json:"state"`
	Actor  string `json:"actor,omitempty"`
	DC     int    `json:"dc,omitempty"`
	Total  int    `json:"total,omitempty"`
	Beaten bool   `json:"beaten,omitempty"`
}

func (DoorBody) isEventBody() {}

// DoorRevealedBody is EventDoorRevealed's typed body: a concealed door as
// the recipient's own atlas and door list now carry it — the patch for both
// cached reads.
type DoorRevealedBody struct {
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
	Doorways []AtlasDoorway `json:"doorways,omitempty"`

	// Approaches is the lock's authored routes, present only while the
	// door is locked — what the recipient's Doors read would now list.
	Approaches []DoorApproach `json:"approaches,omitempty"`
}

func (DoorRevealedBody) isEventBody() {}

// RegionRevealedBody is EventRegionRevealed's typed body: the region's whole
// atlas slice, exactly as the recipient's own Atlas read now answers it —
// derived from that answer by the composition, so the patch and the map
// cannot disagree.
type RegionRevealedBody struct {
	// Region is the region's atlas entry: id, name, cells, archetype,
	// lighting.
	Region AtlasRegion `json:"region"`

	// Props is everything standing on the region's cells.
	Props []AtlasProp `json:"props,omitempty"`

	// Boundaries is every boundary touching the region's cells that the
	// recipient may now see — border walls included, still withholding any
	// shared with a hidden neighbour.
	Boundaries []AtlasBoundary `json:"boundaries,omitempty"`

	// Segments is the walls the recipient DID NOT HAVE AND NOW DOES: the ones
	// inside the room being revealed, which were withheld with it, and not the
	// border walls they could already see (rpg-toolkit#1480).
	//
	// A DIFFERENCE, not a slice of the room, because a segment carries no
	// footprint to ask which cells it stands on — see [AtlasSegment]. Apply
	// these to the cached atlas and its Segments is what Atlas would now
	// answer; the composition pins that agreement byte for byte.
	Segments []AtlasSegment `json:"segments,omitempty"`

	// Sealed is the cells of the revealed region nobody can stand on. A
	// recipient who has just been handed the room's cells still needs telling
	// which of them are not a place to put feet.
	//
	// APPLY IT AS A REPLACEMENT WITHIN THE ROOM, NOT AS AN ADDITION — this is
	// the one place the two new fields behave differently, and getting it
	// wrong leaves a room you can see and cannot walk into at its edges.
	// Cells LEAVE the sealed set on a reveal: the floor a presented wall
	// stands on reaches a non-knower as ownerless, which is floor nobody
	// stands on, and becomes ordinary standable floor the moment the room is
	// theirs. So for the cells in [RegionRevealedBody.Region], this list is
	// the whole answer and the cache's previous one is discarded; everything
	// outside the room is untouched. Segments, by contrast, are a pure
	// addition and nothing ever leaves.
	Sealed []spatial.Position `json:"sealed,omitempty"`
}

func (RegionRevealedBody) isEventBody() {}

// Door is one door's identity and live state — the dynamic half of
// [AtlasDoorway], which carries the same door's edges and never changes.
// Read via [Manager.Doors] once, then updated from EventDoor beats.
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
	// reaches a client as EventStanceChanged.
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
// guessing that giving Join and Spawn separate verbs removed.
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

	// ShortfallUnavailable is the ability's own precondition refusing: already
	// raging, already at full hit points. NOT a budget — nothing ran out,
	// Currency is empty, and waiting will not help the way it does for
	// [ShortfallNoBudget].
	//
	// This is the seam's word for what features.Feature.CanActivate refuses,
	// which is a different question from what the economy refuses. A projection
	// that collapsed the two would tell a raging barbarian to come back next
	// turn.
	ShortfallUnavailable ShortfallReason = "unavailable"
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

	// TargetPath is Move's selector shape: a walk along a path on the turn
	// clock. Move declarations carry no candidates; the path is chosen at
	// execution time and priced whole.
	TargetPath TargetKind = "path"
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
