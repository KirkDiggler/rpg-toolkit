package dungeonspec

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SingleRoomSourceSuite struct {
	suite.Suite
	raw []byte
}

func (s *SingleRoomSourceSuite) SetupTest() {
	var err error
	s.raw, err = os.ReadFile("testdata/world-builder-v3.yaml")
	s.Require().NoError(err)
}
func (s *SingleRoomSourceSuite) TestDecodePreservesSourceFacts() {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	s.Require().NotNil(out.Spec)
	s.Equal(-2.25, out.Spec.Room.Scene.Items[0].Transform.X)
	s.Equal(1.5, *out.Spec.Room.Scene.Items[0].HeightScale)
	s.False(*out.Spec.Room.Gameplay.PropDeclarations["table"].BlocksLineOfSight)
	s.Equal("table", out.Spec.Room.Scene.Items[1].SupportID)
	s.Equal("furniture", out.Spec.Room.Scene.Items[1].ParentID)
}
func (s *SingleRoomSourceSuite) TestDecodeRoundTripsJSONAndNestedGraph() {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	encoded, err := json.Marshal(out.Spec)
	s.Require().NoError(err)
	again, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: encoded})
	s.Require().NoError(err)
	s.Equal(out.Spec, again.Spec)
	s.False(*again.Spec.Room.Gameplay.PropDeclarations["table"].BlocksLineOfSight)
}
func (s *SingleRoomSourceSuite) TestDecodeRejectsLoadBearingInvalidValues() {
	cases := []struct{ name, old, repl string }{{"missing transform coordinate", "x: -2.25, ", ""}, {"unsupported policy", "standing: centre-covered", "standing: invented"}, {"nonfinite footprint", "width: 1.2", "width: .nan"}, {"missing start coordinate", "partyStart: {q: 0, r: 0}", "partyStart: {r: 0}"}, {"wrong monster kind", "dnd5e:monsters:skeleton", "dnd5e:props:table"}, {"bad color", "#ff9d52", "#gggggg"}, {"bad workspace", "horizontalLimit: 12", "horizontalLimit: -1"}}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			raw := []byte(strings.Replace(string(s.raw), tc.old, tc.repl, 1))
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
			s.Error(err)
		})
	}
}
func (s *SingleRoomSourceSuite) TestDecodeRejectsUnknownAndSecondDocument() {
	for _, raw := range [][]byte{append(append([]byte{}, s.raw...), []byte("\n---\nversion: 3\n")...), append(append([]byte{}, s.raw...), []byte("\nextra: true\n")...)} {
		_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
		s.Error(err)
	}
}
func TestSingleRoomSourceSuite(t *testing.T) { suite.Run(t, new(SingleRoomSourceSuite)) }
