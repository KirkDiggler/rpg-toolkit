// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// conditions_test.go covers rpg-project#520 R16: a sighting carries every
// condition on the sighted member, snapshotted as testimony at sight refresh and
// never read live from the sheet.

const (
	faerieFire   = "dnd5e:conditions:faerie_fire"
	guidingBolt  = "dnd5e:conditions:guiding_bolt"
	clericSource = "cleric"
)

// conditionTable is an Equipment-with-Conditions capability answering from a
// table that a test can change MID-SCENE. A member absent from the table is
// answered for with nil: nothing to observe. Hands are never observed.
type conditionTable struct {
	held  map[encounter.MemberID]*encounter.ConditionSet
	calls int
}

func (c *conditionTable) Equipment(members []encounter.MemberID) (map[encounter.MemberID]*encounter.HeldEquipment, error) {
	return noHandsAreObserved{}.Equipment(members)
}

func (c *conditionTable) Conditions(members []encounter.MemberID) (map[encounter.MemberID]*encounter.ConditionSet, error) {
	c.calls++
	out := make(map[encounter.MemberID]*encounter.ConditionSet, len(members))
	for _, id := range members {
		out[id] = c.held[id]
	}
	return out, nil
}

// conditionsSkipping omits one member from its answer.
type conditionsSkipping struct {
	noHandsAreObserved
	skip encounter.MemberID
}

func (c conditionsSkipping) Conditions(members []encounter.MemberID) (map[encounter.MemberID]*encounter.ConditionSet, error) {
	out := noConditionsObserved(members)
	delete(out, c.skip)
	return out, nil
}

// conditionsForAStranger names somebody who is not a member.
type conditionsForAStranger struct{ noHandsAreObserved }

func (conditionsForAStranger) Conditions(members []encounter.MemberID) (map[encounter.MemberID]*encounter.ConditionSet, error) {
	out := noConditionsObserved(members)
	out["nobody-here"] = nil
	return out, nil
}

// handsOnly is an Equipment that does not answer Conditions.
type handsOnly struct{}

func (handsOnly) Equipment(members []encounter.MemberID) (map[encounter.MemberID]*encounter.HeldEquipment, error) {
	return noHandsAreObserved{}.Equipment(members)
}

// heldSet builds an observed set. Its list is never nil, matching what decode
// yields for an observed empty set.
func heldSet(conditions ...encounter.ConditionKey) *encounter.ConditionSet {
	return &encounter.ConditionSet{Conditions: append([]encounter.ConditionKey{}, conditions...)}
}

type sightingConditionsSuite struct {
	suite.Suite
	table *conditionTable
	sight *sightList
	enc   *encounter.Encounter
}

func TestSightingConditionsSuite(t *testing.T) {
	suite.Run(t, new(sightingConditionsSuite))
}

func (s *sightingConditionsSuite) SetupTest() {
	s.table = &conditionTable{held: map[encounter.MemberID]*encounter.ConditionSet{
		goblin: heldSet(encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource}),
		bob:    heldSet(),
	}}
	s.sight = &sightList{fallback: unlimitedSight}
	enc, err := encounter.NewEncounter(s.setup(s.table))
	s.Require().NoError(err)
	s.enc = enc
}

func (s *sightingConditionsSuite) SetupSubTest() { s.SetupTest() }

func (s *sightingConditionsSuite) setup(equipment encounter.Equipment) *encounter.SetupInput {
	return &encounter.SetupInput{
		Sight: s.sight, Equipment: equipment, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 1)},
		},
		Members: []encounter.MemberInput{
			{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(4, 0)},
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(2, 0)},
			{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
			{ID: bob, Kind: encounter.KindPlayer, Position: cellAt(1, 0)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	}
}

func (s *sightingConditionsSuite) loadInput(data encounter.EncounterData, equipment encounter.Equipment) *encounter.LoadEncounterInput {
	return &encounter.LoadEncounterInput{
		Data: data, Sight: s.sight, Equipment: equipment, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
	}
}

