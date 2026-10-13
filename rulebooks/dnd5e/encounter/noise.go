// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/sound"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// noise.go is HEARING (rpg-project#569, rulings R4 and R5): what a deed sounds
// like, how far that carries through the castle's doors, and who is listening
// hard enough to catch it.
//
// # A sound names no maker
//
// A heard report is a KIND and a PLACE, stamped with when (R4). It lands on
// the hearers' sounds channel through [sound.Land], which carries no actor by
// construction, and the [BeatHeard] a player receives carries none either. A
// guard who hears a clang walks to the clang ([SelectorSound]) and finds out
// who by looking.
//
// # Nobody rolls
//
// Hearing is passive (R5). A sound's DC is the rulebook's content for its kind
// ([soundDCs]); every region hop between the sound and the listener adds
// [dcPerHop], and a closed door on the crossing counts as one hop more. The
// listener's passive Perception ([SheetFacts.PassivePerception]) answers it
// and nothing else does: no die, no check, no beat for a near miss.
//
// # Hops are regions, door to door
//
// The graph is the field's regions joined by its doors' edges, derived once
// per encounter from the construction-time doors ([Encounter.regionLinks]).
// Only a door's STATE is read live, because it is the one thing about the
// graph a verb changes.

// The sound kinds this rulebook's composition makes, sealed (R4: "one per use
// case"). A `heard: { kind }` naming any other word is refused at every door
// a table comes in through, because no deed here would ever make it.
const (
	// SoundBoom is a damaging spell delivered.
	SoundBoom = "boom"

	// SoundCrash is a door forced open by strength.
	SoundCrash = "crash"

	// SoundClang is an attack made, struck or missed: steel on steel.
	SoundClang = "clang"

	// SoundVoice is a creature speaking a line on any answer row.
	SoundVoice = "voice"
)

// SoundKinds is every kind, in the order a refusal lists them.
var SoundKinds = []string{SoundBoom, SoundCrash, SoundClang, SoundVoice}

// soundDCs is each kind's loudness as a DC in the room it was made in.
// CONTENT, tuned on the walk (the plan's open items, confirmed by the
// operator 2026-10-13): lower is louder.
var soundDCs = map[string]int{
	SoundBoom:  5,
	SoundCrash: 8,
	SoundClang: 10,
	SoundVoice: 10,
}

// dcPerHop is what each region crossed adds to a sound's DC. A closed door on
// a crossing is one hop more, so it adds this again.
const dcPerHop = 5

// BeatHeard is the "beat" value of the story beat a PLAYER receives when a
// sound reaches them: `{listener, kind, position, hops}`, audience that one
// listener (H5).
//
// NEVER THE MAKER, for R4: the beat says what kind of sound and where, which
// is all the listener's ears told them. A client turns position and hops into
// "metal clanging, two rooms north"; this module does not compose that line.
//
// EXPORTED BECAUSE A DECODER READS IT, [BeatAnswered]'s reason.
const BeatHeard = "heard"

// The causes [soundOf] maps. Each is the deed or act that makes the sound,
// named once so the four sites cannot spell them two ways.
const (
	noiseAttack       = DeedAttack
	noiseForcedDoor   = "forced-door"
	noiseDamagingCast = "damaging-cast"
	noiseSay          = "say"
)

// soundOf is what one act sounds like: its kind and its DC at the source.
// ok is false for an act that makes no sound this build hears.
//
// THE CONTENT MAP, in one place: an attack made clangs, a door forced crashes,
// a damaging cast booms, and every line said is a voice.
func soundOf(cause string) (kind string, dc int, ok bool) {
	switch cause {
	case noiseAttack:
		kind = SoundClang
	case noiseForcedDoor:
		kind = SoundCrash
	case noiseDamagingCast:
		kind = SoundBoom
	case noiseSay:
		kind = SoundVoice
	default:
		return "", 0, false
	}

	return kind, soundDCs[kind], true
}

// knownSoundKind reports whether a kind is one of [SoundKinds].
func knownSoundKind(kind string) bool {
	_, ok := soundDCs[kind]

	return ok
}

// forcedBy reports whether a beaten lock's applied route FORCED the door
// rather than picking it: the Strength route, spelled the one way every lock
// this repository's content authors spells it — `str`, the lowercase ability
// ref ([CheckApproach.Ability]). No dialect produces `strength`, and no lock
// is authored with `athletics`, so neither is read.
//
// [deedForSkill]'s rule for [deedForSkill]'s reason: the approach's ability
// word is carried, never interpreted, except here, to choose whether the
// door made a sound — the third of the three places that compare it.
func forcedBy(ability string) bool {
	return ability == "str"
}

// regionLink is one way out of a region: the region on the far side and the
// door the crossing goes through.
type regionLink struct {
	to   RegionID
	door *doorRecord
}

// links is the region graph: every region's ways out, derived on first use
// from the doors' construction-time edges and cached for the encounter's life.
//
// ONCE, because nothing it is made of changes: a door's edges and the field's
// region owners are construction truth. The door POINTER is kept rather than
// its state, so [Encounter.hopsFrom] reads whether it is closed at the moment
// a sound is made.
//
// A DOOR WHOSE TWO CELLS ARE ONE REGION'S joins nothing: it is a door inside a
// room, and crossing it changes no region.
func (e *Encounter) links() map[RegionID][]regionLink {
	if e.regionLinks != nil {
		return e.regionLinks
	}
	out := map[RegionID][]regionLink{}
	for _, d := range e.doors {
		for _, edge := range d.edges {
			from, okFrom := e.field.regionOf(edge.From)
			to, okTo := e.field.regionOf(edge.To)
			if !okFrom || !okTo || from == to {
				continue
			}
			out[from] = append(out[from], regionLink{to: to, door: d})
			out[to] = append(out[to], regionLink{to: from, door: d})
		}
	}
	e.regionLinks = out

	return out
}

