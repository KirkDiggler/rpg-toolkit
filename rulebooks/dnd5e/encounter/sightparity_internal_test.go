// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// sightparity_internal_test.go holds the sight sweep's two layers
// (visible.go) to one answer when secrets are on the field (rpg-project#958):
// a member who knows the secret and one who does not each learn exactly the
// cells their own sight reaches now — the full per-cell test, the oracle —
// whichever layer answered; and only a view with no face hidden from it is
// answered from the site's shared memo ([hiddenView.facesNone]). Over a
// prop-only secret, a secret hiding floor, a secret door, and a secret
// blocking placed prop, and a mid-run reveal of the last.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	parityKnower   = MemberID("knower")
	parityStranger = MemberID("stranger")
	paritySecret   = ConcealmentID("secret")
	parityRecord   = IntelID("the-secret")
)

// paritySite is the sweep scene's two rooms with one concealment and both
// members in the west room; the knower then comes to hold the record that
// reveals it, as a told or looted record lands, and sight is refreshed.
func paritySite(t *testing.T, placed []PlacedPropInput, secret ConcealmentInput) *Encounter {
	t.Helper()
	field := sweepField()
	field.Placed = placed
	secret.ID, secret.Checks = paritySecret, []CheckApproach{{Ability: "perception", DC: 30}}
	field.Concealments = []ConcealmentInput{secret}
	field.Intel = []IntelRecord{{ID: parityRecord, Reveals: RevealTargets{Concealment: paritySecret}}}
	caps := sweepCapabilities()
	caps.Sheets = sheetFacts{parityKnower: {SpeedFeet: 30}, parityStranger: {SpeedFeet: 30}}
	caps.CheckResolver, caps.Witness = RefusingCheckResolver{}, nobodyPerceives{}
	enc, err := NewEncounter(&SetupInput{
		Sites: NewMemorySites(nil),
		Field: field,
		Members: []MemberInput{
			{ID: parityKnower, Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
			{ID: parityStranger, Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 0}},
		},
		Endings:      []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
		Capabilities: caps,
	})
	require.NoError(t, err)
	tell(t, enc, parityKnower)
	require.True(t, enc.world.knowsConcealment(parityKnower, paritySecret), "precondition: the knower knows it")
	require.False(t, enc.world.knowsConcealment(parityStranger, paritySecret), "precondition: the stranger does not")
	return enc
}

// tell lands the record on m and refreshes everyone's sight.
func tell(t *testing.T, enc *Encounter, m MemberID) {
	t.Helper()
	require.NoError(t, enc.holdings.holdIntel(m, parityRecord, "test"))
	require.NoError(t, enc.applyReveals(m, parityRecord, toldCause, 0))
	_, _, err := enc.refreshSight(enc.rosterIDs())
	require.NoError(t, err)
}

// paritySees is the oracle: every floor cell the member's sight reaches now,
// under their own knowledge, and their own cell.
func paritySees(t *testing.T, enc *Encounter, m MemberID) map[spatial.Position]bool {
	t.Helper()
	cells, err := enc.sightNow()
	require.NoError(t, err)
	positions := map[MemberID]spatial.Position{}
	for id := range enc.members {
		if at, ok := enc.canvas.GetEntityPosition(string(id)); ok {
			positions[id] = at
		}
	}
	reach := enc.newSightReach(positions, cells)
	hidden := enc.hiddenFrom(m)
	out := map[spatial.Position]bool{positions[m]: true}
	for _, cell := range enc.field.cells {
		if reach.seesCell(m, cell, hidden) {
			out[cell] = true
		}
	}
	return out
}

// requireParity: each member knows exactly what the oracle says it sees, and
// its sweep's answer is in the site's shared memo exactly when strangerRaw
// (the knower, who has nothing hidden, always) says its view hides no face.
func requireParity(t *testing.T, enc *Encounter, strangerRaw bool, when string) {
	t.Helper()
	for m, raw := range map[MemberID]bool{parityKnower: true, parityStranger: strangerRaw} {
		require.Equal(t, paritySees(t, enc, m), enc.world.knownCells(m), "%s: %s knows exactly what it sees", when, m)
		key := sightSweepKey(enc.sightSweeps[m])
		require.NotEmpty(t, key, "%s: %s swept", when, m)
		_, shared := enc.site.sight.entries[key]
		require.Equal(t, raw, shared, "%s: %s's answer is the site's only when its view hides no face", when, m)
	}
}

func TestTheSweepsTwoLayersGiveOneAnswerOverSecrets(t *testing.T) {
	at := func(col, row int) spatial.Point {
		return spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{CellWidth: 5, Orientation: spatial.HexOrientationPointyTop}).
			CellCentre(HexCellAt(HexesArePointyTop(), col, row))
	}
	box := func(col, row int) spatial.FootprintPlacement {
		return spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 1}}, Origin: at(col, row)}
	}
	grate := []PlacedPropInput{{ID: "grate", Placement: box(3, 2)}}
	screen := []PlacedPropInput{{ID: "screen", Placement: box(2, 1), BlocksMovement: true, BlocksLineOfSight: true}}

	for name, tc := range map[string]struct {
		placed []PlacedPropInput
		secret ConcealmentInput
		raw    bool
	}{
		"a prop-only secret":       {grate, ConcealmentInput{Props: []PropID{"grate"}}, true},
		"a secret hiding floor":    {nil, ConcealmentInput{Cells: []spatial.Position{{X: 3, Y: 0}}}, true},
		"a secret door":            {nil, ConcealmentInput{Doors: []DoorID{sweepNeckDor}}, false},
		"a secret blocking screen": {screen, ConcealmentInput{Props: []PropID{"screen"}}, false},
	} {
		t.Run(name, func(t *testing.T) {
			requireParity(t, paritySite(t, tc.placed, tc.secret), tc.raw, "at first light")
		})
	}

	t.Run("the screen revealed mid-run", func(t *testing.T) {
		enc := paritySite(t, screen, ConcealmentInput{Props: []PropID{"screen"}})
		under := enc.field.placedCells(enc.field.placed[enc.field.placedIndexOf("screen")].placement)
		require.NotEmpty(t, under)
		require.True(t, enc.world.knownCells(parityKnower)[under[0]], "precondition: the screen's face teaches the knower")
		require.False(t, enc.world.knownCells(parityStranger)[under[0]], "the stranger learns nothing behind a face it cannot see")

		tell(t, enc, parityStranger)
		require.True(t, enc.world.knownCells(parityStranger)[under[0]], "told, the stranger's next sweep reads the face")
		requireParity(t, enc, true, "after the reveal")
	})
}
