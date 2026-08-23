// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	monsterActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// DeclaredConsequenceTestSuite covers the half of a rider that used to be
// stranded in the machine: WHAT a failed save costs, as opposed to what the
// save is (rpg-toolkit#1013).
//
// The wolf's own knockdown is pinned end to end by the existing headline test
// TestAWolfBitesTheHeroAndKnocksHimDown, which is unmodified and still passes
// — that is the "nothing regressed" half. These are the cases it could never
// catch, because prone was the only answer the machine could give.
type DeclaredConsequenceTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestDeclaredConsequenceSuite(t *testing.T) {
	suite.Run(t, new(DeclaredConsequenceTestSuite))
}

func (s *DeclaredConsequenceTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// biteProfile is the wolf's bite as its own compiler produces it.
func (s *DeclaredConsequenceTestSuite) biteProfile() AttackProfile {
	data := monsters.NewWolf(wolfID).ToData()
	s.Require().Len(data.Actions, 1)

	profile, err := AttackFromMonsterAction(data.Actions[0])
	s.Require().NoError(err)

	return profile
}

// world places the hero and the wolf three cells apart — far enough that
// prone's range predicate stays out of these cases.
func (s *DeclaredConsequenceTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 8, Y: 5}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// hero is a sheet with a +5 STR save: DC 11 is a real contest either way.
func (s *DeclaredConsequenceTestSuite) hero() *character.Data {
	return &character.Data{
		ID:       heroID,
		PlayerID: "player-1",
		Name:     "Grog",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:        14,
		MaxHitPoints:     14,
		ArmorClass:       14,
		ProficiencyBonus: 2,
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
		},
	}
}

func (s *DeclaredConsequenceTestSuite) resolve(machine Machine) (*Output, error) {
	return Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: s.world(),
		Participants: []Participant{
			{Character: s.hero()},
			{Monster: monsters.NewWolf(wolfID).ToData()},
		},
		Machine: machine,
	})
}

// THE COMPILER NAMES IT. The bite's knockdown arrives as a gate AND as prone,
// both from the compiler — the machine is told, it does not know.
func (s *DeclaredConsequenceTestSuite) TestTheBitesCompilerNamesProneAlongsideItsGate() {
	profile := s.biteProfile()

	s.Require().NotNil(profile.Gate, "the bite declares a contest")
	s.Require().NotNil(profile.Imposes, "and names what rides on it")
	s.Require().Equal(refs.Conditions.Prone(), profile.Imposes.atStake().Ref,
		"a stat block's KnockdownDC means prone, and the compiler is where that is translated")
}

// A plain weapon declares neither half. The generic melee action and a
// character's weapon both compile to no gate, so neither carries a
// consequence that nothing could trigger.
func (s *DeclaredConsequenceTestSuite) TestAnUngatedAttackNamesNoConsequence() {
	skeleton := monsters.NewSkeleton("sk-1").ToData()
	var melee AttackProfile
	for _, action := range skeleton.Actions {
		if action.Ref.ID == refs.MonsterActions.Melee().ID {
			compiled, err := AttackFromMonsterAction(action)
			s.Require().NoError(err)
			melee = compiled

			break
		}
	}

	s.Require().NotEmpty(melee.Damage, "the skeleton has a generic melee action")
	s.Require().Nil(melee.Gate, "a plain weapon just hits")
	s.Require().Nil(melee.Imposes, "so there is nothing riding on it")
}

// A bite with no gate names no consequence either. Content can author one —
// BiteConfig's own doc says "Nil means the bite just bites" — and a
// consequence sitting on a profile nothing can trigger is a claim the code
// would be making and never honouring.
func (s *DeclaredConsequenceTestSuite) TestAGatelessBiteNamesNoConsequence() {
	config, err := json.Marshal(monsterActions.BiteConfig{
		AttackBonus: 4,
		Damage:      []damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}},
		// No SaveGate, no KnockdownDC: a bite that just bites.
	})
	s.Require().NoError(err)

	profile, err := AttackFromMonsterAction(monster.ActionData{
		Ref:    *refs.MonsterActions.Bite(),
		Config: config,
	})
	s.Require().NoError(err)

	s.Require().Nil(profile.Gate, "nothing to contest")
	s.Require().Nil(profile.Imposes, "so nothing rides on it")
	s.Require().Equal([]damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}}, profile.Damage,
		"and it is still a bite")
}

// A gate that names no consequence is refused BY NAME, before anything rolls.
//
// The alternative is worse than an error: the target rolls a save, can fail
// it, and suffers nothing — a rule that reads as working and means nothing.
func (s *DeclaredConsequenceTestSuite) TestAGateWithNoConsequenceIsRefused() {
	orphaned := s.biteProfile()
	orphaned.Imposes = nil

	err := orphaned.validate()
	s.Require().ErrorIs(err, ErrBadAttack)
	s.Require().Contains(err.Error(), "names no consequence")
}

