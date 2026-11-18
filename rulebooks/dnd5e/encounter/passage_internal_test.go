// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// passage_internal_test.go is what the passage (rpg-project#958) needs inside
// the package: a held speaker refused the line whole, and the cheap answers a
// field whose secrets are props alone gives.

// TestAHeldSpeakerIsRefusedThePassageWhole: on the escape site, the prisoner
// the guards hold says a line whose row would set them down in the cell, and
// the line is refused whole — the guards' sentence, nothing written, nobody
// moved.
func TestAHeldSpeakerIsRefusedThePassageWhole(t *testing.T) {
	at := spatial.Position{X: 3, Y: 1}
	enc := escapeSite(t, CustodyHeld, at)
	enc.members[escGuard].Options = []Option{{
		ID: "go", Say: "Take me down.", Success: []Answer{{Weight: 1}},
		Grants: []RowGrant{{Consequence: Consequence{Relocate: &RelocateGrant{To: escCell, Words: "Down you go."}}}},
	}}
	where := func() spatial.Position {
		members, err := enc.Members()
		require.NoError(t, err)
		for _, m := range members {
			if m.ID == escPris {
				return m.Position
			}
		}
		t.Fatal("no prisoner")
		return spatial.Position{}
	}
	before := where()
	story, err := enc.Story(&StoryInput{Audience: escPris})
	require.NoError(t, err)

	_, err = enc.Speak(context.Background(), &SpeakInput{
		Actor: escPris, Target: escGuard, OptionID: "go", Range: 3, Roller: lowestDie{},
	})
	require.ErrorIs(t, err, ErrInCustody)
	require.Equal(t, before, where(), "nobody moved")
	after, err := enc.Story(&StoryInput{Audience: escPris})
	require.NoError(t, err)
	require.Len(t, after, len(story), "the line wrote nothing")
}

// TestAPropOnlySecretCostsNoSweep holds the two cheap answers a field whose
// secrets are props gives (rpg-project#958): a hidden prop that blocks no
// sight, or stands on a cell, is no face a sweep reads; one that blocks
// sight is. And no crossing masquerades until a secret hides floor.
func TestAPropOnlySecretCostsNoSweep(t *testing.T) {
	no := false
	box := func(col, row int) spatial.FootprintPlacement {
		return spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 1}},
			Origin: spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{CellWidth: 5, Orientation: spatial.HexOrientationPointyTop}).
				CellCentre(HexCellAt(HexesArePointyTop(), col, row))}
	}
	caps := escapeCapabilities()
	caps.CheckResolver, caps.Witness = RefusingCheckResolver{}, nobodyPerceives{}
	site := func(cells []spatial.Position) *Encounter {
		enc, err := NewEncounter(&SetupInput{
			Field: FieldInput{
				Canvas:  openAir(),
				Regions: []RegionInput{rectRegion("hall", 0, 0, 10, 5)},
				Props:   []PropInput{{ID: "rug", Ref: "dnd5e:props:rug", At: spatial.Position{X: 1, Y: 1}, BlocksMovement: &no, BlocksLineOfSight: &no}},
				Placed: []PlacedPropInput{
					{ID: "grate", Placement: box(4, 2)},
					{ID: "screen", Placement: box(7, 2), BlocksMovement: true, BlocksLineOfSight: true},
				},
				Concealments: []ConcealmentInput{{
					ID: "secret", Checks: []CheckApproach{{Ability: "perception", DC: 30}}, Props: []PropID{"rug", "grate", "screen"},
					Cells: cells,
				}},
			},
			Members:      []MemberInput{{ID: "hero", Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 0}}},
			Endings:      []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
			Capabilities: caps,
		})
		require.NoError(t, err)
		return enc
	}
	enc := site(nil)
	require.False(t, enc.field.mayFaceSight("rug"), "a cell prop is never a face")
	require.False(t, enc.field.mayFaceSight("grate"), "a placed prop that blocks no sight is no face")
	require.True(t, enc.field.mayFaceSight("screen"))
	require.True(t, enc.field.mayFaceSight("nothing"), "an unknown id is never cheaper")
	require.False(t, enc.field.concealsFloorOrDoors())
	require.True(t, site([]spatial.Position{{X: 9, Y: 4}}).field.concealsFloorOrDoors())

	hidden := enc.hiddenFrom("hero")
	require.False(t, hidden.facesNone(enc.field), "the hidden screen is a face")
	delete(hidden.props, "screen")
	require.True(t, hidden.facesNone(enc.field), "the rug and the grate are not")
}
