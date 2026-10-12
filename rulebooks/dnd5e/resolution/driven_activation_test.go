// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// goblinID is the driven monster in these scenes: the creature rpg-toolkit#2032
// is about, whose Nimble Escape lets its driver spend a bonus action to
// Disengage. It sorts after the hero so the world's first turn is the hero's
// and loading it asks no monster sheets.
const goblinID = "nimble-goblin"

// drivenWorld puts a hero beside a goblin — adjacent, so the goblin's walk
// away is the step that provokes.
func (s *ActivationTestSuite) drivenWorld() encounter.EncounterData {
	return s.world(
		encounter.MemberInput{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: goblinID, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}},
	)
}

// drivenCast is the hero as a reactor (a reaction on the meter and the free
// opportunity attack on the sheet) and the goblin as the given sheet.
func (s *ActivationTestSuite) drivenCast(goblin *monster.Data) []Participant {
	return []Participant{{Character: reactorSheet(s.T(), heroID)}, {Monster: goblin}}
}

// drivenDisengage is the exact input the session seam sends once encounter
// has spent the goblin's bonus action.
func drivenDisengage() *ActivationInput {
	return &ActivationInput{
		MemberID: goblinID, Ability: refs.CombatAbilities.Disengage(),
		Driven: true, Roller: dice.NewRoller(),
	}
}

// monsterConditionRefs reads the condition refs off a monster sheet as it
// would be WRITTEN — the blob a reload rebuilds from — for conditionRefs'
// reason.
func monsterConditionRefs(data *monster.Data) []string {
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

// dirtyMonster finds one monster's sheet among the sheets an output handed
// back, failing the test when it came back clean.
func (s *ActivationTestSuite) dirtyMonster(out *Output, id string) *monster.Data {
	for _, sheet := range out.DirtyMonsters {
		if sheet.ID == id {
			return sheet
		}
	}
	s.FailNow("monster sheet came back clean", "%q is not among the dirty monsters", id)
	return nil
}

// THE DRIVEN ARM'S POINT: a monster whose driver already spent the economy
// Disengages through resolution, and the condition is on the sheet as it is
// WRITTEN — the monster side of the cast, the same place a Strike's monster
// state lands.
//
// The source is heard on the bus and pinned: the player's Disengage publishes
// with ConditionSourceCombatAbility, and parity is the claim.
func (s *ActivationTestSuite) TestADrivenMonsterDisengageLandsDisengagingOnItsSheet() {
	bus := events.NewEventBus()
	var heard []dnd5eEvents.ConditionSource
	_, err := dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			heard = append(heard, event.Source)
			return nil
		})
	s.Require().NoError(err)

	machine, err := NewActivation(drivenDisengage())
	s.Require().NoError(err)
	out, err := s.resolveOnBus(bus, machine, s.drivenWorld(),
		s.drivenCast(monsters.NewGoblin(goblinID).ToData()), nil)
	s.Require().NoError(err)
	s.Equal([]dnd5eEvents.ConditionSource{dnd5eEvents.ConditionSourceCombatAbility}, heard,
		"the player's Disengage source, and one application")

	goblin := s.dirtyMonster(out, goblinID)
	s.Equal([]string{refs.Conditions.Disengaging().String()}, monsterConditionRefs(goblin))

	got, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok, "a driven activation reports an activation outcome, the player arm's shape")
	s.Equal(refs.CombatAbilities.Disengage().String(), got.Ability)
	s.Empty(got.GrantedCapacity, "Disengage banks nothing")
	s.Require().Len(got.Effects, 1)
	s.Equal(EffectConditionApplied, got.Effects[0].Kind)
	s.Equal(goblinID, got.Effects[0].TargetID)
	s.Equal(refs.Conditions.Disengaging().String(), got.Effects[0].Ref)
	s.Equal("Disengaging", got.Effects[0].Name)
}

// The undriven door is unchanged: a monster that DECLARES is refused with the
// sentence it always was.
func (s *ActivationTestSuite) TestAnUndrivenMonsterDisengageIsStillRefused() {
	in := drivenDisengage()
	in.Driven = false

	_, err := s.run(in, s.drivenWorld(), s.drivenCast(monsters.NewGoblin(goblinID).ToData()))

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "driven, not declared")
}

