// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// launchscene_internal_test.go is launchscene_test.go's twin for the suites
// that live inside the package: the same scene, the same one Launch, written
// down again only because an internal test file cannot import an external one.

// scene is a run in a form a test can write down: the floor, the party already
// saved in the fake repository, the monsters on the board, and the dungeon's
// own endings (a launch declares [EndingWithdrawn] itself).
type scene struct {
	Field    encounter.FieldInput
	Party    []sceneSeat
	Monsters []dungeonspec.MonsterPlacement
	Endings  []encounter.EndingInput
}

// sceneSeat is one party member and the authored cell they start on.
type sceneSeat struct {
	ID string
	At spatial.Position
}

func seatAt(id string, col, row int) sceneSeat {
	return sceneSeat{ID: id, At: spatial.Position{X: float64(col), Y: float64(row)}}
}

func monsterAt(id, ref string, col, row int) dungeonspec.MonsterPlacement {
	return dungeonspec.MonsterPlacement{
		Ref: ref, ID: id, MemberID: id, At: spatial.Position{X: float64(col), Y: float64(row)},
	}
}

// launchScene builds the dungeonspec.Compiled literal a scene stands for and
// launches it under the session id "sess".
func launchScene(t *testing.T, m *Manager, sc scene) *LaunchOutput {
	t.Helper()
	seats := make([]dungeonspec.Seat, 0, len(sc.Party))
	party := make([]string, 0, len(sc.Party))
	for _, seat := range sc.Party {
		seats = append(seats, dungeonspec.Seat{At: seat.At})
		party = append(party, seat.ID)
	}
	endings := make([]encounter.EndingInput, 0, len(sc.Endings))
	for _, ending := range sc.Endings {
		if ending.Key != EndingWithdrawn {
			endings = append(endings, ending)
		}
	}
	out, err := m.Launch(context.Background(), &LaunchInput{
		Session: "sess",
		Dungeon: &dungeonspec.Compiled{
			Field: sc.Field, PartyStart: seats, Monsters: sc.Monsters, Endings: endings,
		},
		Party: party,
	})
	require.NoError(t, err, "launching the scene")
	return out
}

// authorTurnClock replaces the stored run's clock for the named members with
// one authored turn clock whose active member is order[active], so a fight
// that never formed from contact is still somebody's turn. The caller hands in
// the stored run itself, not a copy of it.
func authorTurnClock(t *testing.T, stored *encounter.EncounterData, order []string, active int) {
	t.Helper()
	ids := make([]core.EntityID, 0, len(order))
	for _, member := range order {
		delete(stored.Clock.Budgets, core.EntityID(member))
		ids = append(ids, core.EntityID(member))
	}
	raw, err := json.Marshal([]struct {
		Order     []core.EntityID `json:"order"`
		ActiveIdx int             `json:"active_idx"`
		Round     int             `json:"round"`
	}{{Order: ids, ActiveIdx: active, Round: 1}})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &stored.Bubbles))
}
