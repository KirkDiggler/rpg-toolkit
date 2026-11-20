// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// wellFedLibrary is a status an author wrote: it lasts until the holder's run
// ends, and grants advantage on Charisma and Strength checks and on
// Constitution saves.
const wellFedLibrary = `{
 "parts":{
  "cha-checks":{"kind":"advantage","params":{"on":"ability_check"},"when":{"fact":{"name":"action.ability","op":"eq","value":"cha"}}},
  "con-saves":{"kind":"advantage","params":{"on":"saving_throw"},"when":{"fact":{"name":"action.ability","op":"eq","value":"con"}}},
  "str-checks":{"kind":"advantage","params":{"on":"ability_check"},"when":{"fact":{"name":"action.ability","op":"eq","value":"str"}}},
  "run-end":{"kind":"end_on","params":{"beat":"run_end","reason":"run_ended"}}
 },
 "benefit_sets":{"fed":["cha-checks","con-saves","str-checks"]},
 "definitions":{
  "well-fed":{"name":"Well fed","coordinator":"recipient","maintenance":[{"key":"run-end","part":"run-end"}],"benefits":[{"key":"fed","set":"fed"}]}
 }
}`

const wellFedName = "Well fed"

// GiveStatusTestSuite gives a member a status an author wrote, and follows it
// through a save, a reload, a roll, a repeated give and a departure.
type GiveStatusTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestGiveStatusSuite(t *testing.T) {
	suite.Run(t, new(GiveStatusTestSuite))
}

func (s *GiveStatusTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *GiveStatusTestSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

func (s *GiveStatusTestSuite) interaction(fixtures *ContestDamageTestSuite, participants ...Participant) *Input {
	return &Input{
		World:        fixtures.world(),
		Participants: participants,
		Capabilities: encounter.Capabilities{
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Participation: everyoneParticipates{},
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     noHandsAreObserved{},
			Conditions:    noHandsAreObserved{},
			Sheets:        noSheetsAsked{},
			Roller:        dice.NewRoller(),
			Actors:        Actors,
		},
	}
}

func cookRef() core.Ref { return core.Ref{Module: "site", Type: "statuses", ID: "well-fed"} }

func (s *GiveStatusTestSuite) give(interaction *Input, member, application string) (*GiveStatusOutput, error) {
	return GiveStatus(s.ctx, &GiveStatusInput{
		Interaction: interaction, Member: member,
		Library: json.RawMessage(wellFedLibrary), StatusID: "well-fed",
		SourceID: "cook", Source: cookRef(), ApplicationID: application,
	})
}

func (s *GiveStatusTestSuite) monsterSheet(out *Output, id string) *monster.Data {
	for _, data := range out.DirtyMonsters {
		if data.ID == id {
			return data
		}
	}
	s.Require().Failf("no dirty monster", "%q did not come back to be saved", id)

	return nil
}

// grantedBy reports whether a keep record names the status as a source of
// advantage.
func grantedBy(keep *dnd5eEvents.DiceKeep, name string) bool {
	if keep == nil {
		return false
	}
	for _, source := range keep.Granted {
		if source.Name == name {
			return true
		}
	}
	return false
}

// A CHARACTER KEEPS IT ACROSS A SAVE: the status lands on the record, loads
// back from it, and a Charisma check rolled from the reloaded record carries
// the advantage naming the status.
func (s *GiveStatusTestSuite) TestACharacterHoldsItAfterAReloadAndItGrantsTheRoll() {
	fixtures := s.fixtures()
	out, err := s.give(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)}), heroID, "well-fed:hero")
	s.Require().NoError(err)
	s.False(out.AlreadyHeld)
	s.NotEmpty(out.Effects)
	for _, effect := range out.Effects {
		s.Equal(ImposedCondition, effect.Kind)
		s.Equal(heroID, effect.RecipientID)
		s.Equal(wellFedName, effect.Description)
	}
	saved := fixtures.sheet(out.Resolved, heroID)
	s.NotEmpty(saved.Conditions)

	checked, err := MakeCheck(s.ctx, &CheckInput{
		Character: saved, Approaches: []encounter.CheckApproach{route(skills.Persuasion, 10)},
		Roller: dice.NewRoller(),
	})
	s.Require().NoError(err)
	keep := keepOf(s.T(), checked.Calculation)
	s.Require().NotNil(keep)
	s.Equal(dnd5eEvents.KeepAdvantage, keep.Rule)
	s.True(grantedBy(keep, wellFedName), "the status names itself as the advantage's source: %+v", keep.Granted)

	// One status is two linked conditions on the sheet (its coordinator and
	// its benefit); the player's status view shows it once.
	loaded, err := character.Load(s.ctx, saved)
	s.Require().NoError(err)
	view, err := loaded.StatusView(&character.StatusViewInput{})
	s.Require().NoError(err)
	rows := 0
	for _, condition := range view.View.Conditions {
		if condition.Name == wellFedName {
			rows++
		}
	}
	s.Equal(1, rows, "one status shows as one row in the status view")
}

