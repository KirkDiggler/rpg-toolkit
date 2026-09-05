// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// holdout_test.go is the hold-out, STEP A — sides and knowledge
// (rpg-project#375, design §9): the acceptance rows this module proves,
// played on the raider camp fixture itself, through the scenario package
// itself, so what a walk will meet is what these scenes met.
//
//   A1  the camp is hostile until a fact; nobody knows; a fight forms on sight
//   A2  the letter carried into the chief's region mid-fight: stance neutral,
//       the fight dissolves, the hold-out ending fires
//   A3  the letter carried into the scout's region, or read by the scout:
//       nothing
//   A8  the graph answers hostile/allied per pair, for resolution to read
//   A9  save after the flip, load: still neutral; no stance stored
//
// plus the consequences the design names beside them: a dead mind cannot
// learn (§3.9), a faction of one has its member as mind (§2), a third faction
// still hostile keeps its fight (§3.5), a fact ending and a round ending fire
// where their events are noticed (§3.8), and the trust boundary at load.

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/scenarios"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	campPath    = "dungeonspec/testdata/reference-raider-camp.yaml"
	campKey     = "reference-raider-camp"
	campFaction = "raiders"
	campFact    = "saved-wiseman"
	campLetter  = "letter"
	campRecord  = campKey + "/wisemans-letter"
	gateYard    = campKey + "/gate-yard"
	yardHut     = campKey + "/yard-hut"
)

var (
	campChief = core.EntityID("chief")
	campScout = core.EntityID("scout")
)

type HoldOutSuite struct {
	suite.Suite

	compiled dungeonspec.Compiled
	standing *downList
	heard    *journal
}

func TestHoldOutSuite(t *testing.T) {
	suite.Run(t, new(HoldOutSuite))
}

func (s *HoldOutSuite) SetupTest() {
	raw, err := os.ReadFile(campPath)
	s.Require().NoError(err)
	s.compiled, err = dungeonspec.Load(raw)
	s.Require().NoError(err, "the fixture compiles")
	s.standing = &downList{}
	s.heard = &journal{}
}

// cast is the party at the gate and the camp's monsters as the file placed
// them — id, faction and holdings straight off the compiled placements, the
// way a host spawns them.
func (s *HoldOutSuite) cast(withScout bool) []encounter.MemberInput {
	seats := s.compiled.PartyStart
	members := []encounter.MemberInput{
		{ID: raider, Kind: encounter.KindPlayer, Position: seats[0].At},
		{ID: partner, Kind: encounter.KindPlayer, Position: seats[1].At},
	}
	for _, m := range s.compiled.Monsters {
		if !withScout && m.ID == string(campScout) {
			continue
		}
		members = append(members, encounter.MemberInput{
			ID: core.EntityID(m.ID), Kind: encounter.KindMonster, Position: m.At,
			Faction: m.Faction, Holds: m.Holds,
		})
	}
	return members
}

// holdOutEnding is the scenario's own declaration, read from the file's
// binding through the scenario package — the same path a host takes.
func (s *HoldOutSuite) holdOutEnding() encounter.EndingInput {
	scenario, ok := scenarios.Lookup(scenarios.HoldOutID)
	s.Require().True(ok)
	declared, err := scenario.New(s.compiled.Scenarios[scenarios.HoldOutID], scenarios.FactsFrom(s.compiled.Field))
	s.Require().NoError(err)
	s.Require().Equal(campFaction, declared.Convince)
	s.Require().Len(declared.Endings, 1)
	return declared.Endings[0]
}

// withdrawn is the ending every scene declares so the run can always end.
func withdrawn() encounter.EndingInput {
	return encounter.EndingInput{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}
}

func (s *HoldOutSuite) open(field encounter.FieldInput, members []encounter.MemberInput, endings ...encounter.EndingInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: journalAnnouncer{j: s.heard},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field:     field,
		Members:   members,
		Endings:   endings,
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	return enc
}

