package environments

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RoomBuilderTestSuite tests room builder functionality
type RoomBuilderTestSuite struct {
	suite.Suite
}

func (s *RoomBuilderTestSuite) TestBasicRoomBuilding() {
	s.Run("builds room with valid dimensions", func() {
		config := BasicRoomBuilderConfig{}
		builder := NewBasicRoomBuilder(config)

		room, err := builder.
			WithSize(10, 10).
			WithTheme("dungeon").
			Build()

		s.Assert().NoError(err)
		s.Assert().NotNil(room)
	})

	s.Run("handles different sizes", func() {
		sizes := [][2]int{
			{5, 5},
			{15, 10},
			{20, 15},
		}

		for _, size := range sizes {
			config := BasicRoomBuilderConfig{}
			builder := NewBasicRoomBuilder(config) // Create fresh builder each time
			room, err := builder.
				WithSize(size[0], size[1]).
				WithTheme("test").
				Build()

			s.Assert().NoError(err, "Size %dx%d should build successfully", size[0], size[1])
			s.Assert().NotNil(room)
		}
	})
}

func (s *RoomBuilderTestSuite) TestWallPatterns() {
	s.Run("builds with different wall patterns", func() {
		patterns := []string{"empty", "random"}

		for _, pattern := range patterns {
			config := BasicRoomBuilderConfig{}
			builder := NewBasicRoomBuilder(config) // Create fresh builder each time
			room, err := builder.
				WithSize(10, 10).
				WithWallPattern(pattern).
				WithTheme("test").
				Build()

			s.Assert().NoError(err, "Pattern %s should build successfully", pattern)
			s.Assert().NotNil(room)
		}
	})
}

// wallSignature captures a built room's wall layout as a comparable value
// (position/type per discretized wall entity), independent of entity
// ID/ordering, for asserting two builds produced the same -- or a
// different -- layout.
func wallSignature(room spatial.Room) []string {
	entities := GetWallEntitiesInRoom(room)
	sig := make([]string, len(entities))
	for i, e := range entities {
		sig[i] = fmt.Sprintf("%v-%d", e.GetPosition(), e.GetWallType())
	}
	sort.Strings(sig)
	return sig
}

// TestBuild_RandomPattern_EntropySeeded covers rpg-toolkit#787: QuickRoom
// (and any other unseeded caller of Build) used to always generate the same
// wall layout because RandomPattern seeded directly off
// PatternParams.RandomSeed, which stayed Go's zero value unless a caller
// explicitly called WithRandomSeed. Every Build() now falls back to an
// entropy seed when unset.
//
// Asserts against patternParams.RandomSeed directly rather than the
// resulting wall layout: RandomPattern's own safety validation collapses a
// wide range of distinct seeds to the same emergency-fallback empty room
// for this package's default shape at 10x10 (most seeds do, in fact --
// verified separately), so "did two builds get different walls" is a flaky
// signal for this specific mechanism even though "did two builds get
// different seeds" is not.
func (s *RoomBuilderTestSuite) TestBuild_RandomPattern_EntropySeeded() {
	s.Run("unseeded builds get distinct, non-zero seeds", func() {
		seeds := make(map[int64]bool)
		for i := 0; i < 5; i++ {
			b := NewBasicRoomBuilder(BasicRoomBuilderConfig{})
			_, err := b.WithSize(10, 10).WithWallPattern(PatternRandom).Build()
			s.Require().NoError(err)
			s.Assert().NotZero(b.patternParams.RandomSeed, "unseeded Build must not leave RandomSeed at its zero value")
			seeds[b.patternParams.RandomSeed] = true
		}
		s.Assert().Len(seeds, 5, "5 unseeded builds should draw 5 distinct entropy seeds, got %v", seeds)
	})

	s.Run("explicit seed is preserved and reproducible", func() {
		build := func() (int64, []string) {
			b := NewBasicRoomBuilder(BasicRoomBuilderConfig{})
			room, err := b.WithSize(10, 10).WithWallPattern(PatternRandom).WithRandomSeed(4).Build()
			s.Require().NoError(err)
			return b.patternParams.RandomSeed, wallSignature(room)
		}

		seed1, layout1 := build()
		seed2, layout2 := build()
		s.Assert().Equal(int64(4), seed1, "WithRandomSeed(4) must not be overwritten by the entropy fallback")
		s.Assert().Equal(seed1, seed2)
		s.Assert().Equal(layout1, layout2, "same explicit seed must reproduce the same wall layout")
		s.Assert().NotEmpty(layout1, "seed 4 is verified non-empty at 10x10 -- empty here means the seed didn't take")
	})
}

// TestQuickRoom_Seed covers the seed seam QuickRoom exposes on top of
// Build()'s entropy default: an explicit seed reproduces a layout, and
// different explicit seeds diverge (rpg-toolkit#787). Seeds 4 and 10 are
// pinned rather than arbitrary: both are verified to produce a non-empty,
// mutually different wall layout at 10x10 with this package's default
// shape, avoiding the same empty-fallback collision
// TestBuild_RandomPattern_EntropySeeded's comment describes.
func (s *RoomBuilderTestSuite) TestQuickRoom_Seed() {
	s.Run("different explicit seeds diverge", func() {
		room1, err := QuickRoom(10, 10, PatternRandom, 4)
		s.Require().NoError(err)
		room2, err := QuickRoom(10, 10, PatternRandom, 10)
		s.Require().NoError(err)

		layout1, layout2 := wallSignature(room1), wallSignature(room2)
		s.Assert().NotEmpty(layout1)
		s.Assert().NotEmpty(layout2)
		s.Assert().NotEqual(layout1, layout2, "different explicit seeds should produce different wall layouts")
	})

	s.Run("explicit seed is reproducible", func() {
		room1, err := QuickRoom(10, 10, PatternRandom, 4)
		s.Require().NoError(err)
		room2, err := QuickRoom(10, 10, PatternRandom, 4)
		s.Require().NoError(err)

		s.Assert().Equal(wallSignature(room1), wallSignature(room2),
			"QuickRoom with the same explicit seed must reproduce the same wall layout")
	})
}

func TestRoomBuilderSuite(t *testing.T) {
	suite.Run(t, new(RoomBuilderTestSuite))
}
