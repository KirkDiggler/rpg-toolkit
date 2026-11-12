// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// act_fold_internal_test.go pins THE FOLD (rpg-project#629, event-stream
// R6, R19, R31, Law 6) on entries shaped exactly as the encounter writes
// them: the segment rules, the segment id, and the outcome inside the act.

import (
	"encoding/json"
	"fmt"
	"strings"
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

const recordedThunderwavePush = `{
	"beat":"moved",
	"member":"goblin",
	"position":{"X":7,"Y":2},
	"cause":"dnd5e:spells:thunderwave",
	"forced":true
}`

// TestALegacyWalkPayloadDecodesAsAnOrdinaryWalk pins the zero-value law at
// the payload seam: the key was absent before displacement was recorded.
func (s *ActFoldSuite) TestALegacyWalkPayloadDecodesAsAnOrdinaryWalk() {
	pieces, err := walkPiece([]byte(`{"beat":"moved","member":"goblin","position":{"X":7,"Y":2}}`))
	s.Require().NoError(err)
	s.Require().Len(pieces, 1)
	walk := pieces[0].body.(WalkAct)
	s.False(walk.Forced, "a payload with no evidence remains a chosen walk")
	s.Equal(MoveOwn, walk.Kind, "and its kind is own")
}

// TestALegacyBeatDerivesItsKind: a beat written before the kind derives it
// from `forced` alone. A cause never decides the kind (rpg-project#825).
func (s *ActFoldSuite) TestALegacyBeatDerivesItsKind() {
	cases := []struct {
		name    string
		payload string
		want    MoveKind
	}{
		{"no cause, no forced", `{"beat":"moved","member":"goblin","position":{"X":7,"Y":2}}`, MoveOwn},
		{"a cause, no forced", `{"beat":"moved","member":"goblin","position":{"X":7,"Y":2},"cause":"dnd5e:spells:dissonant-whispers"}`, MoveOwn},
		{"forced", recordedThunderwavePush, MoveDisplaced},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			pieces, err := walkPiece([]byte(tc.payload))
			s.Require().NoError(err)
			s.Require().Len(pieces, 1)
			walk := pieces[0].body.(WalkAct)
			s.Equal(tc.want, walk.Kind)
			s.Equal(tc.want == MoveDisplaced, walk.Forced, "forced mirrors displaced")
		})
	}
}

// TestAWalkSegmentBreaksWhenTheKindChanges: same mover, same cause, compelled
// then displaced — two walks, because a segment never mixes kinds.
func (s *ActFoldSuite) TestAWalkSegmentBreaksWhenTheKindChanges() {
	const move = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	cause := "dnd5e:spells:thunderwave"
	acts := foldActs("sess", "fighter", []record.Entry{
		s.beat(move, map[string]any{"beat": encounter.BeatMoved, "member": "goblin", "position": cell(7, 2), "cause": cause, "kind": "compelled"}),
		s.beat(move, map[string]any{"beat": encounter.BeatMoved, "member": "goblin", "position": cell(8, 2), "cause": cause, "kind": "compelled"}),
		s.beat(move, map[string]any{"beat": encounter.BeatMoved, "member": "goblin", "position": cell(9, 2), "cause": cause, "kind": "displaced", "forced": true}),
	})
	s.Require().Len(acts, 2, "the kind changed, so the walk broke")
	first := acts[0].Body.(WalkAct)
	second := acts[1].Body.(WalkAct)
	s.Equal(MoveCompelled, first.Kind)
	s.Len(first.Path, 2, "the two compelled steps are one walk")
	s.Equal(MoveDisplaced, second.Kind)
	s.True(second.Forced)
	s.Equal([]spatial.Position{cell(8, 2), cell(9, 2)}, path(second), "the segment still starts where the prior walk ended")
}

// TestAForcedWalkActRoundTripsThroughSerialization keeps the act body honest
// for the host transport that serializes exported session values.
func (s *ActFoldSuite) TestAForcedWalkActRoundTripsThroughSerialization() {
	want := WalkAct{
		Mover:  member("goblin"),
		Path:   []WalkStep{{Cell: cell(7, 2)}},
		Cause:  "dnd5e:spells:thunderwave",
		Kind:   MoveDisplaced,
		Forced: true,
	}
	raw, err := json.Marshal(want)
	s.Require().NoError(err)
	var wire map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(raw, &wire))
	s.Contains(wire, "forced", "the serialized WalkAct keeps the beat payload's forced key")
	s.Contains(wire, "kind", "and carries the kind")
	var got WalkAct
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal(want, got)
}

// TestRecordedThunderwavePushDecodesAsForced is the real moved-beat shape
// from the encounter record. Keep the fixture literal so a renamed or omitted
// payload key fails at the session boundary rather than in a client.
func (s *ActFoldSuite) TestRecordedThunderwavePushDecodesAsForced() {
	pieces, err := walkPiece([]byte(recordedThunderwavePush))
	s.Require().NoError(err)
	s.Require().Len(pieces, 1)
	walk := pieces[0].body.(WalkAct)
	s.True(walk.Forced)
	s.Equal(MoveDisplaced, walk.Kind, "a legacy forced beat is displaced")
	s.Equal("dnd5e:spells:thunderwave", walk.Cause)
}

