// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// bothways_test.go is DISPOSITIONS TURN BOTH WAYS (rpg-project#493,
// ideas/living-world/disposition/both-ways.md): the scenes for the direction
// that did not exist — a neutral pair becoming hostile — and for the law that
// turns one with nothing authored at all.
//
//	R1  an `until` on a NEUTRAL pair turns it hostile; on allied it is refused
//	R2  an `until` takes a fact, a fall, a round and another pair's stance
//	R3  attacking across a neutral pair turns it hostile, and the fight forms
//	R4  a world NPC is not a target, and refusing turns nothing
//
// The hostile-to-neutral direction is holdout_test.go's, unchanged, and the
// scenes there are the proof that it stayed that way.
//
// THE YARD is deliberately plainer than the raider camp: one open region,
// everybody in sight of everybody at first light, no doors and no intel
// unless the scene needs some. What is being tested is the stance table and
// what reads it, and geometry that can surprise a reader is geometry that
// makes a failure ambiguous.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	bwGoblins = "goblins"
	bwDogs    = "dogs"
	bwKobolds = "kobolds"
	bwFact    = "we-saved-the-wiseman"
	bwScroll  = encounter.PropID("yard-scroll")
)

var (
	bwChief  = core.EntityID("chief")
	bwScout  = core.EntityID("scout")
	bwDog    = core.EntityID("dog")
	bwKobold = core.EntityID("kobold")
	bwVendor = core.EntityID("vendor")
)

type BothWaysSuite struct {
	suite.Suite

	standing *downList
}

func TestBothWaysSuite(t *testing.T) { suite.Run(t, new(BothWaysSuite)) }

func (s *BothWaysSuite) SetupTest() { s.standing = &downList{} }

// yard is the field every scene here opens on: one region, whatever factions
// and dispositions the scene declares, and the scroll only when it is asked
// for.
func (s *BothWaysSuite) yard(
	factions []encounter.FactionInput, dispositions []encounter.DispositionInput, withScroll bool,
) encounter.FieldInput {
	field := encounter.FieldInput{
		Canvas: openAir(),
		// TWO REGIONS, and the hut is not decoration: presence transfer is
		// per REGION (design §3.6), so a one-room yard would teach every
		// faction mind standing in it whatever anybody picked up, and the
		// scene about a scout reading a letter alone would be impossible to
		// write. The hut is where a mind stands to be out of earshot.
		Regions: []encounter.RegionInput{
			rectRegion("yard", 0, 0, 6, 6),
			rectRegion("hut", 10, 0, 3, 3),
		},
		Factions:     factions,
		Dispositions: dispositions,
	}
	if withScroll {
		field.Intel = []encounter.IntelRecord{{ID: "letter", Reveals: encounter.RevealTargets{Fact: bwFact}}}
		prop := holdableProp(bwScroll, "dnd5e:props:scroll", spatial.Position{X: 1, Y: 1})
		prop.Holds = []encounter.IntelID{"letter"}
		field.Props = []encounter.PropInput{prop}
	}
	return field
}

func (s *BothWaysSuite) open(
	field encounter.FieldInput, members []encounter.MemberInput,
) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field: field, Members: members, Endings: []encounter.EndingInput{withdrawn()},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	return enc
}

// neutralUntil is the disposition every R1/R2 scene is about: the goblins
// civil to the party until something happens.
func neutralUntil(t encounter.Trigger) encounter.DispositionInput {
	return encounter.DispositionInput{
		Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
		Stance:  encounter.StanceNeutral, Until: t,
	}
}

func player(id core.EntityID, x, y float64) encounter.MemberInput {
	return encounter.MemberInput{ID: id, Kind: encounter.KindPlayer, Position: spatial.Position{X: x, Y: y}}
}

func monster(id core.EntityID, faction encounter.FactionID, x, y float64) encounter.MemberInput {
	return encounter.MemberInput{
		ID: id, Kind: encounter.KindMonster, Faction: faction, Position: spatial.Position{X: x, Y: y},
	}
}

