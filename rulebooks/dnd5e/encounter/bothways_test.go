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
//	R4  a world NPC is not a target through any hostile door, and refusing
//	    turns nothing, while a kindness still reaches one
//	R5  provocation is read off the DELIVERY, not the door: a swing, a save
//	    asked, and a gateless harm all provoke; a kindness provokes nobody
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

// formed is the one bubble-formed beat a scene produced, and nil when none
// was: which members entered initiative, and which of them entered unaware.
func (s *BothWaysSuite) formed(enc *encounter.Encounter, member core.EntityID) map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	var found map[string]any
	for _, beat := range story {
		var body map[string]any
		s.Require().NoError(json.Unmarshal(beat.Payload, &body))
		if body["beat"] != "bubble-formed" {
			continue
		}
		s.Require().Nil(found, "two fights formed where the scene expects one")
		found = body
	}
	return found
}

// engaged is who the bubble-formed beat put in initiative, as plain strings.
func engaged(beat map[string]any) []string {
	order, _ := beat["order"].([]any)
	out := make([]string, 0, len(order))
	for _, id := range order {
		out = append(out, id.(string))
	}
	return out
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

	// AND THE FIGHT FORMS IN THAT SAME SETTLE. Alice and the chief have been
	// looking at each other since first light, so no later sight refresh will
	// ever report a first contact between them; the turn is the transition,
	// and without this the camp turns hostile and stands there.
	s.Equal(encounter.ClockTurn, s.clockOf(enc, alice))
	s.Equal(encounter.ClockTurn, s.clockOf(enc, bwChief))
	form := s.formed(enc, alice)
	s.Require().NotNil(form, "a camp that turns with the party in view goes to initiative")
	s.ElementsMatch([]string{string(alice), string(bwChief)}, engaged(form),
		"the fallen scout is in no fight; everyone else who could see is")
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

	s.Run("and the newcomers went to initiative without anyone stepping", func() {
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bwChief))
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bwDog))
		// JOINED, NOT FORMED. The kobold's fight was already running, so a
		// pair turning inside one is the straggler rule — the same arm a wolf
		// rounding the corner takes — rather than a second bubble.
		form := s.formed(enc, alice)
		s.Require().NotNil(form)
		s.ElementsMatch([]string{string(alice), string(bwKobold)}, engaged(form),
			"the only bubble that ever formed is the one the kobold started")
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
	s.Run("and everyone who could see it is in the fight, nobody surprised", func() {
		s.Equal(encounter.ClockTurn, s.clockOf(enc, alice))
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bwScout))
		s.Equal(encounter.ClockTurn, s.clockOf(enc, bwChief))

		form := s.formed(enc, alice)
		s.Require().NotNil(form)
		s.ElementsMatch([]string{string(alice), string(bwScout), string(bwChief)}, engaged(form),
			"the attacker and every member of the camp that could see it")
		// SURPRISE IS READ FROM THE CURRENT VIEW, not from the synthesized
		// contact (trigger.go). Everyone in this yard has been watching
		// everyone since first light, so nobody enters unaware — which is the
		// right answer and is NOT what a first contact would imply on its own.
		s.NotContains(form, "surprised")
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

