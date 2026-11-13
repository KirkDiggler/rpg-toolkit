// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// sharing.go is THE PARTY'S SHARING DIAL at the act stream (rpg-project#930
// R5, R11; rpg-project#935): at shared view, a player's acts are folded from
// their own story merged with every party friend's.
//
// IT LIVES ONLY IN THE ACT READ PATH. The merge reads stories the encounter
// already decided; it never asks the encounter anything else, never writes,
// and no verb, offer or rule reads what it builds ([Manager.Afford],
// [Manager.Speak] and every check read the encounter, never an act). So
// sharing changes what a player is SHOWN, never what the rules decided or
// whom the record told.
//
// THE MERGE, PER RECIPIENT (R11):
//
//   - one copy of each record entry: an entry in the recipient's own story is
//     theirs; one only a friend holds is marked [Perceived.Via] with every
//     friend whose story holds it;
//   - one [SpeechAct] per utterance: the recipient's own version, with each
//     part they lack taken from any friend's version, and Via naming exactly
//     the friends whose versions held a part the recipient's own lacked
//     ([mergedSpeech]);
//   - one pick per creature's answer: a seer who caught none of a line holds
//     the stripped pick and the line's catchers the full one — two entries of
//     ONE answer — and the merged story keeps the full one ([pairedPicks]).
//
// At each their own, and for anyone outside the party, nothing is merged:
// the stream is exactly the recipient's own story, folded as before.

// sharingParty is who shares at the encounter's dial: THE ONE MEMBERSHIP
// READ (rpg-project#935, Q1). Nil when the dial is not shared view.
type sharingParty struct {
	// members are the sharers, by id.
	members []encounter.MemberID
}

// partyMember is the one rule for who is of the party that shares (Q1, to
// start): a player character in the run's current roster. Not a creature,
// not a creature allied to the players by stance, not a member of the
// `party` faction by authoring — the rule names the kind, so widening it to
// "the party faction" or "allied to the party" is this one function.
//
// A PLAYER WHO HAS LEFT THE RUN is not in the current roster (an exit or a
// departure removes them; [encounter.EncounterData.EverMembers] keeps only
// their id), so they share nothing and are shared nothing: their own
// catch-up is their own story, and what they perceived while they were here
// is no longer merged into a friend's catch-up.
func partyMember(kind encounter.MemberKind) bool { return kind == encounter.KindPlayer }

// newSharingParty reads the party from one roster read: every member
// [partyMember] admits, sorted by id. Nil unless the dial is shared view;
// shared knowledge never reaches here, because the encounter refuses to
// launch or load it ([encounter.ErrSharedKnowledgeNotBuilt]).
func newSharingParty[M any](sharing encounter.Sharing, roster []M, of func(M) (encounter.MemberID, encounter.MemberKind)) *sharingParty {
	if sharing != encounter.SharingView {
		return nil
	}
	p := &sharingParty{}
	for _, m := range roster {
		if id, kind := of(m); partyMember(kind) {
			p.members = append(p.members, id)
		}
	}
	slices.Sort(p.members)
	return p
}

// rosterParty is the party read from a live roster ([encounter.Encounter.Members]).
func rosterParty(sharing encounter.Sharing, roster []encounter.Member) *sharingParty {
	return newSharingParty(sharing, roster, func(m encounter.Member) (encounter.MemberID, encounter.MemberKind) {
		return m.ID, m.Kind
	})
}

// dataParty is the party read from a world view's roster.
func dataParty(sharing encounter.Sharing, roster []encounter.MemberData) *sharingParty {
	return newSharingParty(sharing, roster, func(m encounter.MemberData) (encounter.MemberID, encounter.MemberKind) {
		return m.ID, m.Kind
	})
}

// friendsOf is who shares with recipient: every other party member, or none
// when the recipient is not of the party (a creature, a player who has
// left) or nobody shares.
func (p *sharingParty) friendsOf(recipient encounter.MemberID) []encounter.MemberID {
	if p == nil || !slices.Contains(p.members, recipient) {
		return nil
	}
	out := make([]encounter.MemberID, 0, len(p.members)-1)
	for _, id := range p.members {
		if id != recipient {
			out = append(out, id)
		}
	}
	return out
}

// storyEntry is one entry a recipient's acts are folded from: the record
// entry, and how it reached them.
type storyEntry struct {
	record.Entry
	// via are the friends whose stories hold it, in party order; nil when it
	// is in the recipient's own story.
	via []encounter.MemberID
	// versions are, for the one `speech` entry that stands for an
	// utterance, every version of that utterance the merge holds, in record
	// order, its own among them. Nil for every other entry.
	versions []utteranceVersion
	// head is what the merge reads of the payload, decoded once.
	head entryHead
}

// entryHead is what the merge reads of an entry's payload: its beat, a
// speech's utterance, an answer's creature, and whether it is a stripped
// pick.
type entryHead struct {
	beat      string
	utterance string
	creature  string
	stripped  bool
}

