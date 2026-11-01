// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// survivors_test.go is WHEN A SIDE IS BEATEN, ITS SURVIVORS CAN BE SPOKEN TO
// (rpg-project#718, Frank's list S4): the band's leader falls, its `until`
// turns the pair neutral, the fight dissolves, and any surviving follower
// speaks the lines the FACTION wrote — the leader's fall does not take them
// down with him.
//
// AND THE WAY BACK IS PROVOCATION (lane rpg-project#722, D4): no list of
// `until` steps. A survivor who refuses keeps the pair neutral, and the first
// blow either way turns it hostile and the fight forms again.
//
// THE DOCUMENT IS COMPILED, not hand-built, because the claim is about the
// file: the lines are authored once on the faction, and what a host hands the
// encounter is what [dungeonspec.Load] laid onto each member.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// The band, the watch, and the hero, as the document names them.
const (
	svBand  = encounter.FactionID("band")
	svWatch = encounter.FactionID("watch")

	svYielded = "band-yielded"
)

var (
	svHero      = core.EntityID("hero")
	svLeader    = core.EntityID("leader")
	svFollower1 = core.EntityID("follower-1")
	svFollower2 = core.EntityID("follower-2")
	svWatchman  = core.EntityID("watchman")
)

// survivorsDoc is the band of three hostile to the party until its leader
// falls, and the faction's two lines.
const survivorsDoc = `
version: 5
key: survivors
play: { void: transparent, lighting: bright, standing: centre-covered }
factions:
  - id: band
    mind: follower-1
    options:
      - id: yield
        say: Yield and live.
        check: [{ ability: persuasion, dc: 10 }]
        success: [{ say: We yield., fact: band-yielded }]
      - id: refuse
        say: Throw down your arms.
        check: [{ ability: intimidation, dc: 15 }]
        failure: [{ say: We fight on. }]
%s
dispositions:
  - { between: [band, party], stance: hostile, until: { down: leader } }
%s
room:
  version: 3
  id: yard
  name: The Yard
  coordinateFrame: { hexRadius: 1 }
  scene: { version: 1, id: yard-scene, name: The Yard, items: [], groups: [] }
  room:
    implicitRegionId: yard-region
    regions:
      - id: yard-region
        name: The Yard
        lighting: { intensity: 1 }
        cells:
          - r: 0
            q: [[0, 4]]
          - r: 1
            q: [[0, 4]]
    propDeclarations: {}
    arrangementDeclarations: {}
    partyStart: { q: 0, r: 0 }
    monsterDeclarations:
      - { id: leader, ref: 'dnd5e:monsters:goblin', startingCell: { location: { q: 1, r: 0 } } }
      - { id: follower-1, ref: 'dnd5e:monsters:goblin', startingCell: { location: { q: 2, r: 0 } } }
      - { id: follower-2, ref: 'dnd5e:monsters:goblin', startingCell: { location: { q: 1, r: 1 } } }
%s
    monsterBindings:
      leader: { faction: band }
      follower-1: { faction: band }
      follower-2: { faction: band }
%s
`

// theWatch is the four holes [survivorsDoc] leaves for a second side: a
// watch the band is at odds with until the watch hears it yield — the second
// disposition a yield can fold. Neutral to the party, so the hero is in no
// fight of the watch's.
var theWatch = []any{
	"  - id: watch\n    mind: watchman",
	"  - { between: [watch, party], stance: neutral }\n" +
		"  - { between: [band, watch], stance: hostile, until: { fact: band-yielded } }",
	"      - { id: watchman, ref: 'dnd5e:monsters:goblin', startingCell: { location: { q: 4, r: 1 } } }",
	"      watchman: { faction: watch }",
}

// SurvivorsSuite is the band, its fall, and what is said after.
type SurvivorsSuite struct {
	suite.Suite

	ctx      context.Context
	compiled dungeonspec.Compiled
	standing *downList
}