// TestAWorldNPCIsNotATarget is R4 as the review of #1868 extended it: EVERY
// hostile door refuses a world member before it appends, and the world is
// exactly as it was afterwards.
//
// THE THIRD AND FOURTH DOORS ARE R5'S DOING. Once a save and a gateless
// delivery reach [encounter.Encounter] through the same provocation path a
// swing does, a vendor left reachable through them holds an `attacked` deed
// against the caster — and a vendor that can be provoked into its own
// `attacked within 3 -> attack: attacker` rows is the opposite of one that
// cannot be attacked. So the refusal asks what the provocation asks.
func (s *BothWaysSuite) TestAWorldNPCIsNotATarget() {
	enc := s.open(
		s.yard(nil, nil, false),
		[]encounter.MemberInput{
			player(alice, 0, 1),
			{ID: bwVendor, Kind: encounter.KindWorld, Position: spatial.Position{X: 2, Y: 1}},
		},
	)

	before, err := enc.NextStorySeq()
	s.Require().NoError(err)

	s.Run("a swing at one is refused by name", func() {
		_, rerr := enc.Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{bwVendor},
			Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
		})
		s.Require().ErrorIs(rerr, encounter.ErrNotATarget)
		s.Contains(rerr.Error(), "is an npc and cannot be attacked; author it as a monster to make it a target")
	})

	s.Run("a spell attack at one is refused the same way", func() {
		_, cerr := enc.RecordCast(&encounter.RecordCastInput{
			Actor: alice, Spell: encounter.SpellIdentity{Ref: "dnd5e:spells:fire-bolt", Name: "Fire Bolt"},
			Targets: []encounter.CastTargetResult{{
				Target: bwVendor,
				Attack: &encounter.RecordInput{
					Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{bwVendor},
					Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
				},
			}},
		})
		s.Require().ErrorIs(cerr, encounter.ErrNotATarget)
	})

	s.Run("a save asked of one is refused the same way", func() {
		_, cerr := enc.RecordCast(&encounter.RecordCastInput{
			Actor: alice, Spell: encounter.SpellIdentity{Ref: "dnd5e:spells:sacred-flame", Name: "Sacred Flame"},
			Targets: []encounter.CastTargetResult{{
				Target: bwVendor,
				Save: &encounter.CastSave{
					Saver: bwVendor, Ability: "dexterity", Roll: 6, Total: 8, DC: 13, Succeeded: false,
					Calculation: saveCalculation(
						encounter.SpellIdentity{Ref: "dnd5e:spells:sacred-flame", Name: "Sacred Flame"},
						"dexterity", 6, 8),
				},
			}},
		})
		s.Require().ErrorIs(cerr, encounter.ErrNotATarget)
		s.Contains(cerr.Error(), "is an npc and cannot be attacked; author it as a monster to make it a target")
	})

	s.Run("and a gateless harm handed to one is refused the same way", func() {
		_, cerr := enc.RecordCast(&encounter.RecordCastInput{
			Actor: alice, Spell: encounter.SpellIdentity{Ref: "dnd5e:spells:sleep", Name: "Sleep"},
			Targets: []encounter.CastTargetResult{{
				Target: bwVendor,
				Results: []encounter.ActivationResult{{
					Kind: encounter.ResultConditionApplied, Name: "Unconscious",
					Address: &encounter.ConditionAddress{
						MemberID: bwVendor, ConditionRef: "dnd5e:conditions:unconscious",
						SourceID: string(alice),
					},
				}},
			}},
		})
		s.Require().ErrorIs(cerr, encounter.ErrNotATarget)
	})

	s.Run("and every refusal happened BEFORE any append", func() {
		after, serr := enc.NextStorySeq()
		s.Require().NoError(serr)
		s.Equal(before, after, "every refused hostile door left no beat behind it")
		s.Empty(s.stanceBeats(enc, alice))
		s.Nil(s.formed(enc, alice))
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

// bwMockery is the save-gated cantrip every R5 scene is about: Vicious
// Mockery, one target, a Wisdom save, 1d4 psychic when it lands.
var bwMockery = encounter.SpellIdentity{Ref: "dnd5e:spells:vicious-mockery", Name: "Vicious Mockery"}

// bwSave is the scout's Wisdom save against that cantrip, made or failed.
func bwSave(succeeded bool) *encounter.CastSave {
	roll, total := 6, 8
	if succeeded {
		roll, total = 17, 19
	}
	return &encounter.CastSave{
		Saver: bwScout, Ability: "wisdom", Roll: roll, Total: total, DC: 13, Succeeded: succeeded,
		Calculation: saveCalculation(bwMockery, "wisdom", roll, total),
	}
}

// bwPsychic is the 1d4 the failed save let through.
func bwPsychic() encounter.ActivationResult {
	return encounter.ActivationResult{
		Kind: encounter.ResultDamageApplied, Target: bwScout,
		Ref: bwMockery.Ref, Name: bwMockery.Name,
		Amount: 3, Requested: 3, Before: 7, After: 4, DamageType: "psychic",
		Calculation: &encounter.RollCalculation{
			Components: []encounter.RollComponent{{
				Source: encounter.RollSource{Ref: bwMockery.Ref, Name: bwMockery.Name, SourceID: string(alice)},
				Dice: &encounter.DiceTrace{
					Notation: "1d4", DieSize: 4,
					OriginalRolls: []int{3}, FinalRolls: []int{3}, Subtotal: 3,
				},
			}},
			Total: 3,
		},
	}
}

// camp is the yard of TestAttackingANeutralCampTurnsItAndFormsTheFight: a
// neutral goblin camp, a scout in the open and a chief across the yard, and
// nobody fighting anybody.
func (s *BothWaysSuite) camp() *encounter.Encounter {
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
	return enc
}

// TestAFailedSaveTurnsTheCampTheWaySwingDoes is R5, and it is the review's
// own probe committed (the independent round on rpg-toolkit#1864): a cantrip
// that delivers through a SAVE used to leave the camp civil, the caster on
// the world clock, and the scout with no testimony that anything had been
// done to it.
//
// THE SAME THREE CLAIMS THE SWING SCENE MAKES, deliberately: the pair turned
// as a camp, the beat says who started it, and everyone who could see it is
// in the fight. A spell is a different delivery, not a different law.
func (s *BothWaysSuite) TestAFailedSaveTurnsTheCampTheWaySwingDoes() {
	enc := s.camp()

	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: alice, Spell: bwMockery,
		Targets: []encounter.CastTargetResult{{
			Target: bwScout, Save: bwSave(false),
			Results: []encounter.ActivationResult{bwPsychic()},
		}},
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

		form := s.formed(enc, alice)
		s.Require().NotNil(form)
		s.ElementsMatch([]string{string(alice), string(bwScout), string(bwChief)}, engaged(form),
			"the caster and every member of the camp that could see it")
	})
}

