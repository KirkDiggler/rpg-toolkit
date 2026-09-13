// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/stage"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// This file is the vocabulary the use cases are written in. Nothing here
// asserts anything on its own; it exists so each proof in usecases_test.go
// reads as the story it is measuring. The scene is the game master: it owns
// the truth, the senses, and the clock, and hands behaviour values.

const (
	sight                      = perception.Sight
	hearing perception.Channel = "hearing"

	room     = "room"
	corridor = "corridor"
	hall     = "hall"

	zombie  core.EntityID = "zombie"
	captain core.EntityID = "captain"
	archer  core.EntityID = "archer"
	goblin  core.EntityID = "goblin"

	knight core.EntityID = "knight"
	mage   core.EntityID = "mage"
	cleric core.EntityID = "cleric"
	banner core.EntityID = "banner"

	armoured = "armoured"
	hooded   = "hooded"

	chants = true
	silent = false
)

// figure is one thing on the truth surface.
type figure struct {
	kind   kind
	of     string
	look   string
	where  string
	chants bool
}

// scene is one truth surface, one game, and a clock. Every verb on it is
// something the game master does; every question is asked of an actor.
type scene struct {
	t       *testing.T
	g       *behavior.Game
	rooms   *rooms
	figures map[core.EntityID]figure
	senses  map[core.EntityID][]string
	actors  []core.EntityID
	tick    uint64
}

func newScene(t *testing.T) *scene {
	t.Helper()

	r := &rooms{doors: make(map[string][]string)}

	g, err := behavior.New(&behavior.NewInput{Reader: reader{}, Space: r})
	require.NoError(t, err)

	return &scene{
		t:       t,
		g:       g,
		rooms:   r,
		figures: make(map[core.EntityID]figure),
		senses:  make(map[core.EntityID][]string),
	}
}

// rooms is the proofs' Space: places are rooms, joined by doors, and a step
// is one door. Distance is doors counted along the shortest way. This is
// the geometry the spike was built on, and it lives here because a real
// board has its own.
type rooms struct {
	doors map[string][]string
}

func (r *rooms) connect(a, b string) {
	r.doors[a] = append(r.doors[a], b)
	r.doors[b] = append(r.doors[b], a)
}

// distances is how many doors each room is from one room.
func (r *rooms) distances(from string) map[string]int {
	dist := map[string]int{from: 0}
	queue := []string{from}

	for len(queue) > 0 {
		here := queue[0]
		queue = queue[1:]

		for _, next := range r.doors[here] {
			if _, seen := dist[next]; seen {
				continue
			}

			dist[next] = dist[here] + 1
			queue = append(queue, next)
		}
	}

	return dist
}

// Distance is doors along the shortest way; unknown when there is none.
func (r *rooms) Distance(in *behavior.DistanceInput) (*behavior.DistanceOutput, error) {
	d, known := r.distances(in.From)[in.To]

	return &behavior.DistanceOutput{Steps: d, Known: known}, nil
}

// Toward is the first door on the way.
func (r *rooms) Toward(in *behavior.TowardInput) (*behavior.TowardOutput, error) {
	dist := r.distances(in.To)

	best, found := "", false

	for _, next := range r.doors[in.From] {
		if d, reachable := dist[next]; reachable && (!found || d < dist[best]) {
			best, found = next, true
		}
	}

	if !found || dist[best] >= dist[in.From] {
		return &behavior.TowardOutput{}, nil
	}

	return &behavior.TowardOutput{Next: best, Found: true}, nil
}

// Away is the door that puts the most rooms between them, and nothing when
// every door leads closer or nowhere.
func (r *rooms) Away(in *behavior.AwayInput) (*behavior.AwayOutput, error) {
	dist := r.distances(in.AwayFrom)

	here, placed := dist[in.From]
	if !placed {
		return &behavior.AwayOutput{}, nil
	}

	best, found := "", false

	for _, next := range r.doors[in.From] {
		d, reachable := dist[next]
		if !reachable || d <= here {
			continue
		}

		if !found || d > dist[best] {
			best, found = next, true
		}
	}

	return &behavior.AwayOutput{Next: best, Found: found}, nil
}

