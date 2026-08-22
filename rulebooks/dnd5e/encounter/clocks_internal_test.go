// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestClockOfReportsAMemberOnNoClockInsteadOfGuessing pins ClockOf's
// on-no-clock check — the net under the clock verbs' non-atomic mutate
// phases (doc.go). It was added in step 4.1 explicitly unreachable, with a
// promise that "a test lands with the verb that can reach it"; this is that
// test, and it is deliberately WHITE-BOX: every public verb upholds R6, and
// the load seam re-homes anyone it finds on no clock, so the only way to
// probe the net is to fabricate the exact defect it exists to catch — a verb
// that dropped somebody from a bubble and forgot to re-home them (the
// mid-Dissolve failure window, frozen).
//
// What makes the check worth a test at all is the SHAPE of the wrong answer
// it prevents: without it, a member on no clock reports ClockWorld — "free
// roaming" — which is plausible, silent, and would be persisted by the next
// save. With it, the defect is loud: ErrInvalidData, never a guess.
func TestClockOfReportsAMemberOnNoClockInsteadOfGuessing(t *testing.T) {
	enc, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: FieldInput{
			Canvas: CanvasInput{Void: VoidIsOpaque()},
			Rooms:  []RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Room: "room-1", Position: spatial.Position{X: 2, Y: 2}},
			{ID: "goblin", Kind: KindMonster, Room: "room-1", Position: spatial.Position{X: 7, Y: 7}},
		},
		Endings: []EndingInput{
			{Key: "called", Trigger: TriggerExternal{}},
		},
	})
	require.NoError(t, err)

	// They saw each other at first light, so the fight already exists — no
	// caller-driven form needed, and none available (rpg-toolkit#964).
	require.Len(t, enc.bubbles, 1, "first light started the fight")

	// The bug, fabricated: alice leaves the fight and nobody re-homes her.
	// No public verb can do this — that is the point of the net.
	_, err = enc.bubbles[0].Remove(&clock.RemoveInput{ID: "alice"})
	require.NoError(t, err)

	_, err = enc.ClockOf(&ClockOfInput{Member: "alice"})
	require.Error(t, err, "a member on no clock must be a loud defect, not a free-roamer")
	require.ErrorIs(t, err, ErrInvalidData)
	require.ErrorContains(t, err, "on no clock")

	// The member still IN the fight is unaffected — the defect is reported
	// exactly where it is, not smeared across the encounter.
	out, err := enc.ClockOf(&ClockOfInput{Member: "goblin"})
	require.NoError(t, err)
	require.Equal(t, ClockTurn, out.Kind)
}

// TestFormRejections pins form's refusals. It lives in the internal package
// because form is unexported as of rpg-toolkit#964: trigger detection is the
// only thing that starts a fight, so the verb's validation is reachable only
// from inside. The refusals still matter — trigger detection builds the input
// and a bad one must not half-start a fight.
func TestFormRejections(t *testing.T) {
	newEnc := func() *Encounter {
		enc, err := NewEncounter(&SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
			Field: FieldInput{Canvas: CanvasInput{Void: VoidIsOpaque()}, Rooms: []RoomInput{
				{ID: "r1", Width: 8, Height: 8, Boundaries: sealedSeam(7, 8)},
				{ID: "r2", Width: 8, Height: 8, Origin: spatial.Position{X: 8, Y: 0}},
			}},
			Members: []MemberInput{
				{ID: "alice", Kind: KindPlayer, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
				{ID: "bob", Kind: KindPlayer, Room: "r1", Position: spatial.Position{X: 2, Y: 1}},
				{ID: "carl", Kind: KindPlayer, Room: "r1", Position: spatial.Position{X: 3, Y: 1}},
				{ID: "dana", Kind: KindPlayer, Room: "r1", Position: spatial.Position{X: 4, Y: 1}},
				// Next door, so first light does not start the fight these
				// cases are about starting badly.
				{ID: "goblin", Kind: KindMonster, Room: "r2", Position: spatial.Position{X: 6, Y: 6}},
			},
			Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		})
		require.NoError(t, err)
		return enc
	}

	t.Run("an empty order", func(t *testing.T) {
		_, err := newEnc().form(&FormInput{Order: nil})
		require.ErrorIs(t, err, ErrNoMember)
	})

	t.Run("a duplicated entry", func(t *testing.T) {
		_, err := newEnc().form(&FormInput{Order: []MemberID{"alice", "goblin", "alice"}})
		require.ErrorIs(t, err, ErrNoMember)
	})

	t.Run("a non-member in the order", func(t *testing.T) {
		_, err := newEnc().form(&FormInput{Order: []MemberID{"alice", "stranger"}})
		require.ErrorIs(t, err, ErrNotMember)
	})

	t.Run("surprised names somebody outside the order", func(t *testing.T) {
		_, err := newEnc().form(&FormInput{
			Order:     []MemberID{"alice", "goblin"},
			Surprised: []MemberID{"bob"},
		})
		require.ErrorIs(t, err, ErrNotMember)
	})

	t.Run("a member who is already fighting", func(t *testing.T) {
		enc := newEnc()
		_, err := enc.form(&FormInput{Order: []MemberID{"alice", "goblin"}})
		require.NoError(t, err)

		_, err = enc.form(&FormInput{Order: []MemberID{"alice", "bob"}})
		require.ErrorIs(t, err, ErrInBubble)
	})

	t.Run("a disjoint second fight while one runs", func(t *testing.T) {
		enc := newEnc()
		_, err := enc.form(&FormInput{Order: []MemberID{"alice", "goblin"}})
		require.NoError(t, err)

		_, err = enc.form(&FormInput{Order: []MemberID{"carl", "dana"}})
		require.ErrorIs(t, err, ErrInBubble)
	})

	t.Run("a rejected form touches nothing", func(t *testing.T) {
		enc := newEnc()
		_, err := enc.form(&FormInput{Order: []MemberID{"alice", "goblin", "stranger"}})
		require.Error(t, err)

		// alice and the goblin were named BEFORE the defect: R5 means they
		// were never pulled off the world clock.
		for _, id := range []MemberID{"alice", "goblin"} {
			out, cerr := enc.ClockOf(&ClockOfInput{Member: id})
			require.NoError(t, cerr)
			require.Equal(t, ClockWorld, out.Kind, "member %q", id)
		}
		require.Empty(t, enc.ToData().Bubbles)
	})
}

