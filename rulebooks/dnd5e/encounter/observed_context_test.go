// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type observedContextEquipment struct {
	hands handsFrom
	calls int
}

func (h *observedContextEquipment) Equipment(ids []encounter.MemberID) (map[encounter.MemberID]*encounter.HeldEquipment, error) {
	h.calls++
	return h.hands.Equipment(ids)
}

func (h *observedContextEquipment) Conditions(ids []encounter.MemberID) (map[encounter.MemberID]*encounter.ConditionSet, error) {
	return noConditionsObserved(ids), nil
}

type observedContextSuite struct {
	suite.Suite
	enc   *encounter.Encounter
	sight *countingSight
	life  *passageAssessment
	hands *observedContextEquipment
}

func TestObservedContextSuite(t *testing.T) {
	suite.Run(t, new(observedContextSuite))
}

func (s *observedContextSuite) SetupTest() {
	s.sight = &countingSight{}
	s.life = &passageAssessment{down: map[encounter.MemberID]bool{}}
	s.hands = &observedContextEquipment{hands: handsFrom{
		goblin: {MainHand: "scimitar"},
	}}
	s.enc = s.newEncounter(nil)
}

func (s *observedContextSuite) SetupSubTest() { s.SetupTest() }

func (s *observedContextSuite) newEncounter(dispositions []encounter.DispositionInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: s.sight, Equipment: s.hands, Standing: s.life,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 1)},
			Dispositions: dispositions,
		},
		Members: []encounter.MemberInput{
			{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(4, 0)},
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(2, 0)},
			{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
			{ID: bob, Kind: encounter.KindPlayer, Position: cellAt(1, 0)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

func (s *observedContextSuite) load(data encounter.EncounterData) *encounter.Encounter {
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: s.sight, Equipment: s.hands, Standing: s.life,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
	return enc
}

func (s *observedContextSuite) read(enc *encounter.Encounter) *encounter.ObservedContextOutput {
	out, err := enc.ObservedContext(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	return out
}

func (s *observedContextSuite) member(out *encounter.ObservedContextOutput, id encounter.MemberID) encounter.ObservedContextMember {
	var found *encounter.ObservedContextMember
	for i := range out.Members {
		if out.Members[i].ID == id {
			found = &out.Members[i]
		}
	}
	s.Require().NotNil(found, "missing observed member %s", id)
	return *found
}

func (s *observedContextSuite) pair(out *encounter.ObservedContextOutput, from, to encounter.MemberID) encounter.ObservedContextPair {
	var found *encounter.ObservedContextPair
	for i := range out.Pairs {
		if out.Pairs[i].From == from && out.Pairs[i].To == to {
			found = &out.Pairs[i]
		}
	}
	s.Require().NotNil(found, "missing observed pair %s -> %s", from, to)
	return *found
}

func (s *observedContextSuite) TestCurrentMembersAndPairs() {
	out := s.read(s.enc)
	s.Equal(alice, out.Observer)
	s.Equal(cellAt(0, 0), out.Position)
	s.Require().Len(out.Members, 3)
	s.Equal([]encounter.MemberID{bob, goblin, "vendor"}, []encounter.MemberID{
		out.Members[0].ID, out.Members[1].ID, out.Members[2].ID,
	})
	s.Require().Len(out.Pairs, 12)
	var keys []string
	for _, pair := range out.Pairs {
		s.NotEqual(pair.From, pair.To)
		keys = append(keys, string(pair.From)+"/"+string(pair.To))
	}
	s.IsIncreasing(keys)
	s.Equal(float64(1), s.pair(out, goblin, bob).DistanceCells)
	s.Equal(float64(2), s.pair(out, alice, goblin).DistanceCells)
	s.Equal(encounter.StanceHostile, s.pair(out, goblin, bob).Stance)
	s.Equal(encounter.StanceNone, s.pair(out, alice, "vendor").Stance,
		"a member in no faction is a known no side, neither neutral nor unknown")
	s.Equal(encounter.StanceNone, s.pair(out, "vendor", goblin).Stance,
		"no side holds whichever end of the pair has no faction")
	s.Require().NotNil(s.member(out, goblin).Down)
	s.False(*s.member(out, goblin).Down, "observed false is a known fact")
}

func (s *observedContextSuite) TestCurrentPropsAndDoorsDoNotEnterTheMemberContext() {
	field := doorField(3, encounter.DoorIsOpen(), "gate", 1)
	field.Props = []encounter.PropInput{{
		// The prop deliberately shares the other member's bare ID, but not
		// its position. Subject-kind filtering must preserve that distinction.
		ID: string(bob), Ref: "test:props:chest", Holdable: true,
		At:             spatial.Position{X: 4, Y: 1},
		BlocksMovement: boolPtr(false), BlocksLineOfSight: boolPtr(false),
	}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: field, Sight: s.sight, Equipment: s.hands, Standing: s.life,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(2, 1)},
			{ID: bob, Kind: encounter.KindPlayer, Position: cellAt(3, 1)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// Prove the shared store actually contains current object testimony;
	// an empty or merely remembered prop/door would not exercise this seam.
	props, err := enc.PropSightings(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(props, 1)
	s.Require().NotNil(props[0].Prop)
	s.Equal(string(bob), props[0].Prop.ID)
	s.Contains(props[0].CurrentVia, perception.Sight)
	doors, err := enc.DoorSightings(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(doors, 1)
	s.Equal("gate", doors[0].Door.ID)
	s.Contains(doors[0].CurrentVia, perception.Sight)

	out := s.read(enc)
	s.Equal(alice, out.Observer)
	s.Equal(cellAt(2, 1), out.Position)
	s.Require().Len(out.Members, 1, "props and doors are not context members")
	s.Equal(bob, out.Members[0].ID)
	s.Equal(cellAt(3, 1), out.Members[0].Position, "the member, not the same-ID prop")
	s.Require().Len(out.Pairs, 2, "pairs range only over the observer and the other member")
	s.Equal(float64(1), s.pair(out, alice, bob).DistanceCells)
	s.Equal(float64(1), s.pair(out, bob, alice).DistanceCells)
}

func (s *observedContextSuite) TestKnownNeutralIsNotUnknown() {
	enc := s.newEncounter([]encounter.DispositionInput{{
		Between: [2]encounter.FactionID{encounter.FactionParty, encounter.FactionMonsters},
		Stance:  encounter.StanceNeutral,
	}})
	s.Equal(encounter.StanceNeutral, s.pair(s.read(enc), goblin, bob).Stance)
}

func (s *observedContextSuite) TestNoSideIsNeverAuthorable() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: s.sight, Equipment: s.hands, Standing: s.life,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 1)},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{encounter.FactionParty, encounter.FactionMonsters},
				Stance:  encounter.StanceNone,
			}},
		},
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.ErrorIs(err, encounter.ErrNoFaction, "no side describes a member, not a posture two sides can hold")
}

