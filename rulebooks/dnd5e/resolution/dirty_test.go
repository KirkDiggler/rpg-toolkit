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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// DirtyTestSuite proves the write half of an effect's owner handle: a condition
// that changes its OWN persisted state comes back from Resolve to be stored.
//
// Resolve returns only participants reporting IsDirty, and until
// rpg-toolkit#1251 nothing a condition did set that flag. It survived on
// coincidence — a rogue who spent a sneak attack also paid an action, and
// paying an action marks the sheet; a raging barbarian who was hit also took
// damage, and damage marks the sheet. The state change rode along with
// something else that happened to mark it.
//
// That coincidence runs out exactly where it is most needed. A turn boundary
// becomes an interaction whose ENTIRE purpose is condition state, with no
// damage and no economy to ride on, so every expiry would fire correctly and
// then be discarded. This is the prerequisite for that clock, which is why it
// gets an isolating test rather than an incidental one.
type DirtyTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestDirtySuite(t *testing.T) {
	suite.Run(t, new(DirtyTestSuite))
}

func (s *DirtyTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// ragingHero is a barbarian who is already raging and is about to swing.
//
// The swing is what makes this test isolating. A raging character records
// DidAttackThisTurn when their attack resolves — and an ATTACKER takes no
// damage, so nothing touches their hit points, and this interaction carries no
// Cost, so nothing touches their action economy. Those are the two accidents
// that were marking sheets dirty on the condition's behalf. With both gone,
// RagingCondition.markDirty is the only thing left that can put this sheet in
// the output.
func (s *DirtyTestSuite) ragingHero() *character.Data {
	raging, err := (&conditions.RagingCondition{
		CharacterID: heroID,
		DamageBonus: 2,
		Level:       1,
		Source:      "dnd5e:features:rage",
	}).ToJSON()
	s.Require().NoError(err)

	return &character.Data{
		ID:       heroID,
		PlayerID: "player-1",
		Name:     "Standre",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints:        20,
		MaxHitPoints:     20,
		ArmorClass:       12,
		ProficiencyBonus: 2,
		Conditions:       []json.RawMessage{raging},
	}
}

func (s *DirtyTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// TestAConditionThatChangesItselfComesBackToBeStored is the isolating test.
//
// The hero swings, so takes no damage; the interaction has no Cost, so pays no
// economy. If markDirty were removed, DidAttackThisTurn would still be set on
// the live object and the sheet would simply not be returned — the update
// applied, then silently thrown away. That is the failure this guards, and it
// is invisible to any test whose subject also took damage or paid an action.
func (s *DirtyTestSuite) TestAConditionThatChangesItselfComesBackToBeStored() {
	hero := s.ragingHero()
	bite := monsters.NewWolf(wolfID).ToData().Actions[0]

	out, err := Resolve(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: hero}, {Monster: monsters.NewWolf(wolfID).ToData()}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: heroID,
			TargetID:   wolfID,
			Definition: bite,
			Roller:     &sequenceRoller{singles: []int{18, 18}, pair: []int{3, 4}},
		}),
		// No Cost. A free action, which is what every Resolve was before the
		// door existed — and the point: nothing here debits an economy.
	})
	s.Require().NoError(err)

	s.Require().Len(out.DirtyCharacters, 1,
		"the attacker took no damage and paid no economy: if this sheet came back, "+
			"a condition asked for it")
	s.Require().Equal(heroID, out.DirtyCharacters[0].ID)

	s.Require().Len(out.DirtyCharacters[0].Conditions, 1)
	var stored map[string]any
	s.Require().NoError(json.Unmarshal(out.DirtyCharacters[0].Conditions[0], &stored))
	s.Require().Equal(true, stored["did_attack_this_turn"],
		"and the change is IN the blob that gets stored, not only on the live object")
}

// TestAnUntouchedParticipantIsNotReturned is the other half: marking is not
// indiscriminate.
//
// The wolf is attacked and misses nothing — it is the target, so it takes the
// damage and legitimately comes back. What must NOT happen is every
// participant coming back on every interaction, which would make the dirty set
// meaningless and hand the host a write per sheet per swing.
func (s *DirtyTestSuite) TestAnUntouchedParticipantIsNotReturned() {
	hero := s.ragingHero()
	bystander := monsters.NewWolf(secondWolfID).ToData()
	bite := monsters.NewWolf(wolfID).ToData().Actions[0]

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 6}},
			{ID: secondWolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	out, err := Resolve(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: enc.ToData(),
		Participants: []Participant{
			{Character: hero},
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Monster: bystander},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: heroID,
			TargetID:   wolfID,
			Definition: bite,
			Roller:     &sequenceRoller{singles: []int{18, 18}, pair: []int{3, 4}},
		}),
	})
	s.Require().NoError(err)

	for _, m := range out.DirtyMonsters {
		s.Require().NotEqual(secondWolfID, m.ID,
			"a participant nothing happened to must not be written back (R3 says pass "+
				"everyone in; it does not say charge for everyone)")
	}
}
