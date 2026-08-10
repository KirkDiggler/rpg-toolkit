package spatial_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// HexGridLoSSymmetrySuite is a regression suite for rpg-toolkit#788:
// HexGrid.lerpCube used to truncate interpolated cube coordinates with
// int() instead of rounding to the nearest cube (as roundCube already did
// for AxialHexGrid). Truncating each axis independently can produce a cube
// that violates the x+y+z=0 invariant partway along a line, and which axis
// absorbs the resulting correction depends on which endpoint the walk
// started from -- so GetLineOfSight(A,B) could diverge from
// GetLineOfSight(B,A) at distance >= 22 hexes. In practice the divergence
// is rare and needs a specific floating-point alignment between the line's
// slope and its length (empirically ~1 in 10,000 random pairs even at
// distances up to 100, and not reliably reproducible at all in the 20-40
// band alone) -- rare enough that an entity pair could go through many
// checks without hitting it, then silently see-without-being-seen once
// they did. TestGetLineOfSight_KnownAsymmetricPair pins one verified
// pre-fix failure directly; TestGetLineOfSight_SymmetricAtLongRange is the
// broader property check the issue asked for.
type HexGridLoSSymmetrySuite struct {
	suite.Suite
	grid *spatial.HexGrid
}

func (s *HexGridLoSSymmetrySuite) SetupTest() {
	// Large enough that random/fixed pairs used below stay well away from
	// IsValidPosition's bounds clipping, which would otherwise break
	// symmetry for reasons unrelated to this bug.
	s.grid = spatial.NewHexGrid(spatial.HexGridConfig{
		Width:       4000,
		Height:      4000,
		Orientation: spatial.HexOrientationPointyTop,
	})
}

// TestGetLineOfSight_KnownAsymmetricPair pins a concrete pair found by
// brute-force search over the pre-fix implementation: forward's path
// included (64,3830) where backward instead repeated (63,3830) twice,
// dropping (64,3830) entirely -- confirmed to fail before this fix and pass
// after.
func (s *HexGridLoSSymmetrySuite) TestGetLineOfSight_KnownAsymmetricPair() {
	from := spatial.Position{X: 52, Y: 3831}
	to := spatial.Position{X: 122, Y: 3829}

	forward := s.grid.GetLineOfSight(from, to)
	backward := s.grid.GetLineOfSight(to, from)

	s.Assert().ElementsMatchf(forward, backward,
		"LoS(%v,%v) must visit the same hexes as LoS(%v,%v): got %v vs %v", from, to, to, from, forward, backward)
}

// TestGetLineOfSight_SymmetricAtLongRange asserts GetLineOfSight(A,B) and
// GetLineOfSight(B,A) visit the same set of hexes across random position
// pairs at hex distance 20-40 -- the range the issue names as a property
// invariant. Note this range rarely if ever reproduces the pre-fix bug on
// its own (see suite doc); it guards the invariant going forward rather
// than serving as the primary regression check for the historical failure
// (that's TestGetLineOfSight_KnownAsymmetricPair).
func (s *HexGridLoSSymmetrySuite) TestGetLineOfSight_SymmetricAtLongRange() {
	//nolint:gosec // G404: seeded for reproducible test pairs, not cryptographic
	rng := rand.New(rand.NewSource(788))

	tested := 0
	for attempts := 0; attempts < 50000 && tested < 100; attempts++ {
		from := spatial.Position{X: float64(rng.Intn(3000) + 500), Y: float64(rng.Intn(3000) + 500)}
		// Offset from `from` by a bounded random delta rather than picking
		// `to` independently across the whole grid -- otherwise two random
		// points on a 4000x4000 board are overwhelmingly likely to land
		// far outside the 20-40 hex-distance band this test targets.
		to := spatial.Position{
			X: from.X + float64(rng.Intn(81)-40),
			Y: from.Y + float64(rng.Intn(81)-40),
		}

		dist := s.grid.Distance(from, to)
		if dist < 20 || dist > 40 {
			continue
		}
		tested++

		forward := s.grid.GetLineOfSight(from, to)
		backward := s.grid.GetLineOfSight(to, from)

		s.Assert().ElementsMatchf(forward, backward,
			"LoS(%v,%v) at distance %.0f must visit the same hexes as LoS(%v,%v): got %v vs %v",
			from, to, dist, to, from, forward, backward)
	}

	s.Require().GreaterOrEqualf(tested, 100,
		"only found %d random pairs at hex distance 20-40 in 50000 attempts", tested)
}

func TestHexGridLoSSymmetrySuite(t *testing.T) {
	suite.Run(t, new(HexGridLoSSymmetrySuite))
}
