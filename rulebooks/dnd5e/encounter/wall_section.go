// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// WallSection is a once-placed visual span owned by a structural wall. Position
// and Width are canonical feet along its From-to-To line. Ref is opaque; this
// value supplies no opening, blocker, independent presence or door state.
type WallSection struct {
	ID       string
	Ref      string
	Position float64
	Width    float64
}

// WallSectionData persists the immutable visual span with its owning wall.
// Omission of the wall's list retains legacy appearance and site identity.
type WallSectionData struct {
	ID       string  `json:"id"`
	Ref      string  `json:"ref"`
	Position float64 `json:"position"`
	Width    float64 `json:"width"`
}

func cloneWallSections(values []WallSection) []WallSection {
	return append([]WallSection(nil), values...)
}

func wallSectionsData(values []WallSection) []WallSectionData {
	if len(values) == 0 {
		return nil
	}
	out := make([]WallSectionData, len(values))
	for i, value := range values {
		out[i] = WallSectionData(value)
	}
	return out
}

func wallSectionsFromData(values []WallSectionData) []WallSection {
	if len(values) == 0 {
		return nil
	}
	out := make([]WallSection, len(values))
	for i, value := range values {
		out[i] = WallSection(value)
	}
	return out
}

func validateWallSections(wall *StructuralWallInput, length float64) error {
	if len(wall.Sections) == 0 {
		return nil
	}
	type interval struct{ start, end float64 }
	cuts := make([]interval, 0, len(wall.Openings)+len(wall.Sections))
	for _, opening := range wall.Openings {
		before, after := opening.Width/2, opening.Width/2
		if opening.Door != nil && opening.Door.FramePresentation != nil && opening.Door.FramePresentation.Span != nil {
			before, after = opening.Door.FramePresentation.Span.Before, opening.Door.FramePresentation.Span.After
		}
		cuts = append(cuts, interval{opening.Position - before, opening.Position + after})
	}
	seen := make(map[string]bool, len(wall.Sections))
	roundoff := 8 * (math.Nextafter(1, 2) - 1) * length
	for _, section := range wall.Sections {
		if strings.TrimSpace(section.ID) == "" || strings.TrimSpace(section.Ref) == "" || seen[section.ID] {
			return fmt.Errorf("wall sections require nonblank refs and unique nonblank identities: %w", ErrNoField)
		}
		seen[section.ID] = true
		if !structuralRepresentable(section.Position) || !structuralRepresentable(section.Width) || section.Width <= 0 {
			return fmt.Errorf("wall section %q requires finite lengths and positive width: %w", section.ID, ErrNoField)
		}
		start, end := section.Position-section.Width/2, section.Position+section.Width/2
		if !structuralRepresentable(start) || !structuralRepresentable(end) || end <= start || start < -roundoff || end > length+roundoff {
			return fmt.Errorf("wall section %q must fit its owning line: %w", section.ID, ErrNoField)
		}
		cuts = append(cuts, interval{math.Max(0, start), math.Min(length, end)})
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].start < cuts[j].start })
	for i := 1; i < len(cuts); i++ {
		if cuts[i].start < cuts[i-1].end {
			return fmt.Errorf("wall sections and opening surrounds must not overlap: %w", ErrNoField)
		}
	}
	return nil
}
