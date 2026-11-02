// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// TestAdoptKeepsTheWaitingReserveKinds guards adopt's waiting-kinds merge
// (rpg-project#753, T12). The castle's far room is two guards waiting in
// reserve on `gate-opened`. adopt reads placed kinds off the encounter's
// roster, which holds no waiting member, so the waiting kinds survive only
// because adopt keeps them from the scope's seeded map. The walk-4 fix needs
// them at the record: an arrival inside it asks the seams about the newcomer
// at once, and a newcomer with no roster kind is refused by name.
func TestAdoptKeepsTheWaitingReserveKinds(t *testing.T) {
	ctx := context.Background()
	mgr, err := NewManager(&Config{Record: DiscardRecord{}, Acts: DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: &scriptedDice{rolls: []int{10, 10, 10, 10, 10, 10}}, TurnDriver: Pass{},
		Sessions:   &strikeSessions{byID: map[string]*SessionData{}},
		Encounters: &strikeEncounters{byID: map[string]*encounter.EncounterData{}},
		Characters: &strikeCharacters{byID: map[string]*character.Data{"fighter": strikeFixtureFighter("fighter")}},
	})
	require.NoError(t, err)

	farRoom := []string{"far-guard-1", "far-guard-2"}
	sc := scene{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("gatehouse", 0, 0, 12, 6)},
		},
		Endings: []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Party:   []sceneSeat{seatAt("fighter", 2, 0)},
		Monsters: []dungeonspec.MonsterPlacement{
			monsterAt("gate-1", refs.Monsters.Goblin().String(), 3, 0),
		},
	}
	for i, id := range farRoom {
		guard := monsterAt(id, refs.Monsters.Goblin().String(), 8+i, 4)
		guard.Arrives = encounter.TriggerFact{Fact: "gate-opened"}
		sc.Monsters = append(sc.Monsters, guard)
	}
	launchScene(t, mgr, sc)

	scope, err := mgr.openForChange(ctx, "attack", "sess")
	require.NoError(t, err)
	placed, err := scope.enc.Members()
	require.NoError(t, err)
	for _, member := range placed {
		require.NotContains(t, farRoom, string(member.ID), "precondition: the far room waits")
	}

	var atRecord map[string]encounter.MemberKind
	_, err = mgr.land(ctx, scope, &resolution.Output{}, &landing{
		Record: func(*encounter.Encounter, concentration) error {
			atRecord = scope.standing.kinds
			return nil
		},
	})
	require.NoError(t, err)

	require.Equal(t, encounter.KindPlayer, atRecord["fighter"], "a placed member's kind is read off the roster")
	require.Equal(t, encounter.KindMonster, atRecord["gate-1"], "a placed member's kind is read off the roster")
	for _, id := range farRoom {
		require.Equal(t, encounter.KindMonster, atRecord[id], "%s waits in reserve and keeps its kind through adopt", id)
	}
}
