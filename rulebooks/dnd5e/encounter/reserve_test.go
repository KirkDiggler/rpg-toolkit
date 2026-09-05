// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// reserve_test.go is the hold-out, STEP B — arrivals and authored endings
// (rpg-project#375, design §3.7, §3.8, R6, R9, R10, §9): the acceptance rows
// this module proves on the SHIPPED raider camp — the letter that arrives at
// round 6, the three zombies that come when the chief falls — through the
// same HoldOutSuite the step-A scenes play on, so the two steps share one
// cast, one field and one set of helpers.
//
//   A4  chief Down: reinforcements at the gate on the next verb, hostile, the
//       fight forms or is joined
//   A5  the letter arrives at round 6 and not before, and never outside a
//       fight
//   A6  reserved placements: every projection byte-identical to a run without
//       them, for every member, until arrival — THE NEVER-AUTHORED YARDSTICK
//
// plus the rest of step B: a reserved monster takes no turn and is in no pair;
// a reload mid-reserve keeps the reserve, on facts alone; an arrival lands on
// the nearest free cell when its own is taken; `{ fact }` on an arrival is the
// truth grain and `{ stance }` fires in the fold after a flip; a host's own
// Spawn path (Join with a predicate) waits in reserve; an ending authored in
// the file fires; `convince` is sugar for exactly that ending; and the
// construction-time refusals for what the reserve cannot keep.

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/scenarios"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

var campReinforcements = []core.EntityID{"reinforcement-1", "reinforcement-2", "reinforcement-3"}

// shipped opens the canonical camp — arrivals and all — with the whole cast
// and the withdrawn ending.
func (s *HoldOutSuite) shipped(endings ...encounter.EndingInput) *encounter.Encounter {
	return s.open(s.canonical.Field, castOf(s.canonical, true), append([]encounter.EndingInput{withdrawn()}, endings...)...)
}

// memberIDs is the roster as ids, sorted.
func (s *HoldOutSuite) memberIDs(enc *encounter.Encounter) []encounter.MemberID {
	members, err := enc.Members()
	s.Require().NoError(err)
	out := make([]encounter.MemberID, 0, len(members))
	for _, m := range members {
		out = append(out, m.ID)
	}
	return out
}

// propIDs is every prop on the truth-grain atlas, by id.
func (s *HoldOutSuite) propIDs(enc *encounter.Encounter) []encounter.PropID {
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	out := make([]encounter.PropID, 0, len(atlas.Props))
	for _, p := range atlas.Props {
		out = append(out, p.ID)
	}
	return out
}

// projections is EVERY member-facing read this composition has, for every
// named member, rendered to bytes: what A6 compares. Keyed by read and
// member so a difference names where it is.
func (s *HoldOutSuite) projections(enc *encounter.Encounter, members []core.EntityID) map[string]string {
	out := map[string]string{}
	put := func(key string, v any, err error) {
		s.Require().NoError(err, key)
		b, merr := json.Marshal(v)
		s.Require().NoError(merr, key)
		out[key] = string(b)
	}
	atlas, err := enc.Atlas()
	put("atlas", atlas, err)
	put("doors", enc.Doors(), nil)
	roster, err := enc.Members()
	put("members", roster, err)
	for _, region := range []encounter.RegionID{"gate", "yard", "hut"} {
		in, err := enc.MembersIn(region)
		put("members-in/"+region, in, err)
	}
	status, err := enc.Status()
	put("status", status, err)
	stance, err := enc.Stance(campFaction, encounter.FactionParty)
	put("stance", stance, err)
	for _, m := range members {
		atlas, err := enc.AtlasFor(m)
		put("atlas-for/"+string(m), atlas, err)
		doors, err := enc.DoorsFor(m)
		put("doors-for/"+string(m), doors, err)
		story, err := enc.Story(&encounter.StoryInput{Audience: m})
		put("story/"+string(m), story, err)
		view, err := enc.View(&encounter.ViewInput{Member: m})
		put("view/"+string(m), view, err)
		clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: m})
		put("clock-of/"+string(m), clock, err)
		for _, other := range members {
			hostile, known := enc.IsHostile(m, other)
			put("hostile/"+string(m)+"/"+string(other), []bool{hostile, known}, nil)
		}
	}
	return out
}

