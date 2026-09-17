package dungeonspec

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

type SingleRoomSpec struct {
	Version int            `yaml:"version" json:"version"`
	Key     string         `yaml:"key" json:"key"`
	Play    SingleRoomPlay `yaml:"play" json:"play"`
	Room    RoomSource     `yaml:"room" json:"room"`
}
type SingleRoomPlay struct {
	Void     string `yaml:"void" json:"void"`
	Lighting string `yaml:"lighting" json:"lighting"`
	Standing string `yaml:"standing" json:"standing"`
}
type RoomSource struct {
	Version         int                          `yaml:"version" json:"version"`
	ID              string                       `yaml:"id" json:"id"`
	Name            string                       `yaml:"name" json:"name"`
	CoordinateFrame encounter.RoomSceneFrame     `yaml:"coordinateFrame" json:"coordinateFrame"`
	Workspace       encounter.RoomSceneWorkspace `yaml:"workspace" json:"workspace"`
	Scene           encounter.RoomVisualScene    `yaml:"scene" json:"scene"`
	Gameplay        RoomGameplaySource           `yaml:"room" json:"room"`
}
type RoomCell struct {
	Q int `yaml:"q" json:"q"`
	R int `yaml:"r" json:"r"`
}
type RoomPartyStart = RoomCell
type RoomMonsterSource struct {
	ID   string   `yaml:"id" json:"id"`
	Ref  string   `yaml:"ref" json:"ref"`
	Cell RoomCell `yaml:"cell" json:"cell"`
}
type RoomFootprint struct {
	Width   float64 `yaml:"width" json:"width"`
	Depth   float64 `yaml:"depth" json:"depth"`
	OffsetX float64 `yaml:"offsetX" json:"offsetX"`
	OffsetZ float64 `yaml:"offsetZ" json:"offsetZ"`
}
type RoomPropDeclaration struct {
	BlocksMovement    *bool         `yaml:"blocksMovement" json:"blocksMovement"`
	BlocksLineOfSight *bool         `yaml:"blocksLineOfSight" json:"blocksLineOfSight"`
	Footprint         RoomFootprint `yaml:"footprint" json:"footprint"`
}
type RoomGameplaySource struct {
	ImplicitRegionID        string                                    `yaml:"implicitRegionId" json:"implicitRegionId"`
	WalkableHexes           []RoomCell                                `yaml:"walkableHexes" json:"walkableHexes"`
	PropDeclarations        map[string]RoomPropDeclaration            `yaml:"propDeclarations" json:"propDeclarations"`
	ArrangementDeclarations map[string]map[string]RoomPropDeclaration `yaml:"arrangementDeclarations" json:"arrangementDeclarations"`
	PartyStart              *RoomCell                                 `yaml:"partyStart,omitempty" json:"partyStart,omitempty"`
	Monsters                []RoomMonsterSource                       `yaml:"monsters" json:"monsters"`
}
type SingleRoomDecodeInput struct{ Source []byte }
type SingleRoomDecodeResult struct{ Spec *SingleRoomSpec }
