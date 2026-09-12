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

// LocationState identifies whether sight testimony is Known(position) or
// Unknown. Location content is independent of Intel's Current or Held currency;
// encounter separately rejects the unsupported Current + Unknown combination.
type LocationState string

const (
	// LocationKnown says the testimony carries a position.
	LocationKnown LocationState = "known"
	// LocationUnknown says the subject remains known without a position.
	LocationUnknown LocationState = "unknown"
)

// SightTestimony is the encounter-owned meaning of a sight payload: everything
// one observer claims to have seen about one subject, at the instant they saw
// it.
//
// # Why this is testimony and not a view of the world
//
// Every field here is SNAPSHOTTED at sight time and thereafter belongs to the
// observer who took it. It is free to disagree with the world, with the
// subject's sheet, and with every other observer — that is not a defect to be
// reconciled, it is the point. A charmed player who believes the ogre is
// holding a toy is a false payload written into that one player's holding; a
// memory (intel Held, CurrentVia empty) keeps what it last saw while the world
// moves on without it.
//
// This is why facts live HERE rather than being read live when somebody asks.
// A live read can only ever return the truth, which forecloses illusion,
// disguise and enchantment permanently rather than deferring them (Kirk,
// rpg-toolkit#1615). Anything an observer can be WRONG about belongs in this
// struct.
//
// # Unknown carries nothing
//
// Unknown testimony means the subject is known to exist without being placed —
// so there is nobody in view to have a standing or a pair of hands. Unknown
// therefore carries no position, no standing and no equipment, and encoding one
// that does is refused rather than silently trimmed.
//
// # Each fact says whether it was observed at all
//
// Standing and Equipment are pointers because "not observed" and "observed to
// be X" are different claims that must not collapse. A bool that is false
// whether the subject is upright or was never looked at is a zero value that
// lies, and testimony is the last place that should happen. Testimony written
// by an older build carries neither, and says so honestly until the next sight
// refresh replaces it — which happens on the very next beat.
type SightTestimony struct {
	// State says whether this testimony places the subject.
	State LocationState

	// Position is where the subject was seen, dungeon-absolute. Meaningful
	// only when State is LocationKnown.
	Position spatial.Position

	// Down is whether the subject was seen off their feet. Nil means standing
	// was not observed — which is not the same as "seen standing".
	Down *bool

	// Equipment is what the subject was seen holding. Nil means their hands
	// were not observed, which is not the same as seeing empty hands; see
	// [Equipment] for the two claims.
	Equipment *HeldEquipment
}

type handsWire struct {
	MainHand string `json:"main_hand,omitempty"`
	OffHand  string `json:"off_hand,omitempty"`
}

type sightWire struct {
	State     string     `json:"state,omitempty"`
	X         *float64   `json:"x,omitempty"`
	Y         *float64   `json:"y,omitempty"`
	Down      *bool      `json:"down,omitempty"`
	Equipment *handsWire `json:"equipment,omitempty"`
}

// sightPayloadFields is the complete set of keys canonical sight testimony may
// carry. It is enumerated rather than inferred so that a key this build does
// not understand is refused at the door instead of being silently dropped into
// a testimony that then reads as confident.
var sightPayloadFields = map[string]struct{}{
	"state": {}, "x": {}, "y": {}, "down": {}, "equipment": {},
}

// EncodeSightTestimony encodes sight testimony in the canonical tagged wire
// form. Known testimony always carries both coordinates; unknown testimony
// carries only its state tag, and carrying anything else is a defect rather
// than a value to trim.
func EncodeSightTestimony(testimony SightTestimony) ([]byte, error) {
	switch testimony.State {
	case LocationKnown:
		x, y := testimony.Position.X, testimony.Position.Y
		wire := sightWire{State: string(LocationKnown), X: &x, Y: &y, Down: testimony.Down}
		if testimony.Equipment != nil {
			wire.Equipment = &handsWire{
				MainHand: testimony.Equipment.MainHand,
				OffHand:  testimony.Equipment.OffHand,
			}
		}
		return json.Marshal(wire)
	case LocationUnknown:
		if testimony.Position != (spatial.Position{}) {
			return nil, fmt.Errorf("unknown location cannot carry a position")
		}
		if testimony.Down != nil {
			return nil, fmt.Errorf("unknown location cannot carry standing")
		}
		if testimony.Equipment != nil {
			return nil, fmt.Errorf("unknown location cannot carry equipment")
		}
		return json.Marshal(sightWire{State: string(LocationUnknown)})
	default:
		return nil, fmt.Errorf("unsupported location state %q", testimony.State)
	}
}

// DecodeSightTestimony decodes canonical tagged sight testimony and the legacy
// untagged known-coordinate form. It rejects malformed, contradictory,
// unknown-field, and trailing JSON rather than inventing testimony.
//
// Testimony written before a fact existed decodes with that fact nil, which is
// the honest reading: an older build did not observe it. The next sight refresh
// replaces the payload wholesale.
func DecodeSightTestimony(payload []byte) (SightTestimony, bool) {
	if len(payload) == 0 {
		return SightTestimony{}, false
	}
	if !hasStrictSightFields(payload) {
		return SightTestimony{}, false
	}

	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	var wire sightWire
	if err := dec.Decode(&wire); err != nil {
		return SightTestimony{}, false
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		return SightTestimony{}, false
	}

	// The wire struct cannot distinguish an omitted state from an explicit
	// empty state, so inspect the original object for that one distinction.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return SightTestimony{}, false
	}
	_, statePresent := fields["state"]
	_, xPresent := fields["x"]
	_, yPresent := fields["y"]
	_, downPresent := fields["down"]
	_, equipmentPresent := fields["equipment"]

	var hands *HeldEquipment
	if wire.Equipment != nil {
		hands = &HeldEquipment{MainHand: wire.Equipment.MainHand, OffHand: wire.Equipment.OffHand}
	}

	if wire.State == "" {
		if statePresent || !xPresent || !yPresent || wire.X == nil || wire.Y == nil {
			return SightTestimony{}, false
		}
		// The legacy untagged form predates every fact but position, so it
		// cannot carry one.
		if downPresent || equipmentPresent {
			return SightTestimony{}, false
		}
		return SightTestimony{
			State:    LocationKnown,
			Position: spatial.Position{X: *wire.X, Y: *wire.Y},
		}, true
	}

	switch LocationState(wire.State) {
	case LocationKnown:
		if wire.X == nil || wire.Y == nil {
			return SightTestimony{}, false
		}
		return SightTestimony{
			State:     LocationKnown,
			Position:  spatial.Position{X: *wire.X, Y: *wire.Y},
			Down:      wire.Down,
			Equipment: hands,
		}, true
	case LocationUnknown:
		// Nobody in view has a position, a standing, or hands to observe.
		if xPresent || yPresent || downPresent || equipmentPresent {
			return SightTestimony{}, false
		}
		return SightTestimony{State: LocationUnknown}, true
	default:
		return SightTestimony{}, false
	}
}

func hasStrictSightFields(payload []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(payload))
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return false
	}

	seen := make(map[string]struct{}, len(sightPayloadFields))
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		key, ok := tok.(string)
		if !ok {
			return false
		}
		if _, allowed := sightPayloadFields[key]; !allowed {
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
	testimony, ok := DecodeSightTestimony(payload)
	if !ok || testimony.State != LocationKnown {
		return spatial.Position{}, false
	}
	return testimony.Position, true
}
