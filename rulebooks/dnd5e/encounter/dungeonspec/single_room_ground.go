// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RoomGround carries a region's optional base and explicit appearance patches.
// Missing BaseRef preserves the existing default; no ref is a movement rule.
type RoomGround struct {
	BaseRef string            `yaml:"baseRef,omitempty" json:"baseRef,omitempty"`
	Paint   []RoomGroundPaint `yaml:"paint,omitempty" json:"paint,omitempty"`
}

// RoomGroundPaint assigns one appearance to existing cells of its region.
type RoomGroundPaint struct {
	Ref   string        `yaml:"ref" json:"ref"`
	Cells []RoomCellRow `yaml:"cells" json:"cells"`
}

func groundShape(region *yaml.Node, path string, add errSink) {
	if optionalNode(region, "ground", path, add) == nil {
		return
	}
	ground := requireMapping(region, "ground", path, add)
	if ground == nil {
		return
	}
	p := path + ".ground"
	if base := optionalNode(ground, "baseRef", p, add); base != nil {
		requireString(ground, "baseRef", p, add)
		if resolveNode(base).Value == "" {
			add(p+".baseRef", "ref must not be empty")
		}
	}
	if optionalNode(ground, "paint", p, add) == nil {
		return
	}
	paint := requireSequence(ground, "paint", p, add)
	if paint == nil {
		return
	}
	for i, row := range paint.Content {
		row = resolveNode(row)
		rp := fmt.Sprintf("%s.paint[%d]", p, i)
		entryShape(row, rp, add)
		if row == nil || row.Kind != yaml.MappingNode {
			continue
		}
		requireString(row, "ref", rp, add)
		rows := requireSequence(row, "cells", rp, add)
		if rows != nil {
			cellRowsShape(rows, rp+".cells", add)
		}
	}
}

func groundValues(regions []RoomRegion, add errSink) {
	for i, region := range regions {
		if region.Ground == nil {
			continue
		}
		p := fmt.Sprintf("room.room.regions[%d].ground", i)
		if strings.TrimSpace(region.Ground.BaseRef) != region.Ground.BaseRef {
			add(p+".baseRef", "ref must not be padded")
		}
		cells := ExpandRows(region.Cells, fmt.Sprintf("room.room.regions[%d].cells", i), func(string, string) {})
		owned := make(map[RoomCell]bool, len(cells))
		for _, c := range cells {
			owned[c] = true
		}
		assigned := make(map[RoomCell]bool)
		for j, paint := range region.Ground.Paint {
			rp := fmt.Sprintf("%s.paint[%d]", p, j)
			if paint.Ref == "" || strings.TrimSpace(paint.Ref) != paint.Ref {
				add(rp+".ref", "ref must be non-empty and unpadded")
			}
			// Bound the new collection by its owner before expanding compact spans.
			count := 0
			for _, row := range paint.Cells {
				for _, span := range row.Q {
					if inBound(span[0]) && inBound(span[1]) && span[0] <= span[1] {
						count += span[1] - span[0] + 1
					}
				}
			}
			if count > len(owned)-len(assigned) {
				add(rp+".cells", "paint lists more cells than its region owns")
				continue
			}
			for _, cell := range ExpandRows(paint.Cells, rp+".cells", add) {
				if !owned[cell] {
					add(rp+".cells", "paint cell is outside its region")
				}
				if assigned[cell] {
					add(rp+".cells", "cell already has ground paint")
				}
				assigned[cell] = true
			}
		}
	}
}

func lowerRegionGround(ground *RoomGround, o spatial.HexOrientation) *encounter.RegionGround {
	if ground == nil {
		return nil
	}
	out := &encounter.RegionGround{BaseRef: ground.BaseRef}
	for _, paint := range ground.Paint {
		row := encounter.GroundPaint{Ref: paint.Ref}
		for _, cell := range ExpandRows(paint.Cells, "ground.paint.cells", func(string, string) {}) {
			row.Cells = append(row.Cells, axialOffset(cell, o))
		}
		out.Paint = append(out.Paint, row)
	}
	return out
}

func groundWithoutFooting(ground *encounter.RegionGround, kept []spatial.Position) *encounter.RegionGround {
	if ground == nil {
		return nil
	}
	allowed := make(map[spatial.Position]bool, len(kept))
	for _, c := range kept {
		allowed[c] = true
	}
	out := &encounter.RegionGround{BaseRef: ground.BaseRef}
	for _, paint := range ground.Paint {
		row := encounter.GroundPaint{Ref: paint.Ref}
		for _, c := range paint.Cells {
			if allowed[c] {
				row.Cells = append(row.Cells, c)
			}
		}
		if len(row.Cells) > 0 {
			out.Paint = append(out.Paint, row)
		}
	}
	return out
}
