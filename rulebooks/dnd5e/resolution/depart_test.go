// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	departAlice = "alice"
	departBob   = "bob"
	departCarol = "carol"
)

// DepartTestSuite proves a leaver is taken out of every hold that names it,
// the hold continuing for whoever it still holds.
type DepartTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestDepartSuite(t *testing.T) {
	suite.Run(t, new(DepartTestSuite))
}

func (s *DepartTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *DepartTestSuite) sheet(id string) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "depart-player", Name: id,
		Level: 1, ProficiencyBonus: 2, RaceID: races.Human, ClassID: classes.Cleric,
		Levels: syntheticLevels(classes.Cleric, 1, 8, 5),
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 10, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 14, abilities.CHA: 10,
		},
		HitPoints: 9, MaxHitPoints: 9,
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.HitDice: {Current: 1, Maximum: 1, ResetType: coreResources.ResetLongRest},
		},
	}
}

func (s *DepartTestSuite) blessed(memberID string) (dnd5eEvents.ConditionAddress, json.RawMessage) {
	condition, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
		MemberID: memberID, SourceID: departBob, SourceRef: refs.Spells.Bless(),
	})
	s.Require().NoError(err)
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	return dnd5eEvents.ConditionAddress{
		MemberID: memberID, ConditionRef: refs.Conditions.Blessed().String(), SourceID: departBob,
	}, raw
}

// scene is bob holding Bless on the named targets, each target's sheet
// carrying Blessed.
func (s *DepartTestSuite) scene(targets ...string) map[string]*character.Data {
	sheets := map[string]*character.Data{
		departAlice: s.sheet(departAlice), departBob: s.sheet(departBob), departCarol: s.sheet(departCarol),
	}
	hold := conditions.NewConcentratingConditionWithInput(conditions.NewConcentratingConditionInput{
		MemberID: departBob, SourceID: departBob, SpellRef: refs.Spells.Bless().String(),
		SpellName: "Bless", TurnEnds: 10,
	})
	for _, target := range targets {
		address, raw := s.blessed(target)
		s.Require().NoError(hold.AddChild(s.ctx, address))
		sheets[target].Conditions = append(sheets[target].Conditions, raw)
	}
	holdJSON, err := hold.ToJSON()
	s.Require().NoError(err)
	sheets[departBob].Conditions = append(sheets[departBob].Conditions, holdJSON)
	return sheets
}

func departRefs(s *DepartTestSuite, data *character.Data) []string {
	var out []string
	for _, raw := range data.Conditions {
		var head struct {
			Ref *core.Ref `json:"ref"`
		}
		s.Require().NoError(json.Unmarshal(raw, &head))
		s.Require().NotNil(head.Ref)
		out = append(out, head.Ref.String())
	}
	return out
}

func (s *DepartTestSuite) holdOf(data *character.Data) *conditions.ConcentratingConditionData {
	for _, raw := range data.Conditions {
		var hold conditions.ConcentratingConditionData
		s.Require().NoError(json.Unmarshal(raw, &hold))
		if hold.Ref != nil && hold.Ref.String() == refs.Conditions.Concentrating().String() {
			return &hold
		}
	}
	return nil
}

func dirtyByID(records []*character.Data) map[string]*character.Data {
	out := map[string]*character.Data{}
	for _, record := range records {
		out[record.ID] = record
	}
	return out
}

