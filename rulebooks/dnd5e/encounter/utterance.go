// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/sound"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BeatSpeech is the "beat" value of one observer group's version of one
// utterance (rpg-project#933, R8): every spoken thing — a character's line, a
// creature's answer, a hail, a bump answer, a creature speaking on its own
// time — writes one such beat per group of members who perceived the same
// parts, every version carrying the same `utterance` id.
//
// THE PARTS, PRESENT OR ABSENT, NEVER BLANKED (R1, R3, R4):
//
//   - `speaker` — present only for a reader who sees the speaker;
//   - `to` — the member spoken to, present only for a reader who sees them or
//     is them; `prop`, the prop a line is said to, only with the words;
//   - `words`, `option` and `check` — present only for a reader the words
//     reached, the check only on a checked line (R3: the roll travels with
//     the words). Named `check`, not `roll`: on every other beat `roll` is a
//     world die's face, and this is the speaker's whole check — skill, DC,
//     total, beaten and the arithmetic;
//   - `hops` — rooms the voice carried, present only for a reader who does
//     not see the speaker;
//   - `place` and `loudness` — on every version.
//
// A reader who neither sees the speaker nor catches the words but noticed a
// voice gets place, loudness and hops alone. A reader who perceived nothing
// gets no beat. EXPORTED BECAUSE A DECODER READS IT.
const BeatSpeech = "speech"

// spokenRoll is a checked line's check, as the words carry it (R3): the
// skill, the DC, the total, whether it beat the DC and the arithmetic.
type spokenRoll struct {
	skill       string
	dc          int
	total       int
	beaten      bool
	calculation *RollCalculation
}

// utterance is one spoken thing ([Encounter.utter]).
type utterance struct {
	speaker  MemberID
	to       MemberID
	prop     PropID
	words    string
	option   string
	loudness Loudness
	roll     *spokenRoll
	at       uint64

	// scene is the scene a kept roll's line was spoken in ([ReservedScene]):
	// a kept settle is perceived as it was said, not as the world stands.
	// Nil for every live utterance.
	scene *ReservedScene

	// silent is a row that teaches a fact and says nothing: its reach is
	// worked out for the fact, and its voice lands on whoever heard it, but
	// no speech beat is written, there being no words.
	silent bool
}

// perceived is one member's facets of one utterance.
type facet struct {
	member  MemberID
	sees    bool
	words   bool
	noticed bool
	seesTo  bool
	hops    int
}

// spoken is what one utterance came to: who caught its words, and every
// member's facets in member order.
type spoken struct {
	catchers []MemberID
	all      []facet
}

// utter carries out one utterance (R8): who sees the speaker, who catches the
// words, who only notices a voice and who sees the person spoken to; one
// [BeatSpeech] per group of members who perceived the same parts; and the
// voice on the sounds channel of everyone who heard it, as every sound lands
// ([sound.Land]). It returns who caught the words, for the facts and
// recognitions the line teaches (R9).
//
// THE SPEAKER ALWAYS PERCEIVES THEIR OWN LINE; THE MEMBER SPOKEN TO ALWAYS
// CATCHES IT (R8, the lead's call: a line is only said within Speak reach). A
// member who is down perceives nothing.
func (e *Encounter) utter(u utterance) (spoken, error) {
	all, err := e.perceive(u)
	if err != nil {
		return spoken{}, fmt.Errorf("utterance: %w", err)
	}
	var out spoken
	var heard []hearing
	for _, p := range all {
		if p.words {
			out.catchers = append(out.catchers, p.member)
		}
		if p.member != u.speaker && (p.words || p.noticed) {
			heard = append(heard, hearing{member: p.member, hops: p.hops})
		}
	}
	out.all = all

	where, err := e.utterancePlace(u)
	if err != nil {
		return spoken{}, fmt.Errorf("utterance: %w", err)
	}
	if !u.silent {
		if err := e.appendSpeechBeats(u, where, all); err != nil {
			return spoken{}, err
		}
	}
	// THE VOICE (R9): every line said is a voice to whoever heard it — its
	// words or only that somebody spoke — kind and place only, never a
	// speaker, so every `heard: { kind: voice }` table keeps working.
	if len(heard) > 0 {
		hearers := make([]core.EntityID, 0, len(heard))
		for _, h := range heard {
			hearers = append(hearers, h.member)
		}
		if err := sound.Land(&sound.LandInput{
			Store:   memberReportStore{e},
			Sound:   sound.Sound{Kind: SoundVoice, Where: where, At: u.at},
			Hearers: hearers,
		}); err != nil {
			return spoken{}, fmt.Errorf("utterance: voice: %w", err)
		}
		e.landHeard(heard, SoundVoice, where, u.speaker, u.at)
	}
	return out, nil
}

