// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestSheetOfRefusesAnIDOutsideTheRoster: every caller checks membership
// first today, so this is the guard for the next one that does not — an id
// nobody placed gets ErrNotMember, never a zero sheet nobody gave. Both an
// occupied roster and an empty one, since sheetsNow answers an empty roster
// without asking.
func TestSheetOfRefusesAnIDOutsideTheRoster(t *testing.T) {
	for name, members := range map[string][]MemberInput{
		"a roster without it": {{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}}},
		"an empty roster":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			enc, err := NewEncounter(&SetupInput{
				Field: FieldInput{
					Canvas:  CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()},
					Regions: []RegionInput{rectRegion("hall", 0, 0, 6, 6)},
				},
				Members: members,
				Endings: []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
				Capabilities: Capabilities{
					Sight:      everyoneSeesTheWholeMap{},
					Equipment:  UnobservedEquipment{},
					Sheets:     sheetFacts{"alice": {SpeedFeet: 30}},
					Standing:   everyoneStanding{},
					Initiative: orderAsGiven{},
					Driver:     passDriver{},
					Actors: Actors{
						Striker:   passStriker{},
						Mover:     quietMover{},
						Announcer: quietAnnouncer{},
					},
				},
			})
			require.NoError(t, err)

			_, err = enc.sheetOf("ghost")
			require.ErrorIs(t, err, ErrNotMember)
		})
	}
}
