package encounter_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/stretchr/testify/suite"
)

type BrokerSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestBrokerSuite(t *testing.T) {
	suite.Run(t, new(BrokerSuite))
}

func (s *BrokerSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *BrokerSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// Subscribers in audience receive; those outside don't.
func (s *BrokerSuite) TestPublish_RoutesByAudience() {
	aliceSub, err := s.broker.Subscribe("enc:1", "alice")
	s.Require().NoError(err)
	bobSub, err := s.broker.Subscribe("enc:1", "bob")
	s.Require().NoError(err)

	move := events.NewMoveEvent("enc:1", 1, "monster-1",
		[]core.Hex{{Q: 0, R: 0, S: 0}},
		map[core.PlayerID]events.MovePlayerSlice{
			"alice": {SeenSegments: []core.Hex{{Q: 0, R: 0, S: 0}}},
			// bob absent — out of audience
		},
	)
	s.Require().NoError(s.broker.Publish(move))

	s.assertReceivesType(aliceSub, "*events.MoveEvent")
	s.assertNoReceive(bobSub)
}

// Cross-encounter isolation.
func (s *BrokerSuite) TestPublish_IsolatesEncounters() {
	sub1, _ := s.broker.Subscribe("enc:1", "alice")
	sub2, _ := s.broker.Subscribe("enc:2", "alice")

	s.Require().NoError(s.broker.Publish(events.NewMoveEvent("enc:1", 1, "x",
		nil, map[core.PlayerID]events.MovePlayerSlice{"alice": {}})))

	s.assertReceivesType(sub1, "*events.MoveEvent")
	s.assertNoReceive(sub2)
}

// Two subscriptions for the same player both receive.
func (s *BrokerSuite) TestSubscribe_MultiSubsForSamePlayer() {
	a1, _ := s.broker.Subscribe("enc:1", "alice")
	a2, _ := s.broker.Subscribe("enc:1", "alice")

	s.Require().NoError(s.broker.Publish(events.NewMoveEvent("enc:1", 1, "x",
		nil, map[core.PlayerID]events.MovePlayerSlice{"alice": {}})))

	s.assertReceivesType(a1, "*events.MoveEvent")
	s.assertReceivesType(a2, "*events.MoveEvent")
}

// Closing one subscription doesn't affect siblings; routing continues.
func (s *BrokerSuite) TestClose_RemovesOnlyClosedSub() {
	a1, _ := s.broker.Subscribe("enc:1", "alice")
	a2, _ := s.broker.Subscribe("enc:1", "alice")
	s.Require().NoError(a1.Close())

	s.Require().NoError(s.broker.Publish(events.NewMoveEvent("enc:1", 1, "x",
		nil, map[core.PlayerID]events.MovePlayerSlice{"alice": {}})))

	// a2 still receives — the meaningful assertion.
	s.assertReceivesType(a2, "*events.MoveEvent")
	// a1's channel is closed; reading returns zero value with !ok.
	select {
	case _, ok := <-a1.Events():
		s.False(ok, "a1's channel should be closed")
	case <-time.After(50 * time.Millisecond):
		s.FailNow("a1's channel did not close")
	}
}

// Subscription.Close is idempotent (sync.Once).
func (s *BrokerSuite) TestSubscriptionClose_Idempotent() {
	sub, _ := s.broker.Subscribe("enc:1", "alice")
	s.Require().NoError(sub.Close())
	s.Require().NoError(sub.Close())
}

// Broker.Close closes all subscriptions; calling Subscription.Close after
// Broker.Close is still safe (no double-close panic).
func (s *BrokerSuite) TestBrokerClose_ClosesAllSubsSafely() {
	sub, _ := s.broker.Subscribe("enc:1", "alice")
	s.Require().NoError(s.broker.Close())

	// Channel should be closed.
	select {
	case _, ok := <-sub.Events():
		s.False(ok)
	case <-time.After(50 * time.Millisecond):
		s.FailNow("sub channel did not close after broker close")
	}

	// Calling Subscription.Close after Broker.Close must not panic.
	s.Require().NoError(sub.Close())
}

// Helpers — shared with other encounter_test suites in this package.
// assertReceivesType takes a `want` string so future tests can assert other
// concrete event types; current call sites only exercise MoveEvent.
//
//nolint:unparam // generic helper; want will diversify as more verbs land
func (s *BrokerSuite) assertReceivesType(sub *encounter.Subscription, want string) {
	s.T().Helper()
	select {
	case evt, ok := <-sub.Events():
		s.Require().True(ok, "channel closed unexpectedly")
		s.Equal(want, fmt.Sprintf("%T", evt))
	case <-time.After(time.Second):
		s.FailNow("did not receive event in 1s")
	}
}

func (s *BrokerSuite) assertNoReceive(sub *encounter.Subscription) {
	s.T().Helper()
	select {
	case evt, ok := <-sub.Events():
		if ok {
			s.FailNowf("unexpected event", "got %T", evt)
		}
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

// ResourceChangedEvent round-trips through the broker codec (encode→transport→decode).
func (s *BrokerSuite) TestBrokerCodec_ResourceChangedEvent_RoundTrip() {
	sub, err := s.broker.Subscribe("enc:1", "p-alice")
	s.Require().NoError(err)

	original := events.NewResourceChangedEvent(
		"enc:1", 7,
		"char-bob",
		"rage_charges",
		1, 2,
		map[core.PlayerID]events.ResourceChangedSlice{
			"p-alice": {Visible: true},
		},
	)
	s.Require().NoError(s.broker.Publish(original))

	var received *events.ResourceChangedEvent
	select {
	case evt, ok := <-sub.Events():
		s.Require().True(ok, "channel closed unexpectedly")
		casted, ok := evt.(*events.ResourceChangedEvent)
		s.Require().True(ok, "expected *events.ResourceChangedEvent, got %T", evt)
		received = casted
	case <-time.After(time.Second):
		s.FailNow("did not receive ResourceChangedEvent in 1s")
	}

	s.Equal(core.EncounterID("enc:1"), received.EncounterID())
	s.Equal(uint64(7), received.Sequence())
	s.Equal(core.EntityID("char-bob"), received.EntityID)
	s.Equal("rage_charges", received.ResourceRef)
	s.Equal(1, received.NewCurrent)
	s.Equal(2, received.Max)
	s.True(received.PerPlayer["p-alice"].Visible)
}
