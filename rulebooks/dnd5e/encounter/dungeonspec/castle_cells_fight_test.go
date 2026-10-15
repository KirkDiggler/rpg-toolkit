// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_cells_fight_test.go is D0 ON THE ROOM THE CASTLE SHIPS
// (rpg-project#569): the mad king's cells, rpg-api's content, with one line
// added to a copy — the prisoners hostile to the garrison — and nothing else.
// A guard and a prisoner who see each other go to initiative with the party
// nowhere in it, which before D0 nothing in the engine could do: classify
// paired players with monsters, so two hostile monster factions in plain
// sight stood there.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// cellsAtWar is the shipped cells with the prisoners declared hostile to
// the garrison — the only change, made to a copy in the test.
func cellsAtWar(t *testing.T) dungeonspec.Compiled {
	t.Helper()
	raw, err := os.ReadFile(cellsFixture) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	const anchor = "dispositions:\n"
	require.Contains(t, string(raw), anchor, "the cells declare dispositions")
	source := strings.Replace(string(raw), anchor,
		anchor+"  - between: [garrison, prisoners]\n    stance: hostile\n", 1)

	return loadSource(t, source)
}

// cellsWarder is the guard the scene posts INSIDE the cell block, at
// [-7, 10], beside the prisoner at [-7, 11]: the file's guards stand in the
// corridor behind a locked door, where nobody in a cell can see them, so a
// fight "on sight" needs a guard where the prisoners can see him.
const cellsWarder = encounter.MemberID("44f4aa72-bd03-46b4-b8f4-d5dc2e895cff")

// cellsCorridorGuard is the other guard, left where the file posts him.
const cellsCorridorGuard = encounter.MemberID("7b8b5d1c-1b8f-4ea1-a681-5abab9420d0a")

// TestAGuardAndAPrisonerFightOnSightWithoutTheParty: at first light, with
// the party at the file's own start, the warder and the prisoners who can see
// him go to initiative, and no player is in it.
func TestAGuardAndAPrisonerFightOnSightWithoutTheParty(t *testing.T) {
	warderInside := map[encounter.MemberID]spatial.Position{cellsWarder: {X: -7, Y: 10}}

	t.Run("control: as shipped, the two factions are neutral and nobody fights", func(t *testing.T) {
		compiled := loadContent(t, cellsFixture)
		enc := cellsRun(t, compiled, compiled.PartyStart[0].At, everyoneSeesTheWholeMap{}, warderInside)
		require.Equal(t, encounter.ClockWorld, clockKind(t, enc, cellsWarder))
	})

	t.Run("declared hostile: the warder and the prisoners fight, the party is not in it", func(t *testing.T) {
		compiled := cellsAtWar(t)
		enc := cellsRun(t, compiled, compiled.PartyStart[0].At, everyoneSeesTheWholeMap{}, warderInside)

		require.Equal(t, encounter.ClockTurn, clockKind(t, enc, cellsWarder), "the warder is in a fight")
		prisoners := 0
		for _, m := range compiled.Monsters {
			if m.Faction == "prisoners" && clockKind(t, enc, encounter.MemberID(m.MemberID)) == encounter.ClockTurn {
				prisoners++
			}
		}
		require.Positive(t, prisoners, "against the prisoners who can see him")
		require.Equal(t, encounter.ClockWorld, clockKind(t, enc, cellsCorridorGuard),
			"the guard behind the locked door saw nothing and is in no fight")
		require.Equal(t, encounter.ClockWorld, clockKind(t, enc, cellsPicker),
			"and the party, at the file's start, is not in it either")
	})
}

// clockKind is the clock one member is on.
func clockKind(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) encounter.ClockKind {
	t.Helper()
	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
	require.NoError(t, err)

	return on.Kind
}
