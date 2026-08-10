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
type SurveilOutput struct {
	FirstContact []Report
	Refreshed    []Subject
	Faded        []Subject
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
// is legal (seeing nothing: fades everything this channel sustained). Errors:
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
			// Known: overwrite payload (copy), channel, at, and add channel to currentVia
			payloadCopy := make([]byte, len(report.Payload))
			copy(payloadCopy, report.Payload)
			h.payload = payloadCopy
			h.channel = in.Channel
			h.at = in.At
			h.currentVia[in.Channel] = struct{}{}
			out.Refreshed = append(out.Refreshed, report.Subject)
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
		return nil, fmt.Errorf("%w", ErrNilInput)
	}

	if in.Observer == "" {
		return nil, fmt.Errorf("%w", ErrNoObserver)
	}

	if in.Channel == "" {
		return nil, fmt.Errorf("%w", ErrNoChannel)
	}

	for _, report := range in.Reports {
		if report.Subject == "" {
			return nil, fmt.Errorf("%w", ErrNoSubject)
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
		return nil, fmt.Errorf("%w", ErrNilInput)
	}

	if in.Observer == "" {
		return nil, fmt.Errorf("%w", ErrNoObserver)
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
		return Holding{}, fmt.Errorf("%w", ErrNilInput)
	}

	if in.Observer == "" {
		return Holding{}, fmt.Errorf("%w", ErrNoObserver)
	}

	if in.Subject == "" {
		return Holding{}, fmt.Errorf("%w", ErrNoSubject)
	}

	observerHoldings, exists := i.holdings[in.Observer]
	if !exists {
		return Holding{}, fmt.Errorf("%w", ErrNotHeld)
	}

	h, exists := observerHoldings[in.Subject]
	if !exists {
		return Holding{}, fmt.Errorf("%w", ErrNotHeld)
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
			// Make a copy of the payload
			payloadCopy := make([]byte, len(report.Payload))
			copy(payloadCopy, report.Payload)
			result = append(result, Report{Subject: report.Subject, Payload: payloadCopy})
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
