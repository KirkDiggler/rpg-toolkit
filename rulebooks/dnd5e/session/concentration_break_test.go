// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// The break, as a scene: one member holding a spell together, one blow, and
// the whole consequence arriving from the single Attack call that caused it.
//
// WHY A DUEL AND NOT A BARD CASTING. What is under test is the seam, not the
// spell. A concentrating condition on a stored sheet is a concentrating
// condition however it got there, and building the scene from two fighters
// keeps the fixture out of the cast door's economy — a second cast in one turn
// is refused by the action ledger, so a caster cannot be made to break its own
// concentration inside one turn anyway.
type ConcentrationBreakSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
	// mark is alice's last story seq when the scene was handed over.
	mark uint64
}

func TestConcentrationBreakSuite(t *testing.T) {
	suite.Run(t, new(ConcentrationBreakSuite))
}

// heldSpellRef and heldSpellName are what bob is holding together. True Strike
// because it is the retrofit this slice's design names, and because a spell
// whose effect sits on somebody ELSE'S sheet is the case where the removal
// train has something to say.
var heldSpellRef = refs.Spells.TrueStrike()

const heldSpellName = "True Strike"

// scene stands up the duel with bob concentrating and the dice scripted, and
// marks where alice's story stood when it was handed over.
//
// The dice are consumed in the order the fight asks for them: the swing's d20,
// the weapon's damage die, then the defender's Constitution check. Authored
// rather than random because every number this suite asserts — the DC most of
// all — is arithmetic on those faces.
func (s *ConcentrationBreakSuite) scene(children []dnd5eEvents.ConditionAddress, rolls ...int) {
	s.T().Helper()

	bob := armedFighter("bob")
	bob.Conditions = []json.RawMessage{s.holdingBlob("bob", children)}

	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), bob)

	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{},
		Dice:            &sequenceDice{rolls: rolls},
		TurnDriver:      session.Pass{},
		Sessions:        s.sessions, Encounters: s.encounters,
		Characters: s.characters,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	launchDuel(s.T(), mgr, s.encounters)
	// Launch long-rests the party, which ends bob's concentration: the
	// concentrating sheet is written back after it, as the host's would be
	// once he cast. The launch's opening beats predate the scene.
	rewound(s.characters, armedFighter("alice"), bob)
	// The launch observed bob rested; the table re-looks at him holding the
	// spell, so the testimony matches the sheet before the swing.
	_, err = mgr.Recheck(context.Background(), &session.RecheckInput{Session: "sess", Members: []string{"bob"}})
	s.Require().NoError(err)
	s.mark = lastSeq(storyBeats(s.T(), mgr, "sess", "alice"))
}

// holdingBlob is one concentrating condition as the host stores it: an opaque
// blob the sheet loads back and attaches.
func (s *ConcentrationBreakSuite) holdingBlob(
	member string, children []dnd5eEvents.ConditionAddress,
) json.RawMessage {
	s.T().Helper()

	holding := conditions.NewConcentratingCondition(
		member, heldSpellRef.String(), heldSpellName, 10)
	holding.Children = children

	blob, err := holding.ToJSON()
	s.Require().NoError(err)
	return blob
}

// swing is alice hitting bob with the selector Afford authored.
func (s *ConcentrationBreakSuite) swing() (*session.AttackOutput, error) {
	return s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
		DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "alice"),
	})
}

// told is what alice's story gained since the scene was handed over.
func (s *ConcentrationBreakSuite) told() []witnessed {
	s.T().Helper()
	return storyBeatsAfter(s.T(), s.mgr, "sess", "alice", s.mark)
}

// kinds is the ordered list of beat words alice was told.
func (s *ConcentrationBreakSuite) kinds() []string {
	s.T().Helper()
	return beatNames(s.told())
}

// savedBeat is a saved beat's payload.
type savedBeat struct {
	Saver       string                   `json:"saver"`
	Ability     string                   `json:"ability"`
	Roll        int                      `json:"roll"`
	Total       int                      `json:"total"`
	DC          int                      `json:"dc"`
	Succeeded   bool                     `json:"succeeded"`
	Source      session.SpellRef         `json:"source"`
	Calculation *session.RollCalculation `json:"calculation"`
}

// saves is every saved beat alice was told, in order.
func (s *ConcentrationBreakSuite) saves() []savedBeat {
	s.T().Helper()
	var out []savedBeat
	for _, beat := range beatsNamed(s.told(), encounter.BeatSaved) {
		var body savedBeat
		beat.decode(s.T(), &body)
		out = append(out, body)
	}
	return out
}

