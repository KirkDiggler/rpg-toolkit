// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dungeonspec compiles an authored dungeon into a world the
// composition can run (rpg-toolkit#1127, rpg-project#256).
//
// # Where this lives, and why it is here rather than in the host
//
// The yaml is content and the target is [encounter.FieldInput], so the compiler
// could plausibly sit on either side of that seam. It sits here because the
// toolkit owns geometry: rpg-api's own words for the arrangement it already has
// are that "rpg-api never computes dungeon geometry itself — a key maps to a
// builder, and only the toolkit turns that spec into an actual Space".
//
// # Version 2: the regions are the floor
//
// Version 1 described a dungeon as a chain of rectangular chambers and
// generated the seam walls and doorways between them. Version 2 — the dialect
// the dungeon builder writes (rpg-project#256) — describes the FLOOR directly:
// named regions painted cell by cell, walls and doors as edges between adjacent
// floor cells, everything in absolute [col,row] under one declared
// orientation. [Spec.Scenery] is the floor that belongs to no region
// (rpg-project#360): the same cells, painted with a different brush, that
// nobody stands on. Version 1 is deleted, not supported: a file that says
// `version: 1` is refused by name, and the reference tomb is re-authored in
// version 2 and compiles to the identical atlas (golden_test.go).
//
// # One gameplay grammar, two geometry dialects
//
// Two dialects are authored here, and they differ ONLY in how they describe
// space. The v2 document paints named regions cell by cell in absolute
// [col,row] under a declared orientation; the single-room document
// (single_room.go, the one the World Builder writes) draws one room in axial
// {q, r} and declares no orientation at all.
//
// Everything that is not space is ONE grammar, shared by both: the factions an
// author declares, the dispositions between them, the answer tables, the
// temperaments and the arms. Their types were always shared ([FactionSpec],
// [DispositionSpec], [AnswerSpec], [SelectorSpec], [TemperSpec]); since
// rpg-project#484 the FUNCTIONS are too — grammar.go holds the validators and
// [ordersOf] holds the compile, and each dialect calls them with its own paths.
//
// A dialect hands the grammar exactly two things, and they are exactly what a
// dialect owns:
//
//   - the CAST it placed ([members]) — who exists, what each one is, and which
//     side it is on;
//   - the FRAME it resolves a cell in ([cells]) — the v2 dialect looks an
//     offset pair up in the floor its regions painted, and the single room
//     looks an axial `{ q, r }` up in its walkable hexes; each refuses the
//     other's spelling by path.
//
// The single-room dialect used to reach the grammar by building a fake v2
// [Spec] and running the v2 validator over it. Nothing does that now, and a
// validator that needs a THIRD input from its dialect is a design change
// rather than another parameter.
//
// A DOOR IS THE SAME SPLIT, ONE NOUN DOWN (rpg-project#485). Its STATE —
// `closed`, and the `locked` check — is grammar: the same [CheckSpec] with
// the same nil-vs-empty law and the same sentences, judged once for both
// dialects ([grammar.doorState]) and compiled once ([doorStateOf]). Its
// GEOMETRY is the dialect's: a v2 door is a position on a wall, and a
// single-room door is the footprint its own `propDeclarations` entry draws,
// because that dialect has no walls to stand one in. So the single room
// declares three things about a placed item, each keyed by its id —
// `propDeclarations` (the shape), `doorBindings` (the door's state) and
// `monsterBindings` (a creature's orders) — and [encounter.DoorInput] takes
// one geometry or the other, never both.
//
// # Templates: stat blocks the author wrote (rpg-project#555)
//
// The single-room dialect may declare root `templates:` — named stat blocks,
// each a rulebook base (`dnd5e:monsters:human`) plus the scores, hit dice,
// armor, proficiency, skills and weapons the author overrides, and the
// experience it is worth, which never inherits ([TemplateSpec]). A placement names one exactly as it names any monster,
// `ref: dnd5e:monsters:guard`, so a template adds no member kind, no
// placement verb and no targeting rule.
//
// THE COMPILER CHECKS SHAPE AND CARRIES THE REST. Every refusal is about what
// a string IS — a base that is not a monster ref, armor that is not an armor
// ref, a weapon that is not a weapon ref, a score outside 1 to 30, hit dice
// that are not NdM, a base that names another template in the same file
// ("templates derive from the rulebook, not from each other"). The blocks
// come out on [Compiled.Templates] as the strings that went in. What a base
// resolves to, and whether a template id shadows a rulebook monster, are
// asked one layer up, for the reason the next section gives.
//
// # What it may not know
//
// This package compiles GEOMETRY and carries everything else. It resolves no
// content: "dnd5e:monsters:skeleton" comes out the far end as the same string
// that went in, and so does "lowest-health", and so does a region's
// `archetype`. The composition may not import a rulebook (design law C1), and
// a compiler that resolved refs here would drag one in. Refs become sheets one
// layer up, where a package may import both.
//
// So the output is deliberately in two halves: a [encounter.FieldInput] that is
// ready to hand to [encounter.NewEncounter] as it stands, and a roster of
// placements that still need somebody who knows what a skeleton is.
//
// # What it may not judge
//
// The World Builder's PRESENTATION (rpg-project#479, R3). The single-room
// dialect's document carries the authored scene — assets, labels, groups,
// parents, supports, height scales, point lights, the coordinate frame's axis
// words, the workspace's drawing limit — and this package carries it as the
// [gopkg.in/yaml.v3.Node] it was authored as, without modelling or judging
// any of it. It reads out of it exactly the values play depends on: the
// frame's hexRadius (which the placement adapter's scale is calibrated to),
// the workspace's hexRadius (which bounds the floor), the scene's name (which
// names the dungeon) and, per prop a `propDeclarations` entry names, the
// three transform numbers that place its footprint. single_room_lowering.go is
// the one reader and lists every one of them by path.
//
// The consequence is the point: an unknown key inside `scene` is the web
// codec's business, not this decoder's, and the editor's scalar bounds no
// longer have to change in two languages at once.
//
// # Validation is path-addressed
//
// [Validate] reports EVERY defect it finds, each naming the YAML path of the
// thing that is wrong (`regions[1].cells[0][3]`, `walls[3]`, `place[4].blocks_los`,
// `start`), because the builder draws each one on the canvas at the thing it
// names. A tool that only wants to know whether a file is a dungeon reads the
// length of the list.
package dungeonspec

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

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
// become a wall. `start`, `lighting` and `lighting.intensity` are pointers for
// the same reason: [0,0] and 0 are legal values an omission must not become.
type Spec struct {
	// Version is the dialect this file is written in. Exactly one is
	// understood — [Version] — and any other is refused by name rather than
	// read hopefully, version 1 included.
	Version int `yaml:"version"`

	// Key is the dungeon's identifier: `[a-z0-9-]`, the file's identity, and
	// the prefix of every door this compile mints so two dungeons in one
	// process cannot collide.
	Key string `yaml:"key"`

	// Name is the dungeon's display title. Carried, never read.
	Name string `yaml:"name"`

	// Orientation is which way the hexes point — "pointy" or "flat".
	// REQUIRED: every [col,row] pair in this file is an offset coordinate,
	// and an offset coordinate means nothing until the orientation is known.
	Orientation string `yaml:"orientation"`

	// Void is what the space between the regions does to a sightline —
	// "opaque" or "transparent", the words [encounter.Void] carries.
	// REQUIRED: [encounter.CanvasInput.Void] has no default by ruling.
	Void string `yaml:"void"`

	// Regions are the OWNED floor: their cells' union.
	Regions []RegionSpec `yaml:"regions"`

	// Scenery is floor nobody stands on, as rows of [col,row] pairs — the
	// same encoding and the same nesting-for-diff-readability as
	// [RegionSpec.Cells] (rpg-project#360, wall-geometry design §3.1).
	// Optional; omitted means none. Written after `regions` and before
	// `start`.
	//
	// A CELL BELONGS TO A REGION OR TO THIS LIST, NEVER BOTH. What the two
	// carry is different in kind: a region's cell has an OWNER, which decides
	// visibility, lighting and archetype, and feet may touch it. A scenery
	// cell has neither — it is floor a wall can stand on, a prop can sit on
	// and a sightline crosses, belonging to nobody and standable by nobody.
	// So a cell in both is not a cell with two answers, it is a cell whose
	// author meant two incompatible things, and [Validate] refuses it naming
	// the cell and the region.
	Scenery [][][2]int `yaml:"scenery,omitempty"`

	// Start is where the party comes in, and optionally which way they are
	// looking when they get there. REQUIRED and explicit — version 1 derived
	// it from an archetype, which is the shape of defaulting
	// rpg-toolkit#1033 forbids. See [StartSpec] for the two spellings.
	Start *StartSpec `yaml:"start"`

	// Walls are STRAIGHT LINES between two picked positions, each blocking
	// both movement and sight along everything it stands in the way of
	// (rpg-project#360). The envelope is implied, never written: a crossing
	// from floor into void is a crossing nobody can make, so a wall may run
	// along the edge of the world and cut nothing. See [WallSpec].
	Walls []WallSpec `yaml:"walls"`

	// Doors are one state each at one position on a wall (rpg-toolkit#1123,
	// rpg-project#360). See [DoorSpec].
	Doors []DoorSpec `yaml:"doors"`

	// Place is everything standing on the floor — props, monsters, and the
	// boss alike, routed by the ref's type segment — at ABSOLUTE cells.
	Place []PlaceSpec `yaml:"place"`

	// Intel is the knowledge this dungeon has authored, as records that can
	// be placed in a holder (rpg-project#372, design §2). Optional; omitted
	// means none.
	//
	// A RECORD IS A THING IN THE FILE, like a door. It has an id and it says
	// what it reveals, and it is placed by NAME on whatever carries it
	// ([PlaceSpec.Holds]) — the same shape a door has, declared once and
	// referred to from wherever it is used. That is what makes intel a
	// general builder capability rather than scenario machinery (R3): a
	// scenario binds the nouns its quest needs and never learns the word
	// intel, and a DM may author a record for anything at any time.
	//
	// This replaces `knows`, which named a door directly on a monster.
	// Kirk (R1): "if we don't have a use case for it then it goes. when a
	// use case does arrive for it it will define the shape without any
	// baggage." A record is one indirection and it is the one that buys
	// everything else — a second holder, a second kind of target, a body to
	// read — without a second spelling of knowledge.
	Intel []IntelSpec `yaml:"intel,omitempty"`

	// Factions are the sides this dungeon authored, and Dispositions how
	// they stand to each other (rpg-project#375, the hold-out design §2).
	// Both optional; omitted means the two reserved factions — `party` and
	// `monsters` — and the one default hostility every dungeon had before
	// factions existed. See [FactionSpec] and [DispositionSpec].
	Factions     []FactionSpec     `yaml:"factions,omitempty"`
	Dispositions []DispositionSpec `yaml:"dispositions,omitempty"`

	// Exits are the ways out of this dungeon: an id and a floor cell each
	// (rpg-project#368, design §3.1). Optional; omitted means none.
	//
	// STRUCTURE, NOT SCENARIO. A dungeon has ways out whatever the party is
	// there for, so an exit is authored beside `start` and the two have the
	// same shape. `start` is NOT implicitly an exit: nothing is defaulted
	// (rpg-toolkit#1033), and a dungeon whose entrance is also its way out
	// says so in one line.
	Exits []ExitSpec `yaml:"exits,omitempty"`

	// Scenarios binds this dungeon to the scenarios it is authored for: a
	// map from scenario id to that scenario's bindings, each binding a field
	// key to the id of something in this file (rpg-project#368, design §3.1).
	// Optional; omitted means none.
	//
	// CARRIED OPAQUELY, AND VALIDATED ONLY AS REFERENCES. This package checks
	// exactly one thing about a binding: that its value names a placement id
	// or an exit id that exists in this file. What the keys mean, which are
	// required, and whether the thing named is the right KIND of thing are
	// the scenario package's own refusals — asked at its `New(cfg)`, in
	// form-filler words. Design law C1 is the reason: this package never
	// resolves content, and a scenario is content.
	//
	// A dungeon may bind SEVERAL scenarios, and the run ends when any bound
	// ending fires (design R8).
	Scenarios map[string]map[string]string `yaml:"scenarios,omitempty"`

	// Endings are the ways this dungeon's run can end, authored in the file
	// itself (rpg-project#375, the hold-out design R10): an id and the
	// predicate that fires it. Optional; omitted means none of its own — the
	// scenarios it binds still declare theirs. Written after `exits` and
	// before `scenarios`.
	//
	// THE PREDICATE GRAMMAR, THIRD CONSUMER. `until` ends a hostility,
	// `arrives` brings a placement in, and `when` ends the run — one
	// spelling, one type ([PredicateSpec]), one evaluator in the engine. A
	// scenario's own field is sugar for one of these:
	// `scenarios: { hold-out: { convince: raiders } }` declares exactly the
	// ending `{ id: hold-out, when: { stance: { between: [raiders, party],
	// is: neutral } } }` would, and the pinning test says so. A scenario
	// package with nothing left to do is the north star's own test.
	Endings []EndingSpec `yaml:"endings,omitempty"`
}