func TestSurvivorsSuite(t *testing.T) { suite.Run(t, new(SurvivorsSuite)) }

func (s *SurvivorsSuite) SetupTest() {
	s.ctx = context.Background()
	s.load("", "", "", "")
	s.standing = &downList{}
}

// load compiles the yard with [survivorsDoc]'s four holes filled.
func (s *SurvivorsSuite) load(holes ...any) {
	compiled, err := dungeonspec.Load([]byte(fmt.Sprintf(survivorsDoc, holes...)))
	s.Require().NoError(err, "the survivors' yard compiles")
	s.compiled = compiled
}

// open seats the hero at the party's first seat and every creature where the
// file posts it, carrying the lines the compile laid onto it.
func (s *SurvivorsSuite) open() *encounter.Encounter {
	members := []encounter.MemberInput{{ID: svHero, Kind: encounter.KindPlayer, Position: s.compiled.PartyStart[0].At}}
	for _, m := range s.compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: core.EntityID(m.ID), Kind: encounter.KindMonster, Position: m.At,
			Faction: m.Faction, Options: m.Options,
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:     s.compiled.Field,
		Members:   members,
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: s.standing,
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			CheckResolver: findsNothing{},
			Witness:       nobodyPerceives{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc
}

// strike records one blow, and when drops is set the target is down before
// the record asks the world who is standing.
func (s *SurvivorsSuite) strike(enc *encounter.Encounter, actor, target core.EntityID, drops bool) {
	if drops {
		s.standing.down = append(s.standing.down, target)
	}
	_, err := recordOne(enc, &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: actor, Targets: []encounter.MemberID{target},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 3},
	})
	s.Require().NoError(err)
}

// say is the hero speaking one line to one creature with the check decided.
func (s *SurvivorsSuite) say(
	enc *encounter.Encounter, target core.EntityID, option, skill string, beaten bool,
) (*encounter.SpeakOutput, error) {
	return enc.Speak(s.ctx, &encounter.SpeakInput{
		Range: speakReach,
		Actor: svHero, Target: target, OptionID: option, Skill: skill,
		Beaten: beaten, DC: 10, Total: map[bool]int{true: 18, false: 4}[beaten], Roller: rollsLowest{},
	})
}

func (s *SurvivorsSuite) stance(enc *encounter.Encounter, a, b encounter.FactionID) encounter.Stance {
	out, err := enc.Stance(a, b)
	s.Require().NoError(err)

	return out
}

func (s *SurvivorsSuite) clock(enc *encounter.Encounter, member core.EntityID) encounter.ClockKind {
	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: member})
	s.Require().NoError(err)

	return out.Kind
}

// knows reports whether a member holds a fact, read off the journal a host
// persists.
func (s *SurvivorsSuite) knows(enc *encounter.Encounter, member core.EntityID, fact string) bool {
	for _, f := range enc.ToData().Journal.Facts {
		if f.Kind == "known:fact:"+fact && core.EntityID(f.Subject) == member {
			return true
		}
	}

	return false
}

// toldStances is every stance the hero was told about the band and the
// party, in story order.
func (s *SurvivorsSuite) toldStances(enc *encounter.Encounter) []string {
	story, err := enc.Story(&encounter.StoryInput{Audience: svHero})
	s.Require().NoError(err)
	var out []string
	for _, beat := range story {
		var body map[string]any
		s.Require().NoError(json.Unmarshal(beat.Payload, &body))
		if body["beat"] != "stance" {
			continue
		}
		between, _ := body["between"].([]any)
		if len(between) == 2 && between[0] == any(string(svBand)) && between[1] == any(encounter.FactionParty) {
			out = append(out, body["stance"].(string))
		}
	}

	return out
}

