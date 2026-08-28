// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// LocationState identifies whether sight testimony names a current position.
type LocationState string

const (
	LocationKnown   LocationState = "known"
	LocationUnknown LocationState = "unknown"
)

// LocationKnowledge is the encounter-owned meaning of a sight payload.
// Unknown locations intentionally carry no position.
type LocationKnowledge struct {
	State    LocationState
	Position spatial.Position
}

type locationWire struct {
	State string   `json:"state,omitempty"`
	X     *float64 `json:"x,omitempty"`
	Y     *float64 `json:"y,omitempty"`
}

// EncodeLocationPayload encodes location knowledge in the canonical tagged
// wire form. Known locations always carry both coordinates; unknown locations
// carry only their state tag.
func EncodeLocationPayload(location LocationKnowledge) ([]byte, error) {
	switch location.State {
	case LocationKnown:
		x, y := location.Position.X, location.Position.Y
		return json.Marshal(locationWire{State: string(LocationKnown), X: &x, Y: &y})
	case LocationUnknown:
		if location.Position != (spatial.Position{}) {
			return nil, fmt.Errorf("unknown location cannot carry a position")
		}
		return json.Marshal(locationWire{State: string(LocationUnknown)})
	default:
		return nil, fmt.Errorf("unsupported location state %q", location.State)
	}
}

// DecodeLocationPayload decodes canonical tagged location testimony and the
// legacy untagged known-coordinate form. It rejects malformed, contradictory,
// unknown-field, and trailing JSON rather than inventing a position.
func DecodeLocationPayload(payload []byte) (LocationKnowledge, bool) {
	if len(payload) == 0 {
		return LocationKnowledge{}, false
	}
	if !hasStrictLocationFields(payload) {
		return LocationKnowledge{}, false
	}

	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	var wire locationWire
	if err := dec.Decode(&wire); err != nil {
		return LocationKnowledge{}, false
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		return LocationKnowledge{}, false
	}

	// The wire struct cannot distinguish an omitted state from an explicit
	// empty state, so inspect the original object for that one distinction.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return LocationKnowledge{}, false
	}
	_, statePresent := fields["state"]
	_, xPresent := fields["x"]
	_, yPresent := fields["y"]
	if wire.State == "" {
		if statePresent || !xPresent || !yPresent || wire.X == nil || wire.Y == nil {
			return LocationKnowledge{}, false
		}
		return LocationKnowledge{State: LocationKnown, Position: spatial.Position{X: *wire.X, Y: *wire.Y}}, true
	}

	switch LocationState(wire.State) {
	case LocationKnown:
		if wire.X == nil || wire.Y == nil {
			return LocationKnowledge{}, false
		}
		return LocationKnowledge{State: LocationKnown, Position: spatial.Position{X: *wire.X, Y: *wire.Y}}, true
	case LocationUnknown:
		if xPresent || yPresent {
			return LocationKnowledge{}, false
		}
		return LocationKnowledge{State: LocationUnknown}, true
	default:
		return LocationKnowledge{}, false
	}
}

func hasStrictLocationFields(payload []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(payload))
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return false
	}

	seen := make(map[string]struct{}, 3)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		key, ok := tok.(string)
		if !ok || (key != "state" && key != "x" && key != "y") {
			return false
		}
		if _, duplicate := seen[key]; duplicate {
			return false
		}
		seen[key] = struct{}{}

		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return false
		}
	}
	tok, err = dec.Token()
	if err != nil {
		return false
	}
	closeDelim, ok := tok.(json.Delim)
	if !ok || closeDelim != '}' {
		return false
	}

	var trailing json.RawMessage
	return dec.Decode(&trailing) == io.EOF
}

// DecodeSightPayload is the compatibility seam for callers that only need a
// known sight position. Unknown testimony is intentionally not a position.
func DecodeSightPayload(payload []byte) (spatial.Position, bool) {
	location, ok := DecodeLocationPayload(payload)
	if !ok || location.State != LocationKnown {
		return spatial.Position{}, false
	}
	return location.Position, true
}

// correctArrivedLocations replaces stale known sight testimony at the
// observer's arrival cell with canonical unknown testimony.
//
// This is deliberately narrower than a general sight correction. It is called
// only after a driven fight-time move has successfully reached its destination
// and receives the complete percept from that move's refresh. The subject's
// live placement is never consulted: the only location evidence here is the
// mover's own held sight testimony, compared with the mover's own arrival cell.
func (e *Encounter) correctArrivedLocations(
	observer MemberID,
	at uint64,
	perceived *IntelDelta,
) ([]intel.Subject, error) {
	member, ok := e.members[observer]
	if !ok {
		return nil, fmt.Errorf("correct arrived locations: observer %q: %w", observer, ErrNotMember)
	}
	arrival, err := e.cellOf(member)
	if err != nil {
		return nil, fmt.Errorf("correct arrived locations: %w", err)
	}

	perceivedSubjects := make(map[intel.Subject]struct{})
	if perceived != nil {
		for _, report := range perceived.FirstContact {
			perceivedSubjects[report.Subject] = struct{}{}
		}
		for _, subject := range perceived.Refreshed {
			perceivedSubjects[subject] = struct{}{}
		}
	}

	holdings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: observer})
	if err != nil {
		return nil, fmt.Errorf("correct arrived locations held by: %w", err)
	}

	var subjects []intel.Subject
	for _, holding := range holdings {
		if holding.Channel != intel.Sight || holding.Status != intel.Held {
			continue
		}
		location, ok := DecodeLocationPayload(holding.Payload)
		if !ok || location.State != LocationKnown || location.Position != arrival {
			continue
		}
		if _, seen := perceivedSubjects[holding.Subject]; seen {
			continue
		}
		subjects = append(subjects, holding.Subject)
	}

	if len(subjects) == 0 {
		return nil, nil
	}
	sort.Slice(subjects, func(i, j int) bool { return subjects[i] < subjects[j] })
	unknown, err := EncodeLocationPayload(LocationKnowledge{State: LocationUnknown})
	if err != nil {
		return nil, fmt.Errorf("correct arrived locations encode unknown: %w", err)
	}
	reports := make([]intel.Report, 0, len(subjects))
	for _, subject := range subjects {
		reports = append(reports, intel.Report{Subject: subject, Payload: unknown})
	}
	if _, err := e.intelLog.Report(&intel.ReportInput{
		Observer: observer,
		Channel:  intel.Sight,
		Reports:  reports,
		At:       at,
	}); err != nil {
		return nil, fmt.Errorf("correct arrived locations report: %w", err)
	}

	return subjects, nil
}