// camp opens the fixture with the whole cast and the withdrawn ending.
func (s *HoldOutSuite) camp(endings ...encounter.EndingInput) *encounter.Encounter {
	return s.open(s.compiled.Field, s.cast(true), append([]encounter.EndingInput{withdrawn()}, endings...)...)
}

func (s *HoldOutSuite) reload(enc *encounter.Encounter) *encounter.Encounter {
	out, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  enc.ToData(),
		Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: journalAnnouncer{j: s.heard},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
	s.Require().NoError(err)
	return out
}

// doorway is the two cells of a doorway, the one in `near` first.
func (s *HoldOutSuite) doorway(enc *encounter.Encounter, door string, near encounter.RegionID) (spatial.Position, spatial.Position) {
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	for _, dw := range atlas.Doorways {
		if dw.Door != door {
			continue
		}
		if r, _ := enc.RegionAt(dw.From); r == near {
			return dw.From, dw.To
		}
		return dw.To, dw.From
	}
	s.Require().Failf("no doorway", "%s is not in the atlas", door)
	return spatial.Position{}, spatial.Position{}
}

func (s *HoldOutSuite) step(enc *encounter.Encounter, member core.EntityID, to spatial.Position) *encounter.StepOutput {
	out, err := enc.Step(&encounter.StepInput{Member: member, To: to})
	s.Require().NoError(err, "step %s to %v", member, to)
	return out
}

// intoTheYard walks a member from the gate through the palisade's doorway,
// returning the step that crossed it.
func (s *HoldOutSuite) intoTheYard(enc *encounter.Encounter, member core.EntityID) *encounter.StepOutput {
	near, far := s.doorway(enc, gateYard, "gate")
	s.step(enc, member, near)
	return s.step(enc, member, far)
}

// intoTheHut walks a member from the yard through the hut's doorway.
func (s *HoldOutSuite) intoTheHut(enc *encounter.Encounter, member core.EntityID) *encounter.StepOutput {
	near, far := s.doorway(enc, yardHut, "yard")
	s.step(enc, member, near)
	return s.step(enc, member, far)
}

func (s *HoldOutSuite) hold(enc *encounter.Encounter, member core.EntityID, prop encounter.PropID) {
	_, err := enc.Hold(&encounter.HoldInput{Member: member, Target: prop})
	s.Require().NoError(err)
}

func (s *HoldOutSuite) stance(enc *encounter.Encounter) encounter.Stance {
	stance, err := enc.Stance(campFaction, encounter.FactionParty)
	s.Require().NoError(err)
	return stance
}

func (s *HoldOutSuite) beats(enc *encounter.Encounter, member core.EntityID) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]map[string]any, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}
	return out
}

func (s *HoldOutSuite) beatsOfKind(enc *encounter.Encounter, member core.EntityID, kind string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, beat := range s.beats(enc, member) {
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}
	return out
}

func (s *HoldOutSuite) clockOf(enc *encounter.Encounter, member core.EntityID) encounter.ClockKind {
	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: member})
	s.Require().NoError(err)
	return out.Kind
}

// TestACampHostileUntilAFactFormsAFightOnSight is A1: nobody knows the fact,
// so the raiders are hostile, and a player walking into the yard starts a
// fight with the scout — by faction, not by kind.
func (s *HoldOutSuite) TestACampHostileUntilAFactFormsAFightOnSight() {
	enc := s.camp()

	s.Run("the declared stance holds while nobody knows", func() {
		s.Equal(encounter.StanceHostile, s.stance(enc))
		hostile, known := enc.IsHostile(raider, campScout)
		s.True(known)
		s.True(hostile)
	})

	out := s.intoTheYard(enc, raider)
	s.Run("a fight forms on sight", func() {
		s.Require().NotNil(out.Formed, "the scout and the raider saw each other")
		s.Contains(out.Formed.Order, raider)
		s.Contains(out.Formed.Order, campScout)
		s.Equal(encounter.ClockTurn, s.clockOf(enc, raider))
		s.Equal(encounter.ClockTurn, s.clockOf(enc, campScout))
	})
}

