// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_exit_test.go is NO EXIT MAY SAVE A RECORD THAT CANNOT LOAD
// (rpg-project#942). A bard hailed at the gate left the run while the
// captain's address still named them; the saved run named a member it no
// longer had, the loader refused it, and every later verb failed for
// everyone. The loader was right and Exit was wrong.
//
// The reproduction is the operator's own walk, and its mirror (the guard of
// the standing address leaves). The property is wider: from every kind of
// standing state the castle's conversation can carry, every member's exit
// leaves a record that loads, and the survivors can still step and speak.

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// hailedScene is the run stored at the moment a gate speaker hails the hero,
// built once per test binary: the hero seated in the gatehouse steps toward
// the gate until hailed, with the bard on the road behind.
var hailedScene struct {
	once  sync.Once
	blob  []byte
	guard encounter.MemberID
	base  *kingScene
	err   error
}

// hailed loads the stored hail into a fresh scene for one test, and names
// the guard addressing the hero.
func hailed(t *testing.T) (*kingScene, encounter.MemberID) {
	t.Helper()
	hailedScene.once.Do(func() {
		tb := &fixtureTB{}
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(fixtureFailed); !ok {
					panic(r)
				}
				hailedScene.err = fmt.Errorf("the hailed scene: %s", tb.failed)
			}
		}()
		start := partyStart(tb)
		s := openTheCastleAs(tb, castleHost{
			at:      gatehouseSeat,
			players: []encounter.MemberInput{{ID: bard, Kind: encounter.KindPlayer, Position: spatial.Position{X: start.X + 2, Y: start.Y}}},
		})
		route, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveToward, Anchor: s.at(kingCaptain), Budget: 50})
		require.NoError(tb, err)
		for i, cell := range route.Path {
			out, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: i == len(route.Path)-1, Reach: speakReach})
			require.NoError(tb, err)
			if len(out.Addressed) > 0 {
				hailedScene.guard = out.Addressed[0].Guard
				break
			}
		}
		require.NotEmpty(tb, hailedScene.guard, "a gate speaker hails the hero")
		hailedScene.blob, err = json.Marshal(s.enc.ToData())
		require.NoError(tb, err)
		hailedScene.base = s
	})
	require.NoError(t, hailedScene.err)
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(hailedScene.blob, &data))
	s := &kingScene{t: t, compiled: hailedScene.base.compiled, host: hailedScene.base.host,
		strikes: &[]cellsStrike{}, standing: &castleStanding{down: map[encounter.MemberID]bool{}}}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: s.host.capabilities(s.strikes, s.standing),
	})
	require.NoError(t, err)
	s.enc = enc
	require.True(t, addressNames(s.enc, hero), "precondition: the address stands")
	return s, hailedScene.guard
}

// loadsBack saves the run and loads it back, as a host does between verbs.
func (s *kingScene) loadsBack() error {
	s.t.Helper()
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(s.t, err)
	var data encounter.EncounterData
	require.NoError(s.t, json.Unmarshal(blob, &data))
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: s.host.capabilities(s.strikes, s.standing),
	})
	if err != nil {
		return err
	}
	s.enc = enc
	return nil
}

// addressNames reports whether any saved address names the member.
func addressNames(enc *encounter.Encounter, member encounter.MemberID) bool {
	for _, in := range enc.ToData().Interactions {
		if in.Address != nil && (in.Address.Member == member || in.Address.Guard == member) {
			return true
		}
	}
	return false
}

// TestMadKingAnAddressedMemberWhoLeavesLeavesARunThatLoads is the
// reproduction: the hero is hailed, so the conversation's address names
// them, and the hero leaves. (On the operator's walk the troupe claim missed
// first; that only kept the address standing, which the hail itself does.)
// Before the fix the saved run was refused: interaction "front-gate" is
// saved addressing "hero", who is not in the run.
func TestMadKingAnAddressedMemberWhoLeavesLeavesARunThatLoads(t *testing.T) {
	t.Parallel()
	s, _ := hailed(t)

	_, err := s.enc.Exit(&encounter.ExitInput{Member: hero})
	require.NoError(t, err, "the hero leaves")
	require.False(t, addressNames(s.enc, hero), "leaving ends the address that named the hero")
	require.NoError(t, s.loadsBack(), "the run the hero left loads")

	s.walkMember(bard, s.at(kingCaptain), false)
	_, err = s.enc.Exit(&encounter.ExitInput{Member: bard})
	require.NoError(t, err, "the bard can still act and leave")
	require.NoError(t, s.loadsBack())
}

