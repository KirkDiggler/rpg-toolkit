// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

var meleeRef = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "melee"}

// BasicTestSuite unit-tests Basic's decision logic directly against
// hand-built MonsterView values — Basic needs nothing more than the view to
// decide, so these tests need no live *encounter.Encounter at all. MonsterView
// is plain data throughout (rpg-project#254 review): Seen[i].Path is
// precomputed, not a callback, so a fixture just sets the field it wants.
type BasicTestSuite struct {
	suite.Suite
}

func TestBasicSuite(t *testing.T) {
	suite.Run(t, new(BasicTestSuite))
}

func (s *BasicTestSuite) TestPassesWithNothingSeen() {
	intent, err := (behavior.Basic{}).Act(encounter.MonsterView{
		Self: "goblin", Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	})
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

func (s *BasicTestSuite) TestPassesWhenOnlyMonstersAreSeen() {
	intent, err := (behavior.Basic{}).Act(encounter.MonsterView{
		Self:   "goblin",
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Seen:   []encounter.SeenMember{{ID: "wolf", Kind: encounter.KindMonster, Standing: true}},
	})
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

func (s *BasicTestSuite) TestIgnoresADownedPlayer() {
	intent, err := (behavior.Basic{}).Act(encounter.MonsterView{
		Self:   "goblin",
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Seen:   []encounter.SeenMember{{ID: "alice", Kind: encounter.KindPlayer, Standing: false}},
	})
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

func (s *BasicTestSuite) TestAttacksTheClosestStandingPlayerWhenInReach() {
	intent, err := (behavior.Basic{}).Act(encounter.MonsterView{
		Self:    "goblin",
		Actions: []encounter.ActionView{{Ref: meleeRef, Name: "Claw", RangeFeet: 5}},
		Budget:  encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Seen: []encounter.SeenMember{
			{ID: "alice", Kind: encounter.KindPlayer, Standing: true, DistanceCells: 1, InReach: map[core.Ref]bool{meleeRef: true}},
			{ID: "bob", Kind: encounter.KindPlayer, Standing: true, DistanceCells: 3, InReach: map[core.Ref]bool{meleeRef: false}},
		},
	})
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: "alice", Action: meleeRef}, intent)
}

func (s *BasicTestSuite) TestPicksTheClosestOfTwoEquallyReachableTargetsDeterministically() {
	view := encounter.MonsterView{
		Self:    "goblin",
		Actions: []encounter.ActionView{{Ref: meleeRef, Name: "Claw", RangeFeet: 5}},
		Budget:  encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Seen: []encounter.SeenMember{
			{ID: "bob", Kind: encounter.KindPlayer, Standing: true, DistanceCells: 1, InReach: map[core.Ref]bool{meleeRef: true}},
			{ID: "alice", Kind: encounter.KindPlayer, Standing: true, DistanceCells: 1, InReach: map[core.Ref]bool{meleeRef: true}},
		},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	first := intent

	// Run it again with Seen reversed — same distances, same tie — the
	// answer must not depend on slice order (C8-style determinism).
	view.Seen[0], view.Seen[1] = view.Seen[1], view.Seen[0]
	intent, err = (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(first, intent, "the same tie must resolve the same way regardless of Seen's order")
}

func (s *BasicTestSuite) TestNoAttacksLeftFallsThroughToMovement() {
	intent, err := (behavior.Basic{}).Act(encounter.MonsterView{
		Self:    "goblin",
		Actions: []encounter.ActionView{{Ref: meleeRef, Name: "Claw", RangeFeet: 5}},
		Budget:  encounter.TurnBudget{AttacksLeft: 0, MovementFeet: 30},
		Seen: []encounter.SeenMember{
			{
				ID: "alice", Kind: encounter.KindPlayer, Standing: true,
				DistanceCells: 3, InReach: map[core.Ref]bool{meleeRef: false},
				Path: []spatial.Position{{X: 1, Y: 0}, {X: 2, Y: 0}},
			},
		},
	})
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 1, Y: 0}}}, intent, "one step only, not the whole path")
}

func (s *BasicTestSuite) TestOutOfReachWithNoMovementLeftPasses() {
	intent, err := (behavior.Basic{}).Act(encounter.MonsterView{
		Self:    "goblin",
		Actions: []encounter.ActionView{{Ref: meleeRef, Name: "Claw", RangeFeet: 5}},
		Budget:  encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 0},
		Seen: []encounter.SeenMember{
			{
				ID: "alice", Kind: encounter.KindPlayer, Standing: true,
				DistanceCells: 3, InReach: map[core.Ref]bool{meleeRef: false},
				Path: []spatial.Position{{X: 1, Y: 0}},
			},
		},
	})
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

