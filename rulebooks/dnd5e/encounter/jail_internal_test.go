// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// jail_internal_test.go holds the two halves of custody's one door
// (rpg-project#925): the engine's own path moves a held member, the exported
// verb refuses the same member; and a jail with nobody to take refuses loudly.

// heldIn is the doorway hall, leaf open, with an arrest holding west.
func heldIn(t *testing.T, custody CustodyPhase) (*Encounter, *interactionRecord) {
	t.Helper()
	enc := doorwayHall(t, DoorIsOpen())
	r := &interactionRecord{arrest: &ArrestRecord{
		State: "arrested", Cell: "hall", Custody: custody,
		Taken: []ArrestedMember{{ID: doorwayWest, InArea: true}},
	}}
	enc.interactions = map[InteractionID]*interactionRecord{"gate": r}
	return enc, r
}

// TestOnlyTheEnginesPathMovesTheHeld: the exported Relocate refuses a held
// member; the engine's relocate with custody set moves the same member, in
// the same run, with the same input.
func TestOnlyTheEnginesPathMovesTheHeld(t *testing.T) {
	enc, _ := heldIn(t, CustodyHeld)
	from, _ := enc.canvas.GetEntityPosition(string(doorwayWest))
	to := enc.field.cellAt(spatial.Position{X: 1, Y: 3})
	in := &RelocateInput{
		Moves: []Relocation{{Member: doorwayWest, To: to}},
		Cause: RelocationCause{Ref: core.Ref{Module: "dnd5e", Type: "consequences", ID: "arrest"}, Words: "Off you go."},
	}

	_, err := enc.Relocate(in)
	var refused *CustodyMoveError
	require.True(t, errors.As(err, &refused), "the exported verb refuses the held: %v", err)
	at, _ := enc.canvas.GetEntityPosition(string(doorwayWest))
	require.Equal(t, from, at, "and moves nobody")

	out, err := enc.relocate(in, true)
	require.NoError(t, err, "the engine's own path moves the held")
	require.Equal(t, to, out.Moved[0].To)
	at, _ = enc.canvas.GetEntityPosition(string(doorwayWest))
	require.Equal(t, to, at)
}

// TestAJailWithNobodyToTakeIsRefused: custody already broken, custody
// already jailed, every taken member gone, and a cell region with no room —
// each refused, nobody moved, nothing recorded.
func TestAJailWithNobodyToTakeIsRefused(t *testing.T) {
	jail := Jail{Words: "Off you go."}
	for name, tc := range map[string]struct {
		custody CustodyPhase
		taken   MemberID
		cell    RegionID
	}{
		"custody broken":       {CustodyBroken, doorwayWest, "hall"},
		"already jailed":       {CustodyJailed, doorwayWest, "hall"},
		"every taken has gone": {CustodyHeld, "gone-player", "hall"},
		"no such cell":         {CustodyHeld, doorwayWest, "nowhere"},
	} {
		t.Run(name, func(t *testing.T) {
			enc, r := heldIn(t, tc.custody)
			r.arrest.Taken[0].ID, r.arrest.Cell = tc.taken, tc.cell
			from, _ := enc.canvas.GetEntityPosition(string(doorwayWest))
			before := len(enc.story.ToData().Entries)

			require.ErrorIs(t, enc.applyJail(r, jail, recognitionLine{interaction: "gate"}), ErrBadPlacement)
			at, _ := enc.canvas.GetEntityPosition(string(doorwayWest))
			require.Equal(t, from, at, "nobody moved")
			require.Equal(t, tc.custody, r.arrest.Custody, "custody unchanged")
			require.Empty(t, r.arrest.Jailed, "nobody recorded jailed")
			require.Len(t, enc.story.ToData().Entries, before, "nothing told")
		})
	}
}
