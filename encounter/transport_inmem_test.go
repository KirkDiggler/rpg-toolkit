package encounter_test

import (
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/stretchr/testify/suite"
)

type TransportInMemSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
}

func TestTransportInMemSuite(t *testing.T) {
	suite.Run(t, new(TransportInMemSuite))
}

func (s *TransportInMemSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
}

func (s *TransportInMemSuite) TearDownTest() {
	_ = s.transport.Close()
}

func (s *TransportInMemSuite) TestPublish_DeliversToSubscriber() {
	sub, err := s.transport.Subscribe("enc:1")
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.Require().NoError(s.transport.Publish("enc:1", []byte("hello")))

	s.assertReceives(sub, []byte("hello"))
}

func (s *TransportInMemSuite) TestPublish_FansOutByChannel() {
	sub1, _ := s.transport.Subscribe("enc:1")
	sub2, _ := s.transport.Subscribe("enc:1")
	other, _ := s.transport.Subscribe("enc:2")
	defer func() { _ = sub1.Close() }()
	defer func() { _ = sub2.Close() }()
	defer func() { _ = other.Close() }()

	s.Require().NoError(s.transport.Publish("enc:1", []byte("a")))

	s.assertReceives(sub1, []byte("a"))
	s.assertReceives(sub2, []byte("a"))
	s.assertNoReceive(other)
}

func (s *TransportInMemSuite) TestSubscribe_NoReplay() {
	s.Require().NoError(s.transport.Publish("enc:1", []byte("missed")))

	sub, _ := s.transport.Subscribe("enc:1")
	defer func() { _ = sub.Close() }()

	s.assertNoReceive(sub)
}

func (s *TransportInMemSuite) TestClose_TerminatesSubscriptions() {
	sub, _ := s.transport.Subscribe("enc:1")
	s.Require().NoError(s.transport.Close())

	select {
	case _, ok := <-sub.Payloads():
		s.False(ok, "channel should be closed")
	case <-time.After(time.Second):
		s.FailNow("channel did not close in 1s")
	}
}

func (s *TransportInMemSuite) TestPublish_AfterCloseErrors() {
	s.Require().NoError(s.transport.Close())
	s.Error(s.transport.Publish("enc:1", []byte("x")))
}

// Subscription.Close is idempotent — sync.Once guards against double-close.
func (s *TransportInMemSuite) TestSubscriptionClose_Idempotent() {
	sub, _ := s.transport.Subscribe("enc:1")
	s.Require().NoError(sub.Close())
	s.Require().NoError(sub.Close())
}

func (s *TransportInMemSuite) assertReceives(sub encounter.TransportSubscription, want []byte) {
	s.T().Helper()
	select {
	case got := <-sub.Payloads():
		s.Equal(want, got)
	case <-time.After(time.Second):
		s.FailNow("did not receive payload in 1s")
	}
}

func (s *TransportInMemSuite) assertNoReceive(sub encounter.TransportSubscription) {
	s.T().Helper()
	select {
	case got, ok := <-sub.Payloads():
		if ok {
			s.FailNowf("unexpected payload", "got %s", got)
		}
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}
