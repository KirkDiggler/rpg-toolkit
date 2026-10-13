// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// turnsilence_test.go pins the two monster-turn dead spots the tier 2 E walk
// found (rpg-toolkit#2000, #2001): a driven turn whose `attack` or `toward`
// did nothing must say why on the story, as a `stayed` beat, instead of
// going straight from `answered` to `turn-ended`.

// silenceTable is the walk's goblin table with the swing first, so the
// fixture die (rollsLowest) takes the swing whenever it is a candidate — the
// walk's own die chose it at beat 49 of run 21865f05. The rows are the
// author's; only their order differs.
func silenceTable() encounter.Table {
	return encounter.Table{
		encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}},
				Attack: &encounter.Selector{Word: encounter.SelectorEnemy}},
			{Weight: 2, When: []encounter.When{{Deed: "attacked", Within: 3}},
				Toward: &encounter.Selector{Word: encounter.SelectorAttacker}},
			{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemySeen}},
				Toward: &encounter.Selector{Word: encounter.SelectorEnemy}},
			{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyNone}}, Hold: true},
		},
	}
}

// vengeanceTable is #2000's row on its own: strike whoever struck me.
func vengeanceTable() encounter.Table {
	return encounter.Table{
		encounter.AnswerTime: {
			{Weight: 1, When: []encounter.When{{Deed: "attacked", Within: 3}},
				Attack: &encounter.Selector{Word: encounter.SelectorAttacker}},
		},
	}
}

// seat is an authored [col,row] — what a member is placed at. The roster
// answers in absolute axial cells, which [cellAt] converts to.
func seat(col, row int) spatial.Position {
	return spatial.Position{X: float64(col), Y: float64(row)}
}

// meleeGoblin is a goblin with one 5-foot action and a 30-foot walk.
func meleeGoblin() encounter.SheetFacts {
	return encounter.SheetFacts{SpeedFeet: 30, Actions: []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: 5}}}
}

// silenceScene builds a hall fight in the order the members are given.
func silenceScene(
	t *testing.T, members []encounter.MemberInput, driver encounter.Driver,
	mover encounter.Mover, striker encounter.Striker, standing encounter.Participation,
) *encounter.Encounter {
	t.Helper()

	return silenceSceneWith(t, silenceSheets(members), orderAsGiven{}, members, driver, mover, striker, standing)
}

// silenceSheets is every member walking 30 feet, the goblin with its one
// melee action.
func silenceSheets(members []encounter.MemberInput) sheetFacts {
	sheets := sheetFacts{}
	for _, m := range members {
		sheets[m.ID] = encounter.SheetFacts{SpeedFeet: 30}
	}
	sheets[goblin] = meleeGoblin()

	return sheets
}

// silenceSceneWith is silenceScene with the sheets supplied, for a scene that
// changes one between turns (sheets are asked at the start of each turn).
func silenceSceneWith(
	t *testing.T, sheets sheetFacts, initiative encounter.InitiativeRoller, members []encounter.MemberInput, driver encounter.Driver,
	mover encounter.Mover, striker encounter.Striker, standing encounter.Participation,
) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 14, 8)},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheets,
			Participation: standing,
			Initiative:    initiative,
			Driver:        driver,
			Roller:        rollsLowest{},
			Actors: encounter.Actors{
				Striker:   striker,
				Mover:     mover,
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	return enc
}

// goblinBeats is every beat naming the goblin as its creature or member, in
// story order, decoded.
func goblinBeats(t *testing.T, enc *encounter.Encounter) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, b := range storyBeats(t, enc, alice) {
		if b["creature"] == string(goblin) || b["member"] == string(goblin) || b["actor"] == string(goblin) {
			out = append(out, b)
		}
	}

	return out
}

// beatWords is the beat names of a run, with the answered beats carrying
// their word so a sequence reads like the story does.
func beatWords(beats []map[string]any) []string {
	out := make([]string, 0, len(beats))
	for _, b := range beats {
		name, _ := b["beat"].(string)
		if name == encounter.BeatAnswered {
			name += ":" + b["word"].(string)
		}
		out = append(out, name)
	}

	return out
}

