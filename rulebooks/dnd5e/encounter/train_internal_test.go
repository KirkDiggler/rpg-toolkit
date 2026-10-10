// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

var swordRef = core.Ref{Module: "test", Type: "actions", ID: "sword"}

// strikeTheLast drives every monster to attack zed, who stays standing.
type strikeTheLast struct{}

func (strikeTheLast) Act(MonsterView) (Decision, error) {
	return Decision{Intent: Attack{Target: "zed", Action: swordRef}}, nil
}

// recordingStriker records the strike as its own nested train, as session's
// striker does, and counts how often it was asked.
type recordingStriker struct{ calls int }

func (s *recordingStriker) Strike(
	_ context.Context, enc *Encounter, attacker, target MemberID, action core.Ref,
) error {
	s.calls++
	_, err := enc.RecordTrain(&RecordTrainInput{Units: []TrainUnit{{Outcome: &RecordInput{
		Kind: OutcomeMissed, Actor: attacker, Targets: []MemberID{target},
		Attack: &AttackIdentity{Ref: action.String(), Name: "Sword", DamageType: "slashing"},
	}}}})
	return err
}

func drivenFight(t *testing.T) (*Encounter, *oneDown, *recordingStriker) {
	t.Helper()
	rulebook := &oneDown{}
	striker := &recordingStriker{}
	enc, err := NewEncounter(&SetupInput{
		Retention: RetentionUnbounded,
		Field:     FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("crypt", 0, 0, 10, 10)}},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 3, Y: 2}},
			{ID: "zed", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 3}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  UnobservedEquipment{},
			Sheets:     sheetFacts{"alice": {}, "zed": {}, "goblin": {SpeedFeet: 30, Actions: []ActionView{{Ref: swordRef, Name: "Sword", RangeFeet: 5, Kind: "melee"}}}},
			Standing:   rulebook,
			Initiative: orderAsGiven{},
			Driver:     strikeTheLast{},
			Actors:     Actors{Striker: striker, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)
	require.Len(t, enc.bubbles, 1, "first light started the fight with alice to move")
	return enc, rulebook, striker
}

func beatKinds(t *testing.T, enc *Encounter) []string {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	kinds := make([]string, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		kinds = append(kinds, beat["beat"].(string))
	}
	return kinds
}

// TestAParkedEndingFiresBeforeTheConsultDrivesAMonster: the fall of the player
// holding the turn hands it to a monster, whose strike is its own nested
// train. A parked ending closes right after the down beat, so no turn is
// handed over and no strike is told after "ended". The control is the same
// fall with nothing parked: the goblin does strike.
func TestAParkedEndingFiresBeforeTheConsultDrivesAMonster(t *testing.T) {
	t.Run("control: the fall hands the turn to a monster that strikes", func(t *testing.T) {
		enc, rulebook, striker := drivenFight(t)
		rulebook.who = "alice"
		_, _, err := enc.noticeDown()
		require.NoError(t, err)
		require.Equal(t, 1, striker.calls, "the scene really drives a strike off the fall")
	})

	t.Run("a parked ending closes first", func(t *testing.T) {
		enc, rulebook, striker := drivenFight(t)
		rulebook.who = "alice"
		_, _, err := enc.noticeDown(participationPassInput{held: &heldEnding{key: "called", at: 0}})
		require.NoError(t, err)

		require.Zero(t, striker.calls, "no monster was driven once the ending fired")
		kinds := beatKinds(t, enc)
		require.Equal(t, "ended", kinds[len(kinds)-1], "nothing is told after the ending: %v", kinds)
		require.Contains(t, kinds, "down")
		status, err := enc.Status()
		require.NoError(t, err)
		require.Equal(t, "called", status.Outcome.Ending)
	})
}

// failingStanding is a rulebook that cannot answer.
type failingStanding struct{ oneDown }

func (failingStanding) Assess([]MemberID) (*ParticipationAssessment, error) {
	return nil, ErrNoStanding
}

// TestAFailedTrainLeavesNoParkedEnding: a train whose deed parked an ending and
// whose consult then failed leaves nothing parked, and the encounter open, for
// the next verb to find.
func TestAFailedTrainLeavesNoParkedEnding(t *testing.T) {
	enc, _, _ := drivenFight(t)
	enc.participation = failingStanding{}

	_, _, err := enc.appendTrainAndNotice("test", []preparedUnit{{
		land: func() error {
			_, closeErr := enc.closeWith("called", 0)
			return closeErr
		},
	}})
	require.ErrorIs(t, err, ErrNoStanding)

	require.Nil(t, enc.heldEnding, "nothing is left parked")
	require.False(t, enc.holdEndings, "and the hold is lifted")
	require.Nil(t, enc.outcome, "the parked ending never closed anything")
}