// observed returns alice's observed-context entry for id, and whether there is
// one at all.
func (s *sightingConditionsSuite) observed(enc *encounter.Encounter, id encounter.MemberID) (encounter.ObservedContextMember, bool) {
	out, err := enc.ObservedContext(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	for _, m := range out.Members {
		if m.ID == id {
			return m, true
		}
	}
	return encounter.ObservedContextMember{}, false
}

func (s *sightingConditionsSuite) recheck(id encounter.MemberID) {
	_, err := s.enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{id}})
	s.Require().NoError(err)
}

func (s *sightingConditionsSuite) TestSightingSnapshotsConditions() {
	member, ok := s.observed(s.enc, goblin)
	s.Require().True(ok, "alice sees the goblin")
	s.Equal(heldSet(encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource}), member.Conditions)
}

func (s *sightingConditionsSuite) TestUnknownAndObservedEmptyStayDistinct() {
	vendor, ok := s.observed(s.enc, "vendor")
	s.Require().True(ok)
	s.Nil(vendor.Conditions, "nothing to observe is not seen holding none")

	b, ok := s.observed(s.enc, bob)
	s.Require().True(ok)
	s.Require().NotNil(b.Conditions, "seen holding none is an observation")
	s.Empty(b.Conditions.Conditions)

	s.Run("and across a save and load", func() {
		raw, err := json.Marshal(s.enc.ToData())
		s.Require().NoError(err)
		var data encounter.EncounterData
		s.Require().NoError(json.Unmarshal(raw, &data))
		// A capability that would now answer differently proves Load reads the
		// saved testimony rather than re-asking.
		s.table.held = map[encounter.MemberID]*encounter.ConditionSet{}
		loaded, err := encounter.LoadEncounter(s.loadInput(data, s.table))
		s.Require().NoError(err)

		vendor, ok := s.observed(loaded, "vendor")
		s.Require().True(ok)
		s.Nil(vendor.Conditions)
		b, ok := s.observed(loaded, bob)
		s.Require().True(ok)
		s.Require().NotNil(b.Conditions)
		s.Empty(b.Conditions.Conditions)
		g, ok := s.observed(loaded, goblin)
		s.Require().True(ok)
		s.Equal(heldSet(encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource}), g.Conditions)
	})
}

func (s *sightingConditionsSuite) TestSightingIsTestimonyNotALiveRead() {
	// Change the truth twice without a refresh: once by replacing the answer
	// and once by editing the very set the capability handed over.
	s.table.held[goblin].Conditions[0].ConditionRef = guidingBolt
	s.table.held[bob] = heldSet(encounter.ConditionKey{ConditionRef: guidingBolt, SourceID: clericSource})

	g, _ := s.observed(s.enc, goblin)
	s.Equal(heldSet(encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource}), g.Conditions,
		"the snapshot neither re-reads nor aliases the sheet")
	b, _ := s.observed(s.enc, bob)
	s.Require().NotNil(b.Conditions)
	s.Empty(b.Conditions.Conditions)

	s.recheck(bob)
	b, _ = s.observed(s.enc, bob)
	s.Equal(heldSet(encounter.ConditionKey{ConditionRef: guidingBolt, SourceID: clericSource}), b.Conditions)
}

func (s *sightingConditionsSuite) TestRecheckPicksUpAddedAndRemovedConditions() {
	s.table.held[goblin] = heldSet(
		encounter.ConditionKey{ConditionRef: guidingBolt, SourceID: clericSource},
		encounter.ConditionKey{ConditionRef: "dnd5e:conditions:prone"},
	)
	s.recheck(goblin)

	g, ok := s.observed(s.enc, goblin)
	s.Require().True(ok)
	s.Equal(heldSet(
		encounter.ConditionKey{ConditionRef: guidingBolt, SourceID: clericSource},
		encounter.ConditionKey{ConditionRef: "dnd5e:conditions:prone"},
	), g.Conditions, "faerie fire gone, two added, in the order reported")

	s.table.held[goblin] = heldSet()
	s.recheck(goblin)
	g, _ = s.observed(s.enc, goblin)
	s.Require().NotNil(g.Conditions)
	s.Empty(g.Conditions.Conditions, "the last condition removed is observed none, not unknown")
}