// TestOneBlowIsOneTrain is the slice's own done-when, at the seam: a member
// holding a spell together is struck, fails the check, and the whole break —
// the roll, the loss, and the effect stripped off somebody else's sheet —
// arrives from ONE Attack call, in the order it happened.
//
// A d20 of 18 hits AC 12; a 5 on the longsword plus 3 for STR 16 is 8 damage,
// which asks for the floor DC of 10; a 3 on the check plus 2 for CON 14
// reaches 5 and loses the spell.
func (s *ConcentrationBreakSuite) TestOneBlowIsOneTrain() {
	held := []dnd5eEvents.ConditionAddress{{
		MemberID: "alice", ConditionRef: refs.Conditions.Concentrating().String(),
	}}
	s.scene(held, 18, 5, 3)

	out, err := s.swing()
	s.Require().NoError(err)
	s.Require().True(out.Hit, "the swing landed, which is what provokes the check")

	// The last beat is the commit's freshness step (rpg-project#520, R19):
	// alice watched bob holding the spell, and he no longer is, so she is
	// told to look at him again.
	s.Equal([]string{
		string(encounter.OutcomeStruck),
		encounter.BeatSaved,
		encounter.BeatConcentrationEnded,
		encounter.BeatActivationResult,
		encounter.BeatSighted,
	}, s.kinds(),
		"one call, one train: the blow, the check it forced, the spell it ended, and the "+
			"effect that came off with it")
}

// TestTheBreakBeatSaysWhoWhatAndWhy reads the beat a client narrates from.
func (s *ConcentrationBreakSuite) TestTheBreakBeatSaysWhoWhatAndWhy() {
	s.scene(nil, 18, 5, 3)

	_, err := s.swing()
	s.Require().NoError(err)

	type breakBeat struct {
		Caster string           `json:"caster"`
		Spell  session.SpellRef `json:"spell"`
		Reason string           `json:"reason"`
	}
	beats := beatsNamed(s.told(), encounter.BeatConcentrationEnded)
	s.Require().Len(beats, 1)
	var body breakBeat
	beats[0].decode(s.T(), &body)
	s.Equal(breakBeat{
		Caster: "bob",
		Spell:  session.SpellRef{Ref: heldSpellRef.String(), Name: heldSpellName},
		// The rulebook's own word for a failed check after damage, crossing
		// unchanged.
		Reason: conditions.ConcentrationEndedDamage,
	}, body)
}

// TestTheCheckIsTheRulebooksArithmeticAndNotThisSeams reads the saved beat.
//
// The DC is the thing worth pinning, and it is pinned as a NUMBER THIS SEAM
// NEVER COMPUTES: 20 damage asks for 10, which is half of it, while 8 asks for
// the floor of 10 as well. Two damage numbers with the same answer would prove
// nothing, so the two scenes ask for different ones.
func (s *ConcentrationBreakSuite) TestTheCheckIsTheRulebooksArithmeticAndNotThisSeams() {
	for name, tc := range map[string]struct {
		damageDie int
		wantDC    int
	}{
		// 5 + 3 = 8 damage, half of which is under the floor.
		"the floor": {damageDie: 5, wantDC: 10},
		// A critical is not in play, so the biggest a longsword swings is
		// 8 + 3 = 11 — still the floor. The floor is what a longsword can
		// reach, and a higher DC belongs to a scene with a bigger weapon in
		// it than this fixture has.
		"the top of a longsword": {damageDie: 8, wantDC: 10},
	} {
		s.Run(name, func() {
			s.scene(nil, 18, tc.damageDie, 3)

			_, err := s.swing()
			s.Require().NoError(err)

			saves := s.saves()
			s.Require().Len(saves, 1)
			saved := saves[0]

			s.Equal("bob", saved.Saver, "the holder rolls, not the striker")
			s.Equal("con", saved.Ability)
			s.Equal(3, saved.Roll)
			s.Equal(5, saved.Total, "3 on the die and 2 for CON 14")
			s.Equal(tc.wantDC, saved.DC)
			s.False(saved.Succeeded)
			s.Equal(session.SpellRef{Ref: heldSpellRef.String(), Name: heldSpellName}, saved.Source,
				"the saved beat names the spell that was at stake")
			s.Require().NotNil(saved.Calculation)
			s.Equal(saved.Total, saved.Calculation.Total)
			s.Require().NotEmpty(saved.Calculation.Components)
			s.Require().NotNil(saved.Calculation.Components[0].Dice)
			s.Equal(saved.Roll, saved.Calculation.Components[0].Dice.Subtotal)
		})
	}
}