func (s *BothWaysSuite) stance(enc *encounter.Encounter, a, b encounter.FactionID) encounter.Stance {
	out, err := enc.Stance(a, b)
	s.Require().NoError(err)
	return out
}

// stanceBeats is every stance beat one member was told, in order — read by
// CONTENT rather than counted, so a scene that says "the camp turned once,
// and here is why" fails on the wrong reason rather than on a bumped number.
func (s *BothWaysSuite) stanceBeats(enc *encounter.Encounter, member core.EntityID) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]map[string]any, 0)
	for _, beat := range story {
		var body map[string]any
		s.Require().NoError(json.Unmarshal(beat.Payload, &body))
		if body["beat"] == "stance" {
			out = append(out, body)
		}
	}
	return out
}

// turnedTo is the one stance beat about a pair, and the scene fails if there
// is not exactly one: a pair turns once, and a second beat saying the same
// thing is a defect this suite exists to catch.
func (s *BothWaysSuite) turnedTo(
	enc *encounter.Encounter, member core.EntityID, a, b encounter.FactionID,
) map[string]any {
	var found map[string]any
	for _, beat := range s.stanceBeats(enc, member) {
		between, ok := beat["between"].([]any)
		s.Require().True(ok, "a stance beat names its pair")
		if len(between) != 2 || between[0] != any(a) || between[1] != any(b) {
			continue
		}
		s.Require().Nil(found, "%s and %s turned twice", a, b)
		found = beat
	}
	s.Require().NotNil(found, "%s and %s never turned", a, b)
	return found
}

func (s *BothWaysSuite) clockOf(enc *encounter.Encounter, member core.EntityID) encounter.ClockKind {
	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: member})
	s.Require().NoError(err)
	return out.Kind
}

// TestANeutralPairTurnsHostileOnAFactTheMindLearns is R1 and R2's fact form
// in the new direction: the camp is civil until its chief finds out what the
// party did, and then it is not.
//
// THE GRAIN IS THE SAME AS THE HOLD-OUT'S, which is the claim worth making —
// the scout reading the same letter changes nothing, exactly as it changes
// nothing when the camp is standing down instead of standing up.
func (s *BothWaysSuite) TestANeutralPairTurnsHostileOnAFactTheMindLearns() {
	field := s.yard(
		[]encounter.FactionInput{{ID: bwGoblins, Mind: bwChief}},
		[]encounter.DispositionInput{neutralUntil(encounter.TriggerFact{Fact: bwFact})},
		true,
	)

	s.Run("a scout reading it is not the camp finding out", func() {
		enc := s.open(field, []encounter.MemberInput{
			player(alice, 0, 1),
			monster(bwChief, bwGoblins, 10, 0),
			monster(bwScout, bwGoblins, 1, 2),
		})
		_, err := enc.Hold(&encounter.HoldInput{Member: bwScout, Target: bwScroll})
		s.Require().NoError(err)

		s.Equal(encounter.StanceNeutral, s.stance(enc, bwGoblins, encounter.FactionParty))
		s.Empty(s.stanceBeats(enc, alice))
	})

	s.Run("the chief finding out turns the camp", func() {
		enc := s.open(field, []encounter.MemberInput{
			player(alice, 0, 1),
			monster(bwChief, bwGoblins, 1, 2),
		})
		s.Require().Equal(encounter.StanceNeutral, s.stance(enc, bwGoblins, encounter.FactionParty),
			"precondition: the camp is civil while nobody knows")
		s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, alice), "precondition: no fight")

		_, err := enc.Hold(&encounter.HoldInput{Member: bwChief, Target: bwScroll})
		s.Require().NoError(err)

		s.Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty))
		hostile, known := enc.IsHostile(alice, bwChief)
		s.True(known)
		s.True(hostile)
		s.Equal(string(encounter.StanceHostile), s.turnedTo(enc, alice, bwGoblins, encounter.FactionParty)["stance"])
		s.Equal(encounter.ClockTurn, s.clockOf(enc, alice), "strangers became enemies in plain sight: a fight")
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bwChief))
	})
}

