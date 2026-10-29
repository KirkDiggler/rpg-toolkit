// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

type blobFlatSuite struct {
	suite.Suite
}

func (s *blobFlatSuite) TestBagKeysAreSiblingsOfTheHeader() {
	b := machine.Blob{
		Ref:      ragingRef,
		MemberID: "barbarian-1",
		State:    stRaging,
		Bag:      machine.Bag{machine.KeyTurnsActive: 3, keyWasHit: true},
	}
	data, err := json.Marshal(&b, json.Deterministic(true))
	s.Require().NoError(err)

	var obj map[string]any
	s.Require().NoError(json.Unmarshal(data, &obj))
	s.Contains(obj, "member_id")
	s.Contains(obj, machine.KeyTurnsActive)
	s.Contains(obj, keyWasHit)
	s.NotContains(obj, "bag")
	s.NotContains(obj, "Bag")
	s.Equal("dnd5e:conditions:raging", obj["ref"])
}

func TestBlobIsFlat(t *testing.T) {
	suite.Run(t, new(blobFlatSuite))
}

type blobTodaySuite struct {
	suite.Suite
}

// todaysRagingBlob is what rulebooks/dnd5e/conditions RagingData writes on
// main: the eight keys, no state.
const todaysRagingBlob = `{"ref":"dnd5e:conditions:raging","member_id":"barbarian-1",` +
	`"source":"dnd5e:features:rage","saw_turn_end":true,"round_activated":2,` +
	`"turns_active":3,"was_hit_this_turn":false,"did_attack_this_turn":true}`

func (s *blobTodaySuite) TestEveryKeyLandsWithItsType() {
	var b machine.Blob
	s.Require().NoError(json.Unmarshal([]byte(todaysRagingBlob), &b))

	s.Equal(ragingRef, b.Ref)
	s.Equal("barbarian-1", b.MemberID)
	s.Equal(&core.Ref{Module: "dnd5e", Type: "features", ID: "rage"}, b.Source)
	s.Empty(b.State, "today's blob carries no state")
	s.Equal(machine.Bag{
		machine.KeySawTurnEnd:     true,
		machine.KeyRoundActivated: 2,
		machine.KeyTurnsActive:    3,
		keyWasHit:                 false,
		keyDidAttack:              true,
	}, b.Bag, "whole numbers land as int, flags as bool")
}

func (s *blobTodaySuite) TestTheProgramAcceptsTodaysBag() {
	var b machine.Blob
	s.Require().NoError(json.Unmarshal([]byte(todaysRagingBlob), &b))
	p, err := rageRecipe().Compile(rageVocabulary())
	s.Require().NoError(err)
	bag, err := p.NewBag(b.Bag)
	s.Require().NoError(err)
	s.Equal(3, bag[machine.KeyTurnsActive])
}

func TestBlobLoadsTodaysRageKeys(t *testing.T) {
	suite.Run(t, new(blobTodaySuite))
}
