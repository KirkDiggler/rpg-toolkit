// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// targetHeldSuite covers the rules for effects an attack's target holds:
// keyed by reference, read from the frame, asked the same way by information
// and by the loaded condition's handler.
type targetHeldSuite struct{ ruleHelpers }

func TestTargetHeldSuite(t *testing.T) { suite.Run(t, new(targetHeldSuite)) }

var (
	ffHeld          = contributions.HeldCondition{Ref: refs.Conditions.FaerieFire().String(), SourceID: "cleric"}
	gbHeld          = contributions.HeldCondition{Ref: refs.Conditions.GuidingBolt().String(), SourceID: "cleric"}
	proneHeld       = contributions.HeldCondition{Ref: refs.Conditions.Prone().String()}
	sanctuaryHeld   = contributions.HeldCondition{Ref: refs.Conditions.Sanctuary().String(), SourceID: "cleric"}
	bladeWardHeld   = contributions.HeldCondition{Ref: refs.Conditions.BladeWard().String()}
	recklessHeld    = contributions.HeldCondition{Ref: refs.Conditions.RecklessAttack().String()}
	concentrateHeld = contributions.HeldCondition{Ref: refs.Conditions.Concentrating().String()}
)

// gobFrame is the base frame: the rogue attacks the goblin, which holds the
// given conditions, from one cell away and in sight.
func gobFrame(held ...contributions.HeldCondition) contributions.Frame {
	frame := rogueFrame(false)
	frame.Target = contributions.Known("gob")
	frame.Held = []contributions.MemberHeld{{Member: "gob", Conditions: held}}
	frame.Pairs = []contributions.PairFacts{{
		From: "rogue", To: "gob", DistanceCells: contributions.Known(1.0), Sees: contributions.Known(true),
	}}
	return frame
}

func (s *targetHeldSuite) heldAnswer(ref string, held contributions.HeldCondition, frame contributions.Frame) contributions.Answer {
	out, err := targetHeldRules[ref]("gob", held).AssessAction(&contributions.AssessActionInput{Frame: frame})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Require().NoError(validateAnswer(out.Answer, contributions.ContributesNow))
	return out.Answer
}

func (s *targetHeldSuite) ff(frame contributions.Frame) contributions.Answer {
	return s.heldAnswer(ffHeld.Ref, ffHeld, frame)
}

func (s *targetHeldSuite) TestFaerieFireHeldRuleAppliesWhenActorSeesTarget() {
	answer := s.ff(gobFrame(ffHeld))

	s.Equal(contributions.Applies, answer.Decision.Applicability)
	s.Equal("You can see the outlined target", answer.Decision.Reason)
	s.Equal("Advantage on the attack roll", answer.Benefit)
	s.Equal(contributions.AttackAdvantage, answer.AttackMode)
}

func (s *targetHeldSuite) TestFaerieFireHeldRuleDependsWhenSightUnknown() {
	frame := gobFrame(ffHeld)
	frame.Pairs[0].Sees = contributions.Unknown[bool]()

	answer := s.ff(frame)

	s.Equal(contributions.Depends, answer.Decision.Applicability)
	s.Equal("Depends on whether you can see the target", answer.Decision.Reason)
	s.Empty(answer.AttackMode)
}

func (s *targetHeldSuite) TestFaerieFireHeldRuleDoesNotApplyWhenActorCannotSee() {
	frame := gobFrame(ffHeld)
	frame.Pairs[0].Sees = contributions.Known(false)

	answer := s.ff(frame)

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("You cannot see the target", answer.Decision.Reason)
}

func (s *targetHeldSuite) TestHeldRuleDoesNotApplyToAnotherTarget() {
	frame := gobFrame(ffHeld)
	frame.Target = contributions.Known("orc")

	answer := s.ff(frame)

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Faerie Fire affects only attacks against its holder", answer.Decision.Reason)
}

func (s *targetHeldSuite) TestHeldRuleDependsWithoutTarget() {
	frame := gobFrame(ffHeld)
	frame.Target = contributions.Unknown[string]()

	answer := s.ff(frame)

	s.Equal(contributions.Depends, answer.Decision.Applicability)
	s.Equal("Depends on the target", answer.Decision.Reason)
}

