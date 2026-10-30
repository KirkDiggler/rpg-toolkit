// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
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
	// LocationLost says the observer went back to where the testimony
	// places the subject and found nobody (rpg-project#668 R3). It keeps
	// everything the last sighting saw — the cell, the facing it was
	// heading — because a search walks on from there; it is never a place
	// to aim at or to walk to, and never current.
	LocationLost LocationState = "lost"
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
// therefore carries no position, no standing, no equipment, no conditions and no facing,
// and encoding one that does is refused rather than silently trimmed.
//
// # Each fact says whether it was observed at all
//
// Standing, Equipment and Conditions are pointers because "not observed" and "observed to
// be X" are different claims that must not collapse. A bool that is false
// whether the subject is upright or was never looked at is a zero value that
// lies, and testimony is the last place that should happen. Testimony written
// by an older build carries neither, and says so honestly until the next sight
// refresh replaces it — which happens on the very next beat.
type SightTestimony struct {
	// State says whether this testimony places the subject.
	State LocationState

	// Position is where the subject was seen, dungeon-absolute. Meaningful
	// only when State is LocationKnown or LocationLost.
	Position spatial.Position

	// AppearanceRef is the optional opaque visual identity captured at sight.
	// Empty means no explicit appearance was captured, including legacy payloads.
	AppearanceRef string

	// DefaultActivityRef is the authored presentation default captured at sight.
	// Empty means no explicit default was captured, including legacy payloads.
	DefaultActivityRef string

	// Down is whether the subject was seen off their feet. Nil means standing
	// was not observed — which is not the same as "seen standing".
	Down *bool

	// BlocksMovement is the observed physical obstruction policy. Nil means unobserved.
	BlocksMovement *bool

	// Equipment is what the subject was seen holding. Nil means their hands
	// were not observed, which is not the same as seeing empty hands; see
	// [Equipment] for the two claims.
	Equipment *HeldEquipment

	// Conditions is every condition the subject was seen holding, as it was
	// at the instant of sight — never a live read of their sheet, and not
	// filtered for perceivability (rpg-project#520 R16). Nil means conditions
	// were not observed, which is not the same as an empty set: seen holding
	// none. See [ConditionSet].
	//
	// STORED, NEVER DELIVERED TO A CLIENT. It persists with the testimony and
	// rules read it through [Encounter.ObservedContext]; every sight payload
	// this module hands a host to relay leaves without it (see
	// [deliveredSightPayload]). The stored form still leaves this module by
	// TWO other exits, both host-side and neither a delivery:
	//
	//   - [MonsterView.Holdings], the raw testimony a [Driver] is handed, so a
	//     mind can read what its creature saw. A host that adapts the view
	//     for its own driver must not relay those payloads to a client.
	//   - [Encounter.ToData], the persisted blob, which a host stores and
	//     hands back to [LoadEncounter], never to a client.
	Conditions *ConditionSet

	// Facing is the way the subject was seen facing: a bearing in degrees, 0
	// is map north, clockwise, in [0, 360) (rpg-project#582, sneaking design
	// R13). Nil means facing was not observed — testimony written before
	// sightings carried one — which is not the same as any bearing. A memory
	// keeps the facing it last saw, as it keeps the hands; nobody reads a
	// subject's facing live.
	Facing *float64

	// Heading is the way THIS OBSERVER saw the subject go: the bearing from
	// the cell its own previous testimony placed the subject on to the cell
	// it sees the subject on now, in Facing's degrees (rpg-project#679 R10).
	// Seen on the same cell, the previous heading is kept; with no placed
	// testimony before, it is nil. So nil means this observer never saw the
	// subject move — a first sighting of a subject standing still says
	// nothing about the step it took out of sight (#673 R8: testimony is per
	// observer) — and a memory keeps the heading it last saw. A `search: {
	// heading: went }` walks along it.
	//
	// UNLIKE FACING, NOT A SNAPSHOT OF THE SUBJECT. Facing is visible at the
	// instant of sight; a past step is not, so the heading is derived per
	// observer in [Encounter.rebuildPercepts] from what that observer held.
	Heading *float64
}

type handsWire struct {
	MainHand string `json:"main_hand,omitempty"`
	OffHand  string `json:"off_hand,omitempty"`
}

type sightWire struct {
	State              string     `json:"state,omitempty"`
	AppearanceRef      string     `json:"appearance_ref,omitempty"`
	DefaultActivityRef string     `json:"default_activity_ref,omitempty"`
	X                  *float64   `json:"x,omitempty"`
	Y                  *float64   `json:"y,omitempty"`
	Down               *bool      `json:"down,omitempty"`
	BlocksMovement     *bool      `json:"blocks_movement,omitempty"`
	Equipment          *handsWire `json:"equipment,omitempty"`
	// Conditions is a pointer to a slice so that nil (not observed) omits the
	// key while an observed empty set encodes [].
	Conditions *[]ConditionKey `json:"conditions,omitempty"`
	Facing     *float64        `json:"facing,omitempty"`
	Heading    *float64        `json:"heading,omitempty"`
}