// EndingSpec is one authored way the run ends: a name, and when.
//
//	endings:
//	  - { id: held-out, when: { round: 6 } }
//	  - { id: turned,   when: { stance: { between: [raiders, party], is: neutral } } }
//
// The id is the ending's key in the run — what the `ended` beat names —
// REQUIRED non-empty and unique among this file's endings. `when` is
// REQUIRED: an ending that does not say when it fires is one nothing can
// fire, and nothing is defaulted (rpg-toolkit#1033). Refused when it can
// never hold — "an ending nobody can reach" — by the same liveness rule every
// predicate in this file meets.
type EndingSpec struct {
	// ID names the ending. REQUIRED non-empty and unique.
	ID string `yaml:"id"`

	// When is the predicate that fires it. REQUIRED. See [PredicateSpec].
	When *PredicateSpec `yaml:"when"`
}

// IntelSpec is one authored piece of knowledge: an id, and what knowing it
// reveals.
//
//	intel:
//	  - id: vault-map
//	    reveals: { door: vault }
//
// The record is what a monster HOLDS ([PlaceSpec.Holds]) and what Loot moves
// off a body. What it reveals is read when it changes hands, not when it is
// placed, which is why the same record can grow a second kind of target
// without anything that carries it changing.
type IntelSpec struct {
	// ID names the record within this dungeon, and is what a holder names.
	// REQUIRED non-empty and unique — refused on collision naming both
	// lines, the same rule a door id and a placement id follow.
	ID string `yaml:"id"`

	// Reveals is what learning this record tells you. REQUIRED, and exactly
	// one target in this cut. See [RevealsSpec].
	Reveals RevealsSpec `yaml:"reveals"`
}

// RevealsSpec is what an intel record tells whoever learns it.
//
// ONE KEY PER USE CASE, and exactly one key set. Today the only one is a
// door, because the only use case anybody has is the way into the vault. A
// region, a location, a lock's approach, a camp's disposition — each is a
// field here, and each arrives WITH the use case that wants it and never
// ahead of it (R5). That is why this is a struct of optional targets rather
// than an open map: a target this build does not understand is a record
// nothing can apply, and it is refused rather than carried hopefully.
type RevealsSpec struct {
	// Door is the id of the door this record reveals the way to — THE v2
	// SPELLING, kept because it is this dialect's vocabulary for the thing
	// it draws. Refused when no door in this dungeon has that id, and
	// refused when no concealment holds that door: the engine's target is a
	// CONCEALMENT now (rpg-project#490, R7), so the lowering resolves the
	// door to the secret holding it and a door anyone can already see is a
	// record revealing nothing.
	//
	// REFUSED BY NAME in the single-room dialect, which has no crossing to
	// hide a door on — it writes `concealment:` instead ([RevealsSpec.Concealment]).
	Door string `yaml:"door,omitempty"`

	// Concealment is the id of the concealment this record gives away — the
	// SINGLE-ROOM dialect's spelling of the same target, because that
	// dialect declares its secrets at the root under `concealments:` and
	// names them directly (rpg-project#490, R7).
	//
	// Refused when no concealment in the document has that id, and refused
	// by name in the v2 dialect, whose secrets are spelled with the two
	// words that dialect already has.
	Concealment string `yaml:"concealment,omitempty"`

	// Fact is the id of the fact this record reveals (rpg-project#375, the
	// hold-out design §2) — the second key, arrived with its use case: a
	// letter that says the party saved the Wiseman.
	//
	// A PLAIN STRING, DECLARED BY MENTION. Nothing declares a fact; a
	// disposition's `until: { fact: x }` and a record's `reveals: { fact: x }`
	// simply agree on the word. A record may reveal a fact no disposition
	// waits for, and a disposition may wait for a fact no record reveals —
	// the dungeon allows both (pre-release: show the cost) and the SCENARIO
	// refuses the second, because a hold-out nobody can win is its business
	// (R8).
	Fact string `yaml:"fact,omitempty"`
}

// FactionSpec is one faction the dungeon authored: a name members belong
// to, and the member it knows through.
//
//	factions:
//	  - { id: raiders, mind: chief }
//
// `party` is never declared — it is the players' side, reserved — and
// `monsters` is where every monster with no `faction` already is. Declaring
// `monsters` is legal, and is how the unauthored side is given a mind.
type FactionSpec struct {
	// ID names the faction, and is what a placement's `faction`, a
	// disposition's `between` and a scenario's `convince` name. REQUIRED
	// non-empty and unique; `party` is refused by name.
	ID string `yaml:"id"`

	// Mind is the placement the faction knows through — "the faction knows
	// what its mind knows" (design R3). MUST name a MONSTER placement in
	// this faction. Optional: a faction of one has its member as its mind,
	// and a faction of many that waits for a fact and names no mind is
	// refused ("name a mind, or the faction cannot learn").
	Mind string `yaml:"mind,omitempty"`

	// Table names a root answer table ([SingleRoomSpec.Tables]) this side
	// answers with (rpg-toolkit#1897). Optional; absent means this faction's
	// orders are its own `on:` alone, exactly as they were before this field
	// existed.
	//
	// THE SAME MECHANISM AS A BINDING'S `table:`, ONE LEVEL UP. A faction's
	// `table:` is what its members' own `on:` layers over, and a member's own
	// `table:` layers over THAT. So a side may share one drill and a single
	// creature may still answer its own — [ordersOf] is unchanged and this
	// adds no rule to it, only another source of the same `On`.
	//
	//	factions:
	//	  - id: watch
	//	    table: watch-drill
	//
	// BOTH SPELLINGS MAY BE WRITTEN, and the NAMED TABLE WINS the keys it
	// carries: it is the more specific declaration, and an author who names a
	// table AND writes `on:` is stating a base plus an addition. That is the
	// same nearest-layer-wins-wholesale rule the rest of the stack uses.
	Table string `yaml:"table,omitempty"`

	// On is the author's ORDERS for every placement in this faction — the
	// same `on:` block a placement writes, inherited (design §1, layer 2).
	//
	//	factions:
	//	  - id: bandits
	//	    on:
	//	      time:
	//	        - { when: { enemy: none }, toward: { at: [3, 4] } }
	//
	// LAYERED, NEAREST KEY WINS WHOLESALE. A placement that writes its own
	// `time` key replaces this one's entirely — there is no merging of entry
	// lists, so an author never has to reason about what was added to what,
	// and the cost of overriding a key is visible in the file.
	On map[string][]AnswerSpec `yaml:"on,omitempty"`

	// Temper is the temperament every placement in this faction has, or a
	// MIX to deal one from per member ([TemperSpec], design §3). A placement
	// that names its own word wins, and the mix is not dealt for it.
	Temper TemperSpec `yaml:"temper,omitempty"`
}

// DispositionSpec is how two factions stand to each other, and what turns
// it.
//
//	dispositions:
//	  - { between: [raiders, party], stance: hostile, until: { fact: saved-wiseman } }
//	  - { between: [guards, party],  stance: neutral, until: { round: 12 } }
//
// One per unordered pair; a pair nobody declares has a default
// ([encounter.DefaultStance]): `party` is hostile to every faction that did
// not say otherwise, and every other pair is neutral.
type DispositionSpec struct {
	// Between is the pair, unordered. Both MUST exist: a declared faction,
	// or `party` or `monsters`.
	Between [2]string `yaml:"between"`

	// Stance is one of hostile, neutral, allied. REQUIRED.
	Stance string `yaml:"stance"`

	// Until is the predicate that TURNS the pair: when it holds the stance
	// becomes the OTHER of hostile and neutral (rpg-project#493, R1). A
	// hostile pair stands down; a neutral pair turns on you — "the guards
	// are civil until the third round of the brawl", "the wolves are calm
	// until you kill the alpha". REFUSED ON `stance: allied`: an allied pair
	// has nothing to become. Optional. See [PredicateSpec] — all four forms
	// are legal here, and only `{ fact }` needs the pair to have a mind,
	// because only a fact is somebody's knowledge.
	//
	// `until: { round: N }` COUNTS A FIGHT'S ROUNDS (hold-out R9): it holds
	// when any fight in the run has started round N, and outside any fight it
	// never holds at all. A party that never comes to blows with anyone walks
	// past these guards forever. There is no world clock to write against
	// yet, so "the guards turn at midnight" has no spelling here — an author
	// who means world time is asking for a predicate this build does not
	// have, not for this one.
	//
	// WHAT NO FILE WRITES is "if attacked, become hostile". Delivering hostile
	// intent across a neutral pair turns it hostile by law, authored or not
	// (R3 as R5 rebound it). What counts is the DELIVERY, not the door it came
	// through — a swing, a spell that asks for a save, or one that simply
	// lands its harm — so a camp is provokable the moment it is neutral, and
	// a delivery that is wholly a kindness provokes nobody.
	Until *PredicateSpec `yaml:"until,omitempty"`
}

// ExitSpec is one authored way out: an id and the floor cell a member stands
// on to leave through it.
//
//	exits:
//	  - { id: entrance, at: [1, 3] }
//
// The shape `start` already has, deliberately (design §3.1) — a way out is
// the same kind of authored fact as a way in.
type ExitSpec struct {
	// ID names the exit within this dungeon, and is what a scenario binding
	// names. REQUIRED non-empty and unique — a binding that named an
	// ambiguous exit would have no answer.
	ID string `yaml:"id"`

	// At is the absolute cell, offset [col,row]. Must be STANDABLE floor:
	// an exit nobody can reach is a liveness hole, which is
	// [encounter.ErrNoEnding]'s own reason applied one layer out.
	At [2]int `yaml:"at"`
}

// Version is the one dialect this build speaks.
//
// A single accepted value rather than a floor, deliberately: "version 2 or
// later is fine" is a promise about files nobody has written yet, and the
// standing precedent for a shape this build does not know is to refuse it by
// name rather than read it hopefully (rpg-toolkit#1053/#1068).
const Version = 2

// RegionSpec is one named set of cells with the per-area facts it carries.
type RegionSpec struct {
	// ID names the region, and survives the compile as the region's ID.
	ID string `yaml:"id"`

	// Name is the region's display name. Carried, never read.
	Name string `yaml:"name"`

	// Archetype is the presentation profile the assets resolve — "crypt" —
	// REQUIRED non-empty, carried unread, and NEVER a mechanic
	// (rpg-project#256 ruling; [encounter.RegionInput.Archetype]).
	Archetype string `yaml:"archetype"`

	// Lighting is the region's light level. REQUIRED, as is its intensity.
	Lighting *LightingSpec `yaml:"lighting"`

	// Cells is the region's floor as rows of [col,row] pairs. The nesting is
	// for diff-readability only — the builder writes one row per line so a
	// repaint diffs as a line change — and the compiler flattens it. A cell
	// in two regions, or twice in one, fails.
	Cells [][][2]int `yaml:"cells"`

	// Concealed marks the region as hidden space: the room that "appears to
	// be a wall unless it is found" (rpg-project#351 — the room hides with
	// its door). DECLARED HERE, NEVER CASCADED from a concealed door: the
	// room and its door are separate authored facts, moved together by
	// choice — but not moved apart: [Validate] refuses the incoherent
	// combinations (a room only enterable through concealed doors that is
	// not itself concealed, and a concealed room anyone can walk into).
	// Carried unread; what a non-knower's map withholds is the world
	// layer's business.
	Concealed bool `yaml:"concealed,omitempty"`
}

// LightingSpec is a region's lighting block: one field today, a block so later
// fields land beside it without reshaping the file.
type LightingSpec struct {
	// Intensity is the light level in [0,1]. REQUIRED — a pointer so a
	// written 0 is distinct from nothing written.
	Intensity *float64 `yaml:"intensity"`
}

// PositionSpec is ONE OF THE SEVEN POINTS a wall or a door may stand at: the
// midpoint of one of a hex's six sides, or its centre (design §3.3).
//
// A cell and an offset, because a point on a hex grid has no name of its own
// and the author already thinks in cells. The CELL names the frame; it need
// not be floor, and often is not — a wall along the outside of a room is
// written from the cells it walls. The OFFSET picks the point within it, in
// BOUNDING-BOX FRACTIONS: x in widths, east positive; y in heights, south
// positive, the same unit and the same axes a prop's offset uses.
//
// # The set is closed, and it compares exactly
//
// Seven values per orientation and nothing else (F8). Every one is a dyadic
// rational — halves, quarters and eighths — so the file's literals compare as
// exact floats and recognising a position needs no tolerance. An offset
// outside the set is refused naming the wall and the value, never snapped to
// the nearest member: a wall a hand's breadth from where its author put it is
// a dungeon they did not draw.
//
// # The same point, named twice, is a corner
//
// Two positions that name the same physical point are the same point, whether
// or not they name it from the same cell. That is the whole of what a corner
// is (F5): the designer writes a join by writing the same point at both ends,
// and this package has no corner concept and needs none.
type PositionSpec struct {
	// Cell is the hex the point is named from, as an absolute offset
	// [col,row] pair. REQUIRED.
	Cell [2]int `yaml:"cell"`

	// Offset is the point within that cell, in bounding-box fractions.
	// REQUIRED, and one of the seven (§3.3). Written even for the centre —
	// `[0,0]` is a real position and an author who meant it says so, which
	// is what keeps a forgotten key from quietly becoming a thick wall.
	Offset [2]float64 `yaml:"offset"`
}

