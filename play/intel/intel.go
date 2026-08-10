// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel

import (
	"fmt"

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

// ReportInput is the input to the Report verb.
type ReportInput struct {
	Observer core.EntityID
	Channel  Channel
	Reports  []Report
	At       uint64
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
	var results []Holding
	for subject, h := range observerHoldings {
		results = append(results, h.toHolding(subject))
	}

	// Sort by Subject (deterministic order)
	for idx := 0; idx < len(results)-1; idx++ {
		for j := idx + 1; j < len(results); j++ {
			if results[j].Subject < results[idx].Subject {
				results[idx], results[j] = results[j], results[idx]
			}
		}
	}

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

	// Build result with survivors in their last position order
	seen := make(map[Subject]bool)
	var result []Report
	for i, report := range reports {
		if lastOccurrence[report.Subject] == i && !seen[report.Subject] {
			// This is the last occurrence of this subject
			seen[report.Subject] = true
		}
	}

	// Collect survivors in their last occurrence positions
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
	// Deep-copy payload
	payloadCopy := make([]byte, len(h.payload))
	copy(payloadCopy, h.payload)

	// Convert currentVia map to sorted slice
	var currentVia []Channel
	if len(h.currentVia) > 0 {
		for ch := range h.currentVia {
			currentVia = append(currentVia, ch)
		}
		// Sort for determinism
		for i := 0; i < len(currentVia)-1; i++ {
			for j := i + 1; j < len(currentVia); j++ {
				if currentVia[j] < currentVia[i] {
					currentVia[i], currentVia[j] = currentVia[j], currentVia[i]
				}
			}
		}
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