// Present is the truth's answer to a swing: is that subject really there.
// A noise is not a subject on the truth surface, so a swing at a chant
// lands on nothing.
func (s *scene) Present(subject core.EntityID) bool {
	_, there := s.figures[subject]

	return there
}

// doors declares the dungeon: each pair is one door.
func (s *scene) doors(pairs ...[2]string) {
	for _, p := range pairs {
		s.rooms.connect(p[0], p[1])
	}
}

func door(a, b string) [2]string { return [2]string{a, b} }

// mind gives an actor a mind and places it. The monsters in these stories
// do not perceive each other: an actor is never a presence.
func (s *scene) mind(who core.EntityID, m behavior.Mind, where string) {
	s.g.Mind(&behavior.MindInput{Actor: who, Mind: m})
	s.g.Place(&behavior.PlaceInput{Actor: who, Where: where})
	s.actors = append(s.actors, who)
}

// bow arms an actor to strike one step away.
func (s *scene) bow(who core.EntityID) {
	s.g.Sheet(&behavior.SheetInput{Actor: who, Sheet: behavior.Sheet{Reach: 1}})
}

// sees gives each actor sight and hearing over the given places.
func (s *scene) sees(reach []string, who ...core.EntityID) {
	for _, o := range who {
		s.senses[o] = reach
	}
}

// person puts somebody on the truth surface: seen, and heard if chanting.
func (s *scene) person(id core.EntityID, where, look string, chanting bool) {
	s.figures[id] = figure{kind: creature, of: "human", look: look, where: where, chants: chanting}
}

// post puts a place a guard was told to stand on the truth surface,
// perceived like anything else.
func (s *scene) post(where string) {
	s.figures[banner] = figure{kind: post, of: "banner", where: where}
}

// moves relocates somebody, actor or figure.
func (s *scene) moves(id core.EntityID, where string) {
	if f, ok := s.figures[id]; ok {
		f.where = where
		s.figures[id] = f

		return
	}

	s.g.Place(&behavior.PlaceInput{Actor: id, Where: where})
}

// leaves removes somebody from the truth surface entirely.
func (s *scene) leaves(id core.EntityID) {
	delete(s.figures, id)
}

// reach is the physics: an observer's senses reach a subject when they
// reach the place the subject is in. The same senses serve every channel.
type reach struct {
	where  map[core.EntityID]string
	senses map[core.EntityID][]string
}

func (r reach) Reaches(_ perception.Channel, observer, subject core.EntityID) bool {
	return slices.Contains(r.senses[observer], r.where[subject])
}

// look advances the clock one tick and lets everyone perceive: one sight
// pass over every figure, one hearing pass over every chant. Every actor is
// in every pass, so what nobody reaches fades.
func (s *scene) look() {
	s.t.Helper()

	s.tick++

	where := make(map[core.EntityID]string)
	seen := make([]perception.Presence, 0, len(s.figures))
	heard := make([]perception.Presence, 0, len(s.figures))

	for id, f := range s.figures {
		where[id] = f.where
		seen = append(seen, perception.Presence{
			ID:      id,
			Payload: encode(percept{Kind: f.kind, Of: f.of, Note: f.look, Where: f.where}),
		})

		if f.chants {
			where[hearingOf(id)] = f.where
			heard = append(heard, perception.Presence{
				ID:      hearingOf(id),
				Payload: encode(percept{Kind: noise, Note: chanting, Where: f.where}),
			})
		}
	}

	r := reach{where: where, senses: s.senses}

	for _, pass := range []perception.Pass{
		{At: s.tick, Channel: sight, Presences: seen, Observers: s.actors, Reach: r},
		{At: s.tick, Channel: hearing, Presences: heard, Observers: s.actors, Reach: r},
	} {
		require.NoError(s.t, s.g.Observe(pass))
	}
}