// utterancePlace is where an utterance was said: the speaker's cell, or the
// cell a kept roll's line was spoken from.
func (e *Encounter) utterancePlace(u utterance) (spatial.Position, error) {
	if u.scene != nil {
		return u.scene.Where, nil
	}
	m, ok := e.members[u.speaker]
	if !ok {
		return spatial.Position{}, fmt.Errorf("speaker %q: %w", u.speaker, ErrNotMember)
	}
	return e.cellOf(m)
}

// perceive is every member's facets of one utterance, in member order, the
// members who perceived nothing included as all-false.
func (e *Encounter) perceive(u utterance) ([]facet, error) {
	if u.scene != nil {
		return e.perceivedAsSpoken(u), nil
	}
	where, seers, err := e.audienceOf(u.speaker)
	if err != nil {
		return nil, err
	}
	var seeTo []MemberID
	if u.to != "" {
		if _, w, err := e.audienceOf(u.to); err == nil {
			seeTo = w
		}
	}
	hops, err := e.voiceReach(where, u.loudness)
	if err != nil {
		return nil, err
	}
	sheets, err := e.sheetsNow()
	if err != nil {
		return nil, err
	}
	down, err := e.downNow()
	if err != nil {
		return nil, err
	}
	parts, err := e.field.partitionsNow()
	if err != nil {
		return nil, err
	}
	rooms, reaches := wordRooms(u.loudness)
	dc, noticeable := noticeDC(u.loudness)

	ids := e.rosterIDs()
	out := make([]facet, 0, len(ids))
	for _, id := range ids {
		p := facet{member: id}
		switch {
		case id == u.speaker:
			p.sees, p.words, p.seesTo = true, true, u.to != ""
		case down[id]:
			// A body perceives nothing.
		default:
			p.sees = slices.Contains(seers, id)
			p.seesTo = id == u.to || slices.Contains(seeTo, id)
			h, reached := -1, false
			if m, ok := e.members[id]; ok {
				if pos, err := e.cellOf(m); err == nil {
					if part, ok := parts.of[pos]; ok {
						h, reached = hops[part]
					}
				}
			}
			if reached {
				p.hops = h
			}
			p.words = id == u.to || (reaches && reached && h <= rooms)
			if !p.words && noticeable && reached {
				passive := sheets[id].PassivePerception
				p.noticed = passive > 0 && passive >= dc+dcPerHop*h
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// perceivedAsSpoken is a kept roll's line perceived as the scene it was
// spoken in recorded it ([ReservedScene.Heard]): who saw the speaker, who saw
// the person spoken to, and who caught the words or noticed the voice, from
// how many rooms. A member the scene does not name perceived nothing.
func (e *Encounter) perceivedAsSpoken(u utterance) []facet {
	scene := u.scene
	heard := map[MemberID]ReservedHearing{}
	for _, h := range scene.Heard {
		heard[h.Member] = h
	}
	ids := e.rosterIDs()
	out := make([]facet, 0, len(ids))
	for _, id := range ids {
		p := facet{member: id}
		if id == u.speaker {
			p.sees, p.words, p.seesTo = true, true, u.to != ""
		} else {
			p.sees = slices.Contains(scene.Witnesses, id)
			p.seesTo = id == u.to || slices.Contains(scene.SawTarget, id)
			if h, ok := heard[id]; ok {
				p.words, p.noticed, p.hops = h.Words, !h.Words, h.Hops
			}
			if id == u.to {
				p.words, p.noticed = true, false
			}
		}
		out = append(out, p)
	}
	return out
}

// voiceReach is every wall partition an utterance at this loudness made at
// where can reach anybody in, and how many rooms away, BOUNDED by the
// farthest any member could perceive it: the words' reach, or the rooms a
// voice can still be noticed across by the best passive Perception on the
// roster. The bound is what [Encounter.hopsFrom] does not have.
func (e *Encounter) voiceReach(where spatial.Position, l Loudness) (map[PartitionID]int, error) {
	parts, err := e.field.partitionsNow()
	if err != nil {
		return nil, err
	}
	origin, ok := parts.of[where]
	if !ok {
		return map[PartitionID]int{}, nil
	}
	bound, _ := wordRooms(l)
	if dc, ok := noticeDC(l); ok {
		sheets, err := e.sheetsNow()
		if err != nil {
			return nil, err
		}
		best := 0
		for _, s := range sheets {
			best = max(best, s.PassivePerception)
		}
		if best >= dc {
			bound = max(bound, (best-dc)/dcPerHop)
		}
	}
	return e.hopsWithin(origin, bound)
}

// hopsWithin is [Encounter.hopsFrom] stopped at bound: every partition no
// more than bound rooms from origin, a closed door on a crossing costing one
// room more. A partition farther is absent.
func (e *Encounter) hopsWithin(origin PartitionID, bound int) (map[PartitionID]int, error) {
	links, err := e.links()
	if err != nil {
		return nil, err
	}
	dist := map[PartitionID]int{origin: 0}
	done := map[PartitionID]bool{}
	for {
		var at PartitionID
		best := -1
		for p, d := range dist {
			if done[p] {
				continue
			}
			if best < 0 || d < best || (d == best && p < at) {
				at, best = p, d
			}
		}
		if best < 0 {
			return dist, nil
		}
		done[at] = true
		for _, l := range links[at] {
			cost := 1
			if l.door.blocks() {
				cost++
			}
			if best+cost > bound {
				continue
			}
			if d, seen := dist[l.to]; !seen || best+cost < d {
				dist[l.to] = best + cost
			}
		}
	}
}

// facetKey is the parts one version carries: the group a member's beat
// shares with every member who perceived the same.
type facetKey struct {
	sees, words, seesTo bool
	hops                int
}

// appendSpeechBeats writes one [BeatSpeech] per facet group, in the order
// each group is first met in member order, every one carrying the same
// utterance id, the next story sequence when the first is written — derived
// from the record, so a reload replays the same versions.
func (e *Encounter) appendSpeechBeats(u utterance, where spatial.Position, all []facet) error {
	next, err := e.story.NextSeq()
	if err != nil {
		return fmt.Errorf("speech: %w", err)
	}
	id := "u" + strconv.FormatUint(next, 10)
	var order []facetKey
	groups := map[facetKey][]MemberID{}
	for _, p := range all {
		if !p.sees && !p.words && !p.noticed {
			continue
		}
		k := facetKey{sees: p.sees, words: p.words, seesTo: p.seesTo && u.to != ""}
		if !p.sees {
			k.hops = p.hops
		}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], p.member)
	}
	loudness := u.loudness
	if loudness == "" {
		loudness = LoudnessSpeak
	}
	for _, k := range order {
		body := map[string]interface{}{
			"beat":      BeatSpeech,
			"utterance": id,
			"place":     where,
			"loudness":  string(loudness),
		}
		if k.sees {
			body["speaker"] = string(u.speaker)
		} else {
			body["hops"] = k.hops
		}
		if k.seesTo {
			body["to"] = string(u.to)
		}
		if k.words {
			body["words"] = u.words
			if u.option != "" {
				body["option"] = u.option
			}
			if u.prop != "" {
				body["prop"] = string(u.prop)
			}
			if u.roll != nil {
				roll := map[string]interface{}{
					"skill": u.roll.skill, "dc": u.roll.dc, "total": u.roll.total, "beaten": u.roll.beaten,
				}
				if u.roll.calculation != nil {
					roll["calculation"] = u.roll.calculation
				}
				body["check"] = roll
			}
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("speech: marshal: %w", err)
		}
		audience := groups[k]
		sort.Slice(audience, func(i, j int) bool { return audience[i] < audience[j] })
		if _, err := e.appendBeat(&record.AppendInput{
			At: u.at, Audience: audience,
			Tags:    map[string]string{"tag": BeatSpeech},
			Payload: payload,
		}); err != nil {
			return fmt.Errorf("speech: %w", err)
		}
	}
	return nil
}
