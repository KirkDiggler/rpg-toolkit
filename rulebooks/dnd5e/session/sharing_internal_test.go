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

// always is a recipient in the run for every entry.
func always(uint64) bool { return true }

// sharedActs folds recipient's acts from the entries merged with friends'.
func sharedActs(recipient string, friends []string, entries ...record.Entry) []Act {
	story := storiesOf(entries...)
	ids := make([]encounter.MemberID, len(friends))
	for i, f := range friends {
		ids[i] = encounter.MemberID(f)
	}
	return foldActs("sess", recipient, sharedStory(story(encounter.MemberID(recipient)), ids, story, always))
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
	s.Equal(ownStory(own), sharedStory(own, nil, storiesOf(entries...), always))
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
	s.Equal(member("bob"), heard.Observer, "whose perception it is: bob's, the first friend by id; alice only reads it")
	s.Equal("alice", acts[1].Recipient)
	s.Equal(member("alice"), acts[2].Body.(Perception).Observer, "her own is hers")
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
	merged := sharedStory(story("alice"), []encounter.MemberID{"bob", "carol"}, story, always)
	s.Len(merged, len(entries), "every entry somebody in the party holds, once")
	for i := 1; i < len(merged); i++ {
		s.Less(merged[i-1].Seq, merged[i].Seq)
	}
}

// TestThePartyIsThePlayersAtSharedViewOnly: the one membership read —
// the current roster's players; nobody at each their own; a recipient who
// is not a player is shown only their own story.
func (s *ActFoldSuite) TestThePartyIsThePlayersAtSharedViewOnly() {
	roster := []encounter.MemberData{
		{ID: "carol", Kind: encounter.KindPlayer}, {ID: "captain", Kind: encounter.KindMonster},
		{ID: "alice", Kind: encounter.KindPlayer}, {ID: "companion", Kind: encounter.KindMonster},
	}
	party := dataParty(encounter.SharingView, roster)
	s.Equal([]encounter.MemberID{"alice", "carol"}, party.players)
	s.True(partyMember(encounter.KindPlayer))
	s.False(partyMember(encounter.KindMonster), "a creature")
	s.False(partyMember(kindUnknown), "kind unknown — an older record's exit — is no player")
	s.Nil(dataParty(encounter.SharingOwn, roster), "each their own: no party")
	s.Nil(dataParty(encounter.SharingKnowledge, roster), "shared knowledge never launches; were it here, nothing would be merged")

	sighted := to(s.beat("", map[string]any{"beat": encounter.BeatSighted, "gained": []string{"goblin"}}), "carol")
	story := storiesOf(sighted)
	s.Len(party.storyFor("alice", nil, story), 1, "alice is shown carol's sighting")
	s.Empty(party.storyFor("companion", nil, story), "a creature, allied or not, is shown nothing of the party's")
	s.Empty(party.storyFor("dave", nil, story), "nor is anyone not of the party")
}

// membershipEntry is a `joined` or `exited` entry for member, told to the
// audience given.
func (s *ActFoldSuite) membershipEntry(beat, member string, kind encounter.MemberKind, audience ...string) record.Entry {
	fields := map[string]any{"beat": beat, "member": member}
	if kind != "" {
		fields["kind"] = kind
	}
	e := to(s.beat("", fields), audience...)
	e.Tags = map[string]string{"tag": "membership"}
	return e
}

