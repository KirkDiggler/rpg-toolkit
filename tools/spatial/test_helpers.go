package spatial

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// RunPositionValidationTests runs common position validation tests for any Grid
func RunPositionValidationTests(t *testing.T, grid Grid) {
	testCases := []struct {
		name     string
		position Position
		expected bool
	}{
		{"origin", Position{X: 0, Y: 0}, true},
		{"center", Position{X: 5, Y: 5}, true},
		{"top-right corner", Position{X: 9, Y: 9}, true},
		{"negative x", Position{X: -1, Y: 5}, false},
		{"negative y", Position{X: 5, Y: -1}, false},
		{"x too large", Position{X: 10, Y: 5}, false},
		{"y too large", Position{X: 5, Y: 10}, false},
		{"both too large", Position{X: 10, Y: 10}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := grid.IsValidPosition(tc.position)
			assert.Equal(t, tc.expected, result)
		})
	}
}