// TestMadKingTheGuardOfAStandingAddressWhoLeavesLeavesARunThatLoads is the
// mirror: the guard making the address leaves. Before the fix: interaction
// "front-gate" is saved addressed by "captain", who is not in the run.
func TestMadKingTheGuardOfAStandingAddressWhoLeavesLeavesARunThatLoads(t *testing.T) {
	t.Parallel()
	s, guard := hailed(t)

	_, err := s.enc.Exit(&encounter.ExitInput{Member: guard})
	require.NoError(t, err, "the hailing guard leaves")
	require.False(t, addressNames(s.enc, guard), "leaving ends the address the guard was making")
	require.NoError(t, s.loadsBack(), "the run the guard left loads")
	s.survivorsAct(hero)
}

// survivorsAct has a surviving player take a step and say a line someone
// offers them, as the next verbs after a reload would.
func (s *kingScene) survivorsAct(player encounter.MemberID) {
	s.t.Helper()
	if clockKind(s.t, s.enc, player) == encounter.ClockTurn {
		// In a fight a player acts on their turn; the reload is the claim.
		return
	}
	route, err := s.enc.Route(encounter.RouteInput{Mover: player, Policy: encounter.MoveToward, Anchor: partyStart(s.t), Budget: 3})
	require.NoError(s.t, err, "%s can plan a step", player)
	if len(route.Path) > 0 {
		_, err = s.enc.Step(&encounter.StepInput{Member: player, To: route.Path[0], EndWalk: true, Reach: speakReach})
		require.NoError(s.t, err, "%s can step", player)
	}
	members, err := s.enc.Members()
	require.NoError(s.t, err)
	for _, m := range members {
		offered, err := s.enc.OptionsFor(player, m.ID)
		if err != nil {
			continue
		}
		for _, o := range offered {
			if o.Pending {
				continue
			}
			in := &encounter.SpeakInput{
				Range: speakReach, Actor: player, Target: m.ID, OptionID: o.ID, Roller: rollsOne{}, Revision: o.Revision,
			}
			if len(o.Check) > 0 {
				in.Skill, in.Beaten, in.DC, in.Total = o.Check[0].Ability, true, o.Check[0].DC, o.Check[0].DC+5
			}
			_, err := s.enc.Speak(s.t.Context(), in)
			if errors.Is(err, encounter.ErrOutOfRange) || errors.Is(err, encounter.ErrUnwitnessed) {
				continue
			}
			require.NoError(s.t, err, "%s can say %q to %s", player, o.ID, m.ID)
			return
		}
	}
}

// everyExitLoads is the property: from the stored run, each member in turn
// leaves, the run saves and loads back, and every surviving player can still
// step and speak. after, when given, runs once more on the loaded run.
func everyExitLoads(t *testing.T, base *kingScene, after func(s *kingScene, left encounter.MemberID)) {
	t.Helper()
	blob, err := json.Marshal(base.enc.ToData())
	require.NoError(t, err)
	members, err := base.enc.Members()
	require.NoError(t, err)
	for _, leaver := range members {
		// Each leaver from its own copy of the stored run, side by side.
		t.Run(string(leaver.ID), func(t *testing.T) {
			t.Parallel()
			var data encounter.EncounterData
			require.NoError(t, json.Unmarshal(blob, &data))
			s := &kingScene{t: t, compiled: base.compiled, host: base.host,
				strikes: &[]cellsStrike{}, standing: &castleStanding{down: map[encounter.MemberID]bool{}}}
			enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Data: data, Capabilities: s.host.capabilities(s.strikes, s.standing),
			})
			require.NoError(t, err)
			s.enc = enc
			_, err = s.enc.Exit(&encounter.ExitInput{Member: leaver.ID})
			require.NoError(t, err, "%s leaves", leaver.ID)
			require.NoError(t, s.loadsBack(), "the run %s left loads", leaver.ID)
			if after != nil {
				after(s, leaver.ID)
			}
			survivors, err := s.enc.Members()
			require.NoError(t, err)
			for _, m := range survivors {
				if m.Kind == encounter.KindPlayer {
					s.survivorsAct(m.ID)
				}
			}
			require.NoError(t, s.loadsBack(), "the run loads after the survivors acted once %s left", leaver.ID)
		})
	}
}

// TestMadKingEveryExitFromAStandingAddressLoads: the address stands, and
// anybody leaves.
func TestMadKingEveryExitFromAStandingAddressLoads(t *testing.T) {
	t.Parallel()
	s, _ := hailed(t)
	everyExitLoads(t, s, nil)
}