func (s *observedContextSuite) TestSnapshotFactsWinOverLiveState() {
	before := s.read(s.enc)
	sightCalls, lifeCalls, equipmentCalls := s.sight.calls, s.life.calls, s.hands.calls
	s.life.down[goblin] = true
	s.life.err = fmt.Errorf("information must not query live participation")
	s.hands.hands[goblin].MainHand = "greataxe"
	for range 2 {
		s.Equal(before, s.read(s.enc))
	}
	s.Equal(sightCalls, s.sight.calls)
	s.Equal(lifeCalls, s.life.calls)
	s.Equal(equipmentCalls, s.hands.calls)
}

func (s *observedContextSuite) TestObservedPositionCanDisagreeWithLivePlacement() {
	data := s.enc.ToData()
	holding := data.Perception.Intel.Holdings["member|alice"]["member|goblin"]
	seen, ok := encounter.DecodeSightTestimony(holding.Payload)
	s.Require().True(ok)
	seen.Position = cellAt(3, 0)
	payload, err := encounter.EncodeSightTestimony(seen)
	s.Require().NoError(err)
	holding.Payload = payload
	data.Perception.Intel.Holdings["member|alice"]["member|goblin"] = holding
	out := s.read(s.load(data))
	s.Equal(cellAt(3, 0), s.member(out, goblin).Position)
	s.Equal(float64(3), s.pair(out, alice, goblin).DistanceCells)
	s.Equal(float64(2), s.pair(out, goblin, bob).DistanceCells)
}

func (s *observedContextSuite) TestMemoryAndOtherChannelsAreExcluded() {
	for _, mode := range []string{"remembered", "unknown-memory", "other-channel"} {
		s.Run(mode, func() {
			data := s.enc.ToData()
			holding := data.Perception.Intel.Holdings["member|alice"]["member|goblin"]
			holding.CurrentVia = nil
			if mode == "unknown-memory" {
				payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationUnknown})
				s.Require().NoError(err)
				holding.Payload = payload
			}
			if mode == "other-channel" {
				holding.Channel = "hearsay"
				holding.CurrentVia = append(holding.CurrentVia, "hearsay")
				holding.Payload = []byte("opaque non-sight testimony")
			}
			data.Perception.Intel.Holdings["member|alice"]["member|goblin"] = holding
			before := s.read(s.load(data))
			s.Require().Len(before.Members, 2)
			s.Require().Len(before.Pairs, 6)
			for _, member := range before.Members {
				s.NotEqual(goblin, member.ID)
			}
			for _, pair := range before.Pairs {
				s.NotEqual(goblin, pair.From)
				s.NotEqual(goblin, pair.To)
			}
			// Move only the unobserved subject's truth, without changing testimony.
			for i := range data.Members {
				if data.Members[i].ID == goblin {
					data.Members[i].Cell = &encounter.PositionData{X: cellAt(5, 0).X, Y: cellAt(5, 0).Y}
				}
			}
			s.Equal(before, s.read(s.load(data)))
		})
	}
}

