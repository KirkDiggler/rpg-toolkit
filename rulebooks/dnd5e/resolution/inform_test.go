// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	informRogue   = "rogue"
	informFighter = "fighter"
	informGoblin1 = "goblin-1"
	informGoblin2 = "goblin-2"
	informGoblins = "goblins"
)

// InformAttackTestSuite holds effect information for an attack: answers drawn
// from the actor's observed context alone, once with no target and once per
// candidate.
type InformAttackTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestInformAttackTestSuite(t *testing.T) {
	suite.Run(t, new(InformAttackTestSuite))
}

func (s *InformAttackTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// sightOf answers every member's sight range in cells, with an override per
// member; anybody not named sees the whole map.
type sightOf map[encounter.MemberID]int

func (m sightOf) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = unlimitedSight
		if cells, ok := m[id]; ok {
			out[id] = cells
		}
	}
	return out, nil
}

// scene seats the rogue beside goblin one, the lone goblin two well away and,
// when withFighter, the fighter on goblin one's far side — adjacent to it and
// two cells from the rogue. The goblins are hostile to the party.
func (s *InformAttackTestSuite) scene(sight encounter.Sight, withFighter bool) *encounter.Encounter {
	members := []encounter.MemberInput{
		{ID: informRogue, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		{ID: informGoblin1, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}, Faction: informGoblins},
		{ID: informGoblin2, Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 3}, Faction: informGoblins},
	}
	if withFighter {
		members = append(members, encounter.MemberInput{
			ID: informFighter, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1},
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:   hexCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("cave", 0, 0, 10, 5)},
			Factions: []encounter.FactionInput{{ID: informGoblins}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{informGoblins, encounter.FactionParty}, Stance: encounter.StanceHostile,
			}},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      sight,
			Equipment:  noHandsAreObserved{},
			Sheets:     standStillSheets{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     encounter.RefusingMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	return enc
}

// rogue is the acting character's loaded sheet: a level-1 rogue carrying
// Sneak Attack.
func (s *InformAttackTestSuite) rogue() *character.Character {
	sneak, err := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{MemberID: informRogue}).ToJSON()
	s.Require().NoError(err)
	sheet, err := character.Load(s.ctx, &character.Data{
		ID: informRogue, PlayerID: "player-rogue", Name: "Rook", Level: 1, ClassID: classes.Rogue, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 16, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 10,
		},
		HitPoints: 10, MaxHitPoints: 10, ProficiencyBonus: 2,
		Conditions: []json.RawMessage{sneak},
	})
	s.Require().NoError(err)
	return sheet
}

// inform asks for the rogue's dagger rows over the scene's observed context.
func (s *InformAttackTestSuite) inform(enc *encounter.Encounter, targets ...string) *InformAttackOutput {
	observed, err := enc.ObservedContext(&encounter.ViewInput{Member: informRogue})
	s.Require().NoError(err)
	out, err := InformAttack(&InformAttackInput{
		Observed: observed, Actor: s.rogue(), Attack: dagger().Attack, Targets: targets,
	})
	s.Require().NoError(err)
	return out
}

// sneakRow finds the Sneak Attack row in a list of effects.
func (s *InformAttackTestSuite) sneakRow(effects []contributions.Effect) contributions.Effect {
	for _, effect := range effects {
		if effect.ID == refs.Features.SneakAttack().String() {
			return effect
		}
	}
	s.FailNow("no Sneak Attack row", "%+v", effects)
	return contributions.Effect{}
}

// TestInformAttackSneakAttackPerTarget: with no target Sneak Attack depends
// on the target; against the goblin the fighter is seen standing beside it
// applies; against the lone goblin it still depends, because sightings never
// prove there is nobody else.
func (s *InformAttackTestSuite) TestInformAttackSneakAttackPerTarget() {
	out := s.inform(s.scene(everyoneSeesTheWholeMap{}, true), informGoblin1, informGoblin2)

	none := s.sneakRow(out.Effects)
	s.Equal(contributions.StateDepends, none.State)
	s.Equal("Depends on the target", none.Reason)

	beside := s.sneakRow(out.ByTarget[informGoblin1])
	s.Equal(contributions.StateApplies, beside.State, "reason: %s", beside.Reason)
	s.Equal("Another enemy of the target is within 5 feet", beside.Reason)
	s.Equal("+1d6 damage", beside.Benefit)

	alone := s.sneakRow(out.ByTarget[informGoblin2])
	s.Equal(contributions.StateDepends, alone.State)
	s.Equal("Needs advantage or another enemy of the target within 5 feet", alone.Reason)

	for target, rows := range out.ByTarget {
		s.Require().Len(rows, len(out.Effects), target)
		for i := range rows {
			s.Equal(out.Effects[i].ID, rows[i].ID, "%s row %d", target, i)
		}
	}
}