// TestMembershipIsAFactOfTheEntrysTime: whom an entry is shared from and to
// is who was a player in the run when it was written, read off the record's
// own joined and exited entries — a late joiner is shown nothing friends
// perceived before she came, a friend's entries from before he left stay in
// her story after he has gone, and nothing reaches anyone after they leave.
func (s *ActFoldSuite) TestMembershipIsAFactOfTheEntrysTime() {
	sight := func(who string) record.Entry {
		return to(s.beat("", map[string]any{"beat": encounter.BeatSighted, "gained": []string{"goblin"}}), who)
	}
	sightsVia := func(entries []storyEntry) []string {
		var out []string
		for _, e := range entries {
			if e.via != nil {
				out = append(out, e.ID)
			}
		}
		return out
	}

	s.Run("a late joiner is not shown what a friend perceived before she came", func() {
		before := sight("bob")
		joined := s.membershipEntry(encounter.BeatJoined, "alice", "", "alice", "bob")
		after := sight("bob")
		party := dataParty(encounter.SharingView, []encounter.MemberData{
			{ID: "alice", Kind: encounter.KindPlayer}, {ID: "bob", Kind: encounter.KindPlayer}})
		story := storiesOf(before, joined, after)
		s.Equal([]string{after.ID}, sightsVia(party.storyFor("alice", story("alice"), story)))
	})
	s.Run("a friend who left: what he perceived while here stays; nothing after", func() {
		here := sight("bob")
		left := s.membershipEntry(encounter.BeatExited, "bob", encounter.KindPlayer, "alice", "bob")
		party := dataParty(encounter.SharingView, []encounter.MemberData{{ID: "alice", Kind: encounter.KindPlayer}})
		story := storiesOf(here, left)
		s.Equal([]string{here.ID}, sightsVia(party.storyFor("alice", story("alice"), story)),
			"bob is a sharer by his exit, and his sighting before it is hers")
	})
	s.Run("a member whose exit names no kind (an older record) is no sharer", func() {
		here := sight("bob")
		left := s.membershipEntry(encounter.BeatExited, "bob", "", "alice", "bob")
		party := dataParty(encounter.SharingView, []encounter.MemberData{{ID: "alice", Kind: encounter.KindPlayer}})
		story := storiesOf(here, left)
		s.Empty(sightsVia(party.storyFor("alice", story("alice"), story)), "kind unknown is not a player: nothing is guessed")
	})
	s.Run("a creature that left is never a sharer", func() {
		here := sight("captain")
		left := s.membershipEntry(encounter.BeatExited, "captain", encounter.KindMonster, "alice", "captain")
		party := dataParty(encounter.SharingView, []encounter.MemberData{{ID: "alice", Kind: encounter.KindPlayer}})
		story := storiesOf(here, left)
		s.Empty(sightsVia(party.storyFor("alice", story("alice"), story)))
	})
	s.Run("one who left is shown nothing after; what came before stays", func() {
		before := sight("bob")
		left := s.membershipEntry(encounter.BeatExited, "alice", encounter.KindPlayer, "alice", "bob")
		after := sight("bob")
		party := dataParty(encounter.SharingView, []encounter.MemberData{{ID: "bob", Kind: encounter.KindPlayer}})
		story := storiesOf(before, left, after)
		s.Equal([]string{before.ID}, sightsVia(party.storyFor("alice", story("alice"), story)))
	})
	s.Run("a friend who leaves and rejoins shares both stays, not the gap", func() {
		first := sight("bob")
		left := s.membershipEntry(encounter.BeatExited, "bob", encounter.KindPlayer, "alice", "bob")
		gap := sight("carol") // carol is no friend of alice's here; the gap holds nothing of bob's
		back := s.membershipEntry(encounter.BeatJoined, "bob", "", "alice", "bob")
		second := sight("bob")
		party := dataParty(encounter.SharingView, []encounter.MemberData{
			{ID: "alice", Kind: encounter.KindPlayer}, {ID: "bob", Kind: encounter.KindPlayer}})
		story := storiesOf(first, left, gap, back, second)
		s.Equal([]string{first.ID, second.ID}, sightsVia(party.storyFor("alice", story("alice"), story)))
	})
}

