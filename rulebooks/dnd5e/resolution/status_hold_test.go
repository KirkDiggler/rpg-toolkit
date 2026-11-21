// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	effects "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// StatusHoldSuite proves a monster keeps, saves for and loses a status an
// author wrote, by the same concentration tie a spell uses.
type StatusHoldSuite struct {
	suite.Suite

	ctx context.Context
}

func TestStatusHoldSuite(t *testing.T) {
	suite.Run(t, new(StatusHoldSuite))
}

func (s *StatusHoldSuite) SetupTest() {
	s.ctx = context.Background()
}

const (
	heldID   = "held"
	heldName = "Held"
)

func (s *StatusHoldSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

// hold delivers the status by hand, with no session: the wolf keeps it and the
// hero and the bard each hold the receiving half.
func (s *StatusHoldSuite) hold() (*monster.Data, *character.Data, *character.Data) {
	f := s.fixtures()
	wolf := f.wolfData()
	hero := f.saver(14)
	bard := f.bard(1)

	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{
		ID:            "status:held",
		CoordinatorID: wolfID,
		Origin:        dnd5eEvents.ConditionOrigin{Ref: StatusSourceRef(heldID), EntityID: wolfID},
		Existing: []conditions.ParticipantConditions{
			{MemberID: wolfID}, {MemberID: heroID}, {MemberID: bardID},
		},
		Spec: conditions.ApplicationSpec{
			Name: heldName,
			CoordinatorParts: []effects.PartDeclaration{
				{Key: "hold", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication},
				{Key: "empty", Kind: effects.KindUntilEmpty, OnTerminal: generic.EndApplication},
			},
			Recipients: []conditions.RecipientSpec{
				s.recipient(heroID), s.recipient(bardID),
			},
		},
	})
	s.Require().NoError(err)

	owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	s.Require().NoError(err)
	raw, err := owner.Condition.ToJSON()
	s.Require().NoError(err)
	var data conditions.LinkedConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	data.Runtime.Lifetime.LiveMembers = append([]string{}, data.Runtime.Lifetime.PendingMembers...)
	data.Runtime.Lifetime.PendingMembers = []string{}
	raw, err = json.Marshal(data)
	s.Require().NoError(err)
	wolf.Conditions = append(wolf.Conditions, raw)

	for i, holder := range []*character.Data{hero, bard} {
		created, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: i})
		s.Require().NoError(err)
		blob, err := created.Condition.ToJSON()
		s.Require().NoError(err)
		holder.Conditions = append(holder.Conditions, blob)
	}

	return wolf, hero, bard
}

func (s *StatusHoldSuite) recipient(member string) conditions.RecipientSpec {
	return conditions.RecipientSpec{
		MemberID: member, Ref: refs.Conditions.LinkedEffects().String(), Name: heldName,
		AddressSourceID: wolfID, Lifetime: conditions.LifetimeShared,
		Parts: []effects.PartDeclaration{
			{Key: "cannot", Kind: effects.KindCannot, Params: json.RawMessage(`{"what":["move"]}`), OnTerminal: generic.EndPart},
			{Key: "combat", Kind: effects.KindEndOn, Params: json.RawMessage(`{"beat":"combat_start","reason":"combat_start"}`), OnTerminal: generic.EndAttachment},
		},
	}
}

// strike has the hero hit the wolf with a claw of the given dice.
func (s *StatusHoldSuite) strike(
	wolf *monster.Data, hero, bard *character.Data, dieSize string, roller dice.Roller,
) (*Output, error) {
	return s.strikeAt(heroID, wolfID, wolf, hero, bard, dieSize, roller)
}

func (s *StatusHoldSuite) strikeAt(
	attackerID, targetID string, wolf *monster.Data, hero, bard *character.Data, dieSize string, roller dice.Roller,
) (*Output, error) {
	f := s.fixtures()
	return resolveOn(s.ctx, &Input{
		World:        f.world(),
		Participants: []Participant{{Character: hero}, {Character: bard}, {Monster: wolf}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: attackerID, TargetID: targetID, Definition: claw(dieSize), Roller: roller,
		}),
		Capabilities: encounter.Capabilities{
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Participation: everyoneParticipates{},
			Sight:         everyoneSeesTheWholeMap{},
			Roller:        dice.NewRoller(),
			Equipment:     noHandsAreObserved{},
			Conditions:    noHandsAreObserved{},
			Sheets:        noSheetsAsked{},
			Actors:        Actors,
		},
	}, newSurface(events.NewEventBus()))
}

func (s *StatusHoldSuite) heldBy(out *Output, id string) bool {
	for _, data := range out.DirtyCharacters {
		if data.ID != id {
			continue
		}
		for _, ref := range storedRefs(s.T(), data.Conditions) {
			if ref == refs.Conditions.LinkedEffects().String() {
				return true
			}
		}
		return false
	}
	// A sheet that did not change still holds what it held.
	return true
}

