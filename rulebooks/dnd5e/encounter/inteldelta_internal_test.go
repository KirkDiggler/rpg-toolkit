// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

func TestIntelDeltaCopiesSurveilOutput(t *testing.T) {
	in := &intel.SurveilOutput{
		FirstContact: []intel.Report{{Subject: "billy", Payload: []byte("known")}},
		Refreshed:    []intel.Subject{"david"},
		Faded:        []intel.Subject{"alice"},
	}

	got := intelDeltaFromSurveil(in)
	in.FirstContact[0].Payload[0] = 'X'
	in.FirstContact[0].Subject = "changed"
	in.Refreshed[0] = "changed"
	in.Faded[0] = "changed"

	require.Equal(t, []intel.Report{{Subject: "billy", Payload: []byte("known")}}, got.FirstContact)
	require.Equal(t, []intel.Subject{"david"}, got.Refreshed)
	require.Equal(t, []intel.Subject{"alice"}, got.Faded)
	require.Empty(t, got.Corrected)
}

func TestMergeIntelDeltasDeduplicatesCategoriesInFirstOccurrenceOrder(t *testing.T) {
	dst := map[MemberID]*IntelDelta{
		"observer": {
			FirstContact: []intel.Report{
				{Subject: "billy", Payload: []byte("first")},
				{Subject: "billy", Payload: []byte("duplicate")},
			},
			Refreshed: []intel.Subject{"david", "david"},
			Faded:     []intel.Subject{"alice"},
			Corrected: []intel.Subject{"alice"},
		},
	}
	originalDst := dst["observer"]
	src := map[MemberID]*IntelDelta{
		"observer": {
			FirstContact: []intel.Report{
				{Subject: "billy", Payload: []byte("later")},
				{Subject: "charlie", Payload: []byte("new")},
				{Subject: "charlie", Payload: []byte("duplicate")},
			},
			Refreshed: []intel.Subject{"david", "erin", "david"},
			Faded:     []intel.Subject{"alice", "bob", "alice"},
			Corrected: []intel.Subject{"alice", "bob", "alice"},
		},
	}

	got := mergeIntelDeltas(dst, src)
	require.Equal(t, &IntelDelta{
		FirstContact: []intel.Report{
			{Subject: "billy", Payload: []byte("first")},
			{Subject: "charlie", Payload: []byte("new")},
		},
		Refreshed: []intel.Subject{"david", "erin"},
		Faded:     []intel.Subject{"alice", "bob"},
		Corrected: []intel.Subject{"alice", "bob"},
	}, got["observer"])

	src["observer"].FirstContact[1].Payload[0] = 'X'
	originalDst.FirstContact[0].Payload[0] = 'X'
	require.Equal(t, []byte("first"), got["observer"].FirstContact[0].Payload)
	require.Equal(t, []byte("new"), got["observer"].FirstContact[1].Payload)
}
