// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_king_custody_test.go is the arrest's custody (rpg-project#925,
// ruled 2026-11-13: "either held and walked off or they can fight, those are
// the only ways out"): from the arrest the taken are held — no walk, no
// dash, no sneak, no cast that carries the caster off — while everyone else
// walks; a fight with a taken member or an arresting guard in it ends
// custody for all the taken; a member who leaves the run stays taken in the
// record; and the phase is kept on reload.

// custodySentence is what a held player is told when they try to walk away.
const custodySentence = "The guards hold you; you cannot walk away."

// takenMembers are the three taken at the castle's arrest.
var takenMembers = []encounter.MemberID{hero, inTheArea, seenOutside}

// custodyTests are the tests that start from [arrestScene], for [TestMain].
var custodyTests = []string{
	"TestMadKingTheTakenAreHeld", "TestMadKingTheFreeWalkWhileTheTakenAreHeld",
	"TestMadKingATakenMembersBlowEndsCustody", "TestMadKingAFriendsBlowEndsCustody",
	"TestMadKingATakenMemberWhoLeavesStaysTaken", "TestMadKingCustodyIsKeptOnReload",
}

// custody is the castle's one arrest's custody phase.
func (s *kingScene) custody() encounter.CustodyPhase {
	s.t.Helper()
	return s.arrested().Custody
}

// stepOff tries one step from where member stands to the free floor cell
// beside them nearest the given direction, and returns the verb's error.
func (s *kingScene) stepOff(member encounter.MemberID) error {
	s.t.Helper()
	from := s.at(member)
	for _, d := range []spatial.Position{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}} {
		to := spatial.Position{X: from.X + d.X, Y: from.Y + d.Y}
		_, err := s.enc.Step(&encounter.StepInput{Member: member, To: to, EndWalk: true})
		if err == nil || errorIsCustody(err) {
			return err
		}
	}
	s.t.Fatalf("%s found no cell to step to from %v", member, from)
	return nil
}

// heldNow reports whether member's next step is refused for custody: a step
// to each neighbour, refused or not for its own reasons (a fight's turn, a
// body in the way) — held only when custody is the refusal.
func (s *kingScene) heldNow(member encounter.MemberID) bool {
	s.t.Helper()
	from := s.at(member)
	_, err := s.enc.Step(&encounter.StepInput{Member: member, To: spatial.Position{X: from.X + 1, Y: from.Y}, EndWalk: true})
	return errorIsCustody(err)
}

func errorIsCustody(err error) bool {
	return err != nil && errors.Is(err, encounter.ErrInCustody)
}

// swingAs records one struck blow of actor's at target, as the session tells
// it ([kingScene.swingAt]'s shape, for any actor).
func (s *kingScene) swingAs(actor, target encounter.MemberID) {
	s.t.Helper()
	in := &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: actor, Targets: []encounter.MemberID{target},
		Attack: &encounter.AttackIdentity{Ref: kingSpear.String(), Name: "Spear", DamageType: "piercing"},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 1},
	}
	_, err := s.enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: in}}})
	require.NoError(s.t, err)
}

// TestMadKingTheTakenAreHeld: each taken member's step — a dash is a step
// on more movement — is refused with the sentence, as is sneaking off and a
// cast that would carry its caster away; nobody moved. Looking stays open.
func TestMadKingTheTakenAreHeld(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	require.Equal(t, encounter.CustodyHeld, s.custody(), "the arrest holds the taken")

	for _, m := range takenMembers {
		at := s.at(m)
		err := s.stepOff(m)
		require.ErrorIs(t, err, encounter.ErrInCustody, "%s cannot walk away", m)
		require.Contains(t, err.Error(), custodySentence, "and is told why")
		require.Equal(t, at, s.at(m), "%s stands where they were taken", m)

		_, err = s.enc.SetSneaking(&encounter.SetSneakingInput{Member: m, Sneaking: true})
		require.ErrorIs(t, err, encounter.ErrInCustody, "%s cannot sneak off", m)
		require.False(t, s.enc.Sneaking(m), "%s is not sneaking: the arrest ended any sneak", m)

		_, err = s.enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: m})
		require.NoError(t, err, "%s still looks about", m)
	}

	beside := s.at(inTheArea)
	_, err := s.enc.RecordCast(&encounter.RecordCastInput{
		Actor: hero, Spell: encounter.SpellIdentity{Ref: "dnd5e:spells:misty-step", Name: "Misty Step"},
		Targets: []encounter.CastTargetResult{{Target: hero}},
		Pushes:  []encounter.CastPush{{Mover: hero, Route: []spatial.Position{{X: beside.X, Y: beside.Y + 1}}}},
	})
	require.ErrorIs(t, err, encounter.ErrInCustody, "a cast that carries its caster off is walking away")
}

