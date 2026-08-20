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
type GridKind string

const (
	// GridSquare is a square grid, where distance is Chebyshev.
	GridSquare GridKind = "square"

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
// ONE MAP. The composition underneath keeps rooms and projects the absolute
// geometry out of them (rpg-project#227); by the time a map reaches here the
// decomposition has done its job and is nobody else's business. What a client
// renders is a set of cells, the ones that block sight, the walls between
// cells, and the doorways — not a list of chambers with anchors and spans it
// would have to reassemble.
//
// Construction truth: unchanged by movement, joins, exits, or endings. Cache
// it per encounter rather than fetching it per frame.
//
// The INBOUND direction is deliberately different, and worth saying out loud
// so the asymmetry is not read as an oversight: StartSessionInput.World is
// authored content and still speaks rooms. Authoring is construction data,
// and the one-map rule governs what a session SEES while it plays.
type Atlas struct {
	// Grid is the coordinate family the whole map speaks. One value, not one
	// per room: a field has a single grid family by law (W1), so a per-room
	// grid was the same answer repeated.
	Grid GridKind `json:"grid"`

	// Layout is how to lay the cells out to draw them, present exactly when
	// Grid is hex and absent otherwise — the composition's own law (a hex
	// field must declare an orientation, a square field must not) mirrored at
	// the wire. Not a default: a square map that said pointy_top would be a
	// client believing something that cannot be true about its grid.
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

	// Doorways is every crossable cell pair, sorted by connection ID.
	Doorways []AtlasDoorway `json:"doorways,omitempty"`
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
}

// AtlasDoorway is one crossable pair of cells. The two are adjacent in
// absolute space, which is what makes crossing one an ordinary step rather
// than a jump between coordinate systems.
//
// It keeps an identifier and carries no room, because a doorway is a thing
// with identity — a door that can be closed or locked is a capability still
// ahead of this, and it will need to name one — while the rooms on either
// side are the composition's own decomposition.
type AtlasDoorway struct {
	// Connection is the doorway's identifier.
	Connection string `json:"connection"`

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

// Sighting is one thing an observer currently perceives.
type Sighting struct {
	// Subject names what is perceived.
	Subject string `json:"subject"`

	// Payload is what the observer knows about it, encoded by the composition.
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

// StoryEntry is one beat of what an observer has witnessed.
type StoryEntry struct {
	// Seq is the beat's sequence: monotonic, gapless, never renumbered. A
	// client that notices a gap has missed a beat and should re-query.
	Seq uint64 `json:"seq"`

	// At is the clock reading when the beat was recorded.
	At uint64 `json:"at,omitempty"`

	// Correlation groups cause and effect across beats. Empty is legal.
	Correlation string `json:"correlation,omitempty"`

	// Tags is queryable metadata describing the beat.
	Tags map[string]string `json:"tags,omitempty"`

	// Payload is the beat itself, encoded by the composition.
	Payload []byte `json:"payload,omitempty"`
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

	// Recipient is the member this projection is addressed to.
	Recipient string `json:"recipient"`

	// Kind names what happened.
	Kind EventKind `json:"kind"`

	// Payload is the kind-specific body, encoded by this package.
	Payload []byte `json:"payload,omitempty"`
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

	// Payload is what the observer learned, encoded by the composition.
	Payload []byte `json:"payload,omitempty"`
}
