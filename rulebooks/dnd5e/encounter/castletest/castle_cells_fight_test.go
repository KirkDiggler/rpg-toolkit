// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_cells_fight_test.go is D0 ON THE ROOM THE CASTLE SHIPS
// (rpg-project#569): the mad king's cells, rpg-api's content, with one line
// added to a copy — the prisoners hostile to the garrison — and nothing else.
// The party walks into the cell block where a guard stands among the
// prisoners: the fight that forms holds the party, the guard and the
// prisoners watching him, and the guard and a prisoner swing at each other.
// Before D0 classify paired players with monsters only, so the prisoners
// stood outside the party's fight with their enemy in it.

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// cellsWarder is the guard the scene posts INSIDE the cell block, at
// [-7, 10], beside the prisoner at [-7, 11]. The file's guards stand in the
// corridor behind a locked door, where no prisoner can see them.
const cellsWarder = encounter.MemberID("44f4aa72-bd03-46b4-b8f4-d5dc2e895cff")

// cellsCorridorGuard is the other guard, left where the file posts him.
const cellsCorridorGuard = encounter.MemberID("7b8b5d1c-1b8f-4ea1-a681-5abab9420d0a")

var cellsClub = core.Ref{Module: "dnd5e", Type: "weapons", ID: "club"}

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

// TestAGuardAndAPrisonerFightEachOtherInThePartysFight: the party steps into
// the cell block two cells from the warder. The garrison is hostile to the
// party as shipped, so the warder and the party go to initiative — and the
// prisoners who have been watching the warder since first light come in
// with him, and the warder and a prisoner trade blows. No prisoner swings at
// the party, which they are neutral to.
func TestAGuardAndAPrisonerFightEachOtherInThePartysFight(t *testing.T) {
	t.Run("control: as shipped, the prisoners are no side of the fight", func(t *testing.T) {
		enc, _ := cellsFight(t, loadContent(t, cellsFixture))
		require.Equal(t, encounter.ClockTurn, clockKind(t, enc, cellsWarder), "the party and the warder fight")
		for _, id := range prisonersOf(t, loadContent(t, cellsFixture)) {
			require.Equal(t, encounter.ClockWorld, clockKind(t, enc, id), "%s is neutral to both", id)
		}
	})

	compiled := cellsAtWar(t)
	enc, strikes := cellsFight(t, compiled)
	prisoners := prisonersOf(t, compiled)

	t.Run("the party, the warder and the prisoners watching him are in one fight", func(t *testing.T) {
		require.Equal(t, encounter.ClockTurn, clockKind(t, enc, cellsPicker))
		require.Equal(t, encounter.ClockTurn, clockKind(t, enc, cellsWarder))
		in := 0
		for _, id := range prisoners {
			if clockKind(t, enc, id) == encounter.ClockTurn {
				in++
			}
		}
		require.Positive(t, in, "a prisoner who can see the warder is in the party's fight")
		require.Equal(t, encounter.ClockWorld, clockKind(t, enc, cellsCorridorGuard),
			"the guard behind the locked door saw nothing and is in no fight")
	})

	t.Run("the warder and a prisoner attack each other, and no prisoner attacks the party", func(t *testing.T) {
		isPrisoner := map[encounter.MemberID]bool{}
		for _, id := range prisoners {
			isPrisoner[id] = true
		}
		var warderHitAPrisoner, aPrisonerHitTheWarder bool
		for _, s := range *strikes {
			if s.attacker == cellsWarder && isPrisoner[s.target] {
				warderHitAPrisoner = true
			}
			if isPrisoner[s.attacker] {
				require.Equal(t, cellsWarder, s.target, "a prisoner's enemy is the garrison, never the party")
				aPrisonerHitTheWarder = true
			}
		}
		require.True(t, warderHitAPrisoner, "the warder swung at the prisoner beside him")
		require.True(t, aPrisonerHitTheWarder, "and a prisoner swung back")
	})
}

// cellsStrike is one swing a driven creature took.
type cellsStrike struct{ attacker, target encounter.MemberID }

// cellsStriker records every swing and lands it as a miss, so the scene is
// about who swings at whom and nobody falls.
type cellsStriker struct{ calls *[]cellsStrike }

func (s cellsStriker) Strike(
	_ context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	*s.calls = append(*s.calls, cellsStrike{attacker, target})
	_, err := enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: attacker, Targets: []encounter.MemberID{target},
		Attack: &encounter.AttackIdentity{Ref: action.String(), Name: "Club", DamageType: "bludgeoning"},
	}}}})
	return err
}

// rollsOne is the table die: the first eligible row always fires.
type rollsOne struct{}

func (rollsOne) Roll(context.Context, int) (int, error) { return 1, nil }

func (rollsOne) RollN(_ context.Context, count, _ int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i] = 1
	}
	return out, nil
}

// cellsFight opens the cells with the party's one player in the cell block
// at [-5, 10], two cells from the warder, every creature swinging at an
// enemy in reach and holding otherwise, and a club for everyone.
func cellsFight(t *testing.T, compiled dungeonspec.Compiled) (*encounter.Encounter, *[]cellsStrike) {
	t.Helper()
	swings := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}},
			Attack: &encounter.Selector{Word: encounter.SelectorEnemy}},
		{Weight: 1, Hold: true},
	}}
	club := encounter.SheetFacts{SpeedFeet: 30, Actions: []encounter.ActionView{{Ref: cellsClub, RangeFeet: 5}}}
	sheets := cellsSheets{cellsPicker: club}
	members := []encounter.MemberInput{{ID: cellsPicker, Kind: encounter.KindPlayer, Position: spatial.Position{X: -5, Y: 10}}}
	for _, m := range compiled.Monsters {
		id := encounter.MemberID(m.MemberID)
		at := m.At
		if id == cellsWarder {
			at = spatial.Position{X: -7, Y: 10}
		}
		sheets[id] = club
		members = append(members, encounter.MemberInput{
			ID: id, Kind: encounter.KindMonster, Position: at, Faction: encounter.FactionID(m.Faction), Table: swings,
		})
	}
	strikes := &[]cellsStrike{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: members,
		Endings: append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        encounter.TableDriver{Roller: rollsOne{}},
			Roller:        rollsOne{},
			CheckResolver: nothingIsEverFound{},
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   cellsStriker{calls: strikes},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	return enc, strikes
}

// cellsSheets answers the scene's sheets, strictly.
type cellsSheets map[encounter.MemberID]encounter.SheetFacts

func (s cellsSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		facts, ok := s[id]
		if !ok {
			return nil, encounter.ErrNoSheets
		}
		out[id] = facts
	}
	return out, nil
}

// prisonersOf is every prisoner the file places.
func prisonersOf(t *testing.T, compiled dungeonspec.Compiled) []encounter.MemberID {
	t.Helper()
	var out []encounter.MemberID
	for _, m := range compiled.Monsters {
		if m.Faction == "prisoners" {
			out = append(out, encounter.MemberID(m.MemberID))
		}
	}
	require.NotEmpty(t, out)
	return out
}

// clockKind is the clock one member is on.
func clockKind(t testing.TB, enc *encounter.Encounter, id encounter.MemberID) encounter.ClockKind {
	t.Helper()
	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
	require.NoError(t, err)

	return on.Kind
}
