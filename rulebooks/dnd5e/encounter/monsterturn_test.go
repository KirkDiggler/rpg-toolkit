// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// MonsterTurnTestSuite covers the monster's turn (rpg-project#254): the
// composition side of TurnDriver.Act(MonsterView), Striker, and
// driveMonsterTurns' build-view/act/execute loop.
type MonsterTurnTestSuite struct {
	suite.Suite
}

func TestMonsterTurnSuite(t *testing.T) {
	suite.Run(t, new(MonsterTurnTestSuite))
}

var testMeleeAction = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "melee"}

// scriptedDriver returns intents from a fixed queue, one per Act call, and
// records every View it was handed so a test can assert on what this
// composition told it. Once the queue is exhausted it returns Pass,
// matching PassDriver's own contract for a well-behaved driver with nothing
// left to say.
type scriptedDriver struct {
	intents []encounter.TurnIntent
	errs    []error
	calls   []encounter.MonsterView
}

func (d *scriptedDriver) Act(view encounter.MonsterView) (encounter.TurnIntent, error) {
	d.calls = append(d.calls, view)
	i := len(d.calls) - 1
	if i < len(d.errs) && d.errs[i] != nil {
		return nil, d.errs[i]
	}
	if i < len(d.intents) {
		return d.intents[i], nil
	}
	return encounter.Pass{}, nil
}

// scriptedStriker records every Strike call and, unless told to fail,
// records a fixed outcome via [encounter.Encounter.Record] itself — the same
// obligation a real Striker implementation carries.
type scriptedStriker struct {
	kind encounter.OutcomeKind
	fail error

	calls []struct {
		Attacker, Target encounter.MemberID
		Action           core.Ref
	}
}

func (s *scriptedStriker) Strike(
	_ context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	s.calls = append(s.calls, struct {
		Attacker, Target encounter.MemberID
		Action           core.Ref
	}{attacker, target, action})

	if s.fail != nil {
		return s.fail
	}

	_, err := enc.Record(&encounter.RecordInput{
		Kind:    s.kind,
		Actor:   attacker,
		Targets: []encounter.MemberID{target},
		Attack:  &encounter.AttackIdentity{Ref: action.String(), Name: "Test Strike", DamageType: "bludgeoning"},
	})
	return err
}

// adjacentSkeletonEncounter builds a one-room fight: alice and a monster
// standing next to each other (co-located rooms form the bubble at first
// light — see twoMemberEncounter's own doc, one file over), the monster
// carrying one melee action at 5 feet reach and a 30-foot walking speed.
func (s *MonsterTurnTestSuite) adjacentSkeletonEncounter(driver encounter.TurnDriver, striker encounter.Striker) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: striker,
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
			{
				ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 3, Y: 2},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{
					{Ref: testMeleeAction, Name: "Shortsword", ReachFeet: 5, Kind: "melee"},
				},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// farSkeletonEncounter is the same fight with the monster placed OUT of the
// melee action's 5-foot reach (one cell) but still within its own sight (an
// unlimited-sight fixture, so contact still forms the bubble at first light)
// and within its own movement budget — near enough to close the distance in
// one Move, far enough that an immediate Attack is [encounter.ErrBadIntent].
func (s *MonsterTurnTestSuite) farSkeletonEncounter(driver encounter.TurnDriver, striker encounter.Striker) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: striker,
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
			{
				ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 6, Y: 2},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{
					{Ref: testMeleeAction, Name: "Shortsword", ReachFeet: 5, Kind: "melee"},
				},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// clockBeats returns every "tag":"clock" beat's decoded payload, in story
// order.
func (s *MonsterTurnTestSuite) clockBeats(enc *encounter.Encounter, audience core.EntityID) []map[string]interface{} {
	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	var out []map[string]interface{}
	for _, entry := range story {
		switch entry.Tags["tag"] {
		case "clock", "outcome", "movement":
		default:
			continue
		}
		var p map[string]interface{}
		s.Require().NoError(json.Unmarshal(entry.Payload, &p))
		out = append(out, p)
	}
	return out
}