// TestTheLetterCarriedToTheChiefMidFightTurnsTheCamp is A2, and the third
// caller of the one place a fight ends (combatend_test's structural pin): the
// stance folds to neutral, the fight dissolves with ByStance, the hold-out
// ending fires, and every member of the fight hears it end.
func (s *HoldOutSuite) TestTheLetterCarriedToTheChiefMidFightTurnsTheCamp() {
	enc := s.camp(s.holdOutEnding())
	s.hold(enc, raider, campLetter)
	formed := s.intoTheYard(enc, raider)
	s.Require().NotNil(formed.Formed, "precondition: the fight is on")
	fighting := append([]encounter.MemberID(nil), formed.Formed.Order...)

	out := s.intoTheHut(enc, raider)

	s.Run("the stance is neutral and the fight is over", func() {
		s.Equal(encounter.StanceNeutral, s.stance(enc))
		hostile, known := enc.IsHostile(raider, campScout)
		s.True(known)
		s.False(hostile, "a raider is no longer an enemy")
		s.Require().NotNil(out.Outcome, "the hold-out ended the run")
		s.Equal(scenarios.HoldOutID, out.Outcome.Ending)
	})

	s.Run("the story reads stance, then bubble-dissolved by stance, then ended", func() {
		beats := s.beats(enc, partner)
		var stanceAt, dissolvedAt, endedAt = -1, -1, -1
		for i, b := range beats {
			switch b["beat"] {
			case "stance":
				stanceAt = i
				s.Equal([]any{encounter.FactionParty, campFaction}, b["between"], "the pair, in its one normalized order")
				s.Equal(string(encounter.StanceNeutral), b["stance"])
			case "bubble-dissolved":
				dissolvedAt = i
				s.Equal(string(encounter.DissolveByStance), b["cause"])
			case "ended":
				endedAt = i
				s.Equal(scenarios.HoldOutID, b["ending"])
			}
		}
		s.Require().NotEqual(-1, stanceAt, "no stance beat: %v", beats)
		s.Require().NotEqual(-1, dissolvedAt, "no dissolution: %v", beats)
		s.Require().NotEqual(-1, endedAt, "no ending: %v", beats)
		s.Less(stanceAt, dissolvedAt, "the flip is the cause, the dissolution its effect")
		s.Less(dissolvedAt, endedAt, "and the ending is the last word")
	})

	s.Run("the fight ended for exactly the members it held", func() {
		want := make([]string, 0, len(fighting))
		for _, id := range fighting {
			want = append(want, string(id))
		}
		sort.Strings(want)
		s.Equal(want, combatEndsIn(s.heard), "%v", s.heard.entries)
	})

	s.Run("the chief knows, as a fact audienced to the chief alone", func() {
		data := enc.ToData()
		s.Require().NotNil(data.World)
		var learned []encounter.FactData
		for _, f := range data.World.Facts {
			if f.Kind == "known:fact:"+campFact {
				learned = append(learned, f)
			}
		}
		s.Require().Len(learned, 2, "the raider from the letter, the chief from the raider's presence: %v", data.World.Facts)
		for _, f := range learned {
			s.Equal(f.Actor, f.Subject, "the learner is the subject")
			s.Equal([]string{f.Actor}, f.Audience, "and the only witness")
		}
		s.Equal(string(campChief), learned[1].Actor)
	})
}