// TestWalkActsDistinguishWalkRoutAndPush proves the stream's three movement
// semantics by kind: a chosen walk (own), a Dissonant Whispers flee and a
// Command flee (compelled), and a Thunderwave push (displaced).
func (s *ActFoldSuite) TestWalkActsDistinguishWalkRoutAndPush() {
	const (
		walkAct    = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
		routAct    = "move:01HZZZZZZZZZZZZZZZZZZZROUT"
		commandAct = "move:01HZZZZZZZZZZZZZZZZZZCMND"
		pushAct    = "move:01HZZZZZZZZZZZZZZZZZZZPUSH"
	)
	acts := foldActs("sess", "fighter", []record.Entry{
		s.beat(walkAct, map[string]any{"beat": encounter.BeatMoved, "member": "goblin", "position": cell(7, 2)}),
		s.beat(routAct, map[string]any{"beat": encounter.BeatMoved, "member": "goblin", "position": cell(8, 2), "cause": "dnd5e:spells:dissonant-whispers", "kind": "compelled"}),
		s.beat(commandAct, map[string]any{"beat": encounter.BeatMoved, "member": "goblin", "position": cell(9, 2), "cause": "dnd5e:conditions:commanded", "kind": "compelled"}),
		s.beat(pushAct, map[string]any{"beat": encounter.BeatMoved, "member": "goblin", "position": cell(10, 2), "cause": "dnd5e:spells:thunderwave", "kind": "displaced", "forced": true}),
	})
	s.Require().Len(acts, 4)

	walk := acts[0].Body.(WalkAct)
	rout := acts[1].Body.(WalkAct)
	command := acts[2].Body.(WalkAct)
	push := acts[3].Body.(WalkAct)
	s.Equal(MoveOwn, walk.Kind)
	s.False(walk.Forced)
	s.Empty(walk.Cause)
	s.Equal(MoveCompelled, rout.Kind)
	s.False(rout.Forced)
	s.Equal("dnd5e:spells:dissonant-whispers", rout.Cause)
	s.Equal(MoveCompelled, command.Kind)
	s.False(command.Forced)
	s.Equal("dnd5e:conditions:commanded", command.Cause)
	s.Equal(MoveDisplaced, push.Kind)
	s.True(push.Forced)
	s.Equal("dnd5e:spells:thunderwave", push.Cause)
}