// A BARE-ABILITY ROUTE SEES THE STATUS: picking a lock with Strength, a route
// that names an ability and no skill, carries the Strength-guarded advantage
// naming the status; a Dexterity route carries none of it.
func (s *GiveStatusTestSuite) TestABareAbilityLockCheckCarriesTheAdvantage() {
	fixtures := s.fixtures()
	out, err := s.give(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)}), heroID, "well-fed:hero")
	s.Require().NoError(err)
	saved := fixtures.sheet(out.Resolved, heroID)

	strength, err := MakeCheck(s.ctx, &CheckInput{
		Character: saved, Approaches: []encounter.CheckApproach{route(string(abilities.STR), 10)},
		Roller: dice.NewRoller(),
	})
	s.Require().NoError(err)
	s.True(grantedBy(keepOf(s.T(), strength.Calculation), wellFedName), "a bare Strength route carries the status's advantage")

	longName, err := MakeCheck(s.ctx, &CheckInput{
		Character: saved, Approaches: []encounter.CheckApproach{route("strength", 10)},
		Roller: dice.NewRoller(),
	})
	s.Require().NoError(err)
	s.True(grantedBy(keepOf(s.T(), longName.Calculation), wellFedName), "a route spelling the ability long carries it too")

	dexterity, err := MakeCheck(s.ctx, &CheckInput{
		Character: saved, Approaches: []encounter.CheckApproach{route(string(abilities.DEX), 10)},
		Roller: dice.NewRoller(),
	})
	s.Require().NoError(err)
	s.False(grantedBy(keepOf(s.T(), dexterity.Calculation), wellFedName), "a bare Dexterity route does not")
}

// A MONSTER KEEPS IT ACROSS A SAVE too, and a Constitution save rolled from
// the reloaded record carries the advantage naming the status.
func (s *GiveStatusTestSuite) TestAMonsterHoldsItAfterAReloadAndItGrantsTheSave() {
	fixtures := s.fixtures()
	out, err := s.give(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Monster: fixtures.wolfData()}), wolfID, "well-fed:wolf")
	s.Require().NoError(err)
	s.False(out.AlreadyHeld)
	saved := s.monsterSheet(out.Resolved, wolfID)
	s.NotEmpty(saved.Conditions)

	interaction := s.interaction(fixtures, Participant{Character: fixtures.saver(14)}, Participant{Monster: saved})
	interaction.Machine = NewSave(&SaveInput{
		SaverID: wolfID, Ability: abilities.CON, DC: 10,
		D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.ViciousMockery(), Name: "Test Save"},
		Roller:    dice.NewRoller(),
	})
	rolled, err := Resolve(s.ctx, interaction)
	s.Require().NoError(err)
	outcome, ok := rolled.Outcome.(SaveOutcome)
	s.Require().True(ok)
	keep := keepOf(s.T(), outcome.Result.Calculation)
	s.Require().NotNil(keep)
	s.True(grantedBy(keep, wellFedName), "the status names itself as the advantage's source: %+v", keep.Granted)
}

