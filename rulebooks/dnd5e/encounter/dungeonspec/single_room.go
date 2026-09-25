package dungeonspec

import "gopkg.in/yaml.v3"

// SingleRoomSpec is the authoring document for one playable room, and the
// SITE document that room belongs to (rpg-project#477, Decision 1).
//
// # The site scope, and why it is at the root
//
// [SingleRoomSpec.Factions] and [SingleRoomSpec.Dispositions] are the scope
// that belongs to NO SINGLE CREATURE: a shared answer table is inherited by
// many and owned by none, so it cannot hang off a selection. They are the
// v2 root's own keys ([FactionSpec], [DispositionSpec]) reaching the
// single-room dialect unchanged — the same shapes, the same refusals, the
// same words — because a second spelling of "who fights whom" is exactly the
// drift a shared dialect exists to prevent.
//
// Both are OPTIONAL and ABSENT WHEN UNAUTHORED. A document that declares
// neither decodes, compiles and marshals exactly as it did before these keys
// existed, which is what lets every room authored before the site scope
// survive it untouched.
//
// # The other four root keys, and the same reason
//
// [SingleRoomSpec.Intel], [SingleRoomSpec.Exits], [SingleRoomSpec.Endings]
// and [SingleRoomSpec.Scenarios] joined them for rule 3 of the placement law
// (rpg-project#488): what is NOT PLACED lives at the root, beside `factions`
// and `dispositions`. A record of knowledge is a thing in the file like a
// door — declared once, referred to by name from whatever carries it
// ([RoomMonsterBinding.Holds]) — and it stands nowhere, so there is no placed
// thing to key it by. A way out, an ending and a scenario binding are the
// same: none of them is a thing standing on the floor.
//
// EACH IS THE V2 ROOT'S OWN KEY IN THIS DIALECT, with its v2 shape, its v2
// sentences and its v2 refusals at this dialect's paths — because a second
// spelling of "how the run ends" is the drift a shared dialect exists to
// prevent. The v2 spellings are [Spec.Intel], [Spec.Exits], [Spec.Endings]
// and [Spec.Scenarios].
//
// ONE OF THEM CARRIES A CELL, AND ONLY ITS FRAME DIFFERS. A v2 exit is
// authored `at: [col, row]`, an absolute offset pair in the orientation that
// document declares; this dialect has no orientation and its cells are axial,
// so an exit here is authored `cell: {q, r}` — [RoomPartyStart]'s own shape,
// by ruling (rpg-project#488 R2). Everything else about an exit is v2's: the
// id it must have, the id no two may share, and the standable floor it must
// stand on. See [RoomExit].
type SingleRoomSpec struct {
	Version int            `yaml:"version" json:"version"`
	Key     string         `yaml:"key" json:"key"`
	Play    SingleRoomPlay `yaml:"play" json:"play"`

	// Factions are the sides this site declared, in authored order. Optional;
	// the shape and every refusal are [FactionSpec]'s.
	Factions []FactionSpec `yaml:"factions,omitempty" json:"factions,omitempty"`

	// Dispositions are how those sides stand to each other, in authored
	// order. Optional; the shape and every refusal are [DispositionSpec]'s.
	Dispositions []DispositionSpec `yaml:"dispositions,omitempty" json:"dispositions,omitempty"`

	// Intel is the knowledge this site declared, in authored order — the v2
	// root's own key ([Spec.Intel], rpg-project#488 R1 rule 3) reaching this
	// dialect unchanged: the same [IntelSpec] shape, the same required and
	// unique id, the same "a record says exactly one thing it reveals".
	// Optional; absent means none.
	//
	// TWO TARGETS IN THIS DIALECT: `fact` and `concealment`.
	// [RevealsSpec.Door] is REFUSED by name at `intel[<i>].reveals.door`
	// (rpg-project#488 R3, retargeted by rpg-project#490 R7): a record never
	// revealed the hinge, it revealed the vault, and this dialect names the
	// vault directly under root `concealments:`. The field stays on the
	// shared shape so the refusal can be a sentence at the author's own path
	// instead of "not a key this build reads".
	Intel []IntelSpec `yaml:"intel,omitempty" json:"intel,omitempty"`

	// Concealments are the secrets this room hides, keyed by id
	// (rpg-project#490, R1). Optional; absent means it hides nothing, which
	// is what every room authored before this key existed does.
	//
	// ONE NOUN, AND EVERYTHING HIDDEN BELONGS TO IT. A concealment lists the
	// cells it hides and the placed things it hides — doors, wall props, a
	// bookcase, the heirloom behind it — and nothing else in the file says
	// "hidden". `doorBindings.<id>.concealed` is gone: a door is hidden by
	// being listed in a concealment's `props`, which is the same sentence
	// said once for the door and for the room behind it.
	//
	// AT THE ROOT, beside `intel:` and `factions:`, by rule 3 of the
	// placement law (rpg-project#488): a secret is not a thing standing on
	// the floor, it is a fact about several things that are. See
	// [ConcealmentSpec].
	Concealments map[string]ConcealmentSpec `yaml:"concealments,omitempty" json:"concealments,omitempty"`

	// Exits are the ways out of this room, in authored order — the v2 root's
	// own key ([Spec.Exits]) reaching this dialect with its cell in this
	// dialect's frame (rpg-project#488 R2). Optional; absent means none.
	//
	// STRUCTURE, NOT SCENARIO, which is [Spec.Exits]' own reason for sitting
	// beside the way in rather than inside a quest: a room has ways out
	// whatever the party is there for. `partyStart` is NOT implicitly one of
	// these — nothing is defaulted (rpg-toolkit#1033) — so a room whose
	// entrance is also its way out says so in one line.
	Exits []RoomExit `yaml:"exits,omitempty" json:"exits,omitempty"`

	// Endings are the ways this room's run can end, in authored order — the
	// v2 root's own key ([Spec.Endings], [EndingSpec]) reaching this dialect
	// unchanged: the same required and unique id, the same required `when`,
	// the same liveness rule. Optional; absent means none of its own, and the
	// scenarios this document binds still declare theirs.
	//
	// THE PREDICATE GRAMMAR, SAME AS EVERY OTHER SINK. `when` is the
	// [PredicateSpec] a disposition's `until` and a binding's `arrives`
	// already are, judged by the one grammar and compiled by the one
	// [predicateOf].
	Endings []EndingSpec `yaml:"endings,omitempty" json:"endings,omitempty"`

	// Scenarios binds this room to the scenarios it is authored for: a map
	// from scenario id to that scenario's bindings, each binding a field key
	// to the id of something in this document — [Spec.Scenarios] verbatim,
	// the same Go shape. Optional; absent means none.
	//
	// CARRIED OPAQUELY, AND VALIDATED ONLY AS REFERENCES. This package checks
	// exactly one thing about a binding: that its value names something this
	// document declares — a monster, a declared prop, an exit or a faction.
	// What the keys mean, which are required, and whether the thing named is
	// the right KIND of thing are the scenario package's own refusals, asked
	// at its `New(cfg)` in form-filler words. Design law C1 is the reason:
	// this package never resolves content, and a scenario is content.
	Scenarios map[string]map[string]string `yaml:"scenarios,omitempty" json:"scenarios,omitempty"`

	// Tables are the answer tables this site declared, keyed by id
	// (rpg-toolkit#1897). Optional; absent means none, and every document
	// authored before this key existed decodes, compiles and marshals
	// exactly as it did.
	//
	// AT THE ROOT, BESIDE `factions:` AND `concealments:`, BY RULE 3 of the
	// placement law (rpg-project#488): what is NOT PLACED lives at the root.
	// A table is declared once and named from whatever answers it
	// ([RoomMonsterBinding.Table]), and it stands nowhere — so there is no
	// placed thing to key it by, exactly as a record of knowledge has none.
	//
	// ONE COPY OF A SHARED TABLE, WHICH IS THE POINT. Before this key a table
	// written for six goblins was written six times; here it is written once
	// and six bindings name it. The grammar INSIDE a table is
	// [RoomMonsterBinding.On]'s — the same map[string][]AnswerSpec, the same
	// triggers, words, bands and refusals — because a second spelling of
	// "what this creature does" is the drift a shared dialect exists to
	// prevent.
	//
	// A BASE, NOT A REPLACEMENT. A referenced table is what a binding's own
	// `on:` layers over, and a binding's own `on:` still layers over its
	// faction's. [ordersOf] is unchanged and this key adds no rule to it —
	// only another source of the same `On`.
	Tables map[string]TableSpec `yaml:"tables,omitempty" json:"tables,omitempty"`

	Room RoomSource `yaml:"room" json:"room"`
}