// TestMonsterViewCarriesStaticFactsAndSeen pins the shape a TurnDriver is
// actually handed: its own static facts verbatim (Self, Position, Actions,
// Targeting), a Seen entry for alice with Distance and InReach computed
// against the monster's OWN action, and a Budget of one attack plus its own
// SpeedFeet.
func (s *MonsterTurnTestSuite) TestMonsterViewCarriesStaticFactsAndSeen() {
	driver := &scriptedDriver{}
	enc := s.adjacentSkeletonEncounter(driver, &scriptedStriker{kind: encounter.OutcomeMissed})

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Require().Len(driver.calls, 1, "one Pass ends the goblin's turn in a single Act call")
	view := driver.calls[0]

	s.Equal(goblin, view.Self)
	s.Equal(spatial.Position{X: 3, Y: 2}, view.Position)
	s.Equal("closest", view.Targeting)
	s.Require().Len(view.Actions, 1)
	s.Equal(testMeleeAction, view.Actions[0].Ref)
	s.Equal(5, view.Actions[0].ReachFeet)
	s.Equal(1, view.Budget.AttacksLeft)
	s.Equal(30, view.Budget.MovementFeet)

	s.Require().Len(view.Seen, 1)
	seen := view.Seen[0]
	s.Equal(alice, seen.ID)
	s.Equal(encounter.KindPlayer, seen.Kind)
	s.True(seen.Standing)
	s.Equal(spatial.Position{X: 2, Y: 2}, seen.Position)
	s.InDelta(1.0, seen.DistanceCells, 0.001, "adjacent cells are 1 cell apart")
	s.True(seen.InReach[testMeleeAction], "1 cell is within a 5-foot (1-cell) reach")
}

// TestAttackExecutesAndConsumesTheBudget: an in-reach Attack calls the
// Striker exactly once, the story carries the recorded outcome, and the
// NEXT Act call (before the goblin's own Pass) sees AttacksLeft at zero —
// the same budget object every call in this turn shares.
func (s *MonsterTurnTestSuite) TestAttackExecutesAndConsumesTheBudget() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Attack{Target: alice, Action: testMeleeAction},
	}}
	striker := &scriptedStriker{kind: encounter.OutcomeMissed}
	enc := s.adjacentSkeletonEncounter(driver, striker)

	et, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(alice, et.Next, "the goblin's whole turn — attack, then pass — still hands play back to alice")

	s.Require().Len(striker.calls, 1)
	s.Equal(goblin, striker.calls[0].Attacker)
	s.Equal(alice, striker.calls[0].Target)
	s.Equal(testMeleeAction, striker.calls[0].Action)

	s.Require().Len(driver.calls, 2, "attack, then a second Act call that passes")
	s.Equal(1, driver.calls[0].Budget.AttacksLeft, "the attack had not executed yet when this view was built")
	s.Equal(0, driver.calls[1].Budget.AttacksLeft, "the executed attack is reflected in the next view")

	beats := s.clockBeats(enc, alice)
	var sawMissed bool
	for _, b := range beats {
		if b["beat"] == "missed" && b["actor"] == string(goblin) {
			sawMissed = true
		}
	}
	s.True(sawMissed, "the striker's recorded outcome is in the story: %+v", beats)
}

// TestAttackOnOutOfReachTargetEndsTheTurnWithoutAborting is the brief's own
// gate case (c): a driver returning Attack on a target its own action
// cannot reach is [encounter.ErrBadIntent] internally, but that must never
// reach the caller — EndTurn still succeeds, and the Striker is never
// called.
func (s *MonsterTurnTestSuite) TestAttackOnOutOfReachTargetEndsTheTurnWithoutAborting() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Attack{Target: alice, Action: testMeleeAction},
	}}
	striker := &scriptedStriker{kind: encounter.OutcomeMissed}
	enc := s.farSkeletonEncounter(driver, striker)

	et, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err, "a bad intent ends the goblin's turn; it does not abort EndTurn")
	s.Equal(alice, et.Next)

	s.Empty(striker.calls, "an out-of-reach attack never reaches the Striker")

	beats := s.clockBeats(enc, alice)
	for _, b := range beats {
		s.NotEqual("missed", b["beat"])
		s.NotEqual("struck", b["beat"])
	}
}

