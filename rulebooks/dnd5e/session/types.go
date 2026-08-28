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
// as a known location payload (encounter.DecodeLocationPayload succeeds). Nil for every
// other channel — that is the ordinary, expected case. Nil on a sight-channel
// holding whose payload fails to decode is NOT a legal state a caller should
// plan for; it means the composition wrote something projectSeen cannot
// read, which is a defect in that layer, not a case this seam is choosing to
// represent as "no sighting". A known memory (CurrentVia empty) keeps the Seen
// it last had — the last-known cell a client draws a faded marker on. An
// explicit unknown location has no Seen and is represented by
// Sighting.LocationState.
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
// TWO VALUES, AND NOT "DOWN". Downed is at zero hit points and out of the
// fight; a bare "down" also reads as PRONE, a posture condition on a member
// still acting (Kirk's ruling, rpg-toolkit#1084 — see ErrDowned).
type Standing string

const (
	// StandingUp is on their feet, in the fight.
	StandingUp Standing = "up"
	// StandingDowned is at zero hit points, out of the fight — still on the
	// map and in the roster.
	StandingDowned Standing = "downed"
)

// Sighting is one thing an observer currently perceives.
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
	// location testimony. It is set for sight-channel holdings only.
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

// LocationState is the encounter-authored state of location knowledge.
type LocationState string

const (
	LocationKnown   LocationState = "known"
	LocationUnknown LocationState = "unknown"
)

// IntelCorrection reports that an observer corrected a subject's location
// knowledge. It contains identifiers only and never exposes concealed truth.
type IntelCorrection struct {
	Observer string `json:"observer"`
	Subject  string `json:"subject"`
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

	// EventDowned reports that a member is DOWNED: at zero hit points, out of
	// the fight. The opposite state is UP.
	//
	// Downed rather than "down" because a bare "down" also reads as PRONE, and
	// prone is a different thing entirely — a posture condition the rulebook
	// tracks, still in the fight and still acting. A client narrating these two
	// the same way would say somebody died every time they were knocked flat.
	// Kirk's ruling, rpg-toolkit#1084.
	//
	// There is no event for coming back UP, and that absence is deliberate
	// rather than an omission: nothing in v1 can revive a downed member, so an
	// event for it would be vocabulary against a future nobody has built. It
	// earns its own name when death saves arrive.
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

	// EventDoor reports a door changing — opened, closed, or an unlock
	// attempt, beaten or not. A failed attempt is as much fiction as a
	// beaten one, and the composition writes both through one path
	// (rpg-project#268). Every member of the encounter hears it: door
	// knowledge is roster-wide until asymmetric perception
	// (rpg-toolkit#1020) narrows it.
	EventDoor EventKind = "door"

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

	// Seq is the story sequence this event was derived from: monotonic,
	// gapless, and never renumbered. A recipient that notices a gap in Seq has
	// missed an event and can re-query the story from its last known value.
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
// kind Event.Body carries: TurnEndedBody, DownedBody, StruckBody,
// MissedBody, FightStartedBody, FightEndedBody, MovedBody, JoinedBody,
// ExitedBody, EndedBody, DoorBody. Sealed the way
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

// DownedBody is EventDowned's typed body: who is at zero hit points and out
// of the fight. Who, and nothing else — see EventDowned's own doc for why
// hit points are not here.
type DownedBody struct {
	Member string `json:"member"`
}

func (DownedBody) isEventBody() {}

// DamageComponent is the replayable subset of one resolved damage component.
// Its order and values come from resolution; consumers display rather than
// recalculate it.
type DamageComponent struct {
	Source     string     `json:"source"`
	SourceRef  string     `json:"source_ref,omitempty"`
	Dice       string     `json:"dice,omitempty"`
	FinalRolls []int      `json:"final_rolls,omitempty"`
	FlatBonus  int        `json:"flat_bonus"`
	DamageType DamageType `json:"damage_type"`
	Multiplier *float64   `json:"multiplier,omitempty"`
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

// ExitedBody is EventExited's typed body: who left the encounter. See
// JoinedBody's own doc — the same reasoning, the opposite beat.
type ExitedBody struct {
	Member string `json:"member"`
}

func (ExitedBody) isEventBody() {}

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

// DoorLock is an authored lock, carried not interpreted — the DC is public
// down to the number (full data until v1.0), and what beats it is
// [Manager.Unlock]'s ruling, never a comparison the composition makes.
type DoorLock struct {
	// DC is the authored difficulty. No omitempty: zero would be an answer
	// if an author wrote one (TestFalseIsAnAnswerOnTheWire's law).
	DC int `json:"dc"`

	// Ability is the opaque rulebook ref the check uses, e.g. "dex".
	Ability string `json:"ability,omitempty"`

	// Tool is the opaque item ref a lock may name; empty when none.
	Tool string `json:"tool,omitempty"`
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
)

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
	// Attack, Move and EndTurn refuse as ErrNotYourTurn.
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
// closed set owned at the seam, mirroring the merged proto's TargetKind: the
// three currently executable verbs fix their kind, and a new kind arrives the
// day a proven executor for it lands — never in advance.
//
// FIXED FOR EVERY COMPILED OR BLOCKED DECLARATION: Attack -> TargetMember,
// Move -> TargetPath, EndTurn -> TargetNone. A blocker keeps the fixed kind
// even with empty candidates, so a client always knows which selector shape
// the verb carries.
type TargetKind string

const (
	// TargetNone is EndTurn's selector shape: no target. EndTurn is governed
	// solely by its clock/turn gate and carries no candidate.
	TargetNone TargetKind = "none"

	// TargetMember is Attack's selector shape: one member from the compiled
	// candidate universe. Every live CurrentVia-nonempty holding except the
	// actor appears exactly once in the declaration's Candidates.
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