// TestANeutralPairTurnsHostileOnAFall is R2's `{ down }` form: "the wolves
// are calm until you kill the alpha". Nobody's mind is declared, and nothing
// needs one — a creature falling is the world's own truth.
func (s *BothWaysSuite) TestANeutralPairTurnsHostileOnAFall() {
	enc := s.open(
		s.yard(
			[]encounter.FactionInput{{ID: bwGoblins}},
			[]encounter.DispositionInput{neutralUntil(encounter.TriggerMemberDown{Member: bwScout})},
			false,
		),
		[]encounter.MemberInput{
			player(alice, 0, 1),
			monster(bwScout, bwGoblins, 4, 1),
			monster(bwChief, bwGoblins, 5, 5),
		},
	)
	s.Require().Equal(encounter.StanceNeutral, s.stance(enc, bwGoblins, encounter.FactionParty))

	// The scout falls, and any verb that asks the world who is standing is
	// where the camp finds out. A step is the plainest one there is.
	s.standing.down = []encounter.MemberID{bwScout}
	_, err := enc.Step(&encounter.StepInput{Member: alice, To: spatial.Position{X: 0, Y: 2}})
	s.Require().NoError(err)

	s.Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty))
	beat := s.turnedTo(enc, alice, bwGoblins, encounter.FactionParty)
	s.Equal(string(encounter.StanceHostile), beat["stance"])
	s.Equal("the fall of scout", beat["cause"], "the streamer is told why the camp turned")
}

// TestTheGuardsTurnAtMidnightAndTheirDogsWithThem is R2's `{ round }` and
// `{ stance }` forms in one scene, because the second is only interesting
// with the first underneath it: the goblins turn on round 3, and the dogs
// turn because the goblins did.
//
// A ROUND IS A FIGHT'S OWN CLOCK (design R9), so the yard needs a fight the
// party is already in for the clock to count at all — the kobolds, hostile by
// default because a declared faction that says nothing about `party` is.
func (s *BothWaysSuite) TestTheGuardsTurnAtMidnightAndTheirDogsWithThem() {
	enc := s.open(
		s.yard(
			[]encounter.FactionInput{{ID: bwGoblins}, {ID: bwDogs}, {ID: bwKobolds}},
			[]encounter.DispositionInput{
				neutralUntil(encounter.TriggerRound{Round: 3}),
				{
					Between: [2]encounter.FactionID{bwDogs, encounter.FactionParty},
					Stance:  encounter.StanceNeutral,
					Until: encounter.TriggerStance{
						Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
						Stance:  encounter.StanceHostile,
					},
				},
			},
			false,
		),
		[]encounter.MemberInput{
			player(alice, 0, 1),
			monster(bwKobold, bwKobolds, 4, 4),
			monster(bwChief, bwGoblins, 4, 1),
			monster(bwDog, bwDogs, 5, 1),
		},
	)
	s.Require().Equal(encounter.ClockTurn, s.clockOf(enc, alice), "precondition: the kobold started a fight")
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, bwChief), "precondition: the goblin is not in it")

	s.Run("round two: still civil", func() {
		end, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
		s.Require().NoError(err)
		s.Require().True(end.RoundWrapped)
		s.Equal(encounter.StanceNeutral, s.stance(enc, bwGoblins, encounter.FactionParty))
		s.Equal(encounter.StanceNeutral, s.stance(enc, bwDogs, encounter.FactionParty))
		s.Empty(s.stanceBeats(enc, alice))
	})

	s.Run("round three: both pairs turn, and the cascade is why the dogs did", func() {
		end, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
		s.Require().NoError(err)
		s.Require().True(end.RoundWrapped)

		s.Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty))
		s.Equal(encounter.StanceHostile, s.stance(enc, bwDogs, encounter.FactionParty))

		goblins := s.turnedTo(enc, alice, bwGoblins, encounter.FactionParty)
		s.Equal("round 3 started", goblins["cause"])
		dogs := s.turnedTo(enc, alice, bwDogs, encounter.FactionParty)
		s.Equal("goblins and party turned hostile", dogs["cause"],
			"the dogs turned because the goblins did, and the beat says so")
	})

	s.Run("and the newcomers are in the fight", func() {
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bwChief))
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bwDog))
	})
}