// TestInformAttackHandaxeDoesNotSneakAttack is the information half of
// rpg-toolkit#1929: with the fighter seen beside the goblin, the dagger's row
// applies, and the same swing with a handaxe — neither finesse nor a ranged
// weapon, thrown or not — does not, for the rule's own reason. The execution
// half is TestHandaxeStrikeDoesNotSneakAttack.
func (s *InformAttackTestSuite) TestInformAttackHandaxeDoesNotSneakAttack() {
	enc := s.scene(everyoneSeesTheWholeMap{}, true)
	observed, err := enc.ObservedContext(&encounter.ViewInput{Member: informRogue})
	s.Require().NoError(err)

	out, err := InformAttack(&InformAttackInput{
		Observed: observed, Actor: s.rogue(), Attack: handaxe().Attack, Targets: []string{informGoblin1},
	})
	s.Require().NoError(err)

	row := s.sneakRow(out.ByTarget[informGoblin1])
	s.Equal(contributions.StateDoesNotApply, row.State)
	s.Equal("Sneak Attack requires a finesse or ranged weapon", row.Reason)
	s.Empty(row.Benefit)
}

// TestUnseenAllyLeavesRowsUnchanged is K3 and K5: the fighter really is beside
// the goblin, but the rogue cannot see that far, so the answer is exactly the
// answer of the same scene with no fighter at all — hidden truth changes
// nothing.
func (s *InformAttackTestSuite) TestUnseenAllyLeavesRowsUnchanged() {
	shortSighted := sightOf{informRogue: 1}

	unseen := s.inform(s.scene(shortSighted, true), informGoblin1)
	absent := s.inform(s.scene(shortSighted, false), informGoblin1)

	s.Equal(absent.ByTarget[informGoblin1], unseen.ByTarget[informGoblin1])
	s.Equal(contributions.StateDepends, s.sneakRow(unseen.ByTarget[informGoblin1]).State)
}

// TestInformAttackInputCarriesNoTargetSheets is K4 by construction: the input
// has exactly the fields information may draw from, and none through which a
// target sheet or the cast could arrive.
func (s *InformAttackTestSuite) TestInformAttackInputCarriesNoTargetSheets() {
	kind := reflect.TypeOf(InformAttackInput{})
	fields := make([]string, 0, kind.NumField())
	for i := range kind.NumField() {
		fields = append(fields, kind.Field(i).Name)
	}

	s.Equal([]string{"Observed", "Actor", "Attack", "Targets"}, fields)
	s.Equal(reflect.TypeOf([]string(nil)), kind.Field(3).Type, "targets are ids, never sheets")
}

// TestInformAttackRefusesWhatItCannotAnswerFrom: no observed context, an
// observed context that is somebody else's, and a repeated target are each
// an error rather than rows.
func (s *InformAttackTestSuite) TestInformAttackRefusesWhatItCannotAnswerFrom() {
	enc := s.scene(everyoneSeesTheWholeMap{}, true)
	theirs, err := enc.ObservedContext(&encounter.ViewInput{Member: informFighter})
	s.Require().NoError(err)
	mine, err := enc.ObservedContext(&encounter.ViewInput{Member: informRogue})
	s.Require().NoError(err)

	_, err = InformAttack(nil)
	s.ErrorIs(err, ErrNilInput)
	_, err = InformAttack(&InformAttackInput{Actor: s.rogue(), Attack: dagger().Attack})
	s.ErrorIs(err, ErrNilInput)
	_, err = InformAttack(&InformAttackInput{Observed: theirs, Actor: s.rogue(), Attack: dagger().Attack})
	s.Error(err)
	_, err = InformAttack(&InformAttackInput{
		Observed: mine, Actor: s.rogue(), Attack: dagger().Attack, Targets: []string{informGoblin1, informGoblin1},
	})
	s.Error(err)
}
