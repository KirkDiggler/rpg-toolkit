// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// bonusdisengage_test.go pins rpg-toolkit#2032 step 2: a monster whose sheet
// says it may Disengage as a bonus action spends that bonus action BEFORE a
// walk whose first step leaves a standing hostile's melee reach — asks its
// [encounter.Disengager], tells the story once, and then walks unchanged.
//
// Every case puts alice (player, hostile) at (1,1) and the goblin beside her
// at (2,1) unless it says otherwise; the goblin is second in initiative, so
// alice's EndTurn drives its turn.

// nimbleEscape is the trait a session answers the goblin's sheet with. The
// encounter carries both strings verbatim and reads neither.
var nimbleEscape = encounter.ActivationIdentity{Ref: "dnd5e:monster_traits:nimble_escape", Name: "Nimble Escape"}

// recordingDisengager records who it was asked to disengage, and fails when
// told to.
type recordingDisengager struct {
	fail  error
	calls []encounter.MemberID
}

func (d *recordingDisengager) Disengage(_ context.Context, _ *encounter.Encounter, member encounter.MemberID) error {
	d.calls = append(d.calls, member)
	return d.fail
}

type disengageScene struct {
	members    []encounter.MemberInput
	sheets     sheetFacts
	intents    []encounter.TurnIntent
	disengager encounter.Disengager

	// participation defaults to everyone standing.
	participation encounter.Participation
}

// goblinBesideAlice is the default scene: the goblin nimble, adjacent to a
// standing hostile, with 30 feet to spend.
func goblinBesideAlice(intents ...encounter.TurnIntent) disengageScene {
	return disengageScene{
		members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(1, 1)},
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(2, 1)},
		},
		sheets: sheetFacts{
			goblin: {SpeedFeet: 30, BonusDisengage: nimbleEscape},
			alice:  {},
		},
		intents: intents,
	}
}

func (sc disengageScene) build(t *testing.T) (*encounter.Encounter, *scriptedDriver) {
	t.Helper()
	driver := &scriptedDriver{intents: sc.intents}
	var participation encounter.Participation = everyoneStanding{}
	if sc.participation != nil {
		participation = sc.participation
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: sc.members,
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sc.sheets,
			Participation: participation,
			Initiative:    arrivalOrder{},
			Driver:        driver,
			Actors: encounter.Actors{
				Striker:    &scriptedStriker{kind: encounter.OutcomeMissed},
				Mover:      quietMover{},
				Announcer:  quietAnnouncer{},
				Disengager: sc.disengager,
			},
		},
	})
	require.NoError(t, err)
	return enc, driver
}

// activatedBeats is every "activated" beat alice heard, in story order, with
// the index each sits at among all her beats.
func activatedBeats(t *testing.T, enc *encounter.Encounter) (found []map[string]any, at []int, all []map[string]any) {
	t.Helper()
	all = storyBeats(t, enc, alice)
	for i, b := range all {
		if b["beat"] == encounter.BeatActivated {
			found = append(found, b)
			at = append(at, i)
		}
	}
	return found, at, all
}

func firstMovedBeatOf(all []map[string]any, member encounter.MemberID) int {
	for i, b := range all {
		if b["beat"] == encounter.BeatMoved && b["member"] == string(member) {
			return i
		}
	}
	return -1
}

func TestNimbleGoblinDisengagesBeforeWalkingOutOfReach(t *testing.T) {
	d := &recordingDisengager{}
	sc := goblinBesideAlice(encounter.Move{Path: []spatial.Position{cellAt(3, 1)}}, encounter.Pass{})
	sc.disengager = d
	enc, driver := sc.build(t)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	require.Equal(t, []encounter.MemberID{goblin}, d.calls, "the goblin's sheet was asked to disengage exactly once")

	beats, at, all := activatedBeats(t, enc)
	require.Len(t, beats, 1, "exactly one disengage beat")
	require.Equal(t, string(goblin), beats[0]["actor"])
	require.Equal(t, map[string]any{"ref": nimbleEscape.Ref, "name": nimbleEscape.Name}, beats[0]["ability"])
	moved := firstMovedBeatOf(all, goblin)
	require.NotEqual(t, -1, moved, "the walk happened")
	require.Less(t, at[0], moved, "the disengage is told before the walk it pays for")

	require.Equal(t, cellAt(3, 1), positionsOf(t, enc)[goblin])

	require.Len(t, driver.calls, 2)
	require.Equal(t, 1, driver.calls[0].Budget.BonusActions, "a turn starts with one bonus action")
	require.Equal(t, 0, driver.calls[1].Budget.BonusActions, "the disengage spent it")
	require.Equal(t, 25, driver.calls[1].Budget.MovementFeet,
		"the bonus action costs no movement: 30 feet less the one cell walked")
}

