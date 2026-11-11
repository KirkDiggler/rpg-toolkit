// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Channel identifies a source of testimony (hearing, sight, smell, etc).
// Sight is the one predeclared channel; vocabulary is open — physical
// channels get physics from the stage, supernatural from rulebooks;
// intel treats all identically.
type Channel string

// Sight is the predeclared visual channel.
const Sight Channel = "sight"

// Subject is an opaque, caller-chosen identification within an
// observer's fidelity: a place key, an entity ID, a believed identity.
// Choosing subjects is part of the testimony.
type Subject string

// Topic names what a holding is about the subject: the third part of the
// record after holder and subject. A holder keeps one holding per subject
// PER TOPIC, so two different things held about one subject never
// overwrite each other. Like Channel and Subject the vocabulary is open,
// opaque and caller-chosen; intel never interprets it.
//
// The empty topic is the default and is legal everywhere: a caller that
// never names a topic keeps exactly the one-holding-per-subject behaviour
// the module had before topics existed.
//
// Example: a ledger clerk holds, about the account "acct-7", one holding on
// the default topic (where its papers were last filed) and another on the
// topic "audited" (that it was signed off) — refiling the papers never
// touches the sign-off.
type Topic string

// Report is one piece of testimony: a subject and an opaque payload.
// A report names no topic of its own: the verb's input names the one topic
// every report in the call is about, as it names the one channel.
// Payload is empty legal; nil and empty are the same legal empty payload.
type Report struct {
	Subject Subject
	Payload []byte
}

// Status identifies whether a holding is Current or Held.
// Status is derived, never stored: Current iff any channel sustains.
type Status string

const (
	// Current indicates the subject is actively sustained by at least one channel.
	Current Status = "current"
	// Held indicates the subject is known but not actively sustained (a ghost).
	Held Status = "held"
)

// Holding is the read-side value: a subject and topic, payload, channel provenance,
// two timestamps, active channels (CurrentVia), and derived status. Observed
// is when this payload was first seen; Confirmed is when it was last landed,
// whether or not the content changed. Channel is provenance of the latest
// accepted testimony. Status is derived (Current iff CurrentVia is
// non-empty).
//
// The store compares payload bytes on landing — it never reads them, so it
// still cannot tell a lie from the truth. That requires payload encoders to
// be deterministic: a non-deterministic one makes every pass look like a
// change, and Observed would silently read as always-fresh.
type Holding struct {
	Subject    Subject
	Topic      Topic
	Payload    []byte
	Channel    Channel
	Observed   uint64
	Confirmed  uint64
	CurrentVia []Channel
	Status     Status
}

// Intel is the container for all observers' intel holdings.
// Not safe for concurrent use. Zero value not usable; construct via
// NewIntel or LoadIntel.
//
// A holding is keyed by (observer, subject, topic): holder and subject at
// the outside edges, the topic carrying the relationship between them.
type Intel struct {
	holdings map[core.EntityID]map[Subject]map[Topic]*holding
}

// internal holding struct stores the mutable state.
type holding struct {
	payload    []byte
	channel    Channel
	observed   uint64
	confirmed  uint64
	currentVia map[Channel]struct{}
}

// NewIntel creates a new Intel container.
func NewIntel() (*Intel, error) {
	return &Intel{
		holdings: make(map[core.EntityID]map[Subject]map[Topic]*holding),
	}, nil
}

// SurveilInput carries a complete percept for this observer+channel+topic.
//
// Topic scopes the complete-percept contract: the percept is complete for
// this observer, on this channel, ABOUT THIS TOPIC, and nothing else. A
// subject absent from the percept loses this channel only on its holding
// for this topic; its holdings on every other topic are untouched. The
// empty Topic is the default topic.
type SurveilInput struct {
	Observer core.EntityID
	Channel  Channel
	Topic    Topic
	Percept  []Report
	At       uint64
}