// sightPayloadFields is the complete set of keys canonical sight testimony may
// carry. It is enumerated rather than inferred so that a key this build does
// not understand is refused at the door instead of being silently dropped into
// a testimony that then reads as confident.
var sightPayloadFields = map[string]struct{}{
	"state": {}, "x": {}, "y": {}, "down": {}, "equipment": {}, "blocks_movement": {}, "conditions": {}, "appearance_ref": {},
	"facing": {}, "heading": {}, "default_activity_ref": {},
}

// EncodeSightTestimony encodes sight testimony in the canonical tagged wire
// form. Known and lost testimony always carry both coordinates; unknown
// testimony carries only its state tag, and carrying anything else is a
// defect rather than a value to trim.
func EncodeSightTestimony(testimony SightTestimony) ([]byte, error) {
	if err := validateAppearanceRef(testimony.AppearanceRef); err != nil {
		return nil, err
	}
	if err := validateDefaultActivityRef(testimony.DefaultActivityRef); err != nil {
		return nil, err
	}
	if err := validateFacing(testimony.Facing); err != nil {
		return nil, asInvalidData(err)
	}
	if err := validateFacing(testimony.Heading); err != nil {
		return nil, asInvalidData(fmt.Errorf("heading: %w", err))
	}
	switch testimony.State {
	case LocationKnown, LocationLost:
		x, y := testimony.Position.X, testimony.Position.Y
		wire := sightWire{State: string(testimony.State), X: &x, Y: &y, Down: testimony.Down, BlocksMovement: testimony.BlocksMovement, AppearanceRef: testimony.AppearanceRef, Facing: testimony.Facing,
			Heading: testimony.Heading, DefaultActivityRef: testimony.DefaultActivityRef}
		if testimony.Equipment != nil {
			wire.Equipment = &handsWire{
				MainHand: testimony.Equipment.MainHand,
				OffHand:  testimony.Equipment.OffHand,
			}
		}
		if testimony.Conditions != nil {
			if err := validateConditionSet(testimony.Conditions); err != nil {
				return nil, fmt.Errorf("invalid conditions: %w", err)
			}
			held := append(make([]ConditionKey, 0, len(testimony.Conditions.Conditions)), testimony.Conditions.Conditions...)
			wire.Conditions = &held
		}
		return json.Marshal(wire)
	case LocationUnknown:
		if testimony.AppearanceRef != "" {
			return nil, asInvalidData(fmt.Errorf("unknown location cannot carry appearance"))
		}
		if testimony.DefaultActivityRef != "" {
			return nil, asInvalidData(fmt.Errorf("unknown location cannot carry default activity"))
		}
		if testimony.Position != (spatial.Position{}) {
			return nil, asInvalidData(fmt.Errorf("unknown location cannot carry a position"))
		}
		if testimony.BlocksMovement != nil {
			return nil, asInvalidData(fmt.Errorf("unknown location cannot carry obstruction"))
		}
		if testimony.Down != nil {
			return nil, asInvalidData(fmt.Errorf("unknown location cannot carry standing"))
		}
		if testimony.Equipment != nil {
			return nil, asInvalidData(fmt.Errorf("unknown location cannot carry equipment"))
		}
		if testimony.Conditions != nil {
			return nil, asInvalidData(fmt.Errorf("unknown location cannot carry conditions"))
		}
		if testimony.Facing != nil {
			return nil, asInvalidData(fmt.Errorf("unknown location cannot carry facing"))
		}
		if testimony.Heading != nil {
			return nil, asInvalidData(fmt.Errorf("unknown location cannot carry a heading"))
		}
		return json.Marshal(sightWire{State: string(LocationUnknown)})
	default:
		return nil, asInvalidData(fmt.Errorf("unsupported location state %q", testimony.State))
	}
}