// StartSpec is where the party comes in: a cell, and optionally the direction
// they are facing when they arrive.
//
// # Two spellings, one meaning
//
// The bare pair is the whole of what a start was before facings existed, and
// it stays legal forever:
//
//	start: [4, 7]
//	start: { at: [4, 7], facing: e }
//
// A dungeon that says nothing about facing is not a dungeon facing north — it
// is a dungeon whose author did not say, and the empty string is that fact
// (zero values tell the truth). Every reference fixture keeps the bare pair,
// and the golden files are byte-unchanged by this addition.
//
// Both spellings are read by hand rather than by a struct tag because a
// scalar and a mapping cannot both satisfy one Go type through the decoder,
// and because a custom unmarshaler bypasses KnownFields — so an unknown key
// has to be refused here, by name, exactly as [PositionSpec] does one type
// over and for that type's stated reason.
type StartSpec struct {
	// At is the absolute [col,row] cell the party arrives on. REQUIRED in
	// both spellings, and it must be floor nobody is standing on — the
	// refusals are unchanged by facing (see validate.go).
	At [2]int

	// Facing is the direction the party is looking on arrival, one of the
	// eight true-compass names props already speak — n|ne|e|se|s|sw|w|nw
	// (rpg-project#272). Empty means the author stated none.
	//
	// It says which way a camera opens, and — lowered at compile to
	// [Compiled.StartFacingDegrees] — which way a host seats the party
	// facing (rpg-project#582, design R13). A word outside the eight is
	// refused BY NAME rather than snapped to the nearest — the vocabulary
	// rule every authored word in this dialect follows.
	Facing string
}

// UnmarshalYAML reads a start in either spelling — the bare cell pair, or the
// object with an explicit `at` and an optional `facing`.
//
// An unknown key is refused by name for [PositionSpec.UnmarshalYAML]'s
// reason: a custom unmarshaler anywhere above this in the tree bypasses the
// decoder's KnownFields, so a typo would be silently dropped by the very
// strictness Decode exists to provide.
func (s *StartSpec) UnmarshalYAML(value *yaml.Node) error {
	// The bare pair, and the ONLY spelling that existed before facings. A
	// sequence here is unambiguous: an object start is a mapping.
	if value.Kind == yaml.SequenceNode {
		var at [2]int
		if err := value.Decode(&at); err != nil {
			return err
		}
		s.At, s.Facing = at, ""
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf(
			"line %d: a start is [col,row] or { at: [col,row], facing: n|ne|e|se|s|sw|w|nw }",
			value.Line)
	}

	var sawAt bool
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "at":
			sawAt = true
		case "facing":
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.StartSpec",
				value.Content[i].Line, key)
		}
	}
	if !sawAt {
		return fmt.Errorf("line %d: a start does not say which cell the party arrives on", value.Line)
	}

	var obj struct {
		At     [2]int `yaml:"at"`
		Facing string `yaml:"facing"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	s.At, s.Facing = obj.At, obj.Facing
	return nil
}

// UnmarshalYAML reads a position, refusing an unknown key and a missing one by
// name. Written by hand because a custom unmarshaler anywhere above this in
// the tree bypasses the decoder's KnownFields, and a typo silently dropped is
// exactly what Decode's strictness exists to prevent.
func (p *PositionSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a position is { cell: [col,row], offset: [x,y] }", value.Line)
	}
	var sawCell, sawOffset bool
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "cell":
			sawCell = true
		case "offset":
			sawOffset = true
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.PositionSpec",
				value.Content[i].Line, key)
		}
	}
	if !sawCell {
		return fmt.Errorf("line %d: a position does not say which cell it is named from", value.Line)
	}
	if !sawOffset {
		return fmt.Errorf(
			"line %d: a position does not say its offset within the cell (the centre is [0,0], written out)",
			value.Line)
	}
	var obj struct {
		Cell   [2]int     `yaml:"cell"`
		Offset [2]float64 `yaml:"offset"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	p.Cell, p.Offset = obj.Cell, obj.Offset

	return nil
}

// WallSpec is one authored wall: A STRAIGHT LINE BETWEEN TWO POSITIONS, and
// the file holds nothing else (design §1.5, F3).
//
//	walls:
//	  - start: { cell: [1, 2],  offset: [-0.25, -0.375] }
//	    end:   { cell: [1, 10], offset: [-0.25,  0.375] }
//	    height: 2
//	    name: the long seam
//
// The crossings it blocks, the cells it passes through and which of those it
// leaves too little of to stand on are ALL DERIVED (§4.2) and never written.
// Kirk: "edges just need the starting and ending hex coordinate… not only be
// easy to read it would be easy to edit."
//
// # The pair form is deleted, not deprecated
//
// The old dialect wrote a wall as the list of crossings it blocked —
// `[[5,0],[6,0]]`, `{between: …}`, `{edges: […]}` — which made a wall a
// bookkeeping exercise in what a line already says, and made a wall standing
// against the void unsayable. A file that uses it is refused by name (F4),
// with no migration: legacy dungeons are deleted and re-authored (C17).
//
// # Thin and thick are not in here
//
// A line along a row of side midpoints shaves a twenty-fourth off its
// neighbours; the one through their centres seals them. Those are COSTS, shown
// by the designer at pick time, not kinds of wall (F16a). This struct knows
// seven positions and twelve directions, and the compiler knows the area rule.
// There is no lower-level decision to relax.
type WallSpec struct {
	// Start and End are the wall's two ends. REQUIRED, both.
	Start PositionSpec `yaml:"start"`
	End   PositionSpec `yaml:"end"`

	// Height is the authored wall-height MULTIPLIER of the standard rendered
	// wall height, REQUIRED in [1, 3] when authored — raise-only by ruling
	// (rpg-project#273: "I am looking to raise the walls not lower them"), so
	// a waist-high wall cannot be authored at all. Nil means not authored:
	// the standard height, exactly what writing 1.0 means. VISUAL ONLY: a
	// wall blocks movement and sight identically at every height.
	Height *float64 `yaml:"height"`

	// Name is the wall's display name, for the human reading the file and the
	// errors about it: "north wall" beats "walls[7]" for the streamers this
	// dialect is authored by. Carried, and read by nothing but a refusal.
	Name string `yaml:"name"`
}

// UnmarshalYAML reads a wall, refusing the deleted pair form by name.
//
// Unknown keys are refused HERE because a custom unmarshaler bypasses the
// decoder's KnownFields, and a typo silently dropped is exactly what Decode's
// strictness exists to prevent. `edges` and `between` are called out
// separately from any other unknown key: a file carrying them is not a typo,
// it is last dialect's dungeon, and saying so is the difference between an
// author who knows what to do and one who does not.
func (w *WallSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		return fmt.Errorf(
			"line %d: %s", value.Line, pairFormRefusal)
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf(
			"line %d: a wall is { start: {cell, offset}, end: {cell, offset} }, with an optional height and name",
			value.Line)
	}
	var sawStart, sawEnd bool
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "edges", "between":
			return fmt.Errorf("line %d: %s", value.Content[i].Line, pairFormRefusal)
		case "start":
			sawStart = true
		case "end":
			sawEnd = true
		case "height", "name":
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.WallSpec",
				value.Content[i].Line, key)
		}
	}
	if !sawStart || !sawEnd {
		return fmt.Errorf("line %d: a wall runs from `start` to `end`, and this one does not say both", value.Line)
	}
	var obj struct {
		Start  PositionSpec `yaml:"start"`
		End    PositionSpec `yaml:"end"`
		Height *float64     `yaml:"height"`
		Name   string       `yaml:"name"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	w.Start, w.End, w.Height, w.Name = obj.Start, obj.End, obj.Height, obj.Name

	return nil
}

// pairFormRefusal is the one sentence a last-dialect file gets, wherever it is
// met (F4, F12). It names the form, says what replaced it, and stops — there
// is no migration to offer.
const pairFormRefusal = "`edges` is the deleted pair form: a wall is now a line, " +
	"`start` and `end`, each a cell and one of the seven offsets, and a door is `at` one position on it"

// DoorSpec is one door: A POSITION ON A WALL, plus the state it is in.
//
//	doors:
//	  - id: crypt-door
//	    at: { cell: [1, 6], offset: [-0.25, 0.375] }
//	    closed: true
//
// The door opens the ONE crossing of the side it is the midpoint of (F11), and
// exactly one wall must pass through it (F10). A wider doorway is two doors.
type DoorSpec struct {
	// ID names the door within this dungeon; the compiled door is
	// `<key>/<id>`.
	ID string `yaml:"id"`

	// At is where the door stands: one position, which must be a SIDE
	// MIDPOINT rather than a centre — a door in the middle of a hex opens no
	// crossing — and must have exactly one wall through it. REQUIRED.
	At PositionSpec `yaml:"at"`

	// Locked, when present, makes the door locked behind the check it
	// carries. Nil with Closed false is an open doorway.
	//
	// NIL, NOT LEN 0, IS "NOT LOCKED" — [PlaceSpec.Offset]'s law: yaml.v3
	// leaves this nil when the key is absent but decodes `locked: []` to a
	// non-nil, zero-length list, and the second is an authored lock that
	// forgot to say how it is beaten — refused at validate rather than
	// silently read as an open doorway.
	Locked CheckSpec `yaml:"locked,omitempty"`

	// Closed makes the door shut but not locked. Ignored when Locked is
	// set — a locked door is shut by definition.
	Closed bool `yaml:"closed,omitempty"`

	// Concealed, when present, hides the door behind the find check it
	// carries — e.g. Perception 15 or Investigation 12. It COMPOSES with
	// plain, closed, or locked underneath: what a door is doing and whether
	// anyone knows it is there are two separate authored facts
	// (rpg-project#350). Nil vs empty is Locked's law: `concealed: []` is a
	// door hidden with no way to ever find it, refused at validate by name.
	Concealed CheckSpec `yaml:"concealed,omitempty"`

	// Opened is the fact this door teaches when it becomes open — the single
	// room's [RoomDoorBinding.Opened], the same shape and the same grammar
	// (rpg-project#569 R12). Nil teaches nothing.
	Opened *OpenedSpec `yaml:"opened,omitempty"`

	// openedNull and openedFactNotString are what the hand-read decode saw
	// and the typed value cannot say: `opened: null` decodes to an absent
	// door fact and `fact: 12` to the fact "12", each without a word. The
	// single room judges both on the node (single_room_doors.go); v2 notes
	// them here so [validation.doors] refuses them at the same paths, in the
	// same sentences (rpg-project#569 R12).
	openedNull, openedFactNotString bool
}

// OpenedSpec is what a door teaches when it becomes open (rpg-project#569
// R12): `opened: { fact: <id> }`, one shape in both dialects.
//
//	doorBindings:
//	  cell-door:
//	    locked: [{ ability: dex, tool: dnd5e:item:thieves-tools, dc: 13 }]
//	    opened: { fact: cells-opened }
//
// A DOOR IS A SOURCE, the way a record's `reveals` is: when it opens — by
// Open, or by a beaten lock, picked or forced — the actor and every member
// who can see the door learn the fact. Hearers are not taught; a door nobody
// saw open teaches nobody. Nothing reads a door's state as a predicate: an
// ending or an until waits on the fact with `{ fact: <id> }`.
type OpenedSpec struct {
	// Fact is the fact id the door teaches. REQUIRED non-empty — the key was
	// written, so the author meant to teach something. A fact id is a plain
	// string declared by mention ([PredicateSpec.Fact]), carried verbatim.
	Fact string `yaml:"fact"`
}

// UnmarshalYAML reads a door, refusing the deleted pair form by name (F12) and
// any unknown key, for [WallSpec.UnmarshalYAML]'s reason.
func (d *DoorSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a door is { id, at: {cell, offset} } with an optional state", value.Line)
	}
	var sawAt bool
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "edges", "between":
			return fmt.Errorf("line %d: %s", value.Content[i].Line, pairFormRefusal)
		case "at":
			sawAt = true
		case "id", "locked", "closed", "concealed":
		case "opened":
			// The hand-read door decodes its body without `KnownFields`, so
			// the one mapping it holds is checked for its keys here, in
			// yaml.v3's words, which the unknown-key translation turns into
			// a path and the list OpenedSpec takes.
			body := value.Content[i+1]
			d.openedNull = body.Tag == "!!null"
			if body.Kind == yaml.MappingNode {
				for j := 0; j < len(body.Content); j += 2 {
					k, v := body.Content[j].Value, body.Content[j+1]
					if k != "fact" {
						return fmt.Errorf("line %d: field %s not found in type dungeonspec.OpenedSpec",
							body.Content[j].Line, k)
					}
					d.openedFactNotString = v.Kind == yaml.ScalarNode && v.Tag != "!!str" && v.Tag != "!!null"
				}
			}
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.DoorSpec",
				value.Content[i].Line, key)
		}
	}
	if !sawAt {
		return fmt.Errorf("line %d: a door does not say where it stands (`at`, one position on a wall)", value.Line)
	}
	var obj struct {
		ID        string       `yaml:"id"`
		At        PositionSpec `yaml:"at"`
		Locked    CheckSpec    `yaml:"locked"`
		Closed    bool         `yaml:"closed"`
		Concealed CheckSpec    `yaml:"concealed"`
		Opened    *OpenedSpec  `yaml:"opened"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	d.ID, d.At, d.Locked, d.Closed, d.Concealed = obj.ID, obj.At, obj.Locked, obj.Closed, obj.Concealed
	d.Opened = obj.Opened

	return nil
}

// knowsRefusal is the deleted `knows` key's sentence, pointing at what
// replaced it (R1). The same shape [pairFormRefusal] takes for the deleted
// wall pair form: a key this build once understood is refused BY NAME with
// the migration in the message, never as a bare unknown field, because the
// author who wrote it had a meaning and deserves to be told where it went.
const knowsRefusal = "`knows` is gone: a monster holds an intel RECORD now — declare it under " +
	"`intel:` with an id and what it reveals, then put its id in this placement's `holds:`"

