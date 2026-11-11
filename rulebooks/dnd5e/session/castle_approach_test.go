// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_approach_test.go is click a hostile to attack's server half
// (rpg-project#872 D4) on the QA castle: an out-of-reach Attack candidate
// names the cell a walk this turn ends on in reach of him, and THE LAW is
// Afford → Move to that cell → Attack succeeds. In free roam the walk has no
// budget; in a fight it is the movement the turn has left, and a guard
// beyond it gets no cell.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// affordAttack is alice's main Attack row and her Move row's id (empty on
// the world clock).
func (g *gatehouse) affordAttack(t *testing.T) (session.Declaration, string) {
	t.Helper()
	out, err := g.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	var attack *session.Declaration
	moveID := ""
	for i, d := range out.Declarations {
		if d.Verb == session.VerbAttack && attack == nil {
			attack = &out.Declarations[i]
		}
		if d.Verb == session.VerbMove {
			moveID = d.ID
		}
	}
	require.NotNil(t, attack, "alice is offered an Attack row")
	return *attack, moveID
}

func candidateOf(t *testing.T, d session.Declaration, member string) session.TargetCandidate {
	t.Helper()
	for _, c := range d.Candidates {
		if c.Member == member {
			return c
		}
	}
	t.Fatalf("%s is not an Attack candidate", member)
	return session.TargetCandidate{}
}

// walkTo is the shortest walk alice's own walk takes to cell: the path the
// encounter's answer carries, which a client's own shortest walk matches in
// length.
func (g *gatehouse) walkTo(t *testing.T, target string, rangeFeet int, cell spatial.Position) []spatial.Position {
	t.Helper()
	enc, err := g.mgr.LoadWorldForTest(context.Background(), "sess")
	require.NoError(t, err)
	out, err := enc.Approaches(encounter.ApproachesInput{
		Mover: "alice", Targets: []encounter.MemberID{encounter.MemberID(target)}, RangeFeet: rangeFeet, Unbounded: true,
	})
	require.NoError(t, err)
	require.True(t, out.Approaches[0].Reached)
	require.Equal(t, cell, out.Approaches[0].Cell, "the offer names the encounter's cell")
	return out.Approaches[0].Path
}

// approachAndSwing is the law for one target: the offer names a cell, the
// Move there is accepted whole, the Attack is offered from it, and it lands
// as an ordinary attack.
func (g *gatehouse) approachAndSwing(t *testing.T, target string, rangeFeet int) {
	t.Helper()
	ctx := context.Background()
	attack, moveID := g.affordAttack(t)
	c := candidateOf(t, attack, target)
	require.False(t, c.Available, "%s starts out of reach", target)
	require.NotNil(t, c.Why)
	require.Equal(t, session.ShortfallTargetOutOfReach, c.Why.Reason)
	require.NotNil(t, c.Approach, "a walk this turn reaches %s", target)

	path := g.walkTo(t, target, rangeFeet, *c.Approach)
	moved, err := walkOn(ctx, g.mgr, &session.MoveInput{Session: "sess", Member: "alice", DeclarationID: moveID, Path: path})
	require.NoError(t, err, "the Move to the offered cell is not refused")
	require.Equal(t, session.MovementCompleted, moved.Status)
	require.Equal(t, *c.Approach, moved.Steps[len(moved.Steps)-1].Position)

	attack, _ = g.affordAttack(t)
	c = candidateOf(t, attack, target)
	require.True(t, c.Available, "from the offered cell %s is in reach", target)
	require.Nil(t, c.Approach, "and an in-reach candidate needs no walk")
	require.True(t, attack.Available)
	_, err = g.mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "alice", Target: target, DeclarationID: attack.ID})
	require.NoError(t, err, "the swing at %s lands as an ordinary attack", target)
}

func TestCastleApproach_FreeRoam(t *testing.T) {
	for _, target := range []string{"captain", "gate-1", "gate-2"} {
		t.Run(target, func(t *testing.T) {
			g := openTheGatehouse(14)
			require.NoError(t, g.walkIn())
			g.approachAndSwing(t, target, 5)
		})
	}
}

