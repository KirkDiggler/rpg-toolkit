// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"
)

// WallFrameSpan is visual replacement extent about an aperture centre along
// its axis, in canonical feet. It never determines passage width or blockers.
type WallFrameSpan struct {
	Before float64
	After  float64
}

// DoorFrameFaceFinish is an opaque fixed-face look with wrapped U phase at the
// aperture centre. PhaseU is in [0,1); zero is a real phase. It never identifies
// a parent wall or gives the distance to a potentially hidden wall start.
type DoorFrameFaceFinish struct {
	Side        WallFaceSide
	MaterialRef string
	PhaseU      float64
}

// DoorFramePresentation is captured immutable aperture-fitted appearance.
// A present value requires Span; empty FaceFinishes retains authored materials.
// All boundaries copy it, including its span and face list. These same typed
// facts persist in site data; no vendor-resource or source-document lookup is
// involved. Absence retains legacy complete-assembly behavior.
type DoorFramePresentation struct {
	Span         *WallFrameSpan
	FaceFinishes []DoorFrameFaceFinish
}

// WallFrameSpanData persists canonical-feet visual extents, never a blocker.
type WallFrameSpanData struct {
	Before float64 `json:"before"`
	After  float64 `json:"after"`
}

// DoorFrameFaceFinishData persists an opaque fixed-face choice and wrapped phase.
// Phase presence is required: missing/null data is not an authored zero.
type DoorFrameFaceFinishData struct {
	Side        WallFaceSide `json:"side"`
	MaterialRef string       `json:"material_ref"`
	PhaseU      *float64     `json:"phase_u"`
}

// DoorFramePresentationData is the detached immutable site representation.
type DoorFramePresentationData struct {
	Span         *WallFrameSpanData        `json:"span"`
	FaceFinishes []DoorFrameFaceFinishData `json:"face_finishes,omitempty"`
}

func doorFramePresentationData(frame *DoorFramePresentation) *DoorFramePresentationData {
	if frame == nil {
		return nil
	}
	out := &DoorFramePresentationData{}
	if frame.Span != nil {
		span := WallFrameSpanData(*frame.Span)
		out.Span = &span
	}
	for _, face := range frame.FaceFinishes {
		phase := face.PhaseU
		out.FaceFinishes = append(out.FaceFinishes, DoorFrameFaceFinishData{Side: face.Side, MaterialRef: face.MaterialRef, PhaseU: &phase})
	}
	return out
}

func doorFramePresentationFromData(frame DoorFramePresentationData) (DoorFramePresentation, error) {
	out := DoorFramePresentation{}
	if frame.Span != nil {
		span := WallFrameSpan(*frame.Span)
		out.Span = &span
	}
	for i, face := range frame.FaceFinishes {
		if face.PhaseU == nil {
			return DoorFramePresentation{}, fmt.Errorf("frame face %d phase is required: %w", i, ErrNoField)
		}
		out.FaceFinishes = append(out.FaceFinishes, DoorFrameFaceFinish{Side: face.Side, MaterialRef: face.MaterialRef, PhaseU: *face.PhaseU})
	}
	return out, nil
}

func validateDoorFramePresentation(frame *DoorFramePresentation, width float64) error {
	if frame == nil {
		return nil
	}
	if frame.Span == nil {
		return fmt.Errorf("frame presentation requires span: %w", ErrNoField)
	}
	for _, extent := range []float64{frame.Span.Before, frame.Span.After} {
		if !structuralRepresentable(extent) || extent <= 0 || extent < width/2 {
			return fmt.Errorf("frame span must have positive finite extents enclosing its aperture: %w", ErrNoField)
		}
	}
	faces := make([]WallFaceFinish, len(frame.FaceFinishes))
	for i, face := range frame.FaceFinishes {
		if math.IsNaN(face.PhaseU) || math.IsInf(face.PhaseU, 0) || face.PhaseU < 0 || face.PhaseU >= 1 {
			return fmt.Errorf("frame phase must be finite and wrapped in [0,1): %w", ErrNoField)
		}
		faces[i] = WallFaceFinish{Side: face.Side, MaterialRef: face.MaterialRef}
	}
	return validateWallFaceFinishes(faces)
}

func cloneWallFrameSpan(span *WallFrameSpan) *WallFrameSpan {
	if span == nil {
		return nil
	}
	copy := *span
	return &copy
}

func cloneDoorFramePresentation(frame *DoorFramePresentation) *DoorFramePresentation {
	if frame == nil {
		return nil
	}
	return &DoorFramePresentation{Span: cloneWallFrameSpan(frame.Span), FaceFinishes: append([]DoorFrameFaceFinish(nil), frame.FaceFinishes...)}
}

func openingFrameSpan(opening structuralOpening) *WallFrameSpan {
	if opening.door == nil || opening.door.framePresentation == nil {
		return nil
	}
	return cloneWallFrameSpan(opening.door.framePresentation.Span)
}

func sameWallFrameSpan(a, b *WallFrameSpan) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameDoorFramePresentation(a, b *DoorFramePresentation) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !sameWallFrameSpan(a.Span, b.Span) || len(a.FaceFinishes) != len(b.FaceFinishes) {
		return false
	}
	for i := range a.FaceFinishes {
		if a.FaceFinishes[i] != b.FaceFinishes[i] {
			return false
		}
	}
	return true
}

func sameStructuralDoor(a, b AtlasStructuralDoor) bool {
	af, bf := a.FramePresentation, b.FramePresentation
	a.FramePresentation, b.FramePresentation = nil, nil
	return a == b && sameDoorFramePresentation(af, bf)
}

func frameSpanRow(span *WallFrameSpan) map[string]interface{} {
	return map[string]interface{}{"before": span.Before, "after": span.After}
}

func doorFramePresentationRow(frame *DoorFramePresentation) map[string]interface{} {
	row := map[string]interface{}{"span": frameSpanRow(frame.Span)}
	if len(frame.FaceFinishes) > 0 {
		faces := make([]map[string]interface{}, len(frame.FaceFinishes))
		for i, face := range frame.FaceFinishes {
			faces[i] = map[string]interface{}{"side": string(face.Side), "material_ref": face.MaterialRef, "phase_u": face.PhaseU}
		}
		row["face_finishes"] = faces
	}
	return row
}
