// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/examples/behavior"
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior/minds"
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior/stage"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// This file is the vocabulary the use cases are written in. Nothing here
// asserts anything on its own; it exists so each proof in usecases_test.go
// reads as the story it is measuring.

const (
	sight                     = testimony.Sight
	hearing testimony.Channel = "hearing"

	room     = "room"
	corridor = "corridor"
	hall     = "hall"

	zombie  testimony.Observer = "zombie"
	captain testimony.Observer = "captain"
	archer  testimony.Observer = "archer"
	goblin  testimony.Observer = "goblin"

	// Ledger handles. They never leave the projection; the test is the game
	// master and may mint track handles from them to assert with.
	knight = "knight"
	mage   = "mage"
	cleric = "cleric"
	banner = "banner"

	armoured = "armoured"
	hooded   = "hooded"

	chanting = true
	silent   = false
)

// scene is one truth surface, one game, and a clock. Every verb on it is
// something the game master does; every question is asked of an actor.
type scene struct {
	t     *testing.T
	g     *behavior.Game
	truth projection.Input
	tick  uint64
}

func newScene(t *testing.T) *scene {
	t.Helper()

	return &scene{t: t, g: behavior.New()}
}

// doors declares the dungeon: each pair is one door.
func (s *scene) doors(pairs ...[2]string) {
	for _, p := range pairs {
		s.g.Connect(&behavior.ConnectInput{A: p[0], B: p[1]})
	}
}

func door(a, b string) [2]string { return [2]string{a, b} }

// mind gives an actor a mind and puts it somewhere on the truth surface,
// where nothing perceives it — the monsters in these stories do not perceive
// each other, and a presence that says nothing to any channel is how that is
// spelled.
func (s *scene) mind(who testimony.Observer, m behavior.Mind, where string) {
	s.g.Mind(&behavior.MindInput{Actor: who, Mind: m})
	s.truth.Presences = append(s.truth.Presences, projection.Presence{Source: string(who), Where: where})
}

// bow arms an actor to strike one region away.
func (s *scene) bow(who testimony.Observer) {
	s.g.Sheet(&behavior.SheetInput{Actor: who, Sheet: behavior.Sheet{Reach: 1}})
}

// senses gives each actor sight and hearing over the given places.
func (s *scene) senses(reach []string, who ...testimony.Observer) {
	for _, o := range who {
		s.truth.Senses = append(s.truth.Senses,
			projection.Sense{Observer: o, Channel: sight, Reach: reach},
			projection.Sense{Observer: o, Channel: hearing, Reach: reach},
		)
	}
}

func says(kind content.Kind, of, note, where string) projection.Says {
	p := content.Percept{Kind: kind, Of: of, Note: note}

	return projection.Says{Payload: content.Encode(p), ChangeKey: content.Key(p), Locus: testimony.Locus{Where: where}}
}

// figure puts somebody on the truth surface: seen, and heard if chanting.
func (s *scene) figure(source, where, look string, chants bool) {
	p := projection.Presence{
		Source: source,
		Where:  where,
		Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "human", look, where)},
	}

	if chants {
		p.Says[hearing] = says(content.Noise, "", minds.Chanting, where)
	}

	s.place(p)
}

// post puts a place a guard was told to stand on the truth surface,
// perceived like anything else.
func (s *scene) post(where string) {
	s.place(projection.Presence{
		Source: banner,
		Where:  where,
		Says:   map[testimony.Channel]projection.Says{sight: says(minds.PostKind, "banner", "", where)},
	})
}

// place adds or replaces a presence by source.
func (s *scene) place(p projection.Presence) {
	for i, q := range s.truth.Presences {
		if q.Source == p.Source {
			s.truth.Presences[i] = p

			return
		}
	}

	s.truth.Presences = append(s.truth.Presences, p)
}

// moves relocates somebody, actor or figure, keeping what they say.
func (s *scene) moves(source, where string) {
	for i, p := range s.truth.Presences {
		if p.Source != source {
			continue
		}

		p.Where = where
		for ch, said := range p.Says {
			said.Locus = testimony.Locus{Where: where}
			p.Says[ch] = said
		}

		s.truth.Presences[i] = p
	}
}

// leaves removes somebody from the truth surface entirely.
func (s *scene) leaves(source string) {
	kept := s.truth.Presences[:0]
	for _, p := range s.truth.Presences {
		if p.Source != source {
			kept = append(kept, p)
		}
	}

	s.truth.Presences = kept
}

// look advances the clock one tick and lets everyone perceive.
func (s *scene) look() {
	s.t.Helper()

	s.tick++
	s.truth.At = testimony.Stamp{Tick: s.tick}
	require.NoError(s.t, s.g.Tick(s.truth))
}

// looks advances the clock n ticks.
func (s *scene) looks(n uint64) {
	for range n {
		s.look()
	}
}