// TestASavedCastStillProvokesBecauseTheAttemptIsTheProvocation is the missed
// swing's rule for the save: the scout shrugged the cantrip off, took nothing
// at all, and the camp is still at war.
//
// A CAMP THAT ONLY TURNS WHEN THE DICE LAND is a camp that forgives a bad
// roll, and R3 never worked that way for a swing — OutcomeMissed lands the
// same deed and the same turn as OutcomeStruck. This is that rule reaching
// the other delivery.
func (s *BothWaysSuite) TestASavedCastStillProvokesBecauseTheAttemptIsTheProvocation() {
	enc := s.camp()

	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: alice, Spell: bwMockery,
		Targets: []encounter.CastTargetResult{{Target: bwScout, Save: bwSave(true)}},
	})
	s.Require().NoError(err)

	s.Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty),
		"the spell delivered nothing; the attempt is what the camp answers")
	beat := s.turnedTo(enc, alice, bwGoblins, encounter.FactionParty)
	s.Equal("attacked by alice", beat["cause"])
	s.Equal(encounter.ClockTurn, s.clockOf(enc, bwChief), "and the camp is in the fight")
}

// TestASaveThatOnlyHelpsProvokesNothing is R5's named exemption, and the one
// place this module answers "is that harmful" at all.
//
// It cannot read a spell's intent — a [encounter.SpellIdentity] is a ref this
// composition may not interpret (C1) — so it reads the DELIVERY: a save whose
// whole delivery to the recipient is a kindness turns nobody. Healing a
// neutral camp's scout through a save-gated cast is not an attack on the
// camp, and the pair is exactly as it was.
func (s *BothWaysSuite) TestASaveThatOnlyHelpsProvokesNothing() {
	enc := s.camp()
	kindness := encounter.SpellIdentity{Ref: "dnd5e:spells:mass-cure-wounds", Name: "Mass Cure Wounds"}

	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: alice, Spell: kindness,
		Targets: []encounter.CastTargetResult{{
			Target: bwScout,
			Save: &encounter.CastSave{
				Saver: bwScout, Ability: "wisdom", Roll: 6, Total: 8, DC: 13, Succeeded: false,
				Calculation: saveCalculation(kindness, "wisdom", 6, 8),
			},
			Results: []encounter.ActivationResult{{
				Kind: encounter.ResultHealingApplied, Target: bwScout,
				Ref: kindness.Ref, Name: kindness.Name,
				Amount: 5, Requested: 5, Before: 4, After: 9,
				Calculation: &encounter.RollCalculation{
					Components: []encounter.RollComponent{{
						Source: encounter.RollSource{Ref: kindness.Ref, Name: kindness.Name, SourceID: string(alice)},
						Dice: &encounter.DiceTrace{
							Notation: "1d8", DieSize: 8,
							OriginalRolls: []int{5}, FinalRolls: []int{5}, Subtotal: 5,
						},
					}},
					Total: 5,
				},
			}},
		}},
	})
	s.Require().NoError(err)

	s.Equal(encounter.StanceNeutral, s.stance(enc, bwGoblins, encounter.FactionParty))
	s.Empty(s.stanceBeats(enc, alice), "nothing turned, so nothing was announced")
	s.Nil(s.formed(enc, alice), "and no fight formed")
}

