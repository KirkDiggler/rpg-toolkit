package events_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/stretchr/testify/suite"
)

const (
	testResPlayerAlice = "p-alice"
	testResPlayerCarol = "p-carol"
)

type ResourceChangedEventSuite struct {
	suite.Suite
}

func TestResourceChangedEventSuite(t *testing.T) {
	suite.Run(t, new(ResourceChangedEventSuite))
}

// ResourceChangedEvent satisfies EncounterEvent.
func (s *ResourceChangedEventSuite) TestSatisfiesInterface() {
	var _ events.EncounterEvent = (*events.ResourceChangedEvent)(nil)
}

// JSON round-trip preserves all fields including unexported encID/seq.
func (s *ResourceChangedEventSuite) TestJSONRoundTrip() {
	original := events.NewResourceChangedEvent(
		"enc-1", 42,
		"char-bob",
		"rage_charges",
		1, 2,
		map[core.PlayerID]events.ResourceChangedSlice{
			testResPlayerAlice: {Visible: true},
		},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.ResourceChangedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(42), decoded.Sequence())
	s.Equal(core.EntityID("char-bob"), decoded.EntityID)
	s.Equal("rage_charges", decoded.ResourceRef)
	s.Equal(1, decoded.NewCurrent)
	s.Equal(2, decoded.Max)
	s.True(decoded.PerPlayer[testResPlayerAlice].Visible)
}

// Audience derives from PerPlayer keys.
func (s *ResourceChangedEventSuite) TestAudience_DerivedFromPerPlayer() {
	evt := events.NewResourceChangedEvent(
		"enc-1", 1, "char-bob", "rage_charges", 1, 2,
		map[core.PlayerID]events.ResourceChangedSlice{
			testResPlayerAlice: {Visible: true},
			testResPlayerCarol: {Visible: false},
		},
	)
	s.ElementsMatch(
		events.AudienceSet{testResPlayerAlice, testResPlayerCarol},
		evt.Audience(),
	)
}
