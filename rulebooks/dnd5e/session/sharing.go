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
//   - one entry per perceived fact several perceivers were each told
//     (a secret revealed, a sound heard): hers if she has one ([onePerFact]).
//
// SHOWN IS NOT LEARNED. Two settings act at two levels. A character's own
// setting for discoveries decides whether a find becomes the other players'
// KNOWLEDGE: shared, the engine writes it to every placed player, each gets
// their own reveal, and their maps hold it. The dial at shared view decides
// what a player is SHOWN: a friend's find whatever that setting, an entry in
// her story marked via the friend, which gives her character nothing. A
// stop where a deliberately private find outranks the dial is not built:
// the entries carry no mark that tells a chosen "private" from the default.
//
// At each their own, and for anyone outside the party, nothing is merged:
// the stream is exactly the recipient's own story, folded as before.

// sharingParty is who shares at the encounter's dial: THE ONE MEMBERSHIP
// READ (rpg-project#935, Q1), taken once per read from the run's current
// roster. Nil when the dial is not shared view.
//
// MEMBERSHIP IS A FACT OF THE ENTRY'S TIME. Whom an entry is shared from and
// to is decided by who was a player in the run when it was written, read
// from the record's own `joined` and `exited` entries — never from a clock,
// and never from today's roster alone — so a reload, a catch-up after a
// friend leaves, and the live publish all read the same answer:
//
//   - FROM a friend: an entry is in a friend's story only if it was written
//     while they were in the run, so their story is already exactly what
//     they perceived as a member. Their sharers are the current roster's
//     players and every member the recipient's own story shows leaving as a
//     player ([storyFor]).
//   - TO a recipient: a friend's entry reaches them only if it was written
//     while they were in the run, by their own `joined` and `exited`
//     entries ([presence]). A late joiner is not shown what friends
//     perceived before they arrived; one who left is shown nothing after.
type sharingParty struct {
	// players are the current roster's player characters, by id.
	players []encounter.MemberID
	// inRoster is every current roster member, any kind.
	inRoster map[encounter.MemberID]bool
	// departed are the members who have left the run and were players when
	// they did, read by a catch-up ([sharingParty.departedFrom]); nil on the
	// live path, where a member who left before this commit perceived
	// nothing in it, and one who left in it is in the recipient's own delta.
	departed []encounter.MemberID
}

// departedFrom adds every member who has left the run as a player: each of
// everMembers not in the current roster, its kind read off its own story's
// membership entries — the latest that names a kind. A member whose entries
// name none (a record older than encounter v0.205.1) is kind unknown, and no
// sharer ([partyMember]).
func (p *sharingParty) departedFrom(everMembers []encounter.MemberID,
	storyOf func(encounter.MemberID) []record.Entry) {
	if p == nil {
		return
	}
	for _, id := range everMembers {
		if p.inRoster[id] {
			continue
		}
		kind := kindUnknown
		for _, m := range membershipsIn(storyOf(id)) {
			if m.member == id && m.kind != kindUnknown {
				kind = m.kind
			}
		}
		if partyMember(kind) {
			p.departed = append(p.departed, id)
		}
	}
}

// partyMember is the one rule for who is of the party that shares (Q1, to
// start): a player character. Not a creature, not a creature allied to the
// players by stance, not a member of the `party` faction by authoring — the
// rule names the kind, so widening it to "the party faction" or "allied to
// the party" is this one function.
//
// KIND UNKNOWN IS NOT A PLAYER. An `exited` entry written before the record
// named the kind (encounter v0.205.1) says nothing of what left, and the
// member is on no roster to ask; the merge does not guess, so such a member
// is no sharer — acceptable before release.
func partyMember(kind encounter.MemberKind) bool {
	if kind == kindUnknown {
		return false
	}
	return kind == encounter.KindPlayer
}

// dataParty reads the party from one roster read — the world view a commit
// already built, or the stored world a catch-up loaded. Nil unless the dial
// is shared view; shared knowledge is refused at launch
// ([encounter.LaunchableSharing]), so no run reads it.
func dataParty(sharing encounter.Sharing, roster []encounter.MemberData) *sharingParty {
	if sharing != encounter.SharingView {
		return nil
	}
	p := &sharingParty{inRoster: make(map[encounter.MemberID]bool, len(roster))}
	for _, m := range roster {
		p.inRoster[m.ID] = true
		if partyMember(m.Kind) {
			p.players = append(p.players, m.ID)
		}
	}
	slices.Sort(p.players)
	return p
}

