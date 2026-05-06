package types_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/types"
	"github.com/stretchr/testify/suite"
)

type TypesSuite struct {
	suite.Suite
}

func TestTypesSuite(t *testing.T) {
	suite.Run(t, new(TypesSuite))
}

func (s *TypesSuite) TestHexSet_HasAndSlice() {
	h := types.Hex{Q: 1, R: -2, S: 1}
	set := types.NewHexSet(h, types.Hex{Q: 0, R: 0, S: 0})

	s.True(set.Has(h))
	s.False(set.Has(types.Hex{Q: 9, R: 9, S: -18}))
	s.Len(set.Slice(), 2)
}

// HexSet round-trips cleanly through JSON. This is load-bearing — the
// PerceptionView struct embeds HexSet and must persist correctly.
func (s *TypesSuite) TestHexSet_JSONRoundTrip() {
	a := types.Hex{Q: 1, R: -1, S: 0}
	b := types.Hex{Q: 2, R: -1, S: -1}
	original := types.NewHexSet(a, b)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded types.HexSet
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Len(decoded, 2)
	s.True(decoded.Has(a))
	s.True(decoded.Has(b))
}

// Empty HexSet round-trips as JSON null or [] — both should decode to empty.
func (s *TypesSuite) TestHexSet_EmptyRoundTrip() {
	payload, err := json.Marshal(types.HexSet{})
	s.Require().NoError(err)

	var decoded types.HexSet
	s.Require().NoError(json.Unmarshal(payload, &decoded))
	s.Empty(decoded)

	var fromNull types.HexSet
	s.Require().NoError(json.Unmarshal([]byte("null"), &fromNull))
	s.NotNil(fromNull)
	s.Empty(fromNull)
}