// SurveilOutput reports the deltas Surveil caused.
//
// Every subject it names is a holding on the input's one Topic: a pass
// touches no other topic, so a subject here is never ambiguous about which
// of its holdings moved.
//
// The four lists answer four different questions and are NOT a partition of
// the percept. In particular see [SurveilOutput.Reacquired], which refines
// Refreshed rather than carving subjects out of it.
type SurveilOutput struct {
	// FirstContact is every subject this observer had no holding for at
	// all, now created by this percept. The payload rides along because
	// nothing the caller holds could supply it.
	FirstContact []Report

	// Refreshed is every subject whose holding already existed and was
	// re-reported. It fires on EVERY pass for EVERY perceived subject —
	// it is "I have a holding for you and I just wrote to it", not a
	// transition. A caller looking for a moment wants one of the two
	// below.
	Refreshed []Subject

	// Changed is every subject in Refreshed whose held payload DIFFERED
	// from the one that just landed — the bytes.Equal comparison the
	// store already makes, below, to decide whether Observed moves.
	// It is the answer to "did this observer's knowledge actually
	// change", reported rather than thrown away, so no consumer has to
	// infer it from stamps: that reconstruction is only correct while At
	// strictly increases, and this field does not depend on At at all.
	//
	// IT REFINES Refreshed AND DOES NOT PARTITION IT, for the same reason
	// [SurveilOutput.Reacquired] does, below. A changed subject already
	// had a holding, so Refreshed is true of it, and Changed says
	// something further true of it besides — carving it out of Refreshed
	// would be exactly the silent narrowing that comment warns about,
	// where a consumer that never heard of Changed starts dropping
	// subjects it used to see.
	//
	// FIRST CONTACT IS NEVER IN Changed: nothing was held before this
	// pass, so "changed" has no prior state to differ from — FirstContact
	// is already its own signal for that subject.
	//
	// Nil and empty payloads compare equal, so landing an empty payload
	// over a nil one (or the reverse) is not a change.
	Changed []Subject

	// Faded is every subject whose holding stopped being current via any
	// channel because this percept omitted it. The holding survives — the
	// ghost goblin — and the observer keeps what they last saw.
	Faded []Subject

	// Reacquired is every subject whose holding was a GHOST at the top of
	// this pass — held, but current via no channel — and is current again
	// because this percept reported it. It is the exact inverse of Faded,
	// and the moment a consumer means by "they came back into view".
	//
	// IT REFINES Refreshed AND DOES NOT PARTITION IT. A re-acquired
	// subject appears in BOTH lists, because both statements are true of
	// it: a holding already existed (Refreshed) and that holding was dark
	// (Reacquired). Every existing caller that reads FirstContact ∪
	// Refreshed as "everything I perceive right now" keeps the same
	// answer, which is why this was added beside Refreshed rather than
	// carved out of it — a silent narrowing of a published list is how a
	// consumer that never heard about the change starts dropping subjects.
	//
	// CHANNELS ARE THE WHOLE OF THE TEST, not sight specifically. A
	// subject still current via hearing is not a ghost, so sight
	// re-reporting them is a refresh and nothing more. That is the honest
	// reading: nothing came back, because nothing had gone.
	Reacquired []Subject
}

// ReportInput is the input to the Report verb. Topic names the one topic
// every report in the call is about; the empty Topic is the default topic.
type ReportInput struct {
	Observer core.EntityID
	Channel  Channel
	Topic    Topic
	Reports  []Report
	At       uint64
}

