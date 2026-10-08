// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"gopkg.in/yaml.v3"
)

// readPropPresentation captures the renderable subset once at the source
// boundary. Mechanical-only legacy items without assetRef stay mechanical-only.
// No catalog, editor hierarchy or parent transform is interpreted here.
func readPropPresentation(item *yaml.Node, id, path string, add errSink) (encounter.PropPresentation, bool) {
	if childNode(item, "assetRef") == nil {
		return encounter.PropPresentation{}, false
	}
	valid := true
	reject := func(p, m string) { valid = false; add(p, m) }
	ref := requireString(item, "assetRef", path, reject)
	if ref == nil {
		return encounter.PropPresentation{}, false
	}
	if ref.Value == "" {
		reject(path+".assetRef", errRequired)
	}
	t := childNode(item, "transform")
	x, _ := sceneNumber(t, "x", path+".transform", reject)
	y, _ := sceneNumber(t, "y", path+".transform", reject)
	z, _ := sceneNumber(t, "z", path+".transform", reject)
	yaw, _ := sceneNumber(t, "rotationY", path+".transform", reject)
	p := encounter.PropPresentation{ID: id, Ref: ref.Value, Origin: spatial.Point{X: x * feetPerSourceUnit, Y: z * feetPerSourceUnit}, Elevation: y * feetPerSourceUnit, FacingDegrees: -yaw * 180 / math.Pi, HeightScale: 1}
	if childNode(item, "label") != nil {
		if label := requireString(item, "label", path, reject); label != nil {
			p.Label = label.Value
		}
	}
	if childNode(item, "heightScale") != nil {
		p.HeightScale, _ = sceneNumber(item, "heightScale", path, reject)
		if p.HeightScale <= 0 {
			reject(path+".heightScale", "must be positive")
		}
	}
	if n := childNode(item, "pointLight"); n != nil {
		n = resolveNode(n)
		if n == nil || n.Kind != yaml.MappingNode {
			reject(path+".pointLight", errNotAMapping)
		} else {
			p.PointLight = readPropPointLight(n, path+".pointLight", reject)
		}
	}
	return p, valid
}

func readPropPointLight(n *yaml.Node, path string, add errSink) *encounter.PropPointLight {
	l := &encounter.PropPointLight{}
	if enabled := childNode(n, "enabled"); enabled != nil {
		enabled = resolveNode(enabled)
		if enabled.Tag != "!!bool" {
			add(path+".enabled", "must be a boolean")
		} else if err := enabled.Decode(&l.Enabled); err != nil {
			add(path+".enabled", "must be a boolean")
		}
	}
	offset := childNode(n, "offset")
	x, _ := sceneNumber(offset, "x", path+".offset", add)
	y, _ := sceneNumber(offset, "y", path+".offset", add)
	z, _ := sceneNumber(offset, "z", path+".offset", add)
	l.Offset = spatial.Point{X: x * feetPerSourceUnit, Y: z * feetPerSourceUnit}
	l.OffsetElevation = y * feetPerSourceUnit
	if color := requireString(n, "color", path, add); color != nil {
		l.Color = color.Value
	}
	l.Intensity, _ = sceneNumber(n, "intensity", path, add)
	l.Range, _ = sceneNumber(n, "range", path, add)
	l.Range *= feetPerSourceUnit
	return l
}

func propPresentationsOf(read roomRead, spec *SingleRoomSpec) []encounter.PropPresentation {
	out := append([]encounter.PropPresentation(nil), read.Presentations...)
	for i := range out {
		if _, bound := spec.Room.Gameplay.DoorBindings[out[i].ID]; bound {
			out[i].DoorID = spec.Key + "/" + out[i].ID
		}
	}
	return out
}
