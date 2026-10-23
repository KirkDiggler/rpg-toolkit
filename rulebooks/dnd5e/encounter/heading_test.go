// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// heading_test.go is the way a subject WENT (rpg-project#679 R10): a sighting
// carries the bearing of the subject's last step beside its facing, standing
// still keeps it, it survives the blob, and a player's testimony is never
// revised by looking (#667 R8).

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// seenBy is one member's own sight testimony of another, through View.
func seenBy(t *testing.T, enc *encounter.Encounter, observer, subject encounter.MemberID) encounter.SightTestimony {
	t.Helper()
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	require.NoError(t, err)
	for _, h := range holdings {
		if h.Channel == perception.Sight && encounter.MemberID(h.Subject) == subject {
			seen, ok := encounter.DecodeSightTestimony(h.Payload)
			require.True(t, ok)

			return seen
		}
	}
	require.FailNowf(t, "no sighting", "%s holds no sighting of %s", observer, subject)

	return encounter.SightTestimony{}
}

// headingCapabilities is facingFight's host, for a load.
func headingCapabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{},
		Conditions: encounter.UnobservedEquipment{}, Sheets: adjacentSkeletonSheets,
		Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{},
		Actors: encounter.Actors{
			Striker: &scriptedStriker{kind: encounter.OutcomeMissed}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		},
	}
}

// headingScene is alice and a goblin who sees her, neither having stepped.
// The goblin's one turn is driver's; nil passes.
func headingScene(t *testing.T, driver encounter.Driver) *encounter.Encounter {
	t.Helper()
	if driver == nil {
		driver = passDriver{}
	}

	return facingFight(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 4}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 4}},
	}, driver, &scriptedStriker{kind: encounter.OutcomeMissed}, quietMover{})
}

// goblinStart is where headingScene places the goblin, dungeon-absolute, and
// goblinStep the cell north of it its one driven step takes.
var (
	goblinStart = encounter.HexCellAt(encounter.HexesArePointyTop(), 7, 4)
	goblinStep  = spatial.Position{X: goblinStart.X, Y: goblinStart.Y - 1}
)

// stepsOnce is a goblin whose one turn is a single step north.
func stepsOnce() *oneMemberDriver {
	return &oneMemberDriver{member: goblin, intent: encounter.Move{Path: []spatial.Position{goblinStep}}}
}

// TestASightingCarriesTheWaySheWent: never seen stepping is no heading; a
// step seen is its bearing, beside the facing it also turned her to; and
// standing still keeps it.
func TestASightingCarriesTheWaySheWent(t *testing.T) {
	enc := headingScene(t, stepsOnce())
	require.Nil(t, seenBy(t, enc, goblin, alice).Heading, "never seen moving: no heading")

	from := memberOf(t, enc, alice).Position
	to := spatial.Position{X: from.X, Y: from.Y + 1}
	_, err := enc.Step(&encounter.StepInput{Member: alice, To: to})
	require.NoError(t, err)
	seen := seenBy(t, enc, goblin, alice)
	require.NotNil(t, seen.Heading)
	require.InDelta(t, stepBearing(t, from, to), *seen.Heading, 1e-9, "the bearing of the step she was seen take")

	// Standing still: the goblin's own step refreshes everyone's sight, and
	// alice, who did not move, keeps the way she went.
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.Equal(t, goblinStep, memberOf(t, enc, goblin).Position, "precondition: the goblin stepped")
	still := seenBy(t, enc, goblin, alice)
	require.NotNil(t, still.Heading)
	require.InDelta(t, stepBearing(t, from, to), *still.Heading, 1e-9, "standing still keeps the heading")
}