// Surveil records a complete percept for an observer+channel+topic. Fading is
// derived: every subject whose holding ON THIS TOPIC was previously current
// via this channel but is absent from Percept has that channel removed from
// that holding's CurrentVia. Holdings on other topics are never read or
// written. If CurrentVia becomes
// empty, the subject fades (still held — the ghost goblin). An empty Percept
// is legal (seeing nothing: fades everything this channel sustained).
//
// RE-ACQUISITION IS DERIVED THE SAME WAY, and symmetrically: a holding that
// was current via NO channel when this pass began, and is current now because
// this percept named it, is reported in [SurveilOutput.Reacquired]. Fading
// and returning are the two transitions a caller can act on; Refreshed is a
// state, and fires every pass. Errors:
// ErrNilInput, ErrNoObserver, ErrNoChannel, ErrNoSubject — validation first,
// all before any mutation (R5).
func (i *Intel) Surveil(in *SurveilInput) (*SurveilOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("surveil: %w", ErrNilInput)
	}

	if in.Observer == "" {
		return nil, fmt.Errorf("surveil: %w", ErrNoObserver)
	}

	if in.Channel == "" {
		return nil, fmt.Errorf("surveil: %w", ErrNoChannel)
	}

	for _, report := range in.Percept {
		if report.Subject == "" {
			return nil, fmt.Errorf("surveil: %w", ErrNoSubject)
		}
	}

	// Dedupe percept (same as Report): last entry wins, survivor at last position
	deduped := dedupeReports(in.Percept)
	dedupeSubjectSet := make(map[Subject]struct{}, len(deduped))
	for _, r := range deduped {
		dedupeSubjectSet[r.Subject] = struct{}{}
	}

	out := &SurveilOutput{}

	// Phantom-observer guard: lazy observer-map creation only after validation
	// and only if the deduped percept is non-empty OR observer already exists
	if len(deduped) == 0 && i.holdings[in.Observer] == nil {
		// Empty percept for unknown observer must not create state
		return out, nil
	}

	// Ensure observer exists if we're about to do something
	if i.holdings[in.Observer] == nil {
		i.holdings[in.Observer] = make(map[Subject]map[Topic]*holding)
	}

	// FADE PASS: from pre-mutation state, remove channel from holdings
	// whose currentVia contains this channel AND subject absent from deduped percept
	var fadedSubjects []Subject
	for subj, topics := range i.holdings[in.Observer] {
		// Only this pass's topic is in scope: a percept about one topic is
		// silent about every other, so it can fade nothing there.
		h, held := topics[in.Topic]
		if !held {
			continue
		}
		// Check if this subject was sustained by this channel
		if _, ok := h.currentVia[in.Channel]; ok {
			// Check if it's absent from the deduped percept
			if _, present := dedupeSubjectSet[subj]; !present {
				// Remove the channel
				delete(h.currentVia, in.Channel)
				// If currentVia is now empty, collect for Faded
				if len(h.currentVia) == 0 {
					fadedSubjects = append(fadedSubjects, subj)
				}
			}
		}
	}

	// Sort Faded by Subject for deterministic transcripts; only assign if non-empty
	if len(fadedSubjects) > 0 {
		slices.SortFunc(fadedSubjects, func(a, b Subject) int {
			if a < b {
				return -1
			}
			if a > b {
				return 1
			}
			return 0
		})
		out.Faded = fadedSubjects
	}

	// LAND PASS: in post-dedupe percept order
	for _, report := range deduped {
		h, exists := i.holdings[in.Observer][report.Subject][in.Topic]
		if !exists {
			// Unknown subject on this topic: create holding with independent
			// payload copy. First contact: observed and confirmed both start
			// here.
			payloadCopy := make([]byte, len(report.Payload))
			copy(payloadCopy, report.Payload)
			i.put(in.Observer, report.Subject, in.Topic, &holding{
				payload:    payloadCopy,
				channel:    in.Channel,
				observed:   in.At,
				confirmed:  in.At,
				currentVia: map[Channel]struct{}{in.Channel: {}},
			})
			// Return independent copy for FirstContact
			fcPayload := make([]byte, len(report.Payload))
			copy(fcPayload, report.Payload)
			out.FirstContact = append(out.FirstContact, Report{
				Subject: report.Subject,
				Payload: fcPayload,
			})
		} else {
			// READ BEFORE THE WRITE. A holding current via no channel is a
			// ghost, and this percept is about to make it current again —
			// but only the line below can tell, because the very next
			// statement destroys the evidence. This is the inverse of the
			// FADE PASS above, detected at the one instant it is visible.
			wasGhost := len(h.currentVia) == 0

			// Known subject: same payload confirms without disturbing
			// observed; a changed payload is a new thing, so both stamps
			// move and the payload is overwritten. Channel is provenance,
			// not content, so it always tracks the latest landing.
			if !bytes.Equal(h.payload, report.Payload) {
				payloadCopy := make([]byte, len(report.Payload))
				copy(payloadCopy, report.Payload)
				h.payload = payloadCopy
				h.observed = in.At
				// Same comparison, same pass: the store already knows this
				// changed, so it says so instead of making a consumer
				// reconstruct it from stamps.
				out.Changed = append(out.Changed, report.Subject)
			}
			h.channel = in.Channel
			h.confirmed = in.At
			h.currentVia[in.Channel] = struct{}{}
			out.Refreshed = append(out.Refreshed, report.Subject)
			if wasGhost {
				// In percept order, like FirstContact and Refreshed beside
				// it. Faded sorts because it walks a map and has no order
				// of its own; this pass walks a slice and already has one.
				out.Reacquired = append(out.Reacquired, report.Subject)
			}
		}
	}

	return out, nil
}

// ReportOutput is the output of the Report verb. Like [SurveilOutput], every
// subject it names is a holding on the input's one Topic.
type ReportOutput struct {
	FirstContact []Report
	Updated      []Subject

	// Changed is every subject in Updated whose held payload DIFFERED
	// from the one that just landed — the same bytes.Equal comparison
	// [SurveilOutput.Changed] reports for Surveil, made here instead
	// against Updated. It refines Updated and does not partition it, for
	// the identical reason: a changed subject is still an updated one.
	// First contact is never in Changed; nil and empty payloads are the
	// same identical-nothing, so replacing one with the other is not a
	// change.
	Changed []Subject
}