func (s *targetHeldSuite) TestHeldRuleDependsWhenHoldingsUnknown() {
	frame := gobFrame()
	frame.Held = nil

	answer := s.ff(frame)

	s.Equal(contributions.Depends, answer.Decision.Applicability)
	s.Equal("Depends on what the target holds", answer.Decision.Reason)
}

func (s *targetHeldSuite) TestHeldRuleDoesNotApplyWhenFrameDoesNotListIt() {
	answer := s.ff(gobFrame(contributions.HeldCondition{Ref: ffHeld.Ref, SourceID: "other"}))

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("The target does not hold this effect", answer.Decision.Reason)
}

func (s *targetHeldSuite) TestHeldRuleDoesNotApplyToASavingThrow() {
	frame := gobFrame(ffHeld)
	frame.Action.Roll = contributions.Known(contributions.RollKindSavingThrow)

	answer := s.ff(frame)

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Faerie Fire affects only attack rolls", answer.Decision.Reason)
}

func (s *targetHeldSuite) TestGuidingBoltDodgingHiddenHeldRules() {
	for name, tc := range map[string]struct {
		held    contributions.HeldCondition
		reason  string
		benefit string
		mode    contributions.AttackMode
	}{
		"guiding bolt": {gbHeld, "The target is lit by Guiding Bolt", "Advantage on the attack roll", contributions.AttackAdvantage},
		"dodging":      {dodgingHeld, "The target is dodging", "Disadvantage on the attack roll", contributions.AttackDisadvantage},
		"hidden":       {hiddenHeld, "The target is hidden", "Disadvantage on the attack roll", contributions.AttackDisadvantage},
	} {
		s.Run(name, func() {
			answer := s.heldAnswer(tc.held.Ref, tc.held, gobFrame(tc.held))
			s.Equal(contributions.Applies, answer.Decision.Applicability)
			s.Equal(tc.reason, answer.Decision.Reason)
			s.Equal(tc.benefit, answer.Benefit)
			s.Equal(tc.mode, answer.AttackMode)
		})
	}
}

func (s *targetHeldSuite) TestProneHeldRuleSplitsAtFiveFeet() {
	near := s.heldAnswer(proneHeld.Ref, proneHeld, gobFrame(proneHeld))
	s.Equal(contributions.Applies, near.Decision.Applicability)
	s.Equal("The prone target is within 5 feet", near.Decision.Reason)
	s.Equal(contributions.AttackAdvantage, near.AttackMode)

	farFrame := gobFrame(proneHeld)
	farFrame.Pairs[0].DistanceCells = contributions.Known(2.0)
	far := s.heldAnswer(proneHeld.Ref, proneHeld, farFrame)
	s.Equal(contributions.Applies, far.Decision.Applicability)
	s.Equal("The prone target is beyond 5 feet", far.Decision.Reason)
	s.Equal("Disadvantage on the attack roll", far.Benefit)
	s.Equal(contributions.AttackDisadvantage, far.AttackMode)

	unknownFrame := gobFrame(proneHeld)
	unknownFrame.Pairs = nil
	unknown := s.heldAnswer(proneHeld.Ref, proneHeld, unknownFrame)
	s.Equal(contributions.Depends, unknown.Decision.Applicability)
	s.Equal("Depends on how far you are from the target", unknown.Decision.Reason)
}

func (s *targetHeldSuite) TestSanctuaryHeldRuleAppliesToAnotherAttacker() {
	answer := s.heldAnswer(sanctuaryHeld.Ref, sanctuaryHeld, gobFrame(sanctuaryHeld))

	s.Equal(contributions.Applies, answer.Decision.Applicability)
	s.Equal("The target is warded by Sanctuary", answer.Decision.Reason)
	s.Equal("Wisdom saving throw first; on a failure the attack is lost", answer.Benefit)
	s.Empty(answer.AttackMode, "the save is resolution's to run; Sanctuary adds no attack mode")
}

func (s *targetHeldSuite) TestSanctuaryHeldRuleDoesNotStopHoldersOwnAttack() {
	frame := gobFrame(sanctuaryHeld)
	frame.Actor = "gob"

	answer := s.heldAnswer(sanctuaryHeld.Ref, sanctuaryHeld, frame)

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Your own ward does not stop your attack", answer.Decision.Reason)
}

