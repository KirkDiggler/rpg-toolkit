// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

func TestIntelDeltaCopiesPerceptionDelta(t *testing.T) {
	in := &perception.Delta{
		FirstContact: []perception.Presence{{ID: "billy", Payload: []byte("known")}},
		Refreshed:    []core.EntityID{"david"},
		Faded:        []core.EntityID{"alice"},
		Changed:      []core.EntityID{"erin"},
		Reacquired:   []core.EntityID{"frank"},
	}

	got := intelDeltaFromPerception(in)
	in.FirstContact[0].Payload[0] = 'X'
	in.FirstContact[0].ID = "changed"
	in.Refreshed[0] = "changed"
	in.Faded[0] = "changed"
	in.Changed[0] = "changed"
	in.Reacquired[0] = "changed"

	require.Equal(t, []perception.Presence{{ID: "billy", Payload: []byte("known")}}, got.FirstContact)
	require.Equal(t, []core.EntityID{"david"}, got.Refreshed)
	require.Equal(t, []core.EntityID{"alice"}, got.Faded)
	require.Equal(t, []core.EntityID{"erin"}, got.Changed)
	require.Equal(t, []core.EntityID{"frank"}, got.Reacquired)
}

func TestMergeIntelDeltasDeduplicatesCategoriesInFirstOccurrenceOrder(t *testing.T) {
	dst := map[MemberID]*IntelDelta{
		"observer": {
			FirstContact: []perception.Presence{
				{ID: "billy", Payload: []byte("first")},
				{ID: "billy", Payload: []byte("duplicate")},
			},
			Refreshed:  []core.EntityID{"david", "david"},
			Faded:      []core.EntityID{"alice"},
			Changed:    []core.EntityID{"alice"},
			Reacquired: []core.EntityID{"alice"},
		},
	}
	originalDst := dst["observer"]
	src := map[MemberID]*IntelDelta{
		"observer": {
			FirstContact: []perception.Presence{
				{ID: "billy", Payload: []byte("later")},
				{ID: "charlie", Payload: []byte("new")},
				{ID: "charlie", Payload: []byte("duplicate")},
			},
			Refreshed:  []core.EntityID{"david", "erin", "david"},
			Faded:      []core.EntityID{"alice", "bob", "alice"},
			Changed:    []core.EntityID{"alice", "bob", "alice"},
			Reacquired: []core.EntityID{"alice", "bob", "alice"},
		},
	}

	got := mergeIntelDeltas(dst, src)
	require.Equal(t, &IntelDelta{
		FirstContact: []perception.Presence{
			{ID: "billy", Payload: []byte("first")},
			{ID: "charlie", Payload: []byte("new")},
		},
		Refreshed:  []core.EntityID{"david", "erin"},
		Faded:      []core.EntityID{"alice", "bob"},
		Changed:    []core.EntityID{"alice", "bob"},
		Reacquired: []core.EntityID{"alice", "bob"},
	}, got["observer"])

	src["observer"].FirstContact[1].Payload[0] = 'X'
	originalDst.FirstContact[0].Payload[0] = 'X'
	require.Equal(t, []byte("first"), got["observer"].FirstContact[0].Payload)
	require.Equal(t, []byte("new"), got["observer"].FirstContact[1].Payload)
}