// Report lands discrete testimony as HELD, on the input's one Topic. A
// subject unknown on that topic creates a new holding; a known one is
// overwritten. Holdings on other topics are never read or written. Dedupes first, last wins,
// survivor at last occurrence's position. Validates in order: nil → observer
// → channel → subjects. All validation before any mutation (R5).
func (i *Intel) Report(in *ReportInput) (*ReportOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("report: %w", ErrNilInput)
	}

	if in.Observer == "" {
		return nil, fmt.Errorf("report: %w", ErrNoObserver)
	}

	if in.Channel == "" {
		return nil, fmt.Errorf("report: %w", ErrNoChannel)
	}

	for _, report := range in.Reports {
		if report.Subject == "" {
			return nil, fmt.Errorf("report: %w", ErrNoSubject)
		}
	}

	// Dedupe: last wins, survivor at last occurrence's position
	deduped := dedupeReports(in.Reports)

	out := &ReportOutput{
		FirstContact: []Report{},
		Updated:      []Subject{},
	}

	// No testimony means no mutation (R9 unreachable state guard)
	if len(deduped) == 0 {
		return out, nil
	}

	// Ensure observer map exists (only after confirming work to do)
	if _, exists := i.holdings[in.Observer]; !exists {
		i.holdings[in.Observer] = make(map[Subject]map[Topic]*holding)
	}

	// Process each deduplicated report
	for _, report := range deduped {
		// Make two independent copies: one for storage, one for FirstContact (copy-out R4)
		storageCopy := make([]byte, len(report.Payload))
		copy(storageCopy, report.Payload)

		h, exists := i.holdings[in.Observer][report.Subject][in.Topic]
		if !exists {
			// New subject: create holding and add to FirstContact with
			// independent copy. First contact: observed and confirmed both
			// start here.
			firstContactCopy := make([]byte, len(report.Payload))
			copy(firstContactCopy, report.Payload)
			i.put(in.Observer, report.Subject, in.Topic, &holding{
				payload:    storageCopy,
				channel:    in.Channel,
				observed:   in.At,
				confirmed:  in.At,
				currentVia: make(map[Channel]struct{}),
			})
			out.FirstContact = append(out.FirstContact, Report{Subject: report.Subject, Payload: firstContactCopy})
		} else {
			// Known subject: same payload confirms without disturbing
			// observed and without rewriting the payload; a changed payload
			// is a new thing, so both stamps move and the payload is
			// overwritten. Channel and currentVia behave exactly as before:
			// channel always tracks the latest landing, currentVia is
			// untouched (a rumor is not a sighting).
			if !bytes.Equal(h.payload, report.Payload) {
				h.payload = storageCopy
				h.observed = in.At
				out.Changed = append(out.Changed, report.Subject)
			}
			h.channel = in.Channel
			h.confirmed = in.At
			out.Updated = append(out.Updated, report.Subject)
		}
	}

	return out, nil
}

// put stores a holding at (observer, subject, topic), creating the
// subject's topic map on demand. The observer map must already exist.
func (i *Intel) put(observer core.EntityID, subject Subject, topic Topic, h *holding) {
	topics := i.holdings[observer][subject]
	if topics == nil {
		topics = make(map[Topic]*holding)
		i.holdings[observer][subject] = topics
	}
	topics[topic] = h
}

// HeldByInput is the input to the HeldBy query.
type HeldByInput struct {
	Observer core.EntityID
}

// HeldBy returns all holdings for an observer across every topic, sorted by
// Subject and then by Topic, with statuses derived and payloads deep-copied.
// Unknown observer returns empty slice, nil error.
func (i *Intel) HeldBy(in *HeldByInput) ([]Holding, error) {
	if in == nil {
		return nil, fmt.Errorf("held by: %w", ErrNilInput)
	}

	if in.Observer == "" {
		return nil, fmt.Errorf("held by: %w", ErrNoObserver)
	}

	results := []Holding{}
	for subject, topics := range i.holdings[in.Observer] {
		for topic, h := range topics {
			results = append(results, h.toHolding(subject, topic))
		}
	}

	slices.SortFunc(results, compareSubjectThenTopic)

	return results, nil
}

// AboutInput is the input to the About query.
type AboutInput struct {
	Observer core.EntityID
	Subject  Subject
}

