// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

const (
	activationFighterID = "activation-fighter"
	activationAllyID    = "activation-ally"
)

// fixedRoller returns one scripted face for every die it is asked to roll,
// whatever the die size — the deterministic stand-in for the crypto-backed
// production roller an activation is handed in the field.
type fixedRoller struct{ face int }

func (r fixedRoller) Roll(_ context.Context, _ int) (int, error) { return r.face, nil }

func (r fixedRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	rolls := make([]int, count)
	for i := range rolls {
		rolls[i] = r.face
	}
	return rolls, nil
}

// calculationModifier returns a fresh pointer so a present zero stays a
// present zero and no fixture aliases another's pointer.
func calculationModifier(value int) *int { return &value }

// secondWindSourceRef returns a fresh copy of the canonical Second Wind ref so
// a mutation proof corrupts nothing shared with the refs package.
func secondWindSourceRef() *core.Ref {
	ref := *refs.Features.SecondWind()
	return &ref
}

// fighterLevelSourceRef returns a fresh copy of the canonical Fighter ref for
// the same reason secondWindSourceRef exists.
func fighterLevelSourceRef() *core.Ref {
	ref := *refs.Classes.Fighter()
	return &ref
}

// secondWindCalculation builds the complete sourced calculation the published
// root Second Wind feature produces for a fixed face-6 1d10 roll at fighter
// level 1: one dice trace, one present level modifier, authoritative total 7.
// Fresh literals and fresh refs on every call.
func secondWindCalculation() *dnd5eEvents.RollCalculation {
	return &dnd5eEvents.RollCalculation{
		Components: []dnd5eEvents.RollComponent{
			{
				Source: dnd5eEvents.RollSource{Ref: secondWindSourceRef(), Name: "Second Wind"},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      "1d10",
					DieSize:       10,
					OriginalRolls: []int{6},
					FinalRolls:    []int{6},
					Subtotal:      6,
				},
			},
			{
				Source: dnd5eEvents.RollSource{
					Ref: fighterLevelSourceRef(), Name: "Fighter", Label: "Fighter level",
				},
				Modifier: calculationModifier(1),
			},
		},
		Total: 7,
	}
}

type ActivationTestSuite struct {
	suite.Suite
	ctx context.Context
}

// TestActivationCalculationSuite is the activation machine's whole suite —
// conditions, capacity, refusals, and since roll traces the healing
// calculation — under one entry. The name carries "Calculation" so the
// focused command this task's plan runs (`Test.*Activation.*Calculation`)
// reaches every test here, not only the ones whose own names happen to say it.
func TestActivationCalculationSuite(t *testing.T) { suite.Run(t, new(ActivationTestSuite)) }

func (s *ActivationTestSuite) SetupTest() { s.ctx = context.Background() }

// barbarian is a level-1 barbarian with rage charges, in combat.
//
// IN COMBAT MATTERS: ActivateAbility answers "not in combat" to a sheet with a
// nil action economy, so a fixture without one would exercise the refusal path
// while looking like it exercised the ability.
func (s *ActivationTestSuite) barbarian(charges int) *character.Data {
	return &character.Data{
		ID: heroID, PlayerID: "player-1", Name: "Standre", Level: 1,
		ClassID: classes.Barbarian, RaceID: races.Human,
		ProficiencyBonus: 2,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 16,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 15, MaxHitPoints: 15, ArmorClass: 15,
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.RageCharges: {
				Current: charges, Maximum: 2, ResetType: coreResources.ResetLongRest,
			},
		},
		Features: []json.RawMessage{json.RawMessage(
			`{"ref":{"module":"dnd5e","type":"features","id":"rage"},` +
				`"id":"rage","name":"Rage","level":1}`)},
		ActionEconomy: &character.ActionEconomyData{
			TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
			ReactionsRemaining: 1, MovementRemaining: 30,
		},
	}
}

func (s *ActivationTestSuite) fighter(id string, hitPoints, maxHitPoints int) *character.Data {
	secondWind, err := json.Marshal(features.SecondWindData{
		Ref: refs.Features.SecondWind(), ID: id + "-second-wind", Name: "Second Wind",
		Level: 1, CharacterID: id, Uses: 1, MaxUses: 1,
	})
	s.Require().NoError(err)

	data := s.barbarian(0)
	data.ID = id
	data.PlayerID = id + "-player"
	data.Name = "Fighter"
	data.ClassID = classes.Fighter
	data.HitPoints = hitPoints
	data.MaxHitPoints = maxHitPoints
	data.Resources = nil
	data.Features = []json.RawMessage{secondWind}
	return data
}

