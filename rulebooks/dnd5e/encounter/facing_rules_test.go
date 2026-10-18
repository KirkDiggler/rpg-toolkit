// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// facing_rules_test.go pins the facing rules the rpg-toolkit#2073 review found
// unpinned (rpg-project#582, sneaking design R13): a Routed walk is the
// walker's own, a driven attack turns before its swing, every turn reads the
// cell the creature HOLDS for its target, and a facing persists — on a
// reserve too — and is refused at load when it is not a bearing.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// pointyStepBearings is the six pointy-top step directions as bearings
// (facing_test.go's table, read from the outside), keyed by the axial step.
var pointyStepBearings = map[spatial.Position]float64{
	{X: 1, Y: 0}: 90, {X: 1, Y: -1}: 30, {X: 0, Y: -1}: 330,
	{X: -1, Y: 0}: 270, {X: -1, Y: 1}: 210, {X: 0, Y: 1}: 150,
}

// stepBearing is the bearing of one step between two adjacent cells.
func stepBearing(t *testing.T, from, to spatial.Position) float64 {
	t.Helper()
	degrees, ok := pointyStepBearings[spatial.Position{X: to.X - from.X, Y: to.Y - from.Y}]
	require.True(t, ok, "%v and %v are adjacent", from, to)

	return degrees
}

// facingRoutedCause is what sends a creature on a Routed walk in these scenes.
var facingRoutedCause = core.Ref{Module: "test", Type: "spells", ID: "command"}

// facingFight is a one-room fight on a pointy-top yard: every MemberInput is
// placed at its authored offset cell, everyone sees the whole map, the
// goblin walks thirty feet and swings at five.
func facingFight(
	t *testing.T, members []encounter.MemberInput, driver encounter.Driver,
	striker encounter.Striker, mover encounter.Mover,
) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members:   members,
		Endings:   []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        adjacentSkeletonSheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        driver,
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

// memberOf is one roster row.
func memberOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) encounter.Member {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if m.ID == id {
			return m
		}
	}
	require.FailNowf(t, "not on the roster", "%s", id)

	return encounter.Member{}
}

// heldFacingOf is the facing one member's own sighting of another carries.
func heldFacingOf(t *testing.T, enc *encounter.Encounter, observer, subject encounter.MemberID) *float64 {
	t.Helper()
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	require.NoError(t, err)
	for _, h := range holdings {
		if h.Channel == perception.Sight && encounter.MemberID(h.Subject) == subject {
			seen, ok := encounter.DecodeSightTestimony(h.Payload)
			require.True(t, ok)

			return seen.Facing
		}
	}
	require.FailNowf(t, "no sighting", "%s holds no sighting of %s", observer, subject)

	return nil
}

// TestARoutedWalkIsTheWalkersOwn pins both halves of routedFace. Toward a
// member, the goblin walks its own legs and ends facing alice, not along its
// last step; away from her, there is nobody to face and it faces along the
// last step it ran. Neither walk is imposed: it turns along every step.
//
// bob stands on the cell an open approach would end on, so the goblin comes
// in from the side: its last step is 210 and alice is due west, 270.
func TestARoutedWalkIsTheWalkersOwn(t *testing.T) {
	members := []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 0}},
		{ID: bob, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}, BlocksMovement: true},
	}

	t.Run("toward ends facing the anchor", func(t *testing.T) {
		mover := &recordingMover{}
		enc := facingFight(t, members, &oneMemberDriver{member: goblin, intent: encounter.Routed{
			Policy: encounter.MoveToward, Anchor: alice, Cause: facingRoutedCause,
		}}, &scriptedStriker{kind: encounter.OutcomeMissed}, mover)
		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
		require.NoError(t, err)

		require.NotEmpty(t, mover.calls, "the goblin walked")
		last := mover.calls[len(mover.calls)-1]
		end := memberOf(t, enc, goblin).Position
		toAlice := stepBearing(t, end, memberOf(t, enc, alice).Position)
		require.NotEqual(t, stepBearing(t, last.From, last.To), toAlice,
			"the scene's last step does not already point at alice")
		require.InDelta(t, toAlice, memberOf(t, enc, goblin).Facing, 1e-9, "the walk ended facing its anchor")
	})

	t.Run("away faces along its last step", func(t *testing.T) {
		mover := &recordingMover{}
		enc := facingFight(t, members, &oneMemberDriver{member: goblin, intent: encounter.Routed{
			Policy: encounter.MoveAway, Anchor: alice, Cause: facingRoutedCause,
		}}, &scriptedStriker{kind: encounter.OutcomeMissed}, mover)
		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
		require.NoError(t, err)

		require.NotEmpty(t, mover.calls, "the goblin ran")
		last := mover.calls[len(mover.calls)-1]
		require.InDelta(t, stepBearing(t, last.From, last.To), memberOf(t, enc, goblin).Facing, 1e-9,
			"a flight does not end turned toward what it fled")
	})
}