// beaten is the band's leader down, the pair stood down and the fight gone:
// the precondition every survivor scene starts from.
func (s *SurvivorsSuite) beaten() *encounter.Encounter {
	enc := s.open()
	s.Require().Equal(encounter.StanceHostile, s.stance(enc, svBand, encounter.FactionParty))
	s.Require().Equal(encounter.ClockTurn, s.clock(enc, svHero), "precondition: the band and the hero are fighting")

	s.strike(enc, svHero, svLeader, true)

	return enc
}

// story is every beat one member was told, in order.
func (s *SurvivorsSuite) story(enc *encounter.Encounter, member core.EntityID) []record.Entry {
	out, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)

	return out
}

// heldPersuade reports whether an observer holds the hero's persuade deed:
// the deed a beaten Persuasion line lands on its witnesses.
func (s *SurvivorsSuite) heldPersuade(enc *encounter.Encounter, observer core.EntityID) bool {
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range holdings {
		if h.Channel == deed.Channel && h.Subject == deed.Subject(svHero, encounter.DeedPersuade) {
			return true
		}
	}

	return false
}

// ids is the ids of a list of lines, in order.
func (s *SurvivorsSuite) ids(options []encounter.Option) []string {
	out := make([]string, 0, len(options))
	for _, o := range options {
		out = append(out, o.ID)
	}

	return out
}

// TestTheLeadersFallEndsTheFight is the hook the slice stands on: the
// `until: { down: leader }` turns the pair neutral and a fight with no
// hostile sides left dissolves.
func (s *SurvivorsSuite) TestTheLeadersFallEndsTheFight() {
	enc := s.beaten()

	s.Equal(encounter.StanceNeutral, s.stance(enc, svBand, encounter.FactionParty))
	s.Equal([]string{string(encounter.StanceNeutral)}, s.toldStances(enc), "the hero is told the band stood down")
	for _, id := range []core.EntityID{svHero, svFollower1, svFollower2} {
		s.Equal(encounter.ClockWorld, s.clock(enc, id), "%s is back on the world clock", id)
	}
}

// TestASurvivorSpeaksTheFactionsLines: every surviving follower offers the
// lines the faction wrote, though none wrote a line of its own.
func (s *SurvivorsSuite) TestASurvivorSpeaksTheFactionsLines() {
	enc := s.beaten()

	for _, id := range []core.EntityID{svFollower1, svFollower2} {
		offered, err := enc.OptionsFor(svHero, id)
		s.Require().NoError(err)
		s.Equal([]string{"yield", "refuse"}, s.ids(offered), "%s speaks the band's lines", id)
	}
}

// TestAYieldResolvesAndNobodySwings: the yield said to a survivor and beaten
// answers from the faction's success row, teaches its fact, and the band
// stays neutral to the party with no fight — nothing further to author.
func (s *SurvivorsSuite) TestAYieldResolvesAndNobodySwings() {
	enc := s.beaten()

	out, err := s.say(enc, svFollower2, "yield", "persuasion", true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Answered)
	s.Equal("We yield.", out.Answered.Answer.Say)
	s.True(s.knows(enc, svFollower2, svYielded), "the survivor addressed learns what it said")
	s.Equal(encounter.StanceNeutral, s.stance(enc, svBand, encounter.FactionParty))
	s.Equal([]string{string(encounter.StanceNeutral)}, s.toldStances(enc),
		"the yield turns the band and the party nothing further")
	s.Equal(encounter.ClockWorld, s.clock(enc, svHero), "nobody swings")
	s.Equal(encounter.ClockWorld, s.clock(enc, svFollower2))
}

