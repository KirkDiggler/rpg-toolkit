// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"strings"
)

// WallFaceSide identifies a geometric face in an authored wall's local frame.
// It is not a region owner, visibility policy or asset material selector.
type WallFaceSide string

const (
	// WallFacePositiveZ is the authored wall's positive local-Z face.
	WallFacePositiveZ WallFaceSide = "positive-z"
	// WallFaceNegativeZ is the authored wall's negative local-Z face.
	WallFaceNegativeZ WallFaceSide = "negative-z"
)

// WallFaceFinish is an immutable visual choice on a permitted wall face.
// Sides are unique and known; MaterialRef is nonblank and carried unread.
// An absent list means no explicit face finish, never a source/region lookup.
type WallFaceFinish struct {
	Side        WallFaceSide
	MaterialRef string
}

// WallFaceFinishData persists a face choice with the wall's immutable site data.
type WallFaceFinishData struct {
	Side        WallFaceSide `json:"side"`
	MaterialRef string       `json:"material_ref"`
}

func validateWallFaceFinishes(values []WallFaceFinish) error {
	var seen uint8
	for _, value := range values {
		var bit uint8
		switch value.Side {
		case WallFacePositiveZ:
			bit = 1
		case WallFaceNegativeZ:
			bit = 2
		default:
			return fmt.Errorf("wall face finish has unknown side %q: %w", value.Side, ErrNoField)
		}
		if strings.TrimSpace(value.MaterialRef) == "" {
			return fmt.Errorf("wall face finish needs a nonblank material ref: %w", ErrNoField)
		}
		if seen&bit != 0 {
			return fmt.Errorf("duplicate wall face finish side %q: %w", value.Side, ErrNoField)
		}
		seen |= bit
	}
	return nil
}

func cloneWallFaceFinishes(values []WallFaceFinish) []WallFaceFinish {
	return append([]WallFaceFinish(nil), values...)
}

func wallFaceFinishesData(values []WallFaceFinish) []WallFaceFinishData {
	if len(values) == 0 {
		return nil
	}
	out := make([]WallFaceFinishData, len(values))
	for i, value := range values {
		out[i] = WallFaceFinishData(value)
	}
	return out
}

func wallFaceFinishesFromData(values []WallFaceFinishData) []WallFaceFinish {
	if len(values) == 0 {
		return nil
	}
	out := make([]WallFaceFinish, len(values))
	for i, value := range values {
		out[i] = WallFaceFinish(value)
	}
	return out
}
