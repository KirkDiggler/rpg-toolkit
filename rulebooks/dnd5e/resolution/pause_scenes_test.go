// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// heroStepsPastWolf announces the hero stepping out of the wolf's reach, in
// the action world, with the wolf's own opportunity attack attached for real.
func heroStepsPastWolf(t *testing.T, hero *character.Data, wolf *monster.Data, machine Machine) (*Output, error) {
	t.Helper()
	return resolveOn(context.Background(), &Input{
		World:        actionWorld(t, 2),
		Participants: []Participant{{Monster: wolf}, {Character: hero}},
		Machine:      machine,
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
			Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Driver: passDriver{},
			Roller: dice.NewRoller(), Actors: Actors,
		},
	}, newSurface(events.NewEventBus()))
}

// heroStep is the hero's step out of the wolf's reach, swung at by anybody
// the capability lets swing.
func heroStep(t *testing.T, reactions ReactionAttacks, roller dice.Roller) Machine {
	t.Helper()
	machine, err := NewMovement(&MovementInput{
		Mover: heroID, MoverKind: "character",
		From: spatial.Position{X: 2, Y: 1}, To: spatial.Position{X: 5, Y: 1},
		Reactions: reactions, Roller: roller,
	})
	require.NoError(t, err)
	return machine
}

// dirtyMonster is the wolf as the output handed it back to be saved.
func dirtyMonster(t *testing.T, out *Output, id string) *monster.Data {
	t.Helper()
	for _, data := range out.DirtyMonsters {
		if data.ID == id {
			return data
		}
	}
	require.Failf(t, "no dirty monster", "%q did not come back to be saved", id)
	return nil
}

// TestAMonstersOpportunityAttackSpendsItsReactionAtTheDoor: the wolf swings
// at a hero leaving its reach, and the door charges its one reaction through
// Monster.SpendReaction — exactly once, or the second charge would have failed
// the walk.
func TestAMonstersOpportunityAttackSpendsItsReactionAtTheDoor(t *testing.T) {
	out, err := heroStepsPastWolf(t, actionHero(), monsters.NewWolf(wolfID).ToData(),
		heroStep(t, &everyoneSwings{}, &actionRoller{singles: []int{15}, damage: [][]int{{3}}}))
	require.NoError(t, err)
	moved := out.Outcome.(MovementOutcome)
	require.Len(t, moved.Reactions, 1, "the wolf swung")
	require.Equal(t, wolfID, moved.Reactions[0].ReactorID)

	spent := dirtyMonster(t, out, wolfID)
	require.True(t, spent.ReactionSpent, "the meter is on the monster's own sheet")
	reloaded, err := monster.LoadFromData(context.Background(), spent, events.NewEventBus())
	require.NoError(t, err)
	require.False(t, reloaded.CanReact())
}

// wrathMover is the hero holding Wrath of the Storm, stepping away from the
// wolf: the wolf's opportunity attack lands and the hero reacts to the hit.
func wrathMover(t *testing.T) *character.Data {
	t.Helper()
	return wrathHero(t)
}

