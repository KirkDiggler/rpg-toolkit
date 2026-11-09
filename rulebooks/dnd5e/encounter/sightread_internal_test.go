// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// sightread_internal_test.go pins TESTIMONY IS DECODED ONCE (rpg-project#879
// S9, F17): a held sighting is decoded on its first read and the decoded form
// answers after that while the stored bytes are unchanged; a write is read
// back decoded, never the testimony it replaced. And the codec reads a
// payload's keys once: the keys its strict walk records are the keys the
// object carries.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// guardsSightingOfAlice is the guard's held sighting of alice, read through
// [Encounter.heldSight], and the stored holding it was read from.
func guardsSightingOfAlice(t *testing.T, enc *Encounter) (SightTestimony, perception.Holding) {
	t.Helper()
	guard := sightMember(postedGuard)
	held, err := enc.intelLog.Held(guard)
	require.NoError(t, err)
	for _, h := range held {
		if h.Channel != perception.Sight || h.Subject != sightMember("alice") {
			continue
		}
		seen, ok := enc.heldSight(guard, h)
		require.True(t, ok, "the guard's sighting of alice decodes")
		decoded, ok := DecodeSightTestimony(h.Payload)
		require.True(t, ok)
		require.Equal(t, decoded, seen, "the held read is the decode of the stored bytes")

		return seen, h
	}
	t.Fatal("precondition: the guard holds a sighting of alice")

	return SightTestimony{}, perception.Holding{}
}

// seeingGuard is the hold hall with the gate open, alice on the south side
// and a posted guard beside her, so he sees her and every step she takes.
func seeingGuard(t *testing.T) *Encounter {
	t.Helper()

	return holdHall(t, holdLeafDoor(DoorIsOpen()), holdSouthEast,
		holder(t, postedGuard, holdSouthWest, holdLeaf, holdSouthWest))
}

func TestHeldTestimonyIsReadBackAfterAWrite(t *testing.T) {
	enc := seeingGuard(t)
	before, _ := guardsSightingOfAlice(t, enc)
	require.Equal(t, holdSouthEast, before.Position, "precondition: he saw her where she stands")

	steps(t, enc, holdBeside, "precondition: alice steps where he still sees her")
	after, _ := guardsSightingOfAlice(t, enc)
	require.Equal(t, holdBeside, after.Position,
		"the pass rewrote his sighting, and the read is the new testimony, never the one it replaced")
}

func TestHeldTestimonyIsDecodedOnce(t *testing.T) {
	enc := seeingGuard(t)
	_, h := guardsSightingOfAlice(t, enc)

	// A sentinel in the slot is what a second read of the same bytes
	// returns: the payload is not decoded again.
	slot := sightSlot{observer: sightMember(postedGuard), subject: h.Subject, channel: h.Channel}
	read, ok := enc.sightReads[slot]
	require.True(t, ok, "precondition: the first read filled the slot")
	read.testimony.Position = spatial.Position{X: 99, Y: 99}
	enc.sightReads[slot] = read

	again, ok := enc.heldSight(sightMember(postedGuard), h)
	require.True(t, ok)
	require.Equal(t, spatial.Position{X: 99, Y: 99}, again.Position, "the unchanged payload is answered from the slot")
}

func TestStrictSightFieldsAreTheObjectsKeys(t *testing.T) {
	down, facing, heading := true, 90.0, 45.0
	encoded := func(testimony SightTestimony) string {
		raw, err := EncodeSightTestimony(testimony)
		require.NoError(t, err)
		return string(raw)
	}
	for name, payload := range map[string]string{
		"every fact": encoded(SightTestimony{
			State: LocationKnown, Position: spatial.Position{X: 3, Y: 4}, Down: &down, BlocksMovement: &down,
			HoldsDoor: "gate", Equipment: &HeldEquipment{MainHand: "sword"},
			Conditions: &ConditionSet{Conditions: []ConditionKey{}}, AppearanceRef: appearanceMerchant,
			DefaultActivityRef: activityThoughtful, Facing: &facing, Heading: &heading,
		}),
		"lost":                          encoded(SightTestimony{State: LocationLost, Position: spatial.Position{X: 1, Y: 2}}),
		"unknown":                       encoded(SightTestimony{State: LocationUnknown}),
		"an older build, place only":    `{"state":"known","x":1,"y":2}`,
		"an older build, no facing":     `{"state":"known","x":1,"y":2,"down":false,"equipment":{}}`,
		"untagged":                      `{"x":1,"y":2}`,
		"missing a coordinate":          `{"state":"known","x":1}`,
		"null facing":                   `{"state":"known","x":1,"y":2,"facing":null}`,
		"null conditions":               `{"state":"known","x":1,"y":2,"conditions":null}`,
		"empty hold":                    `{"state":"known","x":1,"y":2,"holds_door":""}`,
		"unknown carrying a place":      `{"state":"unknown","x":1,"y":2}`,
		"an empty object":               `{}`,
		"whitespace around the keys":    " { \"state\" : \"known\" , \"x\" : 1 , \"y\" : 2 } ",
		"a key this build does not say": `{"state":"known","x":1,"y":2,"smell":"bad"}`,
		"a key in the wrong case":       `{"State":"known","x":1,"y":2}`,
		"a key twice":                   `{"state":"known","x":1,"x":1,"y":2}`,
		"trailing JSON":                 `{"state":"known","x":1,"y":2}{}`,
		"an array":                      `[1,2]`,
		"not JSON":                      `state=known`,
	} {
		t.Run(name, func(t *testing.T) {
			present, strict := strictSightFields([]byte(payload))
			if !strict {
				_, decodes := DecodeSightTestimony([]byte(payload))
				require.False(t, decodes, "a payload the strict walk refuses never decodes")
				return
			}
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(payload), &fields), "a strict payload parses as an object")
			for key, field := range sightPayloadFields {
				_, parsed := fields[key]
				require.Equal(t, parsed, present.has(field), "key %q: the walk's answer is the parse's", key)
			}
		})
	}

	// The refusals the walk owns, named, so a walk that waved them through
	// fails here and not only in the decode.
	for _, payload := range []string{
		`{"state":"known","x":1,"y":2,"smell":"bad"}`, `{"State":"known","x":1,"y":2}`,
		`{"state":"known","x":1,"x":1,"y":2}`, `{"state":"known","x":1,"y":2}{}`, `[1,2]`, `state=known`,
	} {
		_, strict := strictSightFields([]byte(payload))
		require.False(t, strict, "%s is refused by the walk", payload)
	}
}

// TestPresenceStillDecidesTheDecode holds the decode's answers that turn on
// a key's presence rather than its value, now read from the walk.
func TestPresenceStillDecidesTheDecode(t *testing.T) {
	for payload, want := range map[string]bool{
		`{"state":"known","x":1,"y":2}`:                     true,
		`{"state":"known","x":1,"y":2,"facing":null}`:       false,
		`{"state":"known","x":1,"y":2,"heading":null}`:      false,
		`{"state":"known","x":1,"y":2,"conditions":null}`:   false,
		`{"state":"known","x":1,"y":2,"holds_door":""}`:     false,
		`{"state":"known","x":1,"y":2,"appearance_ref":""}`: false,
		`{"state":"unknown"}`:                               true,
		`{"state":"unknown","x":0}`:                         false,
		`{"state":"unknown","down":null}`:                   false,
		`{"state":"unknown","equipment":null}`:              false,
	} {
		_, ok := DecodeSightTestimony([]byte(payload))
		require.Equal(t, want, ok, "%s", payload)
	}
}
