// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type NimbleEscapeTestSuite struct {
	suite.Suite
}

func TestNimbleEscapeTestSuite(t *testing.T) {
	suite.Run(t, new(NimbleEscapeTestSuite))
}

// TestRoundTripsThroughTheLoader: the stored blob names the trait and its
// owner, and the one loader restores a trait that writes the same blob back.
func (s *NimbleEscapeTestSuite) TestRoundTripsThroughTheLoader() {
	stored, err := conditions.NimbleEscape("goblin-1").ToJSON()
	s.Require().NoError(err)

	var data conditions.NimbleEscapeData
	s.Require().NoError(json.Unmarshal(stored, &data))
	s.Equal(refs.MonsterTraits.NimbleEscape().String(), data.Ref.String())
	s.Equal("goblin-1", data.OwnerID)

	loaded, err := conditions.LoadJSON(stored)
	s.Require().NoError(err)
	s.Equal(refs.MonsterTraits.NimbleEscape(), loaded.Ref())

	again, err := loaded.ToJSON()
	s.Require().NoError(err)
	s.JSONEq(string(stored), string(again))
}

// TestFactoryBlobIsTheTraitsOwnBlob: what a stat block stores before any bus
// exists is exactly what the trait itself would persist.
func (s *NimbleEscapeTestSuite) TestFactoryBlobIsTheTraitsOwnBlob() {
	fromTrait, err := conditions.NimbleEscape("goblin-1").ToJSON()
	s.Require().NoError(err)
	s.JSONEq(string(fromTrait), string(conditions.MustNimbleEscapeJSON("goblin-1")))
}

// TestTheFactIsReadOffTheStoredSheet is the engine's read: a seam deciding a
// turn asks HoldsRef of the stored conditions, loading nothing.
func (s *NimbleEscapeTestSuite) TestTheFactIsReadOffTheStoredSheet() {
	stored := []json.RawMessage{conditions.MustNimbleEscapeJSON("goblin-1")}

	holds, err := conditions.HoldsRef(stored, refs.MonsterTraits.NimbleEscape())
	s.Require().NoError(err)
	s.True(holds, "a sheet carrying the trait may Disengage as a bonus action")

	holds, err = conditions.HoldsRef(stored, refs.MonsterTraits.PackTactics())
	s.Require().NoError(err)
	s.False(holds, "the blob names Nimble Escape and nothing else")
}

// TestDescriptorRenders: the trait has a status-visible name, and its detail
// names only the Disengage half the engine honours.
func (s *NimbleEscapeTestSuite) TestDescriptorRenders() {
	display, ok := conditions.DisplayFor(*refs.MonsterTraits.NimbleEscape())
	s.Require().True(ok, "nimble escape must be registered")
	s.Equal(conditions.Display{
		Name:   "Nimble Escape",
		Detail: "Can take the Disengage action as a bonus action on each of its turns.",
	}, display)
	s.NotContains(display.Detail, "Hide", "the Hide half is not represented")
}

// TestApplyTwiceIsRefusedAndRemoveReleases: the trait subscribes to nothing,
// but it still knows whether it is on a bus.
func (s *NimbleEscapeTestSuite) TestApplyTwiceIsRefusedAndRemoveReleases() {
	ctx := context.Background()
	bus := events.NewEventBus()
	trait := conditions.NimbleEscape("goblin-1")

	s.Require().NoError(trait.Apply(ctx, bus))
	s.True(trait.IsApplied())
	s.Error(trait.Apply(ctx, bus))

	s.Require().NoError(trait.Remove(ctx, bus))
	s.False(trait.IsApplied())
	s.Require().NoError(trait.Apply(ctx, bus), "a removed trait can be applied again")
}