// hopsFrom is every region a sound made in origin reaches, and how many hops
// away it is: one per region crossed, one more for each closed door crossed.
// The cheapest way through wins, so an open door beside a closed one is the
// way the sound goes. origin itself is zero hops.
//
// A region no door leads to is absent: no sound reaches it.
func (e *Encounter) hopsFrom(origin RegionID) map[RegionID]int {
	links := e.links()
	dist := map[RegionID]int{origin: 0}
	done := map[RegionID]bool{}
	for {
		var at RegionID
		best := -1
		for region, d := range dist {
			if done[region] {
				continue
			}
			if best < 0 || d < best || (d == best && region < at) {
				at, best = region, d
			}
		}
		if best < 0 {
			return dist
		}
		done[at] = true
		for _, l := range links[at] {
			cost := 1
			if l.door.blocks() {
				cost++
			}
			if d, seen := dist[l.to]; !seen || best+cost < d {
				dist[l.to] = best + cost
			}
		}
	}
}

// hearing is one member who heard a sound, and from how many hops away.
type hearing struct {
	member MemberID
	hops   int
}

// hearersOf is every standing member, other than the maker, whose passive
// Perception beats a sound of this DC made at this cell, sorted by member
// (C8): a member hears when PassivePerception >= dc + dcPerHop × hops.
//
// THE MAKER IS NOT A HEARER of its own sound. It knows what it did — that is
// its deed, held on the deeds channel — and a guard who swung would otherwise
// turn toward its own clang.
//
// A BODY HEARS NOTHING: a down member is skipped, the same down set every
// other sense here reads. A cell no region owns is heard by nobody, which no
// standing member can be on.
//
// WHO IS DOWN IS ASKED ONLY WHEN SOMEBODY'S EARS COULD CATCH IT. Participation
// is a capability with a price per question, and most sounds in most scenes
// reach nobody — a roster whose sheets answer no passive Perception hears
// nothing at all — so the question is put only once a listener has qualified
// on the arithmetic.
func (e *Encounter) hearersOf(cell spatial.Position, dc int, maker MemberID) ([]hearing, error) {
	origin, ok := e.field.regionOf(cell)
	if !ok {
		return nil, nil
	}
	sheets, err := e.sheetsNow()
	if err != nil {
		return nil, fmt.Errorf("hearers: %w", err)
	}
	hops := e.hopsFrom(origin)

	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var near []hearing
	for _, id := range ids {
		if id == maker {
			continue
		}
		pos, err := e.cellOf(e.members[id])
		if err != nil {
			continue
		}
		region, ok := e.field.regionOf(pos)
		if !ok {
			continue
		}
		h, reached := hops[region]
		if !reached {
			continue
		}
		if sheets[id].PassivePerception >= dc+dcPerHop*h {
			near = append(near, hearing{member: id, hops: h})
		}
	}
	if len(near) == 0 {
		return nil, nil
	}

	down, err := e.downNow()
	if err != nil {
		return nil, fmt.Errorf("hearers: %w", err)
	}
	out := near[:0]
	for _, h := range near {
		if !down[h.member] {
			out = append(out, h)
		}
	}

	return out, nil
}

// makeSound lands the sound one act makes, at the maker's cell, on everyone
// who hears it, and tells each listening player with a [BeatHeard].
//
// AFTER THE DEED, at every site: the witnesses' testimony of what was done
// lands first, then the noise of it reaches the ones who did not see. A
// witness who also hears holds both, which is the truth.
//
// STAMPED WITH THE CAUSE'S CLOCK READING, the [Encounter.landDeedAt] rule: a
// verb reads the clock once and everything it produces carries that reading.
func (e *Encounter) makeSound(cause string, maker MemberID, at uint64) error {
	kind, dc, ok := soundOf(cause)
	if !ok {
		return nil
	}
	record, known := e.members[maker]
	if !known {
		return fmt.Errorf("sound: maker %q: %w", maker, ErrNotMember)
	}
	where, err := e.cellOf(record)
	if err != nil {
		return fmt.Errorf("sound: maker %q: %w", maker, err)
	}
	heard, err := e.hearersOf(where, dc, maker)
	if err != nil {
		return fmt.Errorf("sound %s: %w", kind, err)
	}
	if len(heard) == 0 {
		return nil
	}

	hearers := make([]core.EntityID, 0, len(heard))
	for _, h := range heard {
		hearers = append(hearers, h.member)
	}
	if err := sound.Land(&sound.LandInput{
		Store:   memberReportStore{e},
		Sound:   sound.Sound{Kind: kind, Where: where, At: at},
		Hearers: hearers,
	}); err != nil {
		return fmt.Errorf("sound %s: %w", kind, err)
	}

	for _, h := range heard {
		if e.members[h.member].Kind != KindPlayer {
			continue
		}
		if err := e.appendHeardBeat(h, kind, where, at); err != nil {
			return err
		}
	}

	return nil
}

// appendHeardBeat tells one player what they heard: the kind, where it was
// made, and how many hops away — audience that player alone, the per-observer
// shape [BeatSighted] keeps.
func (e *Encounter) appendHeardBeat(h hearing, kind string, where spatial.Position, at uint64) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat":     BeatHeard,
		"listener": string(h.member),
		"kind":     kind,
		"position": where,
		"hops":     h.hops,
	})
	if err != nil {
		return fmt.Errorf("heard beat: %w", err)
	}
	if _, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: []MemberID{h.member},
		Tags:     map[string]string{"tag": BeatHeard},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("heard beat: %w", err)
	}

	return nil
}