func (s *BasicTestSuite) TestOutOfReachWithNoPathAvailablePasses() {
	intent, err := (behavior.Basic{}).Act(encounter.MonsterView{
		Self:    "goblin",
		Actions: []encounter.ActionView{{Ref: meleeRef, Name: "Claw", RangeFeet: 5}},
		Budget:  encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Seen: []encounter.SeenMember{
			{
				ID: "alice", Kind: encounter.KindPlayer, Standing: true,
				DistanceCells: 3, InReach: map[core.Ref]bool{meleeRef: false},
				Path: nil, // sighted but unreachable — see SeenMember.Path's own doc
			},
		},
	})
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

func (s *BasicTestSuite) TestPrefersSeenOverRemembered() {
	view := encounter.MonsterView{
		Seen: []encounter.SeenMember{{
			ID: "david", Kind: encounter.KindPlayer, Standing: true,
			DistanceCells: 1, Path: []spatial.Position{{X: 1}},
		}},
		Remembered: []encounter.RememberedMember{{
			ID: "billy", Kind: encounter.KindPlayer, DistanceCells: 1,
			Path: []spatial.Position{{X: -1}},
		}},
		Budget: encounter.TurnBudget{MovementFeet: 30},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 1}}}, intent)
}

func (s *BasicTestSuite) TestChoosesClosestReachableRemembered() {
	view := encounter.MonsterView{
		Remembered: []encounter.RememberedMember{
			{ID: "alice", Kind: encounter.KindPlayer, DistanceCells: 1},
			{ID: "billy", Kind: encounter.KindPlayer, DistanceCells: 2, Path: []spatial.Position{{X: 1}}},
		},
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 1}}}, intent)
}

// TestIgnoresCloserReachableRememberedNonPlayer catches a remembered-target
// selector that pursues a monster instead of the closest remembered player.
func (s *BasicTestSuite) TestIgnoresCloserReachableRememberedNonPlayer() {
	view := encounter.MonsterView{
		Remembered: []encounter.RememberedMember{
			{ID: "wolf", Kind: encounter.KindMonster, DistanceCells: 1, Path: []spatial.Position{{X: -1}}},
			{ID: "alice", Kind: encounter.KindPlayer, DistanceCells: 2, Path: []spatial.Position{{X: 1}}},
		},
		Budget: encounter.TurnBudget{MovementFeet: 30},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 1}}}, intent)
}

// TestPassesWithOnlyRememberedNonPlayers catches a remembered-target
// selector that treats non-player knowledge as actionable pursuit.
func (s *BasicTestSuite) TestPassesWithOnlyRememberedNonPlayers() {
	view := encounter.MonsterView{
		Remembered: []encounter.RememberedMember{{
			ID: "wolf", Kind: encounter.KindMonster, DistanceCells: 1,
			Path: []spatial.Position{{X: -1}},
		}},
		Budget: encounter.TurnBudget{MovementFeet: 30},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

// TestRememberedMoveUsesOnlyTheFirstCell catches a remembered pursuit that
// consumes an entire route in one turn instead of one movement step.
func (s *BasicTestSuite) TestRememberedMoveUsesOnlyTheFirstCell() {
	view := encounter.MonsterView{
		Remembered: []encounter.RememberedMember{{
			ID: "alice", Kind: encounter.KindPlayer, DistanceCells: 3,
			Path: []spatial.Position{{X: 1}, {X: 2}, {X: 3}},
		}},
		Budget: encounter.TurnBudget{MovementFeet: 30},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 1}}}, intent)
}

func (s *BasicTestSuite) TestRememberedTieBreaksByIDRegardlessOfSliceOrder() {
	view := encounter.MonsterView{
		Remembered: []encounter.RememberedMember{
			{ID: "bob", Kind: encounter.KindPlayer, DistanceCells: 2, Path: []spatial.Position{{X: 2}}},
			{ID: "alice", Kind: encounter.KindPlayer, DistanceCells: 2, Path: []spatial.Position{{X: 1}}},
		},
		Budget: encounter.TurnBudget{MovementFeet: 30},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 1}}}, intent)

	view.Remembered[0], view.Remembered[1] = view.Remembered[1], view.Remembered[0]
	intent, err = (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 1}}}, intent)
}

func (s *BasicTestSuite) TestNeverAttacksRememberedTarget() {
	view := encounter.MonsterView{
		Actions: []encounter.ActionView{{Ref: meleeRef, Name: "Claw", RangeFeet: 5}},
		Remembered: []encounter.RememberedMember{{
			ID: "alice", Kind: encounter.KindPlayer, DistanceCells: 1,
			Path: []spatial.Position{{X: 1}},
		}},
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 1}}}, intent)
}