// bwForce is 9 force damage delivered with NO gate at all — magic missile's
// shape, and the review's own probe on #1868.
func bwForce() encounter.ActivationResult {
	missile := encounter.SpellIdentity{Ref: "dnd5e:spells:magic-missile", Name: "Magic Missile"}
	return encounter.ActivationResult{
		Kind: encounter.ResultDamageApplied, Target: bwScout,
		Ref: missile.Ref, Name: missile.Name,
		Amount: 9, Requested: 9, Before: 12, After: 3, DamageType: "force",
		Calculation: &encounter.RollCalculation{
			Components: []encounter.RollComponent{{
				Source: encounter.RollSource{Ref: missile.Ref, Name: missile.Name, SourceID: string(alice)},
				Dice: &encounter.DiceTrace{
					Notation: "3d4", DieSize: 4,
					OriginalRolls: []int{2, 2, 2}, FinalRolls: []int{2, 2, 2}, Subtotal: 6,
				},
			}, {
				Source:   encounter.RollSource{Ref: missile.Ref, Name: missile.Name, SourceID: string(alice)},
				Modifier: func() *int { m := 3; return &m }(),
			}},
			Total: 9,
		},
	}
}

// TestAGatelessDeliveryProvokesOnWhatItDelivered is R5 as the review of #1868
// amended it: the third door.
//
// A MAGIC MISSILE ASKS FOR NEITHER ROLL. It has no attack roll and offers no
// save, and a predicate that answered by ARM read that as "no provocation" —
// the same fail-silent R5 removed for the save, surviving one delivery
// further over, and self-contradictory besides: a save that delivered
// NOTHING provoked, while delivered harm with no arms did not. The law reads
// the delivery now, so this camp turns on what was done to it.
func (s *BothWaysSuite) TestAGatelessDeliveryProvokesOnWhatItDelivered() {
	enc := s.camp()

	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: alice, Spell: encounter.SpellIdentity{Ref: "dnd5e:spells:magic-missile", Name: "Magic Missile"},
		Targets: []encounter.CastTargetResult{{
			Target: bwScout, Results: []encounter.ActivationResult{bwForce()},
		}},
	})
	s.Require().NoError(err)

	s.Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty),
		"nine force damage is nine force damage, whatever gate it came through")
	beat := s.turnedTo(enc, alice, bwGoblins, encounter.FactionParty)
	s.Equal("attacked by alice", beat["cause"])

	form := s.formed(enc, alice)
	s.Require().NotNil(form)
	s.ElementsMatch([]string{string(alice), string(bwScout), string(bwChief)}, engaged(form),
		"the caster and every member of the camp that could see it")
}

