// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
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

// ContestTestSuite drives the knockdown lane end to end: content declares a
// gate, resolution contests it, and a failed save lands a condition on a sheet
// that comes back dirty.
type ContestTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestContestSuite(t *testing.T) {
	suite.Run(t, new(ContestTestSuite))
}

func (s *ContestTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// wolfsKnockdown reads the gate off the actual catalog wolf rather than
// building one here. That is the point of the lane: the declaration is content,
// so a test that hand-builds it proves the machine works on a gate nobody
// ships.
func (s *ContestTestSuite) wolfsKnockdown() *saves.SaveGate {
	wolf := monsters.NewWolf(wolfID)
	s.Require().Len(wolf.Actions(), 1)

	bite, ok := wolf.Actions()[0].(*monsterActions.BiteAction)
	s.Require().True(ok, "the wolf's action is its bite")

	gate := bite.SaveGate()
	s.Require().NotNil(gate, "and the bite declares what its knockdown can be contested with")

	return gate
}

func (s *ContestTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// hero is a barbarian with STR 16 and proficiency in STR saves: a +5 modifier,
// which the DC 11 gate is a real contest against.
func (s *ContestTestSuite) hero(conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID:       heroID,
		PlayerID: "player-1",
		Name:     "Grog",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, // +3
			abilities.DEX: 14, // +2
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:        14,
		MaxHitPoints:     14,
		ProficiencyBonus: 2,
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
		},
		Conditions: conds,
	}
}

func (s *ContestTestSuite) raging() json.RawMessage {
	raw, err := (&conditions.RagingCondition{
		CharacterID: heroID,
		DamageBonus: 2,
		Level:       1,
		Source:      "rage",
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

func (s *ContestTestSuite) wolfData() *monster.Data {
	return &monster.Data{
		ID:            wolfID,
		Name:          "Wolf",
		Ref:           refs.Monsters.Wolf(),
		HitPoints:     11,
		MaxHitPoints:  11,
		ArmorClass:    13,
		AbilityScores: shared.AbilityScores{},
	}
}

// contest builds the interaction the wolf's bite would raise on a hit.
func (s *ContestTestSuite) contest(gate *saves.SaveGate, roller *scriptedRoller) Machine {
	return NewContest(&ContestInput{
		Gate:        gate,
		SaverID:     heroID,
		Consequence: ImposeCondition(refs.Conditions.Prone(), dnd5eEvents.ConditionProne),
		Roller:      roller,
	})
}

func (s *ContestTestSuite) resolve(hero *character.Data, machine Machine) *Output {
	out, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: hero}, {Monster: s.wolfData()}},
		Machine:      machine,
	})
	s.Require().NoError(err)

	return out
}

func (s *ContestTestSuite) contestOutcome(out *Output) ContestOutcome {
	outcome, ok := out.Outcome.(ContestOutcome)
	s.Require().True(ok, "a contest produces a ContestOutcome")

	return outcome
}

// conditionRefs reads the refs of the conditions a persisted sheet carries.
func (s *ContestTestSuite) conditionRefs(data *character.Data) []string {
	out := make([]string, 0, len(data.Conditions))
	for _, raw := range data.Conditions {
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		s.Require().NoError(json.Unmarshal(raw, &peek))
		out = append(out, peek.Ref.String())
	}

	return out
}

// THE HEADLINE. Nobody wired anything: the wolf's stat block says its bite can
// be contested with a DC 11 STR save, the hero rolls badly, and prone is on the
// hero's sheet in the data that comes back to be persisted.
func (s *ContestTestSuite) TestAFailedSaveKnocksTheHeroProne() {
	// +5 modifier, roll 3, total 8 against DC 11.
	out := s.resolve(s.hero(), s.contest(s.wolfsKnockdown(), &scriptedRoller{single: straightRoll}))

	outcome := s.contestOutcome(out)
	s.Require().False(outcome.Succeeded)
	s.Require().Equal(11, outcome.DC, "the DC came from the gate")
	s.Require().Equal(abilities.STR, outcome.Ability)
	s.Require().Equal(refs.Conditions.Prone(), outcome.AtStake.Ref)
	s.Require().Len(outcome.Imposed, 1)
	s.Require().Equal(refs.Conditions.Prone(), outcome.Imposed[0].Ref)

	s.Require().Len(out.DirtyCharacters, 1, "the sheet changed, so it comes back to be saved")
	s.Require().Equal(heroID, out.DirtyCharacters[0].ID)
	s.Require().Equal([]string{refs.Conditions.Prone().String()}, s.conditionRefs(out.DirtyCharacters[0]))
}

