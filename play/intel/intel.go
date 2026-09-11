// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel

import (
	"fmt"
	"slices"

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

// Report is one piece of testimony: a subject and an opaque payload.
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

// Holding is the read-side value: a subject, payload, channel provenance,
// timestamp, active channels (CurrentVia), and derived status. Channel and
// At are provenance of the latest accepted testimony. Status is derived
// (Current iff CurrentVia is non-empty).
type Holding struct {
	Subject    Subject
	Payload    []byte
	Channel    Channel
	At         uint64
	CurrentVia []Channel
	Status     Status
}

// Intel is the container for all observers' intel holdings.
// Not safe for concurrent use. Zero value not usable; construct via
// NewIntel or LoadIntel.
type Intel struct {
	holdings map[core.EntityID]map[Subject]*holding
}

// internal holding struct stores the mutable state.
type holding struct {
	payload    []byte
	channel    Channel
	at         uint64
	currentVia map[Channel]struct{}
}

// NewIntel creates a new Intel container.
func NewIntel() (*Intel, error) {
	return &Intel{
		holdings: make(map[core.EntityID]map[Subject]*holding),
	}, nil
}

// SurveilInput carries a complete percept for this observer+channel.
type SurveilInput struct {
	Observer core.EntityID
	Channel  Channel
	Percept  []Report
	At       uint64
}

// SurveilOutput reports the deltas Surveil caused.
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

// ReportInput is the input to the Report verb.
type ReportInput struct {
	Observer core.EntityID
	Channel  Channel
	Reports  []Report
	At       uint64
}

// Surveil records a complete percept for an observer+channel. Fading is
// derived: every subject previously current via this channel but absent from
// Percept has that channel removed from CurrentVia. If CurrentVia becomes
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
		i.holdings[in.Observer] = make(map[Subject]*holding)
	}

	// FADE PASS: from pre-mutation state, remove channel from holdings
	// whose currentVia contains this channel AND subject absent from deduped percept
	var fadedSubjects []Subject
	for subj, h := range i.holdings[in.Observer] {
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
		h, exists := i.holdings[in.Observer][report.Subject]
		if !exists {
			// Unknown subject: create holding with independent payload copy
			payloadCopy := make([]byte, len(report.Payload))
			copy(payloadCopy, report.Payload)
			i.holdings[in.Observer][report.Subject] = &holding{
				payload:    payloadCopy,
				channel:    in.Channel,
				at:         in.At,
				currentVia: map[Channel]struct{}{in.Channel: {}},
			}
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

			// Known: overwrite payload (copy), channel, at, and add channel to currentVia
			payloadCopy := make([]byte, len(report.Payload))
			copy(payloadCopy, report.Payload)
			h.payload = payloadCopy
			h.channel = in.Channel
			h.at = in.At
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

// ReportOutput is the output of the Report verb.
type ReportOutput struct {
	FirstContact []Report
	Updated      []Subject
}

// Report lands discrete testimony as HELD. Unknown subjects create new
// holdings; known subjects are overwritten. Dedupes first, last wins,
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
		i.holdings[in.Observer] = make(map[Subject]*holding)
	}

	// Process each deduplicated report
	for _, report := range deduped {
		// Make two independent copies: one for storage, one for FirstContact (copy-out R4)
		storageCopy := make([]byte, len(report.Payload))
		copy(storageCopy, report.Payload)

		if _, exists := i.holdings[in.Observer][report.Subject]; !exists {
			// New subject: create holding and add to FirstContact with independent copy
			firstContactCopy := make([]byte, len(report.Payload))
			copy(firstContactCopy, report.Payload)
			i.holdings[in.Observer][report.Subject] = &holding{
				payload:    storageCopy,
				channel:    in.Channel,
				at:         in.At,
				currentVia: make(map[Channel]struct{}),
			}
			out.FirstContact = append(out.FirstContact, Report{Subject: report.Subject, Payload: firstContactCopy})
		} else {
			// Known subject: overwrite payload, channel, at; leave currentVia untouched
			h := i.holdings[in.Observer][report.Subject]
			h.payload = storageCopy
			h.channel = in.Channel
			h.at = in.At
			out.Updated = append(out.Updated, report.Subject)
		}
	}

	return out, nil
}

// HeldByInput is the input to the HeldBy query.
type HeldByInput struct {
	Observer core.EntityID
}

// HeldBy returns all holdings for an observer, sorted by Subject,
// with statuses derived and payloads deep-copied. Unknown observer
// returns empty slice, nil error.
func (i *Intel) HeldBy(in *HeldByInput) ([]Holding, error) {
	if in == nil {
		return nil, fmt.Errorf("held by: %w", ErrNilInput)
	}

	if in.Observer == "" {
		return nil, fmt.Errorf("held by: %w", ErrNoObserver)
	}

	observerHoldings, exists := i.holdings[in.Observer]
	if !exists {
		return []Holding{}, nil
	}

	// Collect and convert holdings to exported type
	results := make([]Holding, 0, len(observerHoldings))
	for subject, h := range observerHoldings {
		results = append(results, h.toHolding(subject))
	}

	// Sort by Subject (deterministic order)
	slices.SortFunc(results, func(a, b Holding) int {
		if a.Subject < b.Subject {
			return -1
		}
		if a.Subject > b.Subject {
			return 1
		}
		return 0
	})

	return results, nil
}

// OnInput is the input to the On query.
type OnInput struct {
	Observer core.EntityID
	Subject  Subject
}

// On returns a single holding for an observer and subject.
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

	observerHoldings, exists := i.holdings[in.Observer]
	if !exists {
		return Holding{}, fmt.Errorf("on: %w", ErrNotHeld)
	}

	h, exists := observerHoldings[in.Subject]
	if !exists {
		return Holding{}, fmt.Errorf("on: %w", ErrNotHeld)
	}

	return h.toHolding(in.Subject), nil
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
func (h *holding) toHolding(subject Subject) Holding {
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
		Payload:    payloadCopy,
		Channel:    h.channel,
		At:         h.at,
		CurrentVia: currentVia,
		Status:     status,
	}
}