func (s *targetHeldSuite) TestValidateAnswerRefusesModeWhenNotApplying() {
	answer := contributions.Answer{
		Decision:      contributions.Decision{Applicability: contributions.DoesNotApply, Reason: "no"},
		Participation: contributions.ContributesNow,
		AttackMode:    contributions.AttackAdvantage,
	}
	s.Error(validateAnswer(answer, contributions.ContributesNow))

	answer.Decision.Applicability = contributions.Applies
	answer.AttackMode = "sideways"
	s.Error(validateAnswer(answer, contributions.ContributesNow), "an unknown mode is a producer defect")
}

func (s *targetHeldSuite) TestAssessTargetHeldEffectsListsBearingHeldEffects() {
	ff := contributions.HeldCondition{Ref: ffHeld.Ref, SourceID: "c1"}
	out, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{
		Frame: gobFrame(ff, concentrateHeld, recklessHeld, proneHeld),
	})
	s.Require().NoError(err)

	_, found := effectByID(out.Effects, heldEffectID(concentrateHeld))
	s.False(found, "Concentrating does not bear on an attack against its holder")

	ids := make([]string, 0, len(out.Effects))
	for _, effect := range out.Effects {
		ids = append(ids, effect.ID)
	}
	s.Equal([]string{heldEffectID(ff), heldEffectID(recklessHeld), heldEffectID(proneHeld)}, ids,
		"the bearing rows, in the frame's held order")

	row, _ := effectByID(out.Effects, "target:dnd5e:conditions:faerie_fire@c1")
	s.Equal(contributions.StateApplies, row.State)
	s.Equal("c1", row.Source.SourceID)
	s.Equal(FaerieFireName, row.Source.Name)
	s.Equal(displayCatalog[ffHeld.Ref].Detail, row.Description)
	s.Equal("Advantage on the attack roll", row.Benefit)

	row, _ = effectByID(out.Effects, "target:"+recklessHeld.Ref)
	s.Equal(contributions.StateApplies, row.State)
	s.Equal("The target is attacking recklessly", row.Reason)
	s.Equal("Advantage on the attack roll", row.Benefit)
	s.Equal(displayCatalog[recklessHeld.Ref].Detail, row.Description)

	row, _ = effectByID(out.Effects, "target:"+proneHeld.Ref)
	s.Equal(contributions.StateApplies, row.State)
	s.Equal("The prone target is within 5 feet", row.Reason)
}

// TestNotYetAnsweringHeldEffectIsUnavailable: a held effect the census marks
// as bearing but not yet answering shows an unavailable row with its
// description, never dropped and never not-applying. No shipped target-held
// ref is in that class today, so the census entry is swapped for the test.
func (s *targetHeldSuite) TestNotYetAnsweringHeldEffectIsUnavailable() {
	original := targetCensus[recklessHeld.Ref]
	targetCensus[recklessHeld.Ref] = notYetAnswering
	s.T().Cleanup(func() { targetCensus[recklessHeld.Ref] = original })

	out, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{Frame: gobFrame(recklessHeld)})
	s.Require().NoError(err)

	row, found := effectByID(out.Effects, heldEffectID(recklessHeld))
	s.Require().True(found)
	s.Equal(contributions.StateUnavailable, row.State)
	s.Equal(unavailableReason, row.Reason)
	s.Equal(displayCatalog[recklessHeld.Ref].Detail, row.Description)
	s.Empty(row.Benefit)
}

// effectByID finds the row with id among effects.
func effectByID(effects []contributions.Effect, id string) (contributions.Effect, bool) {
	for _, effect := range effects {
		if effect.ID == id {
			return effect, true
		}
	}
	return contributions.Effect{}, false
}

// TestTargetDefencesYieldNoRow is R21: a target's armour class and
// resistances are not the attacker's to know, so a target holding only effects
// that bear through them shows the attacker no row at all.
func (s *targetHeldSuite) TestTargetDefencesYieldNoRow() {
	for _, ref := range []string{
		refs.Conditions.Raging().String(),
		refs.Conditions.BladeWard().String(),
		refs.Conditions.ShieldOfFaith().String(),
		refs.Conditions.UnarmoredDefense().String(),
		refs.Conditions.FightingStyleDefense().String(),
		refs.Spells.Shield().String(),
	} {
		s.Run(ref, func() {
			out, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{
				Frame: gobFrame(contributions.HeldCondition{Ref: ref, SourceID: "someone"}),
			})
			s.Require().NoError(err)
			s.Empty(out.Effects)
			s.NotContains(TargetBearingRefs(), ref)
		})
	}
}