// TestReservedPlacementsProjectAsNeverAuthored is A6, THE YARDSTICK: the
// shipped camp holds three zombies and the letter in reserve; the step-A camp
// never wrote them. For every member, every read the composition offers is
// byte-identical between the two runs — at first light, after a walk that
// forms a fight, and after a turn ends — until something arrives.
func (s *HoldOutSuite) TestReservedPlacementsProjectAsNeverAuthored() {
	// The step-A camp's letter lies on the ground, which the shipped camp's
	// does not until round 6 — so the yardstick's twin is the step-A camp
	// with the letter taken out entirely: a field where the reserved things
	// were NEVER WRITTEN.
	without := s.compiled.Field
	props := make([]encounter.PropInput, 0, len(without.Props))
	for _, p := range without.Props {
		if p.ID != campLetter {
			props = append(props, p)
		}
	}
	without.Props = props

	members := []core.EntityID{raider, partner, campChief, campScout}
	withReserve := s.shipped()
	never := s.open(without, castOf(s.compiled, true), withdrawn())

	compare := func(when string) {
		a, b := s.projections(withReserve, members), s.projections(never, members)
		for key, want := range b {
			s.Equal(want, a[key], "%s: %s differs with the reserve", when, key)
		}
		s.Len(a, len(b))
	}
	compare("at first light")

	s.intoTheYard(withReserve, raider)
	s.intoTheYard(never, raider)
	compare("after a fight formed")

	for _, enc := range []*encounter.Encounter{withReserve, never} {
		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: raider})
		s.Require().NoError(err)
	}
	compare("after a round wrapped")

	s.Run("the reserve is not a projection, and the blob says what waits", func() {
		data := withReserve.ToData()
		s.Len(data.Reserve, 3)
		s.Nil(never.ToData().Reserve)
	})
}

// TestTheLetterArrivesAtRoundSixAndNotBefore is A5, and R9: the letter is
// nowhere — not holdable, in no atlas — through eight ticks of the world
// clock and five rounds of a fight, and lies at the gate the moment round 6
// starts, with an `arrived` beat to everyone.
func (s *HoldOutSuite) TestTheLetterArrivesAtRoundSixAndNotBefore() {
	enc := s.shipped()

	absent := func(when string) {
		s.NotContains(s.propIDs(enc), campLetter, "%s: the letter is on the map", when)
		for _, m := range []core.EntityID{raider, partner} {
			atlas, err := enc.AtlasFor(m)
			s.Require().NoError(err)
			for _, p := range atlas.Props {
				s.NotEqual(campLetter, p.ID, "%s: %s sees the letter", when, m)
			}
		}
		_, err := enc.Hold(&encounter.HoldInput{Member: partner, Target: campLetter, Range: 5})
		s.Require().ErrorIs(err, encounter.ErrNoProp, "%s: the letter refuses as a thing that is not here", when)
		s.Empty(s.beatsOfKind(enc, partner, "arrived"))
	}
	absent("at first light")

	s.Run("outside any fight the world clock counts nothing", func() {
		for i := 0; i < 8; i++ {
			_, err := enc.Pump(&encounter.PumpInput{})
			s.Require().NoError(err)
		}
		absent("after eight ticks")
	})

	out := s.intoTheYard(enc, raider)
	s.Require().NotNil(out.Formed, "the fight is on")
	s.Require().Equal(raider, out.Formed.Order[0])

	s.Run("rounds two through five: still nothing", func() {
		for round := 2; round <= 5; round++ {
			end, err := enc.EndTurn(&encounter.EndTurnInput{Member: raider})
			s.Require().NoError(err)
			s.Require().True(end.RoundWrapped, "round %d", round)
			absent("in round " + string(rune('0'+round)))
		}
	})

	s.Run("round six: the letter lies at the gate for everyone", func() {
		end, err := enc.EndTurn(&encounter.EndTurnInput{Member: raider})
		s.Require().NoError(err)
		s.Require().True(end.RoundWrapped)

		s.Contains(s.propIDs(enc), campLetter)
		atlas, err := enc.AtlasFor(partner)
		s.Require().NoError(err)
		found := false
		for _, p := range atlas.Props {
			if p.ID == campLetter {
				found = true
				s.Equal(cellAt(1, 3), p.At, "where the author drew it")
				s.True(p.Holdable)
			}
		}
		s.True(found, "the partner's map shows the letter")

		arrived := s.beatsOfKind(enc, partner, "arrived")
		s.Require().Len(arrived, 1)
		s.Equal(campLetter, arrived[0]["id"])
		s.Equal(encounter.ArrivedProp, arrived[0]["kind"])
		s.Equal(map[string]any{"x": float64(cellAt(1, 3).X), "y": float64(cellAt(1, 3).Y)}, arrived[0]["cell"])
		s.Len(s.beatsOfKind(enc, raider, "arrived"), 1, "everyone hears it")

		_, err = enc.Hold(&encounter.HoldInput{Member: partner, Target: campLetter, Range: 5})
		s.Require().NoError(err, "and it can be picked up")
	})
}

