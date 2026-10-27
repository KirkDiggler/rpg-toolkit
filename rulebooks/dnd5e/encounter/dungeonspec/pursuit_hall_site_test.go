// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// pursuit_hall_site_test.go plays the pursuit hall (rpg-project#671, smarter
// monsters S4b): a guard whose table is the `dasher` bundle, thirteen cells
// down a hall from the party start. A walk cannot reach the hero this turn and
// a run can, so the guard runs twelve cells, lands beside the hero, and does
// not swing. Written by hand in the studio export's shape (the file's header
// says so); its compiled picture is committed beside it.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const pursuitHallFixture = "testdata/pursuit-hall.yaml"

const (
	pursuitHero  = encounter.MemberID("hero")
	pursuitGuard = encounter.MemberID("guard")
)

// pursuitSight is the hall's light: everybody sees twenty cells.
type pursuitSight struct{}

func (pursuitSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 20
	}

	return out, nil
}

// pursuitSheets is thirty feet for everybody and a five-foot spear.
type pursuitSheets struct{}

func (pursuitSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{
			SpeedFeet: 30,
			Actions:   []encounter.ActionView{{Ref: kingSpear, RangeFeet: 5}},
		}
	}

	return out, nil
}

// TestThePursuitHallGuardRunsAndDoesNotSwing plays the hall's one turn.
func TestThePursuitHallGuardRunsAndDoesNotSwing(t *testing.T) {
	compiled := loadContent(t, pursuitHallFixture)
	members := []encounter.MemberInput{
		{ID: pursuitHero, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At},
	}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Facing: m.FacingDegrees,
		})
	}
	strikes := []cellsStrike{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:     compiled.Field,
		Members:   members,
		Endings:   append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         pursuitSight{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        pursuitSheets{},
			Participation: &castleStanding{down: map[encounter.MemberID]bool{}},
			Initiative:    orderAsGiven{},
			Driver:        encounter.TableDriver{Roller: rollsOne{}},
			Roller:        rollsOne{},
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   cellsStriker{calls: &strikes},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	// THE GUARD ACTS FIRST: the fight forms the moment they see each other,
	// its member sorts before the hero's, and its turn is driven inside the
	// setup that formed it.
	start := compiled.PartyStart[0].At
	require.Equal(t, 13.0, enc.Distance(start, compiled.Monsters[0].At), "precondition: thirteen cells down the hall")
	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: pursuitHero})
	require.NoError(t, err)
	require.Equal(t, encounter.ClockTurn, on.Kind, "precondition: seeing each other started the fight")
	require.Equal(t, pursuitHero, on.Active, "and the guard's turn is over")

	require.Equal(t, 1.0, enc.Distance(start, whereIsMember(t, enc, pursuitGuard)), "it ran to the hero's side")
	require.Empty(t, strikes, "it ran and did not swing")

	story, err := enc.Story(&encounter.StoryInput{Audience: pursuitHero})
	require.NoError(t, err)
	var moved int
	var pick map[string]any
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		switch {
		case beat["beat"] == encounter.BeatMoved && beat["member"] == string(pursuitGuard):
			moved++
			require.Equal(t, "encounter:table:dash", beat["cause"])
		case beat["beat"] == encounter.BeatAnswered && beat["creature"] == string(pursuitGuard) && pick == nil:
			pick = beat
		}
	}
	require.Equal(t, 12, moved, "twelve cells: twice what thirty feet walks")
	require.NotNil(t, pick)
	require.Equal(t, "dash", pick["word"], "the dasher's run row fired")
	require.Equal(t, []any{0.0, 2.0}, pick["path"], "inside the bundle, its third row")
}

// whereIsMember is one member's cell, off the public roster.
func whereIsMember(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) (cell spatial.Position) {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	require.Failf(t, "not placed", "member %q has no cell", id)

	return cell
}
