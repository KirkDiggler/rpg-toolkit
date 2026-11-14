// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestAHeldDoerIsRefusedTheThingWhole (rpg-project#959): on the escape site,
// the prisoner the guards hold does a thing whose row would set them down in
// the cell, and it is refused whole — the guards' sentence, nothing written,
// nobody moved.
func TestAHeldDoerIsRefusedTheThingWhole(t *testing.T) {
	no := false
	hatch := PropInput{
		ID: "hatch", Ref: "dnd5e:props:test:hatch", At: spatial.Position{X: 3, Y: 2},
		BlocksMovement: &no, BlocksLineOfSight: &no,
		Options: []Option{{
			ID: "drop", Do: "You drop through the hatch.", Success: []Answer{{Weight: 1}},
			Grants: []RowGrant{{Consequence: Consequence{Relocate: &RelocateGrant{To: escCell, Words: "Down you go."}}}},
		}},
	}
	enc := escapeSiteWith(t, CustodyHeld, spatial.Position{X: 3, Y: 1}, []PropInput{hatch})
	where := func() spatial.Position {
		at, ok := enc.canvas.GetEntityPosition(string(escPris))
		require.True(t, ok)
		return at
	}
	before := where()
	story, err := enc.Story(&StoryInput{Audience: escPris})
	require.NoError(t, err)

	_, err = enc.Interact(&InteractInput{Actor: escPris, Prop: "hatch", OptionID: "drop", Range: 3, Roller: lowestDie{}})
	require.ErrorIs(t, err, ErrInCustody)
	require.Equal(t, before, where(), "nobody moved")
	after, err := enc.Story(&StoryInput{Audience: escPris})
	require.NoError(t, err)
	require.Len(t, after, len(story), "the thing wrote nothing")
}