func (s *ActivationTestSuite) world(members ...encounter.MemberInput) encounter.EncounterData {
	if len(members) == 0 {
		members = []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{},
		Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc.ToData()
}

func (s *ActivationTestSuite) run(
	in *ActivationInput, world encounter.EncounterData, participants []Participant,
) (*Output, error) {
	machine, err := NewActivation(in)
	s.Require().NoError(err)

	return Resolve(s.ctx, &Input{
		World:        world,
		Participants: participants,
		Initiative:   orderAsGiven{}, TurnDriver: passDriver{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Roller:  dice.NewRoller(),
		Machine: machine,
		// Cost stays nil ON PURPOSE — see NewActivation's doc. The ability
		// spends its own slot, and a Cost here would charge the same ledger
		// twice.
	})
}

// conditionRefs reads the condition refs off a sheet as it would be WRITTEN,
// not as it sits in memory.
//
// This is the whole point of the test below. Character.Load rebuilds conditions
// from these blobs, so a condition that never reached Data is a condition that
// does not survive the next load — and the in-memory object would have told us
// nothing about that.
func conditionRefs(data *character.Data) []string {
	out := make([]string, 0, len(data.Conditions))
	for _, raw := range data.Conditions {
		var envelope struct {
			Ref string `json:"ref"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}
		out = append(out, envelope.Ref)
	}
	return out
}

func dirty(out *Output, id string) *character.Data {
	for _, d := range out.DirtyCharacters {
		if d != nil && d.ID == id {
			return d
		}
	}
	return nil
}

// THE TEST THIS MACHINE EXISTS FOR.
//
// Rage does not attach its own condition: it publishes one on
// ConditionAppliedTopic and the owner's SheetKeeper applies it. Off the bus
// that publish SUCCEEDS and applies nothing, so the assertion has to be about
// the sheet that would be saved rather than about the call returning nil.
func (s *ActivationTestSuite) TestRagingReachesTheSheetThatWouldBeSaved() {
	out, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().NoError(err)
	s.Require().NotNil(out)

	sheet := dirty(out, heroID)
	s.Require().NotNil(sheet, "the barbarian should come back dirty")
	s.Contains(conditionRefs(sheet), "dnd5e:conditions:raging")

	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Equal([]ActivationEffect{{
		Kind: EffectConditionApplied, TargetID: heroID,
		Ref: refs.Conditions.Raging().String(), Name: "Raging",
	}}, outcome.Effects)
}

// The charge is spent on the sheet too, and from the same run. A rage that
// applied its condition without spending a use would be free forever.
func (s *ActivationTestSuite) TestTheChargeIsSpentOnTheSameSheet() {
	out, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().NoError(err)
	sheet := dirty(out, heroID)
	s.Require().NotNil(sheet)
	s.Equal(1, sheet.Resources[resources.RageCharges].Current)
}

// And the bonus action goes with it — the ability charging its own slot, which
// is why Input.Cost stays nil.
func (s *ActivationTestSuite) TestTheAbilitySpendsItsOwnSlot() {
	out, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().NoError(err)
	sheet := dirty(out, heroID)
	s.Require().NotNil(sheet)
	s.Require().NotNil(sheet.ActionEconomy)
	s.Equal(0, sheet.ActionEconomy.BonusActionsRemaining, "rage costs the bonus action")
	s.Equal(1, sheet.ActionEconomy.ActionsRemaining, "and nothing else")
}

// Dodge is the same journey with no resource in it: a combat ability rather
// than a feature, reached through the other arm of ActivateAbility's lookup.
func (s *ActivationTestSuite) TestDodgeReachesTheSheetTheSameWay() {
	out, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.CombatAbilities.Dodge(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().NoError(err)
	sheet := dirty(out, heroID)
	s.Require().NotNil(sheet)
	s.Contains(conditionRefs(sheet), "dnd5e:conditions:dodging")
	s.Equal(0, sheet.ActionEconomy.ActionsRemaining, "dodge costs the action")

	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Equal([]ActivationEffect{
		{
			Kind: EffectConditionApplied, TargetID: heroID,
			Ref: refs.Conditions.Dodging().String(), Name: "Dodging",
		},
		{
			Kind: EffectCapacityGranted, TargetID: heroID,
			Description: "dodging until next turn",
		},
	}, outcome.Effects, "returned capacity follows facts published during activation")
}

func (s *ActivationTestSuite) TestHelpCapturesTheSelectedAllyAsTheAffectedTarget() {
	ally := s.barbarian(2)
	ally.ID = activationAllyID
	ally.PlayerID = "ally-player"
	ally.Name = "Ally"
	world := s.world(
		encounter.MemberInput{ID: heroID, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: activationAllyID, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 2, Y: 1}},
	)

	out, err := s.run(
		&ActivationInput{
			MemberID: heroID, Ability: refs.CombatAbilities.Help(), TargetID: activationAllyID,
			Roller: dice.NewRoller(),
		},
		world,
		[]Participant{{Character: s.barbarian(2)}, {Character: ally}},
	)

	s.Require().NoError(err)
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Equal([]ActivationEffect{{
		Kind: EffectConditionApplied, TargetID: activationAllyID,
		Ref: refs.Conditions.Helped().String(), Name: "Helped",
	}}, outcome.Effects)
	s.Contains(conditionRefs(dirty(out, activationAllyID)), refs.Conditions.Helped().String())
}

func (s *ActivationTestSuite) TestDashAppendsCapacityAfterBusEffects() {
	out, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.CombatAbilities.Dash(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().NoError(err)
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Equal("30ft movement", outcome.GrantedCapacity)
	s.Equal([]ActivationEffect{{
		Kind: EffectCapacityGranted, TargetID: heroID, Description: "30ft movement",
	}}, outcome.Effects)
}

// THE HEADLINE FOR HEALING: the injected interaction roller scripts the roll,
// and the collector output owns the COMPLETE Second Wind calculation — dice
// trace, fighter-level modifier, authoritative total — not a flattened
// projection of it. No process-global randomness is touched to get here.
func (s *ActivationTestSuite) TestSecondWindActivationCapturesTheCompleteCalculation() {
	out, err := s.run(
		&ActivationInput{
			MemberID: activationFighterID, Ability: refs.Features.SecondWind(),
			Roller: fixedRoller{face: 6},
		},
		s.world(encounter.MemberInput{ID: activationFighterID, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}}),
		[]Participant{{Character: s.fighter(activationFighterID, 8, 10)}},
	)

	s.Require().NoError(err)
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Equal([]ActivationEffect{{
		Kind: EffectHealingApplied, TargetID: activationFighterID,
		Ref: refs.Features.SecondWind().String(), Name: "Second Wind",
		Amount: 2, Requested: 7, Before: 8, After: 10,
		Calculation: secondWindCalculation(),
	}}, outcome.Effects)
}

// A zero applied amount is still a fact: the requested healing and unchanged HP
// endpoints must not disappear just because the target was full, and the
// calculation survives the clamp alongside them.
func (s *ActivationTestSuite) TestSecondWindCapturesZeroAppliedWithoutDroppingRollFacts() {
	out, err := s.run(
		&ActivationInput{
			MemberID: activationFighterID, Ability: refs.Features.SecondWind(),
			Roller: fixedRoller{face: 6},
		},
		s.world(encounter.MemberInput{ID: activationFighterID, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}}),
		[]Participant{{Character: s.fighter(activationFighterID, 10, 10)}},
	)

	s.Require().NoError(err)
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Equal([]ActivationEffect{{
		Kind: EffectHealingApplied, TargetID: activationFighterID,
		Ref: refs.Features.SecondWind().String(), Name: "Second Wind",
		Amount: 0, Requested: 7, Before: 10, After: 10,
		Calculation: secondWindCalculation(),
	}}, outcome.Effects)
}

// A REFUSAL IS AN ERROR, AND NOTHING COMES BACK TO SAVE.
//
// The sheet answers "no rage uses remaining" as (output{Success:false}, nil) —
// a successful call carrying a false. Without the translation the interaction
// would finish, report Done, and hand back dirty sheets for something that
// never happened.
func (s *ActivationTestSuite) TestAnAbilityThatRefusesIsAnErrorNotADoneInteraction() {
	record := s.barbarian(0)
	before, marshalErr := json.Marshal(record)
	s.Require().NoError(marshalErr)

	out, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: record}},
	)

	s.Require().Error(err)
	s.Require().ErrorIs(err, ErrActivationRefused)
	s.Nil(out, "a refused activation returns no successful outcome or effects")
	// The ability's own words survive the crossing, the way a shortfall's
	// currency does.
	s.Contains(err.Error(), "no rage uses remaining")

	after, marshalErr := json.Marshal(record)
	s.Require().NoError(marshalErr)
	s.JSONEq(string(before), string(after),
		"loading and refusing must not mutate or alias the caller's persisted record")
}

// A refusal is NOT a malformed activation, and the two sentinels must not be
// reachable from one another — the ErrBadCost/ErrCannotPay line, applied to the
// same problem: a developer chasing ErrBadActivation must never be handed a
// barbarian who is simply out of rages.
func (s *ActivationTestSuite) TestARefusalIsNotAMalformedActivation() {
	_, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: s.barbarian(0)}},
	)

	s.Require().Error(err)
	s.False(errors.Is(err, ErrBadActivation))
}

func (s *ActivationTestSuite) TestAMemberWhoIsNotInTheCastIsRefused() {
	_, err := s.run(
		&ActivationInput{MemberID: "nobody", Ability: refs.Features.Rage(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "not a participant")
}

// A monster is IN the cast and still cannot declare: its abilities are driven
// by behaviour and its economy is not on its sheet. The refusal says which of
// the two problems it is, because "not a participant" would send somebody
// looking for a missing sheet that is right there.
func (s *ActivationTestSuite) TestAMonsterCannotDeclareAnActivation() {
	wolf := monsters.NewWolf(wolfID).ToData()
	world := s.world(
		encounter.MemberInput{ID: heroID, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: wolfID, Kind: encounter.KindMonster,
			Position: spatial.Position{X: 3, Y: 3}},
	)

	_, err := s.run(
		&ActivationInput{MemberID: wolfID, Ability: refs.CombatAbilities.Dodge(), Roller: dice.NewRoller()},
		world,
		[]Participant{{Character: s.barbarian(2)}, {Monster: wolf}},
	)

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "driven, not declared")
}

func (s *ActivationTestSuite) TestATargetWhoIsNotInTheCastIsRefused() {
	_, err := s.run(
		&ActivationInput{
			MemberID: heroID, Ability: refs.CombatAbilities.Help(), TargetID: "ghost",
			Roller: dice.NewRoller(),
		},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "target")
}

// --- Door refusals: before the world is loaded ---

func (s *ActivationTestSuite) TestTheDoorRefusesWhatNobodyCouldRun() {
	_, err := NewActivation(nil)
	s.Require().ErrorIs(err, ErrNilInput)

	_, err = NewActivation(&ActivationInput{Ability: refs.Features.Rage()})
	s.Require().ErrorIs(err, ErrBadActivation)

	_, err = NewActivation(&ActivationInput{MemberID: heroID})
	s.Require().ErrorIs(err, ErrBadActivation)

	// A ref that exists but names nothing — the shape a caller building one
	// from wire strings can actually produce.
	_, err = NewActivation(&ActivationInput{MemberID: heroID, Ability: &core.Ref{}})
	s.Require().ErrorIs(err, ErrBadActivation)

	// Everything else in order, but no roller: refused at the door, before the
	// world is loaded and before any payment — the same rule a machine which
	// rolls carries its own roller holds resolve.Input to.
	_, err = NewActivation(&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage()})
	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "roller")
}

// A machine that rolls carries its own roller, so a nil one is a caller defect
// caught at construction — never a silent fall back to process-global
// randomness mid-interaction.
func (s *ActivationTestSuite) TestTheDoorRefusesARollerlessActivation() {
	_, err := NewActivation(&ActivationInput{
		MemberID: activationFighterID, Ability: refs.Features.SecondWind(),
	})

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "roller")
}

// The roller is carried, not recreated: the machine stores the interface the
// caller supplied so the roll that happens during activation is the roll the
// caller scripted.
func (s *ActivationTestSuite) TestTheRollerIsCarriedNotRecreated() {
	roller := fixedRoller{face: 6}
	machine, err := NewActivation(&ActivationInput{
		MemberID: heroID, Ability: refs.Features.Rage(), Roller: roller,
	})
	s.Require().NoError(err)

	s.Equal(roller, machine.(*activationMachine).roller)
}

// The input is copied, so a caller that reuses its observer slice cannot change
// what this machine will roll against after it was constructed. Mirrors
// NewBoundary's own copy and the test that holds it.
func (s *ActivationTestSuite) TestTheInputIsCopied() {
	observers := []int{10, 12}
	in := &ActivationInput{
		MemberID: heroID, Ability: refs.CombatAbilities.Hide(),
		ObserverPassivePerceptions: observers,
		Roller:                     dice.NewRoller(),
	}
	machine, err := NewActivation(in)
	s.Require().NoError(err)

	observers[0] = 99

	s.Equal([]int{10, 12}, machine.(*activationMachine).observers)
}

// THE TRAP, MADE EXECUTABLE.
//
// This is the failure NewActivation exists to prevent, run deliberately: the
// same barbarian, the same ability, the same call — on a sheet that was LOADED
// but never ATTACHED. Rage's Activate guards its publish with
// `if input.Bus != nil`, so with no bus it skips the publish, spends the
// charge, and RETURNS NIL.
//
// Every signal a caller has says it worked. The call succeeded. Success is
// true. The charge came off. And the sheet that would be written carries no
// condition at all — which is the shape that bit the equip path (rpg-api#842)
// and the reason the assertions above are about Data rather than about the
// object in hand.
//
// If this test ever starts finding the condition, one of two good things
// happened — Rage learned to attach its own, or something else did — and the
// machine's whole justification needs re-reading. That is worth being told
// about, which is why this is a test and not a comment.
func (s *ActivationTestSuite) TestOffTheBusTheSameCallSucceedsAndAppliesNothing() {
	sheet, err := character.Load(s.ctx, s.barbarian(2))
	s.Require().NoError(err)

	out, err := sheet.ActivateAbility(s.ctx, &character.ActivateAbilityInput{
		AbilityRef: refs.Features.Rage(),
	})

	// Nothing complained.
	s.Require().NoError(err)
	s.Require().True(out.Success, "the sheet reports success with no bus to publish on")

	data := sheet.ToData()
	// The charge is gone, so something definitely happened...
	s.Equal(1, data.Resources[resources.RageCharges].Current)
	// ...and the barbarian is not raging.
	s.NotContains(conditionRefs(data), "dnd5e:conditions:raging")
}

// --- The target contract, which the doc promised before the code kept it ---

// Help takes somebody. A Help with nobody named is a malformed call, not an
// ability refusing.
func (s *ActivationTestSuite) TestAnAbilityThatNeedsATargetRefusesWithoutOne() {
	_, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.CombatAbilities.Help(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "needs a target")
}

// And the other way: a target on Dodge is REFUSED rather than ignored. A
// target quietly dropped is a client that believes it aimed Dodge at somebody
// and a server that knows better — a disagreement nobody finds until it
// matters.
func (s *ActivationTestSuite) TestAnUntargetedAbilityRefusesATarget() {
	_, err := s.run(
		&ActivationInput{
			MemberID: heroID, Ability: refs.CombatAbilities.Dodge(), TargetID: heroID,
			Roller: dice.NewRoller(),
		},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "takes no target")
}

// THE CONTRACT CHECK STAYS QUIET WHEN THE SHEET CANNOT ANSWER.
//
// AvailableAbilities is empty out of combat, so nothing can be looked up. That
// is an actor-state refusal, not a caller defect, and reporting it as
// ErrBadActivation would send somebody hunting a malformed request that does
// not exist — the confusion the two sentinels are there to prevent.
func (s *ActivationTestSuite) TestOutOfCombatIsARefusalNotAMalformedCall() {
	cold := s.barbarian(2)
	cold.ActionEconomy = nil

	_, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.CombatAbilities.Help(), Roller: dice.NewRoller()},
		s.world(),
		[]Participant{{Character: cold}},
	)

	s.Require().ErrorIs(err, ErrActivationRefused)
	s.False(errors.Is(err, ErrBadActivation))
	s.Contains(err.Error(), "not in combat")
}

// The ability ref is copied too, not just the observer slice. core.Ref is a
// mutable struct, so keeping the caller's pointer would let a reused ref change
// WHICH ABILITY this machine activates after it was built.
func (s *ActivationTestSuite) TestTheAbilityRefIsCopied() {
	ref := *refs.Features.Rage()
	machine, err := NewActivation(&ActivationInput{MemberID: heroID, Ability: &ref, Roller: dice.NewRoller()})
	s.Require().NoError(err)

	ref.ID = "second_wind"

	s.Equal("rage", machine.(*activationMachine).ability.ID)
}

// MUTATE EVERY PUBLISHER-OWNED FACT after synchronous publication: the source
// ref, every dice face, the subtotal, the modifier pointer, the total. What
// this interaction captured was cloned at capture time, so none of it lands.
func (s *ActivationTestSuite) TestActivationCollectorCapturesCalculationWithoutAliasing() {
	bus := events.NewEventBus()
	collector, err := newActivationEffectCollector(s.ctx, bus)
	s.Require().NoError(err)
	defer func() { s.Require().NoError(collector.Close(s.ctx)) }()

	source := *refs.Features.SecondWind()
	calculation := secondWindCalculation()
	s.Require().NoError(dnd5eEvents.HealingAppliedTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.HealingAppliedEvent{
			TargetID: heroID, Requested: 7, Applied: 2, HPBefore: 8, HPAfter: 10,
			SourceRef: &source, SourceName: "Second Wind", Calculation: calculation,
		}))

	source.ID = "tampered"
	calculation.Components[0].Source.Name = "tampered"
	calculation.Components[0].Dice.OriginalRolls[0] = 1
	calculation.Components[0].Dice.FinalRolls[0] = 1
	calculation.Components[0].Dice.Subtotal = 1
	*calculation.Components[1].Modifier = 9
	calculation.Total = 10

	s.Equal([]ActivationEffect{{
		Kind: EffectHealingApplied, TargetID: heroID,
		Ref: refs.Features.SecondWind().String(), Name: "Second Wind",
		Amount: 2, Requested: 7, Before: 8, After: 10,
		Calculation: secondWindCalculation(),
	}}, collector.Effects(),
		"the captured calculation is an owned clone, not the publisher's trace")
}

func (s *ActivationTestSuite) TestActivationCollectorCapturesConditionRemovedWithCatalogIdentity() {
	bus := events.NewEventBus()
	collector, err := newActivationEffectCollector(s.ctx, bus)
	s.Require().NoError(err)
	defer func() { s.Require().NoError(collector.Close(s.ctx)) }()

	err = dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.ConditionRemovedEvent{
			MemberID: heroID, ConditionRef: refs.Conditions.Dodging().String(),
			Reason: "turn started",
		})

	s.Require().NoError(err)
	s.Equal([]ActivationEffect{{
		Kind: EffectConditionRemoved, TargetID: heroID,
		Ref: refs.Conditions.Dodging().String(), Name: "Dodging", Reason: "turn started",
	}}, collector.Effects())
}

func (s *ActivationTestSuite) TestActivationCollectorPreservesBusOrderAndReturnsClones() {
	bus := events.NewEventBus()
	collector, err := newActivationEffectCollector(s.ctx, bus)
	s.Require().NoError(err)
	defer func() { s.Require().NoError(collector.Close(s.ctx)) }()

	source := *refs.Features.SecondWind()
	calculation := secondWindCalculation()
	s.Require().NoError(dnd5eEvents.HealingAppliedTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.HealingAppliedEvent{
			TargetID: heroID, Requested: 7, Applied: 2, HPBefore: 8, HPAfter: 10,
			SourceRef: &source, SourceName: "Second Wind", Calculation: calculation,
		}))
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.ConditionAppliedEvent{
			Target:    activationTestEntity{id: activationAllyID},
			Condition: conditions.NewDodgingCondition(activationAllyID),
		}))
	s.Require().NoError(dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.ConditionRemovedEvent{
			MemberID: activationAllyID, ConditionRef: refs.Conditions.Dodging().String(),
			Reason: "turn started",
		}))

	// Mutating the publisher-owned identity after synchronous publication cannot
	// rewrite the captured canonical identity. The full calculation mutation
	// proof lives in TestActivationCollectorCapturesCalculationWithoutAliasing.
	source.ID = "tampered"
	got := collector.Effects()
	s.Equal([]ActivationEffect{
		{
			Kind: EffectHealingApplied, TargetID: heroID,
			Ref: refs.Features.SecondWind().String(), Name: "Second Wind",
			Amount: 2, Requested: 7, Before: 8, After: 10,
			Calculation: secondWindCalculation(),
		},
		{
			Kind: EffectConditionApplied, TargetID: activationAllyID,
			Ref: refs.Conditions.Dodging().String(), Name: "Dodging",
		},
		{
			Kind: EffectConditionRemoved, TargetID: activationAllyID,
			Ref: refs.Conditions.Dodging().String(), Name: "Dodging", Reason: "turn started",
		},
	}, got)

	got[0].Name = "caller mutation"
	got[0].Calculation.Components[0].Dice.Subtotal = 999
	after := collector.Effects()
	s.Equal("Second Wind", after[0].Name,
		"Effects must return a defensive slice copy")
	s.Equal(6, after[0].Calculation.Components[0].Dice.Subtotal,
		"the captured calculation must not alias the returned slice's trace")
	s.Len(collector.subscriptionIDs, 3, "every typed subscription ID is retained for cleanup")
}

func (s *ActivationTestSuite) TestActivationCollectorRefusesHealingWithoutCanonicalSource() {
	tests := []struct {
		name       string
		sourceRef  *core.Ref
		sourceName string
	}{
		{name: "nil ref", sourceName: "Second Wind"},
		{name: "invalid ref", sourceRef: &core.Ref{}, sourceName: "Second Wind"},
		{name: "empty name", sourceRef: refs.Features.SecondWind()},
		{name: "whitespace-only name", sourceRef: refs.Features.SecondWind(), sourceName: " \t\n "},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			bus := events.NewEventBus()
			collector, err := newActivationEffectCollector(s.ctx, bus)
			s.Require().NoError(err)

			err = dnd5eEvents.HealingAppliedTopic.On(bus).Publish(s.ctx,
				dnd5eEvents.HealingAppliedEvent{
					TargetID: heroID, Requested: 7, Applied: 2, HPBefore: 8, HPAfter: 10,
					SourceRef: test.sourceRef, SourceName: test.sourceName,
					Calculation: secondWindCalculation(),
				})

			s.Require().Error(err)
			s.Empty(collector.Effects(), "an invalid fact must not leave a partial effect")
			s.Require().NoError(collector.Close(s.ctx))
		})
	}
}

// LEGACY HEALING IS NOT AN ACTIVATION RESULT.
//
// A heal published without a calculation — Hit Dice's scalar-only shape — is
// not something an ability activated, so the collector captures nothing for it
// and reports no error. The scalars are the legacy publisher's own record; the
// collector no longer mirrors them onto effects.
func (s *ActivationTestSuite) TestActivationCollectorSkipsLegacyHealingWithoutCalculation() {
	bus := events.NewEventBus()
	collector, err := newActivationEffectCollector(s.ctx, bus)
	s.Require().NoError(err)
	defer func() { s.Require().NoError(collector.Close(s.ctx)) }()

	source := secondWindSourceRef()
	err = dnd5eEvents.HealingAppliedTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.HealingAppliedEvent{
			TargetID: heroID, Requested: 7, Applied: 2, HPBefore: 8, HPAfter: 10,
			Roll: 6, Modifier: 1, SourceRef: source, SourceName: "Second Wind",
		})

	s.Require().NoError(err)
	s.Empty(collector.Effects(), "a scalar-only heal is not an activation result")
}

// A trace-bearing heal with an invalid calculation is refused whole: no partial
// effect, and the interaction's collector stays otherwise usable.
func (s *ActivationTestSuite) TestActivationCollectorRefusesAnInvalidHealingCalculation() {
	tests := []struct {
		name        string
		calculation *dnd5eEvents.RollCalculation
	}{
		{
			name:        "no components",
			calculation: &dnd5eEvents.RollCalculation{Total: 7},
		},
		{
			name: "total disagrees with components",
			calculation: func() *dnd5eEvents.RollCalculation {
				calc := secondWindCalculation()
				calc.Total = 9
				return calc
			}(),
		},
		{
			name: "subtotal disagrees with faces",
			calculation: func() *dnd5eEvents.RollCalculation {
				calc := secondWindCalculation()
				calc.Components[0].Dice.Subtotal = 5
				return calc
			}(),
		},
		{
			name: "reroll does not replay",
			calculation: func() *dnd5eEvents.RollCalculation {
				calc := secondWindCalculation()
				calc.Components[0].Dice.Rerolls = []dnd5eEvents.DiceReroll{{
					DieIndex: 0, Before: 2, After: 6,
					Source: dnd5eEvents.RollSource{Ref: secondWindSourceRef(), Name: "Second Wind"},
				}}
				return calc
			}(),
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			bus := events.NewEventBus()
			collector, err := newActivationEffectCollector(s.ctx, bus)
			s.Require().NoError(err)

			err = dnd5eEvents.HealingAppliedTopic.On(bus).Publish(s.ctx,
				dnd5eEvents.HealingAppliedEvent{
					TargetID: heroID, Requested: 7, Applied: 2, HPBefore: 8, HPAfter: 10,
					SourceRef: secondWindSourceRef(), SourceName: "Second Wind",
					Calculation: test.calculation,
				})

			s.Require().Error(err)
			s.Contains(err.Error(), "calculation")
			s.Empty(collector.Effects(), "an invalid fact must not leave a partial effect")
			s.Require().NoError(collector.Close(s.ctx))
		})
	}
}

func (s *ActivationTestSuite) TestActivationCollectorRefusesUnknownConditionDisplay() {
	bus := events.NewEventBus()
	collector, err := newActivationEffectCollector(s.ctx, bus)
	s.Require().NoError(err)
	defer func() { s.Require().NoError(collector.Close(s.ctx)) }()

	err = dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.ConditionAppliedEvent{
			Target: activationTestEntity{id: heroID},
			Condition: activationUnknownCondition{
				ref: core.Ref{Module: "dnd5e", Type: "conditions", ID: "unknown"},
			},
		})

	s.Require().Error(err)
	s.Contains(err.Error(), "display")
	s.Empty(collector.Effects())
}

func (s *ActivationTestSuite) TestActivationCollectorCloseJoinsAllErrorsAndIsIdempotent() {
	first := errors.New("first unsubscribe failed")
	second := errors.New("second unsubscribe failed")
	third := errors.New("third unsubscribe failed")
	bus := newActivationFaultBus()
	bus.unsubscribeErrs = []error{first, second, third}
	collector, err := newActivationEffectCollector(s.ctx, bus)
	s.Require().NoError(err)
	s.Len(bus.active, 3)

	err = collector.Close(s.ctx)

	s.Require().ErrorIs(err, first)
	s.Require().ErrorIs(err, second)
	s.Require().ErrorIs(err, third)
	s.Empty(bus.active, "every subscription is removed even when every removal reports an error")
	s.Equal([]string{
		collector.subscriptionIDs[2],
		collector.subscriptionIDs[1],
		collector.subscriptionIDs[0],
	}, bus.unsubscribeCalls, "subscriptions are revoked newest-first")

	secondCloseErr := collector.Close(s.ctx)
	s.Require().ErrorIs(secondCloseErr, first)
	s.Require().ErrorIs(secondCloseErr, second)
	s.Require().ErrorIs(secondCloseErr, third)
	s.Len(bus.unsubscribeCalls, 3, "idempotent Close does not unsubscribe twice")
}

func (s *ActivationTestSuite) TestActivationCollectorSubscribeFailurePreservesCleanupError() {
	subscribeErr := errors.New("condition subscription refused")
	cleanupErr := errors.New("collector rollback failed")
	bus := newActivationFaultBus()
	bus.failSubscribeAt = 2
	bus.subscribeErr = subscribeErr
	bus.unsubscribeErrs = []error{cleanupErr}

	collector, err := newActivationEffectCollector(s.ctx, bus)

	s.Nil(collector)
	s.Require().ErrorIs(err, subscribeErr)
	s.Require().ErrorIs(err, cleanupErr)
	s.Empty(bus.active, "the earlier successful subscription is rolled back")
	s.Equal(2, bus.subscribeCalls)
	s.Len(bus.unsubscribeCalls, 1)
}

func (s *ActivationTestSuite) TestActivationCollectorLeavesNoHandlerAfterClose() {
	bus := newActivationFaultBus()
	collector, err := newActivationEffectCollector(s.ctx, bus)
	s.Require().NoError(err)
	s.Require().NoError(collector.Close(s.ctx))
	s.Empty(bus.active)

	s.Require().NoError(dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.ConditionRemovedEvent{
			MemberID: heroID, ConditionRef: refs.Conditions.Dodging().String(), Reason: "late",
		}))
	s.Empty(collector.Effects(), "the interaction collector cannot observe after cleanup")
}

func (s *ActivationTestSuite) TestActivationSuccessPreservesCollectorCleanupError() {
	cleanupErr := errors.New("collector cleanup failed")
	bus := newActivationFaultBus()
	bus.unsubscribeErrs = []error{cleanupErr}
	machine, err := NewActivation(&ActivationInput{
		MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller(),
	})
	s.Require().NoError(err)

	out, err := resolveOn(s.ctx, &Input{
		World: s.world(), Participants: []Participant{{Character: s.barbarian(2)}},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Machine: machine,
	}, newSurface(bus))

	s.Nil(out, "cleanup failure cannot accompany a successful outcome")
	s.Require().ErrorIs(err, cleanupErr)
	s.Empty(bus.active)
}

func (s *ActivationTestSuite) TestActivationErrorJoinsCollectorCleanupError() {
	cleanupErr := errors.New("collector cleanup failed")
	bus := newActivationFaultBus()
	bus.unsubscribeErrs = []error{cleanupErr}
	machine, err := NewActivation(&ActivationInput{
		MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller(),
	})
	s.Require().NoError(err)

	out, err := resolveOn(s.ctx, &Input{
		World: s.world(), Participants: []Participant{{Character: s.barbarian(0)}},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Machine: machine,
	}, newSurface(bus))

	s.Nil(out)
	s.Require().ErrorIs(err, ErrActivationRefused)
	s.Require().ErrorIs(err, cleanupErr)
	s.Empty(bus.active)
}

type activationTestEntity struct{ id string }

func (e activationTestEntity) GetID() string          { return e.id }
func (activationTestEntity) GetType() core.EntityType { return "activation-test" }

type activationUnknownCondition struct{ ref core.Ref }

func (c activationUnknownCondition) Ref() *core.Ref { return &c.ref }
func (activationUnknownCondition) IsApplied() bool  { return true }
func (activationUnknownCondition) Apply(context.Context, events.EventBus) error {
	return nil
}
func (activationUnknownCondition) Remove(context.Context, events.EventBus) error {
	return nil
}
func (activationUnknownCondition) ToJSON() (json.RawMessage, error) {
	return json.RawMessage(`{"ref":"dnd5e:conditions:unknown"}`), nil
}

// activationFaultBus delegates to a real synchronous bus while making
// subscribe/unsubscribe failures and the live registration set observable.
type activationFaultBus struct {
	inner events.EventBus

	active           map[string]struct{}
	subscribeCalls   int
	failSubscribeAt  int
	subscribeErr     error
	unsubscribeErrs  []error
	unsubscribeCalls []string
}

func newActivationFaultBus() *activationFaultBus {
	return &activationFaultBus{inner: events.NewEventBus(), active: make(map[string]struct{})}
}

func (b *activationFaultBus) Subscribe(
	ctx context.Context, topic events.Topic, handler any,
) (string, error) {
	b.subscribeCalls++
	if b.failSubscribeAt != 0 && b.subscribeCalls == b.failSubscribeAt {
		return "", b.subscribeErr
	}

	id, err := b.inner.Subscribe(ctx, topic, handler)
	if err == nil {
		b.active[id] = struct{}{}
	}
	return id, err
}

func (b *activationFaultBus) Unsubscribe(ctx context.Context, id string) error {
	if _, ok := b.active[id]; !ok {
		return b.inner.Unsubscribe(ctx, id)
	}

	if err := b.inner.Unsubscribe(ctx, id); err != nil {
		return err
	}
	delete(b.active, id)
	b.unsubscribeCalls = append(b.unsubscribeCalls, id)

	if len(b.unsubscribeErrs) == 0 {
		return nil
	}
	err := b.unsubscribeErrs[0]
	b.unsubscribeErrs = b.unsubscribeErrs[1:]
	return err
}

func (b *activationFaultBus) Publish(
	ctx context.Context, topic events.Topic, event any,
) error {
	return b.inner.Publish(ctx, topic, event)
}