func (s *sightingConditionsSuite) TestConditionsAskedOncePerRefresh() {
	before := s.table.calls
	s.recheck(goblin)
	s.Equal(before+1, s.table.calls, "one question for the whole roster, whatever the observer count")
}

func (s *sightingConditionsSuite) TestUnsightedMemberExposesNothing() {
	// Alice's light shrinks to her own cell, then the goblin's truth changes.
	s.sight.reach = map[encounter.MemberID]int{alice: 0}
	s.recheck(alice)
	s.table.held[goblin] = heldSet(encounter.ConditionKey{ConditionRef: guidingBolt, SourceID: clericSource})
	s.recheck(goblin)

	_, ok := s.observed(s.enc, goblin)
	s.False(ok, "an unsighted member is not in the observed context")

	// What alice keeps, as stored, is a memory of what she saw, never the new
	// truth.
	h := s.enc.ToData().Perception.Intel.Holdings["member|alice"]["member|goblin"]
	s.Empty(h.CurrentVia, "a memory, not a current sighting")
	memory, valid := encounter.DecodeSightTestimony(h.Payload)
	s.Require().True(valid)
	s.Equal(heldSet(encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource}), memory.Conditions)

	// Bob still sees the goblin, and sees the new truth.
	bobView, err := s.enc.ObservedContext(&encounter.ViewInput{Member: bob})
	s.Require().NoError(err)
	found := false
	for _, m := range bobView.Members {
		if m.ID == goblin {
			found = true
			s.Equal(heldSet(encounter.ConditionKey{ConditionRef: guidingBolt, SourceID: clericSource}), m.Conditions)
		}
	}
	s.True(found)
}

func (s *sightingConditionsSuite) TestLegacyTestimonyLoadsAsNotObserved() {
	data := s.enc.ToData()
	holding := data.Perception.Intel.Holdings["member|alice"]["member|goblin"]
	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State: encounter.LocationKnown, Position: cellAt(2, 0),
	})
	s.Require().NoError(err)
	s.NotContains(string(payload), "conditions")
	holding.Payload = payload
	data.Perception.Intel.Holdings["member|alice"]["member|goblin"] = holding

	loaded, err := encounter.LoadEncounter(s.loadInput(data, s.table))
	s.Require().NoError(err)
	g, ok := s.observed(loaded, goblin)
	s.Require().True(ok)
	s.Nil(g.Conditions, "testimony from before conditions existed says they were not observed")
}

func (s *sightingConditionsSuite) TestNewEncounterRefusesEquipmentWithoutConditions() {
	enc, err := encounter.NewEncounter(s.setup(handsOnly{}))
	s.Require().ErrorIs(err, encounter.ErrNoConditions)
	s.Nil(enc)
}

func (s *sightingConditionsSuite) TestLoadRefusesEquipmentWithoutConditions() {
	enc, err := encounter.LoadEncounter(s.loadInput(s.enc.ToData(), handsOnly{}))
	s.Require().ErrorIs(err, encounter.ErrNoConditions)
	s.Nil(enc)
}

func (s *sightingConditionsSuite) TestConditionsNowRefusesStrangerAndSkippedMember() {
	s.Run("stranger", func() {
		_, err := encounter.NewEncounter(s.setup(conditionsForAStranger{}))
		s.Require().ErrorIs(err, encounter.ErrNotMember)
	})
	s.Run("skipped", func() {
		_, err := encounter.NewEncounter(s.setup(conditionsSkipping{skip: goblin}))
		s.Require().ErrorIs(err, encounter.ErrNoConditions)
	})
}