// TestMadKingTheFreeWalkWhileTheTakenAreHeld: the one who left (wanted) and
// the one never near (neither) walk as they like while the taken are held,
// and the taken stay held: nothing waits on a held member.
func TestMadKingTheFreeWalkWhileTheTakenAreHeld(t *testing.T) {
	t.Parallel()
	s, before := theArrest(t)
	s.walkMember(leftTheGate, onTheRoadInSight, false)
	require.NotEqual(t, before[leftTheGate], s.at(leftTheGate), "the one who left walks: wanted is not held")
	s.walkMember(neverNear, furtherDownTheRoad, false)
	require.NotEqual(t, before[neverNear], s.at(neverNear), "the one never near walks")
	require.ErrorIs(t, s.stepOff(hero), encounter.ErrInCustody, "and the taken are still held")
	require.Equal(t, encounter.CustodyHeld, s.custody())
}

// TestMadKingATakenMembersBlowEndsCustody: the speaker strikes the captain;
// the fight forms with him and the captain in it, and custody ends for every
// taken member. The record keeps who was taken and who is wanted.
func TestMadKingATakenMembersBlowEndsCustody(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	before := s.arrested()
	s.swingAs(hero, kingCaptain)
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "the blow forms the fight")
	require.Equal(t, encounter.CustodyBroken, s.custody(), "the fight ends custody")

	after := s.arrested()
	require.Equal(t, before.Taken, after.Taken, "the taken are still recorded")
	require.Equal(t, before.Wanted, after.Wanted, "and so are the wanted")
	for _, m := range takenMembers {
		require.False(t, s.heldNow(m), "%s is held no more", m)
	}
}

// TestMadKingAFriendsBlowEndsCustody: the one who left — wanted, never held
// — strikes gate-2, an arresting guard; the fight that forms ends custody
// for the taken, who can walk again.
func TestMadKingAFriendsBlowEndsCustody(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	s.swingAs(leftTheGate, kingGate2)
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, kingGate2), "the blow forms the fight with the guard in it")
	require.Equal(t, encounter.CustodyBroken, s.custody(), "a fight with an arresting guard in it ends custody")
	for _, m := range takenMembers {
		require.False(t, s.heldNow(m), "%s is held no more", m)
	}
}

// TestMadKingATakenMemberWhoLeavesStaysTaken: a taken player may leave the
// run (a closed tab is not refused); the record keeps them taken and the
// rest held.
func TestMadKingATakenMemberWhoLeavesStaysTaken(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	_, err := s.enc.Exit(&encounter.ExitInput{Member: seenOutside})
	require.NoError(t, err, "a taken member may leave the run")
	require.Contains(t, takenOf(s.arrested()), seenOutside, "and stays taken in the record")
	require.Equal(t, encounter.CustodyHeld, s.custody(), "the rest are still held")
	require.ErrorIs(t, s.stepOff(hero), encounter.ErrInCustody)
}

// TestMadKingCustodyIsKeptOnReload: held survives a save and a load, broken
// does too, and a record naming a phase nobody wrote is refused on load.
func TestMadKingCustodyIsKeptOnReload(t *testing.T) {
	t.Parallel()
	s, _ := theArrest(t)
	s.roundTrip()
	require.Equal(t, encounter.CustodyHeld, s.custody(), "held is kept")
	require.ErrorIs(t, s.stepOff(hero), encounter.ErrInCustody, "and still holds")

	s.swingAs(hero, kingCaptain)
	s.roundTrip()
	require.Equal(t, encounter.CustodyBroken, s.custody(), "broken is kept")

	data := s.enc.ToData()
	blob, err := json.Marshal(data)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(blob, &raw))
	for _, it := range raw["interactions"].([]any) {
		if arrest, ok := it.(map[string]any)["arrest"].(map[string]any); ok {
			arrest["custody"] = "pardoned"
		}
	}
	blob, err = json.Marshal(raw)
	require.NoError(t, err)
	var bad encounter.EncounterData
	require.NoError(t, json.Unmarshal(blob, &bad))
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: bad, Capabilities: s.host.capabilities(s.strikes, s.standing)})
	require.ErrorContains(t, err, `custody "pardoned"`)
}