// TestTheWalkFoldRefusesToMergeDifferentMovementSemantics is rpg-toolkit#2227
// item 3: same mover and cause still form separate acts when one step pushes.
func (s *ActFoldSuite) TestTheWalkFoldRefusesToMergeDifferentMovementSemantics() {
	const move = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	cause := "dnd5e:spells:thunderwave"
	acts := foldActs("sess", "fighter", []record.Entry{
		s.beat(move, map[string]any{"beat": encounter.BeatMoved, "member": "goblin", "position": cell(7, 2), "cause": cause}),
		s.beat(move, map[string]any{"beat": encounter.BeatMoved, "member": "goblin", "position": cell(8, 2), "cause": cause, "forced": true}),
	})
	s.Require().Len(acts, 2, "a push may not be absorbed into a same-cause walk")

	first := acts[0].Body.(WalkAct)
	second := acts[1].Body.(WalkAct)
	s.False(first.Forced)
	s.True(second.Forced, "the continued segment inherits the incoming step, not the preceding walk")
	s.Equal([]spatial.Position{cell(7, 2), cell(8, 2)}, path(second), "the segment still starts where the prior walk ended")
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
	s.Equal([]RollModifier{
		{Source: "STR", Value: 3, SourceRef: "dnd5e:abilities:str"},
		{Source: "proficiency", Value: 2, SourceRef: "dnd5e:proficiency:proficiency"},
	}, roll.Modifiers)
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

// TestRollCarriesAdvantageSources is R17 (a roll carries its sources) for
// advantage: a swing with advantage from a named source folds to one roll
// influence with that ref and source, and a disadvantage source after it; a
// roll whose calculation kept the high face reads its granting source from
// the dice keep.
func (s *ActFoldSuite) TestRollCarriesAdvantageSources() {
	swing := s.beat("attack:01HZZZZZZZZZZZZZZZZZZZSWNG", map[string]any{
		"beat": string(encounter.OutcomeStruck), "actor": "fighter", "targets": []string{"gate-1"},
		"roll": 15, "total": 20, "against": 16, "amount": 6,
		"attack":               map[string]string{"ref": "dnd5e:weapons:longsword", "name": "Longsword", "damage_type": "slashing"},
		"advantage_sources":    []map[string]string{{"source_ref": "dnd5e:conditions:hidden", "source_id": "fighter"}},
		"disadvantage_sources": []map[string]string{{"source_ref": "dnd5e:conditions:frightened"}},
	})
	acts := foldActs("sess", "fighter", []record.Entry{swing})
	s.Require().Len(acts, 1)
	attack, ok := acts[0].Body.(AttackAct)
	s.Require().True(ok)
	s.Require().NotNil(attack.Roll)
	s.Equal([]RollInfluence{
		{Kind: InfluenceAdvantage, SourceRef: "dnd5e:conditions:hidden", SourceID: "fighter"},
		{Kind: InfluenceDisadvantage, SourceRef: "dnd5e:conditions:frightened"},
	}, attack.Roll.Influences)

	calc := json.RawMessage(`{"components":[
		{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword","source_id":"alice"},"dice":{"notation":"2d20","die_size":20,"original_rolls":[7,15],"final_rolls":[7,15],"kept_indices":[1],"keep":{"rule":"advantage","granted":[{"ref":"dnd5e:conditions:helped","name":"Helped","source_id":"bob"}]},"subtotal":15}},
		{"source":{"ref":"dnd5e:abilities:str","name":"STR"},"modifier":3},
		{"source":{"ref":"dnd5e:proficiency:proficiency","name":"proficiency"},"modifier":2}
	],"total":20}`)
	roll, err := rollOf(calc, []int{15}, 20, intp(16), boolp(true))
	s.Require().NoError(err)
	s.Equal([]RollInfluence{{Kind: InfluenceAdvantage, SourceRef: "dnd5e:conditions:helped", SourceID: "bob", Name: "Helped"}}, roll.Influences)
}

// keepCalc is a d20 attack calculation whose pool carries the given keep JSON
// over the given faces and kept indices.
func keepCalc(faces, kept, keep string, subtotal int) json.RawMessage {
	keptKey := ""
	if kept != "" {
		keptKey = `"kept_indices":` + kept + `,`
	}
	return json.RawMessage(fmt.Sprintf(`{"components":[
		{"source":{"ref":"dnd5e:weapons:longsword","name":"Longsword","source_id":"alice"},"dice":{"notation":"%dd20","die_size":20,"original_rolls":%s,"final_rolls":%s,%s"keep":%s,"subtotal":%d}},
		{"source":{"ref":"dnd5e:abilities:str","name":"STR"},"modifier":3}
	],"total":%d}`, strings.Count(faces, ",")+1, faces, faces, keptKey, keep, subtotal, subtotal+3))
}

// TestRollInfluencesComeFromTheKeep is rpg-project#462 R1 on the act roll:
// the calculation's keep is the record, for disadvantage and for a
// cancellation as for advantage, and each influence keeps its source's name.
func (s *ActFoldSuite) TestRollInfluencesComeFromTheKeep() {
	disadvantage := keepCalc(`[7,15]`, `[0]`,
		`{"rule":"disadvantage","imposed":[{"ref":"dnd5e:conditions:prone","name":"Prone","source_id":"alice"}]}`, 7)
	roll, err := rollOf(disadvantage, nil, 10, intp(16), boolp(false))
	s.Require().NoError(err)
	s.Equal([]RollInfluence{
		{Kind: InfluenceDisadvantage, SourceRef: "dnd5e:conditions:prone", SourceID: "alice", Name: "Prone"},
	}, roll.Influences)

	cancelled := keepCalc(`[11]`, ``,
		`{"rule":"cancelled","granted":[{"ref":"dnd5e:conditions:helped","name":"Helped","source_id":"bob"}],"imposed":[{"ref":"dnd5e:conditions:prone","name":"Prone","source_id":"alice"}]}`, 11)
	roll, err = rollOf(cancelled, nil, 14, intp(16), boolp(false))
	s.Require().NoError(err)
	s.Equal([]RollInfluence{
		{Kind: InfluenceAdvantage, SourceRef: "dnd5e:conditions:helped", SourceID: "bob", Name: "Helped"},
		{Kind: InfluenceDisadvantage, SourceRef: "dnd5e:conditions:prone", SourceID: "alice", Name: "Prone"},
	}, roll.Influences, "a cancellation names both sides, as the keep does")
}

// TestTheKeepWinsOverRetiredSourceKeys is R1's precedence: the
// advantage_sources and disadvantage_sources keys are retired, so a swing
// that carries both them and a keep reads the keep alone — never both
// appended. The keys are read only by a beat with no keep.
func (s *ActFoldSuite) TestTheKeepWinsOverRetiredSourceKeys() {
	var calc map[string]any
	s.Require().NoError(json.Unmarshal(keepCalc(`[7,15]`, `[1]`,
		`{"rule":"advantage","granted":[{"ref":"dnd5e:conditions:helped","name":"Helped","source_id":"bob"}]}`, 15), &calc))
	swing := s.beat("attack:01HZZZZZZZZZZZZZZZZZZZKEEP", map[string]any{
		"beat": string(encounter.OutcomeStruck), "actor": "alice", "targets": []string{"gate-1"},
		"roll": 15, "total": 18, "against": 16, "amount": 6, "calculation": calc,
		"attack":               map[string]string{"ref": "dnd5e:weapons:longsword", "name": "Longsword", "damage_type": "slashing"},
		"advantage_sources":    []map[string]string{{"source_ref": "dnd5e:conditions:hidden", "source_id": "alice"}},
		"disadvantage_sources": []map[string]string{{"source_ref": "dnd5e:conditions:frightened"}},
	})
	acts := foldActs("sess", "alice", []record.Entry{swing})
	s.Require().Len(acts, 1)
	s.Require().Empty(acts[0].Undecoded)
	attack, ok := acts[0].Body.(AttackAct)
	s.Require().True(ok)
	s.Require().NotNil(attack.Roll)
	s.Equal([]RollInfluence{
		{Kind: InfluenceAdvantage, SourceRef: "dnd5e:conditions:helped", SourceID: "bob", Name: "Helped"},
	}, attack.Roll.Influences)
}

// TestRollTermsCarryTheirSources is R17 for dice terms: a Bless-blessed save
// folds to a d4 term with Bless's ref and name beside its d20, and Bane's d4
// is a subtracted term with Bane's.
func (s *ActFoldSuite) TestRollTermsCarryTheirSources() {
	calc := json.RawMessage(`{"components":[
		{"source":{"ref":"dnd5e:saves:wis","name":"Wisdom save","source_id":"fighter"},"dice":{"notation":"d20","die_size":20,"original_rolls":[12],"final_rolls":[12],"subtotal":12}},
		{"source":{"ref":"dnd5e:spells:bless","name":"Bless","source_id":"cleric"},"dice":{"notation":"d4","die_size":4,"original_rolls":[3],"final_rolls":[3],"subtotal":3}},
		{"source":{"ref":"dnd5e:spells:bane","name":"Bane","source_id":"cultist"},"dice":{"notation":"d4","die_size":4,"original_rolls":[2],"final_rolls":[2],"subtotal":2},"subtract_dice":true},
		{"source":{"ref":"dnd5e:abilities:wis","name":"WIS"},"modifier":1}
	],"total":14}`)
	roll, err := rollOf(calc, nil, 14, intp(13), boolp(true))
	s.Require().NoError(err)
	s.Equal([]RollDiceTerm{
		{Count: 1, DieSize: 20, Faces: []int{12}, Rolled: []int{12}, SourceRef: "dnd5e:saves:wis", SourceName: "Wisdom save", SourceID: "fighter"},
		{Count: 1, DieSize: 4, Faces: []int{3}, Rolled: []int{3}, SourceRef: "dnd5e:spells:bless", SourceName: "Bless", SourceID: "cleric"},
		{Count: 1, DieSize: 4, Faces: []int{2}, Rolled: []int{2}, Subtract: true, SourceRef: "dnd5e:spells:bane", SourceName: "Bane", SourceID: "cultist"},
	}, roll.Terms)
	s.Equal("1d20+1d4-1d4+1", roll.Expression)
}

// TestRollTermCountsTheFacesThatCounted: a 2d6 pool is ONE term with both
// faces, and an advantage d20 is one term counting only its kept face.
func (s *ActFoldSuite) TestRollTermCountsTheFacesThatCounted() {
	damage := json.RawMessage(`{"components":[
		{"source":{"ref":"dnd5e:weapons:greatsword","name":"Greatsword","source_id":"alice"},"dice":{"notation":"2d6","die_size":6,"original_rolls":[4,5],"final_rolls":[4,5],"subtotal":9}},
		{"source":{"ref":"dnd5e:abilities:str","name":"STR"},"modifier":3}
	],"total":12}`)
	roll, err := rollOf(damage, nil, 12, nil, nil)
	s.Require().NoError(err)
	s.Equal([]RollDiceTerm{
		{Count: 2, DieSize: 6, Faces: []int{4, 5}, Rolled: []int{4, 5}, SourceRef: "dnd5e:weapons:greatsword", SourceName: "Greatsword", SourceID: "alice"},
	}, roll.Terms)

	advantage := keepCalc(`[7,15]`, `[1]`,
		`{"rule":"advantage","granted":[{"ref":"dnd5e:conditions:helped","name":"Helped","source_id":"bob"}]}`, 15)
	roll, err = rollOf(advantage, nil, 18, intp(16), boolp(true))
	s.Require().NoError(err)
	s.Require().Len(roll.Terms, 1)
	s.Equal(1, roll.Terms[0].Count)
	s.Equal([]int{15}, roll.Terms[0].Faces)
}

// TestRollKeepsTheRerollRecord is #643 B: a Great Weapon Fighting damage
// roll keeps the pool as first rolled and the sourced reroll beside its
// final faces, so a reader can show die 0 rerolled from 1 to 4 and by whom.
// Mutants: dropping the reroll copy, or swapping before and after, fails it.
func (s *ActFoldSuite) TestRollKeepsTheRerollRecord() {
	calc := json.RawMessage(`{"components":[
		{"source":{"ref":"dnd5e:weapons:greatsword","name":"Greatsword","label":"weapon","source_id":"fighter"},"dice":{"notation":"2d6","die_size":6,"original_rolls":[1,5],
			"rerolls":[{"die_index":0,"before":1,"after":4,"source":{"ref":"dnd5e:fighting-styles:great-weapon-fighting","name":"Great Weapon Fighting","label":"fighting style","source_id":"fighter"}}],
			"final_rolls":[4,5],"subtotal":9}}
	],"total":9}`)
	roll, err := rollOf(calc, nil, 9, nil, nil)
	s.Require().NoError(err)
	s.Require().Len(roll.Terms, 1)
	term := roll.Terms[0]
	s.Equal([]int{1, 5}, term.Rolled, "the pool as first rolled")
	s.Equal([]int{4, 5}, term.Faces, "the faces that counted, after the reroll")
	s.Equal("weapon", term.SourceLabel, "the term's source arrives whole")
	s.Equal([]RollReroll{{
		DieIndex: 0, Before: 1, After: 4,
		SourceRef: "dnd5e:fighting-styles:great-weapon-fighting", SourceName: "Great Weapon Fighting", SourceID: "fighter",
		SourceLabel: "fighting style",
	}}, term.Rerolls)
	s.Equal(9, roll.Total)
}

// TestRollKeptIndicesWithAdvantage is #643 B for a keep: an advantage d20
// rolled [7, 15] keeps index 1, so the term says the pool, the kept index,
// and only the kept face counted. Mutant: returning the pool as Faces fails.
func (s *ActFoldSuite) TestRollKeptIndicesWithAdvantage() {
	advantage := keepCalc(`[7,15]`, `[1]`,
		`{"rule":"advantage","granted":[{"ref":"dnd5e:conditions:helped","name":"Helped","source_id":"bob"}]}`, 15)
	roll, err := rollOf(advantage, nil, 18, intp(16), boolp(true))
	s.Require().NoError(err)
	s.Require().Len(roll.Terms, 1)
	term := roll.Terms[0]
	s.Equal([]int{7, 15}, term.Rolled)
	s.Equal([]int{1}, term.KeptIndices)
	s.Equal([]int{15}, term.Faces)
	s.Empty(term.Rerolls, "nothing rerolled this pool")

	// Each influence's source arrives whole, label included, on both sides
	// of a cancelled keep.
	cancelled := keepCalc(`[9]`, ``,
		`{"rule":"cancelled","granted":[{"ref":"dnd5e:conditions:helped","name":"Helped","label":"help","source_id":"bob"}],"imposed":[{"ref":"dnd5e:conditions:prone","name":"Prone","label":"condition","source_id":"alice"}]}`, 9)
	roll, err = rollOf(cancelled, nil, 12, intp(16), boolp(false))
	s.Require().NoError(err)
	s.Equal([]RollInfluence{
		{Kind: InfluenceAdvantage, SourceRef: "dnd5e:conditions:helped", SourceID: "bob", Name: "Helped", SourceLabel: "help"},
		{Kind: InfluenceDisadvantage, SourceRef: "dnd5e:conditions:prone", SourceID: "alice", Name: "Prone", SourceLabel: "condition"},
	}, roll.Influences)
}

// TestRollModifierCarriesItsSource is #643 D (R17, "modifiers with
// sources"): Second Wind's healing modifier names its source by ref, name
// and role, not by a display name alone. Mutant: dropping the ref fill fails.
func (s *ActFoldSuite) TestRollModifierCarriesItsSource() {
	calc := json.RawMessage(`{"components":[
		{"source":{"ref":"dnd5e:features:second_wind","name":"Second Wind","source_id":"fighter"},"dice":{"notation":"1d10","die_size":10,"original_rolls":[6],"final_rolls":[6],"subtotal":6}},
		{"source":{"ref":"dnd5e:classes:fighter","name":"Fighter","label":"Fighter level","source_id":"fighter"},"modifier":1}
	],"total":7}`)
	roll, err := rollOf(calc, nil, 7, nil, nil)
	s.Require().NoError(err)
	s.Equal([]RollModifier{{
		Source: "Fighter", Value: 1,
		SourceRef: "dnd5e:classes:fighter", SourceID: "fighter", SourceLabel: "Fighter level",
	}}, roll.Modifiers)
	s.Equal("1d10+1", roll.Expression)
}

// TestRevealedDoorwaysNameTheirDoor is #643 E: the reveal beat hangs each
// door's doorways under the door, naming only the crossing; the act names
// the door on every doorway, as v1's projection did. A doorway that names
// its own door keeps it. Mutant: dropping the fill fails the first fixture.
func (s *ActFoldSuite) TestRevealedDoorwaysNameTheirDoor() {
	from, to := cell(4, 2), cell(5, 2)
	reveal := func(doorway map[string]any) *SecretRevealed {
		e := s.beat("search:01HZZZZZZZZZZZZZZZZZZZVALT", map[string]any{
			"beat": encounter.BeatConcealmentRevealed, "concealment": "concealed-vault",
			"doors": []map[string]any{{"door": "concealed-vault/vault-door", "state": "closed", "doorways": []map[string]any{doorway}}},
		})
		acts := foldActs("sess", "fighter", []record.Entry{e})
		s.Require().Len(acts, 1)
		p, ok := acts[0].Body.(Perception)
		s.Require().True(ok, "%T", acts[0].Body)
		s.Require().NotNil(p.Revealed)
		return p.Revealed
	}

	r := reveal(map[string]any{"from": from, "to": to})
	s.Require().Len(r.Doors, 1)
	s.Equal([]AtlasDoorway{{Door: "concealed-vault/vault-door", From: from, To: to}}, r.Doors[0].Doorways)

	r = reveal(map[string]any{"door": "concealed-vault/inner-door", "from": from, "to": to})
	s.Equal("concealed-vault/inner-door", r.Doors[0].Doorways[0].Door, "a doorway naming its own door keeps it")
}

// TestRevealedCrossingsKeepTheirMark is rpg-project#852 (H6): a footprint
// door's crossings arrive on both reveal beats marked `crossing`, and the act
// carries the mark onto the doorway; an edge door's doorway arrives unmarked
// and reads false. Mutant: dropping the field's tag fails both true rows.
func (s *ActFoldSuite) TestRevealedCrossingsKeepTheirMark() {
	from, to := cell(4, 2), cell(5, 2)
	perceived := func(payload map[string]any) Perception {
		acts := foldActs("sess", "fighter", []record.Entry{s.beat("search:01HZZZZZZZZZZZZZZZZZZZVALT", payload)})
		s.Require().Len(acts, 1)
		p, ok := acts[0].Body.(Perception)
		s.Require().True(ok, "%T", acts[0].Body)
		return p
	}
	secret := func(doorway map[string]any) []AtlasDoorway {
		p := perceived(map[string]any{
			"beat": encounter.BeatConcealmentRevealed, "concealment": "c",
			"doors": []map[string]any{{"door": "c/leaf", "state": "open", "doorways": []map[string]any{doorway}}},
		})
		s.Require().NotNil(p.Revealed)
		s.Require().Len(p.Revealed.Doors, 1)
		return p.Revealed.Doors[0].Doorways
	}
	s.Equal([]AtlasDoorway{{Door: "c/leaf", From: from, To: to, Crossing: true}},
		secret(map[string]any{"from": from, "to": to, "crossing": true}), "a revealed footprint door's crossing is marked")
	s.Equal([]AtlasDoorway{{Door: "c/leaf", From: from, To: to}},
		secret(map[string]any{"from": from, "to": to}), "an edge door's revealed doorway is not")

	cells := perceived(map[string]any{
		"beat": encounter.BeatCellsRevealed,
		"doorways": []map[string]any{
			{"door": "hall/gate", "from": from, "to": to, "crossing": true},
			{"door": "hall/edge", "from": to, "to": from},
		},
	})
	s.Require().NotNil(cells.Cells)
	s.Equal([]AtlasDoorway{
		{Door: "hall/gate", From: from, To: to, Crossing: true},
		{Door: "hall/edge", From: to, To: from},
	}, cells.Cells.Doorways, "cells learned carry each doorway's mark as the beat wrote it")
}

// TestCastActivationCarriesItsSpell is #643 F and rpg-toolkit#2100: Bane's
// cast, its save, then the activation-result applying Baned fold to an
// ActivateAct under the cast's act id that names the spell as its ability,
// with the Baned result inside. Mutant: continues returning an empty act
// fails it.
func (s *ActFoldSuite) TestCastActivationCarriesItsSpell() {
	const cast = "cast:01HZZZZZZZZZZZZZZZZZZZBANE"
	bane := map[string]string{"ref": "dnd5e:spells:bane", "name": "Bane"}
	entries := []record.Entry{
		s.beat(cast, map[string]any{"beat": encounter.BeatCast, "actor": "cultist", "spell": bane, "targets": []string{"fighter"}}),
		s.beat(cast, map[string]any{
			"beat": encounter.BeatSaved, "saver": "fighter", "ability": "cha", "roll": 4, "total": 5, "dc": 13,
			"succeeded": false, "source": bane,
		}),
		s.beat(cast, map[string]any{
			"beat": encounter.BeatActivationResult, "actor": "cultist",
			"result": map[string]any{"kind": "condition-applied", "target": "fighter", "ref": "dnd5e:conditions:baned", "name": "Baned", "source_id": "cultist"},
		}),
	}

	acts := foldActs("sess", "fighter", entries)

	// The result CONTINUES the cast in a later segment; it never absorbs
	// into the open CastAct, which would replace the cast's body and drop
	// the fighter's failed save.
	castAt, activateAt := -1, -1
	var castBody CastAct
	var activate *ActivateAct
	for i, a := range acts {
		s.Equal(cast, a.Act, "every piece stays under the cast's act id")
		switch body := a.Body.(type) {
		case CastAct:
			castAt, castBody = i, body
		case ActivateAct:
			activateAt, activate = i, &body
		}
	}
	s.Require().NotEqual(-1, castAt, "the cast keeps its own segment")
	s.Require().Len(castBody.Results, 1)
	s.Equal(Party{Kind: PartyMember, ID: "fighter"}, castBody.Results[0].Target)
	s.Equal(CastFailed, castBody.Results[0].Outcome, "the cast keeps the fighter's failed save")
	s.Require().NotNil(activate, "the Baned result rides an activate act")
	s.Greater(activateAt, castAt, "the activation is a separate, later segment")
	s.Greater(acts[activateAt].Segment, acts[castAt].Segment)
	s.Equal(AbilityRef{Ref: "dnd5e:spells:bane", Name: "Bane"}, activate.Ability)
	s.Equal(Party{Kind: PartyMember, ID: "cultist"}, activate.By)
	s.Require().Len(activate.Results, 1)
	s.Equal("dnd5e:conditions:baned", activate.Results[0].Ref)
	s.Equal("Baned", activate.Results[0].Name)
}

// TestStateEntriesNameTheirSource is rpg-toolkit#2100's sweep: the other
// state entries a cast or an activation leaves already name what they came
// from. A concentration's end names its spell, and a lifted condition names
// itself by ref and name.
func (s *ActFoldSuite) TestStateEntriesNameTheirSource() {
	entries := []record.Entry{
		s.beat("cast:01HZZZZZZZZZZZZZZZZZZZCONC", map[string]any{
			"beat": encounter.BeatConcentrationEnded, "caster": "cleric",
			"spell": map[string]string{"ref": "dnd5e:spells:bless", "name": "Bless"}, "reason": "damage",
		}),
		s.beat("activate:01HZZZZZZZZZZZZZZZZZZZLIFT", map[string]any{
			"beat": encounter.BeatActivationResult, "actor": "cleric",
			"result": map[string]any{"kind": "condition-removed", "target": "fighter", "ref": "dnd5e:conditions:baned", "name": "Baned", "source_id": "cultist", "reason": "ended"},
		}),
	}

	acts := foldActs("sess", "fighter", entries)

	s.Require().Len(acts, 2)
	ended, ok := acts[0].Body.(StateChange)
	s.Require().True(ok, "%T", acts[0].Body)
	s.Require().NotNil(ended.Concentration)
	s.Equal(SpellRef{Ref: "dnd5e:spells:bless", Name: "Bless"}, ended.Concentration.Spell)

	lifted, ok := acts[1].Body.(StateChange)
	s.Require().True(ok, "%T", acts[1].Body)
	s.Require().NotNil(lifted.Condition)
	s.Equal("dnd5e:conditions:baned", lifted.Condition.Ref)
	s.Equal("Baned", lifted.Condition.Name)
}

// TestFoldATableBeatIsNoAct is rpg-project#674's "record only, never the act
// stream": a creature's described table, written beside the pick that
// indexes it, folds into no act and leaves no undecoded piece — the pick is
// the only thing the recipient's acts carry.
func (s *ActFoldSuite) TestFoldATableBeatIsNoAct() {
	const pick = "time:01HZZZZZZZZZZZZZZZZZZZPICK"
	table := s.beat(pick, map[string]any{
		"beat": encounter.BeatTable, "creature": "gate-1", "option": "", "key": "time",
		"temper": "", "percents": map[string]int{"attack": 100},
		"rows": []map[string]any{{"entry": 0, "when": []string{"enemy seen"}, "word": "attack", "selector": "enemy"}},
	})
	answered := s.beat(pick, map[string]any{
		"beat": encounter.BeatAnswered, "creature": "gate-1", "key": "time", "roll": 1, "of": 1, "entry": 0,
		"word": "attack", "selector": "enemy", "candidates": []any{}, "rows": []any{},
	})

	withTable := foldActs("sess", "fighter", []record.Entry{table, answered})
	without := foldActs("sess", "fighter", []record.Entry{answered})

	s.Require().NotEmpty(withTable, "precondition: the pick folds")
	s.Equal(len(without), len(withTable), "the table adds no act")
	for _, act := range withTable {
		s.NotEqual(table.ID, act.ID, "no act is named by the table beat")
		s.Empty(act.Undecoded, "and nothing about it is undecoded")
	}
}

// TestFoldAnInteractionBeatIsNoAct is Gate S2's record-only interaction
// (rpg-project#922): the entrance's transition, written beside the Speak
// verdict that advanced it, folds into no act and leaves no undecoded piece —
// in particular no "has no family" entry. The next slice may project it.
func (s *ActFoldSuite) TestFoldAnInteractionBeatIsNoAct() {
	const pick = "time:01HZZZZZZZZZZZZZZZZZZZPICK"
	interaction := s.beat(pick, map[string]any{
		"beat": encounter.BeatInteraction, "interaction": "front-gate", "from": "fresh",
		"to": "warned-guest", "reason": "guest-failed", "option": "guest", "beaten": false,
	})
	answered := s.beat(pick, map[string]any{
		"beat": encounter.BeatAnswered, "creature": "gate-1", "key": "time", "roll": 1, "of": 1, "entry": 0,
		"word": "attack", "selector": "enemy", "candidates": []any{}, "rows": []any{},
	})

	s.Empty(foldActs("sess", "fighter", []record.Entry{interaction}), "the interaction alone is no act")

	withInteraction := foldActs("sess", "fighter", []record.Entry{interaction, answered})
	without := foldActs("sess", "fighter", []record.Entry{answered})

	s.Require().NotEmpty(withInteraction, "precondition: the pick folds")
	s.Equal(len(without), len(withInteraction), "the interaction adds no act")
	for _, act := range withInteraction {
		s.NotEqual(interaction.ID, act.ID, "no act is named by the interaction beat")
		s.Empty(act.Undecoded, "and nothing about it is undecoded")
	}
}

// Diagnostic evidence stays record-only even if an entry is accidentally
// audienced. It cannot become an undecoded clock act or a public state change.
func (s *ActFoldSuite) TestBehaviorDiagnosticsAreRecordOnly() {
	for _, kind := range []string{encounter.BeatBehaviorConfig, encounter.BeatBehaviorDecision, encounter.BeatBehaviorWork} {
		s.Run(kind, func() {
			entry := s.beat("time:01HZZZZZZZZZZZZZZZZZZZPICK", map[string]any{
				"beat": kind, "schema": 1, "member": "guard", "config": "original",
				"on": "heard", "operation": "observe", "status": "partial",
			})
			s.Empty(foldActs("sess", "fighter", []record.Entry{entry}), "diagnostics must not enter player narrative")
		})
	}
}

// TestFold_StateBeat is rpg-project#766 S6: the castle reserve's machine
// entering `marching` because the gate is fighting, written as the encounter
// writes it (machine.go appendStateBeat, audience the whole roster), folds
// into one state piece for every seat, the party naming the reserve.
func (s *ActFoldSuite) TestFold_StateBeat() {
	state := s.beat("", map[string]any{
		"beat": encounter.BeatState, "member": "reserve-1",
		"from": "waiting", "to": "marching", "reason": "the gate is fighting",
	})

	for _, seat := range []string{"alice", "bob", "fighter"} {
		acts := foldActs("sess", seat, []record.Entry{state})

		s.Require().Len(acts, 1, seat)
		s.Equal(FamilyState, acts[0].Family, seat)
		s.Empty(acts[0].Undecoded, seat)
		change, ok := acts[0].Body.(StateChange)
		s.Require().True(ok, "%s: %T", seat, acts[0].Body)
		s.Equal(StateChange{
			Party: Party{Kind: PartyMember, ID: "reserve-1"}, Change: StateMachine,
			Machine: &MachineChange{From: "waiting", To: "marching", Reason: "the gate is fighting"},
		}, change, seat)
	}
}

// TestFold_StateBeatNamingNoStateIsUndecoded is the state piece failing
// closed: a state beat naming no creature or no state entered is an
// undecoded entry, never a blank line on the panel. A beat naming no state
// left is the same: the writer steps only a machine that leaves one.
func (s *ActFoldSuite) TestFold_StateBeatNamingNoStateIsUndecoded() {
	for name, fields := range map[string]map[string]any{
		"no member": {"beat": encounter.BeatState, "from": "waiting", "to": "marching"},
		"no from":   {"beat": encounter.BeatState, "member": "reserve-1", "to": "marching"},
		"no to":     {"beat": encounter.BeatState, "member": "reserve-1", "from": "waiting"},
	} {
		acts := foldActs("sess", "alice", []record.Entry{s.beat("", fields)})

		s.Require().Len(acts, 1, name)
		s.Equal(FamilyState, acts[0].Family, name)
		s.NotEmpty(acts[0].Undecoded, name)
	}
}

// TestFold_Barred is rpg-project#842: a bump, written as the encounter
// writes it (barred.go), folds into one act. A step's names the cell it was
// refused into and no door; a door verb's names the door and the verb and
// no cell.
func (s *ActFoldSuite) TestFold_Barred() {
	cell := spatial.Position{X: -3, Y: 7}
	for name, tc := range map[string]struct {
		fields map[string]any
		want   BarredAct
	}{
		"a step": {
			fields: map[string]any{"beat": encounter.BeatBarred, "mover": "alice", "barrier": "gate-1", "cell": cell},
			want:   BarredAct{Mover: member("alice"), Barrier: member("gate-1"), Cell: &cell},
		},
		"a door verb": {
			fields: map[string]any{"beat": encounter.BeatBarred, "mover": "alice", "barrier": "gate-1", "door": "keep/gate", "verb": "unlock"},
			want:   BarredAct{Mover: member("alice"), Barrier: member("gate-1"), Door: "keep/gate", Verb: "unlock"},
		},
	} {
		acts := foldActs("sess", "alice", []record.Entry{s.beat("", tc.fields)})

		s.Require().Len(acts, 1, name)
		s.Equal(FamilyAct, acts[0].Family, name)
		s.Empty(acts[0].Undecoded, name)
		s.Equal(tc.want, acts[0].Body, name)
	}
}

// TestFold_BarredNamingNoPartyOrPlaceIsUndecoded is the bump failing
// closed: the writer names both parties and a cell or a door every time.
func (s *ActFoldSuite) TestFold_BarredNamingNoPartyOrPlaceIsUndecoded() {
	for name, fields := range map[string]map[string]any{
		"no mover":   {"beat": encounter.BeatBarred, "barrier": "gate-1", "door": "keep/gate"},
		"no barrier": {"beat": encounter.BeatBarred, "mover": "alice", "door": "keep/gate"},
		"no place":   {"beat": encounter.BeatBarred, "mover": "alice", "barrier": "gate-1"},
	} {
		acts := foldActs("sess", "alice", []record.Entry{s.beat("", fields)})

		s.Require().Len(acts, 1, name)
		s.Equal(FamilyAct, acts[0].Family, name)
		s.NotEmpty(acts[0].Undecoded, name)
	}
}

// TestFold_WakeCut is rpg-project#766 S6: a raise cut at the bound, written
// as the encounter writes it (wakebeats.go appendWakeCutBeat), folds into
// one clock entry naming the cut, what it matched and what it dropped.
func (s *ActFoldSuite) TestFold_WakeCut() {
	cut := s.beat("", map[string]any{"beat": encounter.BeatWakeCut, "matched": 4096, "dropped": 17})

	acts := foldActs("sess", "alice", []record.Entry{cut})

	s.Require().Len(acts, 1)
	s.Equal(FamilyClock, acts[0].Family)
	s.Empty(acts[0].Undecoded)
	entry, ok := acts[0].Body.(ClockEntry)
	s.Require().True(ok, "%T", acts[0].Body)
	s.Equal(ClockEntry{Mark: ClockWakeCut, WakeCut: &WakeCut{Matched: 4096, Dropped: 17}}, entry)
}

// TestFold_WakeCutWithNoCountIsUndecoded is the wake cut failing closed: the
// writer cuts only when it drops at least one beat, so a cut naming no count,
// or none dropped, is an undecoded entry, never a "0 dropped" that reads true.
func (s *ActFoldSuite) TestFold_WakeCutWithNoCountIsUndecoded() {
	for name, fields := range map[string]map[string]any{
		"no dropped":   {"beat": encounter.BeatWakeCut, "matched": 4096},
		"none dropped": {"beat": encounter.BeatWakeCut, "matched": 4096, "dropped": 0},
		"no matched":   {"beat": encounter.BeatWakeCut, "dropped": 17},
	} {
		acts := foldActs("sess", "alice", []record.Entry{s.beat("", fields)})

		s.Require().Len(acts, 1, name)
		s.Equal(FamilyClock, acts[0].Family, name)
		s.NotEmpty(acts[0].Undecoded, name)
	}
}