// UnmarshalYAML reads a placement, refusing the deleted `knows` key by name
// (R1) and every unknown key, for [WallSpec.UnmarshalYAML]'s reason.
func (pl *PlaceSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a placement is { ref, at } plus what it declares", value.Line)
	}
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "knows":
			return fmt.Errorf("line %d: %s", value.Content[i].Line, knowsRefusal)
		case "id", "ref", "at", "blocks_movement", "blocks_los", "facing",
			"offset", "targeting", "actions", "boss", "holds", "holdable", "faction",
			"arrives", "options", "on", "temper":
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.PlaceSpec",
				value.Content[i].Line, key)
		}
	}
	// A plain alias so decoding the body does not re-enter this method.
	type placeBody PlaceSpec
	var obj placeBody
	if err := value.Decode(&obj); err != nil {
		return err
	}
	*pl = PlaceSpec(obj)

	return nil
}

// CheckSpec is one authored check: the accepted approaches through it, AT
// LEAST ONE, success by any listed one (ruled on rpg-project#350 — a locked
// door is forced with Strength or picked with Dexterity and tools; a
// concealed door is spotted with Perception or reasoned out with
// Investigation). The whole list is carried to the composition opaquely, for
// the reason the single-approach lock was: "does a DEX check of 12 succeed"
// is a rule, and this package never learns what "perception" means either.
//
// A bare list rather than a wrapping object, deliberately: the builder
// authors a check as approach rows, and the YAML reads as the rows it is.
//
// This is the generalized [DoorSpec] lock shape — LockSpec's single
// dc/ability/tool became one entry in this list, in place, free under the
// pin system pre-adoption.
type CheckSpec []ApproachSpec

// ApproachSpec is one accepted route through a check: an ability or skill,
// maybe a tool, and the DC that route must beat. Every field is carried
// uninterpreted; the DC is priced PER APPROACH, not per check — forcing the
// door and picking its lock need not cost the same.
type ApproachSpec struct {
	// Ability is the opaque rulebook ref this approach rolls — "str",
	// "dex", "perception", "investigation". REQUIRED non-empty, and never
	// inspected here.
	Ability string `yaml:"ability"`

	// Tool is the opaque item ref for a named tool, e.g.
	// "dnd5e:item:thieves-tools". Optional: empty means the approach names
	// none — the reference tomb's lock does not.
	Tool string `yaml:"tool,omitempty"`

	// DC is what this route must beat. REQUIRED at least 1 — a check with
	// dc 0 is what an undeclared one would look like.
	DC int `yaml:"dc"`
}

// The one key [PlaceSpec.On] accepts, spelled as the composition spells it
// ([encounter.TableKeys]) so the file, the validator and the run cannot
// disagree about what a trigger is called.
//
// THE FOUR SOCIAL KEYS RETIRED TO OPTIONS (rpg-project#569 R2): what a
// creature answers when spoken to lives on the [OptionSpec] that was spoken,
// under its own `success` and `failure`, and an `on:` naming one of the old
// four is refused by name with where it went.
const (
	// OnTime is what the creature does when it has time: its turn in a
	// fight, or a round of the world (rpg-project#465).
	//
	// A PLAIN STRING, not [encounter.AnswerKey]. The keys of an `on:` block
	// are YAML map keys and this package decodes them as written; the
	// composition's typed key is what they compile TO.
	OnTime = string(encounter.AnswerTime)
)

// OptionSpec is one line a character can say to a creature or a prop
// (rpg-project#569, the mad king use case §2): its id, the line, the check it
// rolls, what it waits on, and what is answered on either verdict.
//
//	options:
//	  - id: troupe
//	    say: "We're the troupe for tonight's feast."
//	    requires: { fact: feast-tonight }
//	    check: [{ ability: performance, dc: 13 }, { ability: deception, dc: 15 }]
//	    success: [{ say: "Through the side gate, then." }]
//	    failure: [{ say: "Troupe? There's no— GUARDS!", fact: party-recognized }]
//
// # Flat, and gated by facts (R1)
//
// An option is not a node in a tree: every option on a binding is offered
// whenever its `requires` holds for the character speaking (R9 — their own
// knowledge and their own pockets), and what the world learns from one line
// is what opens the next.
//
// # The answer lives on the option (R2)
//
// `success` and `failure` are the same weighted rows a table carries
// ([AnswerSpec]), rolled by the world's die on the verdict the check landed.
// Only the words a verdict can answer with are legal — `fact`, `flee` and a
// line — and a `when` is refused, because the verdict IS the condition.
//
// # The deed is the skill (R3)
//
// A beaten check lands the deed of the SKILL it rolled on every witness:
// Intimidation lands `intimidated`, Persuasion lands `persuaded`, every other
// skill lands nothing. The line's words decide nothing.
type OptionSpec struct {
	// ID names the line on its binding. REQUIRED non-empty and unique among
	// one binding's options.
	ID string `yaml:"id" json:"id"`

	// Say is the character's line, carried VERBATIM. REQUIRED non-empty.
	Say string `yaml:"say" json:"say"`

	// Check is the approaches through the line's check — [CheckSpec], the
	// list a lock carries. REQUIRED, at least one.
	Check CheckSpec `yaml:"check" json:"check"`

	// Requires is what must hold for the speaker before the line is offered:
	// a [PredicateSpec] of exactly one of two forms, `{ fact: <id> }` (the
	// speaker knows it) or `{ holds: <prop id> }` (the speaker carries a
	// placed prop somebody can pick up). Optional; absent is always offered.
	Requires *PredicateSpec `yaml:"requires,omitempty" json:"requires,omitempty"`

	// Success is the rows answered when the check was beaten. Optional;
	// absent answers nothing. Written but empty is refused.
	Success []AnswerSpec `yaml:"success,omitempty" json:"success,omitempty"`

	// Failure is the rows answered when the check was missed — the half that
	// makes trying worse than not trying. Optional, with Success's rules.
	Failure []AnswerSpec `yaml:"failure,omitempty" json:"failure,omitempty"`
}

// optionKeys is every key an [OptionSpec] reads, for its hand-written
// unknown-key refusal.
var optionKeys = map[string]bool{
	"id": true, "say": true, "check": true, "requires": true, "success": true, "failure": true,
}

// UnmarshalYAML reads one option, refusing an unknown key by name — for
// [PredicateSpec.UnmarshalYAML]'s reason: an option is read inside
// [PlaceSpec]'s own unmarshaler, which the decoder's KnownFields does not
// reach, and a typo silently dropped is exactly what Decode's strictness
// exists to prevent. A `requirs:` dropped would be a line offered to
// everybody that the author meant to gate.
//
// THE CHECK'S APPROACHES ARE READ BY HAND HERE TOO, because reading the option
// by hand is what puts them out of KnownFields' reach in EITHER dialect. They
// are refused in yaml.v3's own words under [ApproachSpec]'s name, so the
// refusal is rewritten at its path with the keys an approach takes, exactly as
// a lock's approach typo is.
func (o *OptionSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: an option is { id, say, check } plus what it requires and answers", value.Line)
	}
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if !optionKeys[key] {
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.OptionSpec",
				value.Content[i].Line, key)
		}
		if key != "check" || i+1 >= len(value.Content) || value.Content[i+1].Kind != yaml.SequenceNode {
			continue
		}
		for _, approach := range value.Content[i+1].Content {
			if approach.Kind != yaml.MappingNode {
				continue
			}
			// THE KEYS ARE [ApproachSpec]'S OWN, read off its yaml tags by the
			// package's one reflection ([authoredKeysOfType]) — never a second
			// spelling of the struct that could drift from it.
			known := authoredKeysOfType("dungeonspec.ApproachSpec")
			for j := 0; j < len(approach.Content); j += 2 {
				if !slices.Contains(known, approach.Content[j].Value) {
					return fmt.Errorf("line %d: field %s not found in type dungeonspec.ApproachSpec",
						approach.Content[j].Line, approach.Content[j].Value)
				}
			}
		}
	}
	type optionBody OptionSpec
	var obj optionBody
	if err := value.Decode(&obj); err != nil {
		return err
	}
	*o = OptionSpec(obj)

	return nil
}

// laterWords are the outcome words this design NAMES and this build does not
// land, refused BY NAME with where they went (rpg-project#458).
//
// THE SAME COURTESY THE DELETED `knows` KEY GETS ([knowsRefusal]). An author
// who wrote `alarm:` meant something real — it is slice three of the
// shenanigans folder — and a dungeon that accepted the line would silently
// never fire it. Refusing as a bare unknown field would tell them the word
// does not exist, which is false and sends them looking for a typo.
var laterWords = map[string]string{
	"alarm":   "designed as the Alarm slice and not built yet",
	"lure":    "designed and not built yet",
	"pretend": "designed as the Insight slice and not built yet",
	"tell":    "not a word: a fact taught to whoever was there is `fact`",
}

// patrolRefusal is the sentence `patrol` is refused with (rpg-project#569 R7).
// The word was named before it was built, and what was built is a route on
// `toward`, so the refusal says what to write rather than that it is missing
// — and what to write is a route in the reading dialect's own spelling
// ([spelledRefusal]).
func patrolRefusal(f cellForms) string {
	return "`patrol` is not a word: a patrol is a route, write toward: { path: " + f.route + " }"
}

