// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// ObeyTestSuite drives the compelled turn: a creature under an order, and what
// the order makes of its turn.
//
// Two of the three words describe a walk and nothing here computes a cell; the
// third applies a condition, which this package finishes rather than describes.
type ObeyTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestObeySuite(t *testing.T) {
	suite.Run(t, new(ObeyTestSuite))
}

func (s *ObeyTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ObeyTestSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

// interaction is the cast, world and seams a verb would build, with no machine
// — Obey supplies that.
func (s *ObeyTestSuite) interaction(fixtures *ContestDamageTestSuite, participants ...Participant) *Input {
	return &Input{
		World: fixtures.world(), Participants: participants,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller(),
	}
}

func commandedRef() core.Ref { return *refs.Conditions.Commanded() }

// A WALKING WORD DESCRIBES AND MOVES NOBODY, exactly as a shove does. The
// policy and the anchor are the whole of what the rules say; which cells those
// are is read off the field by whoever owns the board.
func (s *ObeyTestSuite) TestAWalkingWordDescribesTheWalkAndWalksNobody() {
	for _, tc := range []struct {
		word   string
		policy combatActions.MovePolicy
		reads  string
	}{
		{wordApproach, combatActions.MoveToward, "a toward move of their own turn's movement"},
		{wordFlee, combatActions.MoveAway, "an away move of their own turn's movement"},
	} {
		s.Run(tc.word, func() {
			fixtures := s.fixtures()
			out, err := Obey(s.ctx, &ObeyInput{
				Interaction: s.interaction(fixtures,
					Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)}),
				Member: heroID, CasterID: bardID, Word: tc.word, Source: commandedRef(),
			})
			s.Require().NoError(err)

			s.Require().Len(out.Effects, 1, "a walking word imposes the walk and nothing else")
			moved := out.Effects[0]
			s.Equal(ImposedMove, moved.Kind)
			s.Equal(heroID, moved.RecipientID)
			s.Equal(refs.Conditions.Commanded(), moved.Ref, "the compulsion is what caused it")
			s.Require().NotNil(moved.Move)
			s.Equal(tc.policy, moved.Move.Policy)
			s.Equal(bardID, moved.Move.AnchorID, "measured from whoever gave the order")
			s.True(moved.Move.Turn)
			s.Zero(moved.Move.Cells)
			s.False(moved.Move.Speed, "exactly one budget, and it is the turn")
			s.Equal(combatActions.PaysNothing, moved.Move.Pays, "the turn IS the price")
			s.True(moved.Move.Provokes, "the creature is walking out on its own legs")
			s.Equal(tc.reads, moved.Description)
		})
	}
}

// GROVEL IS APPLIED, NOT DESCRIBED, and the source is the compulsion rather
// than the spell: the order is what put the creature on the floor, and the
// spell that chose the word may have ended a turn ago.
func (s *ObeyTestSuite) TestGrovelPutsTheCreatureProneOnTheBus() {
	fixtures := s.fixtures()
	bus := events.NewEventBus()
	var applied []string
	_, err := dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			applied = append(applied, string(event.Type)+":"+event.Target.GetID()+":"+string(event.Source))
			return nil
		})
	s.Require().NoError(err)

	machine, err := newObey(&ObeyInput{
		Interaction: &Input{}, Member: heroID, CasterID: bardID,
		Word: wordGrovel, Source: commandedRef(),
	})
	s.Require().NoError(err)
	interaction := s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)})
	interaction.Machine = machine
	out, err := resolveOn(s.ctx, interaction, newSurface(bus))
	s.Require().NoError(err)

	s.Equal([]string{refs.Conditions.Prone().ID + ":" + heroID + ":" + string(dnd5eEvents.ConditionSourceSpell)},
		applied)

	outcome, ok := out.Outcome.(ObeyOutcome)
	s.Require().True(ok, "an obeyed word produces an ObeyOutcome")
	s.Require().Len(outcome.Effects, 1)
	s.Equal(ImposedCondition, outcome.Effects[0].Kind)
	s.Equal(refs.Conditions.Prone(), outcome.Effects[0].Ref)
	s.Equal(heroID, outcome.Effects[0].RecipientID)

	s.Equal(1, s.countProne(fixtures.sheet(out, heroID).Conditions),
		"the creature came back to be saved on the floor")
}

func (s *ObeyTestSuite) countProne(stored []json.RawMessage) int {
	found := 0
	for _, raw := range stored {
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		s.Require().NoError(json.Unmarshal(raw, &peek))
		if peek.Ref.Equals(refs.Conditions.Prone()) {
			found++
		}
	}

	return found
}

