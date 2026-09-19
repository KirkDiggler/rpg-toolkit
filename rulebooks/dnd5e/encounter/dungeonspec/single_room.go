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
// decoded document gets the author's own bytes back. They are NOT JSON
// fields: a yaml.Node has no honest JSON shape, and a package that does not
// model the presentation has nothing to put there.
type RoomSource struct {
	Version int    `yaml:"version" json:"version"`
	ID      string `yaml:"id" json:"id"`
	Name    string `yaml:"name" json:"name"`

	CoordinateFrame yaml.Node `yaml:"coordinateFrame" json:"-"`
	Workspace       yaml.Node `yaml:"workspace" json:"-"`
	Scene           yaml.Node `yaml:"scene" json:"-"`

	Gameplay RoomGameplaySource `yaml:"room" json:"room"`
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
// The THIRD declaration kind on a placed thing, after `propDeclarations` and
// ahead of the proposed `doorBindings`, and keyed the same way: by the id of
// the thing it is about. A binding naming a creature no `monsters:` entry
// declares is refused exactly as a prop declaration with no live owner is —
// a declaration can never outlive the thing it names.
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
}

// SingleRoomDecodeInput supplies YAML source for decoding.
type SingleRoomDecodeInput struct{ Source []byte }

// SingleRoomDecodeResult contains the decoded single-room specification.
type SingleRoomDecodeResult struct{ Spec *SingleRoomSpec }