// TestTheLetterCarriedToTheScoutFlipsNothing is A3, twice over: a scout
// authored holding the letter's record turns nothing, and the letter carried
// into the scout's own region teaches nobody — a scout is not the camp's
// mind, and presence teaches minds alone.
//
// HOLDING IS NOT KNOWING. The authored scout carries the record and has not
// learned the fact (no `known:fact` is written at seed, exactly as a monster
// authored with a door's record does not "know" the door), so no scene here
// has a scout who KNOWS and still turns nothing. That variant — a non-mind
// coming to know a fact, and the flip staying with the mind — waits on the
// word-spreads shelf (design §11), where members learn by presence or by
// witnessing the handover and the mind learns N turns later. Until then the
// graph's own shape pins it: the Settle reads the mind's flag and nobody
// else's (world/graph settle_test).
func (s *HoldOutSuite) TestTheLetterCarriedToTheScoutFlipsNothing() {
	s.Run("a scout authored knowing the fact turns nothing", func() {
		members := s.cast(true)
		for i := range members {
			if members[i].ID == campScout {
				members[i].Holds = []encounter.IntelID{campRecord}
			}
		}
		enc := s.open(s.compiled.Field, members, withdrawn())
		s.Equal(encounter.StanceHostile, s.stance(enc))
		out := s.intoTheYard(enc, raider)
		s.NotNil(out.Formed, "the fight forms exactly as if the scout knew nothing")
		s.Empty(s.beatsOfKind(enc, partner, "stance"))
	})

	s.Run("the letter carried to the scout turns nothing", func() {
		s.SetupTest()
		enc := s.camp()
		s.hold(enc, raider, campLetter)
		out := s.intoTheYard(enc, raider)
		s.Require().NotNil(out.Formed)
		// Stand beside the scout, in its region, holding the letter.
		s.step(enc, raider, cellAt(5, 2))
		s.Equal(encounter.StanceHostile, s.stance(enc))
		s.Equal(encounter.ClockTurn, s.clockOf(enc, campScout), "the fight goes on")
		s.Empty(s.beatsOfKind(enc, partner, "stance"))
		s.Empty(s.beatsOfKind(enc, partner, "bubble-dissolved"))
	})
}

// TestADeadMindCannotLearn is design §3.9: the chief Down, the letter carried
// in — nothing. A consequence, not a loss.
func (s *HoldOutSuite) TestADeadMindCannotLearn() {
	enc := s.camp(s.holdOutEnding())
	s.hold(enc, raider, campLetter)
	s.standing.down = []encounter.MemberID{campChief}
	s.intoTheYard(enc, raider)
	out := s.intoTheHut(enc, raider)

	s.Nil(out.Outcome, "the camp cannot turn")
	s.Equal(encounter.StanceHostile, s.stance(enc))
	s.Empty(s.beatsOfKind(enc, partner, "stance"))
}

// TestAFactionOfOneHasItsMemberAsMind is design §2's rule: no mind declared,
// one skeleton in the camp — the COMPILER declares it the mind, and the
// letter carried to it turns the camp. The run itself never infers a mind
// (R7): the same field with the mind blanked by hand is a faction that
// cannot learn.
func (s *HoldOutSuite) TestAFactionOfOneHasItsMemberAsMind() {
	raw, err := os.ReadFile(campPath)
	s.Require().NoError(err)
	source := strings.Replace(string(raw), "  - { id: raiders, mind: chief }", "  - { id: raiders }", 1)
	scoutLine := `  - { id: scout,  ref: "dnd5e:monsters:skeleton",         at: [4,2],  faction: raiders }` + "\n"
	s.Require().Contains(source, scoutLine)
	source = strings.Replace(source, scoutLine, "", 1)

	s.Run("the compiler declares the sole member as the mind", func() {
		compiled, err := dungeonspec.Load([]byte(source))
		s.Require().NoError(err)
		s.Require().Equal([]encounter.FactionInput{{ID: campFaction, Mind: campChief}}, compiled.Factions)

		s.compiled = compiled
		enc := s.open(compiled.Field, s.cast(true), withdrawn())
		s.hold(enc, raider, campLetter)
		s.intoTheYard(enc, raider)
		s.intoTheHut(enc, raider)
		s.Equal(encounter.StanceNeutral, s.stance(enc))
		s.Len(s.beatsOfKind(enc, partner, "stance"), 1)
	})

	s.Run("the run infers nothing: a faction with no declared mind cannot learn", func() {
		s.SetupTest()
		field := s.compiled.Field
		field.Factions = []encounter.FactionInput{{ID: campFaction}}
		enc := s.open(field, s.cast(false), withdrawn())
		s.hold(enc, raider, campLetter)
		s.intoTheYard(enc, raider)
		s.intoTheHut(enc, raider)
		s.Equal(encounter.StanceHostile, s.stance(enc))
		s.Empty(s.beatsOfKind(enc, partner, "stance"))
	})
}

