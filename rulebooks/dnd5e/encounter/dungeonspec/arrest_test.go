// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// arrest_test.go is the arrest outcome's grammar (rpg-project#925): `arrest:
// { cell, area }` on an interaction's outcome, every mistake refused at the
// author's own path, and the engine refusing an area cell off the floor.

// theArrest is the castle's authored arrest, as the castle writes it.
const theArrest = `            cell: region-cells
            area:
              regions: [region-gatehouse]
              cells:
                - {r: 13, q: [[-8, -5]]}
                - {r: 14, q: [[-8, -6]]}
`

func TestDungeonspec_TheArrestIsRefusedAtTheAuthorsPath(t *testing.T) {
	const at = "interactions.front-gate.outcomes.arrested[0]"
	for _, test := range []struct {
		name, replacement, path, message string
	}{
		{
			name:        "a cell that is not a region",
			replacement: "            cell: the-dungeon\n            area:\n              regions: [region-gatehouse]\n",
			path:        at + ".arrest.cell", message: `"the-dungeon" is not a region of this site`,
		},
		{
			name:        "no cell",
			replacement: "            cell: ''\n            area:\n              regions: [region-gatehouse]\n",
			path:        at + ".arrest.cell", message: "an arrest names the region the arrested are held in",
		},
		{
			name:        "no area",
			replacement: "            cell: region-cells\n            area: {}\n",
			path:        at + ".arrest.area", message: "an arrest's area names its regions or its cells: no area is chosen for you",
		},
		{
			name:        "an area region that is not a region",
			replacement: "            cell: region-cells\n            area:\n              regions: [the-road]\n",
			path:        at + ".arrest.area.regions[0]", message: `"the-road" is not a region of this site`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireExactDefect(t, refusals(t, castleWith(t, theArrest, test.replacement)), test.path, test.message)
		})
	}

	t.Run("both kinds in one consequence", func(t *testing.T) {
		requireExactDefect(t, refusals(t, castleWith(t, "        - arrest:\n"+theArrest,
			"        - recognize: {topic: arrested, holder: spoken_to, about: party_in_sight}\n          arrest:\n"+theArrest)),
			at, "a consequence is one kind: `recognize` or `arrest`, not both")
	})
}

// TestDungeonspec_AnArrestAreaOffTheFloorIsRefusedAtLaunch: the area's cells
// are judged against the floor when the run is built, by the engine.
func TestDungeonspec_AnArrestAreaOffTheFloorIsRefusedAtLaunch(t *testing.T) {
	c := loadSource(t, castleWith(t, "                - {r: 14, q: [[-8, -6]]}\n", "                - {r: 14, q: [[-8, -6]]}\n                - {r: 60, q: [[60, 60]]}\n"))
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        c.Field,
		Endings:      []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.RefusingCapabilities(),
	})
	require.ErrorIs(t, err, encounter.ErrBadArrest)
	require.ErrorContains(t, err, "which is not floor")
}
