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
// # The third root key, and the same reason
//
// [SingleRoomSpec.Intel] joined them for rule 3 of the placement law
// (rpg-project#488): what is NOT PLACED lives at the root. A record of
// knowledge is a thing in the file like a door — declared once, referred to
// by name from whatever carries it ([RoomMonsterBinding.Holds]) — and it
// stands nowhere, so there is no placed thing to key it by.
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
	// ONE TARGET IN THIS DIALECT, AND IT IS `fact`. [RevealsSpec.Door] is
	// REFUSED by name at `intel[<i>].reveals.door` (rpg-project#488 R3), in
	// the shape [RoomDoorBinding.Concealed]'s refusal takes: revealing the
	// way to a door means something, it just needs a CONCEALED door on a
	// crossing to mean it, and a single room has no crossing to hide one on.
	// The field stays on the shared shape so the refusal can be a sentence at
	// the author's own path instead of "not a key this build reads".
	Intel []IntelSpec `yaml:"intel,omitempty" json:"intel,omitempty"`

	Room RoomSource `yaml:"room" json:"room"`
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

// RoomMonsterSource declares a monster, its authored placement cell, and the
// side it is on.
//
// THE ACTOR CARRIES IDENTITY; THE BINDING CARRIES ORDERS (rpg-project#477,
// Decision 4). Everything a faction SUPPLIES — its table, its temperament,
// its arms — is overridable and lives in [RoomMonsterBinding]. `faction` is
// what SELECTS those defaults, nothing overrides it, and membership must not
// require a binding: four goblins in one faction with no overrides would
// otherwise need four orders blocks that exist only to record who they are.
type RoomMonsterSource struct {
	ID   string   `yaml:"id" json:"id"`
	Ref  string   `yaml:"ref" json:"ref"`
	Cell RoomCell `yaml:"cell" json:"cell"`

	// Faction is the faction this monster is in, AS AUTHORED. Optional, and
	// ABSENT WHEN UNAUTHORED — never written out as `faction: monsters`.
	// `factionOf` (encounter/field.go) stores it as given and resolves the
	// kind's default on every read, so a creature whose author named no side
	// persists byte-identically to one from before factions existed. A
	// faction this document does not declare is refused by name, and `party`
	// is refused as the players' side.
	Faction string `yaml:"faction,omitempty" json:"faction,omitempty"`
}

// RoomMonsterBinding is one creature's ORDERS: what it does, what it is like,
// and what it fights with (rpg-project#477, Decision 4).
//
// One of the FOUR declaration kinds on a placed thing, beside
// `propDeclarations`, `doorBindings` and `propBindings`, and keyed the same
// way: by the id of the thing it is about. A binding naming a creature no
// `monsters:` entry declares is refused exactly as a prop declaration with no
// live owner is — a declaration can never outlive the thing it names.
//
// EVERY FIELD IS OPTIONAL, and the common state of this whole block is
// absence: a creature with nothing to override needs no binding at all.
type RoomMonsterBinding struct {
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
	// `monsters:` entry (rule 1). "The captain is not a role: it is a monster
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
	// OMITTED MEANS DERIVED, NOT UNGATED: absent, the rulebook rolls
	// Intimidation against the stat block's own passive Insight. The
	// NIL-VS-EMPTY LAW IS [RoomDoorBinding.Locked]'s — nil is "the author
	// said nothing", and `intimidate: []` is an authored check that forgot to
	// say how it is beaten, refused by name.
	Intimidate CheckSpec `yaml:"intimidate,omitempty" json:"intimidate,omitempty"`

	// Persuade is [RoomMonsterBinding.Intimidate]'s twin — the check to talk
	// this creature round ([PlaceSpec.Persuade], rpg-project#458;
	// rpg-project#488 R5). The same shape, the same derived default when
	// absent, and the same nil-vs-empty law.
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
// # What it does NOT take
//
// `concealed` is refused by name at its own path, in the shape the `at:`
// refusal takes ([singleRoomCellSelector]): the concealed-door laws are
// written for a door on a crossing, and a hidden rectangle standing in the
// middle of a room is a picture question the World Builder has not asked yet.
// It is a field here rather than an unknown key so the author gets that
// sentence at `…doorBindings.<id>.concealed` instead of "not a key this build
// reads" — the word means something, it is simply not built.
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

	// Concealed is refused in this dialect (R2). Carried on the shape only
	// so the refusal can be a sentence at this key's own path; nothing reads
	// it, and nothing compiles it.
	Concealed CheckSpec `yaml:"concealed,omitempty" json:"concealed,omitempty"`
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
	Monsters                []RoomMonsterSource                       `yaml:"monsters" json:"monsters"`

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