// TestAttackingANeutralCampTurnsItAndFormsTheFight is R3, and it is the
// design's opening paragraph answered: the swing lands, the camp is hostile,
// and the fight is on — where before this slice the victim alone swung back
// with no initiative and no friends.
func (s *BothWaysSuite) TestAttackingANeutralCampTurnsItAndFormsTheFight() {
	enc := s.open(
		s.yard(
			[]encounter.FactionInput{{ID: bwGoblins}},
			[]encounter.DispositionInput{{
				Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
			false,
		),
		[]encounter.MemberInput{
			player(alice, 0, 1),
			monster(bwScout, bwGoblins, 4, 1),
			monster(bwChief, bwGoblins, 5, 5),
		},
	)
	s.Require().Equal(encounter.StanceNeutral, s.stance(enc, bwGoblins, encounter.FactionParty))
	s.Require().Equal(encounter.ClockWorld, s.clockOf(enc, alice), "precondition: nobody is fighting")

	_, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{bwScout},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	})
	s.Require().NoError(err)

	s.Run("the camp turned, as a camp", func() {
		s.Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty))
		hostile, known := enc.IsHostile(alice, bwChief)
		s.True(known)
		s.True(hostile, "the chief across the yard is at war too — a pair is a pair")
	})
	s.Run("the beat says who started it", func() {
		beat := s.turnedTo(enc, alice, bwGoblins, encounter.FactionParty)
		s.Equal(string(encounter.StanceHostile), beat["stance"])
		s.Equal("attacked by alice", beat["cause"])
	})
	s.Run("and everyone who could see it is in the fight", func() {
		s.Equal(encounter.ClockTurn, s.clockOf(enc, alice))
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bwScout))
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bwChief))
	})
}

// TestAnAlliedPairIsNotTurnedByAnAttack is R3's named exclusion: friendly
// fire is not betrayal in this cut.
func (s *BothWaysSuite) TestAnAlliedPairIsNotTurnedByAnAttack() {
	enc := s.open(
		s.yard(
			[]encounter.FactionInput{{ID: bwGoblins}},
			[]encounter.DispositionInput{{
				Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
				Stance:  encounter.StanceAllied,
			}},
			false,
		),
		[]encounter.MemberInput{player(alice, 0, 1), monster(bwScout, bwGoblins, 4, 1)},
	)

	_, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{bwScout},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	})
	s.Require().NoError(err)

	s.Equal(encounter.StanceAllied, s.stance(enc, bwGoblins, encounter.FactionParty))
	s.Empty(s.stanceBeats(enc, alice), "nothing turned, so nothing was announced")
	s.Equal(encounter.ClockWorld, s.clockOf(enc, alice), "and no fight formed")
}

