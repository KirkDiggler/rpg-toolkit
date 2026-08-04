package spatial

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CubeCoordinateValiditySuite struct {
	suite.Suite
}

// TestCubeCoordinateValiditySuite verifies cube-coordinate identity without
// relying on overflowing native-int arithmetic.
func TestCubeCoordinateValiditySuite(t *testing.T) {
	suite.Run(t, new(CubeCoordinateValiditySuite))
}

func (s *CubeCoordinateValiditySuite) TestIsValidAtNativeIntExtremes() {
	testCases := []struct {
		name string
		cube CubeCoordinate
		want bool
	}{
		{name: "zero", cube: CubeCoordinate{}, want: true},
		{name: "max plus one plus min", cube: CubeCoordinate{X: math.MaxInt, Y: 1, Z: math.MinInt}, want: true},
		{name: "min plus max plus one", cube: CubeCoordinate{X: math.MinInt, Y: math.MaxInt, Z: 1}, want: true},
		{name: "opposite max pair", cube: CubeCoordinate{X: math.MaxInt, Y: 0, Z: -math.MaxInt}, want: true},
		{name: "min and max are not opposites", cube: CubeCoordinate{X: math.MinInt, Y: math.MaxInt, Z: 0}, want: false},
		{name: "wrapped positive sum", cube: CubeCoordinate{X: math.MaxInt, Y: math.MaxInt, Z: 2}, want: false},
		{name: "double min", cube: CubeCoordinate{X: math.MinInt, Y: math.MinInt, Z: 0}, want: false},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.Equal(testCase.want, testCase.cube.IsValid())
		})
	}
}