// TestTheStanceIsDerivedNotStored is A9: save after the flip, load: still
// neutral, and nothing in the blob but the facts says so — the declared
// disposition still reads hostile, the fold reads neutral.
func (s *HoldOutSuite) TestTheStanceIsDerivedNotStored() {
	enc := s.camp()
	s.hold(enc, raider, campLetter)
	s.intoTheYard(enc, raider)
	s.intoTheHut(enc, raider)
	s.Require().Equal(encounter.StanceNeutral, s.stance(enc))

	data := enc.ToData()
	s.Run("the declaration is untouched and no stance is written", func() {
		s.Require().Len(data.Field.Dispositions, 1)
		s.Equal(string(encounter.StanceHostile), data.Field.Dispositions[0].Stance)
		structure, err := json.Marshal(struct {
			Field    encounter.FieldData
			Members  []encounter.MemberData
			World    *encounter.WorldData
			Holdings *encounter.HoldingsData
		}{data.Field, data.Members, data.World, data.Holdings})
		s.Require().NoError(err)
		s.NotContains(string(structure), `"neutral"`, "the only place neutral appears is the story's own beat")
	})

	back := s.reload(enc)
	s.Run("the fold after load says neutral", func() {
		s.Equal(encounter.StanceNeutral, s.stance(back))
		hostile, known := back.IsHostile(raider, campScout)
		s.True(known)
		s.False(hostile)
	})

	s.Run("save, load, save is byte-identical", func() {
		first, err := json.Marshal(data)
		s.Require().NoError(err)
		second, err := json.Marshal(back.ToData())
		s.Require().NoError(err)
		s.Equal(string(first), string(second))
	})
}

// TestTheGraphAnswersHostileAndAlliedForResolution is A8's encounter half:
// the read resolution's cast will make, before and after the flip.
func (s *HoldOutSuite) TestTheGraphAnswersHostileAndAlliedForResolution() {
	enc := s.camp()

	ask := func(a, b core.EntityID) (bool, bool) {
		hostile, hk := enc.IsHostile(a, b)
		allied, ak := enc.IsAllied(a, b)
		s.Require().True(hk)
		s.Require().True(ak)
		return hostile, allied
	}
	s.Run("before: the camp is the party's enemy and its own ally", func() {
		hostile, allied := ask(raider, campScout)
		s.True(hostile)
		s.False(allied)
		hostile, allied = ask(raider, partner)
		s.False(hostile)
		s.True(allied)
		hostile, allied = ask(campScout, campChief)
		s.False(hostile)
		s.True(allied)
	})
	s.Run("somebody who is not here is not an answer", func() {
		_, known := enc.IsHostile(raider, "a-ghost")
		s.False(known)
		_, known = enc.IsAllied("a-ghost", raider)
		s.False(known)
	})

	s.hold(enc, raider, campLetter)
	s.intoTheYard(enc, raider)
	s.intoTheHut(enc, raider)
	s.Run("after: neither enemy nor ally — neutral is not alliance", func() {
		hostile, allied := ask(raider, campScout)
		s.False(hostile)
		s.False(allied)
		hostile, allied = ask(campScout, campChief)
		s.False(hostile)
		s.True(allied, "the camp is still its own")
	})
}