// TestTheChiefsFallBringsTheReinforcementsToTheGate is A4: the chief falls,
// and on the very next verb three zombies stand at the gate where the file
// drew them — in the raiders' faction, hostile to the party — and the partner
// standing at the gate is pulled into the running fight with them.
func (s *HoldOutSuite) TestTheChiefsFallBringsTheReinforcementsToTheGate() {
	enc := s.shipped()
	formed := s.intoTheYard(enc, raider)
	s.Require().NotNil(formed.Formed, "precondition: the raider and the scout are fighting")
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, partner), "precondition: the partner is at the gate, out of it")

	s.standing.down = []encounter.MemberID{campChief}
	// The next verb: the partner takes a step at the gate. Its participation
	// pass notices the chief down, and the reinforcements arrive.
	s.step(enc, partner, cellAt(0, 4))

	s.Run("three zombies stand at the gate, in the faction, hostile", func() {
		roster := s.memberIDs(enc)
		for _, id := range campReinforcements {
			s.Contains(roster, id)
			m := mustMember(s.T(), enc, id)
			s.Equal(encounter.KindMonster, m.Kind)
			s.Equal(campFaction, m.Faction)
			s.Equal("gate", m.Region)
			hostile, known := enc.IsHostile(partner, id)
			s.True(known)
			s.True(hostile)
		}
		s.Equal(cellAt(1, 4), mustMember(s.T(), enc, campReinforcements[0]).Position)
		s.Equal(cellAt(2, 4), mustMember(s.T(), enc, campReinforcements[1]).Position)
		s.Equal(cellAt(1, 5), mustMember(s.T(), enc, campReinforcements[2]).Position)
	})

	s.Run("the arrivals are narrated to everyone, after the fall", func() {
		beats := s.beats(enc, partner)
		downAt, firstArrival := -1, -1
		var arrivals []map[string]any
		for i, b := range beats {
			switch b["beat"] {
			case "down":
				if b["member"] == string(campChief) {
					downAt = i
				}
			case "arrived":
				if firstArrival < 0 {
					firstArrival = i
				}
				arrivals = append(arrivals, b)
			}
		}
		s.Require().NotEqual(-1, downAt, "%v", beats)
		s.Require().Len(arrivals, 3, "%v", beats)
		s.Less(downAt, firstArrival, "the fall is the cause")
		for i, b := range arrivals {
			s.Equal(string(campReinforcements[i]), b["id"], "in id order")
			s.Equal(encounter.ArrivedMonster, b["kind"])
		}
		s.Len(s.beatsOfKind(enc, raider, "arrived"), 3)
		s.Len(s.beatsOfKind(enc, campReinforcements[0], "arrived"), 3, "the arrival hears itself arrive")
	})

	s.Run("the fight is joined: the zombies and the partner are in it", func() {
		for _, id := range append([]core.EntityID{partner}, campReinforcements...) {
			s.Equal(encounter.ClockTurn, s.clockOf(enc, id), id)
		}
		s.Equal(encounter.ClockTurn, s.clockOf(enc, raider))
		s.Empty(s.beatsOfKind(enc, partner, "bubble-dissolved"), "one fight, grown")
	})

	s.Run("the journal records each arrival once, on the truth grain", func() {
		data := enc.ToData()
		s.Nil(data.Reserve, "nothing is waiting any more")
		s.Require().NotNil(data.Holdings)
		var arrived []encounter.FactData
		for _, f := range data.Holdings.Facts {
			if strings.HasPrefix(f.Kind, "arrived:") {
				arrived = append(arrived, f)
			}
		}
		s.Require().Len(arrived, 3)
		s.Equal("arrived:reinforcement-1@"+coords(cellAt(1, 4)), arrived[0].Kind)
		s.Equal("member:reinforcement-1", arrived[0].Subject)
		s.Equal("reinforcement-1", arrived[0].Actor)
		s.Empty(arrived[0].Audience, "truth grain")
	})
}