func (s *sightingConditionsSuite) TestConditionsNowRefusesMalformedEntries() {
	cases := map[string]*encounter.ConditionSet{
		"empty ref": heldSet(encounter.ConditionKey{SourceID: clericSource}),
		"repeated": heldSet(
			encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource},
			encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource},
		),
	}
	for name, set := range cases {
		s.Run(name, func() {
			table := &conditionTable{held: map[encounter.MemberID]*encounter.ConditionSet{goblin: set}}
			_, err := encounter.NewEncounter(s.setup(table))
			s.Require().ErrorIs(err, encounter.ErrInvalidData)
		})
	}
	s.Run("same ref from two sources is two conditions", func() {
		table := &conditionTable{held: map[encounter.MemberID]*encounter.ConditionSet{goblin: heldSet(
			encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource},
			encounter.ConditionKey{ConditionRef: faerieFire, SourceID: "druid"},
		)}}
		_, err := encounter.NewEncounter(s.setup(table))
		s.Require().NoError(err)
	})
}

func TestSightTestimonyConditionsRoundTrip(t *testing.T) {
	for name, set := range map[string]*encounter.ConditionSet{
		"not observed": nil,
		"seen none":    heldSet(),
		"seen two": heldSet(
			encounter.ConditionKey{ConditionRef: guidingBolt, SourceID: clericSource},
			encounter.ConditionKey{ConditionRef: faerieFire},
		),
	} {
		t.Run(name, func(t *testing.T) {
			payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
				State: encounter.LocationKnown, Position: spatial.Position{X: 1, Y: 2}, Conditions: set,
			})
			require.NoError(t, err)

			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(payload, &fields))
			raw, present := fields["conditions"]
			switch {
			case set == nil:
				require.False(t, present, "not observed omits the key")
			case len(set.Conditions) == 0:
				require.JSONEq(t, `[]`, string(raw), "seen none encodes an empty list")
			default:
				require.JSONEq(t, `[{"condition_ref":"`+guidingBolt+`","source_id":"cleric"},{"condition_ref":"`+faerieFire+`","source_id":""}]`, string(raw))
			}

			got, ok := encounter.DecodeSightTestimony(payload)
			require.True(t, ok)
			require.Equal(t, set, got.Conditions, "order and the nil/empty distinction survive")
		})
	}
}

func TestSightTestimonyRefusesConditionsOnUnknownAndLegacy(t *testing.T) {
	_, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State: encounter.LocationUnknown, Conditions: heldSet(),
	})
	require.Error(t, err, "unknown location cannot carry conditions")

	for name, payload := range map[string]string{
		"unknown carrying conditions":         `{"state":"unknown","conditions":[]}`,
		"legacy untagged carrying conditions": `{"x":1,"y":2,"conditions":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, ok := encounter.DecodeSightTestimony([]byte(payload))
			require.False(t, ok)
		})
	}
}

func TestDecodeRefusesMalformedConditions(t *testing.T) {
	for name, conditions := range map[string]string{
		"empty ref":     `[{"condition_ref":"","source_id":"cleric"}]`,
		"missing ref":   `[{"source_id":"cleric"}]`,
		"repeated":      `[{"condition_ref":"a","source_id":"x"},{"condition_ref":"a","source_id":"x"}]`,
		"null":          `null`,
		"unknown field": `[{"condition_ref":"a","state":{}}]`,
		"not a list":    `{"condition_ref":"a"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, ok := encounter.DecodeSightTestimony([]byte(`{"state":"known","x":1,"y":2,"conditions":` + conditions + `}`))
			require.False(t, ok)
		})
	}

	_, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State: encounter.LocationKnown, Conditions: heldSet(encounter.ConditionKey{SourceID: "x"}),
	})
	require.Error(t, err, "encode validates the same way")
}

// storedSightOf decodes the testimony observer holds about subject as it is
// STORED — the persisted form, conditions included.
func (s *sightingConditionsSuite) storedSightOf(enc *encounter.Encounter, observer, subject encounter.MemberID) encounter.SightTestimony {
	h, ok := enc.ToData().Perception.Intel.Holdings[core.EntityID("member|"+observer)][intel.Subject("member|"+subject)]
	s.Require().True(ok, "%s holds no stored sighting of %s", observer, subject)
	got, valid := encounter.DecodeSightTestimony(h.Payload)
	s.Require().True(valid)
	return got
}

