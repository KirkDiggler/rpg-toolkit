// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// group_test.go is the nested table through the blob (rpg-project#670): a
// group row and a pattern saved, loaded and used. The S1 lesson is the
// reason it exists: a field the blob drops comes back as a wordless row, and
// a dropped `rows` would be exactly that.

// groupedTable is a pursuer group (from, params, as and two rows) beside a
// row that listens for a pattern of sounds, and a hold.
func groupedTable() encounter.Table {
	return encounter.Table{encounter.AnswerTime: {
		{Weight: 2, As: "toward", From: "pursuer", Params: map[string]string{"distance": "8", "heading": "went"},
			Rows: []encounter.Answer{
				{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyRemembered}},
					Toward: &encounter.Selector{Word: encounter.SelectorEnemy}},
				{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyLost}},
					Search: &encounter.Search{Distance: 8, Heading: encounter.HeadingWent}},
			}},
		{Weight: 1, When: []encounter.When{{Heard: &encounter.HeardWhen{
			Kind: behavior.Name{Pattern: "clang|voice"}, Within: 2}}},
			Toward: &encounter.Selector{Word: encounter.SelectorSound}},
		{Weight: 1, When: []encounter.When{{Not: &encounter.When{Fact: behavior.Name{Pattern: "alarm-.*"}}}},
			Hold: true},
	}}
}

// TestAGroupAndAPatternSurviveTheBlob: ToData writes the group's rows, `as`,
// `from` and `params` and each pattern as it was written; LoadEncounter
// accepts them at its door; and the reloaded encounter writes the same table
// again, row for row, so a saved run plays the table it was saved with.
func TestAGroupAndAPatternSurviveTheBlob(t *testing.T) {
	enc := facingFight(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 4}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 4}, Table: groupedTable()},
	}, passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietMover{})

	blob, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	for _, written := range []string{
		`"rows":[{"weight":1,"when":[{"enemy":"remembered"}]`,
		`"as":"toward"`, `"from":"pursuer"`, `"params":{"distance":"8","heading":"went"}`,
		`"kind":"/clang|voice/"`, `"fact":"/alarm-.*/"`,
	} {
		require.Contains(t, string(blob), written)
	}

	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(blob, &data))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: headingCapabilities()})
	require.NoError(t, err)

	again, err := json.Marshal(loaded.ToData())
	require.NoError(t, err)
	require.Equal(t, goblinTableOf(t, blob), goblinTableOf(t, again), "the same table, saved and loaded")
}

// TestTheDoorJudgesEveryRowInsideAGroup: a word illegal under the key is as
// illegal one group down, and the composition's door refuses it naming the
// group and the row.
func TestTheDoorJudgesEveryRowInsideAGroup(t *testing.T) {
	bad := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, From: "coward", Rows: []encounter.Answer{{Weight: 1, Flee: true}}},
	}}
	err := joinGoblin(bad)
	require.Error(t, err)
	require.Contains(t, err.Error(), `answer "time" entry 0, inside its group: answer "time" entry 0: `+
		"`flee` answers a social verdict, not time")

	unheard := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Heard: &encounter.HeardWhen{
			Kind: behavior.Name{Pattern: "bell|gong"}, Within: 2}}}, Hold: true},
	}}
	err = joinGoblin(unheard)
	require.Error(t, err)
	require.Contains(t, err.Error(), "`heard: { kind: /bell|gong/ }` matches no sound this build makes")
}

// joinGoblin is facingFight's setup with the goblin holding this table, and
// the refusal NewEncounter answers it with.
func joinGoblin(table encounter.Table) error {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 4}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 4}, Table: table},
		},
		Endings:   []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        adjacentSkeletonSheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: encounter.Actors{
				Striker: &scriptedStriker{kind: encounter.OutcomeMissed}, Mover: quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})

	return err
}

// goblinTableOf is the goblin's persisted table, as raw JSON, out of a blob.
func goblinTableOf(t *testing.T, blob []byte) string {
	t.Helper()
	var data struct {
		Members []struct {
			ID    string          `json:"id"`
			Table json.RawMessage `json:"table"`
		} `json:"members"`
	}
	require.NoError(t, json.Unmarshal(blob, &data))
	for _, m := range data.Members {
		if m.ID == string(goblin) {
			return string(m.Table)
		}
	}
	require.Fail(t, "the goblin is in the blob")

	return ""
}