// coords spells a cell the way an arrived or dropped kind does.
func coords(p spatial.Position) string {
	b, _ := json.Marshal([]float64{p.X, p.Y})
	return strings.Trim(string(b), "[]")
}

// TestAReservedMonsterTakesNoTurnAndIsInNoPair pins design §3.7's list
// directly: before the chief falls, the reinforcements are on no roster, on
// no clock, in no fight's order, in no story, and cannot be addressed by any
// verb — as if they were never written.
func (s *HoldOutSuite) TestAReservedMonsterTakesNoTurnAndIsInNoPair() {
	enc := s.shipped()
	for _, id := range campReinforcements {
		s.NotContains(s.memberIDs(enc), id)
		_, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
		s.ErrorIs(err, encounter.ErrNotMember)
		_, err = enc.Story(&encounter.StoryInput{Audience: id})
		s.ErrorIs(err, encounter.ErrNoMember, "no story was ever told to it")
		_, err = enc.Exit(&encounter.ExitInput{Member: id})
		s.ErrorIs(err, encounter.ErrNotMember)
		_, err = enc.AtlasFor(id)
		s.ErrorIs(err, encounter.ErrNotMember)
		_, err = enc.Join(&encounter.JoinInput{Member: id, Kind: encounter.KindMonster, Cell: cellAt(1, 1), Faction: campFaction})
		s.ErrorIs(err, encounter.ErrNoMember, "but it is in the encounter: nobody else may join under its id")
	}

	out := s.intoTheYard(enc, raider)
	s.Require().NotNil(out.Formed)
	for _, id := range campReinforcements {
		s.NotContains(out.Formed.Order, id, "in no pair")
	}
	// Every turn that ends is the raider's or the scout's; nothing waits on
	// a member who is not here.
	for i := 0; i < 3; i++ {
		end, err := enc.EndTurn(&encounter.EndTurnInput{Member: raider})
		s.Require().NoError(err)
		s.Equal(raider, end.Next)
	}
	for _, b := range s.beatsOfKind(enc, raider, "turn-ended") {
		s.Contains([]any{string(raider), string(campScout)}, b["member"])
	}
}