// inAFightAway starts the fight with a swing at gate-1 from his approach
// cell, then walks alice `away` cells off him on her own turn.
func inAFightAway(t *testing.T, away int) *gatehouse {
	t.Helper()
	ctx := context.Background()
	g := openTheGatehouse(14)
	require.NoError(t, g.walkIn())
	g.approachAndSwing(t, "gate-1", 5)
	enc, err := g.mgr.LoadWorldForTest(ctx, "sess")
	require.NoError(t, err)
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, encounter.ClockTurn, clock.Kind, "precondition: the swing started a fight")
	require.Equal(t, encounter.MemberID("alice"), clock.Active, "precondition: it is alice's turn")
	route, err := enc.Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveAway, Anchor: standingAt(enc, "gate-1"), Budget: away})
	require.NoError(t, err)
	require.Len(t, route.Path, away)
	_, moveID := g.affordAttack(t)
	moved, err := walkOn(ctx, g.mgr, &session.MoveInput{Session: "sess", Member: "alice", DeclarationID: moveID, Path: route.Path})
	require.NoError(t, err)
	require.Equal(t, session.MovementCompleted, moved.Status)
	return g
}

func TestCastleApproach_InAFight(t *testing.T) {
	t.Run("within the movement left", func(t *testing.T) {
		// Fifteen feet left, and gate-1's cell is three steps back.
		g := inAFightAway(t, 3)
		g.approachAndSwing(t, "gate-1", 5)
	})

	t.Run("beyond the movement left", func(t *testing.T) {
		// Ten feet left: two cells. A guard whose walk is longer gets no
		// cell, and one whose walk fits gets its cell inside the budget.
		g := inAFightAway(t, 4)
		ctx := context.Background()
		attack, _ := g.affordAttack(t)
		enc, err := g.mgr.LoadWorldForTest(ctx, "sess")
		require.NoError(t, err)
		beyond := 0
		for _, c := range attack.Candidates {
			if c.Available {
				continue
			}
			free, err := enc.Approaches(encounter.ApproachesInput{
				Mover: "alice", Targets: []encounter.MemberID{encounter.MemberID(c.Member)}, RangeFeet: 5, Unbounded: true,
			})
			require.NoError(t, err)
			walk := free.Approaches[0]
			if c.Approach == nil {
				if walk.Reached {
					require.Greater(t, len(walk.Path), 2, "%s: no cell only when the walk is longer than ten feet", c.Member)
					beyond++
				}
				continue
			}
			require.True(t, walk.Reached)
			require.LessOrEqual(t, len(walk.Path), 2, "%s: a cell only inside the movement left", c.Member)
			require.Equal(t, walk.Cell, *c.Approach)
		}
		require.Positive(t, beyond, "precondition: some guard is beyond the movement left")
		gate := candidateOf(t, attack, "gate-1")
		require.False(t, gate.Available)
		require.Nil(t, gate.Approach, "gate-1's walk back is three steps, past ten feet")
	})
}

func TestCastleApproach_AReachWeaponStopsFarther(t *testing.T) {
	alice := armedFighter("alice")
	alice.Inventory = []character.InventoryItemData{{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Glaive), Quantity: 1}}
	alice.EquipmentSlots = character.EquipmentSlots{character.SlotMainHand: string(weapons.Glaive)}
	g := openTheGatehouseAs(alice, 14)
	require.NoError(t, g.walkIn())

	attack, _ := g.affordAttack(t)
	c := candidateOf(t, attack, "gate-1")
	require.NotNil(t, c.Approach)
	enc, err := g.mgr.LoadWorldForTest(context.Background(), "sess")
	require.NoError(t, err)
	require.Equal(t, float64(2), enc.Distance(*c.Approach, standingAt(enc, "gate-1")),
		"a glaive's ten feet stop two cells off, not beside him")
	g.approachAndSwing(t, "gate-1", 10)
}
