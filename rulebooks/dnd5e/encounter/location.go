// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

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

// DecodeSightPayload is the compatibility seam for callers that only need a
// known sight position. Unknown testimony is intentionally not a position.
func DecodeSightPayload(payload []byte) (spatial.Position, bool) {
	location, ok := DecodeLocationPayload(payload)
	if !ok || location.State != LocationKnown {
		return spatial.Position{}, false
	}
	return location.Position, true
}