// TestAGatelessConditionProvokesAndAGatelessKindnessDoesNot is the pair of
// answers the delivery rule makes on the same door, and the reason it is a
// ruling rather than a branch: an ungated condition is something done TO a
// creature, and an ungated blessing is not.
func (s *BothWaysSuite) TestAGatelessConditionProvokesAndAGatelessKindnessDoesNot() {
	s.Run("a debuff laid on a neutral camp's scout turns the camp", func() {
		enc := s.camp()
		_, err := enc.RecordCast(&encounter.RecordCastInput{
			Actor: alice, Spell: encounter.SpellIdentity{Ref: "dnd5e:spells:sleep", Name: "Sleep"},
			Targets: []encounter.CastTargetResult{{
				Target: bwScout,
				Results: []encounter.ActivationResult{{
					Kind: encounter.ResultConditionApplied, Name: "Unconscious",
					Address: &encounter.ConditionAddress{
						MemberID: bwScout, ConditionRef: "dnd5e:conditions:unconscious",
						SourceID: string(alice),
					},
				}},
			}},
		})
		s.Require().NoError(err)
		s.Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty))
		s.Equal("attacked by alice", s.turnedTo(enc, alice, bwGoblins, encounter.FactionParty)["cause"])
	})

	s.Run("a buff laid on the same scout turns nobody", func() {
		enc := s.camp()
		_, err := enc.RecordCast(&encounter.RecordCastInput{
			Actor: alice, Spell: encounter.SpellIdentity{Ref: "dnd5e:spells:bless", Name: "Bless"},
			Targets: []encounter.CastTargetResult{{
				Target: bwScout,
				Results: []encounter.ActivationResult{{
					Kind: encounter.ResultConditionRemoved, Name: "Poisoned", Reason: "dispelled",
					Address: &encounter.ConditionAddress{
						MemberID: bwScout, ConditionRef: "dnd5e:conditions:poisoned",
						SourceID: string(alice),
					},
				}},
			}},
		})
		s.Require().NoError(err)
		s.Equal(encounter.StanceNeutral, s.stance(enc, bwGoblins, encounter.FactionParty))
		s.Empty(s.stanceBeats(enc, alice), "nothing turned, so nothing was announced")
		s.Nil(s.formed(enc, alice), "and no fight formed")
	})
}

// TestAKindnessStillReachesAWorldNPC is the other half of R4 extended, and
// the reason the refusal asks what the provocation asks rather than refusing
// every cast at a world member.
//
// HEALING THE MERCHANT IS NONE OF R4'S BUSINESS. The ruling is that a vendor
// cannot be attacked, not that it cannot be touched: a delivery with nothing
// hostile in it provokes nobody, lands no deed, and has no reason to be
// refused. A blanket refusal would have been the easier line and the wrong
// one — it would make a healer unable to help an NPC the party likes.
func (s *BothWaysSuite) TestAKindnessStillReachesAWorldNPC() {
	enc := s.open(
		s.yard(nil, nil, false),
		[]encounter.MemberInput{
			player(alice, 0, 1),
			{ID: bwVendor, Kind: encounter.KindWorld, Position: spatial.Position{X: 2, Y: 1}},
		},
	)
	cure := encounter.SpellIdentity{Ref: "dnd5e:spells:cure-wounds", Name: "Cure Wounds"}

	out, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: alice, Spell: cure,
		Targets: []encounter.CastTargetResult{{
			Target: bwVendor,
			Results: []encounter.ActivationResult{{
				Kind: encounter.ResultHealingApplied, Target: bwVendor,
				Ref: cure.Ref, Name: cure.Name,
				Amount: 5, Requested: 5, Before: 4, After: 9,
				Calculation: &encounter.RollCalculation{
					Components: []encounter.RollComponent{{
						Source: encounter.RollSource{Ref: cure.Ref, Name: cure.Name, SourceID: string(alice)},
						Dice: &encounter.DiceTrace{
							Notation: "1d8", DieSize: 8,
							OriginalRolls: []int{5}, FinalRolls: []int{5}, Subtotal: 5,
						},
					}},
					Total: 5,
				},
			}},
		}},
	})
	s.Require().NoError(err, "a vendor may be healed; only a hostile door is refused")
	s.NotEmpty(out.Seqs, "and the cast is in the story")
	s.Empty(s.stanceBeats(enc, alice))
	s.Nil(s.formed(enc, alice))
	s.Equal(encounter.ClockWorld, s.clockOf(enc, alice), "nobody was provoked into a fight")
}
