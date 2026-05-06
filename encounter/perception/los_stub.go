package perception

import "github.com/KirkDiggler/rpg-toolkit/encounter/core"

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

// VisibleHexesAt returns the hexes within sightRange of from, including from.
//
// STUB: ignores walls, lighting, conditions. Replaced with real LoS in a
// future slice.
func VisibleHexesAt(from core.Hex, sightRange int) core.HexSet {
	out := make(core.HexSet)
	for dq := -sightRange; dq <= sightRange; dq++ {
		for dr := -sightRange; dr <= sightRange; dr++ {
			ds := -dq - dr
			// dq and dr are in range by construction; only ds needs filtering.
			if abs(ds) > sightRange {
				continue
			}
			h := core.Hex{Q: from.Q + dq, R: from.R + dr, S: from.S + ds}
			out[h] = struct{}{}
		}
	}
	return out
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