// AnswerSpec is ONE ENTRY in an outcome's table: how likely it is, what the
// creature says, and the one thing it does.
//
// # One entry fires, and weights are relative
//
// The engine sums the table's weights and rolls one die of that size, so an
// author may write 3 and 1 or 75 and 25 and mean the same thing. An omitted
// weight is 1, which makes a table of entries that all omit it an even split
// and a single entry the certainty it looks like.
//
// # Exactly one word, or none
//
// `fact` or `flee`, never both — two words in one entry is refused at
// validation so an author never has to guess ordering. Wanting two things to
// happen is not expressible today and two entries do not express it either; a
// later slice may add a list under one entry when a use case pays for one.
//
// An entry with NO word is legal only when it has a line to say: a creature
// that answers and does nothing is a real outcome, and one that neither speaks
// nor acts is a row written for no reason.
type AnswerSpec struct {
	// Weight is this entry's relative share. OMITTED MEANS 1, which is why
	// this is a pointer: `weight: 0` is a row that can never fire, and an
	// int could not tell it apart from a row that named no weight at all.
	// Refused below 1.
	Weight *int `yaml:"weight,omitempty"`

	// Say is the creature's line, carried VERBATIM onto the beat. The engine
	// never composes it. Omitted means the creature says nothing.
	//
	// "This is also a place where the dungeon author can put text the goblin
	// would say for each outcome" (Kirk).
	Say string `yaml:"say,omitempty"`

	// Fact is the authored fact id every witness learns when this entry
	// fires — what the old `on: { intimidated: { fact: … } }` said, now one
	// entry of one outcome's table. A disposition's `until: { fact: … }`
	// does the rest.
	//
	// ON A `time` ROW IT IS AN ALARM (rpg-project#569 R6): the creature says
	// it aloud with its own time, and everyone who sees it AND everyone who
	// hears its voice through the doors learns it.
	//
	// CARRIED VERBATIM, not key-prefixed — a fact is a word a disposition
	// waits for by name, like a faction rather than a door. The dungeon
	// ALLOWS a fact nothing else mentions (R8, pre-release: show the cost),
	// so this is not checked against the dispositions.
	//
	// A POINTER for [AnswerSpec.Weight]'s reason: `fact: ""` is an author
	// who wrote the key and did not finish the sentence, and a plain string
	// could not tell that apart from an entry that named no fact at all. The
	// two get different defects because they are different mistakes.
	Fact *string `yaml:"fact,omitempty"`

	// Flee lands a `fled` deed on the creature, naming whoever made it run —
	// `flee: {}`.
	//
	// IT MOVES NOBODY. The running is the creature's own answer on its own
	// time: the rulebook's default table carries a
	// `{ when: { fled: { within: 3 } }, away: actor, weight: 3 }` row, so a
	// creature that has been made to run spends its next rounds walking away
	// from the one who did it, weighted heavily enough to beat the rest of the
	// table. An author who wants a different flight writes a different row
	// rather than a different word here, and a creature whose table has no
	// `fled` row does not run at all — which is the author's to decide, and
	// visible to them, in a way a one-shot walk hidden behind this word was
	// not.
	//
	// AN EMPTY MAPPING rather than a bare word because a word with no body has
	// nowhere to grow: the day fleeing takes a span of its own,
	// `flee: { within: … }` is an addition to this file and not a break in
	// every file that has one.
	Flee *FleeSpec `yaml:"flee,omitempty"`

	// When is the condition this entry is on the table under
	// (rpg-project#465, design §2). `time` ONLY: a social key IS the
	// condition — `intimidated` already means "the threat landed" — and a
	// second one under it would be an author asking when a thing that just
	// happened happened.
	//
	//	on:
	//	  time:
	//	    - { when: { enemy: reach },              attack: enemy }
	//	    - { when: { enemy: seen },               toward: enemy }
	//	    - { when: { attacked: { within: 3 } },   attack: attacker, weight: 3 }
	//
	// An entry whose condition does not hold is NOT ON THE TABLE for that
	// roll — absent rather than weighted zero, so the beat's candidate list
	// is the honest account of what the creature could have done.
	//
	// A LIST, AND EVERY ENTRY MUST HOLD (rpg-project#569 R8). One condition
	// may still be written as a single mapping, which reads as a list of one:
	//
	//	- { when: { enemy: seen }, toward: enemy }
	//	- { when: [{ fact: guards-turned }, { enemy: none }], toward: { at: [20, 12] } }
	When WhenListSpec `yaml:"when,omitempty"`

	// Hold is `hold: {}` — the creature does nothing with its time. `time`
	// only. An EMPTY MAPPING for [FleeSpec]'s reason: a word with no body has
	// nowhere to grow.
	//
	//	on:
	//	  time:
	//	    - { hold: {} }
	Hold *HoldSpec `yaml:"hold,omitempty"`

	// Attack strikes the selected member. `time` only, and REFUSED off the
	// turn clock at the table: an enemy in reach on the world clock is a
	// fight sight already formed.
	//
	//	on:
	//	  time:
	//	    - { attack: attacker }
	Attack *SelectorSpec `yaml:"attack,omitempty"`

	// Toward walks toward the selected member's believed position, or an
	// authored cell, for the turn's movement. `time` only, and the ONLY word
	// `at:` is legal on — walking away from a fixed cell is a direction
	// rather than a flight, and nothing has paid for one.
	//
	//	on:
	//	  time:
	//	    - { when: { enemy: none }, toward: { at: [3, 4] } }
	Toward *SelectorSpec `yaml:"toward,omitempty"`

	// Dash RUNS at the selected member's believed position, an authored
	// cell, a route or a sound, spending the turn on the run
	// (rpg-project#671, R17): `toward`'s selectors, twice the movement, and
	// no swing that turn. `time` only. A dash at a foe already in reach is
	// not on the table; every other threshold is the row's own `when`:
	//
	//	on:
	//	  time:
	//	    - { when: [{ enemy: dash }, { threat: none }], dash: enemy }
	Dash *SelectorSpec `yaml:"dash,omitempty"`

	// Away walks away from the selected member for the turn's movement —
	// the coward's run. `time` only.
	//
	//	on:
	//	  time:
	//	    - { when: { fled: { within: 3 } }, away: actor, weight: 5 }
	Away *SelectorSpec `yaml:"away,omitempty"`

	// Search walks on from a LOST enemy's cell along a heading, then gives up
	// (rpg-project#668 R4, R5). `time` only, and eligible only under `enemy:
	// lost`. `distance` is cells and defaults to [DefaultSearchDistance];
	// `heading` is went, faced, left or right and defaults to
	// [DefaultSearchHeading] (rpg-project#679 R10); `search: {}` is the
	// default search. A guessing guard writes a left row and a right row.
	//
	//	on:
	//	  time:
	//	    - { when: { enemy: remembered }, toward: enemy }
	//	    - { when: { enemy: lost },       search: { distance: 6, heading: went } }
	Search *SearchSpec `yaml:"search,omitempty"`

	// Use makes this row a GROUP: its answer is the named bundle's rows,
	// rolled when the group is picked (rpg-project#670, R12). The bundle is
	// declared once under the root `bundles:` ([SingleRoomSpec.Bundles]) and
	// used by name wherever a table wants that behaviour:
	//
	//	time:
	//	  - use: pursuer
	//	    with: { distance: 8 }
	//	    weight: 2
	//	    as: toward
	//
	// A `use:` row takes `weight`, an optional `when` (a gate on the whole
	// group), `as` and `with`, and NOTHING ELSE a row can carry: it has no
	// word, no `search`, no `say` and no `fact` of its own — its rows do.
	Use *string `yaml:"use,omitempty"`

	// With is the bundle's parameters for this use, by name: each replaces
	// every scalar `$name` in the bundle's rows, whole (a route can be a
	// parameter). A name the bundle does not declare is refused, and so is a
	// declared one with no default that this use does not give.
	With map[string]yaml.Node `yaml:"with,omitempty"`

	// As is the temperament word a group counts as (rpg-project#670 R13):
	// one of attack, toward, away, flee or hold, or omitted for none, which
	// loads the group at 100.
	As *string `yaml:"as,omitempty"`

	// Rows, From and Params are a `use:` row RESOLVED: the bundle's rows with
	// the parameters substituted, the bundle's name, and every parameter's
	// value, defaults included, as written. NOT AUTHORED: filled by
	// [resolveBundles] before the grammar or the compiler reads the row, and
	// never marshalled back, so a re-encoded file still says `use:`.
	Rows   []AnswerSpec      `yaml:"-" json:"-"`
	From   string            `yaml:"-" json:"-"`
	Params map[string]string `yaml:"-" json:"-"`

	// Line is the file line this entry was written on, captured at decode so
	// the validator can point an author at it.
	//
	// NOT AUTHORED, and `yaml:"-"` says so: the decoder fills it. The
	// validator's own [FieldError] speaks in paths (`place[2].on.time[0]`),
	// which names the entry but not the line an editor jumps to, and every
	// refusal this slice adds is about a line somebody wrote.
	Line int `yaml:"-"`
}

// FleeSpec is `flee: {}` — the word, with no options yet. It exists as a type
// rather than a bool so the key is written the way every other outcome word
// is, and so the first option it grows is a field here.
type FleeSpec struct{}

// HoldSpec is `hold: {}` — the word, with no options. [FleeSpec]'s twin under
// `time`, and written the same way for the same reason.
type HoldSpec struct{}

// DefaultSearchDistance is how many cells a `search` with no `distance` walks
// on from the lost cell (rpg-project#668 R4).
const DefaultSearchDistance = 6

// DefaultSearchHeading is the heading a `search` with no `heading` follows:
// the way the quarry went (rpg-project#679 R10).
const DefaultSearchHeading = encounter.HeadingWent

// SearchSpec is `search: { distance: N, heading: <word> }`. A pointer
// distance so an omitted one is told from a written zero: omitted is
// [DefaultSearchDistance], and a written 0 is refused at the form. An omitted
// heading is [DefaultSearchHeading]; a word outside went, faced, left and
// right is refused at the form.
type SearchSpec struct {
	Distance *int    `yaml:"distance,omitempty"`
	Heading  *string `yaml:"heading,omitempty"`
}

// UnmarshalYAML reads `search: { distance, heading }`, refusing every other
// key by name. Refused BY HAND because a body decoded inside [AnswerSpec]'s
// own unmarshaler is not strict, and a misspelt `distance` silently read as
// the default search would be an author's six quietly ignored.
func (r *SearchSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: `search` is { distance: N, heading: went }, or {} for %d cells the way they went",
			value.Line, DefaultSearchDistance)
	}
	for i := 0; i < len(value.Content); i += 2 {
		if key := value.Content[i].Value; key != "distance" && key != "heading" {
			return fmt.Errorf("line %d: %q is not a key `search` reads: it reads distance and heading",
				value.Content[i].Line, key)
		}
	}
	type searchBody SearchSpec
	var body searchBody
	if err := value.Decode(&body); err != nil {
		return err
	}
	*r = SearchSpec(body)

	return nil
}

// WhenListSpec is a row's `when`: every condition in it must hold
// (rpg-project#569 R8). Written as a list, or as one mapping that reads as a
// list of one, so every file authored before the list existed still reads.
type WhenListSpec []WhenSpec

// UnmarshalYAML reads a `when` as one condition or a list of them, refusing an
// empty list: a row with no conditions is a standing order, and the way to
// write one is to leave `when` out.
func (l *WhenListSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.SequenceNode {
		var one WhenSpec
		if err := value.Decode(&one); err != nil {
			return err
		}
		*l = WhenListSpec{one}

		return nil
	}
	if len(value.Content) == 0 {
		return fmt.Errorf("line %d: a `when` with no condition in it is on the table always — omit it", value.Line)
	}
	out := make(WhenListSpec, 0, len(value.Content))
	for _, item := range value.Content {
		var one WhenSpec
		if err := item.Decode(&one); err != nil {
			return err
		}
		out = append(out, one)
	}
	*l = out

	return nil
}

// deed is the first condition in the list that names a deed, or nil — the
// deed `actor` names the actor of. A deed inside a `not` is not one: it
// names something that did not happen, and nobody was its actor.
func (l WhenListSpec) deed() *WhenSpec {
	for i := range l {
		if l[i].Deed != "" {
			return &l[i]
		}
	}

	return nil
}

// WhenSpec is ONE condition on what the creature holds: EXACTLY ONE of an
// enemy state, a deed within a span, a sound heard within a span, a fact
// held, or the negative of another condition (rpg-project#465 design §2;
// rpg-project#569 R4, R6, R8).
//
//	when: { enemy: reach }
//	when: { enemy: seen }
//	when: { enemy: remembered }
//	when: { enemy: none }
//	when: { attacked: { within: 3 } }
//	when: { heard: { kind: clang, within: 2 } }
//	when: { fact: alarm-raised }
//	when: { not: { enemy: seen } }
//
// TWO KEYS IN ONE MAPPING IS REFUSED, not read as `and`. The `and` is the
// list ([WhenListSpec]): an author who wants both writes two entries, and the
// refusal says so.
type WhenSpec struct {
	// Enemy is one of the four BANDS — `reach`, `seen`, `remembered`, `none`
	// — or empty when this condition names a deed instead.
	//
	// EXCLUSIVE BY DEFINITION: exactly one holds at any moment, so an author
	// writes one entry per band and knows which fires. `seen` is "in sight
	// and NOT in reach", which is the gap `toward` is for; `reach` is where a
	// swing belongs, because a swing at somebody out of reach is a pass.
	Enemy string

	// Deed is the deed kind — `attacked`, `intimidated`, `persuaded`,
	// `fled` — or empty when this condition names an enemy instead.
	Deed string

	// Within is how many rounds ago the deed may have landed and still
	// count. AT LEAST 1: a span is counted from 1.
	Within int

	// Scope says WHOSE deed this condition is about — empty for the creature
	// itself (the reading that existed before scopes), `ally` for one of its
	// own side, `actor` for a deed the creature did. It refines the deed
	// form rather than adding a second condition, so a `when` still names
	// exactly one thing (rpg-toolkit#1883, rpg-project#498).
	//
	// ALLY IS A RELATIONSHIP, NOT A FACTION. Which members are "its own side"
	// is the stance graph's answer at evaluation time — so a table keeps
	// working when a disposition changes, and a document never names a
	// faction as an ally.
	//
	// ACTOR IS THE PAUSE: `{ attacked: { within: 2, as: actor } }` is "I
	// struck recently", which is the fact "after I strike, stand still"
	// needs.
	Scope encounter.DeedScope

	// Heard is `heard: { kind, within }`: a sound of one of the sealed kinds
	// ([encounter.SoundKinds]) the creature holds, no older than the span.
	// Nil when this condition names something else.
	Heard *HeardSpec

	// Fact is `fact: <id>`: the creature holds that fact. Empty when this
	// condition names something else. CARRIED VERBATIM, [AnswerSpec.Fact]'s
	// rule: a fact nothing teaches is a cost, not a defect.
	Fact string

	// Not is `not: { … }`: holds when the one condition inside it does not.
	Not *WhenSpec

	// Threat is `threat: none | walk | dash` (rpg-project#671, R18): who
	// else could reach the cell this row's move lands on, and swing, on
	// their next turn. Empty when this condition names something else. One
	// of [behavior.ThreatWords], refused at decode otherwise.
	Threat string

	// Line is the file line, captured at decode for [AnswerSpec.Line]'s
	// reason.
	Line int
}

// HeardSpec is the body of a `heard` condition: which kind, and how recently.
type HeardSpec struct {
	// Kind is the sound's kind, one of [encounter.SoundKinds].
	Kind string

	// Within is how many rounds ago the sound may have been made and still
	// count. AT LEAST 1, the deed condition's rule.
	Within int
}

// heardBody is what a `heard` condition's mapping decodes into, pointers so
// an omitted key can be told from a zero.
type heardBody struct {
	Kind   *string `yaml:"kind"`
	Within *int    `yaml:"within"`
}

// The keys a `when` entry may name besides a deed word, sealed so the
// refusals and the decode below cannot drift apart.
const (
	whenEnemy  = "enemy"
	whenHeard  = "heard"
	whenFact   = "fact"
	whenNot    = "not"
	whenThreat = "threat"
)

// The three keys a deed condition's body may carry, sealed so the hand-read
// refusal above and the decode below cannot drift apart.
const (
	whenBodyWithin = "within"
	whenBodyOn     = "on"
	whenBodyAs     = "as"
)

// withinSpec is the body a deed condition takes: `{ within: N }`, and — since
// rpg-toolkit#1883 — optionally WHOSE deed it is about.
//
// THE TWO SPELLINGS CANNOT BOTH BE GIVEN: `on: ally` reads a blow to my side,
// `as: actor` reads what I did. Two at once is an author asking a question this
// grammar has not paid for, refused by name rather than silently preferring
// one.
type withinSpec struct {
	Within *int    `yaml:"within"`
	On     *string `yaml:"on"`
	As     *string `yaml:"as"`
}

