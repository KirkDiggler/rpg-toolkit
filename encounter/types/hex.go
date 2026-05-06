package types

import (
	"encoding/json"
	"sort"
)

// Hex is a cube-coordinate hex on the encounter grid.
//
// Real spatial logic eventually moves to rpg-toolkit/tools/spatial; we use
// bare coordinates here for the slice and stub LoS computations.
type Hex struct {
	Q, R, S int
}

// HexSet is a set of hexes with O(1) membership.
//
// JSON: HexSet marshals as a stable-ordered slice of Hex. Go's encoding/json
// cannot encode struct keys in a map directly — without custom marshaling,
// HexSet would silently round-trip as an empty object. The MarshalJSON /
// UnmarshalJSON methods below convert to/from a slice for wire format while
// preserving set semantics in memory.
type HexSet map[Hex]struct{}

// NewHexSet builds a HexSet from a slice of Hex.
func NewHexSet(hexes ...Hex) HexSet {
	out := make(HexSet, len(hexes))
	for _, h := range hexes {
		out[h] = struct{}{}
	}
	return out
}

// Has reports whether the set contains h.
func (s HexSet) Has(h Hex) bool {
	_, ok := s[h]
	return ok
}

// Slice returns the set as an unordered slice.
func (s HexSet) Slice() []Hex {
	out := make([]Hex, 0, len(s))
	for h := range s {
		out = append(out, h)
	}
	return out
}

// MarshalJSON encodes the set as a sorted slice for stable, deterministic
// wire output (sorted by Q then R then S).
func (s HexSet) MarshalJSON() ([]byte, error) {
	out := s.Slice()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Q != out[j].Q {
			return out[i].Q < out[j].Q
		}
		if out[i].R != out[j].R {
			return out[i].R < out[j].R
		}
		return out[i].S < out[j].S
	})
	return json.Marshal(out)
}

// UnmarshalJSON decodes a slice of Hex back into a HexSet.
// Accepts JSON null and the empty array as the empty set.
func (s *HexSet) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*s = make(HexSet)
		return nil
	}
	var slice []Hex
	if err := json.Unmarshal(b, &slice); err != nil {
		return err
	}
	out := make(HexSet, len(slice))
	for _, h := range slice {
		out[h] = struct{}{}
	}
	*s = out
	return nil
}
