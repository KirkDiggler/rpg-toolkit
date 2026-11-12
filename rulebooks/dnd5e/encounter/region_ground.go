// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RegionGround is immutable appearance intent. Empty BaseRef means no authored
// base; the renderer retains its existing default. Refs never decide mechanics.
// Paint cells use their containing region's frame: authored offset at input,
// dungeon-absolute in an atlas. A nil block means no ground was authored.
type RegionGround struct {
	BaseRef string
	Paint   []GroundPaint
}

// GroundPaint explicitly assigns an appearance to existing cells of one region.
// Equality with BaseRef does not make the assignment implicit.
type GroundPaint struct {
	Ref   string
	Cells []spatial.Position
}

// RegionGroundData persists optional ground alongside region construction data.
type RegionGroundData struct {
	BaseRef string            `json:"base_ref,omitempty"`
	Paint   []GroundPaintData `json:"paint,omitempty"`
}

// GroundPaintData stores cells in the containing saved region's authored frame.
type GroundPaintData struct {
	Ref   string         `json:"ref"`
	Cells []PositionData `json:"cells"`
}

func validateRegionGround(ground *RegionGround, cells []spatial.Position) error {
	if ground == nil {
		return nil
	}
	if strings.TrimSpace(ground.BaseRef) != ground.BaseRef {
		return fmt.Errorf("ground base ref is padded: %w", ErrNoField)
	}
	owned := make(map[spatial.Position]bool, len(cells))
	for _, c := range cells {
		owned[c] = true
	}
	painted := make(map[spatial.Position]bool)
	for i, paint := range ground.Paint {
		if paint.Ref == "" || strings.TrimSpace(paint.Ref) != paint.Ref {
			return fmt.Errorf("ground paint[%d] has an empty or padded ref: %w", i, ErrNoField)
		}
		for j, c := range paint.Cells {
			if !isAuthoredCell(c) || !owned[c] {
				return fmt.Errorf("ground paint[%d].cells[%d] is not owned by its region: %w", i, j, ErrNoField)
			}
			if painted[c] {
				return fmt.Errorf("ground paint[%d].cells[%d] repeats an assignment: %w", i, j, ErrNoField)
			}
			painted[c] = true
		}
	}
	return nil
}

func copyRegionGround(ground *RegionGround, convert func(spatial.Position) spatial.Position) *RegionGround {
	if ground == nil {
		return nil
	}
	out := &RegionGround{BaseRef: ground.BaseRef}
	if ground.Paint != nil {
		out.Paint = make([]GroundPaint, len(ground.Paint))
	}
	for i, paint := range ground.Paint {
		out.Paint[i] = GroundPaint{Ref: paint.Ref, Cells: append([]spatial.Position(nil), paint.Cells...)}
		if convert != nil {
			for j, c := range out.Paint[i].Cells {
				out.Paint[i].Cells[j] = convert(c)
			}
			sortCells(out.Paint[i].Cells)
		}
	}
	return out
}

// groundForCells is the shared projection cut; it never returns source aliases.
func groundForCells(ground *RegionGround, cells []spatial.Position) *RegionGround {
	if ground == nil {
		return nil
	}
	allowed := make(map[spatial.Position]bool, len(cells))
	for _, c := range cells {
		allowed[c] = true
	}
	out := &RegionGround{BaseRef: ground.BaseRef}
	for _, paint := range ground.Paint {
		var kept []spatial.Position
		for _, c := range paint.Cells {
			if allowed[c] {
				kept = append(kept, c)
			}
		}
		if len(kept) > 0 {
			out.Paint = append(out.Paint, GroundPaint{Ref: paint.Ref, Cells: kept})
		}
	}
	return out
}

func regionGroundData(ground *RegionGround) *RegionGroundData {
	if ground == nil {
		return nil
	}
	out := &RegionGroundData{BaseRef: ground.BaseRef}
	for _, paint := range ground.Paint {
		cells := make([]PositionData, len(paint.Cells))
		for i, c := range paint.Cells {
			cells[i] = PositionData{X: c.X, Y: c.Y}
		}
		out.Paint = append(out.Paint, GroundPaintData{Ref: paint.Ref, Cells: cells})
	}
	return out
}

func regionGroundFromData(data *RegionGroundData) *RegionGround {
	if data == nil {
		return nil
	}
	out := &RegionGround{BaseRef: data.BaseRef}
	for _, paint := range data.Paint {
		cells := make([]spatial.Position, len(paint.Cells))
		for i, c := range paint.Cells {
			cells[i] = spatial.Position{X: c.X, Y: c.Y}
		}
		out.Paint = append(out.Paint, GroundPaint{Ref: paint.Ref, Cells: cells})
	}
	return out
}
