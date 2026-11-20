// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// party_status_internal_test.go is SEVERAL RECEIVERS, ALL OR NOTHING
// (rpg-project#990) at the one place a session gives a row's statuses
// ([Manager.giveStatuses]), on a plain store, where a landed sheet is saved
// at once. Through Speak, a receiver the rules have no sheet for is refused
// earlier, by the verb's own sight (party_status_test.go); this pins the
// give itself, so a later route that reaches it cannot keep half a line.

// heldGrant is the hold, given to who.
func heldGrant(who encounter.MemberID) encounter.GrantedStatus {
	return encounter.GrantedStatus{Member: who, Grant: encounter.StatusGrant{ID: "held", To: "party_in_sight",
		Status: json.RawMessage(`{"name":"Held","changes":[{"cannot":["move"]}],` +
			`"maintenance":[{"ends_on":"fight starts"},{"ends_on":"run"}],"again":"ignored"}`)}}
}

// givingParty launches two fighters beside a cook on a plain store and opens
// a write scope over the run.
func givingParty(t *testing.T) (*Manager, *strikeCharacters, *writeScope) {
	t.Helper()
	ctx := context.Background()
	chars := &strikeCharacters{byID: map[string]*character.Data{
		"alice": strikeFixtureFighter("alice"), "bob": strikeFixtureFighter("bob"),
	}}
	mgr, err := NewManager(&Config{Record: DiscardRecord{}, Acts: DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: &scriptedDice{rolls: []int{10, 10, 10, 10, 10, 10}}, TurnDriver: Pass{},
		Sessions:   &strikeSessions{byID: map[string]*SessionData{}},
		Encounters: &strikeEncounters{byID: map[string]*encounter.EncounterData{}},
		Characters: chars,
	})
	require.NoError(t, err)
	cook := monsterAt("cook", refs.Monsters.Goblin().String(), 4, 0)
	cook.Faction = "kitchen"
	launchScene(t, mgr, scene{
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("kitchen", 0, 0, 8, 4)},
			Factions:     []encounter.FactionInput{{ID: "kitchen"}},
			Dispositions: []encounter.DispositionInput{{Between: [2]encounter.FactionID{"kitchen", encounter.FactionParty}, Stance: encounter.StanceNeutral}},
		},
		Endings:  []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Party:    []sceneSeat{seatAt("alice", 2, 0), seatAt("bob", 3, 1)},
		Monsters: []dungeonspec.MonsterPlacement{cook},
	})
	scope, err := mgr.openForChange(ctx, "speak", "sess")
	require.NoError(t, err)
	return mgr, chars, scope
}

// Catches a partial give on a plain store: alice's give is asked and good,
// the next receiver's is refused, and alice's sheet was never saved.
func TestARefusedReceiverSavesNoSheetOfTheLine(t *testing.T) {
	mgr, chars, scope := givingParty(t)
	saves := chars.saves
	err := mgr.giveStatuses(context.Background(), scope, "cook",
		[]encounter.GrantedStatus{heldGrant("alice"), heldGrant("nobody")})
	require.ErrorContains(t, err, `"nobody" is not in the run`)
	require.Equal(t, saves, chars.saves, "no sheet of the line was saved")
}

// And the good case: each receiver is given it, and each sheet is saved once.
func TestEveryReceiverIsGivenItOnce(t *testing.T) {
	mgr, chars, scope := givingParty(t)
	saves := chars.saves
	require.NoError(t, mgr.giveStatuses(context.Background(), scope, "cook",
		[]encounter.GrantedStatus{heldGrant("alice"), heldGrant("bob")}))
	require.Equal(t, saves+2, chars.saves, "alice's and bob's sheets, each saved once")
	for _, who := range []string{"alice", "bob"} {
		require.Contains(t, string(mustRaw(t, chars.byID[who].Conditions)), "Held", "%s holds it", who)
	}
}

// mustRaw is a value's JSON.
func mustRaw(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}
