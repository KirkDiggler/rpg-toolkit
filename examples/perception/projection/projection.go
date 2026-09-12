// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package projection turns what is true into what each observer's senses
// deliver. It is the only place in this spike where a lie is authored.
//
// # It is a pure function
//
// [Project] takes the truth surface as VALUES and returns percepts as values.
// It imports no world, holds no state, reads no clock and writes nothing. That
// is what lets the same projection serve a tactical canvas and a living region
// without knowing which one produced its input: the composition reads whichever
// truth surface is live and hands the facts over.
//
// # It has no query surface, and must never grow one
//
// This is the only code with access to both the truth and the observers, so it
// is the one place where somebody could ask "does this player know about the
// goblin" and get an answer read off reality. There is deliberately no function
// here that answers a question about an observer. It emits percepts and that is
// all it does. A read door added here would quietly undo the whole design.
//
// # A thing says different things to different channels
//
// [Presence.Says] is keyed by channel, so one creature presents as a placed
// shape to sight and an unplaced clatter to hearing — two fidelities of the
// same thing, which is what makes them worth holding separately. A channel the
// presence says nothing to cannot perceive it at all, so a silent creature and
// an odourless one cost no extra machinery.
//
// # Where the lie goes, and where it does not
//
// A [Forgery] forges a channel's delivery. It never touches [Input.Presences],
// so the truth surface never contains the illusion — the wizard casting Silent
// Image is a true fact about the world, and the dragon is not a fact at all.
//
// Because a forgery is also keyed by channel, "a visual illusion has no sound"
// is the ordinary case: sight is forged, hearing still reports what is really
// there, and the observer ends up holding two tracks that disagree with nothing
// in the store able to tell them so.
//
// [Forgery.Hides] is the same primitive pointed the other way — suppress a
// presence on some channels and report nothing, which is invisibility.
package projection