// TestOneEntryPerPerceivedFact: a fact every perceiver is told in an entry
// of their own — a secret revealed, a sound heard — reaches the recipient
// once: her own entry when she has one, unmarked; else one entry naming
// every friend who was told it.
func (s *ActFoldSuite) TestOneEntryPerPerceivedFact() {
	reveal := func(concealment string) map[string]any {
		return map[string]any{"beat": encounter.BeatConcealmentRevealed, "concealment": concealment}
	}
	clang := map[string]any{"beat": encounter.BeatHeard, "kind": "clang", "position": cell(3, 3), "hops": 1}
	revealed := func(acts []Act) map[string][]Party {
		out := map[string][]Party{}
		for _, a := range acts {
			if p, ok := a.Body.(Perception); ok && p.Revealed != nil {
				s.Require().NotContains(out, p.Revealed.Concealment, "one act per secret")
				out[p.Revealed.Concealment] = viaOf(a)
			}
		}
		return out
	}

	s.Run("her own reveal stands; the friends' copies of it add nothing", func() {
		acts := sharedActs("alice", []string{"bob", "carol"},
			to(s.beat("", reveal("vault")), "bob"), to(s.beat("", reveal("vault")), "alice"), to(s.beat("", reveal("vault")), "carol"))
		s.Equal(map[string][]Party{"vault": nil}, revealed(acts))
	})
	s.Run("two friends told one secret: one reveal, through both", func() {
		acts := sharedActs("alice", []string{"bob", "carol"},
			to(s.beat("", reveal("vault")), "carol"), to(s.beat("", reveal("vault")), "bob"))
		s.Equal(map[string][]Party{"vault": {member("bob"), member("carol")}}, revealed(acts))
	})
	s.Run("two secrets are two reveals", func() {
		acts := sharedActs("alice", []string{"bob"},
			to(s.beat("", reveal("vault")), "bob"), to(s.beat("", reveal("crypt")), "bob"))
		s.Equal(map[string][]Party{"vault": {member("bob")}, "crypt": {member("bob")}}, revealed(acts))
	})
	s.Run("a clang she and a friend each heard is one, hers", func() {
		bobs, hers := to(s.beat("", clang), "bob"), to(s.beat("", clang), "alice")
		hers.At = bobs.At // one noise: the same moment
		acts := sharedActs("alice", []string{"bob"}, bobs, hers)
		s.Require().Len(acts, 1)
		s.Nil(viaOf(acts[0]))
		s.Equal(hers.ID, acts[0].ID, "hers")

		later := to(s.beat("", clang), "bob")
		s.Len(sharedActs("alice", []string{"bob"}, bobs, hers, later), 2, "a second clang at another moment is its own")
	})
}

// TestHerOwnTwoOfAKindAreNeverMerged: the merge may drop a friend's copy of
// a fact she holds, and collapse friends' copies of one fact into one; it
// never drops or merges two of her own entries.
func (s *ActFoldSuite) TestHerOwnTwoOfAKindAreNeverMerged() {
	clang := map[string]any{"beat": encounter.BeatHeard, "kind": "clang", "position": cell(3, 3), "hops": 1}
	same := func(es ...record.Entry) []record.Entry {
		for i := range es {
			es[i].At = es[0].At
		}
		return es
	}
	ownCount := func(acts []Act) (own, via int) {
		for _, a := range acts {
			if len(viaOf(a)) == 0 {
				own++
			} else {
				via++
			}
		}
		return own, via
	}

	s.Run("two of her own, alike in every way, are two", func() {
		es := same(to(s.beat("", clang), "alice"), to(s.beat("", clang), "alice"))
		own := foldActs("sess", "alice", ownStory(es))
		view := sharedActs("alice", []string{"bob"}, es...)
		s.Len(own, 2)
		s.Equal(own, view, "at shared view as at her own")
	})
	s.Run("two of hers and one of a friend's: her two, his dropped", func() {
		es := same(to(s.beat("", clang), "alice"), to(s.beat("", clang), "bob"), to(s.beat("", clang), "alice"))
		o, v := ownCount(sharedActs("alice", []string{"bob"}, es...))
		s.Equal(2, o)
		s.Zero(v)
	})
	s.Run("one of hers and two of a friend's: hers, and his second through him", func() {
		es := same(to(s.beat("", clang), "bob"), to(s.beat("", clang), "alice"), to(s.beat("", clang), "bob"))
		o, v := ownCount(sharedActs("alice", []string{"bob"}, es...))
		s.Equal(1, o)
		s.Equal(1, v)
	})
	s.Run("two friends' one clang is one, through both", func() {
		es := same(to(s.beat("", clang), "bob"), to(s.beat("", clang), "carol"))
		acts := sharedActs("alice", []string{"bob", "carol"}, es...)
		s.Require().Len(acts, 1)
		s.Equal([]Party{member("bob"), member("carol")}, viaOf(acts[0]))
	})
}