// TestAPostHitInsideAMovementReportsTheSettledReactionWithThePause: the
// wolf's opportunity attack hits, and the hero's post-hit reaction pauses the
// step. The hit is told with the pause, on the movement outcome; resuming
// tells only the continued half — and bills the wolf, once.
func TestAPostHitInsideAMovementReportsTheSettledReactionWithThePause(t *testing.T) {
	out, err := heroStepsPastWolf(t, wrathMover(t), monsters.NewWolf(wolfID).ToData(),
		heroStep(t, &everyoneSwings{}, &actionRoller{singles: []int{15}, damage: [][]int{{3}}}))
	require.NoError(t, err)
	require.NotNil(t, out.Posed)
	require.Equal(t, PausePostHit, out.Posed.Kind)
	h, err := readFrozen(out.Posed.Frozen)
	require.NoError(t, err)
	require.Equal(t, machineMovement, h.Machine)

	settled, ok := out.Outcome.(MovementOutcome)
	require.True(t, ok, "the settled reaction is the paused output's outcome")
	require.Len(t, settled.Reactions, 1)
	require.Equal(t, wolfID, settled.Reactions[0].ReactorID)
	require.True(t, settled.Reactions[0].Struck.Hit, "the hit settled before the pause")
	require.Positive(t, settled.Reactions[0].Struck.Damage, "and dealt its damage")

	machine, err := Resume(&ResumeInput{Pause: *out.Posed, Answer: Decline(), Roller: dice.NewRoller()})
	require.NoError(t, err)
	var hero *character.Data
	for _, sheet := range out.DirtyCharacters {
		if sheet.ID == heroID {
			hero = sheet
		}
	}
	require.NotNil(t, hero)
	resumed, err := heroStepsPastWolf(t, hero, monsters.NewWolf(wolfID).ToData(), machine)
	require.NoError(t, err)
	require.Nil(t, resumed.Posed)
	moved := resumed.Outcome.(MovementOutcome)
	require.Len(t, moved.Reactions, 1, "only the paused reaction's continuation")
	require.True(t, moved.Reactions[0].Struck.Continued)
	require.Zero(t, moved.Reactions[0].Struck.Damage, "the hit is not told twice")
	require.True(t, dirtyMonster(t, resumed, wolfID).ReactionSpent, "the wolf pays when its reaction finishes")
}

// postHitOnce subscribes a post-hit reaction for the target of the FIRST hit
// on this bus, and none after: one window, so the scene pauses exactly once.
func postHitOnce(t *testing.T, bus events.EventBus) {
	t.Helper()
	offered := false
	_, err := dnd5eEvents.PostHitChain.On(bus).SubscribeWithChain(context.Background(),
		func(_ context.Context, e *dnd5eEvents.PostHitEvent,
			c chain.Chain[*dnd5eEvents.PostHitEvent],
		) (chain.Chain[*dnd5eEvents.PostHitEvent], error) {
			if offered {
				return c, nil
			}
			offered = true
			e.Offers = append(e.Offers, dnd5eEvents.PostHitOffer{
				ReactorID: e.TargetID, Ref: *refs.Conditions.Dodging(), Name: "a test reaction",
				Options: []dnd5eEvents.PostHitOption{{
					ID: "zap", Label: "Zap", Ability: abilities.DEX, DC: 10, Dice: "1d4", DamageType: damage.Lightning,
				}},
			})
			return c, nil
		})
	require.NoError(t, err)
}

// pausedMultiattack is a two-swing multiattack whose first swing hits a
// concentrating caster holding Fog Cloud, who fails the check, and then poses
// a post-hit reaction.
func (s *ConcentrationTestSuite) pausedMultiattack() (*Output, encounter.EncounterData) {
	hold := conditions.NewConcentratingCondition(heroID, refs.Spells.FogCloud().String(), "Fog Cloud", 600)
	holdJSON, err := hold.ToJSON()
	s.Require().NoError(err)

	world := s.fixtures().world()
	world.SightAreas = []encounter.SightAreaData{fogCloudArea(heroID)}
	bus := events.NewEventBus()
	postHitOnce(s.T(), bus)
	machine, err := NewAction(&ActionInput{
		Definition: twoClawSequence, AttackerID: wolfID, TargetID: heroID,
		Components: []combatActions.Definition{twoClawSequence, claw("1d6")},
		// The first claw's d20, then the concentration save's: 3 + CON 2
		// fails DC 10.
		Roller: &sequenceRoller{singles: []int{straightRoll, straightRoll}, pair: []int{6}},
	})
	s.Require().NoError(err)
	out, err := resolveOn(s.ctx, &Input{
		World:        world,
		Participants: []Participant{{Character: s.fixtures().saver(40, holdJSON)}, {Monster: s.fixtures().wolfData()}},
		Machine:      machine,
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{}, Driver: passDriver{}, Standing: everyoneStanding{},
			Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
			Roller: dice.NewRoller(), Actors: Actors,
		},
	}, newSurface(bus))
	s.Require().NoError(err)
	return out, world
}

