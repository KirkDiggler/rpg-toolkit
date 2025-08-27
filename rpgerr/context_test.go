package rpgerr_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

type ContextTestSuite struct {
	suite.Suite
}

func TestContextSuite(t *testing.T) {
	suite.Run(t, new(ContextTestSuite))
}

func (s *ContextTestSuite) TestContextMetadataAccumulation() {
	// Start with base context
	ctx := context.Background()

	// Add game-level metadata
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("game_id", "game-123"),
		rpgerr.Meta("turn", 5),
	)

	// Add player-level metadata
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("player_id", "player-456"),
		rpgerr.Meta("character", "wizard"),
	)

	// Add action-level metadata
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("action", "cast_spell"),
		rpgerr.Meta("spell", "fireball"),
	)

	// Create error with all accumulated context
	err := rpgerr.ResourceExhaustedCtx(ctx, "spell slots")

	meta := rpgerr.GetMeta(err)
	s.Equal("game-123", meta["game_id"])
	s.Equal(5, meta["turn"])
	s.Equal("player-456", meta["player_id"])
	s.Equal("wizard", meta["character"])
	s.Equal("cast_spell", meta["action"])
	s.Equal("fireball", meta["spell"])
}

func (s *ContextTestSuite) TestContextMetadataOverwrite() {
	ctx := context.Background()

	// Set initial value
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("phase", "main"),
		rpgerr.Meta("priority", "normal"),
	)

	// Overwrite with new value
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("phase", "combat"),
		rpgerr.Meta("priority", "urgent"),
	)

	err := rpgerr.NewCtx(ctx, rpgerr.CodeTimingRestriction, "wrong phase")

	meta := rpgerr.GetMeta(err)
	s.Equal("combat", meta["phase"]) // Should be overwritten
	s.Equal("urgent", meta["priority"])
}

func (s *ContextTestSuite) TestWrapCtx() {
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("pipeline", "AttackPipeline"),
		rpgerr.Meta("attacker", "fighter"),
	)

	// Create a base error
	baseErr := rpgerr.OutOfRange("melee attack",
		rpgerr.WithMeta("distance", 30),
		rpgerr.WithMeta("weapon_range", 5),
	)

	// Wrap with context
	wrapped := rpgerr.WrapCtx(ctx, baseErr, "attack failed")

	meta := rpgerr.GetMeta(wrapped)
	// Should have both original and context metadata
	s.Equal("AttackPipeline", meta["pipeline"])
	s.Equal("fighter", meta["attacker"])
	s.Equal(30, meta["distance"])
	s.Equal(5, meta["weapon_range"])
}

func (s *ContextTestSuite) TestNestedPipelineContext() {
	// Simulate nested pipeline execution with context accumulation

	// Outer pipeline
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("pipeline", "SpellCastPipeline"),
		rpgerr.Meta("spell", "fireball"),
		rpgerr.Meta("caster", "wizard"),
	)

	// Inner pipeline (damage calculation)
	innerCtx := rpgerr.WithMetadata(ctx,
		rpgerr.Meta("pipeline", "DamagePipeline"),
		rpgerr.Meta("damage_type", "fire"),
		rpgerr.Meta("base_damage", 8*6), // 8d6
	)

	// Resistance check
	resistCtx := rpgerr.WithMetadata(innerCtx,
		rpgerr.Meta("stage", "ResistanceCheck"),
		rpgerr.Meta("target", "fire_elemental"),
		rpgerr.Meta("immunity", "fire"),
	)

	// Create error at deepest level
	err := rpgerr.ImmuneCtx(resistCtx, "fire damage")

	meta := rpgerr.GetMeta(err)
	// Should have metadata from all levels
	s.Equal("fireball", meta["spell"])
	s.Equal("wizard", meta["caster"])
	s.Equal("ResistanceCheck", meta["stage"])
	s.Equal("fire_elemental", meta["target"])
	s.Equal("fire", meta["immunity"])
}

