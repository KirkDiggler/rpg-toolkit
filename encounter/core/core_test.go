package core_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
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
