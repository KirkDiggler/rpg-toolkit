// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// v2_door_opened_test.go is rpg-project#569 R12 in the v2 dialect: a door
// written under `doors:` takes the same `opened: { fact }` the single room's
// binding does, judged by the one grammar at `doors[<i>].opened.fact`.

// hallTombDoor is the heirloom tomb's locked door, as the fixture writes it.
const hallTombDoor = `  - id: hall-tomb
    at: { cell: [16,4], offset: [-0.25, 0.375] }
    locked: [{ ability: dex, dc: 12 }]
`

// tombDoorOpened is the heirloom tomb with the hall-tomb door's `opened`
// written as given.
func tombDoorOpened(t *testing.T, opened string) string {
	t.Helper()
	source := heirloomSource(t)
	require.Contains(t, source, hallTombDoor, "the fixture still has the door these scenes edit")

	return strings.Replace(source, hallTombDoor, hallTombDoor+"    opened: "+opened+"\n", 1)
}

func TestAV2DoorThatOpensTeachesAFact(t *testing.T) {
	t.Run("the fact lowers onto the door", func(t *testing.T) {
		compiled, err := dungeonspec.Load([]byte(tombDoorOpened(t, "{ fact: tomb-opened }")))
		require.NoError(t, err)
		var opened encounter.FactID
		for _, d := range compiled.Field.Doors {
			if strings.HasSuffix(string(d.ID), "/hall-tomb") {
				opened = d.Opened
			}
		}
		require.Equal(t, encounter.FactID("tomb-opened"), opened)
	})

	t.Run("no fact is refused at the door's path", func(t *testing.T) {
		requireDefect(t, defectsIn(t, tombDoorOpened(t, "{}")),
			"doors[1].opened.fact: opening this door teaches a fact, and this does not say which")
	})

	t.Run("an unknown key under opened is named", func(t *testing.T) {
		requireDefect(t, decodeDefects(t, tombDoorOpened(t, "{ fact: tomb-opened, when: open }")),
			"doors[1].opened.when", `"when" is not a key this build reads: they are fact`)
	})
}