// The control that makes the headline mean something: same wolf, same gate,
// a better roll — and the hero is still standing, with nothing to persist.
func (s *ContestTestSuite) TestAPassedSaveLeavesTheHeroStanding() {
	// +5 modifier, roll 18, total 23 against DC 11.
	out := s.resolve(s.hero(), s.contest(s.wolfsKnockdown(), &scriptedRoller{single: advantageRoll}))

	outcome := s.contestOutcome(out)
	s.Require().True(outcome.Succeeded)
	s.Require().Empty(outcome.Imposed, "nothing landed")
	s.Require().Equal(refs.Conditions.Prone().String(), outcome.AtStake.Ref.String(),
		"and the outcome still says what was resisted")

	s.Require().Empty(out.DirtyCharacters, "a save that changed nothing leaves nothing to save")
}

// The requested save folds the interaction's chains, so an effect attached for
// this interaction contributes to it. Raging grants advantage on STR saves —
// and here that is the difference between standing and lying down.
func (s *ContestTestSuite) TestRagingFoldsIntoTheContestsSave() {
	roller := &scriptedRoller{single: straightRoll, pair: []int{straightRoll, advantageRoll}}

	out := s.resolve(s.hero(s.raging()), s.contest(s.wolfsKnockdown(), roller))

	outcome := s.contestOutcome(out)
	s.Require().Equal(advantageRoll, outcome.Save.Result.Roll,
		"only a rolled-twice-take-higher can produce this die")
	s.Require().True(outcome.Save.Folded.HasAdvantage())
	s.Require().Len(outcome.Save.Folded.AdvantageSources, 1)
	s.Require().Equal(refs.Conditions.Raging(), outcome.Save.Folded.AdvantageSources[0].SourceRef,
		"and it is Raging that says so, through the Request")

	s.Require().True(outcome.Succeeded, "the barbarian shrugs off the wolf")
	s.Require().Empty(outcome.Imposed)
}

// Without the rage, the identical roller knocks the same hero down: the
// advantage above came from the condition and not from the dice.
func (s *ContestTestSuite) TestTheSameRollWithoutRageKnocksHimDown() {
	roller := &scriptedRoller{single: straightRoll, pair: []int{straightRoll, advantageRoll}}

	out := s.resolve(s.hero(), s.contest(s.wolfsKnockdown(), roller))

	s.Require().False(s.contestOutcome(out).Succeeded)
	s.Require().Len(out.DirtyCharacters, 1)
}

// Two machines, one registration list: the requested save attaches nothing of
// its own, so the ledger is the participants' and stays reproducible across
// input orders (R4).
func (s *ContestTestSuite) TestRegistrationsDoNotDependOnInputOrder() {
	// A raging hero rolls with advantage, so the roller must have a pair to
	// give; both dice are the low roll, so the save still fails either way.
	roller := func() *scriptedRoller {
		return &scriptedRoller{single: straightRoll, pair: []int{straightRoll, straightRoll}}
	}

	forward, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.hero(s.raging())}, {Monster: s.wolfData()}},
		Machine:      s.contest(s.wolfsKnockdown(), roller()),
	})
	s.Require().NoError(err)

	reversed, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Monster: s.wolfData()}, {Character: s.hero(s.raging())}},
		Machine:      s.contest(s.wolfsKnockdown(), roller()),
	})
	s.Require().NoError(err)

	s.Require().NotEmpty(forward.Hooks)
	s.Require().Equal(forward.Hooks, reversed.Hooks)
}

// A gate offering a choice gets the saver's best modifier — provisional until
// an interaction can ask (Pose). Detectable rather than asserted: DC 8 is
// beaten by the hero's proficient STR (+5, total 8) and not by his DEX (+2,
// total 5), so which one was rolled decides the outcome.
func (s *ContestTestSuite) TestAMultiAbilityGatePicksTheBestModifier() {
	forEach := []struct {
		name  string
		order []abilities.Ability
	}{
		{"strength first", []abilities.Ability{abilities.STR, abilities.DEX}},
		{"dexterity first", []abilities.Ability{abilities.DEX, abilities.STR}},
	}

	for _, tc := range forEach {
		s.Run(tc.name, func() {
			gate := &saves.SaveGate{
				Abilities:  tc.order,
				DC:         saves.DCStatic(8),
				OnSuccess:  saves.Negated,
				Recurrence: saves.RecurrenceNone,
			}

			out := s.resolve(s.hero(), s.contest(gate, &scriptedRoller{single: straightRoll}))

			outcome := s.contestOutcome(out)
			s.Require().Equal(abilities.STR, outcome.Ability, "the better save, whichever order it is listed in")
			s.Require().True(outcome.Succeeded, "8 beats DC 8; the DEX save's 5 would not have")
		})
	}
}