// facingAtStrike is a Striker that records nothing: it notes the attacker's
// facing at the instant of the swing, so the driven arm's own turn is the one
// observed — no RecordTrain runs behind it to turn the attacker again.
type facingAtStrike struct {
	at    float64
	calls int
}

func (f *facingAtStrike) Strike(_ context.Context, enc *encounter.Encounter, attacker, _ encounter.MemberID, _ core.Ref) error {
	f.calls++
	members, err := enc.Members()
	if err != nil {
		return err
	}
	for _, m := range members {
		if m.ID == attacker {
			f.at = m.Facing
		}
	}

	return nil
}

// TestADrivenAttackTurnsBeforeTheSwing pins the driven Attack arm's turn. The
// goblin stands east of alice facing the unstated 180; its attack turns it to
// 270 before the striker is asked. The striker records no train, so nothing
// refreshes behind the swing; the next look — alice's own step — puts the
// turn on her sighting of the goblin.
func TestADrivenAttackTurnsBeforeTheSwing(t *testing.T) {
	striker := &facingAtStrike{}
	enc := facingFight(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
	}, &oneMemberDriver{member: goblin, intent: encounter.Attack{Target: alice, Action: testMeleeAction}}, striker, quietMover{})
	require.Equal(t, encounter.UnstatedFacing, memberOf(t, enc, goblin).Facing)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.Equal(t, 1, striker.calls, "the goblin swung")
	require.InDelta(t, 270.0, striker.at, 1e-9, "the attacker turned toward alice before the swing")

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: spatial.Position{X: 0, Y: 2}})
	require.NoError(t, err)
	seen := heldFacingOf(t, enc, alice, goblin)
	require.NotNil(t, seen)
	require.InDelta(t, 270.0, *seen, 1e-9, "a watcher sees the turn at its next look")
}