// deliveredSightPayload returns one stored sight payload as this module hands
// it OUT — through [Encounter.View], [IntelDelta.FirstContact] and
// [ExitOutput.Carry] — which is the stored testimony with its Conditions
// removed.
//
// # Stored and delivered are two encodings on purpose
//
// The stored form (intel, [EncounterData], the minds' [MonsterView.Holdings] and
// [Encounter.ObservedContext]) carries every condition a sighting saw, because
// rules read them there (rpg-project#520 R16). The delivered form is what a
// host relays to a client as an opaque payload, and a client learns what it
// holds only as typed effect rows the rules compiled — never as raw condition
// refs it would have to recognise by name. So the conditions stop at this
// module's edge, and the only place that edge exists is here: a host cannot
// strip what it was never told was there.
//
// A payload whose testimony carries no conditions is returned unchanged,
// byte for byte, so a delivered payload is exactly what it was before
// sightings learned conditions. A payload that does not decode is returned
// unchanged too, unless it claims a conditions key — that one is refused
// (ErrInvalidData) rather than relayed, because passing it on is the leak
// this function exists to prevent.
func deliveredSightPayload(payload []byte) ([]byte, error) {
	testimony, ok := DecodeSightTestimony(payload)
	if !ok {
		var fields map[string]json.RawMessage
		if json.Unmarshal(payload, &fields) == nil {
			if _, claims := fields["conditions"]; claims {
				return nil, fmt.Errorf("undecodable sight testimony carries conditions: %w", ErrInvalidData)
			}
		}
		return append([]byte(nil), payload...), nil
	}
	if testimony.Conditions == nil {
		return append([]byte(nil), payload...), nil
	}
	testimony.Conditions = nil
	out, err := EncodeSightTestimony(testimony)
	if err != nil {
		return nil, fmt.Errorf("deliver sight testimony: %w", err)
	}
	return out, nil
}

// deliveredHoldings returns holdings with every sight payload in its delivered
// form; see [deliveredSightPayload]. Other channels pass through.
func deliveredHoldings(holdings []perception.Holding) ([]perception.Holding, error) {
	for i := range holdings {
		if holdings[i].Channel != perception.Sight {
			continue
		}
		payload, err := deliveredSightPayload(holdings[i].Payload)
		if err != nil {
			return nil, fmt.Errorf("sighting of %q: %w", holdings[i].Subject, err)
		}
		holdings[i].Payload = payload
	}
	return holdings, nil
}

// DecodeSightTestimony decodes canonical tagged sight testimony. It rejects
// malformed, contradictory, unknown-field, untagged, and trailing JSON rather
// than inventing testimony: a payload with no state is not read as a known
// location.
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

	// The wire struct cannot distinguish an omitted key from a zero value, so
	// inspect the original object for which keys are present.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return SightTestimony{}, false
	}
	_, xPresent := fields["x"]
	_, yPresent := fields["y"]
	_, downPresent := fields["down"]
	_, equipmentPresent := fields["equipment"]
	_, blocksPresent := fields["blocks_movement"]
	_, conditionsPresent := fields["conditions"]
	_, appearancePresent := fields["appearance_ref"]
	_, activityPresent := fields["default_activity_ref"]
	_, facingPresent := fields["facing"]
	_, headingPresent := fields["heading"]
	// A present key must hold a bearing: "facing": null would decode as not
	// observed while claiming the key, and a value outside [0, 360) is a
	// facing no build writes.
	if facingPresent && (wire.Facing == nil || validateFacing(wire.Facing) != nil) {
		return SightTestimony{}, false
	}
	// The heading is a bearing on Facing's rule, for Facing's reasons.
	if headingPresent && (wire.Heading == nil || validateFacing(wire.Heading) != nil) {
		return SightTestimony{}, false
	}
	if appearancePresent && (wire.AppearanceRef == "" || validateAppearanceRef(wire.AppearanceRef) != nil) {
		return SightTestimony{}, false
	}
	if activityPresent && (wire.DefaultActivityRef == "" || validateDefaultActivityRef(wire.DefaultActivityRef) != nil) {
		return SightTestimony{}, false
	}

	var hands *HeldEquipment
	if wire.Equipment != nil {
		hands = &HeldEquipment{MainHand: wire.Equipment.MainHand, OffHand: wire.Equipment.OffHand}
	}

	// A present key must hold a list: "conditions": null would decode as
	// not observed while claiming the key, which is two answers in one.
	if conditionsPresent && wire.Conditions == nil {
		return SightTestimony{}, false
	}
	var held *ConditionSet
	if wire.Conditions != nil {
		held = &ConditionSet{Conditions: append(make([]ConditionKey, 0, len(*wire.Conditions)), *wire.Conditions...)}
		if validateConditionSet(held) != nil {
			return SightTestimony{}, false
		}
	}

	switch LocationState(wire.State) {
	case LocationKnown, LocationLost:
		if wire.X == nil || wire.Y == nil {
			return SightTestimony{}, false
		}
		return SightTestimony{
			State:              LocationState(wire.State),
			Position:           spatial.Position{X: *wire.X, Y: *wire.Y},
			Down:               wire.Down,
			BlocksMovement:     wire.BlocksMovement,
			Equipment:          hands,
			Conditions:         held,
			AppearanceRef:      wire.AppearanceRef,
			DefaultActivityRef: wire.DefaultActivityRef,
			Facing:             wire.Facing,
			Heading:            wire.Heading,
		}, true
	case LocationUnknown:
		// Nobody in view has a position, a standing, hands or conditions to
		// observe.
		if xPresent || yPresent || downPresent || equipmentPresent || blocksPresent || conditionsPresent || appearancePresent ||
			facingPresent || headingPresent || activityPresent {
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
