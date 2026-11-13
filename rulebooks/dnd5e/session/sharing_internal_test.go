// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// sharing_internal_test.go is the merge itself (sharing.go, rpg-project#935
// R11): what one recipient's acts are folded from at shared view, on record
// entries built by hand so every rule is asked of exactly one case.

// to is entry e told to the members given.
func to(e record.Entry, members ...string) record.Entry {
	e.Audience = nil
	for _, m := range members {
		e.Audience = append(e.Audience, core.EntityID(m))
	}
	return e
}

// storiesOf is each member's story of the entries: those addressed to them,
// in record order — what [encounter.Encounter.Story] answers.
func storiesOf(entries ...record.Entry) func(encounter.MemberID) []record.Entry {
	return func(id encounter.MemberID) []record.Entry {
		var out []record.Entry
		for _, e := range entries {
			for _, a := range e.Audience {
				if string(a) == string(id) {
					out = append(out, e)
				}
			}
		}
		return out
	}
}

// sharedActs folds recipient's acts from the entries merged with friends'.
func sharedActs(recipient string, friends []string, entries ...record.Entry) []Act {
	story := storiesOf(entries...)
	ids := make([]encounter.MemberID, len(friends))
	for i, f := range friends {
		ids[i] = encounter.MemberID(f)
	}
	return foldActs("sess", recipient, sharedStory(story(encounter.MemberID(recipient)), ids, story))
}

// viaOf is an act's Via, nil when it has no Perceived.
func viaOf(a Act) []Party {
	if a.Perceived == nil {
		return nil
	}
	return a.Perceived.Via
}

// TestNoFriendsIsTheOwnFold: with no friend — each their own, a creature, a
// player who has left — the merged story is the recipient's own, entry for
// entry, and folds to exactly the acts it always did.
func (s *ActFoldSuite) TestNoFriendsIsTheOwnFold() {
	walk := "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	entries := []record.Entry{
		to(s.step(walk, "alice", cell(1, 0)), "alice", "bob"),
		to(s.beat("", map[string]any{"beat": encounter.BeatHeard, "kind": "clang", "position": cell(3, 3), "hops": 1}), "bob"),
		to(s.step(walk, "alice", cell(2, 0)), "alice", "bob"),
	}
	own := storiesOf(entries...)("alice")
	s.Equal(ownStory(own), sharedStory(own, nil, storiesOf(entries...)))
	s.Equal(foldActs("sess", "alice", ownStory(own)), sharedActs("alice", nil, entries...))
}

// TestAFriendsEntryArrivesOnceMarkedVia: an entry the recipient holds is
// theirs, however many friends also hold it; one only friends hold arrives
// once, after the entries before it in the record, naming every friend
// whose story held it.
func (s *ActFoldSuite) TestAFriendsEntryArrivesOnceMarkedVia() {
	shared := to(s.beat("", map[string]any{"beat": encounter.BeatTick}), "alice", "bob", "carol")
	clang := to(s.beat("", map[string]any{"beat": encounter.BeatHeard, "kind": "clang", "position": cell(3, 3), "hops": 1}), "bob", "carol")
	mine := to(s.beat("", map[string]any{"beat": encounter.BeatHeard, "kind": "boom", "position": cell(0, 0), "hops": 0}), "alice")

	acts := sharedActs("alice", []string{"bob", "carol"}, shared, clang, mine)
	s.Require().Len(acts, 3, "each entry once")
	s.Equal([]string{shared.ID, clang.ID, mine.ID}, []string{acts[0].ID, acts[1].ID, acts[2].ID}, "in record order")
	s.Nil(acts[0].Perceived, "the tick is hers")

	heard := acts[1].Body.(Perception)
	s.Equal("clang", heard.Heard.Kind)
	s.Equal(member("alice"), heard.Observer, "the recipient, as every perception names")
	s.Equal(&Perceived{Senses: []Sense{SenseHearing}, Via: []Party{member("bob"), member("carol")}}, acts[1].Perceived,
		"heard, through both friends who heard it")
	s.Equal(&Perceived{Senses: []Sense{SenseHearing}}, acts[2].Perceived, "her own: the sense, and no via")
}