// headOf decodes payload's [entryHead] — one decode per entry merged.
func headOf(payload []byte) entryHead {
	var p struct {
		Beat      string `json:"beat"`
		Utterance string `json:"utterance"`
		Creature  string `json:"creature"`
	}
	if json.Unmarshal(payload, &p) != nil {
		return entryHead{}
	}
	h := entryHead{beat: p.Beat}
	switch p.Beat {
	case encounter.BeatSpeech:
		h.utterance = p.Utterance
	case encounter.BeatAnswered:
		h.creature, h.stripped = p.Creature, strippedPickOf(payload)
	}
	return h
}

// utteranceVersion is one perceiver's version of an utterance, and whose it is.
type utteranceVersion struct {
	payload []byte
	// via is nil for the recipient's own version, else the friends whose
	// version it is.
	via []encounter.MemberID
}

// ownStory is a recipient's own entries, as the fold reads them: nothing
// merged, nothing marked.
func ownStory(entries []record.Entry) []storyEntry {
	out := make([]storyEntry, len(entries))
	for i, e := range entries {
		out[i] = storyEntry{Entry: e}
	}
	return out
}

// sharedStory is recipient's story merged with each friend's (the friends in
// party order, each story the same window of the record): every entry once,
// in record order, then one entry per utterance and one pick per answer.
// With no friends it is [ownStory].
func sharedStory(own []record.Entry, friends []encounter.MemberID, storyOf func(encounter.MemberID) []record.Entry) []storyEntry {
	if len(friends) == 0 {
		return ownStory(own)
	}
	bySeq := make(map[uint64]int, len(own))
	out := ownStory(own)
	for i, e := range own {
		bySeq[e.Seq] = i
	}
	for _, friend := range friends {
		for _, e := range storyOf(friend) {
			i, held := bySeq[e.Seq]
			if !held {
				bySeq[e.Seq] = len(out)
				out = append(out, storyEntry{Entry: e, via: []encounter.MemberID{friend}})
				continue
			}
			if out[i].via != nil {
				out[i].via = append(out[i].via, friend)
			}
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Seq < out[b].Seq })
	for i := range out {
		out[i].head = headOf(out[i].Payload)
	}
	return pairedPicks(oneSpeechPerUtterance(out))
}

// oneSpeechPerUtterance keeps one `speech` entry per utterance — the
// recipient's own version where they have one, else the first in record
// order — carrying every version, and drops the rest.
func oneSpeechPerUtterance(entries []storyEntry) []storyEntry {
	stand := map[string]int{}
	for i, e := range entries {
		u := e.head.utterance
		if u == "" {
			continue
		}
		at, seen := stand[u]
		if !seen || (entries[at].via != nil && e.via == nil) {
			stand[u] = i
		}
	}
	if len(stand) == 0 {
		return entries
	}
	versions := map[string][]utteranceVersion{}
	for _, e := range entries {
		if u := e.head.utterance; u != "" {
			versions[u] = append(versions[u], utteranceVersion{payload: e.Payload, via: e.via})
		}
	}
	out := entries[:0:0]
	for i, e := range entries {
		u := e.head.utterance
		if u == "" {
			out = append(out, e)
			continue
		}
		if stand[u] != i {
			continue
		}
		if len(versions[u]) > 1 {
			e.versions = versions[u]
		}
		out = append(out, e)
	}
	return out
}

