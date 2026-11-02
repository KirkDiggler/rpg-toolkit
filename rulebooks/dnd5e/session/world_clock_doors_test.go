// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// world_clock_doors_test.go pins the doors the bar opens on the world clock
// (rpg-project#785 S1, R1: one round of the world per action). Each declaring
// door that refused every free-roam selector as stale now admits one, and is
// refused only by its own facts: reach for a swing, a spell slot for a cast,
// a charge for an ability. Each admitted act keeps no round on the sheet: the
// economy lent to the action is put out before the sheet is written.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type WorldClockDoorsSuite struct {
	suite.Suite
	characters *fakeCharacters
	dice       *sequenceDice
	mgr        *session.Manager
}

func TestWorldClockDoorsSuite(t *testing.T) {
	suite.Run(t, new(WorldClockDoorsSuite))
}

// freeRoam launches the sheets side by side on an open hall with nobody else
// in it, so nothing forms a fight and every member stays on the world clock.
func (s *WorldClockDoorsSuite) freeRoam(rolls []int, at []spatial.Position, sheets ...*character.Data) {
	s.T().Helper()
	s.characters = newFakeCharacters(sheets...)
	s.dice = &sequenceDice{rolls: rolls}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: s.characters,
	})
	s.Require().NoError(err)
	s.mgr = mgr
	party := make([]sceneSeat, 0, len(sheets))
	for i, sheet := range sheets {
		party = append(party, seatAt(sheet.ID, int(at[i].X), int(at[i].Y)))
	}
	launchScene(s.T(), mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 12, 8)}},
		Party: party,
	})
	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: sheets[0].ID})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock, "precondition: free roam, nobody is fighting")
}

// row is the member's one Afford row for a verb (and, for Cast and Activate,
// the named spell or ability).
func (s *WorldClockDoorsSuite) row(member string, verb session.Verb, ref string) session.Declaration {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, out.Clock)
	for _, d := range out.Declarations {
		if d.Verb != verb {
			continue
		}
		switch {
		case ref == "":
			return d
		case d.Spell != nil && d.Spell.Ref == ref, d.Ability != nil && d.Ability.Ref == ref:
			return d
		}
	}
	s.Require().Failf("no row", "%s offered no %s row %s", member, verb, ref)
	return session.Declaration{}
}

func (s *WorldClockDoorsSuite) stored(id string) *character.Data {
	s.T().Helper()
	sheet, err := s.characters.GetCharacter(context.Background(), id)
	s.Require().NoError(err)
	return sheet
}

// ATTACK: the active-member comparison is skipped on the world clock and
// nothing else is. The swing is selected from the row Afford offered, rolls,
// lands, and the round lent to price it does not survive on the sheet.
func (s *WorldClockDoorsSuite) TestAFreeRoamSwingIsAdmitted() {
	s.freeRoam([]int{15, 5}, []spatial.Position{{X: 1, Y: 1}, {X: 2, Y: 1}}, armedFighter("alice"), armedFighter("bob"))
	rewound(s.characters, armedFighter("alice"), armedFighter("bob"))

	row := s.row("alice", session.VerbAttack, "")
	s.Require().True(row.Available, "bob is in reach: %+v", row.Why)
	s.Equal(session.SlotNone, row.Slot, "the world clock draws no slot")

	out, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: row.ID,
	})
	s.Require().NoError(err)
	s.True(out.Hit)
	s.Less(s.stored("bob").HitPoints, 24, "the blow landed on bob's stored sheet")
	s.Nil(s.stored("alice").ActionEconomy, "the round lent to the swing is not kept")
}

// ATTACK, REFUSED BY ITS OWN FACT: a target out of reach. The row is offered
// and unavailable for that reason alone, and the door refuses the selector
// before a die is rolled.
func (s *WorldClockDoorsSuite) TestAFreeRoamSwingOutOfReachIsRefused() {
	s.freeRoam([]int{15, 5}, []spatial.Position{{X: 1, Y: 1}, {X: 6, Y: 1}}, armedFighter("alice"), armedFighter("bob"))
	rewound(s.characters, armedFighter("alice"), armedFighter("bob"))

	row := s.row("alice", session.VerbAttack, "")
	s.False(row.Available)
	s.Require().NotNil(row.Why)
	s.Equal(session.ShortfallNoTargetInReach, row.Why.Reason, "reach, and not the clock, is what refuses")

	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: row.ID,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.NotErrorIs(err, session.ErrNotYourTurn)
	s.Zero(s.dice.next, "no die rolls for a refused swing")
	s.Equal(24, s.stored("bob").HitPoints)
}

