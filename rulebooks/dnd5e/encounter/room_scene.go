package encounter

// RoomSceneFrame is the portable authoring coordinate declaration.
type RoomSceneFrame struct {
	HorizontalPlane string  `yaml:"horizontalPlane" json:"horizontalPlane"`
	VerticalAxis    string  `yaml:"verticalAxis" json:"verticalAxis"`
	DistanceUnit    string  `yaml:"distanceUnit" json:"distanceUnit"`
	HexRadius       float64 `yaml:"hexRadius" json:"hexRadius"`
	FootprintFrame  string  `yaml:"footprintFrame" json:"footprintFrame"`
}

type RoomSceneWorkspace struct {
	HexRadius       float64 `yaml:"hexRadius" json:"hexRadius"`
	HorizontalLimit float64 `yaml:"horizontalLimit" json:"horizontalLimit"`
}
type RoomSceneTransform struct {
	X         float64 `yaml:"x" json:"x"`
	Y         float64 `yaml:"y" json:"y"`
	Z         float64 `yaml:"z" json:"z"`
	RotationY float64 `yaml:"rotationY" json:"rotationY"`
}
type RoomSceneOffset struct {
	X float64 `yaml:"x" json:"x"`
	Y float64 `yaml:"y" json:"y"`
	Z float64 `yaml:"z" json:"z"`
}
type RoomSceneLight struct {
	Enabled   bool            `yaml:"enabled" json:"enabled"`
	Offset    RoomSceneOffset `yaml:"offset" json:"offset"`
	Color     string          `yaml:"color" json:"color"`
	Intensity float64         `yaml:"intensity" json:"intensity"`
	Range     float64         `yaml:"range" json:"range"`
}
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
type RoomSceneGroup struct {
	Kind      string             `yaml:"kind" json:"kind"`
	ID        string             `yaml:"id" json:"id"`
	Label     string             `yaml:"label" json:"label"`
	ParentID  string             `yaml:"parentId,omitempty" json:"parentId,omitempty"`
	Transform RoomSceneTransform `yaml:"transform" json:"transform"`
}
type RoomVisualScene struct {
	Version int              `yaml:"version" json:"version"`
	ID      string           `yaml:"id" json:"id"`
	Name    string           `yaml:"name" json:"name"`
	Items   []RoomSceneItem  `yaml:"items" json:"items"`
	Groups  []RoomSceneGroup `yaml:"groups" json:"groups"`
}
type RoomScenePresentation struct {
	Version   int                `yaml:"version" json:"version"`
	Frame     RoomSceneFrame     `yaml:"coordinateFrame" json:"coordinateFrame"`
	Workspace RoomSceneWorkspace `yaml:"workspace" json:"workspace"`
	Scene     RoomVisualScene    `yaml:"scene" json:"scene"`
}
