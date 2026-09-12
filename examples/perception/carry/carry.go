// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package carry is the distillation: what an observer takes out of a run that
// is over.
//
// A run's raw testimony belongs to the run. It is an append-only log of every
// pass, keyed by handles the projection minted inside that run, and it is a run
// artifact in the same way the dungeon's own state is — the party was ephemeral,
// the quest is finished, and nobody wants the log.
//
// What leaves is a conclusion, not the evidence.
//
// # You can only carry out what you can name
//
// A track handle means nothing outside the run that minted it — that opacity is
// deliberate, and it is what stops an observer merging two channels by string
// comparison. So a belief can only travel if it has been re-keyed onto a
// [belief.Name]: something a person could speak about later. "There are goblins
// in the eastern tunnels" travels. "Something is through that door" does not,
// and [Result.Unnamed] says so out loud rather than dropping it quietly.
//
// # It arrives as a ghost, because that is what it is
//
// Carried knowledge lands through [testimony.Store.Report], held and never
// current: something you know and are not currently perceiving. The store
// already had a word for that, and it is the same word — so a memory carried out
// of a dungeon needs no new state of its own, and it stays exactly as
// falsifiable as it was inside. Carrying a lie out carries the lie.
//
// # This is the one place folding is legitimate, and it still refuses a tie
//
// Everywhere else, two tracks that disagree are handed back side by side and
// somebody above decides. Here the observer IS that somebody: deciding what you
// took away from a run is an authored act, not a read. So when two tracks arrive
// under one name on one channel, they collapse, the most recently confirmed one
// wins, and [Result.Superseded] names what lost.
//
// When nothing is more recent — two tracks under one name, equally fresh — there
// is no answer to be had, and picking one by handle order would be this package
// inventing a conclusion on the observer's behalf. So it carries NEITHER and
// names both in [Result.Ambiguous]. The observer called two things by one word
// and the evidence cannot separate them; that is a fact about their beliefs, and
// the honest thing is to say so rather than to guess.
//
// # What is lost, deliberately
//
//   - The log. Only the latest entry travels. You carry what you concluded, not
//     every pass that led there.
//   - First sighting. [testimony.Entry.Observed] collapses onto Confirmed, since
//     when you first noticed something inside a finished run is a run detail.
//
// # What this requires, and does not yet have
//
// Stamps have to be on a clock that OUTLIVES the run. A carried belief keeps its
// Confirmed stamp, so staleness stays measurable after the run ends — but only
// if that number still means something. Re-stamping to the moment of leaving
// would make nine-day-old knowledge look fresh, which is the exact staleness lie
// the two stamps exist to kill. Carrying the run-local stamp out unchanged is
// the honest half of the fix; the other half is a clock above the run, and there
// isn't one here.
package carry