func (s *targetHeldSuite) TestAssessTargetHeldEffectsUnknownHoldingsYieldNoRows() {
	frame := gobFrame()
	frame.Held = nil

	out, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{Frame: frame})

	s.Require().NoError(err)
	s.Empty(out.Effects)
}

func (s *targetHeldSuite) TestAssessTargetHeldEffectsRequiresKnownTarget() {
	frame := gobFrame(ffHeld)
	frame.Target = contributions.Unknown[string]()

	_, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{Frame: frame})
	s.Error(err)

	_, err = AssessTargetHeldEffects(nil)
	s.Error(err)
	_, err = AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{})
	s.Error(err, "an invalid frame is an error")
}

func (s *targetHeldSuite) TestAssessTargetHeldEffectsRefusesUnclassifiedRef() {
	_, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{
		Frame: gobFrame(contributions.HeldCondition{Ref: "dnd5e:conditions:totally_unknown"}),
	})
	s.Error(err)
}

func (s *targetHeldSuite) TestTargetHeldIDsNeverEqualActorRowIDs() {
	frame := gobFrame(proneHeld)
	frame.Target = contributions.Known("gob")

	actor, err := AssessActionEffects(&AssessActionEffectsInput{
		Conditions: []dnd5eEvents.ConditionBehavior{NewProneCondition("rogue")}, Frame: frame,
	})
	s.Require().NoError(err)
	held, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{Frame: frame})
	s.Require().NoError(err)

	s.Require().Len(actor.Effects, 1)
	s.Require().Len(held.Effects, 1)
	s.NotEqual(actor.Effects[0].ID, held.Effects[0].ID, "both prone, two different rows")
}

func (s *targetHeldSuite) TestAssessingHeldEffectsSpendsNothing() {
	bolt, err := NewGuidingBoltCondition(NewGuidingBoltConditionInput{
		MemberID: "gob", SourceID: "cleric", SourceRef: refs.Spells.GuidingBolt(),
	})
	s.Require().NoError(err)
	before, err := bolt.ToJSON()
	s.Require().NoError(err)

	_, err = AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{Frame: gobFrame(heldOf("gob", bolt))})
	s.Require().NoError(err)
	_, err = bolt.heldRule().AssessAction(&contributions.AssessActionInput{Frame: gobFrame(heldOf("gob", bolt))})
	s.Require().NoError(err)

	after, err := bolt.ToJSON()
	s.Require().NoError(err)
	s.JSONEq(string(before), string(after))
}

func (s *targetHeldSuite) attackOn(bus events.EventBus, frame contributions.Frame) (dnd5eEvents.AttackChainEvent, error) {
	return s.publishAttack(bus, dnd5eEvents.AttackChainEvent{AttackerID: "rogue", TargetID: "gob", Frame: frame})
}

func (s *targetHeldSuite) TestFaerieFireHandlerReadsFrameSight() {
	bus := events.NewEventBus()
	ff, err := NewFaerieFireCondition(NewFaerieFireConditionInput{MemberID: "gob", SourceID: "cleric", SourceRef: refs.Spells.FaerieFire()})
	s.Require().NoError(err)
	s.Require().NoError(ff.Apply(context.Background(), bus))

	seen, err := s.attackOn(bus, gobFrame(ffHeld))
	s.Require().NoError(err)
	s.Require().Len(seen.AdvantageSources, 1)
	s.Equal(refs.Conditions.FaerieFire(), seen.AdvantageSources[0].SourceRef)
	s.Equal("cleric", seen.AdvantageSources[0].SourceID)

	blind := gobFrame(ffHeld)
	blind.Pairs[0].Sees = contributions.Known(false)
	unseen, err := s.attackOn(bus, blind)
	s.Require().NoError(err)
	s.Empty(unseen.AdvantageSources)
}

func (s *targetHeldSuite) TestFaerieFireHandlerFailsWhenSightUnknown() {
	bus := events.NewEventBus()
	ff, err := NewFaerieFireCondition(NewFaerieFireConditionInput{MemberID: "gob", SourceID: "cleric", SourceRef: refs.Spells.FaerieFire()})
	s.Require().NoError(err)
	s.Require().NoError(ff.Apply(context.Background(), bus))
	frame := gobFrame(ffHeld)
	frame.Pairs[0].Sees = contributions.Unknown[bool]()

	_, err = s.attackOn(bus, frame)

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
}