// TableSpec is one authored answer table at the root: a named block of orders
// any number of bindings may name (rpg-toolkit#1897).
//
// THE SHAPE IS THE TABLE ITSELF, NOT A WRAPPER AROUND ONE. A root table is
// keyed by id in [SingleRoomSpec.Tables], so the id is the MAP KEY and the
// value is the table — [SingleRoomSpec.Concealments]' decision, and for its
// reason: an `id:` field inside the value would be a second place the same
// name is written, and the two could disagree.
//
//	tables:
//	  goblin-mind:
//	    time:
//	      - { when: { enemy: reach }, attack: enemy }
//	      - { when: { enemy: none }, hold: {} }
//
// A named type rather than a bare map so the field has somewhere to live and
// so a refusal can name it; the underlying shape is
// [RoomMonsterBinding.On]'s exactly.
type TableSpec map[string][]AnswerSpec

// RoomExit is one authored way out: an id, and the cell a member stands on to
// leave through it (rpg-project#488 R2).
//
//	exits:
//	  - { id: entrance, cell: { q: 1, r: 3 } }
//
// [ExitSpec] with `at` replaced by `cell`, and that swap is the whole
// difference between the dialects. `at` is an absolute [col,row] offset pair
// resolved through a document `orientation`, and this dialect declares none —
// its cells are axial — so the same bytes would name two different cells in
// the two dialects. The shape it takes here is [RoomPartyStart]'s, which is
// the point: a way out is the same kind of authored cell as a way in, and the
// two answering differently about one cell would be the bug.
//
// A DOOR'S HEX IS A THRESHOLD, and an exit standing in one composes with it
// without a second spelling: a closed or locked door blocks movement, so
// nobody stands on an exit cell inside one until it is opened — DoorState's
// own law, needing no rule of its own (R2). Exit-as-door is the sites layer's
// question, deferred and not foreclosed.
type RoomExit struct {
	// ID names the exit within this document, and is what a scenario binding
	// names. REQUIRED non-empty and unique — [ExitSpec.ID]'s rule and its
	// sentences: a binding that named an ambiguous exit would have no answer.
	ID string `yaml:"id" json:"id"`

	// Cell is where somebody stands to leave through it, in this dialect's
	// axial frame. REQUIRED — an exit with no cell has no honest default,
	// since `{q: 0, r: 0}` is a real hex somebody may have painted.
	//
	// Must be STANDABLE, which is [ExitSpec.At]'s rule answered by this
	// dialect's own geometry: the cell is put through the same
	// [encounter.ValidateStaticPlacements] `partyStart` and every monster
	// cell goes through, at this exit's own path. An exit nobody can stand on
	// is a run nobody can leave.
	Cell RoomCell `yaml:"cell" json:"cell"`
}