// sealedSeam is the wall a room draws along its own +X edge, with no opening:
// every crossing to the column beyond it, diagonals included.
//
// It exists because "next door" stopped being a hiding place. The field is one
// canvas now (rpg-toolkit#1106), so two chambers side by side share an open
// seam unless somebody says otherwise, and a fixture whose whole premise is
// "first light does not start the fight" has to say it. Diagonals are included
// for the reason testwalls_test.go's squareSeamWall spells out: a square cell
// has eight neighbours, and a wall of same-row edges leaks at every corner.
func sealedSeam(atX, height int) []spatial.Boundary {
	out := make([]spatial.Boundary, 0, height*3)
	for y := 0; y < height; y++ {
		for _, dy := range []int{-1, 0, 1} {
			to := y + dy
			if to < 0 || to >= height {
				continue
			}
			out = append(out, spatial.Boundary{
				From:              spatial.Position{X: float64(atX), Y: float64(y)},
				To:                spatial.Position{X: float64(atX + 1), Y: float64(to)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			})
		}
	}
	return out
}

// TestFormRefusesAPlayerFreeBubble pins driveMonsterTurns's defensive check
// via form: a fight with no player in it at all has nobody to ever hand the
// clock back to, and rpg-toolkit#1162 refuses loudly (ErrNoPlayerInBubble)
// rather than looping forever or silently ending every member's turn.
//
// UNREACHABLE THROUGH TRIGGER DETECTION in the running game — a bubble only
// forms on contact between a player and something else — so this drives
// form directly, the same white-box reason TestFormRejections does.
func TestFormRefusesAPlayerFreeBubble(t *testing.T) {
	enc, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: FieldInput{Canvas: CanvasInput{Void: VoidIsOpaque()}, Rooms: []RoomInput{
			{ID: "r1", Width: 8, Height: 8},
		}},
		Members: []MemberInput{
			{ID: "goblin", Kind: KindMonster, Room: "r1", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "wolf", Kind: KindMonster, Room: "r1", Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	_, err = enc.form(&FormInput{Order: []MemberID{"goblin", "wolf"}})
	require.ErrorIs(t, err, ErrNoPlayerInBubble)
}

// rogueTurnIntent is a TurnIntent this package's own switch cannot possibly
// have a case for — constructible only from inside this package, since
// TurnIntent is sealed on an unexported method (ADR-0038's practice for
// sealed vocabularies at this seam). It exists to reach ErrBadTurnOutcome's
// one white-box path, the same reason rogueTurnOutcome-style fixtures exist
// beside every other sealed vocabulary in this module.
type rogueTurnIntent struct{}

func (rogueTurnIntent) isTurnIntent() {}

// rogueDriver always returns the sealed vocabulary's own defect.
type rogueDriver struct{}

func (rogueDriver) Act(MonsterView) (TurnIntent, error) {
	return rogueTurnIntent{}, nil
}

// TestADriverReturningAnUnrecognisedIntentIsErrBadTurnOutcome pins the
// sealed vocabulary's own defensive net: TurnIntent is sealed so nothing
// OUTSIDE this package can construct a fourth case, but a value satisfying
// the interface from INSIDE it (a future case added here without every call
// site catching up) must fail loudly rather than silently fall through
// driveMonsterTurns' switch.
func TestADriverReturningAnUnrecognisedIntentIsErrBadTurnOutcome(t *testing.T) {
	enc, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: rogueDriver{}, Striker: passStriker{},
		Field: FieldInput{
			Canvas: CanvasInput{Void: VoidIsOpaque()},
			Rooms:  []RoomInput{{ID: "room-1", Width: 8, Height: 8}},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "goblin", Kind: KindMonster, Room: "room-1", Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	_, err = enc.EndTurn(&EndTurnInput{Member: "alice"})
	require.ErrorIs(t, err, ErrBadTurnOutcome)
}
