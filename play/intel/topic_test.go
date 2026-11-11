// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

// TopicSuite pins the record's third part: a holding is keyed by holder,
// subject AND topic, so two things held about one subject never overwrite
// each other, and a complete percept on one topic is silent about the rest.
type TopicSuite struct {
	suite.Suite
	intel *intel.Intel
}

func (s *TopicSuite) SetupTest() {
	var err error
	s.intel, err = intel.NewIntel()
	s.Require().NoError(err)
}

const (
	clerk    = core.EntityID("clerk")
	acct7    = intel.Subject("acct-7")
	acct9    = intel.Subject("acct-9")
	audited  = intel.Topic("audited")
	disputed = intel.Topic("disputed")
	ledgerCh = intel.Channel("ledger")
)

func (s *TopicSuite) surveil(topic intel.Topic, at uint64, percept ...intel.Report) *intel.SurveilOutput {
	out, err := s.intel.Surveil(&intel.SurveilInput{
		Observer: clerk, Channel: intel.Sight, Topic: topic, At: at, Percept: percept,
	})
	s.Require().NoError(err)
	return out
}

func (s *TopicSuite) report(topic intel.Topic, at uint64, reports ...intel.Report) *intel.ReportOutput {
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: clerk, Channel: ledgerCh, Topic: topic, At: at, Reports: reports,
	})
	s.Require().NoError(err)
	return out
}

// on reads the clerk's holding about acct7 on one topic.
func (s *TopicSuite) on(topic intel.Topic) intel.Holding {
	h, err := s.intel.On(&intel.OnInput{Observer: clerk, Subject: acct7, Topic: topic})
	s.Require().NoError(err)
	return h
}

// Two topics about one subject are two holdings: neither write touches the
// other's payload, channel, stamps or status.
func (s *TopicSuite) TestTwoTopicsAboutOneSubjectDoNotOverwrite() {
	s.surveil("", 1, intel.Report{Subject: acct7, Payload: []byte("shelf 3")})
	first := s.report(audited, 2, intel.Report{Subject: acct7, Payload: []byte("signed")})
	s.Equal([]intel.Report{{Subject: acct7, Payload: []byte("signed")}}, first.FirstContact,
		"a new topic about a held subject is first contact on that topic")

	// A later pass on the default topic moves only the default holding.
	s.surveil("", 5, intel.Report{Subject: acct7, Payload: []byte("shelf 4")})

	whereabouts := s.on("")
	s.Equal(intel.Topic(""), whereabouts.Topic)
	s.Equal([]byte("shelf 4"), whereabouts.Payload)
	s.Equal(intel.Sight, whereabouts.Channel)
	s.Equal(uint64(5), whereabouts.Observed)
	s.Equal(uint64(5), whereabouts.Confirmed)
	s.Equal(intel.Current, whereabouts.Status)

	signed := s.on(audited)
	s.Equal(audited, signed.Topic)
	s.Equal([]byte("signed"), signed.Payload)
	s.Equal(ledgerCh, signed.Channel)
	s.Equal(uint64(2), signed.Observed)
	s.Equal(uint64(2), signed.Confirmed)
	s.Equal(intel.Held, signed.Status)
	s.Empty(signed.CurrentVia)
}

// A complete percept on one topic can fade nothing on another: the default
// topic's empty percept fades the default holding and leaves a sight-
// sustained holding on another topic current.
func (s *TopicSuite) TestCompletePerceptNeverFadesAnotherTopic() {
	s.surveil("", 1, intel.Report{Subject: acct7, Payload: []byte("shelf 3")})
	s.surveil(audited, 1, intel.Report{Subject: acct7, Payload: []byte("signed")})

	out := s.surveil("", 2) // sees nothing on the default topic
	s.Equal([]intel.Subject{acct7}, out.Faded)

	s.Equal(intel.Held, s.on("").Status)
	other := s.on(audited)
	s.Equal(intel.Current, other.Status, "another topic's percept said nothing about this one")
	s.Equal([]intel.Channel{intel.Sight}, other.CurrentVia)

	// And the reverse: the other topic's own empty percept fades only it.
	out = s.surveil(audited, 3)
	s.Equal([]intel.Subject{acct7}, out.Faded)
	s.Equal(intel.Held, s.on(audited).Status)
}

// Within a topic, fading and re-acquiring behave exactly as on the default
// topic; reacquisition on one topic is not reported by another topic's pass.
func (s *TopicSuite) TestFadeAndReacquireStayWithinTheirTopic() {
	s.surveil(audited, 1, intel.Report{Subject: acct7, Payload: []byte("signed")})
	s.surveil(audited, 2)
	s.surveil("", 3, intel.Report{Subject: acct7, Payload: []byte("shelf 3")})

	s.Equal(intel.Held, s.on(audited).Status,
		"a default-topic sighting does not revive the other topic's ghost")

	back := s.surveil(audited, 4, intel.Report{Subject: acct7, Payload: []byte("signed")})
	s.Equal([]intel.Subject{acct7}, back.Reacquired)
	s.Equal([]intel.Subject{acct7}, back.Refreshed)
	s.Empty(back.FirstContact)
	s.Empty(back.Changed)
}