// TestReloadMidReserveKeepsTheReserve is the persistence half of §3.7: saved
// with three zombies waiting, loaded, they are still waiting — nothing but the
// reserve entries and, once they come, the `arrived:` facts say anything
// about them — and the reloaded run brings them in on the chief's fall exactly
// as the original would have. Save, load, save is byte-identical either side
// of the arrival.
func (s *HoldOutSuite) TestReloadMidReserveKeepsTheReserve() {
	enc := s.shipped()
	s.intoTheYard(enc, raider)

	data := enc.ToData()
	s.Run("the blob holds the reserve and no arrival", func() {
		s.Require().Len(data.Reserve, 3)
		for i, r := range data.Reserve {
			s.Equal(campReinforcements[i], r.ID)
			s.Equal(encounter.KindMonster, r.Kind)
			s.Equal(campFaction, r.Faction)
			s.Equal(encounter.TriggerData{Kind: "member_down", Member: campChief}, r.Arrives)
		}
		s.Nil(data.Holdings, "nothing has been held, dropped or arrived")
		for _, m := range data.Members {
			s.NotContains(campReinforcements, m.ID)
		}
		first, err := json.Marshal(data)
		s.Require().NoError(err)
		second, err := json.Marshal(s.reload(enc).ToData())
		s.Require().NoError(err)
		s.Equal(string(first), string(second), "save, load, save")
	})

	back := s.reload(enc)
	s.Run("the reloaded run still holds them back", func() {
		for _, id := range campReinforcements {
			s.NotContains(s.memberIDs(back), id)
		}
		s.NotContains(s.propIDs(back), campLetter)
	})

	s.standing.down = []encounter.MemberID{campChief}
	s.step(back, partner, cellAt(0, 4))
	s.Run("and brings them in when the chief falls", func() {
		for _, id := range campReinforcements {
			s.Contains(s.memberIDs(back), id)
		}
		after := back.ToData()
		s.Nil(after.Reserve)
		s.Require().NotNil(after.Holdings)
		s.Len(after.Holdings.Facts, 3, "three arrived facts and nothing else: %v", after.Holdings.Facts)

		again := s.reload(back)
		for _, id := range campReinforcements {
			s.Contains(s.memberIDs(again), id)
			s.Equal(encounter.ClockTurn, s.clockOf(again, id))
		}
		first, err := json.Marshal(after)
		s.Require().NoError(err)
		second, err := json.Marshal(again.ToData())
		s.Require().NoError(err)
		s.Equal(string(first), string(second), "save, load, save after the arrival")
	})

	s.Run("the load boundary refuses a reserve that has already arrived", func() {
		// A zombie that came and then left: in ever_members, off the roster,
		// its arrival in the journal — and a hand-edited blob that says it is
		// still waiting.
		_, err := back.Exit(&encounter.ExitInput{Member: campReinforcements[0]})
		s.Require().NoError(err)
		edited := back.ToData()
		edited.Reserve = []encounter.ReserveData{{
			ID: campReinforcements[0], Kind: encounter.KindMonster, Faction: campFaction,
			Cell:    encounter.PositionData{X: cellAt(1, 4).X, Y: cellAt(1, 4).Y},
			Arrives: encounter.TriggerData{Kind: "member_down", Member: campChief},
		}}
		_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data:  edited,
			Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		})
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Contains(err.Error(), "already arrived")
	})
}

// TestAnArrivalLandsOnTheNearestFreeCellWhenItsCellIsTaken is §3.7's second
// clause: the partner stands where the first zombie was drawn, so it lands on
// the nearest free cell of the gate instead — never under anybody's feet.
func (s *HoldOutSuite) TestAnArrivalLandsOnTheNearestFreeCellWhenItsCellIsTaken() {
	members := castOf(s.canonical, true)
	for i := range members {
		if members[i].ID == partner {
			members[i].Position = spatial.Position{X: 1, Y: 4}
		}
	}
	enc := s.open(s.canonical.Field, members, withdrawn())
	s.standing.down = []encounter.MemberID{campChief}
	s.step(enc, raider, cellAt(0, 4))

	first := mustMember(s.T(), enc, campReinforcements[0])
	s.NotEqual(cellAt(1, 4), first.Position, "not on the partner")
	s.Equal("gate", first.Region, "in the same region")
	s.Equal(1.0, enc.Distance(cellAt(1, 4), first.Position), "the nearest free cell")
	s.Equal(cellAt(2, 4), mustMember(s.T(), enc, campReinforcements[1]).Position, "the others land where they were drawn")
	s.Equal(cellAt(1, 5), mustMember(s.T(), enc, campReinforcements[2]).Position)
	taken := map[spatial.Position]core.EntityID{}
	for _, m := range mustMembers(s.T(), enc) {
		prev, dup := taken[m.Position]
		s.False(dup, "%s and %s share %v", prev, m.ID, m.Position)
		taken[m.Position] = m.ID
	}
	arrived := s.beatsOfKind(enc, partner, "arrived")
	s.Require().Len(arrived, 3)
	s.Equal(map[string]any{"x": first.Position.X, "y": first.Position.Y}, arrived[0]["cell"], "the beat names where it actually landed")
}