// looks advances the clock n ticks.
func (s *scene) looks(n uint64) {
	for range n {
		s.look()
	}
}

// happens lands a deed on everyone whose senses reached the place.
func (s *scene) happens(actor core.EntityID, verb string, target core.EntityID, where string) {
	s.t.Helper()

	s.tick++

	var witnesses []core.EntityID

	for _, o := range s.actors {
		if slices.Contains(s.senses[o], where) {
			witnesses = append(witnesses, o)
		}
	}

	require.NoError(s.t, stage.Land(&stage.LandInput{
		Game:      s.g,
		Deed:      deed.Deed{Actor: actor, Target: target, Verb: verb, Where: where},
		Witnesses: witnesses,
		At:        s.tick,
	}))
}

// frightens puts the frightened condition on an actor.
func (s *scene) frightens(who, of core.EntityID) {
	s.g.Frighten(&behavior.FrightenInput{Actor: who, Source: of})
}

// turn asks an actor what it means to do right now.
func (s *scene) turn(who core.EntityID) *turn {
	s.t.Helper()

	out, err := s.g.Turn(&behavior.TurnInput{Actor: who, At: s.tick})
	require.NoError(s.t, err)

	return &turn{s: s, who: who, out: out}
}

// turn is one actor's decision, with the questions a proof asks of it.
type turn struct {
	s   *scene
	who core.EntityID
	out *behavior.TurnOutput
}

// aims is the subject the intent's target really resolves to.
func (t *turn) aims() core.EntityID {
	t.s.t.Helper()

	aimed, err := stage.Aim(&stage.AimInput{Truth: t.s, Situation: t.out.Situation, Intent: t.out.Intent})
	require.NoError(t.s.t, err)

	return aimed.Source
}

// steps is where the intent would walk the actor, and whether it can.
func (t *turn) steps() (string, bool) {
	t.s.t.Helper()

	stepped, err := stage.Step(&stage.StepInput{Space: t.s.rooms, Situation: t.out.Situation, Intent: t.out.Intent})
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
func (t *turn) attacks(source core.EntityID, why string) {
	t.s.t.Helper()

	require.Equal(t.s.t, behavior.Attack, t.out.Intent.Verb, why)
	assert.Equal(t.s.t, source, t.aims(), why)
}

// walksTo asserts the actor steps toward something and arrives at a place.
func (t *turn) walksTo(where, why string) {
	t.s.t.Helper()

	require.Equal(t.s.t, behavior.Toward, t.out.Intent.Verb, why)

	to, moved := t.steps()
	require.True(t.s.t, moved, why)
	assert.Equal(t.s.t, where, to, why)
}

// backsOff asserts the actor steps away from a place and lands somewhere
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

// moved walks the actor where it decided to go.
func (t *turn) moved() {
	t.s.t.Helper()

	to, ok := t.steps()
	require.True(t.s.t, ok)
	t.s.moves(t.who, to)
}

// contactHolding is the actor's contact that holds a subject.
func (t *turn) contactHolding(subject core.EntityID) behavior.Contact {
	t.s.t.Helper()

	for _, c := range t.out.Situation.Contacts {
		if c.Holds(subject) {
			return c
		}
	}

	require.Failf(t.s.t, "not held", "%s holds nothing of %s", t.who, subject)

	return behavior.Contact{}
}

// holdsNothingOf asserts the actor has no holding at all on a subject.
func (t *turn) holdsNothingOf(subject core.EntityID, why string) {
	t.s.t.Helper()

	for _, c := range t.out.Situation.Contacts {
		assert.False(t.s.t, c.Holds(subject), why)
	}
}

// hearingOf is the subject a figure's chant is heard under: qualified by
// channel, so the store can never merge it with the sight of the figure.
// Merging is the mind's judgment.
func hearingOf(id core.EntityID) core.EntityID {
	return core.EntityID(string(hearing) + "|" + string(id))
}
