// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// pursuit_hall_site_test.go plays the pursuit hall (rpg-project#671, smarter
// monsters S4b): a guard whose table is the castle's `dasher` bundle,
// fourteen cells down a hall from the party start. Not even a run reaches the
// hero this turn, so the guard runs twelve cells, does not swing, and walks up
// and swings the turn after; a hero inside one run of reach is walked at. Written by hand in the studio export's shape (the file's header
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

// openThePursuitHall seats the hero at a cell and the guard where the file
// posts it. The guard sorts before the hero, so its first turn is driven
// inside the setup that formed the fight.
func openThePursuitHall(t *testing.T, heroAt spatial.Position) (*encounter.Encounter, *[]cellsStrike) {
	t.Helper()
	compiled := loadContent(t, pursuitHallFixture)
	members := []encounter.MemberInput{{ID: pursuitHero, Kind: encounter.KindPlayer, Position: heroAt}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Facing: m.FacingDegrees,
		})
	}
	strikes := &[]cellsStrike{}
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
				Striker:   cellsStriker{calls: strikes},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)
	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: pursuitHero})
	require.NoError(t, err)
	require.Equal(t, encounter.ClockTurn, on.Kind, "precondition: seeing each other started the fight")
	require.Equal(t, pursuitHero, on.Active, "and the guard's first turn is over")

	return enc, strikes
}

// guardBeats is the guard's moved beats' causes and its picks, in order.
func guardBeats(t *testing.T, enc *encounter.Encounter) (moved []any, picks []map[string]any) {
	t.Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: pursuitHero})
	require.NoError(t, err)
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		switch {
		case beat["beat"] == encounter.BeatMoved && beat["member"] == string(pursuitGuard):
			moved = append(moved, beat["cause"])
		case beat["beat"] == encounter.BeatAnswered && beat["creature"] == string(pursuitGuard):
			picks = append(picks, beat)
		}
	}

	return moved, picks
}

// TestThePursuitHallGuardRunsThenSwings: fourteen cells down the hall, not
// even a run reaches the hero this turn, so the guard runs — twelve cells,
// no swing — and on its next turn walks up and swings.
func TestThePursuitHallGuardRunsThenSwings(t *testing.T) {
	compiled := loadContent(t, pursuitHallFixture)
	start := compiled.PartyStart[0].At
	enc, strikes := openThePursuitHall(t, start)
	require.Equal(t, 14.0, enc.Distance(start, compiled.Monsters[0].At), "precondition: fourteen cells down the hall")

	require.Equal(t, 2.0, enc.Distance(start, whereIsMember(t, enc, pursuitGuard)), "it ran twelve")
	require.Empty(t, *strikes, "and did not swing")
	moved, picks := guardBeats(t, enc)
	require.Len(t, moved, 12, "twelve cells: its walk and its speed once more")
	for _, cause := range moved {
		require.Equal(t, "encounter:table:dash", cause)
	}
	require.NotEmpty(t, picks)
	require.Equal(t, "dash", picks[0]["word"], "the dasher's `far` row fired")
	require.Equal(t, []any{0.0, 3.0}, picks[0]["path"], "inside the bundle, its fourth row")

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: pursuitHero})
	require.NoError(t, err)
	require.Equal(t, 1.0, enc.Distance(start, whereIsMember(t, enc, pursuitGuard)), "it walked up")
	require.Len(t, *strikes, 1, "and swung on the turn after the run")
	moved, _ = guardBeats(t, enc)
	require.Len(t, moved, 13)
	require.Nil(t, moved[12], "the walk up is a walk, not a run")
}

// TestThePursuitHallGuardWalksAtAHeroInsideOneRun: nine cells from the
// guard, a run would land beside the hero with no swing and swing the round
// after anyway, so the castle's dasher walks at him instead.
func TestThePursuitHallGuardWalksAtAHeroInsideOneRun(t *testing.T) {
	compiled := loadContent(t, pursuitHallFixture)
	guardAt := compiled.Monsters[0].At
	near := spatial.Position{X: guardAt.X - 9, Y: guardAt.Y}
	enc, strikes := openThePursuitHall(t, near)
	require.Equal(t, 9.0, enc.Distance(near, guardAt), "precondition: nine cells from the guard")

	require.Equal(t, 3.0, enc.Distance(near, whereIsMember(t, enc, pursuitGuard)), "it walked six")
	require.Empty(t, *strikes)
	moved, picks := guardBeats(t, enc)
	require.Len(t, moved, 6)
	for _, cause := range moved {
		require.Nil(t, cause, "walked at, not run at")
	}
	require.Equal(t, "toward", picks[0]["word"])
	require.Equal(t, []any{0.0, 2.0}, picks[0]["path"], "the dasher's `dash` row, which walks")
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
