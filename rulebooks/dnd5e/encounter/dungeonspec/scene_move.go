package dungeonspec

// SceneMoveSpec names the guards to dismiss and optional arrival anchors.
type SceneMoveSpec struct {
	Guards  []string   `yaml:"guards" json:"guards"`
	Anchors []RoomCell `yaml:"anchors,omitempty" json:"anchors,omitempty"`
	Leave   string     `yaml:"leave" json:"leave"`
}