// UnmarshalYAML reads one condition, refusing two conditions in one entry,
// an unknown key by name, a span counted from zero, a `heard` with no kind or
// a kind no deed here makes, and an empty `fact`.
func (w *WhenSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode || len(value.Content) == 0 {
		return fmt.Errorf("line %d: a `when` is one of { enemy: %s }, { <deed>: { within: N } }, "+
			"{ heard: { kind, within } }, { fact: <id> }, { threat: %s } or { not: { … } }",
			value.Line, strings.Join(enemyWords(), " | "), strings.Join(threatWords(), " | "))
	}
	if len(value.Content) > 2 {
		return fmt.Errorf("line %d: a `when` entry is one condition, and this names %d: "+
			"give each its own entry in a list, `when: [{ … }, { … }]`, and every entry must hold",
			value.Line, len(value.Content)/2)
	}

	key := value.Content[0].Value
	body := value.Content[1]
	w.Line = value.Line

	switch key {
	case whenHeard:
		heard, err := decodeHeard(body)
		if err != nil {
			return err
		}
		w.Heard = heard

		return nil
	case whenFact:
		if body.Kind != yaml.ScalarNode || body.Value == "" {
			return fmt.Errorf("line %d: `fact` in a `when` names the fact the creature must hold, and this names none",
				body.Line)
		}
		w.Fact = body.Value

		return nil
	case whenThreat:
		// A SEALED WORD, REFUSED IN THIS DIALECT'S VOICE: what the word means
		// is mind/behavior's, which words this build reads is said here.
		if body.Kind != yaml.ScalarNode || !knownThreatWord(body.Value) {
			return fmt.Errorf("line %d: `threat: %s` is not a threat this build reads: they are %s "+
				"(who else could reach where this row lands: nobody, a walker, or only a dasher)",
				body.Line, body.Value, strings.Join(threatWords(), ", "))
		}
		w.Threat = body.Value

		return nil
	case whenNot:
		var inner WhenSpec
		if err := body.Decode(&inner); err != nil {
			return err
		}
		w.Not = &inner

		return nil
	}

	if key == whenEnemy {
		word := body.Value
		if !knownEnemyWord(word) {
			return fmt.Errorf("line %d: `enemy: %s` is not a condition this build reads: they are %s",
				body.Line, word, strings.Join(enemyWords(), ", "))
		}
		w.Enemy = word

		return nil
	}

	if !knownWhenDeed(key) {
		return fmt.Errorf("line %d: `%s` is not a deed this build holds: they are %s (and `enemy`, `heard`, `fact`, `threat`, `not`)",
			value.Content[0].Line, key, strings.Join(encounter.WhenDeeds, ", "))
	}
	// UNKNOWN KEYS ARE REFUSED BY HAND, because Decode's strictness does not
	// reach inside a custom unmarshaler (rpg-toolkit#1890 thread 3). Before
	// scopes a dropped key in this body did nothing; now the dropped key
	// decides WHOSE deeds the row reads — `no: ally` would leave the condition
	// on the self reading, firing for the wrong wound and never for its own.
	// [PredicateSpec.UnmarshalYAML] refuses its own keys for this same reason.
	if body.Kind == yaml.MappingNode {
		for i := 0; i < len(body.Content); i += 2 {
			switch bodyKey := body.Content[i].Value; bodyKey {
			case whenBodyWithin, whenBodyOn, whenBodyAs:
			default:
				return fmt.Errorf("line %d: field %s not found in type dungeonspec.withinSpec",
					body.Content[i].Line, bodyKey)
			}
		}
	}

	var within withinSpec
	if err := body.Decode(&within); err != nil {
		return fmt.Errorf("line %d: `%s` takes { within: N }: %w", body.Line, key, err)
	}
	if within.Within == nil {
		return fmt.Errorf("line %d: `%s` names no span — write { within: N }", body.Line, key)
	}
	if *within.Within < 1 {
		return fmt.Errorf("line %d: a span of %d rounds is counted from 1", body.Line, *within.Within)
	}
	scope, err := scopeOf(within, body.Line, key)
	if err != nil {
		return err
	}
	w.Deed = key
	w.Within = *within.Within
	w.Scope = scope

	return nil
}

// decodeHeard reads a `heard` body, refusing an unknown key and a kind no
// deed here makes — the sealed vocabulary, which is the rulebook's. A missing
// kind or a span below 1 is mind/behavior's shape rule and is refused on the
// form in its words ([grammar.entryShape]), not restated here.
func decodeHeard(body *yaml.Node) (*HeardSpec, error) {
	if body.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: `heard` takes { kind, within: N }", body.Line)
	}
	for i := 0; i < len(body.Content); i += 2 {
		switch k := body.Content[i].Value; k {
		case "kind", "within":
		default:
			return nil, fmt.Errorf("line %d: field %s not found in type dungeonspec.heardBody",
				body.Content[i].Line, k)
		}
	}
	var heard heardBody
	if err := body.Decode(&heard); err != nil {
		return nil, fmt.Errorf("line %d: `heard` takes { kind, within: N }: %w", body.Line, err)
	}
	out := &HeardSpec{}
	if heard.Kind != nil {
		out.Kind = *heard.Kind
	}
	if heard.Within != nil {
		out.Within = *heard.Within
	}
	// A PATTERN IS JUDGED AGAINST THE SOUNDS THIS FILE CAN HEAR
	// (rpg-project#684 L3): a pattern that matches none of them is a
	// condition that can never hold, refused rather than accepted as "never
	// fires". Whether it compiles is mind/behavior's sentence, asked on the
	// form ([grammar.entryShape]).
	if pattern, ok := patternOf(out.Kind); ok {
		if pattern == "" {
			return nil, fmt.Errorf("line %d: `heard: { kind: // }` is a pattern between empty slashes, "+
				"and names no sound", body.Line)
		}
		if _, err := regexp.Compile(pattern); err == nil && !anySoundMatches(nameOf(out.Kind)) {
			return nil, fmt.Errorf("line %d: `heard` kind %s matches no sound this file makes: they are %s",
				body.Line, out.Kind, strings.Join(encounter.SoundKinds, ", "))
		}

		return out, nil
	}
	if out.Kind != "" && !knownSoundKind(out.Kind) {
		return nil, fmt.Errorf("line %d: `heard: { kind: %s }` is not a sound this build makes: they are %s",
			body.Line, out.Kind, strings.Join(encounter.SoundKinds, ", "))
	}

	return out, nil
}

// anySoundMatches reports whether a `heard` kind's pattern matches at least
// one of [encounter.SoundKinds]: every sound a file can make, since any
// creature in it can swing, speak, force a door or cast.
func anySoundMatches(kind behavior.Name) bool {
	for _, k := range encounter.SoundKinds {
		if kind.Matches(k) {
			return true
		}
	}

	return false
}

// knownSoundKind reports whether a kind is one of [encounter.SoundKinds].
func knownSoundKind(kind string) bool {
	for _, k := range encounter.SoundKinds {
		if k == kind {
			return true
		}
	}

	return false
}

// scopeOf reads WHOSE deed a condition is about off its body — `on: ally`,
// `as: actor`, or neither for the creature itself.
//
// THE TWO SPELLINGS ARE ONE ANSWER, so giving both is refused rather than
// silently preferring one: `on: ally` and `as: actor` are different questions
// ("was my side hit" vs "did I act"), and an author who wrote both has made a
// mistake this grammar cannot resolve for them.
//
// AN UNKNOWN WORD IS REFUSED BY NAME, in the same shape every other refusal in
// this dialect takes — the word means something, it is simply not a scope this
// build reads.
func scopeOf(in withinSpec, line int, deed string) (encounter.DeedScope, error) {
	if in.On != nil && in.As != nil {
		return "", fmt.Errorf(
			"line %d: `%s` names both `on: %s` and `as: %s`, and a condition asks one thing: "+
				"`on: ally` is a deed against your side, `as: actor` is one you did",
			line, deed, *in.On, *in.As)
	}
	if in.On != nil {
		if *in.On != string(encounter.ScopeAlly) {
			return "", fmt.Errorf(
				"line %d: `on: %s` is not a scope this build reads: they are %s "+
					"(and omitting it means the creature itself)",
				line, *in.On, scopeWords())
		}

		return encounter.ScopeAlly, nil
	}
	if in.As != nil {
		if *in.As != string(encounter.ScopeActor) {
			return "", fmt.Errorf(
				"line %d: `as: %s` is not a scope this build reads: they are %s "+
					"(and omitting it means the creature itself)",
				line, *in.As, scopeWords())
		}

		return encounter.ScopeActor, nil
	}

	return encounter.ScopeSelf, nil
}

// scopeWords names the scopes a refusal lists: the two named readings, since
// the default is what omitting the field means and saying so twice would read
// as a third option.
func scopeWords() string {
	named := make([]string, 0, len(encounter.DeedScopes))
	for _, scope := range encounter.DeedScopes {
		if scope != encounter.ScopeSelf {
			named = append(named, string(scope))
		}
	}

	return strings.Join(named, ", ")
}

// SelectorSpec names what a `attack`/`toward`/`away` entry acts on: a word,
// an authored cell, or an authored route.
//
//	attack: enemy
//	toward: sound
//	toward: { at: [3, 4] }                                   # v2: an offset
//	toward: { path: [[2, 3], [6, 3], [6, 8], [2, 8]] }       # v2: offsets
//	toward: { at: { q: 1, r: -1 } }                          # single room: axial
//	toward: { path: [{ q: -2, r: 0 }, { q: 0, r: -2 }, …] }  # single room: axial
//	toward: { path: [{ q: -2, r: 0, wait: 2, facing: s }, …] } # a pause
//
// A SCALAR OR A MAPPING, and the author writes whichever says what they mean.
// The words are sealed in mind/behavior ([encounter.SelectorWords]); `at:` and
// `path:` are legal on `toward` alone, and `sound` is refused on `attack`, by
// the validator with the reason.
//
// A CELL IS SPELT IN ITS DIALECT'S FRAME, and the spelling says which frame
// (rpg-project#605, D1a). The v2 document's cells are offset pairs resolved
// through its `orientation`; the single room's are the axial `{ q, r }` every
// one of its walkable hexes and starting cells is written in. Both spellings
// decode here, into their own fields, because the decoder does not know which
// dialect it is reading — and the DIALECT refuses the other one's spelling by
// path, through its [cells]. The same bytes therefore never mean two cells:
// `[4, 2]` is only ever an offset and `{ q: 4, r: 2 }` only ever axial.
type SelectorSpec struct {
	// Word is `enemy`, `attacker`, `actor` or `sound`, or empty when a cell
	// or a route is written.
	Word string

	// At is the authored cell, offset [col,row] — the v2 spelling — or nil.
	// It must be floor, the same rule a placement's own `at` keeps.
	At *[2]int

	// Path is an authored route of cells, offset [col,row] — the v2 spelling —
	// walked in order and round again: a patrol (rpg-project#569 R7). Each
	// must be floor. THE ROUTE HOLDS NO STATE: which waypoint is next is
	// derived from where the creature stands each time the row fires.
	Path [][2]int

	// AtHex is the authored cell in the single room's own axial frame, `{ q,
	// r }`, or nil. It must be one of the room's walkable hexes.
	AtHex *RoomCell

	// PathHex is an authored route in the single room's axial frame, each
	// waypoint `{ q, r }` and each one walkable. A route that mixes the two
	// spellings fills both this and [SelectorSpec.Path], and whichever
	// dialect reads it refuses the spelling that is not its own.
	PathHex []RoomCell

	// PathHexFacing is, index for index with [SelectorSpec.PathHex], the
	// compass word a waypoint states for the PAUSE there — `{ q, r, facing:
	// s }` — or "" where it states none (rpg-project#624). Nil when no
	// waypoint states one, so an unfaced route decodes as it always did. The
	// word is the placement's own vocabulary, refused in the placement's own
	// sentence ([facings]), and lowered through the same
	// [encounter.FacingFromCompass].
	//
	// TAKEN WHILE STANDING (Kirk, 2026-10-19), so only beside a wait
	// ([SelectorSpec.PathHexWait]; a facing alone is refused): the walker
	// faces it while it stands out the wait; walking, it faces its step
	// (sneaking design R17–R20); a walk that ends mid-leg faces its last
	// step.
	//
	// The v2 offset pair `[col, row]` has no key to carry one: a faced
	// waypoint is spelt in the axial mapping.
	PathHexFacing []string

	// PathHexWait is, index for index with [SelectorSpec.PathHex], how many
	// rounds the walker STANDS on a waypoint before it walks on — `{ q, r,
	// wait: 2 }` — or 0 where it states none (rpg-project#624; Kirk,
	// 2026-10-19: "the patrol walks to an area, stands for N turns, then
	// goes on"). Nil when no waypoint states one. Axial waypoints only, for
	// [SelectorSpec.PathHexFacing]'s reason.
	PathHexWait []int

	// Line is the file line, captured at decode for [AnswerSpec.Line]'s
	// reason.
	Line int
}

// cellForms is how a dialect spells a cell and a route, for the hints its
// refusals give. Each frame names its own ([floorCells.forms],
// [singleRoomCells.forms]).
type cellForms struct {
	cell, route string
}

// spelledRefusal is a DECODE refusal whose hint names a cell spelling. The
// decoder cannot tell the dialects apart — the selector and answer types are
// shared — so the refusal carries its sentence unspelt, and the dialect that
// read the file spells it with its own frame's forms ([decodeErrorsIn]). Its
// Error is the v2 spelling, word for word what v2 authors have always read.
type spelledRefusal struct {
	say func(cellForms) string
}

// Error is the refusal in the v2 dialect's spelling.
func (e spelledRefusal) Error() string { return e.say(floorCells{}.forms()) }

// selectorForms is the forms a selector takes, for the decode refusals.
func selectorForms(f cellForms) string {
	return "or { at: " + f.cell + " }, or { path: " + f.route + " }"
}