// pairedPicks drops a stripped pick whose full pick the merged story also
// holds (rpg-project#948): the encounter writes a creature's answer to a line
// as the full pick, to the line's catchers, and IMMEDIATELY AFTER it the
// stripped pick, to the seers who caught none of it — two entries of one
// answer. The full one stands, marked as it reached the recipient: theirs if
// they caught the line, else via the friends who did.
func pairedPicks(entries []storyEntry) []storyEntry {
	out := entries[:0:0]
	for i, e := range entries {
		if i > 0 && e.head.stripped && fullPickFor(entries[i-1], e) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// strippedPickOf reports whether payload is a stripped pick: an `answered`
// beat carrying nothing but who answered and the cue they are seen
// performing ([encounter]'s strippedPick names exactly those).
func strippedPickOf(payload []byte) bool {
	var keys map[string]json.RawMessage
	if json.Unmarshal(payload, &keys) != nil {
		return false
	}
	var beat string
	if json.Unmarshal(keys["beat"], &beat) != nil || beat != encounter.BeatAnswered {
		return false
	}
	for k := range keys {
		if k != "beat" && k != "creature" && k != "activity" {
			return false
		}
	}
	return true
}

// fullPickFor reports whether prev is the full pick stripped was cut from:
// the record entry just before it, an answer by the same creature under the
// same act.
func fullPickFor(prev, stripped storyEntry) bool {
	return prev.Seq+1 == stripped.Seq && prev.Tags[encounter.ActTag] == stripped.Tags[encounter.ActTag] &&
		prev.head.beat == encounter.BeatAnswered && !prev.head.stripped &&
		prev.head.creature != "" && prev.head.creature == stripped.head.creature
}

// spoken is one decoded version of an utterance.
type spoken struct {
	act    SpeechAct
	senses []Sense
	via    []encounter.MemberID
}

// mergedSpeech is the one [SpeechAct] a recipient is shown of an utterance
// several party members perceived (R11): the recipient's own version, each
// part they lack taken from a friend's version in record order, and
// [Perceived] naming the senses of the versions merged and exactly the
// friends whose versions held a part the recipient's own lacked. Every
// version is decoded by [speechActPiece], so each fails closed as the
// recipient's own would; one that does not read fails the entry.
func mergedSpeech(versions []utteranceVersion) (piece, error) {
	var own *spoken
	var friends []spoken
	for _, v := range versions {
		p, err := speechActPiece(v.payload)
		if err != nil {
			return piece{}, err
		}
		s := spoken{act: p.body.(SpeechAct), senses: p.perceived.Senses, via: v.via}
		if v.via == nil {
			own = &s
			continue
		}
		friends = append(friends, s)
	}
	merged := SpeechAct{}
	var senses []Sense
	if own != nil {
		merged, senses = own.act, own.senses
	}
	var via []encounter.MemberID
	for i, f := range friends {
		if own != nil && !lacksAPart(own.act, f.act) {
			continue
		}
		if own == nil && i == 0 {
			merged = SpeechAct{Utterance: f.act.Utterance, Place: f.act.Place, Hops: f.act.Hops}
		}
		fillSpeech(&merged, f.act)
		senses = append(senses, f.senses...)
		for _, id := range f.via {
			if !slices.Contains(via, id) {
				via = append(via, id)
			}
		}
	}
	if merged.Speaker != (Party{}) {
		merged.Hops = nil
	}
	slices.Sort(via)
	perceived := &Perceived{Senses: orderedSenses(senses)}
	for _, id := range via {
		perceived.Via = append(perceived.Via, member(string(id)))
	}
	return piece{family: FamilyAct, body: merged, aside: true, perceived: perceived}, nil
}

// lacksAPart reports whether a friend's version holds a part the
// recipient's own does not.
func lacksAPart(own, friend SpeechAct) bool {
	return (own.Speaker == Party{} && friend.Speaker != Party{}) ||
		(own.SpokenTo == Party{} && friend.SpokenTo != Party{}) ||
		(own.Words == nil && friend.Words != nil) ||
		(own.Option == "" && friend.Option != "") ||
		(own.Fact == "" && friend.Fact != "") ||
		(own.Skill == "" && friend.Skill != "") ||
		(own.Roll == nil && friend.Roll != nil) ||
		(own.Loudness == "" && friend.Loudness != "")
}

// fillSpeech sets every part merged lacks from a friend's version. The roll
// travels with the words (R3): it is only ever present beside them.
func fillSpeech(merged *SpeechAct, friend SpeechAct) {
	if merged.Speaker == (Party{}) {
		merged.Speaker = friend.Speaker
	}
	if merged.SpokenTo == (Party{}) {
		merged.SpokenTo = friend.SpokenTo
	}
	if merged.Words == nil && friend.Words != nil {
		words := *friend.Words
		merged.Words = &words
	}
	if merged.Option == "" {
		merged.Option = friend.Option
	}
	if merged.Fact == "" {
		merged.Fact = friend.Fact
	}
	if merged.Skill == "" {
		merged.Skill = friend.Skill
	}
	if merged.Roll == nil {
		merged.Roll = friend.Roll
	}
	if merged.Loudness == "" {
		merged.Loudness = friend.Loudness
	}
}

// orderedSenses is a union of speech senses in [Perceived]'s order: sight,
// then hearing.
func orderedSenses(senses []Sense) []Sense {
	var out []Sense
	for _, s := range []Sense{SenseSight, SenseHearing} {
		if slices.Contains(senses, s) {
			out = append(out, s)
		}
	}
	return out
}

// marked sets how an entry reached its recipient on each of its pieces: a
// perception keeps its sense, and an entry a friend's story carried names
// them in Via.
func marked(e storyEntry, pieces []piece) []piece {
	if e.via == nil {
		return pieces
	}
	via := make([]Party, len(e.via))
	for i, id := range e.via {
		via[i] = member(string(id))
	}
	for i := range pieces {
		p := Perceived{Via: via}
		if pieces[i].perceived != nil {
			p.Senses = pieces[i].perceived.Senses
		}
		pieces[i].perceived = &p
	}
	return pieces
}

// entryPieces is what one story entry contributes: the merged speech for an
// utterance several members perceived, else [piecesOf], marked.
func entryPieces(recipient, tag string, e storyEntry) []piece {
	if len(e.versions) > 0 {
		p, err := mergedSpeech(e.versions)
		if err != nil {
			return []piece{{family: FamilyPerception, undecoded: fmt.Sprintf("%s: %v", encounter.BeatSpeech, err)}}
		}
		return []piece{p}
	}
	return marked(e, piecesOf(recipient, tag, e.Payload))
}

// sameVia reports whether two acts reached their recipient through the same
// friends: a piece that came through a friend never folds into a segment the
// recipient perceived themself, nor the other way about.
func sameVia(a, b *Perceived) bool {
	var av, bv []Party
	if a != nil {
		av = a.Via
	}
	if b != nil {
		bv = b.Via
	}
	return slices.Equal(av, bv)
}
