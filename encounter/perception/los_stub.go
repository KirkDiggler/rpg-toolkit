package perception

import (
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// HexDistance is the cube-coordinate hex distance between two hexes.
// Exported for use by the encounter package's verbs.
func HexDistance(a, b core.Hex) int {
	dq := abs(a.Q - b.Q)
	dr := abs(a.R - b.R)
	ds := abs(a.S - b.S)
	if dq > dr {
		if dq > ds {
			return dq
		}
		return ds
	}
	if dr > ds {
		return dr
	}
	return ds
}

// VisibleHexesAt returns the hexes within sightRange of from, including from,
// that are not wall-blocked from from.
//
// room is wall-aware LoS (Wave 1, rpg-toolkit#757): a hex within range is
// excluded when room.IsLineOfSightBlocked reports a wall between from and
// that hex. room may be nil — encounters with no SpaceData (pre-wave-1
// fixtures, non-spatial encounters) get the original pure-radius behavior.
// SightRange always caps distance regardless of room.
func VisibleHexesAt(from core.Hex, sightRange int, room spatial.Room) core.HexSet {
	out := make(core.HexSet)
	fromPos := from.ToPosition()
	for dq := -sightRange; dq <= sightRange; dq++ {
		for dr := -sightRange; dr <= sightRange; dr++ {
			ds := -dq - dr
			// dq and dr are in range by construction; only ds needs filtering.
			if abs(ds) > sightRange {
				continue
			}
			h := core.Hex{Q: from.Q + dq, R: from.R + dr, S: from.S + ds}
			if room != nil {
				targetPos := h.ToPosition()
				if !room.GetGrid().IsValidPosition(targetPos) || room.IsLineOfSightBlocked(fromPos, targetPos) {
					continue
				}
			}
			out[h] = struct{}{}
		}
	}
	return out
}

// CanSeeAt reports whether a viewer can currently see the given hex: within
// the viewer's SightRange (cube distance) AND, when room is non-nil, not
// wall-blocked (room.IsLineOfSightBlocked). Returns false for a nil viewer.
// room may be nil for the pre-wave-1 pure-radius behavior.
func CanSeeAt(viewer *View, target core.Hex, room spatial.Room) bool {
	if viewer == nil {
		return false
	}
	if HexDistance(viewer.Position, target) > viewer.SightRange {
		return false
	}
	if room != nil {
		targetPos := target.ToPosition()
		if !room.GetGrid().IsValidPosition(targetPos) || room.IsLineOfSightBlocked(viewer.Position.ToPosition(), targetPos) {
			return false
		}
	}
	return true
}

// HexNeighbors returns the six adjacent hexes (cube coords).
func HexNeighbors(h core.Hex) []core.Hex {
	return []core.Hex{
		{Q: h.Q + 1, R: h.R - 1, S: h.S},
		{Q: h.Q + 1, R: h.R, S: h.S - 1},
		{Q: h.Q, R: h.R + 1, S: h.S - 1},
		{Q: h.Q - 1, R: h.R + 1, S: h.S},
		{Q: h.Q - 1, R: h.R, S: h.S + 1},
		{Q: h.Q, R: h.R - 1, S: h.S + 1},
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