func (s *StatusHoldSuite) TestAMonsterHoldingAStatusIsAdmittedToAStrike() {
	wolf, hero, bard := s.hold()

	out, err := s.strike(wolf, hero, bard, "1d6",
		&sequenceRoller{singles: []int{straightRoll, 18}, pair: []int{4}})
	s.Require().NoError(err)
	s.Require().NotNil(out)
}

func (s *StatusHoldSuite) TestAMonsterThatFailsTheCheckTakesTheStatusOffEveryone() {
	wolf, hero, bard := s.hold()

	// The claw's d20, then the wolf's concentration save: 3 fails DC 10.
	out, err := s.strike(wolf, hero, bard, "1d6",
		&sequenceRoller{singles: []int{straightRoll, 3}, pair: []int{4}})
	s.Require().NoError(err)

	s.Require().Len(out.ConcentrationBreaks, 1)
	broke := out.ConcentrationBreaks[0]
	s.Equal(encounter.MemberID(wolfID), broke.Caster)
	s.Equal(statusRefString(), broke.Spell.Ref)
	s.Equal(heldName, broke.Spell.Name)
	s.Equal(conditions.ConcentrationEndedDamage, broke.Reason)
	s.NotNil(broke.Save)
	s.Require().Len(broke.Removed, 2)
	for _, removed := range broke.Removed {
		s.Equal(heldName, removed.Name)
	}
	s.False(s.heldBy(out, heroID), "the hero no longer holds it")
	s.False(s.heldBy(out, bardID), "the bard no longer holds it")
}

func (s *StatusHoldSuite) TestAMonsterThatMakesTheCheckKeepsItAndItIsRecorded() {
	wolf, hero, bard := s.hold()

	out, err := s.strike(wolf, hero, bard, "1d6",
		&sequenceRoller{singles: []int{straightRoll, 18}, pair: []int{4}})
	s.Require().NoError(err)

	s.Require().Len(out.ConcentrationChecks, 1)
	s.Equal(heldName, out.ConcentrationChecks[0].Spell.Name)
	s.Equal(statusRefString(), out.ConcentrationChecks[0].Spell.Ref)
	s.Empty(out.ConcentrationBreaks)
	s.True(s.heldBy(out, heroID))
	s.True(s.heldBy(out, bardID))
}

func (s *StatusHoldSuite) TestAMonsterDroppedToZeroTakesTheStatusOffEveryone() {
	wolf, hero, bard := s.hold()

	out, err := s.strike(wolf, hero, bard, "1d12",
		&sequenceRoller{singles: []int{straightRoll}, pair: []int{12}})
	s.Require().NoError(err)

	s.Require().Len(out.ConcentrationBreaks, 1)
	broke := out.ConcentrationBreaks[0]
	s.Equal(conditions.ConcentrationEndedCasterDown, broke.Reason)
	s.Nil(broke.Save)
	s.Equal(heldName, broke.Spell.Name)
	s.Require().Len(broke.Removed, 2)
	s.False(s.heldBy(out, heroID))
	s.False(s.heldBy(out, bardID))
}

// A spell's hold keeps reading as it did: Bless's receiver is named for what
// it is, not for the linked wrapper.
func (s *StatusHoldSuite) TestASpellHoldStillReadsAsItDid() {
	f := s.fixtures()
	blessed := fixtureNamed(s.T(), namedFixtureInput{
		Ref: refs.Conditions.Blessed(), Member: heroID, Source: bardID, Origin: refs.Spells.Bless(),
	})
	hold := fixtureHold(s.T(), holdFixtureInput{
		Member: bardID, Source: refs.Spells.Bless(), Name: "Bless", Turns: 10,
		Children: []dnd5eEvents.ConditionAddress{blessed.ConditionAddress()},
	})
	holdJSON, err := hold.ToJSON()
	s.Require().NoError(err)
	blessedJSON, err := blessed.ToJSON()
	s.Require().NoError(err)
	hero := f.saver(14, blessedJSON)
	bard := f.bard(1)
	bard.Conditions = append(bard.Conditions, holdJSON)

	out, err := s.strikeAt(wolfID, bardID, f.wolfData(), hero, bard, "1d6",
		&sequenceRoller{singles: []int{straightRoll, 3}, pair: []int{4}})
	s.Require().NoError(err)

	s.Require().Len(out.ConcentrationBreaks, 1)
	broke := out.ConcentrationBreaks[0]
	s.Equal(refs.Spells.Bless().String(), broke.Spell.Ref)
	s.Require().Len(broke.Removed, 1)
	s.Equal("Blessed", broke.Removed[0].Name)
}

func statusRefString() string {
	ref := StatusSourceRef(heldID)
	return ref.String()
}