// CAST: the active-member comparison is skipped; the spell slot is the
// sheet's own and is spent; the round is not kept.
func (s *WorldClockDoorsSuite) TestAFreeRoamCastIsAdmittedAndSpendsItsSlot() {
	patient := armedFighter("patient")
	patient.MaxHitPoints = 20
	s.freeRoam([]int{4}, []spatial.Position{{X: 1, Y: 1}, {X: 2, Y: 1}}, healingCleric(), patient)
	// Wounded after the launch: its first-admission rest would heal her.
	s.characters.byID["patient"].HitPoints = 10
	before := s.stored("cleric").Resources[resources.SpellSlotLevel1].Current

	row := s.row("cleric", session.VerbCast, refs.Spells.CureWounds().String())
	s.Require().True(row.Available, "the patient is in touch: %+v", row.Why)
	s.Equal(session.SlotNone, row.Slot)
	for _, component := range row.Cost {
		s.Equal(session.CurrencyCharges, component.Currency, "only the sheet's own charge is priced on the world clock")
	}

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"patient"},
	})
	s.Require().NoError(err)
	s.Greater(s.stored("patient").HitPoints, 10, "the heal landed")
	s.Equal(before-1, s.stored("cleric").Resources[resources.SpellSlotLevel1].Current, "a slot is the sheet's, spent on either clock")
	s.Nil(s.stored("cleric").ActionEconomy, "the round lent to the cast is not kept")
}

// CAST, REFUSED BY ITS OWN FACT: no slot left. The row says so in the sheet's
// own currency, never in a turn's, and the door refuses the selector.
func (s *WorldClockDoorsSuite) TestAFreeRoamCastWithNoSlotIsRefused() {
	patient := armedFighter("patient")
	s.freeRoam([]int{4}, []spatial.Position{{X: 1, Y: 1}, {X: 2, Y: 1}}, healingCleric(), patient)
	cleric := s.characters.byID["cleric"]
	slots := cleric.Resources[resources.SpellSlotLevel1]
	slots.Current = 0
	cleric.Resources[resources.SpellSlotLevel1] = slots

	row := s.row("cleric", session.VerbCast, refs.Spells.CureWounds().String())
	s.False(row.Available)
	s.Require().NotNil(row.Why)
	s.Equal(session.ShortfallNoBudget, row.Why.Reason)
	s.Equal(session.CurrencyCharges, row.Why.Currency, "the slot is short, not the turn")

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "cleric", DeclarationID: row.ID, Targets: []string{"patient"},
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.NotErrorIs(err, session.ErrNotYourTurn)
	s.Zero(s.dice.next, "no die rolls for a refused cast")
}

// ACTIVATE: the active-member comparison is skipped; the ability spends its
// own charge out of the sheet, and its bonus action out of the lent round,
// which is not kept.
func (s *WorldClockDoorsSuite) TestAFreeRoamActivationIsAdmitted() {
	s.freeRoam([]int{6}, []spatial.Position{{X: 1, Y: 1}}, secondWindFighter(s.T(), "alice", 10, 28))
	s.characters.byID["alice"].HitPoints = 10
	ref := refs.Features.SecondWind().String()

	row := s.row("alice", session.VerbActivate, ref)
	s.Require().True(row.Available, "%+v", row.Why)
	s.Equal(session.SlotNone, row.Slot)

	_, err := s.mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: row.ID,
	})
	s.Require().NoError(err)
	s.Greater(s.stored("alice").HitPoints, 10, "Second Wind healed her")
	s.Nil(s.stored("alice").ActionEconomy, "the round lent to the activation is not kept")

	// REFUSED BY ITS OWN FACT the second time: the charge is spent, which the
	// row says, and the door refuses the old selector.
	again := s.row("alice", session.VerbActivate, ref)
	s.False(again.Available)
	s.Require().NotNil(again.Why)
	// The ability's own words, exactly as on the turn clock: the uses are
	// gone. Never a turn currency.
	s.Equal(session.ShortfallUnavailable, again.Why.Reason)
	s.Contains(again.Why.Text, "uses remaining")
	s.Empty(again.Why.Currency, "no turn currency falls short on the world clock")
	_, err = s.mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: row.ID,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.NotErrorIs(err, session.ErrNotYourTurn)
}