func (s *observedContextSuite) TestUnknownObservedFieldsStayUnknown() {
	data := s.enc.ToData()
	holding := data.Perception.Intel.Holdings["member|alice"]["member|goblin"]
	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State: encounter.LocationKnown, Position: cellAt(2, 0),
	})
	s.Require().NoError(err)
	holding.Payload = payload
	data.Perception.Intel.Holdings["member|alice"]["member|goblin"] = holding
	member := s.member(s.read(s.load(data)), goblin)
	s.Nil(member.Down)
	s.Nil(member.Equipment)
}

func (s *observedContextSuite) TestDetachedAndReadOnly() {
	beforeData, err := json.Marshal(s.enc.ToData())
	s.Require().NoError(err)
	before := s.read(s.enc)
	mutated := s.read(s.enc)
	mutated.Position = cellAt(5, 0)
	for i := range mutated.Members {
		if mutated.Members[i].ID == goblin {
			mutated.Members[i].Position = cellAt(5, 0)
			s.Require().NotNil(mutated.Members[i].Down)
			*mutated.Members[i].Down = true
			s.Require().NotNil(mutated.Members[i].Equipment)
			mutated.Members[i].Equipment.MainHand = "greataxe"
		}
	}
	mutated.Pairs[0].Stance = encounter.StanceNeutral
	mutated.Pairs[0].DistanceCells = 999
	s.Equal(before, s.read(s.enc))
	afterData, err := json.Marshal(s.enc.ToData())
	s.Require().NoError(err)
	s.Equal(string(beforeData), string(afterData))
}

func (s *observedContextSuite) TestEmptyObservationUniverse() {
	data := s.enc.ToData()
	for subject, holding := range data.Perception.Intel.Holdings["member|alice"] {
		holding.CurrentVia = nil
		data.Perception.Intel.Holdings["member|alice"][subject] = holding
	}
	out := s.read(s.load(data))
	s.Equal(alice, out.Observer)
	s.Equal(cellAt(0, 0), out.Position)
	s.Empty(out.Members)
	s.Empty(out.Pairs, "live roster members must not fill an empty observed universe")
}

func (s *observedContextSuite) TestObservedEmptyHandsRemainPresent() {
	data := s.enc.ToData()
	holding := data.Perception.Intel.Holdings["member|alice"]["member|goblin"]
	seen, ok := encounter.DecodeSightTestimony(holding.Payload)
	s.Require().True(ok)
	seen.Equipment = &encounter.HeldEquipment{}
	payload, err := encounter.EncodeSightTestimony(seen)
	s.Require().NoError(err)
	holding.Payload = payload
	data.Perception.Intel.Holdings["member|alice"]["member|goblin"] = holding
	member := s.member(s.read(s.load(data)), goblin)
	s.Require().NotNil(member.Equipment)
	s.Empty(member.Equipment.MainHand)
	s.Empty(member.Equipment.OffHand)
}

func (s *observedContextSuite) TestOtherObserversTestimonyDoesNotChangeThisAnswer() {
	before := s.read(s.enc)
	data := s.enc.ToData()
	holding := data.Perception.Intel.Holdings["member|bob"]["member|goblin"]
	seen, ok := encounter.DecodeSightTestimony(holding.Payload)
	s.Require().True(ok)
	seen.Position = cellAt(5, 0)
	payload, err := encounter.EncodeSightTestimony(seen)
	s.Require().NoError(err)
	holding.Payload = payload
	data.Perception.Intel.Holdings["member|bob"]["member|goblin"] = holding
	loaded := s.load(data)
	s.Equal(before, s.read(loaded))
	forBob, err := loaded.ObservedContext(&encounter.ViewInput{Member: bob})
	s.Require().NoError(err)
	s.Equal(cellAt(5, 0), s.member(forBob, goblin).Position)
}

func (s *observedContextSuite) TestInputErrors() {
	out, err := s.enc.ObservedContext(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
	s.Nil(out)
	for _, id := range []encounter.MemberID{"", "stranger"} {
		out, err = s.enc.ObservedContext(&encounter.ViewInput{Member: id})
		s.ErrorIs(err, encounter.ErrNotMember)
		s.Nil(out)
	}
}