// TestAPostHitInsideASequenceReportsTheSettledSwingWithThePause: the swing
// that hit, the check it forced and the hold it broke are all told at the
// pause, on the paused output's sequence; the area that hold kept up closes
// at the pause.
func (s *ConcentrationTestSuite) TestAPostHitInsideASequenceReportsTheSettledSwingWithThePause() {
	out, _ := s.pausedMultiattack()
	s.Require().NotNil(out.Posed)
	s.Equal(PausePostHit, out.Posed.Kind)

	sequence, ok := out.Outcome.(SequenceOutcome)
	s.Require().True(ok, "the settled swing is the paused output's outcome")
	s.Equal(0, sequence.From)
	s.Require().Len(sequence.Steps, 1)
	first := sequence.Steps[0]
	s.True(first.Strike.Hit)
	s.Equal(6, first.Strike.Damage)
	s.Require().Len(first.ConcentrationBreaks, 1, "the hold broke on the swing that hit, told with it")
	s.Equal(encounter.MemberID(heroID), first.ConcentrationBreaks[0].Caster)
	s.Require().NotNil(first.ConcentrationBreaks[0].Save, "the failed check rides its break")
	s.Empty(first.ConcentrationChecks, "a failed check is recorded as the break it caused")
	s.Empty(out.ConcentrationBreaks, "a sequence's record rides its steps")
	s.Equal([]string{heroID}, out.ClosedAreas, "the area closes at the pause")
}

// TestTheResumeReportsOnlyWhatSettledAfterThePause: resuming that pause with
// Decline() tells the paused swing's continued half — every hit field zero —
// then the second swing, and nothing that was told at the pause.
func (s *ConcentrationTestSuite) TestTheResumeReportsOnlyWhatSettledAfterThePause() {
	out, _ := s.pausedMultiattack()
	s.Require().NotNil(out.Posed)

	// The second claw's d20 and damage: the resumed machine rolls with the
	// roller it was resumed with.
	machine, err := Resume(&ResumeInput{Pause: *out.Posed, Answer: Decline(),
		Roller: &sequenceRoller{singles: []int{straightRoll}, pair: []int{4}}})
	s.Require().NoError(err)
	var hero *character.Data
	for _, sheet := range out.DirtyCharacters {
		if sheet.ID == heroID {
			hero = sheet
		}
	}
	s.Require().NotNil(hero, "the broken hold left the sheet")

	resumed, err := resolveOn(s.ctx, &Input{
		World:        out.World,
		Participants: []Participant{{Character: hero}, {Monster: s.fixtures().wolfData()}},
		Machine:      machine,
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{}, Driver: passDriver{}, Standing: everyoneStanding{},
			Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
			Roller: &sequenceRoller{singles: []int{straightRoll}, pair: []int{4}},
			Actors: Actors,
		},
	}, newSurface(events.NewEventBus()))
	s.Require().NoError(err)
	s.Require().Nil(resumed.Posed)

	sequence := resumed.Outcome.(SequenceOutcome)
	s.Equal(0, sequence.From, "the resume starts at the swing that paused")
	s.Require().Len(sequence.Steps, 2)
	continued := sequence.Steps[0].Strike
	s.True(continued.Continued)
	s.Equal(heroID, continued.TargetID)
	s.Equal(wolfID, continued.AttackerID)
	s.False(continued.Hit)
	s.Zero(continued.Damage)
	s.Zero(continued.Roll)
	s.Nil(continued.Calculation)
	s.Empty(continued.FollowUps)
	s.Empty(sequence.Steps[0].ConcentrationBreaks, "the break was told at the pause")
	s.True(sequence.Steps[1].Strike.Hit, "then the second swing")
	s.Empty(resumed.ClosedAreas, "the area closed at the pause, not again")
}