// TestAFactEndingFiresAtTheAppend is design §3.8's fact site, on the truth
// grain: anyone learning the fact ends the run, the raider reading the
// letter included.
func (s *HoldOutSuite) TestAFactEndingFiresAtTheAppend() {
	enc := s.camp(encounter.EndingInput{Key: "word-is-out", Trigger: encounter.TriggerFact{Fact: campFact}})
	s.hold(enc, raider, campLetter)
	status, err := enc.Status()
	s.Require().NoError(err)
	s.False(status.Open)
	s.Equal("word-is-out", status.Outcome.Ending)
}

// TestARoundEndingFiresWhereTheRoundStarts is design §3.8's round site, R9:
// the fight's own clock, never the world's — a run with no fight never
// reaches round 2 however many ticks pass.
func (s *HoldOutSuite) TestARoundEndingFiresWhereTheRoundStarts() {
	enc := s.camp(encounter.EndingInput{Key: "held-out", Trigger: encounter.TriggerRound{Round: 2}})

	s.Run("outside any fight the world clock counts nothing", func() {
		for i := 0; i < 3; i++ {
			_, err := enc.Pump(&encounter.PumpInput{})
			s.Require().NoError(err)
		}
		status, err := enc.Status()
		s.Require().NoError(err)
		s.True(status.Open)
	})

	out := s.intoTheYard(enc, raider)
	s.Require().NotNil(out.Formed)
	s.Require().Equal(raider, out.Formed.Order[0], "the raider acts first")

	s.Run("the round wraps and the ending fires", func() {
		_, err := enc.EndTurn(&encounter.EndTurnInput{Member: raider})
		s.Require().NoError(err)
		status, err := enc.Status()
		s.Require().NoError(err)
		s.False(status.Open, "round 2 started when the order wrapped: %v", s.heard.entries)
		s.Equal("held-out", status.Outcome.Ending)
	})
}

// TestMonstersSpawnedThroughJoinCarryTheirFaction is the host's own path
// (design §3 Spawn): the world is built empty and each skeleton arrives through
// Join with its faction hand-carried — and the mind must arrive in its own
// faction.
func (s *HoldOutSuite) TestMonstersSpawnedThroughJoinCarryTheirFaction() {
	enc := s.open(s.compiled.Field, s.cast(true)[:2], withdrawn())

	s.Run("the mind must arrive in its faction", func() {
		_, err := enc.Join(&encounter.JoinInput{
			Member: campChief, Kind: encounter.KindMonster, Cell: cellAt(12, 4),
		})
		s.Require().ErrorIs(err, encounter.ErrNoFaction)
		s.Contains(err.Error(), "is the mind of faction")
	})
	s.Run("a faction the field does not declare is refused", func() {
		_, err := enc.Join(&encounter.JoinInput{
			Member: campScout, Kind: encounter.KindMonster, Cell: cellAt(4, 2), Faction: "kobolds",
		})
		s.Require().ErrorIs(err, encounter.ErrNoFaction)
	})

	for _, m := range s.compiled.Monsters {
		_, err := enc.Join(&encounter.JoinInput{
			Member: core.EntityID(m.ID), Kind: encounter.KindMonster, Cell: cellAt(int(m.At.X), int(m.At.Y)),
			Faction: m.Faction, Holds: m.Holds,
		})
		s.Require().NoError(err)
	}
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		switch m.ID {
		case raider, partner:
			s.Equal(encounter.FactionParty, m.Faction)
		default:
			s.Equal(campFaction, m.Faction, m.ID)
		}
	}

	out := s.intoTheYard(enc, raider)
	s.NotNil(out.Formed, "the joined skeletons are the camp")
}