// UnmarshalYAML reads one selector, refusing a word outside the sealed list
// and a mapping that names anything but `at` or `path`. How many targets a
// selector names — both, or a route with no waypoints — is mind/behavior's
// shape rule, refused on the form in its words ([grammar.entryShape]); which
// SPELLING of a cell is legal is the dialect's ([cells]).
func (sel *SelectorSpec) UnmarshalYAML(value *yaml.Node) error {
	sel.Line = value.Line

	if value.Kind == yaml.ScalarNode {
		if !knownSelectorWord(value.Value) {
			line, word := value.Line, value.Value

			return spelledRefusal{say: func(f cellForms) string {
				return fmt.Sprintf("line %d: %q is not a selector this build resolves: they are %s, %s",
					line, word, strings.Join(selectorWords(), ", "), selectorForms(f))
			}}
		}
		sel.Word = value.Value

		return nil
	}

	if value.Kind != yaml.MappingNode {
		line := value.Line

		return spelledRefusal{say: func(f cellForms) string {
			return fmt.Sprintf("line %d: a selector is one of %s, %s",
				line, strings.Join(selectorWords(), ", "), selectorForms(f))
		}}
	}
	// A REPEATED KEY IS REFUSED, in the parser's own words: yaml.v3 refuses
	// one when it decodes a mapping into a struct, and this mapping is read
	// by hand, so the check is spelt out — last-one-wins would be a selector
	// the author wrote twice and got once.
	seen := map[string]int{}
	for i := 0; i+1 < len(value.Content); i += 2 {
		body := value.Content[i+1]
		if err := repeatedKey(seen, value.Content[i]); err != nil {
			return err
		}
		switch value.Content[i].Value {
		case "at":
			if err := sel.decodeAt(body); err != nil {
				return err
			}
		case "path":
			if err := sel.decodePath(body); err != nil {
				return err
			}
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.SelectorSpec",
				value.Content[i].Line, value.Content[i].Value)
		}
	}

	return nil
}

// decodeAt reads `at`: an offset pair into [SelectorSpec.At], an axial
// mapping into [SelectorSpec.AtHex].
func (sel *SelectorSpec) decodeAt(body *yaml.Node) error {
	if resolveNode(body) != nil && resolveNode(body).Kind == yaml.MappingNode {
		cell, err := decodeHexCell(resolveNode(body))
		if err != nil {
			return err
		}
		sel.AtHex = &cell

		return nil
	}
	var at *[2]int
	if err := body.Decode(&at); err != nil {
		return err
	}
	sel.At = at

	return nil
}

// decodePath reads `path`, each waypoint into the field its spelling names.
// A route of offset pairs decodes exactly as it always did — an empty list
// included, which mind/behavior refuses as a route with no waypoints.
func (sel *SelectorSpec) decodePath(body *yaml.Node) error {
	seq := resolveNode(body)
	if seq == nil || seq.Kind != yaml.SequenceNode {
		var path *[][2]int
		if err := body.Decode(&path); err != nil {
			return err
		}
		if path != nil {
			sel.Path = *path
		}

		return nil
	}
	offsets := [][2]int{}
	faced, waited := false, false
	var words []string
	var waits []int
	for _, waypoint := range seq.Content {
		if w := resolveNode(waypoint); w != nil && w.Kind == yaml.MappingNode {
			cell, pause, err := decodeWaypoint(w)
			if err != nil {
				return err
			}
			sel.PathHex = append(sel.PathHex, cell)
			words = append(words, pause.facing)
			waits = append(waits, pause.wait)
			faced = faced || pause.facing != ""
			waited = waited || pause.wait != 0

			continue
		}
		var pair [2]int
		if err := waypoint.Decode(&pair); err != nil {
			return err
		}
		offsets = append(offsets, pair)
	}
	if len(offsets) > 0 || len(sel.PathHex) == 0 {
		sel.Path = offsets
	}
	if faced {
		sel.PathHexFacing = words
	}
	if waited {
		sel.PathHexWait = waits
	}

	return nil
}

// waypointPause is what one axial waypoint states for the pause there, as
// written: the facing word, or "", and the rounds to stand, or 0.
type waypointPause struct {
	facing string
	wait   int
}

// decodeWaypoint reads one axial waypoint of a route: the strict `{ q, r }`
// [decodeHexCell] reads, and the optional `facing` word and `wait` count for
// the pause there (rpg-project#624). The word is judged by the dialect beside
// a placement's ([facings]) and the count's sign by the dialect too; here
// the word must only be a word and the count a whole number.
func decodeWaypoint(node *yaml.Node) (RoomCell, waypointPause, error) {
	cell := &yaml.Node{Kind: yaml.MappingNode, Tag: node.Tag, Line: node.Line, Column: node.Column}
	var pause waypointPause
	seen := map[string]int{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, val := node.Content[i], node.Content[i+1]
		if key.Value != "facing" && key.Value != "wait" {
			cell.Content = append(cell.Content, key, val)

			continue
		}
		if err := repeatedKey(seen, key); err != nil {
			return RoomCell{}, waypointPause{}, err
		}
		v := resolveNode(val)
		if key.Value == "facing" {
			if v == nil || v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
				return RoomCell{}, waypointPause{}, fmt.Errorf(
					"line %d: a waypoint's facing is a word, one of n|ne|e|se|s|sw|w|nw", key.Line)
			}
			pause.facing = v.Value

			continue
		}
		if v == nil || v.Kind != yaml.ScalarNode || v.Tag != "!!int" {
			return RoomCell{}, waypointPause{}, fmt.Errorf(
				"line %d: a waypoint's wait is a whole number of rounds", key.Line)
		}
		n, err := strconv.Atoi(v.Value)
		if err != nil {
			return RoomCell{}, waypointPause{}, fmt.Errorf(
				"line %d: a waypoint's wait is a whole number of rounds", key.Line)
		}
		pause.wait = n
	}
	out, err := decodeHexCell(cell)
	if err != nil {
		return RoomCell{}, waypointPause{}, err
	}

	return out, pause, nil
}

// repeatedKey records a key of a hand-read mapping and refuses one already
// seen, in yaml.v3's own sentence for the defect it would have caught had
// the mapping been decoded into a struct.
func repeatedKey(seen map[string]int, key *yaml.Node) error {
	if first, again := seen[key.Value]; again {
		return fmt.Errorf("line %d: mapping key %q already defined at line %d", key.Line, key.Value, first)
	}
	seen[key.Value] = key.Line

	return nil
}

// decodeHexCell reads one axial `{ q, r }` STRICTLY: both keys, each an
// integer, nothing else. A custom unmarshaller's nested decode does not keep
// the document's KnownFields, so the strictness every other mapping in the
// gameplay grammar gets is spelt out here — a missing `r` read as 0 would be
// a creature sent to a cell nobody wrote.
func decodeHexCell(node *yaml.Node) (RoomCell, error) {
	var cell RoomCell
	seen := map[string]int{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, val := node.Content[i], resolveNode(node.Content[i+1])
		if key.Value != "q" && key.Value != "r" {
			return RoomCell{}, fmt.Errorf("line %d: field %s not found in type dungeonspec.RoomCell",
				key.Line, key.Value)
		}
		if err := repeatedKey(seen, key); err != nil {
			return RoomCell{}, err
		}
		if val == nil || val.Kind != yaml.ScalarNode || val.Tag != "!!int" {
			return RoomCell{}, fmt.Errorf("line %d: a cell's %s must be an integer", key.Line, key.Value)
		}
		n, err := strconv.Atoi(val.Value)
		if err != nil {
			return RoomCell{}, fmt.Errorf("line %d: a cell's %s must be an integer", key.Line, key.Value)
		}
		if key.Value == "q" {
			cell.Q = n
		} else {
			cell.R = n
		}
	}
	for _, k := range []string{"q", "r"} {
		if _, ok := seen[k]; !ok {
			return RoomCell{}, fmt.Errorf("line %d: a cell is { q, r }, and this one has no %s", node.Line, k)
		}
	}

	return cell, nil
}

// TemperSpec is a temperament: one word, or a MIX to deal one from
// (rpg-project#465, design §3).
//
//	temper: coward
//	temper: { coward: 1, soldier: 2, aggressive: 1 }
//
// A WORD ON A PLACEMENT, A MIX ON A FACTION — and a faction may carry a word
// too, which every placement in it then has. "Four goblins, one table, four
// behaviours": the mix is dealt per member at spawn, through the shared dice
// with the faction as the die's entity, and the result is a beat so the
// streamer sees which goblin came out the coward.
//
// A PLACEMENT'S OWN WORD WINS and the mix is not dealt for it. An author who
// named this one's temperament has already answered the question the mix
// exists to ask.
type TemperSpec struct {
	// Word is the authored temperament, or empty when Mix is written.
	Word string

	// Mix is word → positive share, or nil when Word is written. A share
	// below 1 is a temperament that can never be dealt, refused by name.
	Mix map[string]int

	// Line is the file line, captured at decode for [AnswerSpec.Line]'s
	// reason.
	Line int
}

// UnmarshalYAML reads a temperament as a word or a mix, refusing a word
// outside the sealed list and a share below 1 by name.
func (t *TemperSpec) UnmarshalYAML(value *yaml.Node) error {
	t.Line = value.Line

	if value.Kind == yaml.ScalarNode {
		if !encounter.ValidTemperWord(value.Value) {
			return fmt.Errorf("line %d: %q is not a temperament this build ships: they are %s",
				value.Line, value.Value, strings.Join(encounter.TemperWords, ", "))
		}
		t.Word = value.Value

		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a temper is a word (%s) or a mix of them with shares",
			value.Line, strings.Join(encounter.TemperWords, " | "))
	}

	mix := map[string]int{}
	for i := 0; i < len(value.Content); i += 2 {
		word := value.Content[i].Value
		if !encounter.ValidTemperWord(word) {
			return fmt.Errorf("line %d: %q is not a temperament this build ships: they are %s",
				value.Content[i].Line, word, strings.Join(encounter.TemperWords, ", "))
		}
		var share int
		if err := value.Content[i+1].Decode(&share); err != nil {
			return fmt.Errorf("line %d: %q takes a share: %w", value.Content[i+1].Line, word, err)
		}
		if share < 1 {
			return fmt.Errorf("line %d: a share of %d can never be dealt — give %q a share of at least 1",
				value.Content[i+1].Line, share, word)
		}
		mix[word] = share
	}
	if len(mix) == 0 {
		return fmt.Errorf("line %d: a temper mix with nothing in it deals nothing", value.Line)
	}
	t.Mix = mix

	return nil
}

// enemyWords is the sealed `enemy:` vocabulary as strings, for a refusal that
// lists what there is.
func enemyWords() []string {
	out := make([]string, 0, len(encounter.EnemyWords))
	for _, w := range encounter.EnemyWords {
		out = append(out, string(w))
	}

	return out
}

// knownEnemyWord reports whether a word is one of them.
func knownEnemyWord(word string) bool {
	for _, w := range encounter.EnemyWords {
		if string(w) == word {
			return true
		}
	}

	return false
}

// threatWords is the sealed `threat:` vocabulary as strings, for a refusal
// that lists what there is.
func threatWords() []string {
	out := make([]string, 0, len(behavior.ThreatWords))
	for _, w := range behavior.ThreatWords {
		out = append(out, string(w))
	}

	return out
}

// knownThreatWord reports whether a word is one of [behavior.ThreatWords].
func knownThreatWord(word string) bool {
	for _, w := range behavior.ThreatWords {
		if string(w) == word {
			return true
		}
	}

	return false
}

// knownWhenDeed reports whether a key names a deed a `when` may read.
func knownWhenDeed(key string) bool {
	for _, d := range encounter.WhenDeeds {
		if d == key {
			return true
		}
	}

	return false
}

// selectorWords is the sealed selector vocabulary as strings.
func selectorWords() []string {
	out := make([]string, 0, len(encounter.SelectorWords))
	for _, w := range encounter.SelectorWords {
		out = append(out, string(w))
	}

	return out
}

// knownSelectorWord reports whether a word is one of them.
func knownSelectorWord(word string) bool {
	for _, w := range encounter.SelectorWords {
		if string(w) == word {
			return true
		}
	}

	return false
}

// UnmarshalYAML reads one answer entry, refusing the designed-but-unbuilt
// words by name ([laterWords]) and every unknown key, for
// [PlaceSpec.UnmarshalYAML]'s reason.
func (r *AnswerSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf(
			"line %d: an answer entry is { weight, say, when } plus exactly one of { fact, flee, hold, attack, toward, dash, away, search }",
			value.Line)
	}
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if why, later := laterWords[key]; later {
			return fmt.Errorf("line %d: `%s` is %s (rpg-project#458)", value.Content[i].Line, key, why)
		}
		if key == "patrol" {
			line := value.Content[i].Line

			return spelledRefusal{say: func(f cellForms) string {
				return fmt.Sprintf("line %d: %s", line, patrolRefusal(f))
			}}
		}
		switch key {
		case "weight", "say", "when", "fact", "flee", "hold", "attack", "toward", "dash", "away", "search",
			"use", "with", "as":
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.AnswerSpec",
				value.Content[i].Line, key)
		}
	}
	type answerBody AnswerSpec
	var obj answerBody
	if err := value.Decode(&obj); err != nil {
		return err
	}
	*r = AnswerSpec(obj)
	r.Line = value.Line

	return nil
}