// askEveryone answers every reactor with a question.
type askEveryone struct{}

func (askEveryone) AttackFor(string) (combatActions.Definition, ReactionAnswer) {
	return validMeleeDefinition(), ReactionAsk
}

// askedStep is the wolf stepping out of alice's and zara's reach, with both
// players asked.
func (s *MovementTestSuite) askedStep() *Output {
	in := s.stepInput()
	in.Reactions = askEveryone{}
	out, err := s.runStep(in, nil, "alice", "zara")
	s.Require().NoError(err)
	return out
}

// TestAnOpportunityAskIsAPauseWithItsPrice: the step finishes as if both
// declined, with one PauseOpportunity per asked player, all standing at once,
// each priced at one reaction and naming what offers it.
func (s *MovementTestSuite) TestAnOpportunityAskIsAPauseWithItsPrice() {
	out := s.askedStep()
	s.Nil(out.Posed, "an ask does not stop the step")
	moved := out.Outcome.(MovementOutcome)
	s.Empty(moved.Reactions, "nobody swung yet")
	s.Require().Len(moved.Asked, 2)
	for i, reactor := range []string{"alice", "zara"} {
		asked := moved.Asked[i]
		s.Equal(PauseOpportunity, asked.Kind)
		s.Equal(reactor, asked.Ask.Audience)
		s.Equal("Opportunity Attack", asked.Ask.Offer.Name)
		s.Equal(refs.Conditions.OpportunityAttack().String(), asked.Ask.Offer.Ref.String(),
			"the offer names its source")
		s.Equal(reactionCost(reactor, ""), asked.Cost, "one reaction")
		h, err := readFrozen(asked.Frozen)
		s.Require().NoError(err)
		s.Equal(machineOpportunity, h.Machine)
	}
	s.Equal(reactorReactions, reactionsLeft(out, "alice"), "asking costs nothing")
	s.Equal(reactorReactions, reactionsLeft(out, "zara"))
}

// TestTakingAnOpportunitySpendsTheReactorsReaction: alice takes hers. She
// swings, exactly one of her reactions is spent through the door, the
// reaction names its source, and zara — asked too, answering nothing — keeps
// hers.
func (s *MovementTestSuite) TestTakingAnOpportunitySpendsTheReactorsReaction() {
	asked := s.askedStep().Outcome.(MovementOutcome).Asked[0]

	machine, err := Resume(&ResumeInput{Pause: asked, Answer: Take(""), Roller: dice.NewRoller()})
	s.Require().NoError(err)
	out, err := s.runMachine(machine, nil, "alice", "zara")
	s.Require().NoError(err)

	moved := out.Outcome.(MovementOutcome)
	s.Require().Len(moved.Reactions, 1)
	s.Equal("alice", moved.Reactions[0].ReactorID)
	s.Equal(wolfID, moved.Reactions[0].Against)
	s.Equal(refs.Conditions.OpportunityAttack().String(), moved.Reactions[0].ConditionRef)
	s.Equal("Opportunity Attack", moved.Reactions[0].ConditionName)
	s.Empty(moved.Asked, "a taken question asks nobody again")
	s.Equal(reactorReactions-1, reactionsLeft(out, "alice"), "exactly one reaction")
	s.Equal(reactorReactions, reactionsLeft(out, "zara"), "only the payer is billed")
}