// stayedBeats is every `stayed` beat of a run.
func stayedBeats(beats []map[string]any) []map[string]any {
	var out []map[string]any
	for _, b := range beats {
		if b["beat"] == encounter.BeatStayed {
			out = append(out, b)
		}
	}

	return out
}

// TestAnAttackRowWithItsTargetOutOfReachSaysWhy is rpg-toolkit#2000's walk,
// reproduced through the shipped table driver: the bard mocks the goblin from
// across the hall, the goblin's `time` table answers `attack: attacker`, and
// the bard is out of reach. The turn used to go `answered … attack` and then
// `turn-ended`, so nobody could tell a goblin that chose to stand from one
// that could not reach. It now says so.
func TestAnAttackRowWithItsTargetOutOfReachSaysWhy(t *testing.T) {
	striker := &scriptedStriker{kind: encounter.OutcomeMissed}
	enc := silenceScene(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: seat(2, 3)},
		{ID: goblin, Kind: encounter.KindMonster, Position: seat(8, 3), Table: vengeanceTable()},
	}, tableDriver(), quietMover{}, striker, everyoneStanding{})

	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
	})
	require.NoError(t, err)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	require.Empty(t, striker.calls, "nobody was in reach, so nothing swung")
	require.Equal(t, cellAt(8, 3), whereIs(t, enc, goblin), "`attack` does not walk")

	beats := goblinBeats(t, enc)
	require.Equal(t, []string{"answered:attack", encounter.BeatStayed, "turn-ended"}, beatWords(beats),
		"the answer is followed by why nothing happened, then the turn ends")
	stayed := stayedBeats(beats)[0]
	require.Equal(t, "encounter:intent:attack", stayed["cause"], "the word that did nothing")
	require.Equal(t, "target out of reach", stayed["why"])
}

// TestARefusedAttackSaysWhyWhoeverDrove is the arm half of #2000: the beat is
// told where the attack is judged, so ANY driver's refused attack is visible,
// not only the table's. Each case is one reason the arm refuses.
func TestARefusedAttackSaysWhyWhoeverDrove(t *testing.T) {
	inReach := seat(3, 3)
	cases := map[string]struct {
		aliceAt spatial.Position
		intents []encounter.TurnIntent
		down    []encounter.MemberID
		why     string
	}{
		"out of reach": {
			aliceAt: seat(9, 3),
			intents: []encounter.TurnIntent{encounter.Attack{Target: alice, Action: testMeleeAction}},
			why:     "target out of reach",
		},
		"nobody named": {
			aliceAt: inReach,
			intents: []encounter.TurnIntent{encounter.Attack{Action: testMeleeAction}},
			why:     "names nobody",
		},
		"nobody seen": {
			aliceAt: inReach,
			intents: []encounter.TurnIntent{encounter.Attack{Target: "ghost", Action: testMeleeAction}},
			why:     "target not in sight",
		},
		"target down": {
			aliceAt: inReach,
			intents: []encounter.TurnIntent{encounter.Attack{Target: bob, Action: testMeleeAction}},
			down:    []encounter.MemberID{bob},
			why:     "target is down",
		},
		"swing already spent": {
			aliceAt: inReach,
			intents: []encounter.TurnIntent{
				encounter.Attack{Target: alice, Action: testMeleeAction},
				encounter.Attack{Target: alice, Action: testMeleeAction},
			},
			why: "no attack left",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			striker := &scriptedStriker{kind: encounter.OutcomeMissed}
			enc := silenceScene(t, []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: tc.aliceAt},
				{ID: bob, Kind: encounter.KindPlayer, Position: seat(2, 4)},
				{ID: goblin, Kind: encounter.KindMonster, Position: seat(2, 3)},
			}, &scriptedDriver{intents: tc.intents}, quietMover{}, striker, &downList{down: tc.down})

			on, err := enc.ClockOf(&encounter.ClockOfInput{Member: goblin})
			require.NoError(t, err)
			require.Equal(t, encounter.ClockTurn, on.Kind, "precondition: a fight")
			for active := on.Active; active != goblin; {
				out, eerr := enc.EndTurn(&encounter.EndTurnInput{Member: active})
				require.NoError(t, eerr)
				if out.Next == "" || out.Next == alice {
					break
				}
				active = out.Next
			}

			if name == "swing already spent" {
				require.Len(t, striker.calls, 1, "the first swing landed; the second never reached the striker")
			} else {
				require.Empty(t, striker.calls, "a refused attack never reaches the striker")
			}
			stayed := stayedBeats(goblinBeats(t, enc))
			require.Len(t, stayed, 1, "one refused attack, one account of why")
			require.Equal(t, "encounter:intent:attack", stayed[0]["cause"])
			require.Equal(t, tc.why, stayed[0]["why"])
		})
	}
}

