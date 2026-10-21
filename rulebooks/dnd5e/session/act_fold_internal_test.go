// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// act_fold_internal_test.go pins THE FOLD (rpg-project#629, event-stream
// R6, R19, R31, Law 6) on entries shaped exactly as the encounter writes
// them: the segment rules, the segment id, and the outcome inside the act.

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type ActFoldSuite struct {
	suite.Suite
	seq uint64
}

func TestActFoldSuite(t *testing.T) { suite.Run(t, new(ActFoldSuite)) }

func (s *ActFoldSuite) SetupTest() { s.seq = 0 }

// beat is one record entry under act tag (none for ""), its payload the
// fields given; ids sort in append order, as the record's do.
func (s *ActFoldSuite) beat(tag string, fields map[string]any) record.Entry {
	s.seq++
	payload, err := json.Marshal(fields)
	s.Require().NoError(err)
	e := record.Entry{Seq: s.seq, ID: fmt.Sprintf("01HZZZZZZZZZZZZZZZZZZZ%04d", s.seq), At: s.seq * 10, Payload: payload}
	if tag != "" {
		e.Tags = map[string]string{encounter.ActTag: tag}
	}
	return e
}

func cell(x, y float64) spatial.Position { return spatial.Position{X: x, Y: y} }

func (s *ActFoldSuite) step(tag, who string, at spatial.Position, doors ...string) record.Entry {
	fields := map[string]any{"beat": encounter.BeatMoved, "member": who, "position": at}
	if len(doors) > 0 {
		fields["doors"] = doors
	}
	return s.beat(tag, fields)
}

// path is a walk's cells, for comparison.
func path(w WalkAct) []spatial.Position {
	out := make([]spatial.Position, 0, len(w.Path))
	for _, st := range w.Path {
		out = append(out, st.Cell)
	}
	return out
}

// TestFoldWalkWithMidWalkReaction is R6 and R19: a guard's walk a, b, c
// (through a door), d, e with the fighter's opportunity attack after b folds
// to two walk segments around one reaction, all under the walk's id, each
// segment named by its last entry.
func (s *ActFoldSuite) TestFoldWalkWithMidWalkReaction() {
	const walk = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	a, b, c, d, e := cell(1, 0), cell(2, 0), cell(3, 0), cell(4, 0), cell(5, 0)
	entries := []record.Entry{
		s.step(walk, "gate-1", a),
		s.step(walk, "gate-1", b),
		s.beat(walk, map[string]any{
			"beat": string(encounter.OutcomeStruck), "actor": "fighter", "targets": []string{"gate-1"},
			"roll": 15, "total": 20, "against": 16, "amount": 6, "critical": false,
			"attack":   map[string]string{"ref": "dnd5e:weapons:longsword", "name": "Longsword", "damage_type": "slashing"},
			"reaction": map[string]string{"ref": "dnd5e:reactions:opportunity_attack", "name": "Opportunity Attack"},
		}),
		s.step(walk, "gate-1", c, "gatehouse-door"),
		s.step(walk, "gate-1", d),
		s.step(walk, "gate-1", e),
	}

	acts := foldActs("sess", "fighter", entries)

	s.Require().Len(acts, 3)
	first, reaction, second := acts[0], acts[1], acts[2]
	for _, act := range acts {
		s.Equal(walk, act.Act)
		s.Equal("move", act.Verb)
		s.Equal("fighter", act.Recipient)
	}

	s.Equal(FamilyAct, first.Family)
	s.Equal(1, first.Segment)
	s.Equal([]spatial.Position{a, b}, path(first.Body.(WalkAct)))
	s.Equal(entries[1].ID, first.ID, "a segment is named by its last entry")

	s.Equal(FamilyReaction, reaction.Family)
	s.Equal(0, reaction.Segment)
	swing := reaction.Body.(AttackAct)
	s.Equal(AttackHit, swing.Outcome)
	s.Require().NotNil(swing.Reaction)
	s.Equal("dnd5e:reactions:opportunity_attack", swing.Reaction.Ref)
	s.Equal(entries[2].ID, reaction.ID)

	s.Equal(FamilyAct, second.Family)
	s.Equal(2, second.Segment)
	w := second.Body.(WalkAct)
	s.Equal(Party{Kind: PartyMember, ID: "gate-1"}, w.Mover)
	s.Equal([]spatial.Position{b, c, d, e}, path(w), "the walk resumes where it was interrupted")
	s.Equal([]string{"gatehouse-door"}, w.Path[1].Doors, "step c went through the door")
	s.Equal(entries[5].ID, second.ID)
	s.Equal(entries[5].At, second.At)
}