// TestDecliningAnOpportunitySpendsNothing: the step is told with nothing to
// report, and nobody's meter moves.
func (s *MovementTestSuite) TestDecliningAnOpportunitySpendsNothing() {
	asked := s.askedStep().Outcome.(MovementOutcome).Asked[0]

	machine, err := Resume(&ResumeInput{Pause: asked, Answer: Decline(), Roller: dice.NewRoller()})
	s.Require().NoError(err)
	out, err := s.runMachine(machine, nil, "alice", "zara")
	s.Require().NoError(err)

	moved := out.Outcome.(MovementOutcome)
	s.Equal(wolfID, moved.Mover)
	s.Empty(moved.Reactions)
	s.Equal(reactorReactions, reactionsLeft(out, "alice"))
	s.Equal(reactorReactions, reactionsLeft(out, "zara"))
}

// TestASaveOfferInsideACastResumesTheCast: the cast pauses on its target's
// save offer — a save_roll question, held by the contest, held by the cast —
// tells nothing at the pause because a cast is one unit, and the resume tells
// the cast once with every target.
func (s *ResistancePoseTestSuite) TestASaveOfferInsideACastResumesTheCast() {
	fixtures := s.fixtures()
	saver := fixtures.saver(14, s.resistanceJSON(heroID, "cleric-1"))
	roller := &countingCastRoller{facedRoller: facedRoller{d20: 10, other: 4}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{heroID, wolfID}, Roller: roller,
	})
	s.Require().NoError(err)

	out, err := fixtures.resolve(saver, machine, baneCost(), baneCaster(1, 2))
	s.Require().NoError(err)
	s.Require().NotNil(out.Posed)
	s.Nil(out.Outcome, "a cast is one told unit, told when it finishes")
	s.Equal(PauseSaveRoll, out.Posed.Kind)
	h, err := readFrozen(out.Posed.Frozen)
	s.Require().NoError(err)
	s.Equal(machineCast, h.Machine)

	resumed, err := Resume(&ResumeInput{Pause: *out.Posed, Answer: Take(""), Roller: roller})
	s.Require().NoError(err)
	resumedOut, err := fixtures.resolve(saver, resumed, nil, baneCaster(1, 2))
	s.Require().NoError(err)
	s.Require().Nil(resumedOut.Posed)
	outcome := resumedOut.Outcome.(CastOutcome)
	s.Require().Len(outcome.Targets, 2, "every target, told once")
	s.Equal(heroID, outcome.Targets[0].TargetID)
	s.Equal(wolfID, outcome.Targets[1].TargetID)
}

// headerOf reads a frozen blob's header and fails the test on anything stale
// or unreadable.
func headerOf(t *testing.T, frozen []byte) frozenHeader {
	t.Helper()
	h, err := readFrozen(frozen)
	require.NoError(t, err)
	require.Equal(t, PauseVersion, h.V)
	return h
}

// innerOf reads a container's inner header out of its state, by field name.
func innerOf(t *testing.T, h frozenHeader, field string) frozenHeader {
	t.Helper()
	var state map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(h.State, &state))
	raw, ok := state[field]
	require.True(t, ok, "%s holds no %q", h.Machine, field)
	return headerOf(t, raw)
}

// resistingAttacker is a character standing where the wolf stands, holding a
// Resistance die: a retaliation's save against it is offered that die.
func resistingAttacker(t *testing.T) *character.Data {
	t.Helper()
	attacker := actionHero()
	attacker.ID = wolfID
	held, err := conditions.NewResistanceCondition(conditions.NewResistanceConditionInput{
		MemberID: wolfID, SourceID: "cleric-1", SourceRef: refs.Spells.Resistance(),
	})
	require.NoError(t, err)
	raw, err := held.ToJSON()
	require.NoError(t, err)
	attacker.Conditions = []json.RawMessage{raw}
	return attacker
}

