// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// TopicSuite pins perception's topic contract: percepts live on the default
// topic and the sight readers (Held, On) read only it; a named topic is
// discrete testimony written by Report and read by HeldOn, About, OnTopic.
type TopicSuite struct {
	suite.Suite
	p *perception.Perception
}

func (s *TopicSuite) SetupTest() {
	var err error
	s.p, err = perception.New()
	s.Require().NoError(err)
}

const (
	clerk    = core.EntityID("clerk")
	acct7    = core.EntityID("acct-7")
	acct9    = core.EntityID("acct-9")
	audited  = perception.Topic("audited")
	disputed = perception.Topic("disputed")
	told     = perception.Channel("told")
)

func (s *TopicSuite) see(at uint64, presences ...perception.Presence) *perception.Delta {
	deltas, err := s.p.Observe(perception.Pass{
		At: at, Channel: perception.Sight, Presences: presences,
		Observers: []core.EntityID{clerk}, Reach: reachAll{},
	})
	s.Require().NoError(err)
	return deltas[clerk]
}

func (s *TopicSuite) tell(topic perception.Topic, at uint64, reports ...perception.Presence) *perception.ReportOutput {
	out, err := s.p.Report(perception.ReportInput{
		Observer: clerk, Channel: told, Topic: topic, At: at, Reports: reports,
	})
	s.Require().NoError(err)
	return out
}

// sightReaders captures every default-topic reader's answer for the clerk.
type sightReaders struct {
	held   []perception.Holding
	on7    perception.Holding
	on9    perception.Holding
	heldOn []perception.Holding
}

func (s *TopicSuite) readSight() sightReaders {
	var r sightReaders
	var err error
	r.held, err = s.p.Held(clerk)
	s.Require().NoError(err)
	r.on7, err = s.p.On(clerk, acct7)
	s.Require().NoError(err)
	r.on9, err = s.p.On(clerk, acct9)
	s.Require().NoError(err)
	r.heldOn, err = s.p.HeldOn(clerk, "")
	s.Require().NoError(err)
	return r
}

// THE SAFETY PROPERTY. Named-topic holdings about subjects the observer has
// sighted change nothing any sight reader answers: same entries, same
// payloads, same order — and every one of them on the default topic.
func (s *TopicSuite) TestNamedTopicsAreInvisibleToSightReaders() {
	s.see(1, perception.Presence{ID: acct7, Payload: []byte("shelf 3")},
		perception.Presence{ID: acct9, Payload: []byte("shelf 5")})
	before := s.readSight()

	s.tell(audited, 2, perception.Presence{ID: acct7, Payload: []byte("signed")},
		perception.Presence{ID: acct9, Payload: []byte("signed")})
	s.tell(disputed, 3, perception.Presence{ID: acct7, Payload: []byte("yes")})

	after := s.readSight()
	s.Equal(before, after)
	for _, h := range after.held {
		s.Equal(perception.Topic(""), h.Topic)
	}
	s.Equal(after.held, after.heldOn, "Held is HeldOn the default topic")
}

// A named topic written once is answered by each of the three questions,
// and every answer names its topic.
func (s *TopicSuite) TestNamedTopicIsReadByEachQuestion() {
	s.see(1, perception.Presence{ID: acct7, Payload: []byte("shelf 3")})
	first := s.tell(audited, 2, perception.Presence{ID: acct9, Payload: []byte("signed")},
		perception.Presence{ID: acct7, Payload: []byte("signed")})
	s.Equal([]perception.Presence{
		{ID: acct9, Payload: []byte("signed")},
		{ID: acct7, Payload: []byte("signed")},
	}, first.FirstContact, "a named topic about a sighted subject is first contact on that topic")
	s.Empty(first.Updated)

	// One (observer, subject, topic).
	h, err := s.p.OnTopic(clerk, acct7, audited)
	s.Require().NoError(err)
	s.Equal(acct7, h.Subject)
	s.Equal(audited, h.Topic)
	s.Equal([]byte("signed"), h.Payload)
	s.Equal(told, h.Channel)
	s.Equal(uint64(2), h.Observed)
	s.Empty(h.CurrentVia, "a report sustains nothing")

	_, err = s.p.OnTopic(clerk, acct7, disputed)
	s.ErrorIs(err, perception.ErrNotHeld)
	def, err := s.p.OnTopic(clerk, acct7, "")
	s.Require().NoError(err)
	on, err := s.p.On(clerk, acct7)
	s.Require().NoError(err)
	s.Equal(on, def, "OnTopic on the default topic is On")

	// Everything about one subject: every topic, the default first.
	about, err := s.p.About(clerk, acct7)
	s.Require().NoError(err)
	topics := make([]perception.Topic, 0, len(about))
	for _, h := range about {
		s.Equal(acct7, h.Subject)
		topics = append(topics, h.Topic)
	}
	s.Equal([]perception.Topic{"", audited}, topics)

	// Everything on one topic: every subject, sorted.
	heldOn, err := s.p.HeldOn(clerk, audited)
	s.Require().NoError(err)
	subjects := make([]core.EntityID, 0, len(heldOn))
	for _, h := range heldOn {
		s.Equal(audited, h.Topic)
		subjects = append(subjects, h.Subject)
	}
	s.Equal([]core.EntityID{acct7, acct9}, subjects)

	// Nothing held is an empty answer, not an error.
	none, err := s.p.HeldOn(clerk, disputed)
	s.Require().NoError(err)
	s.NotNil(none)
	s.Empty(none)
	none, err = s.p.About("stranger", acct7)
	s.Require().NoError(err)
	s.NotNil(none)
	s.Empty(none)
}