// TestAFriendsSpeechFillsWhatTheRecipientLacked is R11 per utterance: the
// recipient's own version, each part she lacked from a friend's, Via exactly
// the friends who held such a part, the senses their union.
func (s *ActFoldSuite) TestAFriendsSpeechFillsWhatTheRecipientLacked() {
	check := map[string]any{"skill": "persuasion", "dc": 10, "total": 15, "beaten": true}
	beaten := true
	roll := &Roll{Total: 15, Against: intp(10), Beaten: &beaten}
	words := "Evening."
	for _, tc := range []struct {
		name     string
		versions []record.Entry
		want     SpeechAct
		perceive Perceived
	}{
		{
			name: "she saw it said; a friend caught the words",
			versions: []record.Entry{
				to(s.beat("", speechVersion(map[string]any{"speaker": "cleric", "to": "pedlar", "words": words, "option": "tonight", "check": check})), "cleric"),
				to(s.beat("", speechVersion(map[string]any{"speaker": "cleric"})), "alice"),
			},
			want: SpeechAct{Utterance: "u1", Speaker: member("cleric"), SpokenTo: member("pedlar"), Words: &words,
				Option: "tonight", Place: cell(4, 2), Loudness: "speak", Skill: "persuasion", Roll: roll},
			perceive: Perceived{Senses: []Sense{SenseSight, SenseHearing}, Via: []Party{member("cleric")}},
		},
		{
			name: "she perceived nothing; the speaker's own version is all of it",
			versions: []record.Entry{
				to(s.beat("", speechVersion(map[string]any{"speaker": "cleric", "to": "pedlar", "words": words, "option": "tonight", "check": check})), "cleric"),
			},
			want: SpeechAct{Utterance: "u1", Speaker: member("cleric"), SpokenTo: member("pedlar"), Words: &words,
				Option: "tonight", Place: cell(4, 2), Loudness: "speak", Skill: "persuasion", Roll: roll},
			perceive: Perceived{Senses: []Sense{SenseSight, SenseHearing}, Via: []Party{member("cleric")}},
		},
		{
			name: "she caught it all; a friend's lesser version adds nothing",
			versions: []record.Entry{
				to(s.beat("", speechVersion(map[string]any{"speaker": "cleric"})), "bob"),
				to(s.beat("", speechVersion(map[string]any{"speaker": "cleric", "to": "pedlar", "words": words, "option": "tonight", "check": check})), "alice"),
			},
			want: SpeechAct{Utterance: "u1", Speaker: member("cleric"), SpokenTo: member("pedlar"), Words: &words,
				Option: "tonight", Place: cell(4, 2), Loudness: "speak", Skill: "persuasion", Roll: roll},
			perceive: Perceived{Senses: []Sense{SenseSight, SenseHearing}},
		},
		{
			name: "two friends: one saw who, one heard what, unseen",
			versions: []record.Entry{
				to(s.beat("", speechVersion(map[string]any{"speaker": "cleric"})), "bob"),
				to(s.beat("", speechVersion(map[string]any{"words": words, "option": "tonight", "check": check, "hops": 1})), "carol"),
			},
			want: SpeechAct{Utterance: "u1", Speaker: member("cleric"), Words: &words,
				Option: "tonight", Place: cell(4, 2), Loudness: "speak", Skill: "persuasion", Roll: roll},
			perceive: Perceived{Senses: []Sense{SenseSight, SenseHearing}, Via: []Party{member("bob"), member("carol")}},
		},
		{
			name: "she heard it unseen; a friend saw who: the speaker named, no distance",
			versions: []record.Entry{
				to(s.beat("", speechVersion(map[string]any{"words": words, "option": "tonight", "check": check, "hops": 1})), "alice"),
				to(s.beat("", speechVersion(map[string]any{"speaker": "cleric"})), "bob"),
			},
			want: SpeechAct{Utterance: "u1", Speaker: member("cleric"), Words: &words,
				Option: "tonight", Place: cell(4, 2), Loudness: "speak", Skill: "persuasion", Roll: roll},
			perceive: Perceived{Senses: []Sense{SenseSight, SenseHearing}, Via: []Party{member("bob")}},
		},
	} {
		s.Run(tc.name, func() {
			acts := sharedActs("alice", []string{"bob", "carol", "cleric"}, tc.versions...)
			s.Require().Len(acts, 1, "one Speech act per utterance")
			s.Equal(tc.want, acts[0].Body)
			s.Equal(&tc.perceive, acts[0].Perceived)
		})
	}
}

