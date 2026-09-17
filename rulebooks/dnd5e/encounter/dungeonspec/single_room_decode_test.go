package dungeonspec

import (
	"os"
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
func (s *SingleRoomSourceSuite) TestDecodeRejectsUnknownAndSecondDocument() {
	for _, raw := range [][]byte{append(append([]byte{}, s.raw...), []byte("\n---\nversion: 3\n")...), append(append([]byte{}, s.raw...), []byte("\nextra: true\n")...)} {
		_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
		s.Error(err)
	}
}
func TestSingleRoomSourceSuite(t *testing.T) { suite.Run(t, new(SingleRoomSourceSuite)) }
