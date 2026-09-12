// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// equipment_test.go covers rpg-toolkit#1615: what a member is holding becomes
// per-observer testimony, snapshotted when they were seen, so that it can later
// be WRONG — which is what illusion and enchantment need and what a live read of
// the sheet could never provide.

// handsFrom answers from a fixed table. A member absent from the table is
// answered for with nil: no hands to observe.
type handsFrom map[encounter.MemberID]*encounter.HeldEquipment

func (h handsFrom) Equipment(
	members []encounter.MemberID,
) (map[encounter.MemberID]*encounter.HeldEquipment, error) {
	out := make(map[encounter.MemberID]*encounter.HeldEquipment, len(members))
	for _, id := range members {
		out[id] = h[id]
	}
	return out, nil
}

// handsSkippingWhenTold omits one member from its answer, which is the defect
// the composition must refuse rather than paper over with empty hands.
type handsSkippingWhenTold struct{ skip encounter.MemberID }

func (h handsSkippingWhenTold) Equipment(
	members []encounter.MemberID,
) (map[encounter.MemberID]*encounter.HeldEquipment, error) {
	out := make(map[encounter.MemberID]*encounter.HeldEquipment, len(members))
	for _, id := range members {
		if id == h.skip {
			continue
		}
		out[id] = nil
	}
	return out, nil
}

// handsForAStranger names somebody who is not a member.
type handsForAStranger struct{}

func (handsForAStranger) Equipment(
	members []encounter.MemberID,
) (map[encounter.MemberID]*encounter.HeldEquipment, error) {
	out := make(map[encounter.MemberID]*encounter.HeldEquipment, len(members)+1)
	for _, id := range members {
		out[id] = nil
	}
	out["nobody-here"] = nil
	return out, nil
}

func TestTheTestimonyCarriesWhatTheSubjectWasHolding(t *testing.T) {
	hands := handsFrom{
		"zara":  {MainHand: "longsword", OffHand: "shield"},
		"alice": {MainHand: "shortbow"},
	}
	enc, err := encounter.NewEncounter(equipmentSetup(hands,
		encounter.MemberInput{ID: "zara", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}, SpeedFeet: 30},
		encounter.MemberInput{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 0}, SpeedFeet: 30},
	))
	require.NoError(t, err)

	seen := heldSightOf(t, enc, "zara", "alice")
	require.NotNil(t, seen.Equipment, "zara watched alice, so alice's hands were observed")
	require.Equal(t, "shortbow", seen.Equipment.MainHand)
	require.Equal(t, "", seen.Equipment.OffHand, "an empty off hand is observed, not unknown")
}

// A subject with no sheet has no hands to observe, and that is a different
// claim from seeing empty hands. Collapsing them is the bug the pointer exists
// to prevent.
func TestNoHandsToObserveIsNotEmptyHands(t *testing.T) {
	hands := handsFrom{
		"zara": {MainHand: "longsword"},
		// the skeleton is deliberately absent: no sheet, no hands
	}
	enc, err := encounter.NewEncounter(equipmentSetup(hands,
		encounter.MemberInput{ID: "zara", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}, SpeedFeet: 30},
		encounter.MemberInput{ID: "skeleton", Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 0}, SpeedFeet: 30},
	))
	require.NoError(t, err)

	seen := heldSightOf(t, enc, "zara", "skeleton")
	require.Nil(t, seen.Equipment, "a skeleton has no hands to observe; it was not seen empty-handed")

	empty := encounter.SightTestimony{
		State:     encounter.LocationKnown,
		Position:  spatial.Position{X: 1, Y: 0},
		Equipment: &encounter.HeldEquipment{},
	}
	payload, err := encounter.EncodeSightTestimony(empty)
	require.NoError(t, err)
	back, ok := encounter.DecodeSightTestimony(payload)
	require.True(t, ok)
	require.NotNil(t, back.Equipment, "observed-empty must survive the wire as observed, not as unknown")
}

func TestAMemberTheAnswerSkippedIsRefused(t *testing.T) {
	_, err := encounter.NewEncounter(equipmentSetup(handsSkippingWhenTold{skip: "alice"},
		encounter.MemberInput{ID: "zara", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}, SpeedFeet: 30},
		encounter.MemberInput{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 0}, SpeedFeet: 30},
	))
	require.ErrorIs(t, err, encounter.ErrNoEquipment)
}

func TestAStrangerInTheAnswerIsRefused(t *testing.T) {
	_, err := encounter.NewEncounter(equipmentSetup(handsForAStranger{},
		encounter.MemberInput{ID: "zara", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}, SpeedFeet: 30},
	))
	require.ErrorIs(t, err, encounter.ErrNotMember)
}

func TestAnEncounterWithNoEquipmentCapabilityIsRefusedAtTheDoor(t *testing.T) {
	setup := equipmentSetup(handsFrom{},
		encounter.MemberInput{ID: "zara", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}, SpeedFeet: 30},
	)
	setup.Equipment = nil
	_, err := encounter.NewEncounter(setup)
	require.ErrorIs(t, err, encounter.ErrNoEquipment)
}

// Unknown testimony means nobody is in view, so there is nothing to have been
// holding anything. Encoding hands onto it is a defect, not a value to trim.
func TestUnknownTestimonyCannotCarryHands(t *testing.T) {
	_, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State:     encounter.LocationUnknown,
		Equipment: &encounter.HeldEquipment{MainHand: "longsword"},
	})
	require.Error(t, err)

	_, ok := encounter.DecodeSightTestimony([]byte(`{"state":"unknown","equipment":{"main_hand":"longsword"}}`))
	require.False(t, ok, "unknown testimony carrying hands must be refused, not silently trimmed")
}

// Testimony written before hands existed decodes as "not observed" rather than
// as "empty-handed". The next sight refresh replaces it wholesale.
func TestOlderTestimonyDidNotObserveHands(t *testing.T) {
	got, ok := encounter.DecodeSightTestimony([]byte(`{"state":"known","x":4,"y":7}`))
	require.True(t, ok)
	require.Equal(t, spatial.Position{X: 4, Y: 7}, got.Position)
	require.Nil(t, got.Equipment, "an older build did not observe hands; it did not see empty ones")
	require.Nil(t, got.Down)
}

func equipmentSetup(hands encounter.Equipment, members ...encounter.MemberInput) *encounter.SetupInput {
	return &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Standing:   everyoneStanding{},
		Sight:      everyoneSeesTheWholeMap{},
		Equipment:  hands,
		TurnDriver: passDriver{},
		Striker:    passStriker{}, Mover: quietMover{},
		Announcer: quietAnnouncer{},
		Retention: encounter.RetentionUnbounded,
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("equipment-yard", 0, 0, 12, 12)},
		},
		Members: members,
		Endings: []encounter.EndingInput{
			{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
		},
	}
}

// heldSightOf returns the sight testimony observer currently holds about
// subject, decoded by the package that encoded it.
func heldSightOf(t *testing.T, enc *encounter.Encounter, observer, subject encounter.MemberID) encounter.SightTestimony {
	t.Helper()
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	require.NoError(t, err)
	for _, h := range holdings {
		if h.Channel != perception.Sight || h.Subject != subject {
			continue
		}
		got, ok := encounter.DecodeSightTestimony(h.Payload)
		require.True(t, ok, "the composition must decode its own testimony")
		return got
	}
	t.Fatalf("%q holds no sight testimony about %q", observer, subject)
	return encounter.SightTestimony{}
}