// SingleRoomPlay declares the runtime assumptions required by a single room.
type SingleRoomPlay struct {
	Void     string `yaml:"void" json:"void"`
	Lighting string `yaml:"lighting" json:"lighting"`
	Standing string `yaml:"standing" json:"standing"`
}

// RoomSource contains a room's gameplay authoring data, and carries the
// World Builder's presentation beside it AS THE AUTHORED NODES IT IS
// (rpg-project#479, R3).
//
// THE PRESENTATION IS CONTENT AND THIS PACKAGE DOES NOT MODEL IT. Assets,
// labels, groups, parents, supports, height scales and point lights belong
// to the World Builder; the player is served the authored file by dungeon
// key and the codec that owns those words (the web's) is the one that judges
// them. [CoordinateFrame], [Workspace] and [Scene] are therefore YAML nodes,
// carried verbatim and walked for exactly the values play depends on —
// single_room_lowering.go names every one of them, by path. An unknown key
// inside any of the three is not this decoder's business.
//
// They marshal back to YAML as the nodes they are, so a host that re-emits a
// decoded document gets the author's own bytes back.
//
// THIS TYPE IS YAML, AND SAYS SO. It carries no JSON tags at all: a
// [yaml.Node] has no honest JSON shape, so a JSON round trip of a room would
// carry the gameplay half and silently drop the presentation — a document
// that looks like it survived and did not. The room is authored as YAML,
// stored as YAML (rpg-api's dungeon registry "never re-marshals a file:
// GetDungeon hands back exactly the bytes that were Put"), and re-emitted as
// YAML. Nothing marshals it as JSON, and the missing tags are what keeps
// that from starting by accident.
type RoomSource struct {
	Version int    `yaml:"version"`
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`

	CoordinateFrame yaml.Node `yaml:"coordinateFrame"`
	Workspace       yaml.Node `yaml:"workspace"`
	Scene           yaml.Node `yaml:"scene"`

	Gameplay RoomGameplaySource `yaml:"room"`
}

// RoomCell is an authored axial hex coordinate.
type RoomCell struct {
	Q int `yaml:"q" json:"q"`
	R int `yaml:"r" json:"r"`
}

// RoomPartyStart identifies the authored party start cell.
type RoomPartyStart = RoomCell

// RoomStartingCell is a monster's authored start: where it stands and which
// way it faces, one noun rather than a cell beside a sibling field. A hex
// with a location has an orientation (rpg-toolkit#1899).
type RoomStartingCell struct {
	// Location is the authored axial hex coordinate.
	Location RoomCell `yaml:"location" json:"location"`

	// Facing is the direction the monster faces when it arrives, one of the
	// eight true-compass names — n|ne|e|se|s|sw|w|nw — or empty, which means
	// the asset's own default facing. Optional: a model has no "no
	// orientation", so absent is a default, not a gap. Carried verbatim and
	// never turned into an angle — angle math is a render concern.
	Facing string `yaml:"facing,omitempty" json:"facing,omitempty"`
}

// RoomMonsterSource declares what a monster is and where it started.
// Shared references and overrides belong to [RoomMonsterBinding], including
// faction membership. This source declaration is not live runtime state.
type RoomMonsterSource struct {
	ID  string `yaml:"id" json:"id"`
	Ref string `yaml:"ref" json:"ref"`

	// StartingCell is where the monster stands and which way it faces when
	// it arrives. See [RoomStartingCell].
	StartingCell RoomStartingCell `yaml:"startingCell" json:"startingCell"`
}

// RoomMonsterBinding holds one creature's shared references and overrides:
// faction, table, temperament, arms, and gameplay interactions. It is keyed by
// the id declared in monsterDeclarations. A binding naming no declared
// creature is refused; a faction-only binding is valid and inherits its orders.
//
// EVERY FIELD IS OPTIONAL, and the common state of this whole block is
// absence: a creature with nothing to override needs no binding at all.
type RoomMonsterBinding struct {
	// Faction names a declared side. Absence preserves the monster's default
	// side; membership is a shared reference, not part of its declaration.
	Faction string `yaml:"faction,omitempty" json:"faction,omitempty"`

	// Table names a root answer table ([SingleRoomSpec.Tables]) this creature
	// answers with (rpg-toolkit#1897). Optional; absent means this creature
	// names no shared table and its orders come from `on:` and its faction
	// alone, exactly as they did before this field existed.
	//
	// A TABLE IS A BASE, AND `on:` STILL WINS. The referenced table is what
	// this binding's own `on:` is laid over — the same key-by-key,
	// nearer-layer-wholesale rule [ordersOf] already applies to a faction's
	// table. So a creature may share a table and override one trigger of it
	// without copying the rest:
	//
	//	monsterBindings:
	//	  gob-2: { table: goblin-mind, on: { time: [{ when: { enemy: none }, hold: {} }] } }
	//
	// THE ID IS RESOLVED AND NEVER INTERPRETED. It names a table this
	// document declares; an id this document does not declare is refused BY
	// NAME at this path, [RoomMonsterBinding.Holds]' treatment of an
	// undeclared record. Nothing here reads the table's contents — [ordersOf]
	// does that, once, for every dialect.
	Table string `yaml:"table,omitempty" json:"table,omitempty"`

	// On is this creature's own answer table, LAID OVER its faction's
	// ([FactionSpec.On]) key by key, the nearer layer winning WHOLESALE. The
	// shape, the grammar and every refusal are [PlaceSpec.On]'s.
	//
	// ONE EXCEPTION, AND IT IS A REFUSAL: a selector may not name a cell
	// here. `{ at: [col, row] }` is an offset resolved through a document
	// `orientation`, and this dialect has none — its cells are axial — so
	// the same bytes would mean two things in the two dialects. Refused with
	// a sentence that says so, until a sites layer gives a creature
	// somewhere else to walk to.
	On map[string][]AnswerSpec `yaml:"on,omitempty" json:"on,omitempty"`

	// Temper is this creature's temperament: ONE WORD, and it WINS over its
	// faction's word or mix ([TemperSpec]). Optional; absent means the
	// faction answers, and a faction with a mix deals one per member.
	Temper string `yaml:"temper,omitempty" json:"temper,omitempty"`

	// Actions is what this creature can do, in the author's own order — the
	// single-room equivalent of [PlaceSpec.Actions]. Full
	// `dnd5e:weapons:<id>` refs, CARRIED AND NEVER INTERPRETED, and THE
	// ORDER IS THE POINT: both drivers take the first action whose target is
	// in reach, so this list is never sorted and never deduplicated.
	Actions []string `yaml:"actions,omitempty" json:"actions,omitempty"`

	// Holds is the intel records this creature carries from spawn, by record
	// id — [PlaceSpec.Holds]'s field and its one refusal (rpg-project#488 R1,
	// the key table's "who carries it"): a record id this document does not
	// declare under root `intel:` is refused by name at
	// `…monsterBindings.<id>.holds[<j>]`.
	//
	// ORDERS, NOT IDENTITY, which is why it is here and not on the
	// `monsterDeclarations:` entry (rule 1). "The captain is not a role: it is a monster
	// holding a record, and nothing in the game needs the word captain."
	//
	// THE SAME RECORD MAY BE HELD BY SEVERAL CREATURES, for [PlaceSpec.Holds]'
	// reason: intel copies rather than moving, so two guards may both know the
	// way in and looting either teaches it. Not refused as a duplicate.
	Holds []string `yaml:"holds,omitempty" json:"holds,omitempty"`

	// Intimidate is the check a character must beat to frighten this creature
	// — [PlaceSpec.Intimidate]'s shape and its sentences (rpg-project#454;
	// rpg-project#488 R5), the same approach list a lock carries, priced per
	// route.
	//
	// OMITTED MEANS NOT OFFERED (rpg-project#494 R1): absent, this creature
	// cannot be intimidated and no default stands behind the silence. The
	// NIL-VS-EMPTY LAW IS [RoomDoorBinding.Locked]'s — nil is "the author
	// said nothing", and `intimidate: []` is an authored check that forgot to
	// say how it is beaten, refused by name.
	Intimidate CheckSpec `yaml:"intimidate,omitempty" json:"intimidate,omitempty"`

	// Persuade is [RoomMonsterBinding.Intimidate]'s twin — the check to talk
	// this creature round ([PlaceSpec.Persuade], rpg-project#458;
	// rpg-project#488 R5). The same shape, the same silence when absent, and
	// the same nil-vs-empty law.
	Persuade CheckSpec `yaml:"persuade,omitempty" json:"persuade,omitempty"`

	// Arrives is the predicate that brings this creature into the run —
	// [PlaceSpec.Arrives]'s field and [PredicateSpec]'s one grammar
	// (rpg-project#488 R1, the key table's "something that comes in later").
	// Optional: absent means it stands there from the first frame, as every
	// creature in this dialect did before this key existed.
	//
	// A CREATURE WITH A PREDICATE IS IN RESERVE until it holds — on no map
	// and in no roster, then placed at its authored `cell:` with an `arrived`
	// beat. Refused when it can never hold, by the liveness rule every
	// predicate in this document meets: a creature waiting on its own fall,
	// or a ring of reserved creatures each waiting on another's.
	Arrives *PredicateSpec `yaml:"arrives,omitempty" json:"arrives,omitempty"`
}

// RoomPropBinding is one placed item's ORDERS: what the party can do with it,
// what it carries, and whether it is here yet (rpg-project#488, R1).
//
// The FOURTH declaration kind on a placed thing, after `propDeclarations`,
// `monsterBindings` and `doorBindings`, KEYED BY THE SAME LAW: the id of the
// placed item it is about, which must be an id `propDeclarations` declares.
//
//	propBindings:
//	  heirloom: { holdable: true }
//	  letter:   { holdable: true, holds: [wisemans-letter], arrives: { round: 6 } }
//
// # Definition, state, orders — and which is which
//
// `propDeclarations` is the item's DEFINITION: its footprint and what it
// blocks, which is the World Builder's own (world-builder-vs-dungeon-builder).
// `doorBindings` is a door's STATE. This is what a PLACED prop DOES, which is
// a binding by rule 1 of the placement law — identity on the actor, orders on
// the binding. An item may carry the definition (required, it is where the
// footprint comes from) plus at most ONE of the other two.
//
// # What it may not be, and why each one says so
//
// An id no `propDeclarations` entry owns has no prop to give orders to. An id
// that is ALSO a door is a door somebody picks up, and nothing says what that
// means until a use case does. An id naming an arrangement TEMPLATE is orders
// inside a stamp, which nothing remaps yet — the same refusal an arrangement
// door gets. Each is refused by name at its own path.
//
// # Where the three keys land
//
// A v4 item compiles to [encounter.PlacedPropInput], a FOOTPRINT, and since
// rpg-toolkit#1854 a footprint carries all three: [applyPropBindings] lays
// them onto the placement the item's own declaration produced, through the
// compilers the monster binding already uses. Reach is DERIVED from the
// rectangle rather than authored — the legacy hold rule applied to every cell
// the footprint stands on ([encounter.PlacedPropInput.Holdable]) — so nothing
// here asks for the anchor cell a v4 item does not have.
//
// This block used to DECODE and not COMPILE, refused by name while that
// primitive was missing. Said here rather than deleted, because the refusal
// is what the World Builder saw for a release and its sentence named the
// issue that lifted it.
type RoomPropBinding struct {
	// Holdable is whether a member can pick this prop up
	// ([PlaceSpec.Holdable], [encounter.PlacedPropInput.Holdable]). Optional.
	//
	// A PLAIN BOOL, unlike v2's pointer, and the difference is the keying
	// law rather than a relaxation. v2's is a pointer only so a MONSTER that
	// wrote `holdable: false` can be told it cannot declare that at all; this
	// block is keyed by a prop id, so there is no monster here to refuse and
	// nothing for the pointer to distinguish. What is left is
	// [encounter.PropInput.Holdable]'s own rule — "there is only one thing
	// 'said nothing about holdable' can mean: a thing nobody declared
	// holdable stays scenery" — which is exactly why
	// [RoomDoorBinding.Closed] is a plain bool too.
	Holdable bool `yaml:"holdable,omitempty" json:"holdable,omitempty"`

	// Holds is the intel records this prop carries, by record id —
	// [RoomMonsterBinding.Holds]'s field on the other holder, with the same
	// refusal ([PlaceSpec.Holds], R6).
	//
	// THE RECORD STAYS ON THE PROP. Picking it up teaches whoever holds it
	// and the paper does not stop saying what it says, so a scroll handed on
	// teaches the next holder too. That is why intel does not need a fight to
	// test: "if we want to test the intel we need to be able to place it on a
	// few things — not the hardest monster to kill in the game."
	//
	// A prop that carries records need NOT be Holdable. Authoring one nobody
	// can pick up is inert, not an error.
	Holds []string `yaml:"holds,omitempty" json:"holds,omitempty"`

	// Arrives is the predicate that brings this prop onto the floor —
	// [RoomMonsterBinding.Arrives]'s field on a prop, the same
	// [PredicateSpec] and the same liveness rule. Optional: absent means it
	// is there from the first frame.
	//
	// A PROP THAT ARRIVES IS NOWHERE UNTIL ITS PREDICATE HOLDS, exactly as a
	// creature is. The authored scene still carries its node — presentation
	// is content — and what is on the floor is the engine's `placed` answer,
	// never the scene (R1).
	Arrives *PredicateSpec `yaml:"arrives,omitempty" json:"arrives,omitempty"`
}

// RoomDoorBinding is one placed item's DOOR STATE: that the item is a door at
// all, and what state it is resting in (rpg-project#485, R2).
//
// The THIRD declaration kind on a placed thing, after `propDeclarations` and
// `monsterBindings` and before `propBindings`, keyed the same way: by the id
// of the thing it is about. An id in BOTH this block and `propBindings` is a
// door somebody picks up, and is refused by name until a use case says what
// that means (rpg-project#488, R1).
//
//	doorBindings:
//	  cellar-door: { closed: true }
//	  gate-1:      { closed: true, locked: [{ ability: str, dc: 15 }] }
//
// # It is v2's door state, and only that
//
// The keys are [DoorSpec]'s with `at` removed: the same [CheckSpec], the same
// nil-vs-empty rule, the same sentences, judged by the one shared grammar
// (grammar.go). `at` is the GEOMETRY, and the geometry is the dialect's own
// to supply — a v2 door is a position on a wall, and this dialect has no
// walls to put one on. Here the door's shape is its `propDeclarations` entry:
// the same footprint every other placed thing gets, with the door's state
// deciding what it blocks instead of two authored flags.
//
// # Where `concealed` went
//
// It was a key here, refused by name, while a hidden rectangle in the middle
// of a room was a picture question nobody had answered. The answer is the
// concealment primitive (rpg-project#490): the key MOVED to the root, where
// a door is hidden by being listed in a `concealments.<id>.props` beside the
// cells and the things it hides. It is not a key on this block at all any
// more, so an author who writes it gets the unknown-key refusal with the
// keys this block does take — which is the right sentence now, because the
// word belongs somewhere else rather than nowhere.
type RoomDoorBinding struct {
	// Closed is whether the door is RESTING shut. Optional; absent is an open
	// doorway, which is [DoorSpec.Closed]'s own rule — absence is a real
	// state here rather than an unanswered question, which is why this key is
	// not required the way a declaration's two blocking flags are.
	//
	// Ignored when Locked is set: a locked door is shut by definition.
	Closed bool `yaml:"closed,omitempty" json:"closed,omitempty"`

	// Locked, when present, makes the door locked behind the check it
	// carries. [DoorSpec.Locked]'s shape and its law: NIL IS "NOT LOCKED",
	// and `locked: []` is an authored lock that forgot to say how it is
	// beaten — refused by name.
	Locked CheckSpec `yaml:"locked,omitempty" json:"locked,omitempty"`
}

// ConcealmentSpec is one secret this room hides: what finds it, and what it
// hides (rpg-project#490, R1).
//
//	concealments:
//	  vault:
//	    notice: [{ ability: investigation, dc: 12 }]
//	    checks: [{ ability: perception, dc: 17 }, { ability: investigation, dc: 15 }]
//	    cells:  [{ q: 12, r: 3 }, { q: 13, r: 3 }]
//	    props:  [vault-door, inner-wall-1, heirloom]
//
// # The cells stay in `walkableHexes` (R2)
//
// The author sees the vault at the table; an unaware player is not sent those
// cells, and the web draws wall on the boundary exactly as it does for any
// edge of the walkable set. Nobody places a wall to hide a room — omission is
// the wall. Whether a listed cell is walkable is the ENGINE's refusal at
// publish, not the web's: the web carries the list and the engine grades it,
// which is the same split `doorBindings` keeps.
type ConcealmentSpec struct {
	// Notice is the PASSIVE tell: the same approach list as Checks,
	// resolved without dice against an observer's passive score
	// (rpg-project#490, R6). Optional, and the NIL-VS-EMPTY law is
	// [RoomDoorBinding.Locked]'s — nil is "no passive tell", and
	// `notice: []` is an author who said there IS one and did not say what
	// beats it, refused by name.
	//
	// CARRIED AND UNREAD IN THIS SLICE ([encounter.ConcealmentInput.Notice]):
	// the passive pass is slice 2, and the key is here so an author can
	// write it today and so the slice that reads it changes no file.
	Notice CheckSpec `yaml:"notice,omitempty" json:"notice,omitempty"`

	// Checks are the ways a Search can find this, priced per route —
	// [DoorSpec.Locked]'s shape, beaten by any listed one. REQUIRED
	// non-empty: a secret nobody can ever find is one the author started and
	// did not finish.
	Checks CheckSpec `yaml:"checks,omitempty" json:"checks,omitempty"`

	// Cells are the hexes this hides, in this dialect's axial frame —
	// [RoomPartyStart]'s shape, for [RoomExit.Cell]'s reason. Every one must
	// be a `walkableHexes` cell, and no cell may be in two concealments.
	// Optional: a concealment that hides only placed things lists none.
	Cells []RoomCell `yaml:"cells,omitempty" json:"cells,omitempty"`

	// Props are the placed things this hides, by the id
	// `propDeclarations` declares them under — doors, wall props, anything.
	// Every one must be a declared prop, and no id may be in two
	// concealments. Optional.
	//
	// A DOOR IS JUST A PLACED ID HERE, because a door IS a prop in this
	// dialect (single_room_doors.go: "a door here is a prop plus a state").
	// The lowering sorts the ids into the engine's two lists by asking
	// `doorBindings` which of them are doors; the author never writes the
	// distinction twice.
	Props []string `yaml:"props,omitempty" json:"props,omitempty"`
}

// RoomFootprint describes a prop's movement-blocking footprint.
type RoomFootprint struct {
	Width   float64 `yaml:"width" json:"width"`
	Depth   float64 `yaml:"depth" json:"depth"`
	OffsetX float64 `yaml:"offsetX" json:"offsetX"`
	OffsetZ float64 `yaml:"offsetZ" json:"offsetZ"`
}

// RoomPropDeclaration describes the gameplay effects of a scene prop.
type RoomPropDeclaration struct {
	BlocksMovement    *bool         `yaml:"blocksMovement" json:"blocksMovement"`
	BlocksLineOfSight *bool         `yaml:"blocksLineOfSight" json:"blocksLineOfSight"`
	Footprint         RoomFootprint `yaml:"footprint" json:"footprint"`
}

// RoomGameplaySource contains the room's walkable cells and placements.
type RoomGameplaySource struct {
	ImplicitRegionID        string                                    `yaml:"implicitRegionId" json:"implicitRegionId"`
	WalkableHexes           []RoomCell                                `yaml:"walkableHexes" json:"walkableHexes"`
	PropDeclarations        map[string]RoomPropDeclaration            `yaml:"propDeclarations" json:"propDeclarations"`
	ArrangementDeclarations map[string]map[string]RoomPropDeclaration `yaml:"arrangementDeclarations" json:"arrangementDeclarations"`
	PartyStart              *RoomCell                                 `yaml:"partyStart,omitempty" json:"partyStart,omitempty"`
	MonsterDeclarations     []RoomMonsterSource                       `yaml:"monsterDeclarations" json:"monsterDeclarations"`

	// MonsterBindings is each creature's orders, under its stable id.
	// Optional, and ABSENT WHEN NOBODY HAS ANY: a room whose creatures
	// override nothing writes the bytes it wrote before this key existed.
	MonsterBindings map[string]RoomMonsterBinding `yaml:"monsterBindings,omitempty" json:"monsterBindings,omitempty"`

	// DoorBindings is which placed items are doors, and the state each one
	// rests in, under its stable id (rpg-project#485). Optional, and ABSENT
	// WHEN THE ROOM HAS NO DOORS — a room that declares none writes exactly
	// the bytes it wrote before this key existed.
	//
	// EVERY ID HERE ALSO NEEDS A PROP DECLARATION, because that is where the
	// door's footprint comes from; a binding without one is refused by name.
	DoorBindings map[string]RoomDoorBinding `yaml:"doorBindings,omitempty" json:"doorBindings,omitempty"`

	// PropBindings is what each placed prop DOES — holdable, what it carries,
	// whether it is here yet — under its stable id (rpg-project#488, R1). The
	// FOURTH declaration kind, under the same keying law as the three above.
	// Optional, and ABSENT WHEN NO PROP HAS ORDERS: a room that declares none
	// writes exactly the bytes it wrote before this key existed.
	//
	// EVERY ID HERE ALSO NEEDS A PROP DECLARATION, for [RoomDoorBinding]'s
	// reason one kind over, and may not be a door or an arrangement template.
	// See [RoomPropBinding] for why this block decodes and does not yet
	// compile.
	PropBindings map[string]RoomPropBinding `yaml:"propBindings,omitempty" json:"propBindings,omitempty"`
}

// SingleRoomDecodeInput supplies YAML source for decoding.
type SingleRoomDecodeInput struct{ Source []byte }

// SingleRoomDecodeResult contains the decoded single-room specification.
type SingleRoomDecodeResult struct{ Spec *SingleRoomSpec }