// TestLoadRefusesKnowledgeThisFieldCannotMint is the trust boundary for the
// new fact kind, in the shape TestABlobWorldMustMatchItsField established.
func (s *HoldOutSuite) TestLoadRefusesKnowledgeThisFieldCannotMint() {
	enc := s.camp()
	s.hold(enc, raider, campLetter)
	base := enc.ToData()
	s.Require().NotNil(base.World)

	load := func(data encounter.EncounterData) error {
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data:  data,
			Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		})
		return err
	}
	withFact := func(f encounter.FactData) encounter.EncounterData {
		data := enc.ToData()
		data.World.Facts = append(append([]encounter.FactData(nil), data.World.Facts...), f)
		return data
	}
	honest := encounter.FactData{
		Kind: "known:fact:" + campFact, Actor: string(campScout), Subject: string(campScout),
		Audience: []string{string(campScout)},
	}

	s.Run("the honest shape loads", func() {
		s.Require().NoError(load(withFact(honest)))
	})
	s.Run("a fact this field never mentions", func() {
		f := honest
		f.Kind = "known:fact:some-other-dungeons-secret"
		err := load(withFact(f))
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Contains(err.Error(), "does not mint")
	})
	s.Run("a learner who is not the subject", func() {
		f := honest
		f.Subject = "fact:" + campFact
		err := load(withFact(f))
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Contains(err.Error(), "does not match its kind")
	})
	s.Run("a member in a faction the field does not have", func() {
		data := enc.ToData()
		for i := range data.Members {
			if data.Members[i].ID == campScout {
				data.Members[i].Faction = "kobolds"
			}
		}
		err := load(data)
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Require().ErrorIs(err, encounter.ErrNoFaction)
	})
}

// TestAPlainDungeonWritesNoSides is the byte-identity claim for this slice:
// a dungeon that declares no faction writes no faction, no disposition and
// no member faction — the exact bytes every pre-faction blob already has —
// and reads every side exactly as it did.
func (s *HoldOutSuite) TestAPlainDungeonWritesNoSides() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field: heirloomField(),
		Members: []encounter.MemberInput{
			{ID: raider, Kind: encounter.KindPlayer, Position: raiderCell},
			{ID: captain, Kind: encounter.KindMonster, Position: captainCell},
		},
		Endings: []encounter.EndingInput{withdrawn()},
	})
	s.Require().NoError(err)

	blob, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	for _, key := range []string{`"factions"`, `"dispositions"`, `"faction"`, `"known:fact`} {
		s.NotContains(string(blob), key)
	}

	hostile, known := enc.IsHostile(raider, captain)
	s.True(known)
	s.True(hostile, "party and monsters, as they always were")
	s.Equal(encounter.FactionParty, mustMember(s.T(), enc, raider).Faction)
	s.Equal(encounter.FactionMonsters, mustMember(s.T(), enc, captain).Faction)
}

// TestTheRunRefusesWhatItCannotKeep is the construction-time liveness for the
// new forms: an until the graph cannot settle, a stance nobody can reach, a
// round that never starts.
func (s *HoldOutSuite) TestTheRunRefusesWhatItCannotKeep() {
	open := func(field encounter.FieldInput, endings ...encounter.EndingInput) error {
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
			Field: field, Members: s.cast(true), Endings: append([]encounter.EndingInput{withdrawn()}, endings...),
		})
		return err
	}
	withUntil := func(t encounter.Trigger) encounter.FieldInput {
		field := s.compiled.Field
		field.Dispositions = []encounter.DispositionInput{{
			Between: [2]encounter.FactionID{campFaction, encounter.FactionParty},
			Stance:  encounter.StanceHostile, Until: t,
		}}
		return field
	}

	s.Run("an until on a member down cannot be settled by the graph yet", func() {
		err := open(withUntil(encounter.TriggerMemberDown{Member: campChief}))
		s.Require().ErrorIs(err, encounter.ErrNoFaction)
		s.Contains(err.Error(), "turns only on a fact")
	})
	s.Run("a stance ending on a pair that can never reach it", func() {
		err := open(s.compiled.Field, encounter.EndingInput{Key: "peace", Trigger: encounter.TriggerStance{
			Between: [2]encounter.FactionID{campFaction, encounter.FactionParty}, Stance: encounter.StanceAllied,
		}})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
		s.Contains(err.Error(), "can never be")
	})
	s.Run("a stance ending on a stance the pair holds from the start", func() {
		err := open(s.compiled.Field, encounter.EndingInput{Key: "war", Trigger: encounter.TriggerStance{
			Between: [2]encounter.FactionID{campFaction, encounter.FactionParty}, Stance: encounter.StanceHostile,
		}})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
		s.Contains(err.Error(), "from the start")
	})
	s.Run("a round that never starts", func() {
		err := open(s.compiled.Field, encounter.EndingInput{Key: "never", Trigger: encounter.TriggerRound{Round: 0}})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})
	s.Run("a fact ending naming no fact", func() {
		err := open(s.compiled.Field, encounter.EndingInput{Key: "blank", Trigger: encounter.TriggerFact{}})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
	})
	s.Run("the fixture's own declaration is accepted", func() {
		s.Require().NoError(open(s.compiled.Field))
	})
}