// TestMadKingEveryExitFromAWaitingRollLoads: the hero's guest claim waits on
// the hero's roll (a reservation, its holder the hero and its target the
// captain) while the address stands, and anybody leaves.
func TestMadKingEveryExitFromAWaitingRollLoads(t *testing.T) {
	t.Parallel()
	s, _ := hailed(t)
	_, err := s.enc.Reserve(&encounter.ReserveInput{
		Actor: hero, Target: kingCaptain, OptionID: "guest", Revision: s.revisionOf(hero, kingCaptain, "guest"), Range: speakReach,
	})
	require.NoError(t, err, "precondition: the guest claim waits on the hero's roll")
	everyExitLoads(t, s, nil)
}

// TestMadKingEveryExitFromAnAbandonedRollLoads: the straggler's case waits
// on their roll and they walk away, so the reservation is abandoned with its
// scene (its witnesses, the seen and the perceived), and anybody leaves; the
// straggler, when still here, can then settle it as rolled.
func TestMadKingEveryExitFromAnAbandonedRollLoads(t *testing.T) {
	t.Parallel()
	s := admittedWithAStraggler(t)
	road := s.at(straggler)
	s.walkMember(straggler, s.at(hero), false)
	at := s.revisionOf(straggler, kingGate1, stragglerLine)
	_, err := s.enc.Reserve(&encounter.ReserveInput{
		Actor: straggler, Target: kingGate1, OptionID: stragglerLine, Revision: at, Range: speakReach,
	})
	require.NoError(t, err)
	s.walkMember(straggler, road, false)
	abandoned := false
	for _, in := range s.enc.ToData().Interactions {
		abandoned = abandoned || (in.Reservation != nil && in.Reservation.Abandoned)
	}
	require.True(t, abandoned, "precondition: the hold is abandoned")
	everyExitLoads(t, s, func(s *kingScene, left encounter.MemberID) {
		if left == straggler || left == kingGate1 {
			return
		}
		_, err := s.enc.Speak(s.t.Context(), &encounter.SpeakInput{
			Range: speakReach, Actor: straggler, Target: kingGate1, OptionID: stragglerLine, Roller: rollsOne{},
			Revision: at, Skill: "persuasion", Beaten: false, DC: 13, Total: 3,
		})
		require.NoError(s.t, err, "the straggler settles the kept roll once %s left", left)
	})
}

// TestMadKingEveryExitFromTheAdmittedGateLoads: the gate admitted the hero,
// so the guards hold recognition records about the party (judged and
// heard), the captain walked into the hall, and anybody leaves — a holder
// of a record or its subject.
func TestMadKingEveryExitFromTheAdmittedGateLoads(t *testing.T) {
	t.Parallel()
	s := admittedWithAStraggler(t)
	require.True(t, s.recognizes(kingCaptain, hero), "precondition: the captain recognises the hero")
	everyExitLoads(t, s, nil)
}

// TestMadKingEveryExitFromAFightLoads: the hero struck a gate guard, so a
// fight's bubble holds its members in order, and anybody leaves.
func TestMadKingEveryExitFromAFightLoads(t *testing.T) {
	t.Parallel()
	s := openTheScript(t, gatehouseSeat, nil)
	s.struck(kingGate1, false)
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "precondition: a fight")
	everyExitLoads(t, s, nil)
}

// TestMadKingAMemberWhoLeftCanComeBack: the hero, hailed, leaves and joins
// the run again; the run loads, and the hero can step and speak.
func TestMadKingAMemberWhoLeftCanComeBack(t *testing.T) {
	t.Parallel()
	s, _ := hailed(t)
	left := s.at(hero)
	_, err := s.enc.Exit(&encounter.ExitInput{Member: hero})
	require.NoError(t, err)
	require.NoError(t, s.loadsBack())
	_, err = s.enc.Join(&encounter.JoinInput{Member: hero, Kind: encounter.KindPlayer, Cell: left})
	require.NoError(t, err, "the hero joins again")
	require.NoError(t, s.loadsBack(), "the run the hero came back to loads")
	s.survivorsAct(hero)
	require.NoError(t, s.loadsBack())
}

// TestMadKingAnAddressedMemberWhoFallsLoads: the hero goes down while the
// address stands. A member who is down is still a member, so the run loads
// with the address; the next step anyone takes ends it as `down`.
func TestMadKingAnAddressedMemberWhoFallsLoads(t *testing.T) {
	t.Parallel()
	s, _ := hailed(t)
	s.standing.down[hero] = true
	require.NoError(t, s.loadsBack(), "the run with a fallen addressee loads")
	s.walkMember(bard, s.at(kingGate2), false)
	require.False(t, addressNames(s.enc, hero), "the next step ends the address on the fallen hero")
	require.NoError(t, s.loadsBack())
}
