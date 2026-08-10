// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel

import (
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// IntelData is the persistent representation of Intel state.
// Holdings maps observer ID → (subject → holding data).
type IntelData struct {
	Holdings map[core.EntityID]map[Subject]HoldingData `json:"holdings,omitempty"`
}

// HoldingData is the persistent representation of a single holding.
// Payload is the opaque testimony. Channel and At are provenance.
// CurrentVia lists channels currently sustaining this holding.
// All fields are omitempty: nil payload marshals to omitted field,
// and empty CurrentVia marshals to omitted (since it's never an empty non-nil slice).
type HoldingData struct {
	Payload    []byte    `json:"payload,omitempty"`
	Channel    Channel   `json:"channel,omitempty"`
	At         uint64    `json:"at,omitempty"`
	CurrentVia []Channel `json:"current_via,omitempty"`
}

// ToData returns a persistent snapshot of this Intel.
// Returns zero IntelData (nil Holdings map, not empty non-nil) if Intel is empty (R8 idle convention).
// All payloads and slices are deep-copied; mutating the returned IntelData
// will not affect this Intel.
func (i *Intel) ToData() IntelData {
	if len(i.holdings) == 0 {
		// Idle convention: return zero value with nil map, not empty map
		return IntelData{}
	}

	// Copy holdings map
	result := IntelData{
		Holdings: make(map[core.EntityID]map[Subject]HoldingData),
	}

	for obs, subjects := range i.holdings {
		subjectMap := make(map[Subject]HoldingData)
		for subj, h := range subjects {
			// Deep-copy payload
			payloadCopy := make([]byte, len(h.payload))
			copy(payloadCopy, h.payload)

			// Copy CurrentVia slice
			var currentViaCopy []Channel
			if len(h.currentVia) > 0 {
				currentViaCopy = make([]Channel, len(h.currentVia))
				// Extract channels from map in sorted order for determinism
				var channels []Channel
				for ch := range h.currentVia {
					channels = append(channels, ch)
				}
				slices.SortFunc(channels, func(a, b Channel) int {
					if a < b {
						return -1
					}
					if a > b {
						return 1
					}
					return 0
				})
				copy(currentViaCopy, channels)
			}

			subjectMap[subj] = HoldingData{
				Payload:    payloadCopy,
				Channel:    h.channel,
				At:         h.at,
				CurrentVia: currentViaCopy,
			}
		}
		result.Holdings[obs] = subjectMap
	}

	return result
}

// LoadIntel reconstructs an Intel from persistent data.
// Returns (*Intel, error). On error, returns nil Intel and error wrapping ErrInvalidData.
// Validates in deterministic order: observer keys, subject keys, channels, CurrentVia.
// All validation before any mutation (R5). Payload nil is legal (see HoldingData doc).
// Deep-copies all data: mutating the caller's IntelData after LoadIntel will not
// affect the loaded Intel (R4).
func LoadIntel(data IntelData) (*Intel, error) {
	// R5: validate everything first, then mutate
	if data.Holdings != nil {
		// Validate all keys and fields
		for obs, subjectMap := range data.Holdings {
			// Empty observer key is rejected
			if obs == "" {
				return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
			}

			// Nil inner map is rejected
			if subjectMap == nil {
				return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
			}

			// Empty (non-nil) inner map is rejected (unreachable state)
			if len(subjectMap) == 0 {
				return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
			}

			for subj, hd := range subjectMap {
				// Empty subject key is rejected
				if subj == "" {
					return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
				}

				// Empty holding Channel is rejected
				if hd.Channel == "" {
					return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
				}

				// Validate CurrentVia: no empty channels, no duplicates
				seen := make(map[Channel]struct{})
				for _, ch := range hd.CurrentVia {
					if ch == "" {
						return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
					}
					if _, dup := seen[ch]; dup {
						return nil, fmt.Errorf("load intel: %w", ErrInvalidData)
					}
					seen[ch] = struct{}{}
				}
			}
		}
	}

	// All validated; now reconstruct the Intel
	intel := &Intel{
		holdings: make(map[core.EntityID]map[Subject]*holding),
	}

	if data.Holdings != nil {
		for obs, subjectMap := range data.Holdings {
			intel.holdings[obs] = make(map[Subject]*holding)
			for subj, hd := range subjectMap {
				// Deep-copy payload (nil payloads remain nil; empty payloads are copied)
				var payloadCopy []byte
				if hd.Payload != nil {
					payloadCopy = make([]byte, len(hd.Payload))
					copy(payloadCopy, hd.Payload)
				}

				// Convert CurrentVia slice to internal set
				currentVia := make(map[Channel]struct{})
				for _, ch := range hd.CurrentVia {
					currentVia[ch] = struct{}{}
				}

				intel.holdings[obs][subj] = &holding{
					payload:    payloadCopy,
					channel:    hd.Channel,
					at:         hd.At,
					currentVia: currentVia,
				}
			}
		}
	}

	return intel, nil
}