// TestEveryPauseWritesOneHeader drives each of the eleven freeze sites and
// reads the one header each wrote — the current version, the machine that
// froze it, and the innermost question as its kind. A container's inner pause
// is a whole header of its own, read the same way.
func TestEveryPauseWritesOneHeader(t *testing.T) {
	type site struct {
		machine string
		kind    PauseKind
	}
	check := func(t *testing.T, pause Pause, want site) frozenHeader {
		t.Helper()
		h := headerOf(t, pause.Frozen)
		require.Equal(t, want.machine, h.Machine)
		require.Equal(t, want.kind, h.Kind)
		require.Equal(t, want.kind, pause.Kind, "the pause states the header's kind")
		return h
	}
	hit := func() *actionRoller {
		return &actionRoller{singles: []int{15}, pairs: [][]int{{15, 2}}, damage: [][]int{{3}}}
	}

	t.Run("strike before its roll", func(t *testing.T) {
		out, err := wolfStrikesHero(t, flareHero(t), NewStrike(&StrikeInput{
			AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(), Roller: hit()}))
		require.NoError(t, err)
		require.NotNil(t, out.Posed)
		check(t, *out.Posed, site{machineBeforeRoll, PauseBeforeRoll})
	})

	t.Run("strike on its d20", func(t *testing.T) {
		check(t, postRollPause(t), site{machinePostRoll, PausePostRoll})
	})

	t.Run("strike after its hit", func(t *testing.T) {
		out, err := wolfStrikesHero(t, wrathHero(t), NewStrike(&StrikeInput{
			AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(), Roller: hit()}))
		require.NoError(t, err)
		require.NotNil(t, out.Posed)
		check(t, *out.Posed, site{machinePostHit, PausePostHit})
	})

	t.Run("a sequence holding a swing", func(t *testing.T) {
		out, err := resolveSequenceAgainst(t, twoClaws(), []combatActions.Definition{validMeleeDefinition()}, flareHero(t), hit())
		require.NoError(t, err)
		require.NotNil(t, out.Posed)
		h := check(t, *out.Posed, site{machineSequence, PauseBeforeRoll})
		inner := innerOf(t, h, "inner")
		require.Equal(t, machineBeforeRoll, inner.Machine)
	})

	t.Run("a step holding a reaction", func(t *testing.T) {
		out, err := heroStepsPastWolf(t, flareHero(t), monsters.NewWolf(wolfID).ToData(),
			heroStep(t, &everyoneSwings{}, hit()))
		require.NoError(t, err)
		require.NotNil(t, out.Posed)
		h := check(t, *out.Posed, site{machineMovement, PauseBeforeRoll})
		require.Equal(t, machineBeforeRoll, innerOf(t, h, "inner").Machine)
	})

	t.Run("a cast holding a contest holding a save", func(t *testing.T) {
		suite := &ResistancePoseTestSuite{ctx: context.Background()}
		suite.SetT(t)
		fixtures := suite.fixtures()
		machine, err := NewAction(&ActionInput{
			Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{heroID},
			Roller: &countingCastRoller{facedRoller: facedRoller{d20: 10, other: 4}},
		})
		require.NoError(t, err)
		out, err := fixtures.resolve(fixtures.saver(14, suite.resistanceJSON(heroID, "cleric-1")), machine, baneCost(), baneCaster(1, 2))
		require.NoError(t, err)
		require.NotNil(t, out.Posed)
		cast := check(t, *out.Posed, site{machineCast, PauseSaveRoll})
		contest := innerOf(t, cast, "inner")
		require.Equal(t, machineContest, contest.Machine)
		require.Equal(t, PauseSaveRoll, contest.Kind)
		save := innerOf(t, contest, "save")
		require.Equal(t, machineSave, save.Machine)
		require.Equal(t, PauseSaveRoll, save.Kind)
	})

	t.Run("a check", func(t *testing.T) {
		suite := &CheckPoseTestSuite{ctx: context.Background(), roller: &scriptedRoller{single: straightRoll}}
		suite.SetT(t)
		out, err := suite.check(suite.checker(suite.guided()))
		require.NoError(t, err)
		require.NotNil(t, out.Posed)
		check(t, *out.Posed, site{machineCheck, PauseCheckRoll})
	})

	t.Run("a retaliation holding its save", func(t *testing.T) {
		strike := func(machine Machine) *Output {
			out, err := resolveOn(context.Background(), &Input{
				World:        actionWorld(t, 2),
				Participants: []Participant{{Character: resistingAttacker(t)}, {Character: wrathHero(t)}},
				Machine:      machine,
				Capabilities: encounter.Capabilities{
					Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
					Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Driver: passDriver{},
					Roller: dice.NewRoller(), Actors: Actors,
				},
			}, newSurface(events.NewEventBus()))
			require.NoError(t, err)
			require.NotNil(t, out.Posed)
			return out
		}
		posed := strike(NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(), Roller: hit()}))
		check(t, *posed.Posed, site{machinePostHit, PausePostHit})

		machine, err := Resume(&ResumeInput{Pause: *posed.Posed, Answer: Take("lightning"), Roller: dice.NewRoller()})
		require.NoError(t, err)
		retaliating := strike(machine)
		h := check(t, *retaliating.Posed, site{machinePostHit, PauseSaveRoll})
		require.Nil(t, retaliating.Posed.Cost, "the reaction was paid when it was taken")
		contest := innerOf(t, h, "retaliation")
		require.Equal(t, machineContest, contest.Machine)
		require.Equal(t, machineSave, innerOf(t, contest, "save").Machine)
	})

	t.Run("an opportunity asked", func(t *testing.T) {
		out, err := heroStepsPastWolf(t, actionHero(), monsters.NewWolf(wolfID).ToData(),
			heroStep(t, askEveryone{}, dice.NewRoller()))
		require.NoError(t, err)
		require.Nil(t, out.Posed)
		asked := out.Outcome.(MovementOutcome).Asked
		require.Len(t, asked, 1)
		check(t, asked[0], site{machineOpportunity, PauseOpportunity})
	})
}