// TestResumeAfterASegmentFoldsTheSameSuffix is R31's point: resuming after
// any segment's id yields exactly the acts that followed it.
func (s *ActFoldSuite) TestResumeAfterASegmentFoldsTheSameSuffix() {
	const walk = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	entries := []record.Entry{
		s.step(walk, "gate-1", cell(1, 0)),
		s.beat(walk, map[string]any{
			"beat": encounter.BeatWindowOpened, "member": "gate-1", "from": cell(1, 0), "to": cell(2, 0),
			"windows": []map[string]any{{"audience": "fighter", "reaction": map[string]string{"ref": "r", "name": "OA"}}},
		}),
		s.step(walk, "gate-1", cell(2, 0)),
		s.beat("", map[string]any{"beat": encounter.BeatTick, "tick": 3}),
	}
	whole := actsAfter("sess", "fighter", entries, "")
	for i, act := range whole {
		s.Equal(orEmpty(whole[i+1:]), orEmpty(actsAfter("sess", "fighter", entries, act.ID)), "after %s", act.ID)
	}
}

// orEmpty reads nil and empty alike, so a suffix compares by its acts.
func orEmpty(acts []Act) []Act {
	if len(acts) == 0 {
		return []Act{}
	}
	return acts
}

// TestPerceptionStaysPerObserver: a sighting during A's walk is A's alone. A's
// walk splits around it; B, who never held the beat, sees one walk.
func (s *ActFoldSuite) TestPerceptionStaysPerObserver() {
	const walk = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	first := s.step(walk, "alice", cell(1, 0))
	sighted := s.beat(walk, map[string]any{"beat": encounter.BeatSighted, "gained": []string{"gate-1"}})
	second := s.step(walk, "alice", cell(2, 0))

	alice := foldActs("sess", "alice", []record.Entry{first, sighted, second})
	bob := foldActs("sess", "bob", []record.Entry{first, second})

	s.Require().Len(alice, 3)
	s.Equal(FamilyPerception, alice[1].Family)
	seen := alice[1].Body.(Perception)
	s.Equal(Party{Kind: PartyMember, ID: "alice"}, seen.Observer)
	s.Equal(SenseSight, seen.Sense)
	s.Equal([]string{"gate-1"}, seen.Sighted.Gained)
	s.Equal(2, alice[2].Segment)

	s.Require().Len(bob, 1)
	s.Equal(FamilyAct, bob[0].Family)
	s.Equal([]spatial.Position{cell(1, 0), cell(2, 0)}, path(bob[0].Body.(WalkAct)))
}

// TestFoldMissAndWarded is Law 6: a miss and a ward are each one swing whose
// outcome is in its own body — no second entry says how it went.
func (s *ActFoldSuite) TestFoldMissAndWarded() {
	const miss, ward = "attack:01HZZZZZZZZZZZZZZZZZZZMISS", "attack:01HZZZZZZZZZZZZZZZZZZWARD"
	attack := map[string]string{"ref": "dnd5e:weapons:longsword", "name": "Longsword", "damage_type": "slashing"}
	entries := []record.Entry{
		s.beat(miss, map[string]any{
			"beat": string(encounter.OutcomeMissed), "actor": "alice", "targets": []string{"bob"},
			"roll": 3, "total": 8, "against": 16, "attack": attack,
		}),
		s.beat(ward, map[string]any{
			"beat": string(encounter.OutcomeWarded), "actor": "alice", "targets": []string{"bob"}, "attack": attack,
			"warded": map[string]any{"source": "cleric", "save": map[string]any{
				"saver": "alice", "ability": "wis", "roll": 4, "total": 5, "dc": 13, "succeeded": false,
			}},
		}),
	}

	acts := foldActs("sess", "alice", entries)

	s.Require().Len(acts, 2)
	missed := acts[0].Body.(AttackAct)
	s.Equal(AttackMiss, missed.Outcome)
	s.Require().NotNil(missed.Roll)
	s.False(*missed.Roll.Beaten)
	s.Equal(16, *missed.Roll.Against)
	s.Nil(missed.Ward)

	warded := acts[1].Body.(AttackAct)
	s.Equal(AttackWarded, warded.Outcome)
	s.Require().NotNil(warded.Ward)
	s.Equal(Party{Kind: PartyMember, ID: "cleric"}, warded.Ward.Source)
	s.Equal("wis", warded.Ward.Ability)
	s.Equal(13, *warded.Ward.Roll.Against)
	s.False(*warded.Ward.Roll.Beaten)
	s.Nil(warded.Roll, "a warded swing rolled nothing against the target")
	for _, act := range acts {
		s.Equal(FamilyAct, act.Family)
	}
}

// TestFoldDoorNamesActor is Law 7 for the door: the door beat's actor
// is the act's By, and an unattributed change names nobody.
func (s *ActFoldSuite) TestFoldDoorNamesActor() {
	acts := foldActs("sess", "alice", []record.Entry{
		s.beat("open_door:01HZZZZZZZZZZZZZZZZZZZDOOR", map[string]any{
			"beat": encounter.BeatDoor, "door": "gatehouse-door", "state": "open", "actor": "alice",
		}),
		s.beat("", map[string]any{"beat": encounter.BeatDoor, "door": "portcullis", "state": "closed"}),
	})

	s.Require().Len(acts, 2)
	s.Equal(DoorAct{By: Party{Kind: PartyMember, ID: "alice"}, Door: "gatehouse-door", State: "open"}, acts[0].Body)
	s.Equal(DoorAct{Door: "portcullis", State: "closed"}, acts[1].Body)
}