// TestTheWaySheWentSurvivesTheBlob is save→load→use: alice steps, the
// encounter goes through JSON and back, and both halves of the heading come
// with it — the goblin's held testimony, and alice's own last step, which a
// member who joins after the load then sees.
func TestTheWaySheWentSurvivesTheBlob(t *testing.T) {
	enc := headingScene(t, nil)
	from := memberOf(t, enc, alice).Position
	to := spatial.Position{X: from.X + 1, Y: from.Y}
	_, err := enc.Step(&encounter.StepInput{Member: alice, To: to})
	require.NoError(t, err)
	want := stepBearing(t, from, to)

	blob, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(blob, &data))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: headingCapabilities()})
	require.NoError(t, err)

	held := seenBy(t, loaded, goblin, alice)
	require.NotNil(t, held.Heading, "the goblin's testimony kept the heading")
	require.InDelta(t, want, *held.Heading, 1e-9)

	_, err = loaded.Join(&encounter.JoinInput{Member: bob, Kind: encounter.KindMonster, Cell: cellAt(7, 7)})
	require.NoError(t, err)
	fresh := seenBy(t, loaded, bob, alice)
	require.NotNil(t, fresh.Heading, "alice's own last step came back with her")
	require.InDelta(t, want, *fresh.Heading, 1e-9, "and a sighting after the load carries it")
}

// TestAStoredWentThatIsNotABearingIsRefused pins the load door for the
// member's last step, on facing's rule.
func TestAStoredWentThatIsNotABearingIsRefused(t *testing.T) {
	data := headingScene(t, nil).ToData()
	bad := 360.0
	data.Members[0].Went = &bad
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: headingCapabilities()})
	require.ErrorIs(t, err, encounter.ErrInvalidData)
	require.ErrorIs(t, err, encounter.ErrBadFacing)
}

// TestAPlayersTestimonyIsNeverRevisedByLooking is #667 R8 for the heading: a
// player who merely views holds the goblin's heading exactly as the sighting
// wrote it — none gained for a goblin that never stepped, and the one it
// was seen with unchanged after it did.
func TestAPlayersTestimonyIsNeverRevisedByLooking(t *testing.T) {
	enc := headingScene(t, stepsOnce())
	for range 3 {
		require.Nil(t, seenBy(t, enc, alice, goblin).Heading, "looking gives no heading")
	}

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.Equal(t, goblinStep, memberOf(t, enc, goblin).Position, "precondition: the goblin stepped")
	first := seenBy(t, enc, alice, goblin)
	require.NotNil(t, first.Heading)
	for range 3 {
		again := seenBy(t, enc, alice, goblin)
		require.Equal(t, first, again, "looking again revises nothing")
	}
	require.InDelta(t, stepBearing(t, goblinStart, goblinStep), *first.Heading, 1e-9)
}

// TestABlobFromBeforeTheHeadingIsRefusedAtLoad names what LoadEncounter does
// with a `search` row persisted before the word existed (rpg-toolkit
// encounter v0.141.x): it REFUSES it, ErrInvalidData wrapping
// behavior.ErrBadSearch, rather than loading a wordless search or guessing
// `went` at the door — a second spelling the dialect already owns. Pre-v1, a
// host starts a fresh encounter. The same blob with its heading loads.
func TestABlobFromBeforeTheHeadingIsRefusedAtLoad(t *testing.T) {
	table := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyLost}},
			Search: &encounter.Search{Distance: 6, Heading: encounter.HeadingWent}},
	}}
	enc := facingFight(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 4}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 4}, Table: table},
	}, passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietMover{})
	blob, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	require.Contains(t, string(blob), `"search":{"distance":6,"heading":"went"}`, "precondition: today's blob names the word")

	load := func(raw string) error {
		var data encounter.EncounterData
		require.NoError(t, json.Unmarshal([]byte(raw), &data))
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: headingCapabilities()})

		return err
	}
	require.NoError(t, load(string(blob)), "the blob with its heading loads")

	old := strings.Replace(string(blob), `"search":{"distance":6,"heading":"went"}`, `"search":{"distance":6}`, 1)
	err = load(old)
	require.ErrorIs(t, err, encounter.ErrInvalidData)
	require.ErrorIs(t, err, behavior.ErrBadSearch)
	require.Contains(t, err.Error(), "`search` heading")
}