// TestOnePickPerAnswerTheFullest: a creature's answer to a line is a full
// pick to the line's catchers and, the next entry, a stripped pick to the
// seers who caught none of it. At shared view the recipient gets ONE pick,
// the fullest — marked via the friends who caught the line when she did not.
func (s *ActFoldSuite) TestOnePickPerAnswerTheFullest() {
	tag := "speak:01HZZZZZZZZZZZZZZZZZZZLINE"
	full := map[string]any{"beat": encounter.BeatAnswered, "creature": "captain", "roll": 7, "of": 20, "entry": 1,
		"candidates": []map[string]any{{"entry": 1, "weight": 1, "percent": 100}}}
	stripped := map[string]any{"beat": encounter.BeatAnswered, "creature": "captain"}

	s.Run("she saw it and caught nothing; the cleric caught the line", func() {
		acts := sharedActs("alice", []string{"cleric"},
			to(s.beat(tag, full), "cleric"), to(s.beat(tag, stripped), "alice"))
		s.Require().Len(acts, 1, "one pick, never two")
		s.NotNil(acts[0].Body.(TableAnswer).Roll, "the fullest")
		s.Equal([]Party{member("cleric")}, viaOf(acts[0]))
	})
	s.Run("she caught the line; a friend only saw", func() {
		acts := sharedActs("alice", []string{"bob"},
			to(s.beat(tag, full), "alice"), to(s.beat(tag, stripped), "bob"))
		s.Require().Len(acts, 1)
		s.NotNil(acts[0].Body.(TableAnswer).Roll)
		s.Nil(acts[0].Perceived, "hers")
	})
	s.Run("only a friend saw it answer", func() {
		acts := sharedActs("alice", []string{"bob"},
			to(s.beat(tag, full), "cleric"), to(s.beat(tag, stripped), "bob"))
		s.Require().Len(acts, 1)
		s.Equal(TableAnswer{Who: member("captain")}, acts[0].Body, "who answered, as bob saw it")
		s.Equal([]Party{member("bob")}, viaOf(acts[0]))
	})
	s.Run("two answers by one creature that are not a pair are both kept", func() {
		acts := sharedActs("alice", []string{"bob"},
			to(s.beat(tag, full), "alice"),
			to(s.beat("", map[string]any{"beat": encounter.BeatTick}), "alice", "bob"),
			to(s.beat(tag, stripped), "bob"))
		s.Len(acts, 3, "a stripped pick pairs only with the entry right before it")
	})
}

// TestAFriendsStepNeverFoldsIntoHerWalk: a walk segment holds only steps
// that reached the recipient the same way, so a step only a friend saw is
// its own act, marked, beside hers.
func (s *ActFoldSuite) TestAFriendsStepNeverFoldsIntoHerWalk() {
	walk := "move:01HZZZZZZZZZZZZZZZZZZZWALK"
	acts := sharedActs("alice", []string{"bob"},
		to(s.step(walk, "goblin", cell(1, 0)), "alice", "bob"),
		to(s.step(walk, "goblin", cell(2, 0)), "bob"),
	)
	s.Require().Len(acts, 2)
	s.Nil(acts[0].Perceived)
	s.Equal([]Party{member("bob")}, viaOf(acts[1]))
}

// TestTheMergeNeverRepeatsOrReordersAnEntry: every merged story is in record
// order with each record entry at most once — the precondition of record
// order and same-id adjacency in the fold.
func (s *ActFoldSuite) TestTheMergeNeverRepeatsOrReordersAnEntry() {
	var entries []record.Entry
	audiences := [][]string{{"alice"}, {"bob"}, {"alice", "bob"}, {"carol"}, {"bob", "carol"}, {"alice", "carol"}}
	for i := 0; i < 30; i++ {
		entries = append(entries, to(s.beat("", map[string]any{"beat": encounter.BeatTick}), audiences[i%len(audiences)]...))
	}
	story := storiesOf(entries...)
	merged := sharedStory(story("alice"), []encounter.MemberID{"bob", "carol"}, story)
	s.Len(merged, len(entries), "every entry somebody in the party holds, once")
	for i := 1; i < len(merged); i++ {
		s.Less(merged[i-1].Seq, merged[i].Seq)
	}
}

// TestThePartyIsThePlayersAtSharedViewOnly: the one membership read —
// players of the current roster, sorted; nobody at each their own, or for a
// recipient outside the party.
func (s *ActFoldSuite) TestThePartyIsThePlayersAtSharedViewOnly() {
	roster := []encounter.MemberData{
		{ID: "carol", Kind: encounter.KindPlayer}, {ID: "captain", Kind: encounter.KindMonster},
		{ID: "alice", Kind: encounter.KindPlayer}, {ID: "companion", Kind: encounter.KindMonster},
	}
	party := dataParty(encounter.SharingView, roster)
	s.Equal([]encounter.MemberID{"alice", "carol"}, party.members)
	s.Equal([]encounter.MemberID{"carol"}, party.friendsOf("alice"))
	s.Empty(party.friendsOf("companion"), "a creature, allied or not, shares nothing")
	s.Empty(party.friendsOf("dave"), "nor does anyone not in the roster — a player who has left")
	s.Nil(dataParty(encounter.SharingOwn, roster), "each their own: no party")
	s.Nil(dataParty(encounter.SharingKnowledge, roster), "shared knowledge never launches; were it here, nothing would be merged")
}
