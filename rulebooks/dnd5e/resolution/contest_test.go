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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
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
	definitions := monsters.NewWolf(wolfID).Actions()
	s.Require().Len(definitions, 1)
	s.Require().Len(definitions[0].Attack.OnHit, 1)
	return definitions[0].Attack.OnHit[0].Save
}

func (s *ContestTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Mover: encounter.RefusingMover{}, Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}},
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
		Application: combatActions.ConditionApplication{Ref: *refs.Conditions.Prone()},
		Roller:      roller,
	})
}

func (s *ContestTestSuite) resolve(hero *character.Data, machine Machine) *Output {
	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
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

	forward, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.hero(s.raging())}, {Monster: s.wolfData()}},
		Machine:      s.contest(s.wolfsKnockdown(), roller()),
	})
	s.Require().NoError(err)

	reversed, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
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

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.hero()}, {Monster: s.wolfData()}},
		Machine: NewContest(&ContestInput{
			Gate:        gate,
			SaverID:     heroID,
			Application: combatActions.ConditionApplication{Ref: *refs.Conditions.Prone()},
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

	_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.hero()}, {Monster: s.wolfData()}},
		Machine:      s.contest(gate, &scriptedRoller{single: straightRoll}),
	})

	s.Require().Error(err)
	s.Require().ErrorIs(err, ErrRecurrenceUnsupported)
	s.Require().Contains(err.Error(), string(saves.RecurrenceEndOfTurn))
}

func (s *ContestTestSuite) TestRefusesAContestItCannotRun() {
	s.Run("no gate", func() {
		_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
			World:        s.world(),
			Participants: []Participant{{Character: s.hero()}},
			Machine: NewContest(&ContestInput{
				SaverID:     heroID,
				Application: combatActions.ConditionApplication{Ref: *refs.Conditions.Prone()},
			}),
		})
		s.Require().ErrorIs(err, ErrNilInput)
	})

	s.Run("an invalid gate", func() {
		_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
			World:        s.world(),
			Participants: []Participant{{Character: s.hero()}},
			Machine: NewContest(&ContestInput{
				Gate:        &saves.SaveGate{DC: saves.DCStatic(11)}, // no ability to roll
				SaverID:     heroID,
				Application: combatActions.ConditionApplication{Ref: *refs.Conditions.Prone()},
			}),
		})
		s.Require().ErrorIs(err, ErrBadGate)
	})

	s.Run("unknown save ability", func() {
		gate := saves.NewSaveGate(abilities.Ability("bogus"), 11)
		_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
			World:        s.world(),
			Participants: []Participant{{Character: s.hero()}},
			Machine: NewContest(&ContestInput{
				Gate:        gate,
				SaverID:     heroID,
				Application: combatActions.ConditionApplication{Ref: *refs.Conditions.Prone()},
			}),
		})
		s.Require().ErrorIs(err, ErrBadGate)
	})

	s.Run("no condition application", func() {
		_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
			World:        s.world(),
			Participants: []Participant{{Character: s.hero()}},
			Machine:      NewContest(&ContestInput{Gate: s.wolfsKnockdown(), SaverID: heroID}),
		})
		s.Require().ErrorIs(err, ErrBadAction)
	})

	s.Run("a saver who is not a participant", func() {
		_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
			World:        s.world(),
			Participants: []Participant{{Character: s.hero()}},
			Machine: NewContest(&ContestInput{
				Gate:        s.wolfsKnockdown(),
				SaverID:     "nobody",
				Application: combatActions.ConditionApplication{Ref: *refs.Conditions.Prone()},
			}),
		})
		s.Require().ErrorIs(err, ErrNoSaver)
	})
}

// Choosing which ability to roll reads a modifier off a sheet, so a saver the
// cast does not have is refused there rather than rolling without one.
func (s *ContestTestSuite) TestASaverTheCastDoesNotHaveIsRefusedBeforeAnyRoll() {
	machine := s.contest(s.wolfsKnockdown(), &scriptedRoller{single: straightRoll})

	step, err := machine.Start(s.ctx, &Participants{
		characters: map[string]*character.Character{},
		monsters:   map[string]*monster.Monster{},
	})

	s.Require().ErrorIs(err, ErrNoSaver)
	s.Require().Nil(step, "and no step is yielded, so nothing is rolled or imposed")
}

// The imposition step says what it does, so a reader of the step log sees
// "impose the prone condition" rather than a fold that folds nothing.