// The other direction is refused too: a consequence with no gate would never
// be imposed, because a strike with no gate never contests anything. Accepting
// it would silently discard a rule its author believed they had written.
//
// The pair is a pair, and validating only one direction would make that a
// comment rather than a contract (Copilot review, #1014).
func (s *DeclaredConsequenceTestSuite) TestAConsequenceWithNoGateIsRefused() {
	stranded := s.biteProfile()
	stranded.Gate = nil

	err := stranded.validate()
	s.Require().ErrorIs(err, ErrBadAttack)
	s.Require().Contains(err.Error(), "declares no gate")
}

// And the machine refuses that direction too, not only the constructor.
func (s *DeclaredConsequenceTestSuite) TestTheMachineRefusesAConsequenceWithNoGate() {
	stranded := s.biteProfile()
	stranded.Gate = nil

	_, err := s.resolve(NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   heroID,
		Attack:     stranded,
		Roller:     &sequenceRoller{singles: []int{hitRoll, 2}, pair: []int{3, 4}, fallback: 2},
	}))
	s.Require().ErrorIs(err, ErrBadAttack)
}

// And the machine refuses it too, rather than only the constructor — a
// hand-built profile reaches Start without passing through any compiler.
func (s *DeclaredConsequenceTestSuite) TestTheMachineRefusesAGateWithNoConsequence() {
	orphaned := s.biteProfile()
	orphaned.Imposes = nil

	_, err := s.resolve(NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   heroID,
		Attack:     orphaned,
		Roller:     &sequenceRoller{singles: []int{hitRoll, 2}, pair: []int{3, 4}, fallback: 2},
	}))
	s.Require().ErrorIs(err, ErrBadAttack)
}

// THE POINT OF THE WHOLE CHANGE: a profile naming a different consequence
// imposes THAT one, and prone never appears.
//
// The pairing is deliberate nonsense — a wolf's bite does not make anyone
// Dodging — and that is exactly what makes it a good test: it is unreachable
// from content, so the only way this passes is if the machine imposes what it
// was told rather than what it assumes. A rules-sensible second gated attack
// arrives with the ghoul's paralysis; this proves the mechanism before the
// content needs it.
func (s *DeclaredConsequenceTestSuite) TestAProfileNamingADifferentConsequenceImposesThatOne() {
	profile := s.biteProfile()
	profile.Imposes = ImposeCondition(refs.Conditions.Dodging(), dnd5eEvents.ConditionDodging)

	out, err := s.resolve(NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   heroID,
		// hitRoll lands the bite; the 2 fails the DC 11 save even with the
		// hero's +5, so the consequence is imposed rather than negated.
		Attack: profile,
		Roller: &sequenceRoller{singles: []int{hitRoll, 2}, pair: []int{3, 4}, fallback: 2},
	}))
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().True(outcome.Hit)
	s.Require().NotNil(outcome.Contest)
	s.Require().False(outcome.Contest.Succeeded, "a 2 against DC 11 fails")

	s.Require().Equal(refs.Conditions.Dodging(), outcome.Contest.AtStake.Ref,
		"the contest was about what the profile named")

	s.Require().Len(out.DirtyCharacters, 1)
	// Require the condition landed before indexing it: a consequence that
	// failed to apply should fail this test by NAMING the missing condition,
	// not by panicking on an empty slice.
	s.Require().Len(out.DirtyCharacters[0].Conditions, 1, "exactly one condition was imposed")
	conditions := string(out.DirtyCharacters[0].Conditions[0])
	s.Require().Contains(conditions, refs.Conditions.Dodging().ID,
		"the hero gained what the profile named")
	s.Require().NotContains(conditions, refs.Conditions.Prone().ID,
		"and NOT prone — the machine imposed no opinion of its own")
}

// The same swing whose save SUCCEEDS imposes nothing at all, so the test above
// is measuring the consequence and not merely the presence of a condition.
func (s *DeclaredConsequenceTestSuite) TestASavedGateImposesNothing() {
	profile := s.biteProfile()
	profile.Imposes = ImposeCondition(refs.Conditions.Dodging(), dnd5eEvents.ConditionDodging)

	out, err := s.resolve(NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   heroID,
		// 18 + the hero's +5 STR beats DC 11 comfortably.
		Attack: profile,
		Roller: &sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}, fallback: 2},
	}))
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().NotNil(outcome.Contest)
	s.Require().True(outcome.Contest.Succeeded)
	s.Require().Len(out.DirtyCharacters, 1)
	s.Require().Empty(out.DirtyCharacters[0].Conditions, "a made save costs nothing")
}

// A gate is still refused when it names abilities nobody can roll — the
// consequence rule is additive to the gate's own validation, not a
// replacement for it.
func (s *DeclaredConsequenceTestSuite) TestTheGatesOwnValidationStillApplies() {
	profile := s.biteProfile()
	profile.Gate = &saves.SaveGate{
		Abilities:  []abilities.Ability{},
		DC:         saves.DCStatic(11),
		OnSuccess:  saves.Negated,
		Recurrence: saves.RecurrenceNone,
	}

	_, err := s.resolve(NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   heroID,
		Attack:     profile,
		Roller:     &sequenceRoller{singles: []int{hitRoll, 2}, pair: []int{3, 4}, fallback: 2},
	}))
	s.Require().Error(err, "a gate nobody can roll is still refused")
}
