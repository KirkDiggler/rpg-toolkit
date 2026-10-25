// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// tablebeat_test.go is THE RECORD CARRIES ITS OWN SYMBOLS (rpg-project#674):
// a creature's table is described once in the record before the first
// `answered` that indexes it, every `answered` names a verdict for every row,
// and the description survives the blob — a reload describes nothing the
// record holds, and a changed table is described exactly once more.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// symbolsCaps is the silence scene's capabilities, built once so the scene
// can be loaded back from its blob with the same host.
func symbolsCaps(members []encounter.MemberInput) encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        silenceSheets(members),
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        tableDriver(),
		Roller:        rollsLowest{},
		Actors: encounter.Actors{
			Striker:   &scriptedStriker{kind: encounter.OutcomeMissed},
			Mover:     &recordingMover{},
			Announcer: quietAnnouncer{},
		},
	}
}

// symbolsScene is alice beside a coward goblin on silenceTable, with alice's
// swing already landed against it so the fight is on and its `attacked` row
// holds.
func symbolsScene(t *testing.T) (*encounter.Encounter, []encounter.MemberInput) {
	t.Helper()
	members := []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: seat(4, 4)},
		{ID: goblin, Kind: encounter.KindMonster, Position: seat(5, 4), Table: silenceTable(),
			Temper: encounter.Temper{Word: "coward", Profile: encounter.TemperProfile{
				Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100}}},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 14, 8)},
		},
		Members:      members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: symbolsCaps(members),
	})
	require.NoError(t, err)
	_, err = recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
	})
	require.NoError(t, err)

	return enc, members
}

// goblinTurn ends alice's turn so the goblin takes one.
func goblinTurn(t *testing.T, enc *encounter.Encounter) {
	t.Helper()
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
}

// goblinBeatsOf is every goblin beat of this kind, in story order.
func goblinBeatsOf(t *testing.T, enc *encounter.Encounter, kind string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, b := range goblinBeats(t, enc) {
		if b["beat"] == kind {
			out = append(out, b)
		}
	}

	return out
}

// reloadSymbols plays on from the encounter's own blob, as a host does.
func reloadSymbols(
	t *testing.T, data encounter.EncounterData, members []encounter.MemberInput,
) *encounter.Encounter {
	t.Helper()
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: symbolsCaps(members)})
	require.NoError(t, err)

	return loaded
}

// TestTheTableIsDescribedBeforeItsFirstAnswer: the goblin's first pick is
// preceded by exactly one `table` beat carrying every row as authored, in
// the studio's compact words, and the temperament that loads them.
func TestTheTableIsDescribedBeforeItsFirstAnswer(t *testing.T) {
	enc, _ := symbolsScene(t)
	goblinTurn(t, enc)

	beats := goblinBeats(t, enc)
	require.Equal(t, encounter.BeatTable, beats[0]["beat"], "the table comes first")
	require.Equal(t, encounter.BeatAnswered, beats[1]["beat"], "immediately before the first answer")

	table := beats[0]
	require.Equal(t, string(goblin), table["creature"])
	require.Equal(t, string(encounter.AnswerTime), table["key"])
	require.Equal(t, "", table["option"], "the creature's own table, not an option's")
	require.Equal(t, "coward", table["temper"])
	require.Equal(t, map[string]any{
		"attack": 50.0, "toward": 50.0, "away": 300.0, "flee": 300.0, "hold": 100.0,
	}, table["percents"])

	rows := table["rows"].([]any)
	require.Len(t, rows, len(silenceTable()[encounter.AnswerTime]), "every row of the key, as authored")
	require.Equal(t, map[string]any{
		"entry": 1.0, "when": []any{"attacked within 3"}, "word": "toward", "selector": "attacker",
		"weight": 2.0, "say": "", "fact": "", "search": nil,
	}, rows[1])
	var whens []any
	for _, r := range rows {
		whens = append(whens, r.(map[string]any)["when"].([]any)...)
	}
	require.Equal(t, []any{"enemy reach", "attacked within 3", "enemy seen", "enemy none"}, whens)
}

// TestAnAnswerNamesAVerdictForEveryRow: the `answered` beat carries `rows`,
// one per row of the key — the two candidates with their loads, and the two
// whose conditions failed.
func TestAnAnswerNamesAVerdictForEveryRow(t *testing.T) {
	enc, _ := symbolsScene(t)
	goblinTurn(t, enc)

	answered := goblinBeatsOf(t, enc, encounter.BeatAnswered)
	require.NotEmpty(t, answered)
	require.Equal(t, []any{
		map[string]any{"entry": 0.0, "held": true, "affordable": true, "weight": 1.0, "percent": 50.0, "loaded": 50.0},
		map[string]any{"entry": 1.0, "held": true, "affordable": true, "weight": 2.0, "percent": 50.0, "loaded": 100.0},
		map[string]any{"entry": 2.0, "held": false, "affordable": true, "weight": 0.0, "percent": 0.0, "loaded": 0.0},
		map[string]any{"entry": 3.0, "held": false, "affordable": true, "weight": 0.0, "percent": 0.0, "loaded": 0.0},
	}, answered[0]["rows"], "in reach and attacked: the swing and the chase are in, seen and none are out")
}

// TestAReloadDescribesNoTableTheRecordHolds is the save→load→use proof: the
// hash the description wrote survives the blob, so the reloaded goblin's
// next pick writes no second `table`; a table changed in the blob is
// described exactly once more.
func TestAReloadDescribesNoTableTheRecordHolds(t *testing.T) {
	enc, members := symbolsScene(t)
	goblinTurn(t, enc)
	require.Len(t, goblinBeatsOf(t, enc, encounter.BeatTable), 1, "precondition: described once")
	picked := len(goblinBeatsOf(t, enc, encounter.BeatAnswered))

	data := enc.ToData()
	var saved *encounter.MemberData
	for i := range data.Members {
		if data.Members[i].ID == goblin {
			saved = &data.Members[i]
		}
	}
	require.NotNil(t, saved)
	require.Contains(t, saved.TableHash, string(encounter.AnswerTime), "the description is remembered in the blob")

	t.Run("the same table is not described again", func(t *testing.T) {
		loaded := reloadSymbols(t, enc.ToData(), members)
		goblinTurn(t, loaded)
		require.Greater(t, len(goblinBeatsOf(t, loaded, encounter.BeatAnswered)), picked, "precondition: a pick after the reload")
		require.Len(t, goblinBeatsOf(t, loaded, encounter.BeatTable), 1, "and no second description")
	})

	t.Run("a changed table is described exactly once more", func(t *testing.T) {
		changed := enc.ToData()
		for i := range changed.Members {
			if changed.Members[i].ID != goblin {
				continue
			}
			rows := changed.Members[i].Table[string(encounter.AnswerTime)]
			rows[1].Weight = 5
		}
		loaded := reloadSymbols(t, changed, members)
		goblinTurn(t, loaded)
		goblinTurn(t, loaded)
		require.GreaterOrEqual(t, len(goblinBeatsOf(t, loaded, encounter.BeatAnswered)), picked+2,
			"precondition: more than one pick after the edit")
		tables := goblinBeatsOf(t, loaded, encounter.BeatTable)
		require.Len(t, tables, 2, "one for the old table, one for the edited, none for the repeat")
		edited := tables[1]["rows"].([]any)[1].(map[string]any)
		require.Equal(t, 5.0, edited["weight"], "and it describes the table as edited")
	})
}