// TestAttackOnAnUnseenTargetEndsTheTurnWithoutAborting: a target this
// composition never told the driver about (not in Seen) is refused exactly
// as an out-of-reach one is — a driver cannot attack what it was not shown.
func (s *MonsterTurnTestSuite) TestAttackOnAnUnseenTargetEndsTheTurnWithoutAborting() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Attack{Target: bob, Action: testMeleeAction},
	}}
	striker := &scriptedStriker{kind: encounter.OutcomeMissed}
	enc := s.adjacentSkeletonEncounter(driver, striker)

	et, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(alice, et.Next)
	s.Empty(striker.calls)
}

// TestMoveWalksTowardTheTargetAndUpdatesTheView: a Move intent executes cell
// by cell via the same stepTo mechanism the pump uses, "moved" beats land in
// the story, and the FOLLOWING Act call in the same turn sees the goblin's
// new position and the spent movement budget.
func (s *MonsterTurnTestSuite) TestMoveWalksTowardTheTargetAndUpdatesTheView() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Move{Path: []spatial.Position{{X: 5, Y: 2}, {X: 4, Y: 2}}},
	}}
	enc := s.farSkeletonEncounter(driver, &scriptedStriker{kind: encounter.OutcomeMissed})

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Require().Len(driver.calls, 2, "move, then a second Act call that passes")
	s.Equal(spatial.Position{X: 6, Y: 2}, driver.calls[0].Position, "the view BEFORE the move is the starting cell")
	s.Equal(spatial.Position{X: 4, Y: 2}, driver.calls[1].Position, "both cells of the path executed")
	s.Equal(30, driver.calls[0].Budget.MovementFeet)
	s.Equal(20, driver.calls[1].Budget.MovementFeet, "2 cells at 5 feet each spent from the 30-foot budget")

	members, err := enc.Members()
	s.Require().NoError(err)
	var goblinPos spatial.Position
	for _, m := range members {
		if m.ID == goblin {
			goblinPos = m.Position
		}
	}
	s.Equal(spatial.Position{X: 4, Y: 2}, goblinPos)

	beats := s.clockBeats(enc, alice)
	var moved int
	for _, b := range beats {
		if b["beat"] == "moved" {
			moved++
		}
	}
	s.Equal(2, moved, "one moved beat per executed cell: %+v", beats)
}

// TestMoveExceedingTheBudgetEndsTheTurnWithoutAborting: a driver asking for
// more cells than it can afford is [encounter.ErrBadIntent] — the turn ends,
// EndTurn still succeeds, and NOTHING in the over-budget path executes (not
// even the affordable prefix): a driver that cannot do arithmetic on its own
// budget gets its turn ended rather than a partial move it did not ask for.
func (s *MonsterTurnTestSuite) TestMoveExceedingTheBudgetEndsTheTurnWithoutAborting() {
	// The goblin's 30-foot speed affords 6 cells; ask for 7.
	path := make([]spatial.Position, 7)
	for i := range path {
		path[i] = spatial.Position{X: float64(6 - i), Y: 2}
	}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Move{Path: path}}}
	enc := s.farSkeletonEncounter(driver, &scriptedStriker{kind: encounter.OutcomeMissed})

	et, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(alice, et.Next)

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == goblin {
			s.Equal(spatial.Position{X: 6, Y: 2}, m.Position, "an over-budget move never starts")
		}
	}
}

// TestAnEmptyMoveEndsTheTurnWithoutAborting: an empty path is nonsense (Pass
// is the way to say "nothing left to do") and is refused the same way.
func (s *MonsterTurnTestSuite) TestAnEmptyMoveEndsTheTurnWithoutAborting() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Move{}}}
	enc := s.farSkeletonEncounter(driver, &scriptedStriker{kind: encounter.OutcomeMissed})

	et, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(alice, et.Next)
}