// PlaceSpec is one authored placement at an ABSOLUTE cell.
type PlaceSpec struct {
	// ID is the author's name for this placement — the third id in this
	// file after a region's and a door's (rpg-project#368, design P2).
	// Optional: a placement nothing binds to needs no name.
	//
	// REQUIRED BY WHATEVER BINDS TO IT, and by nothing else. A knowledge
	// link's SUBJECT needs none — `knows` names doors, and a monster that
	// knows one is just a monster. A scenario binding needs one, and a
	// holdable prop needs one because both the binding and the `held` beat
	// have to be able to say WHICH thing. Refused on collision, naming both
	// lines.
	ID string `yaml:"id,omitempty"`

	// Ref is content's identifier, "module:type:id". Its TYPE segment routes
	// the placement: "props" becomes a prop, "monsters" becomes a member.
	// Nothing else about it is read.
	Ref string `yaml:"ref"`

	// At is the absolute cell, offset [col,row]. Must be floor, and no two
	// placements may share one.
	At [2]int `yaml:"at"`

	// BlocksMovement is whether a prop stops somebody standing there.
	// REQUIRED on every prop, and REFUSED on anything else — a blocker
	// nobody declared is a wall nobody drew ([encounter.PropInput]).
	BlocksMovement *bool `yaml:"blocks_movement,omitempty"`

	// BlocksLoS is whether a prop stops a sightline. REQUIRED on every prop
	// and refused on anything else, for BlocksMovement's reason.
	BlocksLoS *bool `yaml:"blocks_los,omitempty"`

	// Facing is the prop's authored direction, one of the EIGHT true-compass
	// names — n|ne|e|se|s|sw|w|nw — the SAME eight under BOTH orientations
	// (rpg-project#272, superseding #261's orientation-scoped six-name
	// sets: compass directions live in world space, and a prop must be able
	// to stand squarely against an axis-true wall whatever way the hexes
	// point). A name outside the set is refused by name, never silently
	// snapped to the nearest valid one. Optional: omitted means the asset's
	// own default facing. REFUSED on monsters today.
	//
	// THAT REFUSAL IS A DEFERRAL, NOT A FINDING, AND IT SAYS SO HERE BECAUSE
	// IT DID NOT BEFORE (rpg-toolkit#1892). The field was authored for props
	// first and the monster case was filed under "not now" with an OWNER —
	// ideas/dungeon-builder/prop-facing-offset.md: "authored spawn facing is
	// a shelf item for the Monster AI journey"; cardinal-facings.md lists
	// "monster spawn facing (#201)" beside free-angle facing and wall height.
	// A later comment kept the decision and dropped both the owner and the
	// reason it was temporary, so it read as settled doctrine and was acted
	// on as one — a session designing a per-placement reference reasoned from
	// it that ANY per-instance monster fact on a placement was a shelf item.
	//
	// TWO THINGS WERE CONFLATED, and only one of them is the AI journey's:
	//
	//   - facing IN PLAY — a monster turns to track a target. That is #201's.
	//   - facing AT SPAWN — which way it stood when the door opened. That is
	//     authored INITIAL STATE, the sibling of [PlaceSpec.At], and nobody
	//     owns it. Kirk, 2026-09-24: "a monster should face a direction when
	//     starting. weird opening a door and the monsters are not facing the
	//     door." That is the staging problem: the encounter's first frame is
	//     wrong, with no read of who was aware and an unearned beat for the
	//     party. None of it is the AI's to fix.
	//
	// WHAT WOULD LIFT IT: a ruling on #1892 accepting `facing` on monsters
	// under `place[]`, separate from #201. The vocabulary already exists
	// ([facings]) and the field already parses, so it is a validation flip and
	// its goldens rather than new capability.
	//
	// THE GENERAL RULE THIS IS AN INSTANCE OF: a "not now" in this dialect
	// must name its owner and its trigger, or it will be read as a ruling. The
	// refusals that get this right say where the word went ([laterWords],
	// [knowsRefusal]); this one said only "no", and the cost was a session
	// arguing against authored spawn facing from a scope fence.
	Facing string `yaml:"facing,omitempty"`

	// Offset is an authored VISUAL displacement: [x,y] within-cell nudge
	// fractions of the cell size, each REQUIRED in [-0.5, 0.5] when
	// authored (Kirk's ruling: "offset is visual only, agreed" — the prop
	// still occupies its whole cell for movement and LOS), plus an OPTIONAL
	// third component: height above the floor in the same cell-size unit,
	// REQUIRED in [0, 3] when authored and deliberately NOT bound to the
	// planar clamp (rpg-project#272: "height should be able to gun higher
	// than the 5 ticks we allow on x and y"). Omitted means centered on the
	// floor, which [0,0] and [0,0,0] also mean — all the same fact by
	// design. A SLICE, not a fixed-size array, so a list of the wrong
	// length is a validate-level defect naming place[i].offset rather than
	// a decode-time line number. REFUSED on monsters, for Facing's reason.
	//
	// NIL, NOT LEN 0, IS "OMITTED": yaml.v3 leaves this nil when the key is
	// absent but decodes `offset: []` to a non-nil, zero-length slice, and
	// the two are different authored facts — the second is a malformed list
	// somebody wrote, refused the same as any other wrong-length one. Every
	// presence check in validate.go tests the pointer-ness (`!= nil`), never
	// the length, for exactly this reason.
	Offset []float64 `yaml:"offset,omitempty"`

	// Targeting is the author's word for how a monster picks a target.
	// Monsters only. Carried opaquely and never interpreted here.
	Targeting *string `yaml:"targeting,omitempty"`

	// Actions is what this monster can do, in the author's own order
	// (rpg-project#448). MONSTERS ONLY, refused on anything else exactly as
	// Targeting is. Optional: omitted means the monster's definition keeps
	// the arms its stat block gives it.
	//
	// FULL REFS, like every other ref in this file: `dnd5e:weapons:shortbow`,
	// never `shortbow`. A weapon a monster carries is the catalog's weapon
	// and carries the catalog's ref, because there is nothing different
	// about a goblin's shortbow and a skeleton's — both are +4 for 1d6+2,
	// and both numbers are the wielder's.
	//
	// THE ORDER IS THE POINT. Both drivers take the first action whose
	// target is in reach, so an archer that lists a blade first swings when
	// you are standing on it and one that lists only a bow shoots you point
	// blank. That is how two goblins in one room tell different stories
	// without a line of Go: `[scimitar, shortbow]` and `[shortbow]`.
	//
	// CARRIED, NOT INTERPRETED, like Targeting and Holds. This package
	// refuses a ref that is not a well-formed `dnd5e:weapons:<id>` and stops
	// there: whether the catalog HAS that weapon is a question only the
	// rulebook can answer, and the rulebook is not a dependency of this
	// module. The session answers it at spawn and refuses the file's boot;
	// a turn never meets an unknown ref.
	//
	// WHY ONLY WEAPONS TODAY. The design keeps authored non-weapon actions —
	// a claw, a bite, a multiattack — in this same list, and none exists
	// yet. Refusing the types that cannot appear is what makes a typo'd
	// `dnd5e:weapon:shortbow` a field error on the form instead of a
	// surprise at boot, and admitting the first real claw is a one-line
	// change here.
	Actions []string `yaml:"actions,omitempty"`

	// Boss marks the monster whose death ends things. Monsters only, and at
	// most one per region.
	//
	// UNTOUCHED BY THIS SLICE and deliberately so (design R8): endings come
	// from scenarios, and the named follow-up converts the reference tomb to
	// the kill-the-captain scenario and deletes this flag then. A dungeon
	// that binds a scenario AND flags a boss has two endings; that is the
	// author's business and it is visible on the form.
	Boss bool `yaml:"boss,omitempty"`

	// Holds is the intel records this placement carries, by record id
	// (rpg-project#372, design §2). Optional; legal on MONSTERS AND PROPS
	// (R6), refused only when the id names no record.
	//
	// A MONSTER CARRIES IT FROM SPAWN. The captain is not a role: it is a
	// monster holding a record, and nothing in the game needs the word
	// captain.
	//
	// A PROP CARRIES IT UNTIL SOMEBODY PICKS IT UP. Kirk, walking the
	// stack: "tech could get intel by holding something too. if we want to
	// test the intel we need to be able to place it on a few things — not
	// the hardest monster to kill in the game." A scroll on a table is the
	// obvious shape, and it is also the whole hold-out's-letter idea with
	// nothing extra: the record stays ON the prop, so whoever holds the prop
	// next is taught in turn.
	//
	// Refused by name when no record has that id.
	//
	// THE SAME RECORD MAY BE HELD BY SEVERAL MONSTERS, because intel copies
	// rather than moving (holdings.go): two guards may both know the way in,
	// and looting either teaches it. That is the difference between a record
	// and a physical thing, and it is why this is not refused as a duplicate.
	Holds []string `yaml:"holds,omitempty"`

	// Holdable is whether a member can pick this prop up (design §5).
	// Optional; PROPS ONLY, and refused on a monster.
	//
	// A POINTER, for [PlaceSpec.BlocksMovement]'s reason one type over: an
	// omitted `holdable` and an authored `holdable: false` are the same fact
	// here — a thing nobody declared holdable stays scenery — but the
	// pointer is what lets the refusal below distinguish "said nothing" from
	// "said false" on a monster, so a monster that wrote `holdable: false`
	// is told it cannot declare that at all rather than silently accepted.
	//
	// A HOLDABLE PROP MUST HAVE AN ID, refused otherwise: the scenario
	// binding names it and the `held` beat names it, and neither can name a
	// thing with no name.
	Holdable *bool `yaml:"holdable,omitempty"`

	// Faction is the faction this monster belongs to (rpg-project#375, the
	// hold-out design §2). MONSTERS ONLY, refused on a prop; MUST name a
	// declared faction (or `monsters`, which it is anyway); absent means the
	// reserved `monsters` faction. `party` is refused — that is the players'
	// side. Carried to the host as [MonsterPlacement.Faction], which hands
	// it to [encounter.MemberInput.Faction] when it spawns the sheet.
	Faction string `yaml:"faction,omitempty"`

	// Options is the lines a character can say to this placement — a
	// creature or a prop — each with its check, what it waits on and what
	// is answered on either verdict ([OptionSpec], rpg-project#569).
	//
	//	options:
	//	  - id: lean-on-it
	//	    say: "Where's the cellar? Tell me or you're next."
	//	    check: [{ ability: intimidation, dc: 12 }]
	//	    success:
	//	      - { weight: 70, say: "Fine! The cellar door is behind the barrels.", fact: goblin-cowed }
	//	      - { weight: 30, say: "Boss! BOSS!", flee: {} }
	//	    failure:
	//	      - { say: "Big talk for someone standing in my doorway." }
	//
	// OMITTED MEANS NOTHING TO SAY (R2): a creature nobody wrote a line for
	// cannot be spoken to, and no default stands behind the silence. THE
	// WORLD BUILDER IS THE ONLY PLACE a thing gains a line, and that is the
	// point.
	//
	// IT REPLACED `intimidate:` AND `persuade:` — two priced verbs whose
	// answers lived on four verb-keyed `on:` keys. Both keys are refused by
	// name with where they went. A prop's options may not `flee`, and a prop
	// carrying options needs an id: a line is said to it by id.
	Options []OptionSpec `yaml:"options,omitempty"`

	// On is the ANSWER TABLE: what this monster does with its time, as a
	// weighted list of entries under `time` (rpg-project#465). MONSTERS ONLY.
	//
	//	on:
	//	  time:
	//	    - { when: { enemy: reach }, attack: enemy }
	//	    - { when: { enemy: seen },  toward: enemy }
	//
	// KEYED BY TRIGGER SO THE NEXT TRIGGER ADDS A KEY, NOT A FIELD. `time` is
	// the one this build rolls; anything else is a field error naming the
	// key, a word the design named but has not built is refused by name with
	// where it went ([laterWords]), and the four retired social keys are
	// refused naming options, where they went (rpg-project#569 R2).
	//
	// Absent means the creature answers nothing, which is the common case.
	On map[string][]AnswerSpec `yaml:"on,omitempty"`

	// Temper is this placement's temperament, one of the sealed words
	// ([TemperSpec], design §3) — `temper: coward`. MONSTERS ONLY, refused
	// on anything else.
	//
	// A WORD HERE, A MIX ON THE FACTION. A placement names ONE creature, so
	// dealing a spread for it would be an author rolling for a goblin they
	// have already described. Absent means the faction's mix deals one, and
	// absent with no mix either means a soldier — every factor 100, which is
	// the zero value telling the truth.
	Temper string `yaml:"temper,omitempty"`

	// Arrives is the predicate that brings this placement into the run
	// (rpg-project#375, the hold-out design §2, §3.7, R6). MONSTERS AND
	// PROPS. Optional: omitted means the thing is there from the first
	// frame, as every placement always was.
	//
	// A PLACEMENT WITH A PREDICATE IS IN RESERVE until it holds — absent
	// from every map and every roster, then placed at `at` (or the nearest
	// free cell of its region when something stands there) with an `arrived`
	// beat. `at` MUST be floor, and for a monster standable, exactly as for
	// a placement that stands there from the start: the cell is where it
	// will arrive. Refused when it can never hold — a round counted from 0,
	// a `{ down }` naming no placement or a prop, a stance the pair can
	// never reach, a placement waiting on its own fall, or a ring of
	// placements each waiting on another's — by the liveness rule every
	// predicate in this file meets. The dungeon ALLOWS a `{ fact }` no record
	// reveals (R8, pre-release: show the cost).
	//
	// Carried to the host as [MonsterPlacement.Arrives] for a monster, which
	// hands it to [encounter.MemberInput.Arrives] when it spawns the sheet;
	// compiled straight onto [encounter.PropInput.Arrives] for a prop.
	Arrives *PredicateSpec `yaml:"arrives,omitempty"`
}
