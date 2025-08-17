package rpgerr_test

import (
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/stretchr/testify/suite"
)

type ErrorsTestSuite struct {
	suite.Suite
}

func TestErrorsSuite(t *testing.T) {
	suite.Run(t, new(ErrorsTestSuite))
}

func (s *ErrorsTestSuite) TestBasicError() {
	err := rpgerr.ResourceExhausted("energy",
		rpgerr.WithMeta("current", 2),
		rpgerr.WithMeta("required", 5),
	)

	s.Equal(rpgerr.CodeResourceExhausted, rpgerr.GetCode(err))
	s.Equal("insufficient energy", err.Error())

	meta := rpgerr.GetMeta(err)
	s.Equal(2, meta["current"])
	s.Equal(5, meta["required"])
}

func (s *ErrorsTestSuite) TestErrorWrapping() {
	original := errors.New("database connection failed")
	wrapped := rpgerr.Wrap(original, "failed to load character",
		rpgerr.WithMeta("character_id", "char-123"),
	)

	s.Equal(rpgerr.CodeUnknown, rpgerr.GetCode(wrapped))
	s.Contains(wrapped.Error(), "failed to load character")
	s.Contains(wrapped.Error(), "database connection failed")
	s.Equal("char-123", rpgerr.GetMeta(wrapped)["character_id"])
	s.Equal(original, wrapped.Unwrap())
}

func (s *ErrorsTestSuite) TestWrapWithCode() {
	original := errors.New("file not found")
	wrapped := rpgerr.WrapWithCode(original, rpgerr.CodeNotFound, "character not found",
		rpgerr.WithMeta("character_id", "char-456"),
	)

	s.Equal(rpgerr.CodeNotFound, rpgerr.GetCode(wrapped))
	s.Contains(wrapped.Error(), "character not found")
}

func (s *ErrorsTestSuite) TestCallStack() {
	err := rpgerr.New(rpgerr.CodeInvalidTarget, "cannot target ally",
		rpgerr.WithCallStack([]string{"AttackPipeline", "TargetValidation"}),
	)

	stack := rpgerr.GetCallStack(err)
	s.Len(stack, 2)
	s.Equal("AttackPipeline", stack[0])
	s.Equal("TargetValidation", stack[1])

	// Test adding to call stack
	err2 := rpgerr.Wrap(err, "attack failed",
		rpgerr.AddToCallStack("CombatSystem"),
	)

	stack2 := rpgerr.GetCallStack(err2)
	s.Len(stack2, 3)
	s.Equal("CombatSystem", stack2[2])
}

func (s *ErrorsTestSuite) TestErrorCodeHelpers() {
	tests := []struct {
		name     string
		err      *rpgerr.Error
		checkFn  func(error) bool
		expected bool
	}{
		{
			name:     "IsResourceExhausted true",
			err:      rpgerr.ResourceExhausted("energy"),
			checkFn:  rpgerr.IsResourceExhausted,
			expected: true,
		},
		{
			name:     "IsResourceExhausted false",
			err:      rpgerr.OutOfRange("attack"),
			checkFn:  rpgerr.IsResourceExhausted,
			expected: false,
		},
		{
			name:     "IsNotAllowed",
			err:      rpgerr.NotAllowed("cast spell while silenced"),
			checkFn:  rpgerr.IsNotAllowed,
			expected: true,
		},
		{
			name:     "IsPrerequisiteNotMet",
			err:      rpgerr.PrerequisiteNotMet("level 5 required"),
			checkFn:  rpgerr.IsPrerequisiteNotMet,
			expected: true,
		},
		{
			name:     "IsOutOfRange",
			err:      rpgerr.OutOfRange("movement"),
			checkFn:  rpgerr.IsOutOfRange,
			expected: true,
		},
		{
			name:     "IsInvalidTarget",
			err:      rpgerr.InvalidTarget("cannot target self"),
			checkFn:  rpgerr.IsInvalidTarget,
			expected: true,
		},
		{
			name:     "IsConflictingState",
			err:      rpgerr.ConflictingState("rage and concentration"),
			checkFn:  rpgerr.IsConflictingState,
			expected: true,
		},
		{
			name:     "IsTimingRestriction",
			err:      rpgerr.TimingRestriction("not your turn"),
			checkFn:  rpgerr.IsTimingRestriction,
			expected: true,
		},
		{
			name:     "IsCooldownActive",
			err:      rpgerr.CooldownActive("second wind"),
			checkFn:  rpgerr.IsCooldownActive,
			expected: true,
		},
		{
			name:     "IsImmune",
			err:      rpgerr.Immune("fire damage"),
			checkFn:  rpgerr.IsImmune,
			expected: true,
		},
		{
			name:     "IsBlocked",
			err:      rpgerr.Blocked("shield spell"),
			checkFn:  rpgerr.IsBlocked,
			expected: true,
		},
		{
			name:     "IsInterrupted",
			err:      rpgerr.Interrupted("counterspell"),
			checkFn:  rpgerr.IsInterrupted,
			expected: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expected, tt.checkFn(tt.err))
		})
	}
}

func (s *ErrorsTestSuite) TestMetadataPreservation() {
	// Create an error with metadata
	err1 := rpgerr.ResourceExhausted("spell slots",
		rpgerr.WithMeta("spell_level", 3),
		rpgerr.WithMeta("caster", "wizard"),
	)

	// Wrap it and add more metadata
	err2 := rpgerr.Wrap(err1, "cannot cast fireball",
		rpgerr.WithMeta("target_count", 5),
	)

	// Original metadata should be preserved
	meta := rpgerr.GetMeta(err2)
	s.Equal(3, meta["spell_level"])
	s.Equal("wizard", meta["caster"])
	s.Equal(5, meta["target_count"])
}

func (s *ErrorsTestSuite) TestNilErrorHandling() {
	// Wrapping nil should create a CodeNil error
	err := rpgerr.Wrap(nil, "something went wrong")
	s.Equal(rpgerr.CodeNil, rpgerr.GetCode(err))
	s.Contains(err.Error(), "nil")
	s.True(rpgerr.IsNil(err))

	// WrapWithCode with nil
	err2 := rpgerr.WrapWithCode(nil, rpgerr.CodeNotFound, "not found")
	s.Equal(rpgerr.CodeNil, rpgerr.GetCode(err2))
	s.True(rpgerr.IsNil(err2))
}

func (s *ErrorsTestSuite) TestFormattedErrors() {
	err := rpgerr.ResourceExhaustedf("insufficient %s: need %d, have %d", "energy", 5, 2)
	s.Equal("insufficient energy: need 5, have 2", err.Error())

	err2 := rpgerr.NotAllowedf("cannot %s while %s", "attack", "stunned")
	s.Equal("cannot attack while stunned", err2.Error())
}