// TestRollFromCalculation is R17: a 1d20+5 attack's roll states its kept
// face, its named modifiers, its total and what it was against.
func (s *ActFoldSuite) TestRollFromCalculation() {
	calculation := json.RawMessage(`{"components":[
		{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword","source_id":"alice"},"dice":{"notation":"1d20","die_size":20,"original_rolls":[14],"final_rolls":[14],"subtotal":14}},
		{"source":{"ref":"dnd5e:abilities:str","name":"STR"},"modifier":3},
		{"source":{"ref":"dnd5e:proficiency:proficiency","name":"proficiency"},"modifier":2}
	],"total":19}`)

	roll, err := rollOf(calculation, []int{14}, 19, intp(16), boolp(true))
	s.Require().NoError(err)

	s.Equal("1d20+5", roll.Expression)
	s.Equal([]int{14}, roll.Dice)
	s.Equal([]RollModifier{{Source: "STR", Value: 3}, {Source: "proficiency", Value: 2}}, roll.Modifiers)
	s.Equal(19, roll.Total)
	s.Equal(16, *roll.Against)
	s.True(*roll.Beaten)
	s.Zero(roll.Of)
}

// TestRollKeepsOnlyTheKeptFaces: an advantage roll's discarded face is not
// one of the roll's dice, and the expression counts the die that counted.
func (s *ActFoldSuite) TestRollKeepsOnlyTheKeptFaces() {
	calculation := json.RawMessage(`{"components":[
		{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword","source_id":"alice"},"dice":{"notation":"2d20","die_size":20,"original_rolls":[7,15],"final_rolls":[7,15],"kept_indices":[1],"keep":{"rule":"advantage","granted":[{"ref":"dnd5e:conditions:helped","name":"Helped","source_id":"bob"}]},"subtotal":15}},
		{"source":{"ref":"dnd5e:abilities:str","name":"STR"},"modifier":3},
		{"source":{"ref":"dnd5e:proficiency:proficiency","name":"proficiency"},"modifier":2}
	],"total":20}`)

	roll, err := rollOf(calculation, []int{15}, 20, intp(16), boolp(true))
	s.Require().NoError(err)

	s.Equal([]int{15}, roll.Dice, "the low face of advantage is not a die that counted")
	s.Equal("1d20+5", roll.Expression)
	s.Equal(20, roll.Total)
}

// TestActsSharingAnIDAreOneDeliveryUnit: a rest that ended a concentration
// folds into the rest and the concentration's end under the rest beat's one
// id, adjacent; resuming after that id skips both.
func (s *ActFoldSuite) TestActsSharingAnIDAreOneDeliveryUnit() {
	const rest = "rest:01HZZZZZZZZZZZZZZZZZZZREST"
	rested := s.beat(rest, map[string]any{
		"beat": encounter.BeatRested, "member": "cleric", "kind": "short",
		"hit_points_restored": 0, "hit_points": 10, "hit_dice_spent": 0, "hit_dice_returned": 0, "hit_dice_remaining": 2,
		"concentration_ended": []map[string]any{{
			"caster": "cleric", "spell": map[string]string{"ref": "dnd5e:spells:bless", "name": "Bless"}, "reason": "rested",
		}},
	})
	tick := s.beat("", map[string]any{"beat": encounter.BeatTick, "tick": 4})
	entries := []record.Entry{rested, tick}

	acts := foldActs("sess", "cleric", entries)

	s.Require().Len(acts, 3)
	s.IsType(RestAct{}, acts[0].Body)
	s.Equal(StateConcentrationEnded, acts[1].Body.(StateChange).Change)
	s.Equal(rest, acts[1].Act, "the end is under the rest's act")
	s.Equal(rested.ID, acts[0].ID)
	s.Equal(rested.ID, acts[1].ID, "one entry, one id: the two are one delivery unit")

	after := actsAfter("sess", "cleric", entries, rested.ID)
	s.Require().Len(after, 1, "resuming after the id skips the whole unit")
	s.Equal(tick.ID, after[0].ID)
}

// TestAnUnreadableBeatFailsOnlyItsEntry is R22's spirit: one bad payload is
// one marked entry, and the fold goes on.
func (s *ActFoldSuite) TestAnUnreadableBeatFailsOnlyItsEntry() {
	acts := foldActs("sess", "alice", []record.Entry{
		s.beat("attack:01HZZZZZZZZZZZZZZZZZZZZBAD", map[string]any{
			"beat": string(encounter.OutcomeStruck), "actor": "alice", "targets": "not a list",
		}),
		s.beat("", map[string]any{"beat": encounter.BeatTick, "tick": 1}),
	})

	s.Require().Len(acts, 2)
	s.NotEmpty(acts[0].Undecoded)
	s.Nil(acts[0].Body)
	s.Equal(FamilyAct, acts[0].Family)
	s.Equal(ClockEntry{Mark: ClockTick, Tick: 1}, acts[1].Body)
}