// About returns everything an observer holds about one subject, one holding
// per topic, sorted by Topic, with statuses derived and payloads
// deep-copied. An observer holding nothing about the subject answers an
// empty slice, nil error. Validates in order: nil → observer → subject.
func (i *Intel) About(in *AboutInput) ([]Holding, error) {
	if in == nil {
		return nil, fmt.Errorf("about: %w", ErrNilInput)
	}

	if in.Observer == "" {
		return nil, fmt.Errorf("about: %w", ErrNoObserver)
	}

	if in.Subject == "" {
		return nil, fmt.Errorf("about: %w", ErrNoSubject)
	}

	topics := i.holdings[in.Observer][in.Subject]
	results := make([]Holding, 0, len(topics))
	for topic, h := range topics {
		results = append(results, h.toHolding(in.Subject, topic))
	}

	slices.SortFunc(results, compareSubjectThenTopic)

	return results, nil
}

// OnTopicInput is the input to the OnTopic query. The empty Topic is the
// default topic and is a legal question.
type OnTopicInput struct {
	Observer core.EntityID
	Topic    Topic
}

// OnTopic returns everything an observer holds on one topic, one holding per
// subject, sorted by Subject, with statuses derived and payloads
// deep-copied. An observer holding nothing on the topic answers an empty
// slice, nil error. Validates in order: nil → observer.
func (i *Intel) OnTopic(in *OnTopicInput) ([]Holding, error) {
	if in == nil {
		return nil, fmt.Errorf("on topic: %w", ErrNilInput)
	}

	if in.Observer == "" {
		return nil, fmt.Errorf("on topic: %w", ErrNoObserver)
	}

	results := []Holding{}
	for subject, topics := range i.holdings[in.Observer] {
		if h, held := topics[in.Topic]; held {
			results = append(results, h.toHolding(subject, in.Topic))
		}
	}

	slices.SortFunc(results, compareSubjectThenTopic)

	return results, nil
}

// OnInput is the input to the On query. The empty Topic is the default
// topic.
type OnInput struct {
	Observer core.EntityID
	Subject  Subject
	Topic    Topic
}

// On returns the single holding for an observer, subject and topic.
// Returns ErrNotHeld if not held. Payload is deep-copied.
func (i *Intel) On(in *OnInput) (Holding, error) {
	if in == nil {
		return Holding{}, fmt.Errorf("on: %w", ErrNilInput)
	}

	if in.Observer == "" {
		return Holding{}, fmt.Errorf("on: %w", ErrNoObserver)
	}

	if in.Subject == "" {
		return Holding{}, fmt.Errorf("on: %w", ErrNoSubject)
	}

	h, exists := i.holdings[in.Observer][in.Subject][in.Topic]
	if !exists {
		return Holding{}, fmt.Errorf("on: %w", ErrNotHeld)
	}

	return h.toHolding(in.Subject, in.Topic), nil
}

// compareSubjectThenTopic is the module's one read order: by Subject, then
// by Topic within a subject.
func compareSubjectThenTopic(a, b Holding) int {
	if c := strings.Compare(string(a.Subject), string(b.Subject)); c != 0 {
		return c
	}
	return strings.Compare(string(a.Topic), string(b.Topic))
}

// dedupeReports deduplicates reports, last wins, survivor at last
// occurrence's position.
func dedupeReports(reports []Report) []Report {
	// Track last occurrence of each subject
	lastOccurrence := make(map[Subject]int)
	for i, report := range reports {
		lastOccurrence[report.Subject] = i
	}

	// Collect survivors in their last occurrence positions
	var result []Report
	for i, report := range reports {
		if lastOccurrence[report.Subject] == i {
			// Payload passed through as-is: both callers copy independently
			// into storage and outputs, so a dedupe-level copy is redundant.
			result = append(result, Report{Subject: report.Subject, Payload: report.Payload})
		}
	}

	return result
}

// toHolding converts an internal holding to an exported Holding,
// deep-copying the payload and sorting CurrentVia.
func (h *holding) toHolding(subject Subject, topic Topic) Holding {
	// Deep-copy payload (nil payloads remain nil; empty payloads are copied)
	var payloadCopy []byte
	if h.payload != nil {
		payloadCopy = make([]byte, len(h.payload))
		copy(payloadCopy, h.payload)
	}

	// Convert currentVia map to sorted slice
	var currentVia []Channel
	if len(h.currentVia) > 0 {
		for ch := range h.currentVia {
			currentVia = append(currentVia, ch)
		}
		// Sort for determinism
		slices.SortFunc(currentVia, func(a, b Channel) int {
			if a < b {
				return -1
			}
			if a > b {
				return 1
			}
			return 0
		})
	}

	// Derive status
	status := Held
	if len(h.currentVia) > 0 {
		status = Current
	}

	return Holding{
		Subject:    subject,
		Topic:      topic,
		Payload:    payloadCopy,
		Channel:    h.channel,
		Observed:   h.observed,
		Confirmed:  h.confirmed,
		CurrentVia: currentVia,
		Status:     status,
	}
}