func (s *targetHeldSuite) TestProneTargetHandlerReadsFrameDistance() {
	bus := events.NewEventBus()
	s.Require().NoError(NewProneCondition("gob").Apply(context.Background(), bus))

	near, err := s.attackOn(bus, gobFrame(proneHeld))
	s.Require().NoError(err)
	s.Require().Len(near.AdvantageSources, 1)
	s.Equal("Prone target within 5 feet", near.AdvantageSources[0].Reason)
	s.Empty(near.DisadvantageSources)

	farFrame := gobFrame(proneHeld)
	farFrame.Pairs[0].DistanceCells = contributions.Known(3.0)
	far, err := s.attackOn(bus, farFrame)
	s.Require().NoError(err)
	s.Require().Len(far.DisadvantageSources, 1)
	s.Equal("Prone target beyond 5 feet", far.DisadvantageSources[0].Reason)
	s.Empty(far.AdvantageSources)
}

func (s *targetHeldSuite) TestProneTargetHandlerFailsWithoutDistance() {
	bus := events.NewEventBus()
	s.Require().NoError(NewProneCondition("gob").Apply(context.Background(), bus))
	frame := gobFrame(proneHeld)
	frame.Pairs = nil

	_, err := s.attackOn(bus, frame)

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
}

// heldHandlers are the loaded conditions whose attack-chain handler asks a
// held rule, each held by "gob".
func (s *targetHeldSuite) heldHandlers() map[string]dnd5eEvents.ConditionBehavior {
	ff, err := NewFaerieFireCondition(NewFaerieFireConditionInput{MemberID: "gob", SourceID: "cleric", SourceRef: refs.Spells.FaerieFire()})
	s.Require().NoError(err)
	gb, err := NewGuidingBoltCondition(NewGuidingBoltConditionInput{MemberID: "gob", SourceID: "cleric", SourceRef: refs.Spells.GuidingBolt()})
	s.Require().NoError(err)
	return map[string]dnd5eEvents.ConditionBehavior{
		"faerie fire":   ff,
		"guiding bolt":  gb,
		"dodging":       NewDodgingCondition("gob"),
		"prone target":  NewProneCondition("gob"),
		"hidden target": NewHiddenCondition("gob"),
	}
}

func (s *targetHeldSuite) TestHeldHandlersRejectZeroFrame() {
	for name, condition := range s.heldHandlers() {
		s.Run(name, func() {
			bus := events.NewEventBus()
			s.Require().NoError(condition.Apply(context.Background(), bus))

			final, err := s.attackOn(bus, contributions.Frame{})

			s.Require().Error(err)
			s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
			s.Empty(final.AdvantageSources)
			s.Empty(final.DisadvantageSources)
		})
	}
}

func (s *targetHeldSuite) TestHeldHandlerSourcesImportNoGameContext() {
	for _, file := range []string{"faerie_fire.go", "prone.go"} {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		s.Require().NoError(err, file)
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			s.Require().NoError(err)
			s.NotContains(path, "gamectx", "%s reads the frame, not the game context", file)
		}
	}
}

// heldRuler is a loaded condition's own held rule constructor.
type heldRuler interface {
	heldRule() contributions.ActionAssessor
}

func (s *targetHeldSuite) TestHandlerAndRegistryAskTheSameHeldRule() {
	frames := map[string]func(contributions.HeldCondition) contributions.Frame{
		"base": func(h contributions.HeldCondition) contributions.Frame { return gobFrame(h) },
		"blind": func(h contributions.HeldCondition) contributions.Frame {
			f := gobFrame(h)
			f.Pairs[0].Sees = contributions.Known(false)
			return f
		},
		"far": func(h contributions.HeldCondition) contributions.Frame {
			f := gobFrame(h)
			f.Pairs[0].DistanceCells = contributions.Known(4.0)
			return f
		},
		"another target": func(h contributions.HeldCondition) contributions.Frame {
			f := gobFrame(h)
			f.Target = contributions.Known("orc")
			return f
		},
		"unknown holdings": func(contributions.HeldCondition) contributions.Frame {
			f := gobFrame()
			f.Held = nil
			return f
		},
	}
	for name, condition := range s.heldHandlers() {
		ruler, ok := condition.(heldRuler)
		s.Require().True(ok, name)
		held := heldOf("gob", condition)
		registry, ok := targetHeldRules[held.Ref]
		s.Require().True(ok, name)
		for frameName, frameOf := range frames {
			frame := frameOf(held)
			handler, err := ruler.heldRule().AssessAction(&contributions.AssessActionInput{Frame: frame})
			s.Require().NoError(err)
			byRef, err := registry("gob", held).AssessAction(&contributions.AssessActionInput{Frame: frame})
			s.Require().NoError(err)
			s.Equal(byRef.Answer, handler.Answer, "%s / %s", name, frameName)
		}
	}
}

