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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type ActivationTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestActivationSuite(t *testing.T) { suite.Run(t, new(ActivationTestSuite)) }

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
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage()},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().NoError(err)
	s.Require().NotNil(out)

	sheet := dirty(out, heroID)
	s.Require().NotNil(sheet, "the barbarian should come back dirty")
	s.Contains(conditionRefs(sheet), "dnd5e:conditions:raging")
}

// The charge is spent on the sheet too, and from the same run. A rage that
// applied its condition without spending a use would be free forever.
func (s *ActivationTestSuite) TestTheChargeIsSpentOnTheSameSheet() {
	out, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage()},
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
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage()},
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
		&ActivationInput{MemberID: heroID, Ability: refs.CombatAbilities.Dodge()},
		s.world(),
		[]Participant{{Character: s.barbarian(2)}},
	)

	s.Require().NoError(err)
	sheet := dirty(out, heroID)
	s.Require().NotNil(sheet)
	s.Contains(conditionRefs(sheet), "dnd5e:conditions:dodging")
	s.Equal(0, sheet.ActionEconomy.ActionsRemaining, "dodge costs the action")
}

// A REFUSAL IS AN ERROR, AND NOTHING COMES BACK TO SAVE.
//
// The sheet answers "no rage uses remaining" as (output{Success:false}, nil) —
// a successful call carrying a false. Without the translation the interaction
// would finish, report Done, and hand back dirty sheets for something that
// never happened.
func (s *ActivationTestSuite) TestAnAbilityThatRefusesIsAnErrorNotADoneInteraction() {
	out, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage()},
		s.world(),
		[]Participant{{Character: s.barbarian(0)}},
	)

	s.Require().Error(err)
	s.Require().ErrorIs(err, ErrActivationRefused)
	s.Nil(out, "a refused activation saves nothing")
	// The ability's own words survive the crossing, the way a shortfall's
	// currency does.
	s.Contains(err.Error(), "no rage uses remaining")
}

// A refusal is NOT a malformed activation, and the two sentinels must not be
// reachable from one another — the ErrBadCost/ErrCannotPay line, applied to the
// same problem: a developer chasing ErrBadActivation must never be handed a
// barbarian who is simply out of rages.
func (s *ActivationTestSuite) TestARefusalIsNotAMalformedActivation() {
	_, err := s.run(
		&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage()},
		s.world(),
		[]Participant{{Character: s.barbarian(0)}},
	)

	s.Require().Error(err)
	s.False(errors.Is(err, ErrBadActivation))
}

func (s *ActivationTestSuite) TestAMemberWhoIsNotInTheCastIsRefused() {
	_, err := s.run(
		&ActivationInput{MemberID: "nobody", Ability: refs.Features.Rage()},
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
		&ActivationInput{MemberID: wolfID, Ability: refs.CombatAbilities.Dodge()},
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
}

// The input is copied, so a caller that reuses its observer slice cannot change
// what this machine will roll against after it was constructed. Mirrors
// NewBoundary's own copy and the test that holds it.
func (s *ActivationTestSuite) TestTheInputIsCopied() {
	observers := []int{10, 12}
	in := &ActivationInput{
		MemberID: heroID, Ability: refs.CombatAbilities.Hide(),
		ObserverPassivePerceptions: observers,
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