// TestTheStrippedEffectIsNamedByTheRulebook reads the removal beat.
//
// THE NAME IS RESOLUTION'S, and this seam neither supplies nor checks it. An
// earlier draft of this suite asserted the SPELL'S name here, because the seam
// was building the removal itself off an address that carries no display name.
// It does not build it any more: the whole result arrives named, and what
// arrives is the CONDITION'S own name, which is the more useful of the two —
// a player watching an effect come off their sheet is owed the effect's name,
// and the spell that held it is already on the break beat one line above.
func (s *ConcentrationBreakSuite) TestTheStrippedEffectIsNamedByTheRulebook() {
	childRef := refs.Conditions.Concentrating().String()
	s.scene([]dnd5eEvents.ConditionAddress{{MemberID: "alice", ConditionRef: childRef}}, 18, 5, 3)

	_, err := s.swing()
	s.Require().NoError(err)

	beats := beatsNamed(s.told(), encounter.BeatActivationResult)
	s.Require().Len(beats, 1)
	var body struct {
		Result struct {
			Kind   string `json:"kind"`
			Target string `json:"target"`
			Ref    string `json:"ref"`
			Name   string `json:"name"`
			Reason string `json:"reason"`
		} `json:"result"`
	}
	beats[0].decode(s.T(), &body)
	removed := body.Result
	s.Equal(string(encounter.ResultConditionRemoved), removed.Kind,
		"a stripped child is a condition-removed result and nothing else")

	s.Equal("alice", removed.Target,
		"the effect came off the sheet it was sitting on, which is not the caster's")
	s.Equal(childRef, removed.Ref)
	s.NotEmpty(removed.Name,
		"the rulebook named it, and a nameless removal is a line a client cannot write")
	s.Equal(conditions.ConcentrationEndedDamage, removed.Reason,
		"one cause for the whole event, so a reader never holds two")
}

// TestAMadeCheckIsARollTheTableSees is the other half of the answer.
//
// A 19 on the check reaches 21 and holds the spell. Nothing ends, so no break
// beat — and the roll is still narrated, because a check nobody saw is a blow
// that mysteriously did not cost anything. The saved beat rides the same train
// as the strike, ahead of any break, which is why it can stand alone here.
//
// This test previously pinned the opposite, when a made check had nowhere in
// the record to live. The shape it named as a gap now exists, and this is what
// says so.
func (s *ConcentrationBreakSuite) TestAMadeCheckIsARollTheTableSees() {
	s.scene(nil, 18, 5, 19)

	_, err := s.swing()
	s.Require().NoError(err)

	s.Equal([]string{string(encounter.OutcomeStruck), encounter.BeatSaved}, s.kinds(),
		"the blow and the check it forced — and nothing ended, so nothing more")

	saves := s.saves()
	s.Require().Len(saves, 1)
	saved := saves[0]
	s.Equal("bob", saved.Saver)
	s.Equal(19, saved.Roll)
	s.Equal(21, saved.Total, "19 on the die and 2 for CON 14")
	s.Equal(10, saved.DC)
	s.True(saved.Succeeded, "the spell was kept, and the beat says so")
	s.Equal(session.SpellRef{Ref: heldSpellRef.String(), Name: heldSpellName}, saved.Source,
		"a made check still names what was at stake")
	s.Require().NotNil(saved.Calculation)
	s.Equal(saved.Total, saved.Calculation.Total)

	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{
		Session: "sess", Member: "bob",
	})
	s.Require().NoError(err)
	for _, row := range turn.Participants {
		if row.Member == "bob" {
			s.True(row.Concentrating, "and the spell is still being held together")
		}
	}
}

// TestAMissBreaksNothing is the regression that matters most, and it is the
// one the composition's own suite names too: a swing that ends nobody's
// concentration must write exactly what it wrote before this shape existed.
func (s *ConcentrationBreakSuite) TestAMissBreaksNothing() {
	s.scene(nil, 2)

	out, err := s.swing()
	s.Require().NoError(err)
	s.Require().False(out.Hit)

	s.Equal([]string{string(encounter.OutcomeMissed)}, s.kinds(), "the miss, and nothing followed")
}