// GIVEN AGAIN, NOTHING HAPPENS: the same application id is the same status,
// so a second give answers AlreadyHeld, changes no sheet, and the member still
// holds exactly one instance.
func (s *GiveStatusTestSuite) TestGivingAgainLeavesOneAndSaysSo() {
	fixtures := s.fixtures()
	first, err := s.give(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)}), heroID, "well-fed:hero")
	s.Require().NoError(err)
	held := fixtures.sheet(first.Resolved, heroID)

	again, err := s.give(s.interaction(fixtures,
		Participant{Character: held}, Participant{Character: fixtures.bard(1)}), heroID, "well-fed:hero")
	s.Require().NoError(err)
	s.True(again.AlreadyHeld)
	s.Empty(again.Effects)
	for _, dirty := range again.Resolved.DirtyCharacters {
		s.NotEqual(heroID, dirty.ID, "a repeated give changes no sheet")
	}
	s.Equal(len(first.Effects), len(held.Conditions),
		"the record the first give saved carries exactly the one status, and the second give saved nothing over it")
}

// DEPARTING ENDS THE RUN for the leaver: the status comes off them, for its
// authored reason, and another member holding the same status keeps theirs.
func (s *GiveStatusTestSuite) TestDepartRemovesItForTheLeaverAndNotForAnother() {
	fixtures := s.fixtures()
	hero, err := s.give(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)}), heroID, "well-fed:hero")
	s.Require().NoError(err)
	heroHeld := fixtures.sheet(hero.Resolved, heroID)
	bard, err := s.give(s.interaction(fixtures,
		Participant{Character: heroHeld}, Participant{Character: fixtures.bard(1)}), bardID, "well-fed:bard")
	s.Require().NoError(err)
	bardHeld := fixtures.sheet(bard.Resolved, bardID)
	s.NotEmpty(bardHeld.Conditions)

	out, err := Depart(s.ctx, &DepartInput{Character: heroHeld, Others: []Participant{{Character: bardHeld}}})
	s.Require().NoError(err)
	s.Empty(out.Character.Conditions, "the leaver's status ended with their run")
	s.NotEmpty(out.Ended)
	for _, ended := range out.Ended {
		s.Equal("run_ended", ended.Reason)
	}
	for _, dirty := range out.DirtyCharacters {
		s.NotEqual(bardID, dirty.ID, "the other member's status is untouched")
	}
}

// EACH REFUSAL IS ITS OWN ERROR, told apart by the caller, and none of them
// opens an interaction that changes anything.
func (s *GiveStatusTestSuite) TestRefusalsAreToldApart() {
	fixtures := s.fixtures()
	for _, tc := range []struct {
		name    string
		library string
		status  string
		member  string
		want    error
	}{
		{"a library that does not compile", strings.Replace(wellFedLibrary, `"beat":"run_end"`, `"beat":"run_over"`, 1), "well-fed", heroID, ErrBadStatusLibrary},
		{"a status not in the library", wellFedLibrary, "well-rested", heroID, ErrUnknownStatus},
		{"a member not in the run", wellFedLibrary, "well-fed", "stranger", ErrBadParticipant},
	} {
		s.Run(tc.name, func() {
			out, err := GiveStatus(s.ctx, &GiveStatusInput{
				Interaction: s.interaction(fixtures,
					Participant{Character: fixtures.saver(14)}, Participant{Character: fixtures.bard(1)}),
				Member: tc.member, Library: json.RawMessage(tc.library), StatusID: tc.status,
				SourceID: "cook", Source: cookRef(), ApplicationID: "well-fed:" + tc.member,
			})
			s.Require().ErrorIs(err, tc.want)
			s.Nil(out)
		})
	}
}