// A creature already on the floor is not knocked down twice: grovel goes
// through the same publish every condition goes through, so it inherits the
// same-ref replacement rather than restating it.
func (s *ObeyTestSuite) TestGrovellingWhileAlreadyProneLeavesOneProne() {
	fixtures := s.fixtures()
	standing, err := conditions.NewProneCondition(heroID).ToJSON()
	s.Require().NoError(err)

	out, err := Obey(s.ctx, &ObeyInput{
		Interaction: s.interaction(fixtures,
			Participant{Character: fixtures.saver(14, standing)}, Participant{Character: fixtures.bard(1)}),
		Member: heroID, CasterID: bardID, Word: wordGrovel, Source: commandedRef(),
	})
	s.Require().NoError(err)

	s.Equal([]ImposedEffectKind{ImposedConditionRemoved, ImposedCondition}, kindsOf(out.Effects),
		"the one they were carrying comes off before the new one lands")
	s.Equal(1, s.countProne(fixtures.sheet(out.Resolved, heroID).Conditions))
}

// FAIL CLOSED ON A WORD NOTHING CAN OBEY. A compelled turn that imposed nothing
// looks exactly like a turn nobody was compelled on, so an order this package
// has no arm for is refused before any interaction opens.
func (s *ObeyTestSuite) TestAnOrderNothingKnowsHowToObeyIsRefused() {
	for _, tc := range []struct {
		name     string
		in       *ObeyInput
		contains string
	}{
		{"a word with no arm", &ObeyInput{
			Interaction: &Input{}, Member: heroID, CasterID: bardID, Word: "halt",
		}, `"halt" is not a word`},
		{"no word at all", &ObeyInput{
			Interaction: &Input{}, Member: heroID, CasterID: bardID,
		}, `"" is not a word`},
		{"nobody compelled", &ObeyInput{
			Interaction: &Input{}, CasterID: bardID, Word: wordFlee,
		}, "nobody was compelled"},
		{"compelled by nobody", &ObeyInput{
			Interaction: &Input{}, Member: heroID, Word: wordFlee,
		}, "compelled by nobody"},
		{"a machine somebody else chose", &ObeyInput{
			Interaction: &Input{Machine: NewSave(&SaveInput{})}, Member: heroID,
			CasterID: bardID, Word: wordFlee,
		}, "its own machine"},
	} {
		s.Run(tc.name, func() {
			out, err := Obey(s.ctx, tc.in)
			s.Require().ErrorIs(err, ErrBadAction)
			s.Require().Contains(err.Error(), tc.contains)
			s.Nil(out)
		})
	}

	s.Run("no input at all", func() {
		_, err := Obey(s.ctx, nil)
		s.Require().ErrorIs(err, ErrNilInput)
	})

	s.Run("no interaction to obey inside", func() {
		_, err := Obey(s.ctx, &ObeyInput{Member: heroID, CasterID: bardID, Word: wordFlee})
		s.Require().ErrorIs(err, ErrNilInput)
	})
}

// The menu content authors and the arms this package holds are two lists that
// have to agree, and nothing in the type system makes them. This is what makes
// a disagreement loud: the failure otherwise is a creature standing still under
// an order nobody could execute.
//
// It runs OBEY rather than newObey, because the vocabulary is written twice —
// the door's refusal and the machine's switch — and a word admitted by one and
// unhandled by the other is exactly the drift this is for.
func (s *ObeyTestSuite) TestEveryWordOnCommandsMenuIsAWordThisPackageObeys() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Command, SpellSaveDC: 13})
	s.Require().NotNil(definition)
	s.Require().NotEmpty(definition.Cast.Options, "Command is the spell with a menu")

	for _, option := range definition.Cast.Options {
		s.Run(option.ID, func() {
			fixtures := s.fixtures()
			out, err := Obey(s.ctx, &ObeyInput{
				Interaction: s.interaction(fixtures,
					Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)}),
				Member: heroID, CasterID: bardID, Word: option.ID, Source: commandedRef(),
			})
			s.Require().NoError(err, "content offers %q and this package has no arm for it", option.ID)
			s.NotEmpty(out.Effects, "and every word imposes something")
		})
	}
}

// The turn budget travels the same road the other two do: content validates
// what may be declared, this layer adds the anchor, and a directive carrying it
// is accepted rather than refused by a validator that had not heard of it.
func (s *ObeyTestSuite) TestTheTurnBudgetSurvivesTheDirectiveValidator() {
	s.Require().NoError(validateMove(&MoveDirective{
		Policy: combatActions.MoveToward, AnchorID: bardID, Turn: true,
	}))

	s.Require().Error(validateMove(&MoveDirective{
		Policy: combatActions.MoveToward, AnchorID: bardID, Turn: true, Speed: true,
	}), "exactly one budget, and content is the one that says so")

	s.Require().ErrorIs(validateMove(&MoveDirective{
		Policy: combatActions.MoveToward, Turn: true,
	}), ErrBadAction, "the anchor is this layer's own field")
}