func (s *BasicTestSuite) TestVisibleStandingTargetOwnsDecisionWhenNonActionable() {
	view := encounter.MonsterView{
		Seen: []encounter.SeenMember{{
			ID: "david", Kind: encounter.KindPlayer, Standing: true,
			DistanceCells: 1,
		}},
		Remembered: []encounter.RememberedMember{{
			ID: "billy", Kind: encounter.KindPlayer, DistanceCells: 2,
			Path: []spatial.Position{{X: -1}},
		}},
		Budget: encounter.TurnBudget{MovementFeet: 30},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

func (s *BasicTestSuite) TestRememberedWithNoMovementPasses() {
	view := encounter.MonsterView{
		Remembered: []encounter.RememberedMember{{
			ID: "alice", Kind: encounter.KindPlayer, DistanceCells: 1,
			Path: []spatial.Position{{X: 1}},
		}},
		Budget: encounter.TurnBudget{MovementFeet: 0},
	}
	intent, err := (behavior.Basic{}).Act(view)
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

func (s *BasicTestSuite) TestNoKnowledgePasses() {
	intent, err := (behavior.Basic{}).Act(encounter.MonsterView{
		Budget: encounter.TurnBudget{MovementFeet: 30},
	})
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

// TestInReachWithNoAttacksLeftPassesInsteadOfClosingFurther pins that a
// target already within reach is not something to walk toward further,
// even with movement left and the attack already spent — doing so would
// spend movement for no gain (Path would already stop adjacent rather than
// walk onto the target — see its own doc — but there is still no reason to
// use it here).
func (s *BasicTestSuite) TestInReachWithNoAttacksLeftPassesInsteadOfClosingFurther() {
	intent, err := (behavior.Basic{}).Act(encounter.MonsterView{
		Self:    "goblin",
		Actions: []encounter.ActionView{{Ref: meleeRef, Name: "Claw", RangeFeet: 5}},
		Budget:  encounter.TurnBudget{AttacksLeft: 0, MovementFeet: 30},
		Seen: []encounter.SeenMember{
			{
				ID: "alice", Kind: encounter.KindPlayer, Standing: true,
				DistanceCells: 1, InReach: map[core.Ref]bool{meleeRef: true},
				Path: nil, // already adjacent: Path is empty by SeenMember's own contract
			},
		},
	})
	s.Require().NoError(err)
	s.Equal(encounter.Pass{}, intent)
}

// TestBasicEndToEndAgainstARealEncounter proves Basic actually drives a
// monster's whole turn through the real composition — not just its own
// decision function — closing the distance one step at a time and then
// attacking once in reach, exactly the tomb fixture's own shape (a–b in the
// brief's gate tests, verified again at the session level later).
func (s *BasicTestSuite) TestBasicEndToEndAgainstARealEncounter() {
	striker := &recordingStriker{kind: encounter.OutcomeMissed}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: behavior.Basic{}, Striker: striker, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{behaviorTestRegion("room-1", 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
			{
				ID: "goblin", Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 0},
				SpeedFeet: 30, Targeting: "closest",
				Actions: []encounter.ActionView{{Ref: meleeRef, Name: "Claw", RangeFeet: 5, Kind: "melee"}},
			},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	et, err := enc.EndTurn(&encounter.EndTurnInput{Member: "alice"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("alice"), et.Next, "the goblin's whole turn resolves back to alice")

	s.Require().Len(striker.calls, 1, "closing to 1 cell (within 5-foot/1-cell reach) affords the attack in the same turn")
	s.Equal(core.EntityID("goblin"), striker.calls[0].attacker)
	s.Equal(core.EntityID("alice"), striker.calls[0].target)

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == "goblin" {
			s.Equal(spatial.Position{X: 1, Y: 0}, m.Position, "closed to adjacent, not all the way onto alice's cell")
		}
	}
}

// --- shared fixtures (mirroring the encounter package's own test style) ---

func behaviorTestRegion(id string, width, height int) encounter.RegionInput {
	cells := make([]spatial.Position, 0, width*height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			cells = append(cells, spatial.Position{X: float64(x), Y: float64(y)})
		}
	}
	return encounter.RegionInput{
		ID: id, Cells: cells, Archetype: "dungeon",
		Lighting: &encounter.Lighting{Intensity: 1},
	}
}

type everyoneSeesTheWholeMap struct{}

func (everyoneSeesTheWholeMap) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 1_000_000
	}
	return out, nil
}

type everyoneStanding struct{}

func (everyoneStanding) Standing([]encounter.MemberID) ([]encounter.MemberID, error) { return nil, nil }

type orderAsGiven struct{}

func (orderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}

type quietAnnouncer struct{}

func (quietAnnouncer) Announce(context.Context, *encounter.Encounter, []encounter.Boundary) error {
	return nil
}

type recordingStriker struct {
	kind  encounter.OutcomeKind
	calls []struct{ attacker, target encounter.MemberID }
}

func (r *recordingStriker) Strike(
	_ context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	r.calls = append(r.calls, struct{ attacker, target encounter.MemberID }{attacker, target})
	_, err := enc.Record(&encounter.RecordInput{
		Kind: r.kind, Actor: attacker, Targets: []encounter.MemberID{target},
		Attack: &encounter.AttackIdentity{Ref: action.String(), Name: "Claw", DamageType: "slashing"},
	})
	return err
}