// happens lands a deed on everyone whose senses reached the place.
func (s *scene) happens(actor, verb, target, where string) {
	s.t.Helper()

	s.tick++
	require.NoError(s.t, stage.Land(&stage.LandInput{
		Game:  s.g,
		Truth: s.truth,
		Deed:  stage.Deed{Actor: actor, Target: target, Verb: verb, Where: where},
		At:    testimony.Stamp{Tick: s.tick},
	}))
}

// frightens puts the frightened condition on an actor.
func (s *scene) frightens(who testimony.Observer, of string) {
	s.g.Frighten(&behavior.FrightenInput{Actor: who, Source: of})
}

// turn asks an actor what it means to do right now.
func (s *scene) turn(who testimony.Observer) *turn {
	s.t.Helper()

	out, err := s.g.Turn(&behavior.TurnInput{Actor: who, At: testimony.Stamp{Tick: s.tick}})
	require.NoError(s.t, err)

	return &turn{s: s, who: who, out: out}
}

// turn is one actor's decision, with the questions a proof asks of it.
type turn struct {
	s   *scene
	who testimony.Observer
	out *behavior.TurnOutput
}

// aims is the ledger handle the intent's target really resolves to.
func (t *turn) aims() string {
	t.s.t.Helper()

	aimed, err := stage.Aim(&stage.AimInput{Truth: t.s.truth, Situation: t.out.Situation, Intent: t.out.Intent})
	require.NoError(t.s.t, err)

	return aimed.Source
}

// steps is where the intent would walk the actor, and whether it can.
func (t *turn) steps() (string, bool) {
	t.s.t.Helper()

	stepped, err := stage.Step(&stage.StepInput{Game: t.s.g, Situation: t.out.Situation, Intent: t.out.Intent})
	require.NoError(t.s.t, err)

	return stepped.To, stepped.Moved
}

// believes is where the actor thinks its target is.
func (t *turn) believes() string {
	t.s.t.Helper()

	recalled, err := stage.Recall(&stage.RecallInput{Situation: t.out.Situation, Name: t.out.Intent.Target})
	require.NoError(t.s.t, err)

	return recalled.Where
}

// attacks asserts the actor swings, and at whom the swing really lands.
func (t *turn) attacks(source, why string) {
	t.s.t.Helper()

	require.Equal(t.s.t, behavior.Attack, t.out.Intent.Verb, why)
	assert.Equal(t.s.t, source, t.aims(), why)
}

// walksTo asserts the actor steps toward something and arrives at a region.
func (t *turn) walksTo(where, why string) {
	t.s.t.Helper()

	require.Equal(t.s.t, behavior.Toward, t.out.Intent.Verb, why)

	to, moved := t.steps()
	require.True(t.s.t, moved, why)
	assert.Equal(t.s.t, where, to, why)
}

// backsOff asserts the actor steps away from a region and lands somewhere
// that is not it.
func (t *turn) backsOff(from, why string) {
	t.s.t.Helper()

	require.Equal(t.s.t, behavior.Away, t.out.Intent.Verb, why)
	assert.Equal(t.s.t, from, t.believes(), why)

	to, moved := t.steps()
	require.True(t.s.t, moved, why)
	assert.NotEqual(t.s.t, from, to, why)
}

// cornered asserts the actor wants to flee and has nowhere to go.
func (t *turn) cornered(why string) {
	t.s.t.Helper()

	require.Equal(t.s.t, behavior.Away, t.out.Intent.Verb, why)

	_, moved := t.steps()
	assert.False(t.s.t, moved, why)
}

// stays asserts the actor does nothing.
func (t *turn) stays(why string) {
	t.s.t.Helper()

	assert.Equal(t.s.t, behavior.Pass, t.out.Intent.Verb, why)
}

// moved is a convenience for tests that walk an actor where it decided to go.
func (t *turn) moved() {
	t.s.t.Helper()

	to, ok := t.steps()
	require.True(t.s.t, ok)
	t.s.moves(string(t.who), to)
}

// contactHolding is the actor's contact that holds a track.
func (t *turn) contactHolding(track testimony.TrackID) behavior.Contact {
	t.s.t.Helper()

	for _, c := range t.out.Situation.Contacts {
		for _, v := range c.Tracks {
			if v.ID == track {
				return c
			}
		}
	}

	require.Failf(t.s.t, "not held", "%s holds no track %s", t.who, track)

	return behavior.Contact{}
}

// holdsNothingOf asserts the actor has no track at all under a handle.
func (t *turn) holdsNothingOf(track testimony.TrackID, why string) {
	t.s.t.Helper()

	for _, c := range t.out.Situation.Contacts {
		for _, v := range c.Tracks {
			assert.NotEqual(t.s.t, track, v.ID, why)
		}
	}
}

func holds(c behavior.Contact, track testimony.TrackID) bool {
	for _, v := range c.Tracks {
		if v.ID == track {
			return true
		}
	}

	return false
}

func sightOf(source string) testimony.TrackID   { return projection.Handle(sight, source) }
func hearingOf(source string) testimony.TrackID { return projection.Handle(hearing, source) }