// TestMoveIntoVoidStopsPartwayWithoutEndingTheTurn: a path whose first cell
// is floor and whose second is not stops there — exactly the refusal a
// player's own walk would meet — WITHOUT treating the whole intent as bad:
// the turn keeps running, and the driver is asked again from wherever the
// walk actually stopped ("the same refusals a player's walk meets").
func (s *MonsterTurnTestSuite) TestMoveIntoVoidStopsPartwayWithoutEndingTheTurn() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		// The room is 10x10 (cells 0..9); (12,2) is outside it — void, not
		// floor. (5,2) is the one cell of progress this path can make.
		encounter.Move{Path: []spatial.Position{{X: 5, Y: 2}, {X: 12, Y: 2}}},
	}}
	enc := s.farSkeletonEncounter(driver, &scriptedStriker{kind: encounter.OutcomeMissed})

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Require().Len(driver.calls, 2, "the partial move does not end the turn; the driver is asked again")
	s.Equal(spatial.Position{X: 5, Y: 2}, driver.calls[1].Position, "the successful cell was kept")
	s.Equal(25, driver.calls[1].Budget.MovementFeet, "only the ONE cell that succeeded was spent")
}

// TestPassDriverAlwaysPasses pins rpg-toolkit#1167: the exported, reusable
// TurnDriver that always ends an unplayed member's turn with no other
// effect — the same v1 behaviour every scene got automatically before this
// capability existed.
func (s *MonsterTurnTestSuite) TestPassDriverAlwaysPasses() {
	intent, err := (encounter.PassDriver{}).Act(encounter.MonsterView{Self: goblin})
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

// TestADriverMalfunctionAbortsTheWholeCall: unlike ErrBadIntent, a driver's
// own Go error is NOT swallowed — it is a malfunction, and EndTurn reports
// it.
func (s *MonsterTurnTestSuite) TestADriverMalfunctionAbortsTheWholeCall() {
	boom := errors.New("driver malfunction")
	driver := &scriptedDriver{errs: []error{boom}}
	enc := s.adjacentSkeletonEncounter(driver, &scriptedStriker{kind: encounter.OutcomeMissed})

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().ErrorIs(err, boom)
}

// TestAStrikerMalfunctionAbortsTheWholeCall: a Striker error is a
// malfunction too (a resolution failure, a corrupt sheet) — not a miss,
// which is recorded rather than returned as an error.
func (s *MonsterTurnTestSuite) TestAStrikerMalfunctionAbortsTheWholeCall() {
	boom := errors.New("striker malfunction")
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Attack{Target: alice, Action: testMeleeAction},
	}}
	striker := &scriptedStriker{fail: boom}
	enc := s.adjacentSkeletonEncounter(driver, striker)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().ErrorIs(err, boom)
}

// TestSetupRefusesAnEncounterWithNoStriker mirrors
// TestSetupRefusesAnEncounterWithNoTurnDriver (clocks_test.go) for the new
// capability: a Striker-less encounter is refused at construction rather
// than discovered when the first monster decides to attack.
func (s *MonsterTurnTestSuite) TestSetupRefusesAnEncounterWithNoStriker() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: room1, Width: 8, Height: 8}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().ErrorIs(err, encounter.ErrNoStriker)
}

// TestLoadRefusesAnEncounterWithNoStriker is the other door — see
// TestSetupRefusesAnEncounterWithNoStriker's own doc.
func (s *MonsterTurnTestSuite) TestLoadRefusesAnEncounterWithNoStriker() {
	saved := s.adjacentSkeletonEncounter(passDriver{}, passStriker{}).ToData()

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  saved,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{},
	})
	s.Require().ErrorIs(err, encounter.ErrNoStriker)
}

// TestMemberFactsRoundTripThroughToDataAndLoadEncounter pins the unified
// member record (Kirk, rpg-project#254 review): SpeedFeet, SightFeet,
// Actions and Targeting are encounter-owned primitives, persisted and
// restored verbatim exactly as Name already is.
func (s *MonsterTurnTestSuite) TestMemberFactsRoundTripThroughToDataAndLoadEncounter() {
	enc := s.adjacentSkeletonEncounter(passDriver{}, passStriker{})
	saved := enc.ToData()

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  saved,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{},
	})
	s.Require().NoError(err)

	members, err := loaded.Members()
	s.Require().NoError(err)
	var found bool
	for _, m := range members {
		if m.ID != goblin {
			continue
		}
		found = true
		s.Equal(30, m.SpeedFeet)
		s.Equal("closest", m.Targeting)
		s.Require().Len(m.Actions, 1)
		s.Equal(testMeleeAction, m.Actions[0].Ref)
		s.Equal("Shortsword", m.Actions[0].Name)
		s.Equal(5, m.Actions[0].ReachFeet)
		s.Equal("melee", m.Actions[0].Kind)
	}
	s.True(found)
}