// membership is one `joined` or `exited` entry: who, and whether they came
// or went, and — on an exit — the kind they left as.
type membership struct {
	seq    uint64
	member encounter.MemberID
	joined bool
	kind   encounter.MemberKind
}

// kindUnknown is the kind an older record's membership entry states: none.
const kindUnknown encounter.MemberKind = ""

// membershipsIn is every `joined` and `exited` entry in a story, in record
// order. Only entries tagged as membership are decoded.
func membershipsIn(entries []record.Entry) []membership {
	var out []membership
	for _, e := range entries {
		if e.Tags["tag"] != "membership" {
			continue
		}
		var p struct {
			Beat   string               `json:"beat"`
			Member encounter.MemberID   `json:"member"`
			Kind   encounter.MemberKind `json:"kind"`
		}
		if json.Unmarshal(e.Payload, &p) != nil || (p.Beat != encounter.BeatJoined && p.Beat != encounter.BeatExited) {
			continue
		}
		out = append(out, membership{seq: e.Seq, member: p.Member, joined: p.Beat == encounter.BeatJoined, kind: p.Kind})
	}
	return out
}

// presence answers whether member was in the run when the entry at a seq
// was written, from member's own membership entries in their story slice:
// in from each `joined`, out from each `exited`. Before the slice's first
// such entry the answer is the opposite of it; a slice holding none — a
// commit's delta mid-run, or a story trimmed past them — answers as the
// current roster does.
func presence(member encounter.MemberID, own []membership, inRosterNow bool) func(seq uint64) bool {
	var mine []membership
	for _, m := range own {
		if m.member == member {
			mine = append(mine, m)
		}
	}
	return func(seq uint64) bool {
		if len(mine) == 0 {
			return inRosterNow
		}
		in := !mine[0].joined
		for _, m := range mine {
			if m.seq > seq {
				break
			}
			in = m.joined
		}
		return in
	}
}

