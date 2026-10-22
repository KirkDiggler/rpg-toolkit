// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// act_filter_internal_test.go pins THE PERCEPTION FILTER (rpg-project#631
// E6, event-stream R35) on entries shaped exactly as the encounter writes
// them into one recipient's story. Each test names the mutant it kills.

import (
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sighted is the recipient's own sighting under act tag.
func (s *ActFoldSuite) sighted(tag string, gained, lost []string) record.Entry {
	return s.beat(tag, map[string]any{"beat": encounter.BeatSighted, "gained": gained, "lost": lost})
}

// subjects is every member an act is about, for "none of these is X".
func subjects(acts []Act) []string {
	var out []string
	for _, a := range acts {
		if a.Body != nil {
			out = append(out, subjectOf(a.Body).ID)
		}
	}
	return out
}

// TestFilterAWalkIntoViewStartsWithTheSighting: a guard walks a, b out of
// the fighter's sight; b's sight refresh gains him; he walks on to c and d.
// The fighter receives the sighting first, then one walk from b — the step
// that brought him into view — and never a.
//
// Mutants: dropping the hold loses b (the walk starts at c); emitting the
// walk before the sighting fails the order.
func (s *ActFoldSuite) TestFilterAWalkIntoViewStartsWithTheSighting() {
	const walk = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	a, b, c, d := cell(1, 0), cell(2, 0), cell(3, 0), cell(4, 0)
	entries := []record.Entry{
		s.step(walk, "guard", a),
		s.step(walk, "guard", b),
		s.sighted(walk, []string{"guard"}, nil),
		s.step(walk, "guard", c),
		s.step(walk, "guard", d),
	}

	acts := actsAfter("sess", "fighter", entries, "")

	s.Require().Len(acts, 2)
	s.Equal(FamilyPerception, acts[0].Family, "the sighting comes first")
	s.Equal([]string{"guard"}, acts[0].Body.(Perception).Sighted.Gained)
	s.Equal(FamilyAct, acts[1].Family)
	s.Equal([]spatial.Position{b, c, d}, path(acts[1].Body.(WalkAct)), "the walk starts at the step into view")
	s.Equal(entries[4].ID, acts[1].ID)

	s.Run("resuming after any act is the suffix", func() {
		for i, act := range acts {
			s.Equal(orEmpty(acts[i+1:]), orEmpty(actsAfter("sess", "fighter", entries, act.ID)), "after %s", act.ID)
		}
	})
}

// TestFilterAStepIntoViewAloneSharesItsSightingsID: a guard who takes one
// step into view and stops. The released step follows its sighting, so it
// is named by the sighting's id: the two are one delivery unit, and a
// resume after the sighting does not hand the sighting back.
//
// Mutant: naming the released step by its own entry fails the id.
func (s *ActFoldSuite) TestFilterAStepIntoViewAloneSharesItsSightingsID() {
	const walk = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	entries := []record.Entry{
		s.step(walk, "guard", cell(1, 0)),
		s.sighted(walk, []string{"guard"}, nil),
		s.beat("", map[string]any{"beat": encounter.BeatTick, "tick": 2}),
	}

	acts := actsAfter("sess", "fighter", entries, "")

	s.Require().Len(acts, 3)
	s.Equal(entries[1].ID, acts[0].ID)
	s.Equal(entries[1].ID, acts[1].ID, "the step into view is named by the sighting that released it")
	s.IsType(WalkAct{}, acts[1].Body)

	after := actsAfter("sess", "fighter", entries, entries[1].ID)
	s.Require().Len(after, 1)
	s.Equal(entries[2].ID, after[0].ID)
}

// TestFilterAWalkOutOfViewEndsWithTheLeavingStep: a seen guard walks a, b;
// b takes him out of view; he walks on to c and d. The fighter receives the
// walk a, b, then the lost sighting, and nothing after.
//
// Mutant: delivering every step fails the trailing walk.
func (s *ActFoldSuite) TestFilterAWalkOutOfViewEndsWithTheLeavingStep() {
	const walk = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	a, b := cell(1, 0), cell(2, 0)
	entries := []record.Entry{
		s.sighted("", []string{"guard"}, nil),
		s.step(walk, "guard", a),
		s.step(walk, "guard", b),
		s.sighted(walk, nil, []string{"guard"}),
		s.step(walk, "guard", cell(3, 0)),
		s.step(walk, "guard", cell(4, 0)),
	}

	acts := actsAfter("sess", "fighter", entries, entries[0].ID)

	s.Require().Len(acts, 2)
	s.Equal([]spatial.Position{a, b}, path(acts[0].Body.(WalkAct)), "the leaving step is delivered")
	s.Equal([]string{"guard"}, acts[1].Body.(Perception).Sighted.Lost)
}

// TestFilterAWalkBackIntoViewStartsWhereItIsSeen: the guard leaves view and
// comes back in the same walk. The second segment starts at the step that
// brought him back, not at the last cell the fighter saw — the steps between
// were never the fighter's.
//
// Mutant: continuing the walk across undelivered steps starts it at b.
func (s *ActFoldSuite) TestFilterAWalkBackIntoViewStartsWhereItIsSeen() {
	const walk = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	back := cell(5, 0)
	entries := []record.Entry{
		s.sighted("", []string{"guard"}, nil),
		s.step(walk, "guard", cell(1, 0)),
		s.step(walk, "guard", cell(2, 0)),
		s.sighted(walk, nil, []string{"guard"}),
		s.step(walk, "guard", cell(3, 0)),
		s.step(walk, "guard", back),
		s.sighted(walk, []string{"guard"}, nil),
	}

	acts := actsAfter("sess", "fighter", entries, entries[0].ID)

	s.Require().Len(acts, 4)
	s.Equal([]spatial.Position{back}, path(acts[3].Body.(WalkAct)))
	s.Equal(2, acts[3].Segment)
}

// TestFilterHeardDeliversOnlyTheSound: the bard, out of sight, hears the
// kitchen door forced. Her story holds the forcer's steps (a moved beat goes
// to the roster) and her own `heard` — never the door beat, which the
// encounter audiences to the door's witnesses. She receives the sound and
// nothing of the forcer.
//
// Mutant: a filter that delivers everything fails it.
func (s *ActFoldSuite) TestFilterHeardDeliversOnlyTheSound() {
	const walk, force = "move:01HZZZZZZZZZZZZZZZZZZZWALK", "open_door:01HZZZZZZZZZZZZZZZZZZFORCE"
	entries := []record.Entry{
		s.step(walk, "fighter", cell(1, 0)),
		s.step(walk, "fighter", cell(2, 0)),
		s.beat(force, map[string]any{
			"beat": encounter.BeatHeard, "listener": "bard", "kind": encounter.SoundCrash,
			"position": cell(3, 0), "hops": 1,
		}),
		s.step(force, "fighter", cell(3, 0)),
	}

	acts := actsAfter("sess", "bard", entries, "")

	s.Require().Len(acts, 1)
	heard := acts[0].Body.(Perception)
	s.Equal(SenseHearing, heard.Sense)
	s.Equal(encounter.SoundCrash, heard.Heard.Kind)
}

// TestFilterOwnAndAddressedActsAlways: the fighter sees no one, and still
// receives her own walk, her line and both answers under it (the second
// answer arrives because the act is hers), a swing at her from out of
// sight, a cast that targets her, and the window asking her.
//
// Mutants: dropping the own rule loses the second answer; dropping the
// addressed rule loses the swing, the cast and the window.
func (s *ActFoldSuite) TestFilterOwnAndAddressedActsAlways() {
	const (
		walk   = "move:01HZZZZZZZZZZZZZZZZZZZWALK"
		line   = "speak:01HZZZZZZZZZZZZZZZZZZLINE"
		swing  = "attack:01HZZZZZZZZZZZZZZZZZSWNG"
		cast   = "cast:01HZZZZZZZZZZZZZZZZZZCAST"
		window = "move:01HZZZZZZZZZZZZZZZZZZZWNDW"
	)
	attack := map[string]string{"ref": "dnd5e:weapons:shortbow", "name": "Shortbow", "damage_type": "piercing"}
	entries := []record.Entry{
		s.step(walk, "fighter", cell(1, 0)),
		s.beat(line, map[string]any{"beat": encounter.BeatSpoke, "actor": "fighter", "target": "voice", "option": "who-goes"}),
		s.beat(line, map[string]any{"beat": encounter.BeatAnswered, "creature": "voice", "key": "success", "say": "A friend."}),
		s.beat(line, map[string]any{"beat": encounter.BeatAnswered, "creature": "voice", "key": "after", "say": "Probably."}),
		s.beat(swing, map[string]any{
			"beat": string(encounter.OutcomeMissed), "actor": "archer", "targets": []string{"fighter"},
			"roll": 3, "total": 8, "against": 16, "attack": attack,
		}),
		s.beat(cast, map[string]any{
			"beat": encounter.BeatCast, "actor": "cultist", "targets": []string{"fighter"},
			"spell": map[string]string{"ref": "dnd5e:spells:bane", "name": "Bane"},
		}),
		s.beat(window, map[string]any{
			"beat": encounter.BeatWindowOpened, "member": "lurker", "from": cell(1, 1), "to": cell(2, 1),
			"windows": []map[string]any{{"audience": "fighter", "reaction": map[string]string{"ref": "r", "name": "OA"}}},
		}),
		s.step(window, "lurker", cell(2, 1)),
	}

	acts := actsAfter("sess", "fighter", entries, "")

	s.Require().Len(acts, 6)
	s.Equal(Party{Kind: PartyMember, ID: "fighter"}, acts[0].Body.(WalkAct).Mover)
	s.Equal("A friend.", acts[1].Body.(SpeakAct).Answer.Say)
	s.Equal("Probably.", acts[2].Body.(TableAnswer).Say, "an answer under her line is hers")
	s.Equal("archer", acts[3].Body.(AttackAct).By.ID)
	s.Equal("cultist", acts[4].Body.(CastAct).By.ID)
	s.Equal(ClockWindowOpened, acts[5].Body.(ClockEntry).Mark)
	s.NotContains(subjects(acts), "lurker", "the window is hers; the unseen lurker's step is not")
}

// TestFilterClockAndRoster: tick reaches everyone; a fight's forming, its
// turns and its end reach only its members; a stance turn with the party
// reaches every party member, and one between two other factions reaches
// none; an unseen arrival reaches the recipient only after the sighting
// that gains it.
//
// Mutant: delivering clock entries by kind alone fails the bard's half.
func (s *ActFoldSuite) TestFilterClockAndRoster() {
	const (
		turn  = "end_turn:01HZZZZZZZZZZZZZZZZZZTURN"
		alarm = "time:01HZZZZZZZZZZZZZZZZZZALARM"
	)
	entries := []record.Entry{
		s.beat("", map[string]any{"beat": encounter.BeatTick, "tick": 1}),
		s.beat("", map[string]any{"beat": encounter.BeatFightStarted, "order": []string{"alice", "goblin"}}),
		s.beat("", map[string]any{"beat": encounter.BeatStance, "between": []string{"goblins", encounter.FactionParty}, "stance": "hostile"}),
		s.beat("", map[string]any{"beat": encounter.BeatStance, "between": []string{"kitchen", "watch"}, "stance": "hostile"}),
		s.beat(turn, map[string]any{"beat": encounter.BeatTurnEnded, "member": "goblin", "next": "alice"}),
		s.beat(alarm, map[string]any{"beat": encounter.BeatArrived, "id": "reserve-1", "kind": "monster", "cell": cell(9, 9)}),
		s.beat(alarm, map[string]any{"beat": encounter.BeatArrived, "id": "reserve-2", "kind": "monster", "cell": cell(9, 8)}),
		s.sighted(alarm, []string{"reserve-1"}, nil),
		s.beat("", map[string]any{"beat": encounter.BeatFightEnded, "members": []string{"alice", "goblin"}, "cause": "defeat"}),
		s.beat("", map[string]any{"beat": encounter.BeatTurnEnded, "member": "goblin", "next": "alice"}),
	}
	marks := func(acts []Act) []string {
		var out []string
		for _, a := range acts {
			switch b := a.Body.(type) {
			case ClockEntry:
				m := string(b.Mark)
				if b.Arrival != nil {
					m += " " + b.Arrival.ID
				}
				out = append(out, m)
			case StateChange:
				out = append(out, string(b.Change)+" "+b.Stance.Between[0].ID)
			case Perception:
				out = append(out, "sighted")
			}
		}
		return out
	}

	s.Equal([]string{
		"tick", "fight_formed", "stance goblins", "turn_ended", "sighted", "arrived reserve-1", "fight_dissolved",
	}, marks(actsAfter("sess", "alice", entries, "")), "alice is in the fight; reserve-2 never came into view")

	var bard []record.Entry
	for _, e := range entries {
		if peekBeat(e.Payload) != encounter.BeatSighted { // alice's sighting is hers alone
			bard = append(bard, e)
		}
	}
	s.Equal([]string{"tick", "stance goblins"}, marks(actsAfter("sess", "bard", bard, "")),
		"the bard is in no fight and sees no reserve")
}

// TestFilterTimePicksOfTheUnseen: a world round's picks reach the recipient
// only for the creatures they see.
//
// Mutant: a filter that skips table answers delivers the cook's pick.
func (s *ActFoldSuite) TestFilterTimePicksOfTheUnseen() {
	pick := func(tag, creature string) record.Entry {
		return s.beat(tag, map[string]any{"beat": encounter.BeatAnswered, "creature": creature, "key": "time", "word": "hold"})
	}
	entries := []record.Entry{
		s.sighted("", []string{"captain"}, nil),
		pick("time:01HZZZZZZZZZZZZZZZZZZZZZZZZ1", "captain"),
		pick("time:01HZZZZZZZZZZZZZZZZZZZZZZZZ2", "cook"),
	}

	acts := actsAfter("sess", "alice", entries, entries[0].ID)

	s.Require().Len(acts, 1)
	s.Equal("captain", acts[0].Body.(TableAnswer).Who.ID)
}

// TestFilterSeedsTheViewFromBeforeTheFold: a catch-up after a later id
// still knows who the recipient saw at first light — the prepass replays
// the sightings before the fold's start.
//
// Mutant: seeding the view from nothing drops the guard's walk.
func (s *ActFoldSuite) TestFilterSeedsTheViewFromBeforeTheFold() {
	entries := []record.Entry{
		s.sighted("", []string{"guard"}, nil),
		s.beat("", map[string]any{"beat": encounter.BeatTick, "tick": 1}),
		s.step("move:01HZZZZZZZZZZZZZZZZZZZWALK", "guard", cell(1, 0)),
	}

	acts := actsAfter("sess", "fighter", entries, entries[1].ID)

	s.Require().Len(acts, 1)
	s.Equal("guard", acts[0].Body.(WalkAct).Mover.ID)
}

// TestFilterAPropArrivesWhenTheRecipientsViewOfThingsChanges: the letter
// arrives under the fighter's end of turn, and the arrival's own sight
// refresh tells her that her view of things changed — she receives the
// arrival after that sighting. A second letter arriving where her view of
// things does not change never reaches her.
//
// Mutant: releasing only on a gained member drops the letter.
func (s *ActFoldSuite) TestFilterAPropArrivesWhenTheRecipientsViewOfThingsChanges() {
	const turn, later = "end_turn:01HZZZZZZZZZZZZZZZZZZTURN", "end_turn:01HZZZZZZZZZZZZZZZZZLATER"
	entries := []record.Entry{
		s.beat(turn, map[string]any{"beat": encounter.BeatArrived, "id": "letter", "kind": encounter.ArrivedProp, "cell": cell(0, 3)}),
		s.beat(turn, map[string]any{"beat": encounter.BeatSighted, "knowledge_changed": true}),
		s.beat(later, map[string]any{"beat": encounter.BeatArrived, "id": "far-letter", "kind": encounter.ArrivedProp, "cell": cell(40, 3)}),
		s.beat(later, map[string]any{"beat": encounter.BeatSighted, "gained": []string{"scout"}}),
	}

	var arrived []string
	for _, a := range actsAfter("sess", "fighter", entries, "") {
		if c, ok := a.Body.(ClockEntry); ok && c.Arrival != nil {
			arrived = append(arrived, c.Arrival.ID)
			s.Equal(entries[1].ID, a.ID, "the arrival follows the sighting that showed it")
		}
	}
	s.Equal([]string{"letter"}, arrived)
}