func (s *ContextTestSuite) TestAllContextConstructors() {
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("test_id", "test-123"),
	)

	tests := []struct {
		name        string
		constructor func() *rpgerr.Error
		code        rpgerr.Code
	}{
		{
			name:        "NotAllowedCtx",
			constructor: func() *rpgerr.Error { return rpgerr.NotAllowedCtx(ctx, "action") },
			code:        rpgerr.CodeNotAllowed,
		},
		{
			name:        "PrerequisiteNotMetCtx",
			constructor: func() *rpgerr.Error { return rpgerr.PrerequisiteNotMetCtx(ctx, "level 5") },
			code:        rpgerr.CodePrerequisiteNotMet,
		},
		{
			name:        "ResourceExhaustedCtx",
			constructor: func() *rpgerr.Error { return rpgerr.ResourceExhaustedCtx(ctx, "energy") },
			code:        rpgerr.CodeResourceExhausted,
		},
		{
			name:        "OutOfRangeCtx",
			constructor: func() *rpgerr.Error { return rpgerr.OutOfRangeCtx(ctx, "attack") },
			code:        rpgerr.CodeOutOfRange,
		},
		{
			name:        "InvalidTargetCtx",
			constructor: func() *rpgerr.Error { return rpgerr.InvalidTargetCtx(ctx, "self") },
			code:        rpgerr.CodeInvalidTarget,
		},
		{
			name:        "ConflictingStateCtx",
			constructor: func() *rpgerr.Error { return rpgerr.ConflictingStateCtx(ctx, "rage") },
			code:        rpgerr.CodeConflictingState,
		},
		{
			name:        "TimingRestrictionCtx",
			constructor: func() *rpgerr.Error { return rpgerr.TimingRestrictionCtx(ctx, "not your turn") },
			code:        rpgerr.CodeTimingRestriction,
		},
		{
			name:        "CooldownActiveCtx",
			constructor: func() *rpgerr.Error { return rpgerr.CooldownActiveCtx(ctx, "ability") },
			code:        rpgerr.CodeCooldownActive,
		},
		{
			name:        "ImmuneCtx",
			constructor: func() *rpgerr.Error { return rpgerr.ImmuneCtx(ctx, "poison") },
			code:        rpgerr.CodeImmune,
		},
		{
			name:        "BlockedCtx",
			constructor: func() *rpgerr.Error { return rpgerr.BlockedCtx(ctx, "shield") },
			code:        rpgerr.CodeBlocked,
		},
		{
			name:        "InterruptedCtx",
			constructor: func() *rpgerr.Error { return rpgerr.InterruptedCtx(ctx, "counterspell") },
			code:        rpgerr.CodeInterrupted,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := tt.constructor()
			s.Equal(tt.code, rpgerr.GetCode(err))

			meta := rpgerr.GetMeta(err)
			s.Equal("test-123", meta["test_id"], "Context metadata should be preserved")
		})
	}
}

func (s *ContextTestSuite) TestFormattedContextErrors() {
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("character", "rogue"),
		rpgerr.Meta("weapon", "dagger"),
	)

	err := rpgerr.NotAllowedfCtx(ctx, "cannot use %s without proficiency", "longbow")
	s.Contains(err.Error(), "cannot use longbow without proficiency")

	meta := rpgerr.GetMeta(err)
	s.Equal("rogue", meta["character"])
	s.Equal("dagger", meta["weapon"])
}

func (s *ContextTestSuite) TestWrapWithCodeCtx() {
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("session", "session-789"),
	)

	baseErr := rpgerr.New(rpgerr.CodeUnknown, "something failed")
	wrapped := rpgerr.WrapWithCodeCtx(ctx, baseErr, rpgerr.CodeInternal, "system error")

	s.Equal(rpgerr.CodeInternal, rpgerr.GetCode(wrapped))
	meta := rpgerr.GetMeta(wrapped)
	s.Equal("session-789", meta["session"])
}