// TestAYieldFoldsASecondDisposition: the success row's fact is a fact like
// any other, so a second pair waiting on it — the band and the watch —
// turns when the faction's mind learns it.
//
// THE BAND'S MIND IS A FOLLOWER here, and that is load-bearing: a fact
// `until` is settled by the faction's mind, and a mind that fell with the
// leader learns nothing (hold-out, TestADeadMindCannotLearn).
func (s *SurvivorsSuite) TestAYieldFoldsASecondDisposition() {
	s.load(theWatch...)
	enc := s.beaten()
	s.Require().Equal(encounter.StanceHostile, s.stance(enc, svBand, svWatch), "precondition: the band and the watch are at odds")
	s.Require().Equal(encounter.ClockWorld, s.clock(enc, svHero), "precondition: the hero's fight is over")
	s.Require().Equal(encounter.ClockTurn, s.clock(enc, svFollower1), "precondition: the band's fight with the watch is not")

	_, err := s.say(enc, svFollower1, "yield", "persuasion", true)
	s.Require().NoError(err)

	s.Equal(encounter.StanceNeutral, s.stance(enc, svBand, svWatch),
		"the band's mind said it yielded, and the band and the watch stand down")
	s.Equal(encounter.ClockWorld, s.clock(enc, svFollower1), "and that fight dissolves too")
	s.Equal(encounter.ClockWorld, s.clock(enc, svWatchman))
}

// TestARefusalThenABlowTurnsThePairHostileAgain is D4: the refuse line
// answers from its failure row and changes nothing, and the first blow either
// way across the neutral pair turns it hostile by provocation and the fight
// forms again — no `until` list.
func (s *SurvivorsSuite) TestARefusalThenABlowTurnsThePairHostileAgain() {
	for _, blow := range []struct {
		name          string
		actor, target core.EntityID
	}{
		{name: "the hero strikes the survivor", actor: svHero, target: svFollower1},
		{name: "the survivor strikes the hero", actor: svFollower1, target: svHero},
	} {
		s.Run(blow.name, func() {
			s.standing = &downList{}
			enc := s.beaten()

			out, err := s.say(enc, svFollower1, "refuse", "intimidation", false)
			s.Require().NoError(err)
			s.Require().NotNil(out.Answered)
			s.Equal("We fight on.", out.Answered.Answer.Say)
			s.Empty(out.Deed, "a refusal lands no deed")
			s.Equal(encounter.StanceNeutral, s.stance(enc, svBand, encounter.FactionParty), "a word turns nothing")
			s.Equal(encounter.ClockWorld, s.clock(enc, svHero))

			s.strike(enc, blow.actor, blow.target, false)

			s.Equal(encounter.StanceHostile, s.stance(enc, svBand, encounter.FactionParty),
				"the leader is still down, and it no longer matters")
			s.Equal([]string{string(encounter.StanceNeutral), string(encounter.StanceHostile)}, s.toldStances(enc),
				"the band stood down, and the truce broke")
			s.Equal(encounter.ClockTurn, s.clock(enc, svHero), "the fight re-forms")
			s.Equal(encounter.ClockTurn, s.clock(enc, svFollower1))
		})
	}
}

// TestSpeakingToADownMemberIsRefused: the fallen leader carries the faction's
// lines like every member, and a line said to it is refused with a sentence,
// before anything is written.
func (s *SurvivorsSuite) TestSpeakingToADownMemberIsRefused() {
	enc := s.beaten()
	facts := len(enc.ToData().Journal.Facts)
	beats := map[core.EntityID]int{}
	for _, id := range []core.EntityID{svHero, svLeader, svFollower1, svFollower2} {
		beats[id] = len(s.story(enc, id))
	}

	_, err := s.say(enc, svLeader, "yield", "persuasion", true)

	s.Require().ErrorIs(err, encounter.ErrMemberDown)
	s.Contains(err.Error(), `"leader" is down and cannot answer`)
	s.Len(enc.ToData().Journal.Facts, facts, "a refused line teaches nothing")
	for id, n := range beats {
		s.Len(s.story(enc, id), n, "%s is told no beat: no spoke, no answered", id)
		s.False(s.heldPersuade(enc, id), "%s holds no persuade deed about the hero", id)
	}
	offered, err := enc.OptionsFor(svHero, svLeader)
	s.Require().NoError(err)
	s.Empty(offered, "and a down member is offered as speaking nothing")
}