func (s *DepartTestSuite) TestDepart() {
	blessedRef := refs.Conditions.Blessed().String()

	s.Run("bless on alice and carol: alice departs, the hold continues for carol", func() {
		sheets := s.scene(departAlice, departCarol)

		out, err := Depart(s.ctx, &DepartInput{
			Character: sheets[departAlice],
			Others:    []Participant{{Character: sheets[departBob]}, {Character: sheets[departCarol]}},
		})
		s.Require().NoError(err)

		s.NotContains(departRefs(s, out.Character), blessedRef, "alice has no Blessed")
		s.Require().Len(out.Ended, 1)
		s.Equal(encounter.ResultConditionRemoved, out.Ended[0].Kind)
		s.Equal(blessedRef, out.Ended[0].Address.ConditionRef)
		s.Equal(DepartedReason, out.Ended[0].Reason)

		dirty := dirtyByID(out.DirtyCharacters)
		s.Require().Contains(dirty, departBob, "bob's hold changed")
		s.NotContains(dirty, departCarol, "carol is untouched")
		hold := s.holdOf(dirty[departBob])
		s.Require().NotNil(hold, "the hold continues")
		s.Equal([]dnd5eEvents.ConditionAddress{{
			MemberID: departCarol, ConditionRef: blessedRef, SourceID: departBob,
		}}, hold.Children, "the hold names only carol")
		s.Empty(out.ConcentrationBreaks, "a hold that continues did not break")
	})

	s.Run("alice was the only target: the hold ends, departed", func() {
		sheets := s.scene(departAlice)
		bus := events.NewEventBus()
		var ended []dnd5eEvents.ConcentrationEndedEvent
		_, err := dnd5eEvents.ConcentrationEndedTopic.On(bus).Subscribe(s.ctx,
			func(_ context.Context, event dnd5eEvents.ConcentrationEndedEvent) error {
				ended = append(ended, event)
				return nil
			})
		s.Require().NoError(err)

		out, err := departOn(s.ctx, &DepartInput{
			Character: sheets[departAlice],
			Others:    []Participant{{Character: sheets[departBob]}, {Character: sheets[departCarol]}},
		}, newSurface(bus))
		s.Require().NoError(err)
		s.Require().Len(ended, 1, "bob's hold ended once")
		s.Equal(departBob, ended[0].CasterID)
		s.Equal(DepartedReason, ended[0].Reason, "concentration ended, departed")

		s.NotContains(departRefs(s, out.Character), blessedRef)
		s.Empty(out.Ended, "the removal is the break's to report, once")

		dirty := dirtyByID(out.DirtyCharacters)
		s.Require().Contains(dirty, departBob)
		s.Nil(s.holdOf(dirty[departBob]), "bob's hold ended")

		s.Require().Len(out.ConcentrationBreaks, 1, "the ended hold reaches the session")
		held := out.ConcentrationBreaks[0]
		s.Equal(encounter.MemberID(departBob), held.Caster, "the caster to tell")
		s.Equal(refs.Spells.Bless().String(), held.Spell.Ref)
		s.Equal(DepartedReason, held.Reason)
		s.Require().Len(held.Removed, 1, "alice's Blessed, once")
		s.Equal(encounter.MemberID(departAlice), held.Removed[0].Address.MemberID)
		s.Equal(DepartedReason, held.Removed[0].Reason)
	})

	s.Run("bob departs: his own hold ends and its effects come off alice and carol", func() {
		sheets := s.scene(departAlice, departCarol)

		out, err := Depart(s.ctx, &DepartInput{
			Character: sheets[departBob],
			Others:    []Participant{{Character: sheets[departAlice]}, {Character: sheets[departCarol]}},
		})
		s.Require().NoError(err)

		s.Nil(s.holdOf(out.Character), "bob holds nothing")
		s.Empty(out.Ended, "nothing came off bob himself")

		s.Require().Len(out.ConcentrationBreaks, 1, "one break for Bless")
		held := out.ConcentrationBreaks[0]
		s.Equal(encounter.MemberID(departBob), held.Caster)
		s.Equal(refs.Spells.Bless().String(), held.Spell.Ref)
		s.Equal(DepartedReason, held.Reason)
		var removedFrom []encounter.MemberID
		for _, removed := range held.Removed {
			removedFrom = append(removedFrom, removed.Address.MemberID)
		}
		s.ElementsMatch([]encounter.MemberID{departAlice, departCarol}, removedFrom, "one removal per target")

		dirty := dirtyByID(out.DirtyCharacters)
		s.Require().Contains(dirty, departAlice)
		s.Require().Contains(dirty, departCarol)
		s.NotContains(departRefs(s, dirty[departAlice]), blessedRef)
		s.NotContains(departRefs(s, dirty[departCarol]), blessedRef)
	})

	s.Run("bob departs with a target not passed in: refused, inputs unchanged", func() {
		sheets := s.scene(departAlice, departCarol)
		before, err := json.Marshal(sheets)
		s.Require().NoError(err)

		out, err := Depart(s.ctx, &DepartInput{
			Character: sheets[departBob],
			Others:    []Participant{{Character: sheets[departAlice]}},
		})
		s.Require().ErrorIs(err, ErrBadParticipant)
		s.Require().ErrorContains(err, departCarol)
		s.Require().Nil(out)

		after, err := json.Marshal(sheets)
		s.Require().NoError(err)
		s.JSONEq(string(before), string(after))
	})

	s.Run("an absent cleric's Bless refuses: a concentration spell, per content", func() {
		sheets := s.scene(departAlice, departCarol)
		before, err := json.Marshal(sheets)
		s.Require().NoError(err)

		out, err := Depart(s.ctx, &DepartInput{
			Character: sheets[departAlice],
			Others:    []Participant{{Character: sheets[departCarol]}},
		})
		s.Require().ErrorIs(err, ErrBadParticipant)
		s.Require().ErrorContains(err, departBob)
		s.Require().Nil(out)

		after, err := json.Marshal(sheets)
		s.Require().NoError(err)
		s.JSONEq(string(before), string(after))
	})

	s.Run("a leaver with no held effects: empty output, nothing dirty", func() {
		sheets := s.scene(departCarol)

		out, err := Depart(s.ctx, &DepartInput{
			Character: sheets[departAlice],
			Others:    []Participant{{Character: sheets[departBob]}, {Character: sheets[departCarol]}},
		})
		s.Require().NoError(err)
		s.Empty(out.Ended)
		s.Empty(out.DirtyCharacters)
		s.Empty(out.DirtyMonsters)
	})

	s.Run("an absent bard's Inspired does not refuse: not a concentration effect, per content", func() {
		alice := s.sheet(departAlice)
		inspired, err := conditions.NewInspiredCondition(departAlice, "absent-bard", "").ToJSON()
		s.Require().NoError(err)
		alice.Conditions = []json.RawMessage{inspired}

		out, err := Depart(s.ctx, &DepartInput{Character: alice})
		s.Require().NoError(err)
		s.Empty(out.Ended)
		s.Contains(departRefs(s, out.Character), refs.Conditions.Inspired().String())
	})

	s.Run("an absent bard's Vicious Mockery does not refuse: a spell, but not concentration, per content", func() {
		alice := s.sheet(departAlice)
		mocked, err := conditions.NewViciousMockeryCondition(departAlice, "absent-bard", "").ToJSON()
		s.Require().NoError(err)
		alice.Conditions = []json.RawMessage{mocked}

		out, err := Depart(s.ctx, &DepartInput{Character: alice})
		s.Require().NoError(err)
		s.Empty(out.Ended)
		s.Contains(departRefs(s, out.Character), refs.Conditions.ViciousMockery().String())
	})

	s.Run("nil input", func() {
		out, err := Depart(s.ctx, nil)
		s.Require().ErrorIs(err, ErrNilInput)
		s.Require().Nil(out)
	})
}