import (
	"hash/fnv"
	"slices"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// Says is what one channel would report about one thing: the opaque payload and
// the emitter's declaration of what counts as a change.
//
// The change key is declared here, by the only party that knows which fields
// are noise. The store never compares payloads, because byte equality on a
// payload carrying a timestamp is never true of itself.
type Says struct {
	Payload   []byte
	ChangeKey string
	// Locus is what THIS channel can tell you about where the thing is, which
	// is not the same as where it actually is. Sight places a shape in a room;
	// hearing through a shut door places nothing. An empty Where is a channel
	// admitting it could not localise what it perceived — known to be there,
	// not known where — and it is the ordinary case, not a failure.
	Locus testimony.Locus
}

// Presence is one thing the truth surface says is somewhere. Footprints are a
// presence; so is the creature that left them, somewhere else, later.
//
// Where is where it really is, and it decides only whether a channel's reach
// covers it. What a channel then REPORTS about where it is comes from that
// channel's own [Says.Locus], because being able to perceive a thing and being
// able to place it are different capabilities.
//
// Source is the ledger's own handle for the thing. It never leaves this
// package: everything downstream sees only the opaque track handle minted from
// it, which is what stops an observer inverting a handle back to an identity
// and merging two channels for free.
type Presence struct {
	Source string
	Where  string
	Says   map[testimony.Channel]Says
}

// Sense is one observer's reach on one channel for one pass. Reach is the whole
// of the physics in this spike — range, blocking, lighting and darkvision all
// collapse into "which places does this channel reach right now", which is
// exactly the shape a capability seam wants: a better answer, nothing else
// moving.
type Sense struct {
	Observer testimony.Observer
	Channel  testimony.Channel
	Reach    []string
}

// Forgery is a lie told to a channel.
type Forgery struct {
	// Where is the place the forgery occupies.
	Where string
	// Source is the forgery's own continuity handle, so the same illusion
	// perceived over several passes is one track rather than a new one each
	// time.
	Source string
	// Observers restricts who is deceived. Empty means everyone whose reach
	// covers Where — an illusion does not choose its audience.
	Observers []testimony.Observer
	// Says is what each deceived channel reports. A channel absent here is not
	// deceived, which is how a silent illusion leaves hearing telling the truth.
	Says map[testimony.Channel]Says
	// Hides are the channels on which Replaces is suppressed.
	Hides []testimony.Channel
	// Replaces names a Presence.Source this forgery stands in front of. Empty
	// adds a percept without hiding anything.
	Replaces string
}

// Input is one pass: what is true, who can sense what, what is being faked.
type Input struct {
	Presences []Presence
	Senses    []Sense
	Forgeries []Forgery
	At        testimony.Stamp
}

// Truth is what is really there, forgeries excluded — the game master's view.
// It exists so a test can assert the thing that matters most: after an illusion
// has deceived everyone who could see it, the truth surface still has no dragon
// in it.
func (in Input) Truth() []Presence {
	return slices.Clone(in.Presences)
}

// Project emits one complete percept per sense.
//
// A sense whose reach holds nothing still emits a percept, with no reports.
// That is not a no-op: looking and finding nothing is how a track stops being
// current, and skipping the empty percept would leave every ghost looking
// freshly perceived forever.
func Project(in Input) []testimony.Percept {
	out := make([]testimony.Percept, 0, len(in.Senses))

	for _, sense := range in.Senses {
		reach := make(map[string]struct{}, len(sense.Reach))
		for _, where := range sense.Reach {
			reach[where] = struct{}{}
		}

		hidden, forged := in.deceive(sense, reach)

		reports := make([]testimony.Report, 0, len(in.Presences)+len(forged))

		for _, p := range in.Presences {
			if _, within := reach[p.Where]; !within {
				continue
			}

			if _, gone := hidden[p.Source]; gone {
				continue
			}

			says, perceptible := p.Says[sense.Channel]
			if !perceptible {
				continue
			}

			reports = append(reports, testimony.Report{
				Track:     Handle(sense.Channel, p.Source),
				Payload:   says.Payload,
				ChangeKey: says.ChangeKey,
				Locus:     says.Locus,
			})
		}

		reports = append(reports, forged...)

		out = append(out, testimony.Percept{
			Observer: sense.Observer,
			Channel:  sense.Channel,
			Reports:  reports,
			At:       in.At,
		})
	}

	return out
}

// deceive collects what this sense has hidden from it and what it is fed
// instead.
func (in Input) deceive(sense Sense, reach map[string]struct{}) (map[string]struct{}, []testimony.Report) {
	hidden := make(map[string]struct{})

	forged := make([]testimony.Report, 0, len(in.Forgeries))

	for _, f := range in.Forgeries {
		if _, within := reach[f.Where]; !within {
			continue
		}

		if len(f.Observers) > 0 && !slices.Contains(f.Observers, sense.Observer) {
			continue
		}

		if f.Replaces != "" && slices.Contains(f.Hides, sense.Channel) {
			hidden[f.Replaces] = struct{}{}
		}

		says, deceived := f.Says[sense.Channel]
		if !deceived {
			continue
		}

		forged = append(forged, testimony.Report{
			Track:     Handle(sense.Channel, f.Source),
			Payload:   says.Payload,
			ChangeKey: says.ChangeKey,
			Locus:     says.Locus,
		})
	}

	return hidden, forged
}

// Bind answers what is REALLY behind a track handle, for the composition that
// has to resolve somebody's intent against the world.
//
// It is a pure function of the truth surface and it never consults an observer,
// so it is not a question about what anybody knows — it is the inverse of
// minting, and only the composition ever asks it.
//
// Forgeries are absent from the result, and that absence is the whole mechanism:
// a track an illusion wrote binds to nothing, so acting on it reaches nothing.
// Nobody had to write a rule for that.
func Bind(in Input) map[testimony.TrackID]string {
	out := make(map[testimony.TrackID]string)

	for _, p := range in.Presences {
		for channel := range p.Says {
			out[Handle(channel, p.Source)] = p.Source
		}
	}

	return out
}

// Handle mints a channel's opaque continuity handle for a source.
//
// It is stable across passes, so the same noise stays one track, and it carries
// no trace of the source, so nothing downstream can recover an identity from
// it. The two properties together are the reason a sight track and a hearing
// track of the same creature cannot be merged by string comparison: continuity
// is the channel's judgment, and identity is the observer's claim.
func Handle(channel testimony.Channel, source string) testimony.TrackID {
	sum := fnv.New64a()
	_, _ = sum.Write([]byte(channel))
	_, _ = sum.Write([]byte{0})
	_, _ = sum.Write([]byte(source))

	return testimony.TrackID("t" + strconv.FormatUint(sum.Sum64(), 36))
}