// TestTowardATargetAlreadyInReachStaysAndSaysSo is the expected half of
// rpg-toolkit#2001: `toward: attacker` with the attacker already in melee
// reach moves nobody, and the story says the creature stayed because it was
// already there rather than going silent.
func TestTowardATargetAlreadyInReachStaysAndSaysSo(t *testing.T) {
	mover := &recordingMover{}
	striker := &scriptedStriker{kind: encounter.OutcomeMissed}
	enc := silenceScene(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: seat(4, 4)},
		{ID: goblin, Kind: encounter.KindMonster, Position: seat(5, 4), Table: silenceTable()},
	}, tableDriver(), mover, striker, everyoneStanding{})

	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
	})
	require.NoError(t, err)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)

	require.Len(t, striker.calls, 1, "the swing came first")
	require.Empty(t, mover.calls, "and `toward` a creature already in reach takes no step")
	require.Equal(t, cellAt(5, 4), whereIs(t, enc, goblin))

	beats := goblinBeats(t, enc)
	require.Equal(t, []string{
		"answered:attack", "missed", "answered:toward", encounter.BeatStayed, "turn-ended",
	}, beatWords(beats))
	stayed := stayedBeats(beats)[0]
	require.Equal(t, "encounter:table:toward", stayed["cause"])
	require.Equal(t, "already in reach", stayed["why"])
}

// reachLeavingMover poses an opportunity-attack window the first time the
// mover leaves a named reactor's reach — the session seam's own rule, reduced
// to adjacency, with each reactor's reaction spent once it is asked.
type reachLeavingMover struct {
	reactors []encounter.MemberID
	asked    map[encounter.MemberID]bool
	steps    []announcedStep
}

func (m *reachLeavingMover) Move(_ context.Context, enc *encounter.Encounter, step encounter.MoveStep) error {
	m.steps = append(m.steps, announcedOf(step))
	members, err := enc.Members()
	if err != nil {
		return err
	}
	for _, r := range m.reactors {
		if m.asked[r] {
			continue
		}
		for _, mem := range members {
			if mem.ID != r {
				continue
			}
			if adjacencyDistance(step.From, mem.Position) == 1 && adjacencyDistance(step.To, mem.Position) > 1 {
				m.asked[r] = true
				return &encounter.StepPausedError{Windows: []encounter.PausedWindow{
					{Audience: r, Reaction: testOpportunityAttack},
				}}
			}
		}
	}

	return nil
}

// goblinFirst is the run's initiative: the goblin, then everybody else in
// the order they were handed over.
type goblinFirst struct{}

func (goblinFirst) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	ordered := []encounter.MemberID{goblin}
	for _, m := range members {
		if m != goblin {
			ordered = append(ordered, m)
		}
	}

	return ordered, nil
}