// viewPayloadOf is the sight payload View delivers to observer about subject.
func (s *sightingConditionsSuite) viewPayloadOf(enc *encounter.Encounter, observer, subject encounter.MemberID) []byte {
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range holdings {
		if h.Channel == perception.Sight && h.Subject == subject {
			return h.Payload
		}
	}
	s.FailNow("no delivered sighting", "%s of %s", observer, subject)
	return nil
}

// requireNoConditionsKey fails when a delivered payload names conditions at
// all — an empty list is a leak of "seen holding none" as much as a full one.
func (s *sightingConditionsSuite) requireNoConditionsKey(payload []byte) {
	var fields map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(payload, &fields))
	s.NotContains(fields, "conditions", "a delivered payload never carries seen conditions: %s", payload)
}

func (s *sightingConditionsSuite) TestDeliveredPayloadsCarryNoConditions() {
	s.Run("View", func() {
		s.requireNoConditionsKey(s.viewPayloadOf(s.enc, alice, goblin))
		s.requireNoConditionsKey(s.viewPayloadOf(s.enc, alice, bob))
		s.NotNil(s.storedSightOf(s.enc, alice, goblin).Conditions, "while the stored testimony keeps them")
	})
	s.Run("first contact", func() {
		joined, err := s.enc.Join(&encounter.JoinInput{Member: "carol", Kind: encounter.KindPlayer, Cell: cellAt(3, 0)})
		s.Require().NoError(err)
		delta := joined.IntelDeltas["carol"]
		s.Require().NotNil(delta)
		found := false
		for _, presence := range delta.FirstContact {
			s.requireNoConditionsKey(presence.Payload)
			if presence.ID == goblin {
				found = true
			}
		}
		s.True(found, "carol's first contact includes the goblin")
		s.Equal(heldSet(encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource}),
			s.storedSightOf(s.enc, "carol", goblin).Conditions)
	})
	s.Run("exit carry", func() {
		exited, err := s.enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().NoError(err)
		found := false
		for _, h := range exited.Carry {
			if h.Channel != perception.Sight {
				continue
			}
			s.requireNoConditionsKey(h.Payload)
			found = found || h.Subject == goblin
		}
		s.True(found, "alice carries her sighting of the goblin out")
	})
	s.Run("the delivered bytes are the testimony without conditions", func() {
		stored := s.storedSightOf(s.enc, alice, goblin)
		stored.Conditions = nil
		want, err := encounter.EncodeSightTestimony(stored)
		s.Require().NoError(err)
		s.Equal(string(want), string(s.viewPayloadOf(s.enc, alice, goblin)))
	})
}

// TestAConditionsOnlyChangeStillTellsWatchers is the R19 consequence of keeping
// conditions off the delivered payload: a Recheck after only a condition
// changed delivers byte-identical sighting bytes, and the watcher is still told
// "changed" — the beat is driven by the declaration, not by the payload diff —
// while the stored testimony and ObservedContext carry the new set.
func (s *sightingConditionsSuite) TestAConditionsOnlyChangeStillTellsWatchers() {
	before := s.viewPayloadOf(s.enc, alice, goblin)
	story, err := s.enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	seen := len(story)

	s.table.held[goblin] = heldSet(encounter.ConditionKey{ConditionRef: guidingBolt, SourceID: clericSource})
	s.recheck(goblin)

	s.Equal(string(before), string(s.viewPayloadOf(s.enc, alice, goblin)), "delivery cannot see the change")
	g, _ := s.observed(s.enc, goblin)
	s.Equal(heldSet(encounter.ConditionKey{ConditionRef: guidingBolt, SourceID: clericSource}), g.Conditions)

	story, err = s.enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	var changed []string
	for _, entry := range story[seen:] {
		var beat struct {
			Beat    string   `json:"beat"`
			Changed []string `json:"changed"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Beat == encounter.BeatSighted {
			changed = append(changed, beat.Changed...)
			s.NotContains(string(entry.Payload), "conditions", "the beat names who, never what")
		}
	}
	s.Equal([]string{string(goblin)}, changed)
}
