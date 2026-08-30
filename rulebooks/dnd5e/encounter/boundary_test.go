// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

// journal is ONE ordered list of everything this composition did to its
// capabilities, written by both the Announcer and the Striker.
//
// Two separate recorders could each be correct about their own calls and say
// nothing about the order the two happened in — which is the only thing the
// tests below are actually about.
type journal struct{ entries []string }

func (j *journal) add(entry string) { j.entries = append(j.entries, entry) }

func (j *journal) indexOf(entry string) int {
	for i, e := range j.entries {
		if e == entry {
			return i
		}
	}
	return -1
}

type journalAnnouncer struct {
	j    *journal
	fail error
}

func (a journalAnnouncer) Announce(
	_ context.Context, _ *encounter.Encounter, crossed []encounter.Boundary,
) error {
	for _, b := range crossed {
		a.j.add(fmt.Sprintf("%s:%s:r%d", b.Kind, b.Subject, b.Round))
	}
	return a.fail
}

type journalStriker struct {
	j     *journal
	inner *scriptedStriker
}

func (s journalStriker) Strike(
	ctx context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	s.j.add("strike:" + string(attacker))
	return s.inner.Strike(ctx, enc, attacker, target, action)
}

type BoundaryTestSuite struct {
	suite.Suite
}

func TestBoundarySuite(t *testing.T) { suite.Run(t, new(BoundaryTestSuite)) }

// adjacentFight is monsterturn_test's adjacentSkeletonEncounter with the
// Announcer opened up, since that is what these tests are about.
func (s *BoundaryTestSuite) adjacentFight(
	driver encounter.TurnDriver, striker encounter.Striker, announcer encounter.Announcer,
) (*encounter.Encounter, error) {
	return s.fightWithMonsters(driver, striker, announcer, goblin)
}

// fightWithMonsters puts alice and every named monster shoulder to shoulder in
// one room, in the order given — so TWO monsters after alice really are driven
// back to back inside a single EndTurn, which is the case
// driveOneMonsterTurn's own announce exists for.
func (s *BoundaryTestSuite) fightWithMonsters(
	driver encounter.TurnDriver, striker encounter.Striker, announcer encounter.Announcer,
	monsters ...core.EntityID,
) (*encounter.Encounter, error) {
	members := []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
	}
	for i, id := range monsters {
		members = append(members, encounter.MemberInput{
			ID: id, Kind: encounter.KindMonster,
			Position:  spatial.Position{X: float64(3 + i), Y: 2},
			SpeedFeet: 30, Targeting: "closest",
			Actions: []encounter.ActionView{
				{Ref: testMeleeAction, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
			},
		})
	}
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: striker, Mover: quietMover{}, Announcer: announcer,
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
}

// TestADrivenMemberHearsItsTurnStartBeforeItSwings is the whole reason
// Announcer is a capability instead of another field on EndTurnOutput.
//
// EndTurn drives every consecutive unplayed member forward INSIDE ONE CALL
// (rpg-toolkit#1162), and those members attack during the drive. Boundaries
// merely returned and published by the caller afterwards would put this
// monster's turn-start after the swing it had already made.
//
// Nothing on a monster's sheet subscribes to a turn boundary today, so this
// ordering is currently invisible in behaviour — which is exactly why it is
// asserted directly rather than through an effect.
func (s *BoundaryTestSuite) TestADrivenMemberHearsItsTurnStartBeforeItSwings() {
	j := &journal{}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Attack{Target: alice, Action: testMeleeAction},
	}}
	striker := journalStriker{j: j, inner: &scriptedStriker{kind: encounter.OutcomeMissed}}
	enc, err := s.adjacentFight(driver, striker, journalAnnouncer{j: j})
	s.Require().NoError(err)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	started := j.indexOf("turn_started:goblin:r1")
	swung := j.indexOf("strike:goblin")
	s.Require().NotEqual(-1, started, "the goblin's turn-start was never announced: %v", j.entries)
	s.Require().NotEqual(-1, swung, "the goblin never swung: %v", j.entries)
	s.Less(started, swung,
		"a driven member must hear its turn start BEFORE it acts, not after: %v", j.entries)
}