// TestRefusingStrikerFailsLoudly pins the construction-only-world Striker: a
// driven turn that reaches it is a host bug (an encounter never meant to run
// EndTurn/form at all — a placement probe, a template's acceptance test),
// and the honest answer is a named error rather than a silently fabricated
// hit.
func (s *MonsterTurnTestSuite) TestRefusingStrikerFailsLoudly() {
	err := (encounter.RefusingStriker{}).Strike(context.Background(), nil, goblin, alice, testMeleeAction)
	s.Require().ErrorIs(err, encounter.ErrRefusingStriker)
}

// TestSetupRejectsANegativeMemberFact and TestJoinRejectsANegativeMemberFact
// pin Copilot's PR #1187 review finding: SpeedFeet, SightFeet and an
// action's ReachFeet are feet CellsFromFeet divides by FeetPerCell, and a
// negative one is a caller defect this composition catches at the door
// rather than letting it produce a nonsense budget or reach later.
func (s *MonsterTurnTestSuite) TestSetupRejectsANegativeMemberFact() {
	base := func(mutate func(*encounter.MemberInput)) *encounter.SetupInput {
		mi := encounter.MemberInput{
			ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 1, Y: 1},
			SpeedFeet: 30, SightFeet: 60,
			Actions: []encounter.ActionView{{Ref: testMeleeAction, Name: "Claw", ReachFeet: 5}},
		}
		mutate(&mi)
		return &encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{},
			Field: encounter.FieldInput{
				Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
				Rooms:  []encounter.RoomInput{{ID: room1, Width: 8, Height: 8}},
			},
			Members: []encounter.MemberInput{mi},
			Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		}
	}

	s.Run("negative speed", func() {
		_, err := encounter.NewEncounter(base(func(mi *encounter.MemberInput) { mi.SpeedFeet = -5 }))
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})
	s.Run("negative sight", func() {
		_, err := encounter.NewEncounter(base(func(mi *encounter.MemberInput) { mi.SightFeet = -5 }))
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})
	s.Run("negative action reach", func() {
		_, err := encounter.NewEncounter(base(func(mi *encounter.MemberInput) { mi.Actions[0].ReachFeet = -5 }))
		s.Require().ErrorIs(err, encounter.ErrNoMember)
	})
}

// TestJoinRejectsANegativeMemberFactBeforeMutating pins that Join validates
// BEFORE placing the entity — unlike Setup's construct-in-a-local safety
// net, Join mutates a LIVE encounter, so a rejected Join must leave no
// half-placed entity behind (Copilot's suppressed PR #1187 comment).
func (s *MonsterTurnTestSuite) TestJoinRejectsANegativeMemberFactBeforeMutating() {
	enc := s.adjacentSkeletonEncounter(passDriver{}, passStriker{})

	_, err := enc.Join(&encounter.JoinInput{
		Member: "wolf", Kind: encounter.KindMonster, Cell: spatial.Position{X: 5, Y: 5},
		SpeedFeet: -10,
	})
	s.Require().ErrorIs(err, encounter.ErrNoMember)

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		s.NotEqual(core.EntityID("wolf"), m.ID, "the rejected join left no half-placed entity behind")
	}
}

// seenFor finds subject's own SeenMember entry in view.Seen, failing the
// test if absent.
func (s *MonsterTurnTestSuite) seenFor(view encounter.MonsterView, subject encounter.MemberID) encounter.SeenMember {
	s.T().Helper()
	for _, sm := range view.Seen {
		if sm.ID == subject {
			return sm
		}
	}
	s.Require().Fail("not in Seen", "%q", subject)
	return encounter.SeenMember{}
}