// TestARecordedAttackTurnsTowardTheHeldCell is the lead's one-source ruling
// on rpg-toolkit#2073: a recorded attack turns its attacker toward the cell
// it HOLDS for its target, never the live one. The goblin's sighting of alice
// is rewritten to a memory of her old cell while she really stands far off;
// the record turns it toward the memory. An Unknown sighting places nobody,
// so the attacker does not turn at all.
func TestARecordedAttackTurnsTowardTheHeldCell(t *testing.T) {
	base := facingFight(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
	}, passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietMover{})
	goblinCell := memberOf(t, base, goblin).Position
	oldCell := memberOf(t, base, alice).Position

	load := func(payload []byte) *encounter.Encounter {
		data := base.ToData()
		for i := range data.Members {
			if data.Members[i].ID == alice {
				far := cellAt(7, 7)
				data.Members[i].Cell = &encounter.PositionData{X: far.X, Y: far.Y}
			}
		}
		editGoblinHolding(t, data, alice, payload)
		loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: data,
			Capabilities: encounter.Capabilities{
				Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{},
				Conditions: encounter.UnobservedEquipment{}, Sheets: adjacentSkeletonSheets,
				Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{},
				Actors: encounter.Actors{
					Striker: &scriptedStriker{kind: encounter.OutcomeMissed}, Mover: quietMover{}, Announcer: quietAnnouncer{},
				},
			},
		})
		require.NoError(t, err)

		return loaded
	}

	t.Run("a memory is where it turns", func(t *testing.T) {
		memory, err := encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationKnown, Position: oldCell})
		require.NoError(t, err)
		enc := load(memory)
		_, err = recordOne(enc, &encounter.RecordInput{
			Kind: encounter.OutcomeMissed, Actor: goblin, Targets: []encounter.MemberID{alice},
		})
		require.NoError(t, err)
		require.InDelta(t, stepBearing(t, goblinCell, oldCell), memberOf(t, enc, goblin).Facing, 1e-9,
			"the goblin turned toward where it holds alice, not where she stands")
	})

	t.Run("an unknown sighting turns nobody", func(t *testing.T) {
		unknown, err := encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationUnknown})
		require.NoError(t, err)
		enc := load(unknown)
		_, err = recordOne(enc, &encounter.RecordInput{
			Kind: encounter.OutcomeMissed, Actor: goblin, Targets: []encounter.MemberID{alice},
		})
		require.NoError(t, err)
		require.Equal(t, encounter.UnstatedFacing, memberOf(t, enc, goblin).Facing,
			"an Unknown sighting places nobody to turn toward")
	})
}

// TestAReservedFacingPersists pins the reserve's facing key: a waiting member
// stated facing 90 keeps it through a save and a load, and a second save
// writes it back.
func TestAReservedFacingPersists(t *testing.T) {
	east := 90.0
	enc := facingFight(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{
			ID: "late", Kind: encounter.KindMonster, Position: spatial.Position{X: 8, Y: 8}, Facing: &east,
			Arrives: encounter.TriggerRound{Round: 9},
		},
	}, passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietMover{})

	reserveFacing := func(data encounter.EncounterData) *float64 {
		for _, r := range data.Reserve {
			if r.ID == "late" {
				return r.Facing
			}
		}
		require.FailNow(t, "late is not in reserve")

		return nil
	}
	saved := enc.ToData()
	require.NotNil(t, reserveFacing(saved))
	require.Equal(t, east, *reserveFacing(saved))

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: saved,
		Capabilities: encounter.Capabilities{
			Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{},
			Conditions: encounter.UnobservedEquipment{}, Sheets: adjacentSkeletonSheets,
			Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{},
			Actors: encounter.Actors{
				Striker: &scriptedStriker{kind: encounter.OutcomeMissed}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)
	again := reserveFacing(loaded.ToData())
	require.NotNil(t, again)
	require.Equal(t, east, *again, "the reloaded reserve still faces 90")
}

// TestAStoredFacingThatIsNotABearingIsRefused pins the load door: a member
// row whose facing is outside [0, 360) is a blob no build writes.
func TestAStoredFacingThatIsNotABearingIsRefused(t *testing.T) {
	enc := facingFight(t, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
	}, passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed}, quietMover{})
	data := enc.ToData()
	bad := 400.0
	data.Members[0].Facing = &bad

	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{},
			Conditions: encounter.UnobservedEquipment{}, Sheets: adjacentSkeletonSheets,
			Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{},
			Actors: encounter.Actors{
				Striker: &scriptedStriker{kind: encounter.OutcomeMissed}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			},
		},
	})
	require.ErrorIs(t, err, encounter.ErrInvalidData)
	require.ErrorIs(t, err, encounter.ErrBadFacing)
}

// oneMemberDriver drives one member with one intent, once, and passes every
// other turn — so a second monster in a scene stands where it was put.
type oneMemberDriver struct {
	member encounter.MemberID
	intent encounter.TurnIntent
	used   bool
}

func (d *oneMemberDriver) Act(view encounter.MonsterView) (encounter.Decision, error) {
	if view.Self != d.member || d.used {
		return encounter.Decision{Intent: encounter.Pass{}}, nil
	}
	d.used = true

	return encounter.Decision{Intent: d.intent}, nil
}