// A sight pass never fades or refreshes a named topic, and a named-topic
// write never touches a sighting.
func (s *TopicSuite) TestPassesAndNamedTopicsNeverTouch() {
	s.see(1, perception.Presence{ID: acct7, Payload: []byte("shelf 3")})
	s.tell(audited, 2, perception.Presence{ID: acct7, Payload: []byte("signed")})
	signedBefore, err := s.p.OnTopic(clerk, acct7, audited)
	s.Require().NoError(err)

	refresh := s.see(3, perception.Presence{ID: acct7, Payload: []byte("shelf 3")})
	s.Equal([]core.EntityID{acct7}, refresh.Refreshed)
	s.Empty(refresh.FirstContact, "the named topic is not a percept to be first-contacted")

	fade := s.see(4) // the clerk sees nothing
	s.Equal([]core.EntityID{acct7}, fade.Faded)
	signedAfter, err := s.p.OnTopic(clerk, acct7, audited)
	s.Require().NoError(err)
	s.Equal(signedBefore, signedAfter, "no pass refreshed, faded or re-stamped the named topic")

	ghost, err := s.p.On(clerk, acct7)
	s.Require().NoError(err)
	s.False(ghost.CurrentOn(perception.Sight))

	// A named-topic write lands nothing on the sighting.
	again := s.tell(audited, 5, perception.Presence{ID: acct7, Payload: []byte("revoked")})
	s.Equal([]core.EntityID{acct7}, again.Changed)
	after, err := s.p.On(clerk, acct7)
	s.Require().NoError(err)
	s.Equal(ghost, after)

	back := s.see(6, perception.Presence{ID: acct7, Payload: []byte("shelf 3")})
	s.Equal([]core.EntityID{acct7}, back.Reacquired)
}

// Both topics survive ToData → JSON → Load → ToData byte-for-byte, and the
// reloaded store answers the same questions the same way.
func (s *TopicSuite) TestRoundTripKeepsBothTopics() {
	s.see(1, perception.Presence{ID: acct7, Payload: []byte("shelf 3")})
	s.tell(audited, 2, perception.Presence{ID: acct7, Payload: []byte("signed")},
		perception.Presence{ID: acct9, Payload: []byte("signed")})

	b1, err := json.Marshal(s.p.ToData())
	s.Require().NoError(err)
	var data perception.Data
	s.Require().NoError(json.Unmarshal(b1, &data))
	loaded, err := perception.Load(data)
	s.Require().NoError(err)
	b2, err := json.Marshal(loaded.ToData())
	s.Require().NoError(err)
	s.Equal(string(b1), string(b2))

	for _, subject := range []core.EntityID{acct7, acct9} {
		want, err := s.p.About(clerk, subject)
		s.Require().NoError(err)
		got, err := loaded.About(clerk, subject)
		s.Require().NoError(err)
		s.Equal(want, got)
	}
	held, err := loaded.Held(clerk)
	s.Require().NoError(err)
	s.Require().NotEmpty(held)
	for _, h := range held {
		s.Equal(acct7, h.Subject, "only the sighting is on the default topic")
		s.True(h.CurrentOn(perception.Sight))
	}
}

// The new reads validate their IDs in On's order: observer, then subject.
func (s *TopicSuite) TestNewReadsValidate() {
	_, err := s.p.HeldOn("", audited)
	s.ErrorIs(err, perception.ErrNoObserver)
	_, err = s.p.About("", "")
	s.ErrorIs(err, perception.ErrNoObserver)
	_, err = s.p.About(clerk, "")
	s.ErrorIs(err, perception.ErrNoSubject)
	_, err = s.p.OnTopic("", "", audited)
	s.ErrorIs(err, perception.ErrNoObserver)
	_, err = s.p.OnTopic(clerk, "", audited)
	s.ErrorIs(err, perception.ErrNoSubject)
}

func TestTopicSuite(t *testing.T) {
	suite.Run(t, new(TopicSuite))
}