// TestSeenMemberPathWalksAroundAWall pins [SeenMember.Path] (rpg-project#254
// review: MonsterView must stay DATA — loggable, replayable,
// fixture-buildable, per rpg-project#235's Debug Feed journey and the
// monster-ai brainstorm's "the decision layer never reaches live state" —
// so this is precomputed per sighting rather than a callback behavior.Basic
// invokes). This fixture places a wall between the monster and its target
// so a straight-line step would fail, and asserts the returned path
// actually goes AROUND it, stopping adjacent rather than on the target's
// own occupied cell.
func (s *MonsterTurnTestSuite) TestSeenMemberPathWalksAroundAWall() {
	// A 5x3 room. A wall spans the room's middle column (x=2) except at
	// y=0, leaving exactly one gap to route through.
	//
	//   x: 0 1 2 3 4
	// y=0: .  .  .  .  .   <- gap
	// y=1: .  .  #  .  .
	// y=2: .  .  #  .  .
	wall := []spatial.Boundary{
		{From: spatial.Position{X: 2, Y: 1}, To: spatial.Position{X: 3, Y: 1}, BlocksMovement: true},
		{From: spatial.Position{X: 2, Y: 2}, To: spatial.Position{X: 3, Y: 2}, BlocksMovement: true},
	}
	driver := &scriptedDriver{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: &scriptedStriker{kind: encounter.OutcomeMissed},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: room1, Width: 5, Height: 3, Boundaries: wall}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 4, Y: 2}},
			{
				ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 0, Y: 2},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{{Ref: testMeleeAction, Name: "Claw", ReachFeet: 5, Kind: "melee"}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Require().Len(driver.calls, 1)
	aliceSeen := s.seenFor(driver.calls[0], alice)
	s.Require().NotEmpty(aliceSeen.Path)
	s.NotEqual(spatial.Position{X: 4, Y: 2}, aliceSeen.Path[len(aliceSeen.Path)-1],
		"the path stops ADJACENT to alice, not on her own occupied cell")

	// The path must cross the wall column through its one gap (a cell with
	// X==2 is not itself illegal to visit — (2,1) and (2,2) sit on the near
	// side of the wall and are legal to stand on, just not to cross FROM at
	// that row — so what the wall's existence actually requires is that
	// SOME step in the path uses the gap itself).
	usedTheGap := false
	for _, p := range aliceSeen.Path {
		if p == (spatial.Position{X: 2, Y: 0}) || p == (spatial.Position{X: 3, Y: 0}) {
			usedTheGap = true
		}
	}
	s.True(usedTheGap, "the path must cross the wall column through its one gap at y=0: %+v", aliceSeen.Path)
}

