// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
)

// TestNilInputsRefused sweeps every Input-taking function with a nil
// Input (and nil required fields) and asserts ErrNilInput — the
// deliberate-over-incidental revision of design R3.
func TestNilInputsRefused(t *testing.T) {
	turn := &clock.Turn{}
	_, err := turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	require.NoError(t, err)
	world, werr := clock.NewTick()
	require.NoError(t, werr)

	cases := []struct {
		name string
		call func() error
	}{
		{"Turn.SetOrder", func() error { _, err := turn.SetOrder(nil); return err }},
		{"Turn.End", func() error { _, err := turn.End(nil); return err }},
		{"Turn.Insert", func() error { _, err := turn.Insert(nil); return err }},
		{"Turn.Remove", func() error { _, err := turn.Remove(nil); return err }},
		{"Turn.Merge", func() error { _, err := turn.Merge(nil); return err }},
		{"Turn.Merge nil Other", func() error {
			_, err := turn.Merge(&clock.MergeInput{Order: []core.EntityID{"a"}})
			return err
		}},
		{"Turn.Dissolve", func() error { _, err := turn.Dissolve(nil); return err }},
		{"Turn.Contains", func() error { _, err := turn.Contains(nil); return err }},
		{"Turn.JoinMember", func() error { _, err := turn.JoinMember(nil); return err }},
		{"Turn.LeaveMember", func() error { _, err := turn.LeaveMember(nil); return err }},
		{"Tick.Join", func() error { _, err := world.Join(nil); return err }},
		{"Tick.Leave", func() error { _, err := world.Leave(nil); return err }},
		{"Tick.Advance", func() error { _, err := world.Advance(nil); return err }},
		{"Tick.Spend", func() error { _, err := world.Spend(nil); return err }},
		{"Tick.Budget", func() error { _, err := world.Budget(nil); return err }},
		{"Tick.Contains", func() error { _, err := world.Contains(nil); return err }},
		{"Tick.JoinMember", func() error { _, err := world.JoinMember(nil); return err }},
		{"Tick.LeaveMember", func() error { _, err := world.LeaveMember(nil); return err }},
		{"Transfer nil input", func() error { _, err := clock.Transfer(nil); return err }},
		{"Transfer nil From", func() error {
			_, err := clock.Transfer(&clock.TransferInput{To: turn, ID: "a"})
			return err
		}},
		{"Transfer nil To", func() error {
			_, err := clock.Transfer(&clock.TransferInput{From: turn, ID: "a"})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.call(), clock.ErrNilInput)
		})
	}
	// guards precede all mutation: the populated turn is untouched
	active, err := turn.Active()
	require.NoError(t, err)
	require.Equal(t, core.EntityID("a"), active)
}