// mustMembers reads the whole roster.
func mustMembers(t interface {
	Fatal(...any)
	Helper()
}, enc *encounter.Encounter) []encounter.Member {
	t.Helper()
	members, err := enc.Members()
	if err != nil {
		t.Fatal(err)
	}
	return members
}

// TestArrivalsOnAFactAndOnAStance covers the two remaining forms on the
// step-A camp: a prop that arrives when the fact exists in the run — the
// TRUTH grain, so the raider reading the letter at the gate is enough (R5) —
// and a monster that arrives when the raiders and the party turn neutral, in
// the fold after the flip.
func (s *HoldOutSuite) TestArrivalsOnAFactAndOnAStance() {
	const reward, messenger = "reward", "messenger"
	field := s.compiled.Field
	prize := holdableProp(reward, "dnd5e:props:chest", spatial.Position{X: 2, Y: 2})
	prize.Arrives = encounter.TriggerFact{Fact: campFact}
	field.Props = append(append([]encounter.PropInput(nil), field.Props...), prize)
	members := append(s.cast(true), encounter.MemberInput{
		ID: messenger, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 6}, Faction: campFaction,
		Arrives: encounter.TriggerStance{Between: [2]encounter.FactionID{campFaction, encounter.FactionParty}, Stance: encounter.StanceNeutral},
	})
	enc := s.open(field, members, withdrawn(), s.holdOutEnding())

	s.NotContains(s.propIDs(enc), reward)
	s.NotContains(s.memberIDs(enc), core.EntityID(messenger))

	s.hold(enc, raider, campLetter)
	s.Run("the raider learning the fact is enough for the prize to appear", func() {
		s.Contains(s.propIDs(enc), reward, "truth grain: anyone knowing it counts")
		s.NotContains(s.memberIDs(enc), core.EntityID(messenger), "the camp has not turned")
		arrived := s.beatsOfKind(enc, partner, "arrived")
		s.Require().Len(arrived, 1)
		s.Equal(reward, arrived[0]["id"])
	})

	s.intoTheYard(enc, raider)
	out := s.intoTheHut(enc, raider)
	s.Run("the camp turning brings the messenger, before the run ends", func() {
		s.Require().NotNil(out.Outcome)
		s.Equal(scenarios.HoldOutID, out.Outcome.Ending)
		s.Contains(s.memberIDs(enc), core.EntityID(messenger))
		s.Equal(campFaction, mustMember(s.T(), enc, messenger).Faction)
		beats := s.beats(enc, partner)
		stanceAt, arrivedAt, endedAt := -1, -1, -1
		for i, b := range beats {
			switch {
			case b["beat"] == "stance":
				stanceAt = i
			case b["beat"] == "arrived" && b["id"] == messenger:
				arrivedAt = i
			case b["beat"] == "ended":
				endedAt = i
			}
		}
		s.Require().NotEqual(-1, arrivedAt, "%v", beats)
		s.Less(stanceAt, arrivedAt, "the flip is the cause")
		s.Less(arrivedAt, endedAt, "and the ending is the last word")
	})
}

// TestAJoinerWithAPredicateWaitsInReserve is the host's own path (design §3
// Spawn): the world is built with the party alone, and every monster arrives
// through Join — the reinforcements with their predicate hand-carried, which
// puts them in reserve rather than on the map, with no beat and nothing
// projected, until the chief they wait on falls.
func (s *HoldOutSuite) TestAJoinerWithAPredicateWaitsInReserve() {
	enc := s.open(s.canonical.Field, castOf(s.canonical, true)[:2], withdrawn())
	before := len(s.beats(enc, partner))

	for _, m := range s.canonical.Monsters {
		out, err := enc.Join(&encounter.JoinInput{
			Member: core.EntityID(m.ID), Kind: encounter.KindMonster, Cell: cellAt(int(m.At.X), int(m.At.Y)),
			Faction: m.Faction, Holds: m.Holds, Arrives: m.Arrives,
		})
		s.Require().NoError(err, m.ID)
		if m.Arrives == nil {
			s.False(out.Reserved, m.ID)
			continue
		}
		s.True(out.Reserved, m.ID)
		s.Zero(out.Seq, "no beat was written")
		s.Nil(out.Formed)
		s.Nil(out.IntelDeltas)
		s.Equal(core.EntityID(m.ID), out.Member.ID)
		s.Equal(campFaction, out.Member.Faction)
		s.Equal("gate", out.Member.Region, "the output names the cell it will arrive at")
	}
	s.Len(s.beats(enc, partner), before+2, "two joined beats: the chief and the scout; the reserve is silent")
	for _, id := range campReinforcements {
		s.NotContains(s.memberIDs(enc), id)
	}

	s.standing.down = []encounter.MemberID{campChief}
	s.step(enc, partner, cellAt(0, 4))
	for _, id := range campReinforcements {
		s.Contains(s.memberIDs(enc), id)
	}
}

