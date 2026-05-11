// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/stretchr/testify/suite"
)

// ReactionReadinessSuite exercises gamectx.IsReactionReady and
// gamectx.WithReactionReadiness.
type ReactionReadinessSuite struct {
	suite.Suite
}

func TestReactionReadinessSuite(t *testing.T) {
	suite.Run(t, new(ReactionReadinessSuite))
}

const (
	testOARef     = "dnd5e:conditions:opportunity_attack"
	testShieldRef = "dnd5e:conditions:shield"
	testCharAlice = "char-alice"
	testCharBob   = "char-bob"
)

// --- No context ---

func (s *ReactionReadinessSuite) TestIsReactionReady_NoContext_ReturnsFalse() {
	ctx := context.Background()
	s.False(gamectx.IsReactionReady(ctx, testCharAlice, testOARef),
		"missing context should return false (safe default)")
}

// --- With context ---

func (s *ReactionReadinessSuite) TestIsReactionReady_WithReadyReaction_ReturnsTrue() {
	ctx := gamectx.WithReactionReadiness(context.Background(), gamectx.ReactionReadinessMap{
		testCharAlice: {testOARef: true},
	})
	s.True(gamectx.IsReactionReady(ctx, testCharAlice, testOARef))
}

func (s *ReactionReadinessSuite) TestIsReactionReady_WithNotReadyReaction_ReturnsFalse() {
	ctx := gamectx.WithReactionReadiness(context.Background(), gamectx.ReactionReadinessMap{
		testCharAlice: {testOARef: false},
	})
	s.False(gamectx.IsReactionReady(ctx, testCharAlice, testOARef))
}

func (s *ReactionReadinessSuite) TestIsReactionReady_EntityNotInMap_ReturnsFalse() {
	ctx := gamectx.WithReactionReadiness(context.Background(), gamectx.ReactionReadinessMap{
		testCharAlice: {testOARef: true},
	})
	s.False(gamectx.IsReactionReady(ctx, testCharBob, testOARef),
		"entity not in map should return false")
}

func (s *ReactionReadinessSuite) TestIsReactionReady_ReactionRefNotInInnerMap_ReturnsFalse() {
	ctx := gamectx.WithReactionReadiness(context.Background(), gamectx.ReactionReadinessMap{
		testCharAlice: {testOARef: true},
	})
	s.False(gamectx.IsReactionReady(ctx, testCharAlice, testShieldRef),
		"reaction ref not in entity map should return false")
}

func (s *ReactionReadinessSuite) TestIsReactionReady_EmptyMap_ReturnsFalse() {
	ctx := gamectx.WithReactionReadiness(context.Background(), gamectx.ReactionReadinessMap{})
	s.False(gamectx.IsReactionReady(ctx, testCharAlice, testOARef))
}

func (s *ReactionReadinessSuite) TestIsReactionReady_NilMap_ReturnsFalse() {
	ctx := gamectx.WithReactionReadiness(context.Background(), nil)
	s.False(gamectx.IsReactionReady(ctx, testCharAlice, testOARef))
}

func (s *ReactionReadinessSuite) TestIsReactionReady_MultipleEntities() {
	ctx := gamectx.WithReactionReadiness(context.Background(), gamectx.ReactionReadinessMap{
		testCharAlice: {testOARef: true, testShieldRef: false},
		testCharBob:   {testOARef: false, testShieldRef: true},
	})

	// alice: OA ready, shield not ready
	s.True(gamectx.IsReactionReady(ctx, testCharAlice, testOARef))
	s.False(gamectx.IsReactionReady(ctx, testCharAlice, testShieldRef))

	// bob: OA not ready, shield ready
	s.False(gamectx.IsReactionReady(ctx, testCharBob, testOARef))
	s.True(gamectx.IsReactionReady(ctx, testCharBob, testShieldRef))
}
