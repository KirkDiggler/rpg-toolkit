package environments

import (
	"slices"
	"testing"
)

// sweepSeedCount is the number of distinct explicit seeds swept for the
// empty-room-rate measurement below. Matches the acceptance criteria in
// rpg-toolkit#792: a 100-seed sweep at 20x20 PatternRandom.
const sweepSeedCount = 100

// TestRandomPattern_EmptyRoomRate_20x20 sweeps sweepSeedCount explicit seeds
// through the real BasicRoomBuilder path (the same path StartEncounter
// drives in rpg-api) at 20x20 with PatternRandom and PatternParams'
// defaults, and reports the fraction of resulting rooms with zero walls.
//
// This is rpg-toolkit#792's acceptance test: at 20x20 PatternRandom, walls
// must appear in >=95% of rooms across the sweep. Before the bounded-retry
// fix, this sat around ~15-20% non-empty (5/6 empty matches the rate
// rpg-api#669 observed against the real StartEncounter path); see the PR
// body for the exact measured before/after numbers.
func TestRandomPattern_EmptyRoomRate_20x20(t *testing.T) {
	emptyCount := 0

	for seed := int64(1); seed <= sweepSeedCount; seed++ {
		builder := NewBasicRoomBuilder(BasicRoomBuilderConfig{}).
			WithSize(20, 20).
			WithWallPattern(PatternRandom).
			WithRandomSeed(seed)

		room, err := builder.Build()
		if err != nil {
			t.Fatalf("seed %d: Build failed: %v", seed, err)
		}

		if len(GetWallEntitiesInRoom(room)) == 0 {
			emptyCount++
		}
	}

	nonEmptyRate := float64(sweepSeedCount-emptyCount) / float64(sweepSeedCount)
	t.Logf(
		"20x20 PatternRandom sweep: %d/%d empty (%.1f%% non-empty)",
		emptyCount, sweepSeedCount, nonEmptyRate*100,
	)

	if nonEmptyRate < 0.95 {
		t.Errorf(
			"non-empty rate %.1f%% is below the 95%% acceptance bar (rpg-toolkit#792): %d/%d seeds fell back to empty",
			nonEmptyRate*100, emptyCount, sweepSeedCount,
		)
	}
}

// TestRandomPattern_EmptyRoomRate_Determinism proves the sweep above (and
// the retry mechanism it exercises) doesn't disturb explicit-seed
// reproducibility: the same seed must still produce the same wall layout
// across repeated builds.
func TestRandomPattern_EmptyRoomRate_Determinism(t *testing.T) {
	build := func(seed int64) []string {
		builder := NewBasicRoomBuilder(BasicRoomBuilderConfig{}).
			WithSize(20, 20).
			WithWallPattern(PatternRandom).
			WithRandomSeed(seed)

		room, err := builder.Build()
		if err != nil {
			t.Fatalf("seed %d: Build failed: %v", seed, err)
		}
		return wallSignature(room)
	}

	for _, seed := range []int64{1, 2, 3, 17, 99} {
		first := build(seed)
		second := build(seed)
		if !slices.Equal(first, second) {
			t.Errorf(
				"seed %d: layout not reproducible across repeated Build() calls\nfirst:  %v\nsecond: %v",
				seed, first, second,
			)
		}
	}
}