// TestSeenMemberPathIsEmptyWhenSightedButUnreachable pins the case Sight
// and walkability disagree on: two floor regions separated by a void gap
// wide enough that no walkable route bridges them, with the void itself
// transparent to sight — a sighting nothing in [MonsterView.Seen]'s own
// InReach/Standing fields would otherwise mark as unusual, and exactly the
// case Path must be able to say "cannot get there" about without also
// claiming the sighting itself is bogus.
func (s *MonsterTurnTestSuite) TestSeenMemberPathIsEmptyWhenSightedButUnreachable() {
	driver := &scriptedDriver{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: &scriptedStriker{kind: encounter.OutcomeMissed},
		Field: encounter.FieldInput{
			// Sight crosses void here (VoidIsTransparent), but a 4-cell gap
			// of void between the two rooms is still not floor for either
			// — nothing walkable bridges them.
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsTransparent()},
			Rooms: []encounter.RoomInput{
				{ID: room1, Width: 2, Height: 2},
				{ID: room2, Width: 2, Height: 2, Origin: spatial.Position{X: 6, Y: 0}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room2, Position: spatial.Position{X: 0, Y: 0}},
			{
				ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 0, Y: 0},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{{Ref: testMeleeAction, Name: "Claw", ReachFeet: 5, Kind: "melee"}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Require().Len(driver.calls, 1)
	aliceSeen := s.seenFor(driver.calls[0], alice)
	s.Empty(aliceSeen.Path, "seen across the gap, but nothing walkable reaches her")
}

// TestSeenMemberPathStopsAtTheMemberOwnLongestReachNotJustAdjacent pins the
// reach-aware stopping distance (Kirk, rpg-project#254 review): a monster
// with a 10-foot reach weapon (2 cells) should stop TWO cells from its
// target, not walk needlessly to melee-adjacent — Path already ends
// wherever InReach would turn true, so a driver never has to check InReach
// before consulting Path.
func (s *MonsterTurnTestSuite) TestSeenMemberPathStopsAtTheMemberOwnLongestReachNotJustAdjacent() {
	reachWeapon := core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "reach"}
	driver := &scriptedDriver{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: &scriptedStriker{kind: encounter.OutcomeMissed},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: room1, Width: 10, Height: 3}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 0, Y: 0}},
			{
				ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 6, Y: 0},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{{Ref: reachWeapon, Name: "Glaive", ReachFeet: 10, Kind: "melee"}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Require().Len(driver.calls, 1)
	aliceSeen := s.seenFor(driver.calls[0], alice)
	s.Require().NotEmpty(aliceSeen.Path)
	last := aliceSeen.Path[len(aliceSeen.Path)-1]
	s.InDelta(2.0, enc.Distance(last, spatial.Position{X: 0, Y: 0}), 0.001,
		"a 10-foot (2-cell) reach stops 2 cells out, not adjacent: %+v", aliceSeen.Path)
}

// TestSeenMemberPathIsEmptyWhenAlreadyWithinReach is the bug the
// end-to-end behavior.Basic test found: a member already at exactly
// bestReachCells' distance (no earlier cell to stop at before the
// target's own) must not return a one-element Path onto the target's own
// cell — Path must be empty, matching its own documented contract ("empty
// ... or this member is already within reach without moving at all").
func (s *MonsterTurnTestSuite) TestSeenMemberPathIsEmptyWhenAlreadyWithinReach() {
	driver := &scriptedDriver{}
	enc := s.adjacentSkeletonEncounter(driver, &scriptedStriker{kind: encounter.OutcomeMissed})

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Require().Len(driver.calls, 1, "an in-reach attack ends the goblin's turn in one Act call")
	aliceSeen := s.seenFor(driver.calls[0], alice)
	s.True(aliceSeen.InReach[testMeleeAction], "control: alice really is in reach at the start")
	s.Empty(aliceSeen.Path, "already in reach — nothing to walk onto her cell for")
}

// TestSeenMemberPathFindsTheNearestInRangeCellNotJustTheRouteToTarget pins
// Copilot's PR #1191 review: a route to the target's own cell and a route
// to "any cell within reach of the target" are different questions the
// moment a wall makes them different. This fixture puts a cell right next
// to the monster's own starting position that is GEOMETRICALLY within a
// generous reach (InReach is distance-only, matching 5e's own reach rule —
// walls do not extend it) but is nowhere near the walking route the wall
// forces toward the target's own cell. The nearest WALKABLE in-range cell
// is one step away; the naive "truncate the route to the target" approach
// would instead walk most of the way around the wall before ever
// mentioning it.
func (s *MonsterTurnTestSuite) TestSeenMemberPathFindsTheNearestInRangeCellNotJustTheRouteToTarget() {
	longReach := core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "long-reach"}
	// A 5x3 room. A wall spans the column between x=1 and x=2 at y=0 and
	// y=1, leaving the bottom row (y=2) as the only gap.
	//
	//   x: 0 1 | 2 3 4
	// y=0: .  . | .  .  .
	// y=1: .  . | .  .  .
	// y=2: .  .   .  .  .   <- gap
	wall := []spatial.Boundary{
		{From: spatial.Position{X: 1, Y: 0}, To: spatial.Position{X: 2, Y: 0}, BlocksMovement: true},
		{From: spatial.Position{X: 1, Y: 1}, To: spatial.Position{X: 2, Y: 1}, BlocksMovement: true},
	}
	driver := &scriptedDriver{}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: &scriptedStriker{kind: encounter.OutcomeMissed},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: room1, Width: 5, Height: 3, Boundaries: wall}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 4, Y: 0}},
			{
				ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 0, Y: 0},
				SpeedFeet: 60, Targeting: "closest",
				// 15 feet (3 cells) — deliberately generous so the cell
				// right beside the monster's own start (1 step away,
				// Chebyshev distance 3 from alice) is already in range,
				// making the detour-vs-shortcut distinction unambiguous.
				Actions: []encounter.ActionView{{Ref: longReach, Name: "Whip", ReachFeet: 15, Kind: "melee"}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Require().Len(driver.calls, 1, "the first Act call already finds a target in range, one step away")
	aliceSeen := s.seenFor(driver.calls[0], alice)
	s.Equal([]spatial.Position{{X: 1, Y: 0}}, aliceSeen.Path,
		"the nearest WALKABLE in-range cell is one step away, not several steps around the wall")
}

