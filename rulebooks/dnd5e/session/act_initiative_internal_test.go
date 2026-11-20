// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Catches a fight-formed entry quietly dropping or half-building an initiative
// roll whose calculation does not replay: the story log must refuse it, as it
// refuses every other roll it cannot show.
func TestClockPiecesRefusesAnInitiativeRollThatDoesNotReplay(t *testing.T) {
	payload := []byte(`{"order":["a"],"surprised":[],"rolls":[` +
		`{"member":"a","roll":10,"total":14,"calculation":{"components":"not a list","total":3}}]}`)

	pieces, err := clockPieces(encounter.BeatFightStarted, payload)

	require.Error(t, err)
	require.Nil(t, pieces)
}
