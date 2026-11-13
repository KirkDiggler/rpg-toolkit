// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// WallFrameSpan is visual replacement extent about an aperture centre, in
// canonical feet. It is not a passage width, blocker or parent association.
type WallFrameSpan struct {
	Before float64 `json:"before"`
	After  float64 `json:"after"`
}

// DoorFrameFaceFinish is a fixed-face appearance choice. PhaseU is required
// present and in [0,1); a pointer distinguishes explicit zero from absence.
type DoorFrameFaceFinish struct {
	Side        string   `json:"side"`
	MaterialRef string   `json:"material_ref"`
	PhaseU      *float64 `json:"phase_u"`
}

// DoorFramePresentation is independently permitted immutable door appearance.
// Present values require Span; an empty face list preserves authored materials.
type DoorFramePresentation struct {
	Span         *WallFrameSpan        `json:"span"`
	FaceFinishes []DoorFrameFaceFinish `json:"face_finishes,omitempty"`
}

func projectWallFrameSpan(span *encounter.WallFrameSpan) *WallFrameSpan {
	if span == nil {
		return nil
	}
	return &WallFrameSpan{Before: span.Before, After: span.After}
}

func projectDoorFramePresentation(frame *encounter.DoorFramePresentation) *DoorFramePresentation {
	if frame == nil {
		return nil
	}
	out := &DoorFramePresentation{Span: projectWallFrameSpan(frame.Span)}
	for _, face := range frame.FaceFinishes {
		phase := face.PhaseU
		out.FaceFinishes = append(out.FaceFinishes, DoorFrameFaceFinish{Side: string(face.Side), MaterialRef: face.MaterialRef, PhaseU: &phase})
	}
	return out
}

func validWallFrameSpan(span *WallFrameSpan, width float64) bool {
	if span == nil {
		return true
	}
	for _, v := range []float64{width, span.Before, span.After} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return false
		}
	}
	return span.Before >= width/2 && span.After >= width/2
}

func validDoorFramePresentation(frame *DoorFramePresentation, width float64) bool {
	if frame == nil {
		return true
	}
	if frame.Span == nil || !validWallFrameSpan(frame.Span, width) {
		return false
	}
	faces := make([]WallFaceFinish, len(frame.FaceFinishes))
	for i, face := range frame.FaceFinishes {
		if face.PhaseU == nil || math.IsNaN(*face.PhaseU) || math.IsInf(*face.PhaseU, 0) || *face.PhaseU < 0 || *face.PhaseU >= 1 {
			return false
		}
		faces[i] = WallFaceFinish{Side: face.Side, MaterialRef: face.MaterialRef}
	}
	return validWallFaceFinishes(faces)
}