// killerStriker wraps scriptedStriker's own outcome-recording, additionally
// marking the STRUCK CALL'S OWN target down in standing BEFORE recording —
// the same order the real session seam keeps (rpg-toolkit#1083: a swing's
// sheet is written before its outcome is recorded), so the standing consult
// inside THIS SAME Record call already sees the killing blow.
//
// Reads target from Strike's own argument rather than a field configured at
// construction (Copilot, PR #1202 review): a fixed field the caller sets up
// front is a value that can drift from what a future scene's driver actually
// declares, silently marking the wrong member down. There is only ever one
// honest answer to "who did this strike hit" — the one Strike itself was
// called with.
type killerStriker struct {
	*scriptedStriker
	standing *downList
}

func (k *killerStriker) Strike(
	ctx context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	k.standing.down = append(k.standing.down, target)
	return k.scriptedStriker.Strike(ctx, enc, attacker, target, action)
}

// TestDrivenKillingBlowEndsTheDriveCleanly reproduces rpg-project#254's live
// walk anomaly #2 (the tomb evidence, round 4:
// "drive monster turns \"skeleton-1\": end: end turn: clock is idle").
//
// A driven member's own strike can be the fight's own killing blow —
// [Encounter.noticeDown], reached through Record (which Strike calls before
// returning), dissolves the SAME bubble driveMonsterTurns is still mid-loop
// over the instant it notices (rpg-toolkit#1078, ByDefeat) — before the loop
// gets back around to closing THIS member's own turn out of it. bubble.End
// then asked an idle clock to end a turn it no longer had, and play/clock's
// own ErrIdle turned an entirely ordinary outcome — a monster winning the
// fight it was in — into an error on the CALLER's own EndTurn. That is
// exactly the asymmetry this file's own doc draws for a driver malfunction
// (TestADriverMalfunctionAbortsTheWholeCall) versus everything else
// (TestAttackOnOutOfReachTargetEndsTheTurnWithoutAborting and friends): only
// a malfunction earns aborting the caller's verb, and the fight ending is
// not one.
func (s *MonsterTurnTestSuite) TestDrivenKillingBlowEndsTheDriveCleanly() {
	standing := &downList{}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{
		encounter.Attack{Target: alice, Action: testMeleeAction},
	}}
	striker := &killerStriker{
		scriptedStriker: &scriptedStriker{kind: encounter.OutcomeStruck},
		standing:        standing,
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: striker,
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: room1, Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: room1, Position: spatial.Position{X: 2, Y: 2}},
			{
				ID: goblin, Kind: encounter.KindMonster, Room: room1, Position: spatial.Position{X: 3, Y: 2},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{
					{Ref: testMeleeAction, Name: "Shortsword", ReachFeet: 5, Kind: "melee"},
				},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	et, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err, "a driven turn that wins the fight must not abort the caller's own EndTurn")
	s.Equal(alice, et.Next,
		"there is no bubble left to name a turn IN; EndTurnOutput.Next reports the caller instead")

	clockOf, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(encounter.ClockWorld, clockOf.Kind, "the bubble the killing blow ended left nobody still in it")

	beats := s.clockBeats(enc, alice)
	var sawStruck, sawDissolved bool
	for _, b := range beats {
		switch b["beat"] {
		case "struck":
			sawStruck = true
		case "bubble-dissolved":
			sawDissolved = true
			s.Equal("defeat", b["cause"])
		}
	}
	s.True(sawStruck, "the killing blow itself is on the record: %+v", beats)
	s.True(sawDissolved, "and so is the fight it ended: %+v", beats)
}