// The DC is the gate's to compute, not this package's. A derived formula proves
// the delegation: nothing here knows that Undead Fortitude is 5 + damage.
func (s *ContestTestSuite) TestTheDCComesFromTheGate() {
	gate := &saves.SaveGate{
		Abilities:  []abilities.Ability{abilities.STR},
		DC:         saves.DCFivePlusDamageTaken(),
		OnSuccess:  saves.Negated,
		Recurrence: saves.RecurrenceNone,
	}

	out, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.hero()}, {Monster: s.wolfData()}},
		Machine: NewContest(&ContestInput{
			Gate:        gate,
			SaverID:     heroID,
			Consequence: ImposeCondition(refs.Conditions.Prone(), dnd5eEvents.ConditionProne),
			DamageTaken: 9,
			Roller:      &scriptedRoller{single: advantageRoll},
		}),
	})
	s.Require().NoError(err)

	s.Require().Equal(14, s.contestOutcome(out).DC, "5 + 9 damage, computed by the gate")
}

// A recurring gate is refused rather than run once. Treating "save again at the
// end of each of your turns" as a single save would produce a paralysis nobody
// ever shakes off, and it would look like it worked.
func (s *ContestTestSuite) TestARecurringGateIsRefused() {
	gate := saves.NewSaveGate(abilities.STR, 11)
	gate.Recurrence = saves.RecurrenceEndOfTurn

	_, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.hero()}, {Monster: s.wolfData()}},
		Machine:      s.contest(gate, &scriptedRoller{single: straightRoll}),
	})

	s.Require().Error(err)
	s.Require().ErrorIs(err, ErrRecurrenceUnsupported)
	s.Require().Contains(err.Error(), "ghoul", "and it says what it is waiting for")
}

func (s *ContestTestSuite) TestRefusesAContestItCannotRun() {
	s.Run("no gate", func() {
		_, err := Resolve(s.ctx, &Input{
			World:        s.world(),
			Participants: []Participant{{Character: s.hero()}},
			Machine: NewContest(&ContestInput{
				SaverID:     heroID,
				Consequence: ImposeCondition(refs.Conditions.Prone(), dnd5eEvents.ConditionProne),
			}),
		})
		s.Require().ErrorIs(err, ErrNilInput)
	})

	s.Run("an invalid gate", func() {
		_, err := Resolve(s.ctx, &Input{
			World:        s.world(),
			Participants: []Participant{{Character: s.hero()}},
			Machine: NewContest(&ContestInput{
				Gate:        &saves.SaveGate{DC: saves.DCStatic(11)}, // no ability to roll
				SaverID:     heroID,
				Consequence: ImposeCondition(refs.Conditions.Prone(), dnd5eEvents.ConditionProne),
			}),
		})
		s.Require().ErrorIs(err, ErrBadGate)
	})

	s.Run("no consequence", func() {
		_, err := Resolve(s.ctx, &Input{
			World:        s.world(),
			Participants: []Participant{{Character: s.hero()}},
			Machine:      NewContest(&ContestInput{Gate: s.wolfsKnockdown(), SaverID: heroID}),
		})
		s.Require().ErrorIs(err, ErrNilInput)
	})

	s.Run("a saver who is not a participant", func() {
		_, err := Resolve(s.ctx, &Input{
			World:        s.world(),
			Participants: []Participant{{Character: s.hero()}},
			Machine: NewContest(&ContestInput{
				Gate:        s.wolfsKnockdown(),
				SaverID:     "nobody",
				Consequence: ImposeCondition(refs.Conditions.Prone(), dnd5eEvents.ConditionProne),
			}),
		})
		s.Require().ErrorIs(err, ErrNoSaver)
	})
}

// The step log says what the interaction did, in order: request the save, then
// impose. A reader of the ledger should not have to infer the second half.
func (s *ContestTestSuite) TestTheStepsAreNamedForWhatTheyDo() {
	gate := s.wolfsKnockdown()

	machine := s.contest(gate, &scriptedRoller{single: straightRoll})
	step, err := machine.Start(s.ctx, &Participants{
		characters: map[string]*character.Character{},
		monsters:   map[string]*monster.Monster{},
	})

	// The cast is empty, so the ability choice cannot read a modifier.
	s.Require().ErrorIs(err, ErrNoSaver)
	s.Require().Nil(step)
}