func TestRoutedNimbleGoblinDisengagesBeforeFleeing(t *testing.T) {
	d := &recordingDisengager{}
	sc := goblinBesideAlice(encounter.Routed{
		Policy: encounter.MoveAway, Anchor: alice, Cause: core.Ref{Module: "dnd5e", Type: "conditions", ID: "frightened"},
	})
	sc.disengager = d
	enc, _ := sc.build(t)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	require.Equal(t, []encounter.MemberID{goblin}, d.calls)
	beats, at, all := activatedBeats(t, enc)
	require.Len(t, beats, 1)
	require.Less(t, at[0], firstMovedBeatOf(all, goblin))
	require.NotEqual(t, cellAt(2, 1), positionsOf(t, enc)[goblin], "the flight happened")
}

func TestSpentBonusActionDisengagesNobody(t *testing.T) {
	// alice (1,1), goblin (2,1), bob (5,1). The goblin leaves alice's reach
	// (spends the bonus action), closes on bob, then leaves bob's reach: the
	// second leaving is not paid for, because nothing is left to pay with.
	d := &recordingDisengager{}
	sc := disengageScene{
		members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(1, 1)},
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(2, 1)},
			{ID: bob, Kind: encounter.KindPlayer, Position: cellAt(5, 1)},
		},
		sheets: sheetFacts{goblin: {SpeedFeet: 30, BonusDisengage: nimbleEscape}, alice: {}, bob: {}},
		intents: []encounter.TurnIntent{
			encounter.Move{Path: []spatial.Position{cellAt(3, 1)}},
			encounter.Move{Path: []spatial.Position{cellAt(4, 1)}},
			encounter.Move{Path: []spatial.Position{cellAt(3, 1)}},
			encounter.Pass{},
		},
		disengager: d,
	}
	enc, driver := sc.build(t)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	require.Equal(t, []encounter.MemberID{goblin}, d.calls, "only the first leaving was paid for")
	beats, _, _ := activatedBeats(t, enc)
	require.Len(t, beats, 1)
	require.Equal(t, cellAt(3, 1), positionsOf(t, enc)[goblin], "every walk still happened")
	require.Equal(t, 15, driver.calls[3].Budget.MovementFeet)
}

func TestGoblinWithoutTheTraitDisengagesNobody(t *testing.T) {
	d := &recordingDisengager{}
	sc := goblinBesideAlice(encounter.Move{Path: []spatial.Position{cellAt(3, 1)}}, encounter.Pass{})
	sc.sheets[goblin] = encounter.SheetFacts{SpeedFeet: 30}
	sc.disengager = d
	enc, driver := sc.build(t)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	require.Empty(t, d.calls)
	beats, _, _ := activatedBeats(t, enc)
	require.Empty(t, beats)
	require.Equal(t, cellAt(3, 1), positionsOf(t, enc)[goblin])
	require.Equal(t, 1, driver.calls[1].Budget.BonusActions, "an unspent bonus action stays unspent")
}

func TestWalkThatDoesNotLeaveReachDisengagesNobody(t *testing.T) {
	t.Run("no hostile in reach", func(t *testing.T) {
		d := &recordingDisengager{}
		sc := goblinBesideAlice(encounter.Move{Path: []spatial.Position{cellAt(5, 5)}}, encounter.Pass{})
		sc.members[1].Position = cellAt(5, 4)
		sc.disengager = d
		enc, _ := sc.build(t)

		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
		require.NoError(t, err)

		require.Empty(t, d.calls)
		require.Equal(t, cellAt(5, 5), positionsOf(t, enc)[goblin])
	})

	t.Run("first step stays in reach", func(t *testing.T) {
		// Found rather than hand-picked: a neighbour of the goblin's cell
		// that is also a neighbour of alice's, by the encounter's own
		// distance — the step slides round her without leaving her reach.
		probe, _ := goblinBesideAlice().build(t)
		aliceAt, goblinAt := cellAt(1, 1), cellAt(2, 1)
		var slide spatial.Position
		found := false
		for x := 0; x <= 4 && !found; x++ {
			for y := 0; y <= 4 && !found; y++ {
				c := cellAt(x, y)
				if c == aliceAt || c == goblinAt {
					continue
				}
				if probe.Distance(c, aliceAt) == 1 && probe.Distance(c, goblinAt) == 1 {
					slide, found = c, true
				}
			}
		}
		require.True(t, found, "a hex grid always has a shared neighbour")

		d := &recordingDisengager{}
		sc := goblinBesideAlice(encounter.Move{Path: []spatial.Position{slide}}, encounter.Pass{})
		sc.disengager = d
		enc, _ := sc.build(t)

		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
		require.NoError(t, err)

		require.Empty(t, d.calls)
		require.Equal(t, slide, positionsOf(t, enc)[goblin])
	})
}