// storyFor is recipient's story as the dial shows it: their own, merged
// with each friend's over the same window at shared view
// ([sharedStory]). Their friends are the current roster's players and the
// players their own story shows leaving, never themself; a recipient who is
// not a player — a creature, an ally by stance — is shown only their own.
func (p *sharingParty) storyFor(recipient encounter.MemberID, own []record.Entry,
	storyOf func(encounter.MemberID) []record.Entry) []storyEntry {
	if p == nil {
		return ownStory(own)
	}
	moves := membershipsIn(own)
	isPlayer := slices.Contains(p.players, recipient)
	friends := make([]encounter.MemberID, 0, len(p.players))
	for _, id := range append(slices.Clone(p.players), p.departed...) {
		if id != recipient {
			friends = append(friends, id)
		}
		if id == recipient {
			isPlayer = true
		}
	}
	for _, m := range moves {
		if m.joined {
			continue
		}
		if !partyMember(m.kind) { // a creature, or kind unknown
			continue
		}
		if m.member == recipient {
			isPlayer = true
		} else if !slices.Contains(friends, m.member) {
			friends = append(friends, m.member)
		}
	}
	if !isPlayer {
		return ownStory(own)
	}
	slices.Sort(friends)
	return sharedStory(own, friends, storyOf, presence(recipient, moves, p.inRoster[recipient]))
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
// speech's utterance, an answer's creature, whether it is a stripped pick,
// and — for a perception every perceiver is told in an entry of their own —
// the one fact it is about.
type entryHead struct {
	beat      string
	utterance string
	creature  string
	stripped  bool
	// fact names what a per-perceiver entry is about, so each perceiver's
	// copy of one fact is recognised as one: a secret revealed (its
	// concealment), a sound heard (its kind and where it was made, under
	// one act at one moment). "" for every other entry.
	fact string
}

// headOf decodes payload's [entryHead] — one decode per entry merged.
func headOf(e record.Entry) entryHead {
	var p struct {
		Beat        string           `json:"beat"`
		Utterance   string           `json:"utterance"`
		Creature    string           `json:"creature"`
		Concealment string           `json:"concealment"`
		Kind        string           `json:"kind"`
		Position    *json.RawMessage `json:"position"`
	}
	if json.Unmarshal(e.Payload, &p) != nil {
		return entryHead{}
	}
	h := entryHead{beat: p.Beat}
	switch p.Beat {
	case encounter.BeatSpeech:
		h.utterance = p.Utterance
	case encounter.BeatAnswered:
		h.creature, h.stripped = p.Creature, strippedPickOf(e.Payload)
	case encounter.BeatConcealmentRevealed:
		h.fact = "revealed\x00" + p.Concealment
	case encounter.BeatHeard:
		where := ""
		if p.Position != nil {
			where = string(*p.Position)
		}
		h.fact = fmt.Sprintf("heard\x00%s\x00%s\x00%s\x00%d", p.Kind, where, e.Tags[encounter.ActTag], e.At)
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
// id order, each story the same window of the record): every entry once, in
// record order — a friend's only when present says the recipient was in the
// run when it was written — then one entry per utterance, one per perceived
// fact and one pick per answer. With no friends it is [ownStory].
func sharedStory(own []record.Entry, friends []encounter.MemberID, storyOf func(encounter.MemberID) []record.Entry,
	present func(seq uint64) bool) []storyEntry {
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
			if !held && !present(e.Seq) {
				continue // written while the recipient was not in the run
			}
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
		out[i].head = headOf(out[i].Entry)
	}
	return pairedPicks(onePerFact(oneSpeechPerUtterance(out)))
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

// onePerFact keeps the recipient's own entries of a fact several perceivers
// were each told in an entry of their own (entryHead.fact), and collapses
// friends' copies of it.
//
// THE MERGE NEVER DROPS OR MERGES TWO OF THE RECIPIENT'S OWN ENTRIES: two
// clangs she heard are two clangs, however alike. A fact's copies are
// counted per perceiver, in record order: her k-th copy stands for every
// friend's k-th copy, which are dropped; beyond her own count, the friends'
// k-th copies collapse into ONE entry — the earliest — naming every friend
// whose k-th copy it was. So a friend who heard two clangs she did not is
// two entries through him, and two friends who heard the one clang are one.
//
// The setting that decides whether a find becomes the party's KNOWLEDGE is
// the character's: when a finder shares discoveries the engine writes each
// player their own reveal, and so nobody's comes through a friend.
func onePerFact(entries []storyEntry) []storyEntry {
	type copies struct {
		own     []int
		friends map[encounter.MemberID][]int
	}
	facts := map[string]*copies{}
	for i, e := range entries {
		f := e.head.fact
		if f == "" {
			continue
		}
		c := facts[f]
		if c == nil {
			c = &copies{friends: map[encounter.MemberID][]int{}}
			facts[f] = c
		}
		if e.via == nil {
			c.own = append(c.own, i)
			continue
		}
		for _, id := range e.via {
			c.friends[id] = append(c.friends[id], i)
		}
	}
	if len(facts) == 0 {
		return entries
	}
	drop := map[int]bool{}
	via := map[int][]encounter.MemberID{}
	for _, c := range facts {
		depth := 0
		for _, list := range c.friends {
			depth = max(depth, len(list))
		}
		for k := 0; k < depth; k++ {
			stand := -1
			if k < len(c.own) {
				stand = c.own[k]
			}
			for _, list := range c.friends {
				if k < len(list) && (stand < 0 || (k >= len(c.own) && list[k] < stand)) {
					stand = list[k]
				}
			}
			for id, list := range c.friends {
				if k >= len(list) {
					continue
				}
				if list[k] != stand {
					drop[list[k]] = true
				}
				if k >= len(c.own) && !slices.Contains(via[stand], id) {
					via[stand] = append(via[stand], id)
				}
			}
		}
	}
	out := entries[:0:0]
	for i, e := range entries {
		if drop[i] {
			continue
		}
		if names, ok := via[i]; ok {
			slices.Sort(names)
			e.via = names
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
	// WHOSE PERCEPTION IT IS: the recipient's own, or — through a friend —
	// that friend's. A perception names its Observer, and an entry a
	// friend's story carried is the friend's perception, never the reader's
	// (the reader is the act's Recipient; Via says through whom). Where one
	// entry reached several friends, the first by id is named, and Via
	// lists them all.
	observer := recipient
	if e.via != nil {
		observer = string(e.via[0])
	}
	return marked(e, piecesOf(observer, tag, e.Payload))
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