// TestAThirdFactionStillHostileKeepsItsFight is design §3.5's second
// sentence, on a hand-built yard: raiders and kobolds both hostile to the
// party, the raiders hostile until the fact. The letter turns the raiders;
// the kobold keeps fighting; the lone raider, opposed to nobody, steps out
// of the fight.
func (s *HoldOutSuite) TestAThirdFactionStillHostileKeepsItsFight() {
	const (
		alice  = core.EntityID("alice")
		lone   = core.EntityID("lone-raider")
		kobold = core.EntityID("kobold")
		scroll = "yard-scroll"
	)
	field := encounter.FieldInput{
		Canvas:  openAir(),
		Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 6)},
		Intel:   []encounter.IntelRecord{{ID: "letter", Reveals: encounter.RevealTargets{Fact: campFact}}},
		Props: []encounter.PropInput{func() encounter.PropInput {
			p := holdableProp(scroll, "dnd5e:props:scroll", spatial.Position{X: 1, Y: 1})
			p.Holds = []encounter.IntelID{"letter"}
			return p
		}()},
		Factions: []encounter.FactionInput{{ID: campFaction, Mind: lone}, {ID: "kobolds"}},
		Dispositions: []encounter.DispositionInput{{
			Between: [2]encounter.FactionID{campFaction, encounter.FactionParty},
			Stance:  encounter.StanceHostile, Until: encounter.TriggerFact{Fact: campFact},
		}},
	}
	enc := s.open(field, []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
		{ID: lone, Kind: encounter.KindMonster, Position: spatial.Position{X: 4, Y: 1}, Faction: campFaction},
		{ID: kobold, Kind: encounter.KindMonster, Position: spatial.Position{X: 4, Y: 4}, Faction: "kobolds"},
	}, withdrawn())

	s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, alice), "precondition: everyone saw everyone at first light")
	s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, lone))
	s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, kobold))

	// alice acts first (sorted order); the scroll is beside her, in the one
	// region the lone raider — the camp's declared mind — stands in. Holding
	// it is the presence that teaches him.
	s.hold(enc, alice, scroll)

	s.Run("the raiders turned; the kobolds did not", func() {
		stance, err := enc.Stance(campFaction, encounter.FactionParty)
		s.Require().NoError(err)
		s.Equal(encounter.StanceNeutral, stance)
		stance, err = enc.Stance("kobolds", encounter.FactionParty)
		s.Require().NoError(err)
		s.Equal(encounter.StanceHostile, stance)
	})
	s.Run("the fight goes on without the raider", func() {
		s.Equal(encounter.ClockTurn, s.clockOf(enc, alice))
		s.Equal(encounter.ClockTurn, s.clockOf(enc, kobold))
		s.Equal(encounter.ClockWorld, s.clockOf(enc, lone), "opposed to nobody, in no pair, in no fight")
		s.Empty(s.beatsOfKind(enc, alice, "bubble-dissolved"))
		s.Len(s.beatsOfKind(enc, alice, "stance"), 1)
	})
}

// mustMember reads one member's roster row.
func mustMember(t *testing.T, enc *encounter.Encounter, id core.EntityID) encounter.Member {
	t.Helper()
	members, err := enc.Members()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("no member %s", id)
	return encounter.Member{}
}