func TestDownHostileInReachIsNotDisengagedFrom(t *testing.T) {
	// bob lies down beside the goblin and alice stands far off: the only
	// body in reach threatens nothing, so the walk away is not paid for.
	d := &recordingDisengager{}
	sc := disengageScene{
		members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(8, 8)},
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(2, 1)},
			{ID: bob, Kind: encounter.KindPlayer, Position: cellAt(1, 1)},
		},
		sheets:        sheetFacts{goblin: {SpeedFeet: 30, BonusDisengage: nimbleEscape}, alice: {}, bob: {}},
		intents:       []encounter.TurnIntent{encounter.Move{Path: []spatial.Position{cellAt(3, 1)}}, encounter.Pass{}},
		disengager:    d,
		participation: &downList{down: []encounter.MemberID{bob}},
	}
	enc, _ := sc.build(t)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	require.Empty(t, d.calls)
	require.Equal(t, cellAt(3, 1), positionsOf(t, enc)[goblin])
}

func TestNoDisengagerSkipsTheAskAndWalks(t *testing.T) {
	sc := goblinBesideAlice(encounter.Move{Path: []spatial.Position{cellAt(3, 1)}}, encounter.Pass{})
	enc, driver := sc.build(t)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	beats, _, _ := activatedBeats(t, enc)
	require.Empty(t, beats, "nothing disengaged, so nothing is told")
	require.Equal(t, cellAt(3, 1), positionsOf(t, enc)[goblin], "the walk happened, and provokes as any walk does")
	require.Equal(t, 1, driver.calls[1].Budget.BonusActions, "nothing was spent on an ask nobody heard")
}

func TestDisengagerFailureAbortsTheTurn(t *testing.T) {
	boom := errors.New("sheet is corrupt")
	d := &recordingDisengager{fail: boom}
	sc := goblinBesideAlice(encounter.Move{Path: []spatial.Position{cellAt(3, 1)}}, encounter.Pass{})
	sc.disengager = d
	enc, _ := sc.build(t)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.ErrorIs(t, err, boom)
	require.Equal(t, []encounter.MemberID{goblin}, d.calls)
	require.Equal(t, cellAt(2, 1), positionsOf(t, enc)[goblin], "no walk after a failed disengage")
	beats, _, _ := activatedBeats(t, enc)
	require.Empty(t, beats, "no beat claims a disengage that failed")
}

func TestWorldWalkAsksNoDisengage(t *testing.T) {
	// On the world clock nobody is in a fight, so no hostile can be in
	// reach — and the round's budget carries no bonus action at all. A
	// nimble goblin walking past a vendor is asked nothing.
	d := &recordingDisengager{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: worldField(),
		Members: []encounter.MemberInput{
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(1, 0), Table: walksTo(cellAt(3, 0))},
			{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(0, 0)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheetFacts{goblin: {SpeedFeet: 5, BonusDisengage: nimbleEscape}, "vendor": {}},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        tableDriver(),
			Actors: encounter.Actors{
				Striker:    passStriker{},
				Mover:      quietMover{},
				Announcer:  quietAnnouncer{},
				Disengager: d,
			},
		},
	})
	require.NoError(t, err)

	_, err = aRound(enc)
	require.NoError(t, err)

	require.Empty(t, d.calls)
	require.Equal(t, cellAt(2, 0), positionsOf(t, enc)[goblin], "the goblin walked its round")
}

func TestSheetRefusesAHalfNamedBonusDisengage(t *testing.T) {
	// The sheet is asked when it is used — here, the goblin's turn budget —
	// and a half-named trait is refused there rather than told half-told.
	for name, id := range map[string]encounter.ActivationIdentity{
		"ref without name": {Ref: nimbleEscape.Ref},
		"name without ref": {Name: nimbleEscape.Name},
	} {
		t.Run(name, func(t *testing.T) {
			sc := goblinBesideAlice(encounter.Move{Path: []spatial.Position{cellAt(3, 1)}})
			sc.sheets[goblin] = encounter.SheetFacts{SpeedFeet: 30, BonusDisengage: id}
			sc.disengager = &recordingDisengager{}
			enc, _ := sc.build(t)

			_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
			require.ErrorIs(t, err, encounter.ErrInvalidData)
		})
	}
}