// TestAnOpportunityAttackMidWalkRepointsAttacker is run 21865f05's beats
// 17-58, replayed, and it is the real cause of rpg-toolkit#2001.
//
// The goblin was NOT walking away from its target. `attacker` names whoever
// most recently attacked it, and an opportunity attack is an attack: the
// fighter's swing as the goblin closed on the bard (beat 47) made the FIGHTER
// the attacker, so the next `toward: attacker` (beat 51) stepped toward the
// fighter — out of the bard's reach, which posed the bard's window. The
// bard's swing (beat 55) made the bard the attacker again, and the goblin
// stepped back (beat 58). Every step was toward the attacker of that moment;
// the router never routed away from its target.
//
// THIS PINS CURRENT BEHAVIOUR, PENDING THE #2001 RULING. It passes on main
// and asserts the oscillation as what happens today, so the cause stays
// demonstrable. Under either candidate ruling — `attacker` fixed for the
// turn, or reactions excluded from the attacker deed — the goblin would stay
// beside the bard and this test goes red: that red is the fix working, not a
// regression, and the assertions should be rewritten to the ruled behaviour.
//
// The run's order and clock are kept, because they decide who is freshest:
// the goblin goes first, the bard's mockery lands on the first round, and the
// fighter's swing on the second. Geometry is the walk's own, shifted ten
// rows: in axial cells the bard is at (9,1), the fighter at (6,1), and the
// goblin arrives at (8,1) between them, as (9,-9), (6,-9) and (8,-9) in the
// run. The goblin's first turn is spent standing still (no movement on its
// sheet) so it starts its second from the run's own approach.
func TestAnOpportunityAttackMidWalkRepointsAttacker(t *testing.T) {
	bard, fighter := alice, bob
	mover := &reachLeavingMover{reactors: []encounter.MemberID{fighter, bard}, asked: map[encounter.MemberID]bool{}}
	striker := &scriptedStriker{kind: encounter.OutcomeStruck}
	members := []encounter.MemberInput{
		{ID: goblin, Kind: encounter.KindMonster, Position: seat(5, 2), Table: silenceTable()},
		{ID: bard, Kind: encounter.KindPlayer, Position: seat(9, 1)},
		{ID: fighter, Kind: encounter.KindPlayer, Position: seat(6, 1)},
	}
	sheets := silenceSheets(members)
	walking := sheets[goblin]
	standing := walking
	standing.SpeedFeet = 0
	sheets[goblin] = standing
	enc := silenceSceneWith(t, sheets, goblinFirst{}, members, tableDriver(), mover, striker, everyoneStanding{})
	require.Empty(t, mover.steps, "precondition: the goblin's first turn took no step")

	// Round one: Vicious Mockery from across the hall. The bard is the
	// goblin's attacker.
	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: bard, Targets: []encounter.MemberID{goblin},
	})
	require.NoError(t, err)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: bard})
	require.NoError(t, err)
	sheets[goblin] = walking
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: fighter})
	require.NoError(t, err)

	// Round two. The fighter's window: the goblin leaves his reach on its way
	// to the bard, and he swings.
	require.True(t, enc.Paused(), "the goblin left the fighter's reach")
	_, err = recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: fighter, Targets: []encounter.MemberID{goblin},
	})
	require.NoError(t, err)
	_, err = enc.Resume(context.Background())
	require.NoError(t, err)

	arrived := spatial.Position{X: 8, Y: 1}
	require.Len(t, striker.calls, 1, "it arrived beside the bard and swung")
	require.Equal(t, bard, striker.calls[0].Target)

	// The bard's window: `toward: attacker` now means the fighter.
	require.True(t, enc.Paused(), "the next toward stepped out of the bard's reach")
	last := mover.steps[len(mover.steps)-1]
	require.Equal(t, arrived, last.From)
	require.Equal(t, spatial.Position{X: 7, Y: 1}, last.To,
		"the step was toward the fighter, whose opportunity attack made him the attacker")
	_, err = recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: bard, Targets: []encounter.MemberID{goblin},
	})
	require.NoError(t, err)
	_, err = enc.Resume(context.Background())
	require.NoError(t, err)

	require.Equal(t, arrived, whereIs(t, enc, goblin),
		"and the bard's swing made her the attacker again, so it stepped back")
}

// TestAPassWithAMalformedCauseIsRefusedNotSilent is the gate's probe on
// #2035: a Pass naming a cause that is not a Ref must not be dropped. Only a
// zero cause is silent; a malformed one aborts the verb with ErrNoCause, as
// a Routed intent's does.
func TestAPassWithAMalformedCauseIsRefusedNotSilent(t *testing.T) {
	enc := silenceScene(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: seat(3, 3)},
		{ID: goblin, Kind: encounter.KindMonster, Position: seat(2, 3)},
	}, &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Pass{Cause: core.Ref{ID: "toward"}, Why: "already in reach"},
	}}, quietMover{}, &scriptedStriker{kind: encounter.OutcomeMissed}, everyoneStanding{})

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.ErrorIs(t, err, encounter.ErrNoCause, "never silence")
}