// TestTheSECONDDrivenMemberAlsoHearsItsTurnStartFirst is the test the one
// above could not be.
//
// With a single monster after alice, that monster's turn-start is announced by
// ALICE's EndTurn — driveOneMonsterTurn's own announce never enters into it. A
// mutant that deleted the announce from the drive loop entirely left the test
// above passing, which is the whole reason this one exists.
//
// Two consecutive unplayed members is the real case: gob1's turn ends and
// gob2's begins INSIDE the drive loop, with nobody's EndTurn in between. If
// that crossing is not announced where it happens, gob2 swings before anything
// says its turn started.
func (s *BoundaryTestSuite) TestTheSECONDDrivenMemberAlsoHearsItsTurnStartFirst() {
	j := &journal{}
	striker := journalStriker{j: j, inner: &scriptedStriker{kind: encounter.OutcomeMissed}}
	enc, err := s.fightWithMonsters(
		killEveryoneStandingDriver{action: testMeleeAction}, striker, journalAnnouncer{j: j},
		goblin, bob)
	s.Require().NoError(err)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	// WHO is driven second is initiative's business, not this test's, so it
	// is derived rather than assumed — the observed order here is
	// [alice, bob, goblin], and hardcoding that would make this test about
	// orderAsGiven instead of about announcement.
	var drivenOrder []string
	for _, e := range j.entries {
		if id, ok := strings.CutPrefix(e, "turn_started:"); ok {
			id = strings.Split(id, ":")[0]
			if id != string(alice) {
				drivenOrder = append(drivenOrder, id)
			}
		}
	}
	s.Require().Len(drivenOrder, 2, "both monsters must have had a turn announced: %v", j.entries)
	first, second := drivenOrder[0], drivenOrder[1]

	// The crossing INSIDE the drive loop: the first driven member's turn ends
	// and the second's begins with nobody's EndTurn in between. This is the
	// pair that driveOneMonsterTurn's own announce is responsible for, and the
	// pair a mutant that deletes it destroys.
	s.Less(j.indexOf("turn_ended:"+first+":r1"), j.indexOf("turn_started:"+second+":r1"),
		"boundaries arrive in causal order: %v", j.entries)
	s.Less(j.indexOf("turn_started:"+second+":r1"), j.indexOf("strike:"+second),
		"the second driven member must hear its turn start BEFORE it acts: %v", j.entries)

	// And the invariant in general: nobody swings before their turn is
	// announced, whoever they are.
	for i, e := range j.entries {
		id, ok := strings.CutPrefix(e, "strike:")
		if !ok {
			continue
		}
		started := j.indexOf("turn_started:" + id + ":r1")
		s.Require().NotEqual(-1, started, "%s swung with no turn-start at all: %v", id, j.entries)
		s.Less(started, i, "%s swung before its turn was announced: %v", id, j.entries)
	}
}

// TestAFightsFirstTurnIsAnnounced closes a gap that existed for as long as
// this composition has: SetOrder has always returned TurnStarted for whoever
// won initiative, and form has always dropped it. Every fight's first turn
// began with nothing said.
func (s *BoundaryTestSuite) TestAFightsFirstTurnIsAnnounced() {
	j := &journal{}
	_, err := s.adjacentFight(
		passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, journalAnnouncer{j: j})
	s.Require().NoError(err)

	s.Contains(j.entries, "turn_started:alice:r1",
		"forming a fight must announce its first turn: %v", j.entries)
}

// TestTheRoundAdvancesWithoutARoundBoundary pins Kirk's 2026-08-27 ruling
// structurally rather than by absence: no round boundary is published, AND the
// round advancing is still visible — on the next turn boundary, which is where
// the information already lived.
func (s *BoundaryTestSuite) TestTheRoundAdvancesWithoutARoundBoundary() {
	j := &journal{}
	enc, err := s.adjacentFight(
		passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, journalAnnouncer{j: j})
	s.Require().NoError(err)

	// alice ends; the goblin is driven and passes; the order wraps back.
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	for _, e := range j.entries {
		s.NotContains(e, "round_", "no round boundary is published: %v", j.entries)
	}
	s.Contains(j.entries, "turn_started:alice:r2",
		"the wrap must still be visible, as a changed Round on the next turn boundary: %v", j.entries)
}

// TestAnnouncerIsRequiredAtBothConstructors — supplied, never defaulted
// (rpg-toolkit#1033). A nil one is not "boundaries are off"; it is every
// turn-scoped condition living forever, silently, which is the defect this
// capability was introduced to end.
func (s *BoundaryTestSuite) TestAnnouncerIsRequiredAtBothConstructors() {
	_, err := s.adjacentFight(passDriver{}, passStriker{}, nil)
	s.Require().ErrorIs(err, encounter.ErrNoAnnouncer)

	enc, err := s.adjacentFight(passDriver{}, passStriker{}, quietAnnouncer{})
	s.Require().NoError(err)

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{},
	})
	s.Require().ErrorIs(err, encounter.ErrNoAnnouncer,
		"a blob that comes back without one is as unusable as a Setup without one")
}

// TestRefusingAnnouncerNamesTheHostBug — the twin of RefusingStriker, and
// deliberately NOT a no-op: a silently-succeeding default is indistinguishable
// from a boundary nobody published, which is the thing being fixed.
func (s *BoundaryTestSuite) TestRefusingAnnouncerNamesTheHostBug() {
	err := encounter.RefusingAnnouncer{}.Announce(context.Background(), nil, []encounter.Boundary{
		{Kind: encounter.TurnEnded, Subject: alice, Round: 1},
	})
	s.Require().ErrorIs(err, encounter.ErrRefusingAnnouncer)
}

// TestAnnouncerFailureAbortsTheVerb — an announcer malfunction is a
// TurnDriver/Striker-class failure: the caller's whole verb fails, and since
// nothing is saved until the caller's own commit, it costs the retry and
// nothing else.
func (s *BoundaryTestSuite) TestAnnouncerFailureAbortsTheVerb() {
	boom := errors.New("announcer exploded")
	j := &journal{}
	enc, err := s.adjacentFight(
		passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietAnnouncer{})
	s.Require().NoError(err)

	// Reload the same world with an announcer that fails, so construction
	// itself is not the thing under test.
	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{},
		Announcer: journalAnnouncer{j: j, fail: boom},
	})
	s.Require().NoError(err)

	_, err = reloaded.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().ErrorIs(err, boom, "an announcer malfunction aborts the caller's verb")
}
