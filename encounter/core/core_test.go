package core_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type CoreSuite struct {
	suite.Suite
}

func TestCoreSuite(t *testing.T) {
	suite.Run(t, new(CoreSuite))
}

func (s *CoreSuite) TestHexSet_HasAndSlice() {
	h := core.Hex{Q: 1, R: -2, S: 1}
	set := core.NewHexSet(h, core.Hex{Q: 0, R: 0, S: 0})

	s.True(set.Has(h))
	s.False(set.Has(core.Hex{Q: 9, R: 9, S: -18}))
	s.Len(set.Slice(), 2)
}

// HexSet round-trips cleanly through JSON. This is load-bearing — the
// PerceptionView struct embeds HexSet and must persist correctly.
func (s *CoreSuite) TestHexSet_JSONRoundTrip() {
	a := core.Hex{Q: 1, R: -1, S: 0}
	b := core.Hex{Q: 2, R: -1, S: -1}
	original := core.NewHexSet(a, b)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded core.HexSet
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Len(decoded, 2)
	s.True(decoded.Has(a))
	s.True(decoded.Has(b))
}

// Empty HexSet round-trips as JSON null or [] — both should decode to empty.
func (s *CoreSuite) TestHexSet_EmptyRoundTrip() {
	payload, err := json.Marshal(core.HexSet{})
	s.Require().NoError(err)

	var decoded core.HexSet
	s.Require().NoError(json.Unmarshal(payload, &decoded))
	s.Empty(decoded)

	var fromNull core.HexSet
	s.Require().NoError(json.Unmarshal([]byte("null"), &fromNull))
	s.NotNil(fromNull)
	s.Empty(fromNull)
}

// ToCube/HexFromCube round-trip: same cube math, different field names
// (rpg-toolkit#757). Negative coordinates exercise the field mapping, not
// just zero-value passthrough.
func (s *CoreSuite) TestHex_CubeRoundTrip() {
	h := core.Hex{Q: 3, R: -5, S: 2}

	cube := h.ToCube()
	s.Equal(spatial.CubeCoordinate{X: 3, Y: -5, Z: 2}, cube)

	back := core.HexFromCube(cube)
	s.Equal(h, back)
}

// ToPosition/HexFromPosition round-trip through spatial's offset-coordinate
// bridge (pointy-top orientation — the only orientation environments.QuickRoom
// builds). Distinct from the cube round-trip: Position is NOT cube
// coordinates, it's the offset representation spatial.Room's LoS/movement
// methods actually take.
func (s *CoreSuite) TestHex_PositionRoundTrip() {
	for _, h := range []core.Hex{
		{Q: 0, R: 0, S: 0},
		{Q: 4, R: -2, S: -2},
		{Q: -3, R: 1, S: 2},
		{Q: 5, R: -5, S: 0},
	} {
		pos := h.ToPosition()
		back := core.HexFromPosition(pos)
		s.Equal(h, back, "round-trip mismatch for %+v via %+v", h, pos)
	}
}
