package dungeonspec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ActivityCueSpec is presentation intent on a spoken answer, not a table word.
// References stay opaque; the consumer checks support for the actual body.
type ActivityCueSpec struct {
	// Ref is the author's opaque, nonempty activity reference.
	Ref string `yaml:"ref" json:"ref"`
	// Cycles is the positive number of complete sustain cycles.
	Cycles uint32 `yaml:"cycles" json:"cycles"`
}

// UnmarshalYAML refuses missing, mistyped and unknown cue fields. In particular,
// a fractional YAML number cannot be truncated into a sustain count.
func (s *ActivityCueSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: activity must be a mapping", node.Line)
	}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		switch key.Value {
		case "ref":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" || value.Value == "" {
				return fmt.Errorf("line %d: activity.ref must be a nonempty string", value.Line)
			}
		case "cycles":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!int" {
				return fmt.Errorf("line %d: activity.cycles must be a positive whole number", value.Line)
			}
		default:
			return fmt.Errorf("line %d: unknown activity field %q", key.Line, key.Value)
		}
	}
	type body ActivityCueSpec
	var value body
	if err := node.Decode(&value); err != nil {
		return fmt.Errorf("activity: %w", err)
	}
	if value.Ref == "" || value.Cycles == 0 {
		return fmt.Errorf("line %d: activity requires ref and positive cycles", node.Line)
	}
	*s = ActivityCueSpec(value)
	return nil
}
