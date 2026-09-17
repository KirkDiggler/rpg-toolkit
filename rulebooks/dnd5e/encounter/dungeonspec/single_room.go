package dungeonspec

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// SingleRoomSpec is the v3 authoring document for one playable room.
type SingleRoomSpec struct {
	Version int            `yaml:"version" json:"version"`
	Key     string         `yaml:"key" json:"key"`
	Play    SingleRoomPlay `yaml:"play" json:"play"`
	Room    RoomSource     `yaml:"room" json:"room"`
}

// SingleRoomPlay declares the runtime assumptions required by a single room.
type SingleRoomPlay struct {
	Void     string `yaml:"void" json:"void"`
	Lighting string `yaml:"lighting" json:"lighting"`
	Standing string `yaml:"standing" json:"standing"`
}

// RoomSource contains a room's presentation and gameplay authoring data.
type RoomSource struct {
	Version         int                          `yaml:"version" json:"version"`
	ID              string                       `yaml:"id" json:"id"`
	Name            string                       `yaml:"name" json:"name"`
	CoordinateFrame encounter.RoomSceneFrame     `yaml:"coordinateFrame" json:"coordinateFrame"`
	Workspace       encounter.RoomSceneWorkspace `yaml:"workspace" json:"workspace"`
	Scene           encounter.RoomVisualScene    `yaml:"scene" json:"scene"`
	Gameplay        RoomGameplaySource           `yaml:"room" json:"room"`
}

// RoomCell is an authored axial hex coordinate.
type RoomCell struct {
	Q int `yaml:"q" json:"q"`
	R int `yaml:"r" json:"r"`
}

// RoomPartyStart identifies the authored party start cell.
type RoomPartyStart = RoomCell

// RoomMonsterSource declares a monster and its authored placement cell.
type RoomMonsterSource struct {
	ID   string   `yaml:"id" json:"id"`
	Ref  string   `yaml:"ref" json:"ref"`
	Cell RoomCell `yaml:"cell" json:"cell"`
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
}

// SingleRoomDecodeInput supplies YAML source for decoding.
type SingleRoomDecodeInput struct{ Source []byte }

// SingleRoomDecodeResult contains the decoded single-room specification.
type SingleRoomDecodeResult struct{ Spec *SingleRoomSpec }