// TestARetaliationsSaveOfferResumesTheRetaliation: a taken post-hit reaction
// whose retaliation save is itself offered a die pauses again, at no price,
// and resuming that save finishes the strike's continued half with the
// retaliation's result — the hit is not told again.
func TestARetaliationsSaveOfferResumesTheRetaliation(t *testing.T) {
	strike := func(machine Machine) *Output {
		out, err := resolveOn(context.Background(), &Input{
			World:        actionWorld(t, 2),
			Participants: []Participant{{Character: resistingAttacker(t)}, {Character: wrathHero(t)}},
			Machine:      machine,
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
				Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Driver: passDriver{},
				Roller: dice.NewRoller(), Actors: Actors,
			},
		}, newSurface(events.NewEventBus()))
		require.NoError(t, err)
		return out
	}
	posed := strike(NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(),
		Roller: &actionRoller{singles: []int{15}, damage: [][]int{{3}}}}))
	require.NotNil(t, posed.Posed)
	taken, err := Resume(&ResumeInput{Pause: *posed.Posed, Answer: Take("lightning"), Roller: dice.NewRoller()})
	require.NoError(t, err)
	retaliating := strike(taken)
	require.NotNil(t, retaliating.Posed)
	require.Equal(t, PauseSaveRoll, retaliating.Posed.Kind)
	require.Nil(t, retaliating.Outcome, "nothing settled between the two pauses")

	finished, err := Resume(&ResumeInput{Pause: *retaliating.Posed, Answer: Decline(), Roller: dice.NewRoller()})
	require.NoError(t, err)
	out := strike(finished)
	require.Nil(t, out.Posed)
	struck := out.Outcome.(StrikeOutcome)
	require.True(t, struck.Continued)
	require.Zero(t, struck.Damage, "the hit was told at the first pause")
	require.NotNil(t, struck.Retaliation, "the retaliation finished")
	require.Equal(t, wolfID, struck.Retaliation.TargetID)
	require.NotNil(t, struck.Retaliation.Result.Save)
}