import (
	"hash/fnv"
	"slices"
	"strconv"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// Namer answers what an observer calls a track. [belief.Beliefs] satisfies it.
type Namer interface {
	NameOf(o testimony.Observer, track testimony.TrackID) (belief.Name, testimony.Stamp, bool)
}

// Recorder is told what the receiving knower calls each belief they were handed.
// [belief.Beliefs] satisfies it.
//
// Without this, a carried belief would arrive with its name baked into a handle
// nobody can read back, and the knower could never carry it anywhere else. With
// it, a player's store is shaped exactly like a run's — tracks plus names — and
// carrying IN is [Out] and [Land] again with the two stores swapped.
type Recorder interface {
	Identify(o testimony.Observer, track testimony.TrackID, as belief.Name, at testimony.Stamp) error
}

// Input is one observer leaving one run.
type Input struct {
	Observer testimony.Observer
	Tracks   []testimony.Track
	Names    Namer
}

// Portable is one belief that can survive the run.
type Portable struct {
	Name    belief.Name
	Channel testimony.Channel
	Entry   testimony.Entry
}

// Result is the whole distillation, including everything that could not travel.
type Result struct {
	// Carried is what leaves, ordered by name then channel.
	Carried []Portable
	// Unnamed is every track the observer had no word for. These do not travel,
	// and naming them is the only way they could.
	Unnamed []testimony.TrackID
	// Superseded is every track that lost a collapse: another track under the
	// same name on the same channel was confirmed more recently.
	Superseded []testimony.TrackID
	// Ambiguous is every track caught in a collapse that could not be resolved,
	// because nothing under that name on that channel was fresher than anything
	// else. Nothing is carried for it.
	Ambiguous []testimony.TrackID
}

type slot struct {
	name    belief.Name
	channel testimony.Channel
}

// bundle groups carried beliefs into one discrete report: a report carries a
// single stamp, so beliefs confirmed at different moments cannot share one.
type bundle struct {
	channel testimony.Channel
	at      testimony.Stamp
}

// Out distills what this observer takes with them.
func Out(in Input) Result {
	var out Result

	claimed := make(map[slot][]testimony.Track)
	order := make([]slot, 0, len(in.Tracks))

	for _, track := range in.Tracks {
		name, _, named := in.Names.NameOf(in.Observer, track.ID)
		if !named {
			out.Unnamed = append(out.Unnamed, track.ID)

			continue
		}

		key := slot{name: name, channel: track.Channel}
		if _, seen := claimed[key]; !seen {
			order = append(order, key)
		}

		claimed[key] = append(claimed[key], track)
	}

	for _, key := range order {
		carried, superseded, ambiguous := resolve(key, claimed[key])

		if carried != nil {
			out.Carried = append(out.Carried, *carried)
		}

		out.Superseded = append(out.Superseded, superseded...)
		out.Ambiguous = append(out.Ambiguous, ambiguous...)
	}

	slices.SortFunc(out.Carried, func(a, b Portable) int {
		if a.Name != b.Name {
			return strings.Compare(string(a.Name), string(b.Name))
		}

		return strings.Compare(string(a.Channel), string(b.Channel))
	})

	slices.Sort(out.Unnamed)
	slices.Sort(out.Superseded)
	slices.Sort(out.Ambiguous)

	return out
}

// resolve settles one name on one channel: the freshest travels, the rest are
// superseded, and a tie for freshest travels not at all.
func resolve(key slot, tracks []testimony.Track) (*Portable, []testimony.TrackID, []testimony.TrackID) {
	if len(tracks) == 1 {
		return &Portable{Name: key.name, Channel: key.channel, Entry: tracks[0].Latest()}, nil, nil
	}

	freshest := tracks[0].Latest().Confirmed
	tied := 1

	for _, track := range tracks[1:] {
		switch at := track.Latest().Confirmed; {
		case at.After(freshest):
			freshest, tied = at, 1
		case at == freshest:
			tied++
		}
	}

	if tied > 1 {
		ambiguous := make([]testimony.TrackID, 0, len(tracks))
		for _, track := range tracks {
			ambiguous = append(ambiguous, track.ID)
		}

		return nil, nil, ambiguous
	}

	var (
		winner     testimony.Track
		superseded []testimony.TrackID
	)

	for _, track := range tracks {
		if track.Latest().Confirmed == freshest {
			winner = track

			continue
		}

		superseded = append(superseded, track.ID)
	}

	return &Portable{Name: key.name, Channel: key.channel, Entry: winner.Latest()}, superseded, nil
}

// Land lodges carried beliefs with whoever is receiving them.
//
// Pointed at a player it is carrying OUT: the party was ephemeral, the player is
// not. Pointed at a run's store with an in-run observer it is carrying IN: a
// party walks into the tunnels already believing something. Same verb, same
// arrival state, opposite direction — because both are "lodge a recollection
// with somebody", and nothing about that changes with which way you are walking.
//
// Each belief keeps its Confirmed stamp, so a carried memory is as old as it
// really is. It arrives held and never current, and its name is recorded so the
// receiver can carry it on again.
func Land(s *testimony.Store, names Recorder, knower testimony.Observer, carried []Portable) error {
	grouped := make(map[bundle][]testimony.Report)

	for _, p := range carried {
		key := bundle{channel: p.Channel, at: p.Entry.Confirmed}

		grouped[key] = append(grouped[key], testimony.Report{
			Track:     Handle(p.Name, p.Channel),
			Payload:   p.Entry.Payload,
			ChangeKey: p.Entry.ChangeKey,
			Locus:     p.Entry.Locus,
		})
	}

	keys := make([]bundle, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}

	slices.SortFunc(keys, func(a, b bundle) int {
		if a.channel != b.channel {
			return strings.Compare(string(a.channel), string(b.channel))
		}

		return a.at.Compare(b.at)
	})

	for _, key := range keys {
		if _, err := s.Report(testimony.Recollection{
			Observer: knower,
			Channel:  key.channel,
			Reports:  grouped[key],
			At:       key.at,
		}); err != nil {
			return err
		}
	}

	for _, p := range carried {
		if err := names.Identify(knower, Handle(p.Name, p.Channel), p.Name, p.Entry.Confirmed); err != nil {
			return err
		}
	}

	return nil
}

// Handle mints the portable handle for a carried belief.
//
// It is derived from the name, which is the whole point: unlike a run-local
// track handle, the same name in a later run resolves to the same held belief,
// so what a player learned underground is there to be confirmed, contradicted,
// or left to go stale.
func Handle(name belief.Name, channel testimony.Channel) testimony.TrackID {
	sum := fnv.New64a()
	_, _ = sum.Write([]byte(channel))
	_, _ = sum.Write([]byte{0})
	_, _ = sum.Write([]byte(name))

	return testimony.TrackID("k" + strconv.FormatUint(sum.Sum64(), 36))
}