// A Report on one topic never touches another topic's holding, and its
// deltas speak only of that topic.
func (s *TopicSuite) TestReportIsScopedToItsTopic() {
	s.report("", 1, intel.Report{Subject: acct7, Payload: []byte("shelf 3")})
	out := s.report(disputed, 2, intel.Report{Subject: acct7, Payload: []byte("yes")})
	s.Empty(out.Updated, "the default holding exists, but this call is about another topic")
	s.Len(out.FirstContact, 1)

	again := s.report(disputed, 3, intel.Report{Subject: acct7, Payload: []byte("settled")})
	s.Equal([]intel.Subject{acct7}, again.Updated)
	s.Equal([]intel.Subject{acct7}, again.Changed)
	s.Equal([]byte("shelf 3"), s.on("").Payload)
	s.Equal(uint64(1), s.on("").Confirmed)
}

// On names one (observer, subject, topic): a topic not held is ErrNotHeld
// even when the subject is held on another.
func (s *TopicSuite) TestOnAsksForOneTopic() {
	s.report(audited, 1, intel.Report{Subject: acct7, Payload: []byte("signed")})

	_, err := s.intel.On(&intel.OnInput{Observer: clerk, Subject: acct7})
	s.True(errors.Is(err, intel.ErrNotHeld), "the default topic is not held")
	_, err = s.intel.On(&intel.OnInput{Observer: clerk, Subject: acct7, Topic: disputed})
	s.True(errors.Is(err, intel.ErrNotHeld))
}

// HeldBy answers every holding across topics, ordered by subject then topic.
func (s *TopicSuite) TestHeldBySpansTopicsInOrder() {
	s.report(disputed, 1, intel.Report{Subject: acct9}, intel.Report{Subject: acct7})
	s.report(audited, 2, intel.Report{Subject: acct7})
	s.report("", 3, intel.Report{Subject: acct9})

	held, err := s.intel.HeldBy(&intel.HeldByInput{Observer: clerk})
	s.Require().NoError(err)
	type key struct {
		Subject intel.Subject
		Topic   intel.Topic
	}
	keys := make([]key, 0, len(held))
	for _, h := range held {
		keys = append(keys, key{h.Subject, h.Topic})
	}
	s.Equal([]key{
		{acct7, audited}, {acct7, disputed},
		{acct9, ""}, {acct9, disputed},
	}, keys)
}

// About answers everything one observer holds about one subject, one per
// topic, ordered by topic; nothing held answers empty, nil error.
func (s *TopicSuite) TestAboutAnswersEveryTopicForOneSubject() {
	s.report(disputed, 1, intel.Report{Subject: acct7, Payload: []byte("yes")})
	s.report("", 2, intel.Report{Subject: acct7, Payload: []byte("shelf 3")})
	s.report(audited, 3, intel.Report{Subject: acct7, Payload: []byte("signed")}, intel.Report{Subject: acct9})

	about, err := s.intel.About(&intel.AboutInput{Observer: clerk, Subject: acct7})
	s.Require().NoError(err)
	topics := make([]intel.Topic, 0, len(about))
	for _, h := range about {
		s.Equal(acct7, h.Subject)
		topics = append(topics, h.Topic)
	}
	s.Equal([]intel.Topic{"", audited, disputed}, topics)
	s.Equal([]byte("signed"), about[1].Payload)

	none, err := s.intel.About(&intel.AboutInput{Observer: clerk, Subject: "acct-0"})
	s.Require().NoError(err)
	s.NotNil(none)
	s.Empty(none)

	none, err = s.intel.About(&intel.AboutInput{Observer: "stranger", Subject: acct7})
	s.Require().NoError(err)
	s.Empty(none)
}

// OnTopic answers everything one observer holds on one topic, ordered by
// subject; the default topic is a legal question.
func (s *TopicSuite) TestOnTopicAnswersEverySubjectForOneTopic() {
	s.report(audited, 1, intel.Report{Subject: acct9}, intel.Report{Subject: acct7})
	s.report(disputed, 2, intel.Report{Subject: acct7})
	s.report("", 3, intel.Report{Subject: acct9})

	on, err := s.intel.OnTopic(&intel.OnTopicInput{Observer: clerk, Topic: audited})
	s.Require().NoError(err)
	subjects := make([]intel.Subject, 0, len(on))
	for _, h := range on {
		s.Equal(audited, h.Topic)
		subjects = append(subjects, h.Subject)
	}
	s.Equal([]intel.Subject{acct7, acct9}, subjects)

	def, err := s.intel.OnTopic(&intel.OnTopicInput{Observer: clerk})
	s.Require().NoError(err)
	s.Require().NotEmpty(def)
	for _, h := range def {
		s.Equal(intel.Topic(""), h.Topic)
		s.Equal(acct9, h.Subject)
	}

	none, err := s.intel.OnTopic(&intel.OnTopicInput{Observer: clerk, Topic: "unknown"})
	s.Require().NoError(err)
	s.NotNil(none)
	s.Empty(none)
}