// TestAnEndingAuthoredInTheFileFires is R10's first half: `endings:` in the
// file compiles to the same [encounter.EndingInput] a scenario would declare,
// and the run ends on it.
func (s *HoldOutSuite) TestAnEndingAuthoredInTheFileFires() {
	compiled := s.compileWithEndings("  - { id: held-out, when: { round: 2 } }\n")
	s.Require().Equal([]encounter.EndingInput{{Key: "held-out", Trigger: encounter.TriggerRound{Round: 2}}}, compiled.Endings)

	enc := s.open(compiled.Field, castOf(compiled, true), append([]encounter.EndingInput{withdrawn()}, compiled.Endings...)...)
	out := s.intoTheYard(enc, raider)
	s.Require().NotNil(out.Formed)
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: raider})
	s.Require().NoError(err)
	status, err := enc.Status()
	s.Require().NoError(err)
	s.False(status.Open)
	s.Equal("held-out", status.Outcome.Ending)
	ended := s.beatsOfKind(enc, partner, "ended")
	s.Require().Len(ended, 1)
	s.Equal("held-out", ended[0]["ending"])
}

// TestConvinceIsSugarForAnAuthoredStanceEnding is R10's second half, pinned
// both ways: the file's `scenarios.hold-out.convince: raiders` and the file's
// `endings: [{ id: hold-out, when: { stance: ... } }]` compile to the SAME
// declared ending, and a run bound to the authored one ends exactly as A2's
// did on the scenario's.
func (s *HoldOutSuite) TestConvinceIsSugarForAnAuthoredStanceEnding() {
	compiled := s.compileWithEndings(
		"  - { id: hold-out, when: { stance: { between: [raiders, party], is: neutral } } }\n")
	s.Require().Len(compiled.Endings, 1)

	scenario, ok := scenarios.Lookup(scenarios.HoldOutID)
	s.Require().True(ok)
	declared, err := scenario.New(compiled.Scenarios[scenarios.HoldOutID], scenarios.FactsFrom(compiled.Field))
	s.Require().NoError(err)
	s.Require().Len(declared.Endings, 1)
	s.Equal(declared.Endings[0], compiled.Endings[0], "the scenario's field is sugar for the authored ending")

	// Bound to the AUTHORED ending alone: A2, word for word.
	enc := s.open(compiled.Field, castOf(compiled, true), withdrawn(), compiled.Endings[0])
	s.hold(enc, raider, campLetter)
	s.intoTheYard(enc, raider)
	out := s.intoTheHut(enc, raider)
	s.Require().NotNil(out.Outcome)
	s.Equal(scenarios.HoldOutID, out.Outcome.Ending)
	s.Equal(encounter.StanceNeutral, s.stance(enc))
}

// compileWithEndings is the step-A camp with an `endings:` block written in
// before `scenarios:`.
func (s *HoldOutSuite) compileWithEndings(entries string) dungeonspec.Compiled {
	raw, err := os.ReadFile(campPath)
	s.Require().NoError(err)
	source := stepASource(s.T(), string(raw))
	s.Require().Equal(1, strings.Count(source, "scenarios:\n"))
	source = strings.Replace(source, "scenarios:\n", "endings:\n"+entries+"\nscenarios:\n", 1)
	compiled, err := dungeonspec.Load([]byte(source))
	s.Require().NoError(err)
	return compiled
}

