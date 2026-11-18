// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"gopkg.in/yaml.v3"
)

// RoomWallFrameSpan is the surround's visual extent about the opening centre,
// in source scene units. Lowering converts lengths once, never passage width.
type RoomWallFrameSpan struct {
	Before float64 `yaml:"before" json:"before"`
	After  float64 `yaml:"after" json:"after"`
}

// RoomDoorFrameFaceFinish is a supplied fixed-face choice and wrapped phase.
// PhaseU is unitless and explicit, including zero; no asset lookup derives it.
type RoomDoorFrameFaceFinish struct {
	Side        encounter.WallFaceSide `yaml:"side" json:"side"`
	MaterialRef string                 `yaml:"materialRef" json:"materialRef"`
	PhaseU      float64                `yaml:"phaseU" json:"phaseU"`
}

// RoomDoorFramePresentation is optional typed appearance on an attached door,
// not a second pose or mechanical declaration. Present values require Span.
type RoomDoorFramePresentation struct {
	Span         *RoomWallFrameSpan        `yaml:"span" json:"span"`
	FaceFinishes []RoomDoorFrameFaceFinish `yaml:"faceFinishes,omitempty" json:"faceFinishes,omitempty"`
}

func wallDoorFrameShape(door *yaml.Node, path string, add errSink) {
	frame := optionalNode(door, "framePresentation", path, add)
	if frame == nil {
		return
	}
	path += ".framePresentation"
	if frame.Kind != yaml.MappingNode {
		add(path, errNotAMapping)
		return
	}
	if span := requireMapping(frame, "span", path, add); span != nil {
		requireNumber(span, "before", path+".span", add)
		requireNumber(span, "after", path+".span", add)
	}
	faces := optionalNode(frame, "faceFinishes", path, add)
	if faces == nil {
		return
	}
	if faces.Kind != yaml.SequenceNode {
		add(path+".faceFinishes", errNotAList)
		return
	}
	for i, raw := range faces.Content {
		at := fmt.Sprintf("%s.faceFinishes[%d]", path, i)
		face := resolveNode(raw)
		if face == nil || isNull(face) {
			add(at, errNotNull)
			continue
		}
		if face.Kind != yaml.MappingNode {
			add(at, errNotAMapping)
			continue
		}
		requireString(face, "side", at, add)
		requireString(face, "materialRef", at, add)
		requireNumber(face, "phaseU", at, add)
	}
}

func wallDoorFrameValues(frame *RoomDoorFramePresentation, path string, add errSink) {
	if frame == nil {
		return
	}
	path += ".framePresentation"
	if frame.Span == nil {
		add(path+".span", errRequired)
	} else {
		wallNumber(frame.Span.Before, path+".span.before", true, add)
		wallNumber(frame.Span.After, path+".span.after", true, add)
	}
	seen := map[encounter.WallFaceSide]bool{}
	for i, face := range frame.FaceFinishes {
		at := fmt.Sprintf("%s.faceFinishes[%d]", path, i)
		if face.Side != encounter.WallFacePositiveZ && face.Side != encounter.WallFaceNegativeZ {
			add(at+".side", "must be positive-z or negative-z")
		}
		if seen[face.Side] {
			add(at+".side", "duplicate fixed-face side")
		}
		seen[face.Side] = true
		if strings.TrimSpace(face.MaterialRef) == "" {
			add(at+".materialRef", "must be nonblank")
		}
		if !wallFinite(face.PhaseU) || face.PhaseU < 0 || face.PhaseU >= 1 {
			add(at+".phaseU", "must be finite and wrapped in [0,1)")
		}
	}
}

func canonicalDoorFrame(frame *RoomDoorFramePresentation) *encounter.DoorFramePresentation {
	if frame == nil {
		return nil
	}
	out := &encounter.DoorFramePresentation{}
	if frame.Span != nil {
		out.Span = &encounter.WallFrameSpan{Before: frame.Span.Before * feetPerSourceUnit, After: frame.Span.After * feetPerSourceUnit}
	}
	for _, face := range frame.FaceFinishes {
		out.FaceFinishes = append(out.FaceFinishes, encounter.DoorFrameFaceFinish{Side: face.Side, MaterialRef: face.MaterialRef, PhaseU: face.PhaseU})
	}
	return out
}