// Fail closed: an ability with no driven arm yet is refused BY NAME, not run
// through the player arm and not silently skipped.
func (s *ActivationTestSuite) TestADrivenMonsterAbilityWithNoArmIsRefusedByName() {
	in := drivenDisengage()
	in.Ability = refs.CombatAbilities.Hide()

	out, err := s.run(in, s.drivenWorld(), s.drivenCast(monsters.NewGoblin(goblinID).ToData()))

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), refs.CombatAbilities.Hide().String())
	s.Contains(err.Error(), "not yet driven")
	s.Nil(out)
}

// "Charges nothing" is a law at the door, not a habit of the caller: a driven
// activation sent with a cost is refused before the world is loaded, and
// nobody is charged — the driver already spent it.
func (s *ActivationTestSuite) TestADrivenActivationWithACostIsRefused() {
	machine, err := NewActivation(drivenDisengage())
	s.Require().NoError(err)

	goblin := monsters.NewGoblin(goblinID).ToData()
	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.drivenWorld(), s.drivenCast(goblin),
		&Cost{PayerID: goblinID})

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "a driven activation carries no cost; its driver spent it")
	s.Nil(out)
}

// A character declares. Driven is a monster's door, and a character sent
// through it would skip the economy its own sheet charges.
func (s *ActivationTestSuite) TestADrivenCharacterActivationIsRefused() {
	in := drivenDisengage()
	in.MemberID = heroID

	_, err := s.run(in, s.drivenWorld(), s.drivenCast(monsters.NewGoblin(goblinID).ToData()))

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "characters declare")
}

// A driven Disengage names nobody, as the player's does: a target is a caller
// defect, refused rather than ignored.
func (s *ActivationTestSuite) TestADrivenDisengageWithATargetIsRefused() {
	in := drivenDisengage()
	in.TargetID = heroID

	_, err := s.run(in, s.drivenWorld(), s.drivenCast(monsters.NewGoblin(goblinID).ToData()))

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "takes no target")
}

// ONE CONDITION PER IDENTITY, mirrored from the player: a second Disengage at
// the same address REPLACES the first (the character keeper's rule). The
// monster keeper has no such guard (rpg-toolkit#1946), so the machine supplies
// it — the sheet ends with one Disengaging, and the effects say a replacement
// happened, in the order the player arm reports it.
func (s *ActivationTestSuite) TestASecondDrivenDisengageReplacesTheFirst() {
	first, err := s.run(drivenDisengage(), s.drivenWorld(), s.drivenCast(monsters.NewGoblin(goblinID).ToData()))
	s.Require().NoError(err)

	second, err := s.run(drivenDisengage(), s.drivenWorld(), s.drivenCast(s.dirtyMonster(first, goblinID)))
	s.Require().NoError(err)

	goblin := s.dirtyMonster(second, goblinID)
	s.Equal([]string{refs.Conditions.Disengaging().String()}, monsterConditionRefs(goblin),
		"one Disengaging, not two")

	got, ok := second.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Require().Len(got.Effects, 2)
	s.Equal(EffectConditionRemoved, got.Effects[0].Kind)
	s.Equal(ConditionReplacedReason, got.Effects[0].Reason)
	s.Equal(EffectConditionApplied, got.Effects[1].Kind)
}

// THE GUARANTEE THE PLAYER GETS: a goblin that Disengaged through the driven
// arm walks away from an adjacent hero holding a reaction, and no opportunity
// attack swings. The control half is the same walk without the activation,
// which DOES swing — so the silence is the condition's, not the scene's.
func (s *ActivationTestSuite) TestADrivenDisengagePreventsTheOpportunityAttack() {
	world := s.drivenWorld()
	walk := func(goblin *monster.Data) MovementOutcome {
		machine, err := NewMovement(&MovementInput{
			Mover: goblinID, MoverKind: "monster",
			From: spatial.Position{X: 2, Y: 1}, To: spatial.Position{X: 5, Y: 1},
			Reactions: &everyoneSwings{}, Roller: dice.NewRoller(),
		})
		s.Require().NoError(err)
		out, err := s.resolveMachine(machine, world, s.drivenCast(goblin))
		s.Require().NoError(err)
		moved, ok := out.Outcome.(MovementOutcome)
		s.Require().True(ok)
		return moved
	}

	control := walk(monsters.NewGoblin(goblinID).ToData())
	s.Require().Len(control.Reactions, 1, "precondition: the undisengaged walk provokes")

	activated, err := s.run(drivenDisengage(), world, s.drivenCast(monsters.NewGoblin(goblinID).ToData()))
	s.Require().NoError(err)

	moved := walk(s.dirtyMonster(activated, goblinID))
	s.Empty(moved.Reactions, "a Disengaging goblin provokes nothing")
	s.True(moved.OAPrevented, "the fold names the prevention")
}