// TestTheRunRefusesAReserveItCannotKeep is the construction-time liveness for
// the reserve, at both ways in: a player has nowhere to wait, a member cannot
// wait on its own fall, a nameless prop cannot be recorded as having come, and
// a predicate that can never hold is refused as an ending's would be.
func (s *HoldOutSuite) TestTheRunRefusesAReserveItCannotKeep() {
	open := func(field encounter.FieldInput, members []encounter.MemberInput) error {
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
			Field: field, Members: members, Endings: []encounter.EndingInput{withdrawn()},
		})
		return err
	}
	withArrival := func(id core.EntityID, t encounter.Trigger) []encounter.MemberInput {
		members := s.cast(true)
		for i := range members {
			if members[i].ID == id {
				members[i].Arrives = t
			}
		}
		return members
	}

	s.Run("a player cannot wait in reserve", func() {
		err := open(s.compiled.Field, withArrival(partner, encounter.TriggerRound{Round: 3}))
		s.Require().ErrorIs(err, encounter.ErrNoMember)
		s.Contains(err.Error(), "only a monster waits in reserve")
	})
	s.Run("a member cannot wait for its own fall", func() {
		err := open(s.compiled.Field, withArrival(campScout, encounter.TriggerMemberDown{Member: campScout}))
		s.Require().ErrorIs(err, encounter.ErrNoMember)
		s.Contains(err.Error(), "its own fall")
	})
	s.Run("a round counted from zero", func() {
		err := open(s.compiled.Field, withArrival(campScout, encounter.TriggerRound{}))
		s.Require().ErrorIs(err, encounter.ErrNoMember)
		s.Contains(err.Error(), "counted from 1")
	})
	s.Run("a stance the pair holds from the start", func() {
		err := open(s.compiled.Field, withArrival(campScout, encounter.TriggerStance{
			Between: [2]encounter.FactionID{campFaction, encounter.FactionParty}, Stance: encounter.StanceHostile,
		}))
		s.Require().ErrorIs(err, encounter.ErrNoMember)
		s.Contains(err.Error(), "from the start")
	})
	s.Run("a prop that arrives must be nameable", func() {
		field := s.compiled.Field
		nameless := holdableProp("", "dnd5e:props:chest", spatial.Position{X: 2, Y: 2})
		nameless.Holdable = false
		nameless.Arrives = encounter.TriggerRound{Round: 2}
		field.Props = append(append([]encounter.PropInput(nil), field.Props...), nameless)
		err := open(field, s.cast(true))
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "arrives on a predicate and has no id")
	})
	s.Run("a prop's predicate that can never hold", func() {
		field := s.compiled.Field
		prize := holdableProp("prize", "dnd5e:props:chest", spatial.Position{X: 2, Y: 2})
		prize.Arrives = encounter.TriggerFact{}
		field.Props = append(append([]encounter.PropInput(nil), field.Props...), prize)
		err := open(field, s.cast(true))
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "names no fact")
	})
	s.Run("Join refuses the same shapes before it mutates", func() {
		enc := s.open(s.compiled.Field, s.cast(true), withdrawn())
		_, err := enc.Join(&encounter.JoinInput{
			Member: "late", Kind: encounter.KindPlayer, Cell: cellAt(1, 1), Arrives: encounter.TriggerRound{Round: 2},
		})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
		_, err = enc.Join(&encounter.JoinInput{
			Member: "late", Kind: encounter.KindMonster, Cell: cellAt(1, 1), Faction: campFaction,
			Arrives: encounter.TriggerMemberDown{Member: "late"},
		})
		s.Require().ErrorIs(err, encounter.ErrNoMember)
		s.NotContains(s.memberIDs(enc), core.EntityID("late"))
		data := enc.ToData()
		s.Nil(data.Reserve, "nothing was reserved by a refused join")
	})
	s.Run("the shipped camp is accepted", func() {
		s.Require().NoError(open(s.canonical.Field, castOf(s.canonical, true)))
	})
}
