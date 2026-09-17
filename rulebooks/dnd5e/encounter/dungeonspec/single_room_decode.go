package dungeonspec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"gopkg.in/yaml.v3"
)

// DecodeSingleRoom strictly reads exactly one lossless dungeon YAML document.
func DecodeSingleRoom(in SingleRoomDecodeInput) (*SingleRoomDecodeResult, error) {
	if len(bytes.TrimSpace(in.Source)) == 0 {
		return nil, singleRoomErrors("source: empty document")
	}
	dec := yaml.NewDecoder(bytes.NewReader(in.Source))
	dec.KnownFields(true)
	var spec SingleRoomSpec
	if err := dec.Decode(&spec); err != nil {
		return nil, singleRoomErrors(err.Error())
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, singleRoomErrors("source: more than one YAML document")
		}
		return nil, singleRoomErrors(err.Error())
	}
	var root yaml.Node
	if err := yaml.Unmarshal(in.Source, &root); err != nil {
		return nil, singleRoomErrors(err.Error())
	}
	errs := validateSingleRoom(&spec, &root)
	errs = append(errs, requiredSingleRoomFields(&root)...)
	if len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	return &SingleRoomDecodeResult{Spec: &spec}, nil
}
func singleRoomErrors(msg string) error {
	return &ValidationError{Errors: []FieldError{{Message: msg}}}
}
func validateSingleRoom(s *SingleRoomSpec, root *yaml.Node) []FieldError {
	var e []FieldError
	add := func(p, m string) { e = append(e, FieldError{Path: p, Message: m}) }
	if s.Version != 3 {
		add("version", fmt.Sprintf("unsupported version %d (want 3)", s.Version))
	}
	if s.Key == "" {
		add("key", "is required")
	}
	if s.Play.Void != "transparent" {
		add("play.void", "must be transparent")
	}
	if s.Play.Lighting != "bright" {
		add("play.lighting", "must be bright")
	}
	if s.Play.Standing != "centre-covered" {
		add("play.standing", "must be centre-covered")
	}
	if s.Room.Version != 3 {
		add("room.version", fmt.Sprintf("unsupported version %d (want 3)", s.Room.Version))
	}
	if s.Room.ID == "" {
		add("room.id", "is required")
	}
	if s.Room.Name == "" {
		add("room.name", "is required")
	}
	if s.Room.Scene.Version != 1 {
		add("room.scene.version", "unsupported version")
	}
	if s.Room.Scene.ID == "" {
		add("room.scene.id", "is required")
	}
	if s.Room.Gameplay.ImplicitRegionID == "" {
		add("room.room.implicitRegionId", "is required")
	}
	f := s.Room.CoordinateFrame
	if f.HorizontalPlane != "world-xz" {
		add("room.coordinateFrame.horizontalPlane", "must be world-xz")
	}
	if f.VerticalAxis != "world-y-up" {
		add("room.coordinateFrame.verticalAxis", "must be world-y-up")
	}
	if f.DistanceUnit != "world-scene-unit" {
		add("room.coordinateFrame.distanceUnit", "must be world-scene-unit")
	}
	if f.FootprintFrame != "owner-local-xz" {
		add("room.coordinateFrame.footprintFrame", "must be owner-local-xz")
	}
	if !finitePositive(f.HexRadius) || f.HexRadius != 1 {
		add("room.coordinateFrame.hexRadius", "must be 1")
	}
	if !finitePositive(s.Room.Workspace.HexRadius) || !finitePositive(s.Room.Workspace.HorizontalLimit) || s.Room.Workspace.HexRadius != 6 || s.Room.Workspace.HorizontalLimit != 12 {
		add("room.workspace", "unsupported workspace preset")
	}
	ids := map[string]string{}
	for i, it := range s.Room.Scene.Items {
		p := fmt.Sprintf("room.scene.items[%d]", i)
		if it.ID == "" {
			add(p+".id", "is required")
		}
		if _, ok := ids[it.ID]; ok {
			add(p+".id", "duplicate id")
		}
		ids[it.ID] = p
		if it.Kind != "prop" {
			add(p+".kind", "must be prop")
		}
		if it.AssetRef == "" {
			add(p+".assetRef", "is required")
		}
		if it.HeightScale != nil && (*it.HeightScale < .25 || *it.HeightScale > 4 || math.IsNaN(*it.HeightScale) || math.IsInf(*it.HeightScale, 0)) {
			add(p+".heightScale", "must be between 0.25 and 4")
		}
		finiteTransform(it.Transform, p, &e)
		if it.PointLight != nil {
			finiteOffset(it.PointLight.Offset, p, &e)
			if it.PointLight.Color == "" {
				add(p+".pointLight.color", "is required")
			}
			if !validHexColor(it.PointLight.Color) {
				add(p+".pointLight.color", "must be a hex color")
			}
			if math.IsNaN(it.PointLight.Intensity) || math.IsInf(it.PointLight.Intensity, 0) || math.IsNaN(it.PointLight.Range) || math.IsInf(it.PointLight.Range, 0) {
				add(p+".pointLight", "numbers must be finite")
			}
		}
	}
	for i, g := range s.Room.Scene.Groups {
		p := fmt.Sprintf("room.scene.groups[%d]", i)
		if g.ID == "" {
			add(p+".id", "is required")
		}
		if _, ok := ids[g.ID]; ok {
			add(p+".id", "duplicate id")
		}
		ids[g.ID] = p
		if g.Kind != "group" {
			add(p+".kind", "must be group")
		}
		finiteTransform(g.Transform, p, &e)
	}
	for i, it := range s.Room.Scene.Items {
		p := fmt.Sprintf("room.scene.items[%d]", i)
		if it.ParentID != "" {
			if g, ok := ids[it.ParentID]; !ok || !strings.Contains(g, "groups") {
				add(p+".parentId", "dangling group id")
			}
		}
		if it.SupportID != "" {
			if x, ok := ids[it.SupportID]; !ok || !strings.Contains(x, "items") {
				add(p+".supportId", "dangling prop id")
			}
		}
	}
	seen := map[RoomCell]string{}
	for i, c := range s.Room.Gameplay.WalkableHexes {
		p := fmt.Sprintf("room.room.walkableHexes[%d]", i)
		if _, ok := seen[c]; ok {
			add(p, "duplicate cell")
		}
		seen[c] = p
	}
	for k, d := range s.Room.Gameplay.PropDeclarations {
		if d.BlocksMovement == nil {
			add("room.room.propDeclarations."+k+".blocksMovement", "is required")
		}
		if d.BlocksLineOfSight == nil {
			add("room.room.propDeclarations."+k+".blocksLineOfSight", "is required")
		}
		if d.Footprint.Width <= 0 || d.Footprint.Depth <= 0 {
			add("room.room.propDeclarations."+k+".footprint", "dimensions must be positive")
		}
	}
	mon := map[string]bool{}
	for i, m := range s.Room.Gameplay.Monsters {
		p := fmt.Sprintf("room.room.monsters[%d]", i)
		if mon[m.ID] {
			add(p+".id", "duplicate id")
		}
		mon[m.ID] = true
		if m.ID == "" {
			add(p+".id", "is required")
		}
		parsed, err := core.ParseString(m.Ref)
		if err != nil {
			add(p+".ref", "invalid ref: "+err.Error())
		} else if parsed.Type != "monsters" {
			add(p+".ref", "must reference monsters")
		}
	}
	for k, d := range s.Room.Gameplay.PropDeclarations {
		if !sceneProp(ids, k) {
			add("room.room.propDeclarations."+k, "must name a live scene prop")
		}
		checkFootprint(d.Footprint, "room.room.propDeclarations."+k+".footprint", &e)
	}
	for k, templates := range s.Room.Gameplay.ArrangementDeclarations {
		for tk, d := range templates {
			checkFootprint(d.Footprint, "room.room.arrangementDeclarations."+k+"."+tk+".footprint", &e)
		}
	}
	for i, g := range s.Room.Scene.Groups {
		if g.ParentID != "" {
			target, ok := ids[g.ParentID]
			if !ok || !strings.Contains(target, "groups") {
				add(fmt.Sprintf("room.scene.groups[%d].parentId", i), "must reference a group")
			}
		}
	}
	for i, it := range s.Room.Scene.Items {
		if it.SupportID == it.ID && it.SupportID != "" {
			add(fmt.Sprintf("room.scene.items[%d].supportId", i), "support cycle")
		}
	}
	return e
}
func validHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
func sceneProp(ids map[string]string, k string) bool {
	p, ok := ids[k]
	return ok && strings.Contains(p, "items")
}
func finitePositive(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
func checkFootprint(f RoomFootprint, p string, e *[]FieldError) {
	for n, v := range map[string]float64{"width": f.Width, "depth": f.Depth, "offsetX": f.OffsetX, "offsetZ": f.OffsetZ} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			*e = append(*e, FieldError{Path: p + "." + n, Message: "must be finite"})
		}
	}
	if !finitePositive(f.Width) || !finitePositive(f.Depth) {
		*e = append(*e, FieldError{Path: p, Message: "dimensions must be positive"})
	}
}
func finiteOffset(o encounter.RoomSceneOffset, p string, e *[]FieldError) {
	for n, v := range map[string]float64{"x": o.X, "y": o.Y, "z": o.Z} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			*e = append(*e, FieldError{Path: p + ".pointLight.offset." + n, Message: "must be finite"})
		}
	}
}
func requiredSingleRoomFields(root *yaml.Node) []FieldError {
	var out []FieldError
	add := func(p string) { out = append(out, FieldError{Path: p, Message: "is required"}) }
	var walk func(*yaml.Node, string)
	walk = func(n *yaml.Node, p string) {
		if n == nil {
			return
		}
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				q := p + "." + k.Value
				switch q {
				case "root.room.coordinateFrame":
					for _, x := range []string{"horizontalPlane", "verticalAxis", "distanceUnit", "hexRadius", "footprintFrame"} {
						if !present(mapValue(v, x)) {
							add(q + "." + x)
						}
					}
				case "root.room.scene.items":
					if v.Kind == yaml.SequenceNode {
						for j, it := range v.Content {
							tv := mapValue(it, "transform")
							if tv == nil {
								add(fmt.Sprintf("room.scene.items[%d].transform", j))
							} else {
								for _, z := range []string{"x", "y", "z", "rotationY"} {
									if !present(mapValue(tv, z)) {
										add(fmt.Sprintf("room.scene.items[%d].transform.%s", j, z))
									}
								}
							}
						}
					}
				case "root.room.room.partyStart":
					for _, x := range []string{"q", "r"} {
						if !present(mapValue(v, x)) {
							add("room.room.partyStart." + x)
						}
					}
				case "root.room.room.propDeclarations":
					if v.Kind == yaml.MappingNode {
						for i := 0; i < len(v.Content); i += 2 {
							k, d := v.Content[i], v.Content[i+1]
							for _, x := range []string{"blocksMovement", "blocksLineOfSight", "footprint"} {
								if !present(mapValue(d, x)) {
									add("room.room.propDeclarations." + k.Value + "." + x)
								}
							}
						}
					}
				case "root.room.scene.groups":
					if v.Kind == yaml.SequenceNode {
						for j, it := range v.Content {
							tv := mapValue(it, "transform")
							if tv == nil {
								add(fmt.Sprintf("room.scene.groups[%d].transform", j))
							} else {
								for _, z := range []string{"x", "y", "z", "rotationY"} {
									if !present(mapValue(tv, z)) {
										add(fmt.Sprintf("room.scene.groups[%d].transform.%s", j, z))
									}
								}
							}
						}
					}
				}
				walk(v, q)
			}
		case yaml.SequenceNode:
			for _, v := range n.Content {
				walk(v, p)
			}
		}
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		walk(root.Content[0], "root")
	} else {
		walk(root, "root")
	}
	return out
}
func present(n *yaml.Node) bool { return n != nil && n.Tag != "!!null" }
func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func finiteTransform(t encounter.RoomSceneTransform, p string, e *[]FieldError) {
	for n, v := range map[string]float64{"x": t.X, "y": t.Y, "z": t.Z, "rotationY": t.RotationY} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			*e = append(*e, FieldError{Path: p + ".transform." + n, Message: "must be finite"})
		}
	}
}