// TestAWorldNPCIsNotATarget is R4: the verb refuses before it appends, and
// the world is exactly as it was.
func (s *BothWaysSuite) TestAWorldNPCIsNotATarget() {
	enc := s.open(
		s.yard(nil, nil, false),
		[]encounter.MemberInput{
			player(alice, 0, 1),
			{ID: bwVendor, Kind: encounter.KindWorld, Position: spatial.Position{X: 2, Y: 1}},
		},
	)

	s.Run("a swing at one is refused by name", func() {
		_, err := enc.Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{bwVendor},
			Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
		})
		s.Require().ErrorIs(err, encounter.ErrNotATarget)
		s.Contains(err.Error(), "is an npc and cannot be attacked; author it as a monster to make it a target")
	})

	s.Run("a spell attack at one is refused the same way", func() {
		_, err := enc.RecordCast(&encounter.RecordCastInput{
			Actor: alice, Spell: encounter.SpellIdentity{Ref: "dnd5e:spells:fire-bolt", Name: "Fire Bolt"},
			Targets: []encounter.CastTargetResult{{
				Target: bwVendor,
				Attack: &encounter.RecordInput{
					Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{bwVendor},
					Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
				},
			}},
		})
		s.Require().ErrorIs(err, encounter.ErrNotATarget)
	})

	s.Run("and the refusal turned nothing and told nobody", func() {
		s.Empty(s.stanceBeats(enc, alice))
		s.Equal(encounter.ClockWorld, s.clockOf(enc, alice))
		allied, known := enc.IsAllied(alice, bwVendor)
		s.False(allied)
		s.True(known, "a world NPC is still a member; it is just on no side")
	})
}

// TestTheBetrayedTruce is why the aggression settle is declared after the
// until settle: a camp that stood down because its chief read the letter, and
// then was attacked anyway, is at war again.
//
// THE SAME PAIR TURNS TWICE HERE, which is the one thing authoring cannot do
// — one disposition, one until, no oscillation — and exactly what a law that
// is not authored is for.
func (s *BothWaysSuite) TestTheBetrayedTruce() {
	enc := s.open(
		s.yard(
			[]encounter.FactionInput{{ID: bwGoblins, Mind: bwChief}},
			[]encounter.DispositionInput{{
				Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
				Stance:  encounter.StanceHostile, Until: encounter.TriggerFact{Fact: bwFact},
			}},
			true,
		),
		[]encounter.MemberInput{player(alice, 0, 1), monster(bwChief, bwGoblins, 1, 2)},
	)
	s.Require().Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty))

	// Alice is the active member of the fight the hostile pair started, and
	// the chief is in her region: picking the letter up carries it into his
	// presence, which is the hold-out's own path (design §3.6, R3).
	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: bwScroll})
	s.Require().NoError(err)
	s.Require().Equal(encounter.StanceNeutral, s.stance(enc, bwGoblins, encounter.FactionParty),
		"the truce the hold-out grants")

	_, err = enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{bwChief},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	})
	s.Require().NoError(err)

	s.Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty),
		"the chief still knows what the party did, and it no longer matters")

	// THE SEQUENCE, NOT THE COUNT: what alice was told about this pair, in
	// order, is the whole claim — a third beat would fail this as loudly as a
	// missing one, and without a number anybody could bump.
	told := make([]string, 0)
	var last map[string]any
	for _, beat := range s.stanceBeats(enc, alice) {
		told = append(told, beat["stance"].(string))
		last = beat
	}
	s.Equal([]string{string(encounter.StanceNeutral), string(encounter.StanceHostile)}, told,
		"the truce, and the truce broken")
	s.Require().NotNil(last)
	s.Equal("attacked by alice", last["cause"])
}

// TestATurnedPairSurvivesASaveAndLoad is the hold-out's A9 for the direction
// this slice adds: nothing stores "the camp turned", so the fact that turned
// it has to be in the blob or a provoked camp reloads civil.
func (s *BothWaysSuite) TestATurnedPairSurvivesASaveAndLoad() {
	field := s.yard(
		[]encounter.FactionInput{{ID: bwGoblins}},
		[]encounter.DispositionInput{{
			Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
			Stance:  encounter.StanceNeutral,
		}},
		false,
	)
	enc := s.open(field, []encounter.MemberInput{player(alice, 0, 1), monster(bwScout, bwGoblins, 4, 1)})

	_, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{bwScout},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	})
	s.Require().NoError(err)
	s.Require().Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty))

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      enc.ToData(),
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
	s.Require().NoError(err)

	s.Equal(encounter.StanceHostile, s.stance(loaded, bwGoblins, encounter.FactionParty))
	hostile, known := loaded.IsHostile(alice, bwScout)
	s.True(known)
	s.True(hostile)
}