func (s *targetHeldSuite) TestExecuteHeldEffectWrapsDepends() {
	frame := gobFrame(ffHeld)
	frame.Pairs[0].Sees = contributions.Unknown[bool]()

	_, err := ExecuteHeldEffect(&ExecuteHeldEffectInput{Holder: "gob", Held: ffHeld, Frame: frame})
	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))

	_, err = ExecuteHeldEffect(&ExecuteHeldEffectInput{Holder: "gob", Held: ffHeld, Frame: contributions.Frame{}})
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer), "an invalid frame cannot be answered from")

	out, err := ExecuteHeldEffect(&ExecuteHeldEffectInput{Holder: "gob", Held: sanctuaryHeld, Frame: gobFrame(sanctuaryHeld)})
	s.Require().NoError(err)
	s.Equal(contributions.Applies, out.Answer.Decision.Applicability)
}

func (s *targetHeldSuite) TestExecuteHeldEffectRefusesRefWithoutRule() {
	_, err := ExecuteHeldEffect(&ExecuteHeldEffectInput{Holder: "gob", Held: bladeWardHeld, Frame: gobFrame(bladeWardHeld)})
	s.Require().Error(err)
	s.False(errors.Is(err, contributions.ErrRuleCannotAnswer), "a ref with no rule is a caller defect, not a missing fact")

	_, err = ExecuteHeldEffect(nil)
	s.Error(err)
}

// TestExecutionRefusesAFrameOmittingTheHandlersOwnAddress is R13 through the
// held list: at execution the loaded condition proves its holder holds it, so
// a frame that lists the holder without this address — none at all, or the
// same condition from another source — fails the attack rather than switching
// the effect off. Information keeps its gate: there a stale sighting is
// legitimate testimony, and the rule answers does not apply.
func (s *targetHeldSuite) TestExecutionRefusesAFrameOmittingTheHandlersOwnAddress() {
	for name, tc := range map[string]struct {
		condition func() dnd5eEvents.ConditionBehavior
		held      contributions.HeldCondition
	}{
		"faerie fire": {func() dnd5eEvents.ConditionBehavior {
			ff, err := NewFaerieFireCondition(NewFaerieFireConditionInput{MemberID: "gob", SourceID: "cleric", SourceRef: refs.Spells.FaerieFire()})
			s.Require().NoError(err)
			return ff
		}, ffHeld},
		"guiding bolt": {func() dnd5eEvents.ConditionBehavior {
			gb, err := NewGuidingBoltCondition(NewGuidingBoltConditionInput{MemberID: "gob", SourceID: "cleric", SourceRef: refs.Spells.GuidingBolt()})
			s.Require().NoError(err)
			return gb
		}, gbHeld},
	} {
		otherSource := tc.held
		otherSource.SourceID = "someone-else"
		for frameName, frame := range map[string]contributions.Frame{
			"holds nothing":       gobFrame(),
			"another source only": gobFrame(otherSource),
		} {
			s.Run(name+"/"+frameName, func() {
				bus := events.NewEventBus()
				condition := tc.condition()
				s.Require().NoError(condition.Apply(context.Background(), bus))

				final, err := s.attackOn(bus, frame)

				s.Require().Error(err)
				s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
				s.Empty(final.AdvantageSources)
				s.True(condition.IsApplied(), "nothing was spent on an attack that failed")

				_, err = ExecuteHeldEffect(&ExecuteHeldEffectInput{Holder: "gob", Held: tc.held, Frame: frame})
				s.True(errors.Is(err, contributions.ErrRuleCannotAnswer), "the exported execute refuses it too")

				out, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{Frame: frame})
				s.Require().NoError(err)
				for _, row := range out.Effects {
					s.NotEqual(heldEffectID(tc.held), row.ID, "information shows no row for what the sighting does not list")
				}
			})
		}
	}
}