// The new queries copy out like the old ones: mutating a returned payload
// never reaches the store.
func (s *TopicSuite) TestNewQueriesCopyOut() {
	s.report(audited, 1, intel.Report{Subject: acct7, Payload: []byte("signed")})

	about, err := s.intel.About(&intel.AboutInput{Observer: clerk, Subject: acct7})
	s.Require().NoError(err)
	about[0].Payload[0] = 'X'
	on, err := s.intel.OnTopic(&intel.OnTopicInput{Observer: clerk, Topic: audited})
	s.Require().NoError(err)
	s.Equal([]byte("signed"), on[0].Payload)
	on[0].Payload[0] = 'X'
	s.Equal([]byte("signed"), s.on(audited).Payload)
}

// The new queries validate in the module's order.
func (s *TopicSuite) TestNewQueriesValidate() {
	_, err := s.intel.About(nil)
	s.True(errors.Is(err, intel.ErrNilInput))
	_, err = s.intel.About(&intel.AboutInput{Subject: acct7})
	s.True(errors.Is(err, intel.ErrNoObserver))
	_, err = s.intel.About(&intel.AboutInput{})
	s.True(errors.Is(err, intel.ErrNoObserver), "observer is checked before subject")
	_, err = s.intel.About(&intel.AboutInput{Observer: clerk})
	s.True(errors.Is(err, intel.ErrNoSubject))

	_, err = s.intel.OnTopic(nil)
	s.True(errors.Is(err, intel.ErrNilInput))
	_, err = s.intel.OnTopic(&intel.OnTopicInput{Topic: audited})
	s.True(errors.Is(err, intel.ErrNoObserver))
}

// Several topics survive ToData → JSON → LoadIntel → ToData byte-for-byte,
// and the reloaded store answers the same.
func (s *TopicSuite) TestRoundTripWithSeveralTopics() {
	s.surveil("", 1, intel.Report{Subject: acct7, Payload: []byte("shelf 3")})
	s.surveil(audited, 2, intel.Report{Subject: acct7, Payload: []byte("signed")},
		intel.Report{Subject: acct9, Payload: []byte("signed")})
	s.surveil(audited, 3, intel.Report{Subject: acct9, Payload: []byte("signed")})
	s.report(disputed, 4, intel.Report{Subject: acct7, Payload: []byte("yes")})

	b1, err := json.Marshal(s.intel.ToData())
	s.Require().NoError(err)
	var data intel.Data
	s.Require().NoError(json.Unmarshal(b1, &data))
	loaded, err := intel.LoadIntel(data)
	s.Require().NoError(err)
	b2, err := json.Marshal(loaded.ToData())
	s.Require().NoError(err)
	s.Equal(string(b1), string(b2))

	want, err := s.intel.HeldBy(&intel.HeldByInput{Observer: clerk})
	s.Require().NoError(err)
	got, err := loaded.HeldBy(&intel.HeldByInput{Observer: clerk})
	s.Require().NoError(err)
	s.Equal(want, got)

	// Behaviour after reload: the topic-scoped fade still holds.
	out, err := loaded.Surveil(&intel.SurveilInput{Observer: clerk, Channel: intel.Sight, Topic: audited, At: 5})
	s.Require().NoError(err)
	s.Equal([]intel.Subject{acct9}, out.Faded)
	h, err := loaded.On(&intel.OnInput{Observer: clerk, Subject: acct7})
	s.Require().NoError(err)
	s.Equal(intel.Current, h.Status)
}

// The stored shape is observer → subject → topic, the default topic under
// the key "". Pinned as exact JSON.
func (s *TopicSuite) TestGoldenJSONWithTopics() {
	s.report("", 1, intel.Report{Subject: acct7})
	s.report(audited, 2, intel.Report{Subject: acct7})

	b, err := json.Marshal(s.intel.ToData())
	s.Require().NoError(err)
	s.Equal(`{"holdings":{"clerk":{"acct-7":{"":{"channel":"ledger","observed":1,"confirmed":1},`+
		`"audited":{"channel":"ledger","observed":2,"confirmed":2}}}}}`, string(b))
}

// A subject entry with no topics is unreachable and rejected (R9).
func (s *TopicSuite) TestLoadRejectsSubjectWithNoTopics() {
	for name, topics := range map[string]map[intel.Topic]intel.HoldingData{
		"nil":   nil,
		"empty": {},
	} {
		_, err := intel.LoadIntel(intel.Data{
			Holdings: map[core.EntityID]map[intel.Subject]map[intel.Topic]intel.HoldingData{
				clerk: {acct7: topics},
			},
		})
		s.True(errors.Is(err, intel.ErrInvalidData), name)
	}
}

// A record saved before topics existed (holdings directly under the
// subject) does not load: it fails closed rather than misreading field
// names as topics.
func (s *TopicSuite) TestPreTopicRecordFailsClosed() {
	old := `{"holdings":{"clerk":{"acct-7":{"payload":"eA==","channel":"sight","observed":1,"confirmed":1}}}}`
	var data intel.Data
	s.Error(json.Unmarshal([]byte(old), &data))
}

func TestTopicSuite(t *testing.T) {
	suite.Run(t, new(TopicSuite))
}
