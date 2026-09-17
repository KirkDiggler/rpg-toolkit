package encounter

// room_scene.go is THE ROOM SCENE PRESENTATION (issue #1753): the typed
// value the complete World Builder source carries beside its gameplay
// geometry, and the one shape the runtime, the blob and the atlas all speak.
//
// The types are the portable authoring document, verbatim: visual scene,
// groups, supports, point lights, height scales, the coordinate frame and
// the workspace. Nothing here is gameplay truth — no verb, fold or refusal
// reads any of it — but every carrier of it owes the author fidelity:
// doubles stay doubles and an authored empty list stays an empty list, so
// [copyRoomScene] is the ONE deep copy every snapshot goes through and
// [ValidateRoomScene] (room_scene_validate.go) is the ONE validator, the
// same function dungeonspec's decoder delegates to.
//
// A nil presentation is legal wherever it appears and means a field without
// one — every field authored before v3 — and stays nil on every carrier it
// rides (see FieldData.RoomScene, Atlas.RoomScene).

// Approved room-scene item kinds. The editor's vocabulary, owned beside the
// validator that enforces it (room_scene_validate.go) so no second spelling
// of the words can drift from it.
const (
	RoomSceneKindProp  = "prop"
	RoomSceneKindGroup = "group"
)

// copyRoomScene deep-copies a presentation: every slice, and the two pointer
// leaves (HeightScale, PointLight) fresh per item. Doubles are float64
// copies — no narrowing, no rounding, and an authored empty Items or Groups
// list stays an empty list rather than collapsing to nil, the same fact it
// was.
//
// THE ONE COPY. Construction, ToData, Load and the atlas all snapshot
// through it, so no caller-owned pointer or slice can reach a running field
// and no two snapshots can alias each other (the PlacedPropInput box copy's
// rule, one level up).
func copyRoomScene(p *RoomScenePresentation) *RoomScenePresentation {
	if p == nil {
		return nil
	}
	out := *p
	out.Scene.Items = make([]RoomSceneItem, len(p.Scene.Items))
	copy(out.Scene.Items, p.Scene.Items)
	for i := range out.Scene.Items {
		if p.Scene.Items[i].HeightScale != nil {
			v := *p.Scene.Items[i].HeightScale
			out.Scene.Items[i].HeightScale = &v
		}
		if p.Scene.Items[i].PointLight != nil {
			v := *p.Scene.Items[i].PointLight
			out.Scene.Items[i].PointLight = &v
		}
	}
	out.Scene.Groups = make([]RoomSceneGroup, len(p.Scene.Groups))
	copy(out.Scene.Groups, p.Scene.Groups)

	return &out
}

// RoomSceneFrame is the portable authoring coordinate declaration.
type RoomSceneFrame struct {
	HorizontalPlane string  `yaml:"horizontalPlane" json:"horizontalPlane"`
	VerticalAxis    string  `yaml:"verticalAxis" json:"verticalAxis"`
	DistanceUnit    string  `yaml:"distanceUnit" json:"distanceUnit"`
	HexRadius       float64 `yaml:"hexRadius" json:"hexRadius"`
	FootprintFrame  string  `yaml:"footprintFrame" json:"footprintFrame"`
}

// RoomSceneWorkspace defines the playable room bounds.
type RoomSceneWorkspace struct {
	HexRadius       float64 `yaml:"hexRadius" json:"hexRadius"`
	HorizontalLimit float64 `yaml:"horizontalLimit" json:"horizontalLimit"`
}

// RoomSceneTransform is a scene object's position and rotation.
type RoomSceneTransform struct {
	X         float64 `yaml:"x" json:"x"`
	Y         float64 `yaml:"y" json:"y"`
	Z         float64 `yaml:"z" json:"z"`
	RotationY float64 `yaml:"rotationY" json:"rotationY"`
}

// RoomSceneOffset is a light's local offset from its item.
type RoomSceneOffset struct {
	X float64 `yaml:"x" json:"x"`
	Y float64 `yaml:"y" json:"y"`
	Z float64 `yaml:"z" json:"z"`
}

// RoomSceneLight describes an optional point light on a scene item.
type RoomSceneLight struct {
	Enabled   bool            `yaml:"enabled" json:"enabled"`
	Offset    RoomSceneOffset `yaml:"offset" json:"offset"`
	Color     string          `yaml:"color" json:"color"`
	Intensity float64         `yaml:"intensity" json:"intensity"`
	Range     float64         `yaml:"range" json:"range"`
}

// RoomSceneItem is a visual prop in a room scene.
type RoomSceneItem struct {
	Kind        string             `yaml:"kind" json:"kind"`
	ID          string             `yaml:"id" json:"id"`
	Label       string             `yaml:"label" json:"label"`
	AssetRef    string             `yaml:"assetRef" json:"assetRef"`
	Transform   RoomSceneTransform `yaml:"transform" json:"transform"`
	ParentID    string             `yaml:"parentId,omitempty" json:"parentId,omitempty"`
	SupportID   string             `yaml:"supportId,omitempty" json:"supportId,omitempty"`
	HeightScale *float64           `yaml:"heightScale,omitempty" json:"heightScale,omitempty"`
	PointLight  *RoomSceneLight    `yaml:"pointLight,omitempty" json:"pointLight,omitempty"`
}

// RoomSceneGroup is a transformable visual grouping in a room scene.
type RoomSceneGroup struct {
	Kind      string             `yaml:"kind" json:"kind"`
	ID        string             `yaml:"id" json:"id"`
	Label     string             `yaml:"label" json:"label"`
	ParentID  string             `yaml:"parentId,omitempty" json:"parentId,omitempty"`
	Transform RoomSceneTransform `yaml:"transform" json:"transform"`
}

// RoomVisualScene contains the authored items and groups for a room.
type RoomVisualScene struct {
	Version int              `yaml:"version" json:"version"`
	ID      string           `yaml:"id" json:"id"`
	Name    string           `yaml:"name" json:"name"`
	Items   []RoomSceneItem  `yaml:"items" json:"items"`
	Groups  []RoomSceneGroup `yaml:"groups" json:"groups"`
}

// RoomScenePresentation carries a room scene with its coordinate declarations.
type RoomScenePresentation struct {
	Version   int                `yaml:"version" json:"version"`
	Frame     RoomSceneFrame     `yaml:"coordinateFrame" json:"coordinateFrame"`
	Workspace RoomSceneWorkspace `yaml:"workspace" json:"workspace"`
	Scene     RoomVisualScene    `yaml:"scene" json:"scene"`
}
