// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func wallSectionsShape(appearance *yaml.Node, path string, add errSink) {
	sections := optionalNode(appearance, "sections", path, add)
	if sections == nil {
		return
	}
	path += ".sections"
	if sections.Kind != yaml.SequenceNode {
		add(path, errNotAList)
		return
	}
	for i, raw := range sections.Content {
		at := fmt.Sprintf("%s[%d]", path, i)
		entryShape(raw, at, add)
		row := resolveNode(raw)
		if row == nil || row.Kind != yaml.MappingNode {
			continue
		}
		requireString(row, "id", at, add)
		requireString(row, "assetRef", at, add)
		requireNumber(row, "position", at, add)
		requireNumber(row, "width", at, add)
	}
}

func wallSectionValues(wall *RoomWall, length float64, path string, add errSink) {
	if len(wall.Appearance.Sections) == 0 {
		return
	}
	type interval struct {
		start, end float64
		path       string
	}
	intervals := make([]interval, 0, len(wall.Openings)+len(wall.Appearance.Sections))
	for i, opening := range wall.Openings {
		before, after := opening.Width/2, opening.Width/2
		if opening.Door != nil && opening.Door.FramePresentation != nil && opening.Door.FramePresentation.Span != nil {
			before, after = opening.Door.FramePresentation.Span.Before, opening.Door.FramePresentation.Span.After
		}
		intervals = append(intervals, interval{opening.Position - before, opening.Position + after, fmt.Sprintf("%s.openings[%d]", path, i)})
	}
	seen := make(map[string]bool, len(wall.Appearance.Sections))
	roundoff := 8 * (math.Nextafter(1, 2) - 1) * length
	for i, section := range wall.Appearance.Sections {
		at := fmt.Sprintf("%s.appearance.sections[%d]", path, i)
		if strings.TrimSpace(section.ID) == "" || seen[section.ID] {
			add(at+".id", "must be nonblank and unique within this wall")
		}
		seen[section.ID] = true
		if strings.TrimSpace(section.AssetRef) == "" {
			add(at+".assetRef", errRequired)
		}
		wallNumber(section.Position, at+".position", false, add)
		wallNumber(section.Width, at+".width", true, add)
		if !wallFinite(section.Position) || !wallFinite(section.Width) || section.Width <= 0 {
			continue
		}
		start, end := section.Position-section.Width/2, section.Position+section.Width/2
		if !wallRepresentable(start) || !wallRepresentable(end) || end <= start || start < -roundoff || end > length+roundoff {
			add(at, wallOpeningOutsideLine)
			continue
		}
		intervals = append(intervals, interval{math.Max(0, start), math.Min(length, end), at})
	}
	sort.SliceStable(intervals, func(i, j int) bool { return intervals[i].start < intervals[j].start })
	for i := 1; i < len(intervals); i++ {
		if intervals[i].start < intervals[i-1].end {
			add(intervals[i].path, "visual section overlaps another section or opening surround")
		}
	}
}
