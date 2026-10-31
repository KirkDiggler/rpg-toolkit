// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLayout_ScalesAndFills is design R2: at 10,000 hexes the keep is scale
// 1 and the floor 10,000; at 30,000 the rooms scale 1.5 (the cap) and the
// floor is 30,000 exactly; no room grows past 1.5 times its plan footprint;
// and the room ids, door pairs and names are the keep scale's.
func TestLayout_ScalesAndFills(t *testing.T) {
	p := DefaultPlan()
	small, err := layoutOf(p, 10000)
	require.NoError(t, err)
	require.Equal(t, 1.0, small.scale)
	require.Equal(t, 10000, small.floor())

	large, err := layoutOf(p, 30000)
	require.NoError(t, err)
	require.Equal(t, 1.5, large.scale)
	require.Equal(t, 30000, large.floor())

	for _, r := range p.Rooms {
		g := large.regions[large.index[r.ID]]
		require.LessOrEqual(t, float64(g.rect.width()), 1.5*float64(r.Width), r.ID)
		require.LessOrEqual(t, float64(g.rect.depth()), 1.5*float64(r.Depth), r.ID)
		s := small.regions[small.index[r.ID]]
		require.Equal(t, r.Width, s.rect.width(), "%s is its plan width at scale 1", r.ID)
		require.Equal(t, r.Depth, s.rect.depth(), "%s is its plan depth at scale 1", r.ID)
	}

	ids := func(l *layout) []string {
		var out []string
		for _, g := range l.regions {
			out = append(out, g.id+"|"+g.name)
		}
		return out
	}
	require.Equal(t, ids(small), ids(large), "the same regions at both sizes")
	s10 := keepAt(t, 10000).site
	s30 := keepAt(t, 30000).site
	doors := func(s *site) []string {
		var out []string
		for _, w := range s.walls {
			if w.door != nil {
				out = append(out, w.door.id)
			}
		}
		return out
	}
	require.Equal(t, doors(s10), doors(s30), "the same doors at both sizes")
	require.Len(t, doors(s10), len(p.Doors))

	// Every cell is one region's: the floor count is the union, not a sum
	// that hides an overlap.
	require.Len(t, large.owners(), 30000)
	require.Len(t, small.owners(), 10000)

	// THE CAP HOLDS FOR ODD WIDTHS (review of rpg-toolkit#2204): a tower 5
	// wide (mess hall 11, so the south side still spans 80) at 19,968 hexes,
	// scale 1.4131, once rounded to 8 wide, past 1.5 x 5. Floored, it is
	// at most 7, at that size and at every size up to the ceiling.
	odd, err := LoadPlan(strings.NewReader(strings.Replace(planWith(t,
		"{ id: tower, name: Tower, kind: hall, width: 6,", "{ id: tower, name: Tower, kind: hall, width: 5,"),
		"{ id: mess-hall, name: Mess Hall, kind: hall, width: 10,", "{ id: mess-hall, name: Mess Hall, kind: hall, width: 11,", 1)))
	require.NoError(t, err)
	for _, hexes := range []int{19968, 10000, 12000, 15000, 18000, 21000, 22499, 30000} {
		l, err := layoutOf(odd, hexes)
		require.NoError(t, err, hexes)
		require.Equal(t, hexes, l.floor(), hexes)
		require.Len(t, l.owners(), hexes, hexes)
		for _, r := range odd.Rooms {
			g := l.regions[l.index[r.ID]]
			require.LessOrEqual(t, float64(g.rect.width()), 1.5*float64(r.Width), "%s at %d hexes", r.ID, hexes)
			require.LessOrEqual(t, float64(g.rect.depth()), 1.5*float64(r.Depth), "%s at %d hexes", r.ID, hexes)
			require.Positive(t, g.rect.width()*g.rect.depth(), r.ID)
		}
	}
}

// TestLayout_FillsAnyCount: the grounds absorb the remainder at sizes that
// do not divide evenly.
func TestLayout_FillsAnyCount(t *testing.T) {
	for _, hexes := range []int{10001, 12345, 22499, 22500, 30001} {
		l, err := layoutOf(DefaultPlan(), hexes)
		require.NoError(t, err, hexes)
		require.Equal(t, hexes, l.floor(), hexes)
		require.Len(t, l.owners(), hexes, hexes)
		require.LessOrEqual(t, l.scale, 1.5)
	}
}

// TestLayout_RefusesBelowPlan: fewer hexes than the plan's own floor.
func TestLayout_RefusesBelowPlan(t *testing.T) {
	_, err := layoutOf(DefaultPlan(), 9999)
	require.ErrorContains(t, err, "the plan needs at least 10000 hexes")
}

// TestLayout_RefusesSidesThatDoNotClose: a north room deeper than its side,
// and a south side shorter than the north.
func TestLayout_RefusesSidesThatDoNotClose(t *testing.T) {
	p, err := LoadPlan(strings.NewReader(planWith(t,
		"{ id: treasury, name: Treasury, kind: service, width: 8, depth: 16,",
		"{ id: treasury, name: Treasury, kind: service, width: 8, depth: 18,")))
	require.NoError(t, err)
	_, err = layoutOf(p, 10000)
	require.ErrorContains(t, err, `north rooms share one depth: "bedrooms" is 16, "treasury" is 18`)

	p, err = LoadPlan(strings.NewReader(planWith(t,
		"{ id: tower, name: Tower, kind: hall, width: 6,",
		"{ id: tower, name: Tower, kind: hall, width: 4,")))
	require.NoError(t, err)
	_, err = layoutOf(p, 10000)
	require.ErrorContains(t, err, "the north side spans 80 hexes and the south side 78")
}
