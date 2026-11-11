// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel

import (
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Data is the persistent representation of Intel state.
// Holdings maps observer ID → subject → topic → holding data: the record's
// three parts, holder, subject and topic, in that order. The default
// (empty) topic is stored under the JSON key "".
type Data struct {
	Holdings map[core.EntityID]map[Subject]map[Topic]HoldingData `json:"holdings,omitempty"`
}

// HoldingData is the persistent representation of a single holding.
// Payload is the opaque testimony. Channel is provenance. Observed is when
// this payload was first seen; Confirmed is when it was last landed,
// whether or not the content changed. CurrentVia lists channels currently
// sustaining this holding. All fields are omitempty: nil payload marshals
// to omitted field, and empty CurrentVia marshals to omitted (since it's
// never an empty non-nil slice).
type HoldingData struct {
	Payload    []byte    `json:"payload,omitempty"`
	Channel    Channel   `json:"channel,omitempty"`
	Observed   uint64    `json:"observed,omitempty"`
	Confirmed  uint64    `json:"confirmed,omitempty"`
	CurrentVia []Channel `json:"current_via,omitempty"`
}

// ToData returns a persistent snapshot of this Intel.
// Returns zero Data (nil Holdings map, not empty non-nil) if Intel is empty (R8 idle convention).
// All payloads and slices are deep-copied; mutating the returned Data
// will not affect this Intel. Encoding is deterministic: encoding/json
// sorts map keys, and CurrentVia is sorted here.
func (i *Intel) ToData() Data {
	if len(i.holdings) == 0 {
		// Idle convention: return zero value with nil map, not empty map
		return Data{}
	}

	result := Data{
		Holdings: make(map[core.EntityID]map[Subject]map[Topic]HoldingData, len(i.holdings)),
	}

	for obs, subjects := range i.holdings {
		subjectMap := make(map[Subject]map[Topic]HoldingData, len(subjects))
		for subj, topics := range subjects {
			topicMap := make(map[Topic]HoldingData, len(topics))
			for topic, h := range topics {
				topicMap[topic] = h.toData()
			}
			subjectMap[subj] = topicMap
		}
		result.Holdings[obs] = subjectMap
	}

	return result
}

// toData deep-copies one internal holding into its persistent shape.
func (h *holding) toData() HoldingData {
	// Deep-copy payload (only when not nil)
	var payloadCopy []byte
	if h.payload != nil {
		payloadCopy = make([]byte, len(h.payload))
		copy(payloadCopy, h.payload)
	}

	// CurrentVia from the internal set, sorted for determinism
	var currentVia []Channel
	if len(h.currentVia) > 0 {
		currentVia = make([]Channel, 0, len(h.currentVia))
		for ch := range h.currentVia {
			currentVia = append(currentVia, ch)
		}
		slices.Sort(currentVia)
	}

	return HoldingData{
		Payload:    payloadCopy,
		Channel:    h.channel,
		Observed:   h.observed,
		Confirmed:  h.confirmed,
		CurrentVia: currentVia,
	}
}

// LoadIntel reconstructs an Intel from persistent data.
// Returns (*Intel, error). On error, returns nil Intel and error wrapping ErrInvalidData.
// Validates every holding: within a single holding the checks run in a fixed
// order (observer key, nil/empty subject map, subject key, nil/empty topic
// map, channel, CurrentVia including empty-channel and duplicate detection,
// then Confirmed < Observed), but observers, subjects and topics are visited
// in Go map order — which defect of a multi-defect Data is hit first is NOT
// deterministic. Every rejection wraps ErrInvalidData with no partial state
// constructed (R5); all but the last check carry no further defect detail,
// while a rejected Confirmed < Observed also names the offending observer,
// subject and topic, since a memory re-confirmed before it was made is a
// data-integrity defect worth locating. All validation completes before any
// construction.
// The empty topic key is legal: it is the default topic.
// Payload nil is legal (see HoldingData doc).
// Deep-copies all data: mutating the caller's Data after LoadIntel will not
// affect the loaded Intel (R4).
func LoadIntel(data Data) (*Intel, error) {
	// R5: validate everything first, then mutate
	for obs, subjectMap := range data.Holdings {
		// Empty observer key is rejected
		if obs == "" {
			return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
		}

		// Nil or empty subject map is rejected (unreachable state)
		if len(subjectMap) == 0 {
			return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
		}

		for subj, topicMap := range subjectMap {
			// Empty subject key is rejected
			if subj == "" {
				return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
			}

			// Nil or empty topic map is rejected (unreachable: a subject
			// entry exists only because a holding on some topic does)
			if len(topicMap) == 0 {
				return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
			}

			for topic, hd := range topicMap {
				if err := validateHoldingData(obs, subj, topic, hd); err != nil {
					return nil, err
				}
			}
		}
	}

	// All validated; now reconstruct the Intel
	intel := &Intel{
		holdings: make(map[core.EntityID]map[Subject]map[Topic]*holding, len(data.Holdings)),
	}

	for obs, subjectMap := range data.Holdings {
		intel.holdings[obs] = make(map[Subject]map[Topic]*holding, len(subjectMap))
		for subj, topicMap := range subjectMap {
			for topic, hd := range topicMap {
				intel.put(obs, subj, topic, holdingFromData(hd))
			}
		}
	}

	return intel, nil
}

// validateHoldingData runs the per-holding R9 checks.
func validateHoldingData(obs core.EntityID, subj Subject, topic Topic, hd HoldingData) error {
	// Empty holding Channel is rejected
	if hd.Channel == "" {
		return fmt.Errorf("load intel: %w", ErrInvalidData)
	}

	// Validate CurrentVia: no empty channels, no duplicates
	seen := make(map[Channel]struct{}, len(hd.CurrentVia))
	for _, ch := range hd.CurrentVia {
		if ch == "" {
			return fmt.Errorf("load intel: %w", ErrInvalidData)
		}
		if _, dup := seen[ch]; dup {
			return fmt.Errorf("load intel: %w", ErrInvalidData)
		}
		seen[ch] = struct{}{}
	}

	// A memory cannot be re-confirmed before it was made: no write can
	// produce Confirmed < Observed.
	if hd.Confirmed < hd.Observed {
		return fmt.Errorf("load intel: observer %s subject %s topic %q: %w", obs, subj, topic, ErrInvalidData)
	}

	return nil
}

// holdingFromData deep-copies one persisted holding into internal state.
func holdingFromData(hd HoldingData) *holding {
	// Deep-copy payload (nil payloads remain nil; empty payloads are copied)
	var payloadCopy []byte
	if hd.Payload != nil {
		payloadCopy = make([]byte, len(hd.Payload))
		copy(payloadCopy, hd.Payload)
	}

	// Convert CurrentVia slice to internal set
	currentVia := make(map[Channel]struct{}, len(hd.CurrentVia))
	for _, ch := range hd.CurrentVia {
		currentVia[ch] = struct{}{}
	}

	return &holding{
		payload:    payloadCopy,
		channel:    hd.Channel,
		observed:   hd.Observed,
		confirmed:  hd.Confirmed,
		currentVia: currentVia,
	}
}
