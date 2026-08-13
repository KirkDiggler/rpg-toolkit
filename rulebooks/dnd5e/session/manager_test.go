// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// copyOf returns an independent copy, the way a real repository does.
//
// This matters more than it looks. A fake that hands back the pointer it stores
// makes every in-memory mutation instantly "persisted", so a verb that forgets
// to save still passes — the store and the working copy are the same object.
// Found by a mutant that deleted a save and survived; every fake in this file
// had the flaw, and had had it since wave 1.
func copyOf[T any](in *T) (*T, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// fakeSessions is an in-memory SessionRepository. Key-value only, matching S12:
// if a test double needed more than get-and-put to satisfy it, the interface
// would be asking too much of a real store.
type fakeSessions struct {
	byID map[string]*session.SessionData
}

func newFakeSessions() *fakeSessions {
	return &fakeSessions{byID: map[string]*session.SessionData{}}
}

func (f *fakeSessions) GetSession(_ context.Context, id string) (*session.SessionData, error) {
	data, ok := f.byID[id]
	if !ok {
		return nil, session.ErrNotFound
	}
	return copyOf(data)
}

func (f *fakeSessions) SaveSession(_ context.Context, data *session.SessionData) error {
	f.byID[data.ID] = data
	return nil
}

// fakeEncounters is an in-memory EncounterRepository.
type fakeEncounters struct {
	byID map[string]*encounter.EncounterData
}

func newFakeEncounters() *fakeEncounters {
	return &fakeEncounters{byID: map[string]*encounter.EncounterData{}}
}

func (f *fakeEncounters) GetEncounter(_ context.Context, id string) (*encounter.EncounterData, error) {
	data, ok := f.byID[id]
	if !ok {
		return nil, session.ErrNotFound
	}
	return copyOf(data)
}

func (f *fakeEncounters) SaveEncounter(_ context.Context, id string, data *encounter.EncounterData) error {
	f.byID[id] = data
	return nil
}

// fakeCharacters is an in-memory CharacterRepository.
type fakeCharacters struct {
	byID map[string]*character.Data

	// loads counts GetCharacter calls, so a test can assert that loading
	// happens per call rather than being cached between them.
	loads int
}

func newFakeCharacters(chars ...*character.Data) *fakeCharacters {
	f := &fakeCharacters{byID: map[string]*character.Data{}}
	for _, c := range chars {
		f.byID[c.ID] = c
	}
	return f
}

func (f *fakeCharacters) GetCharacter(_ context.Context, id string) (*character.Data, error) {
	f.loads++
	data, ok := f.byID[id]
	if !ok {
		return nil, session.ErrNotFound
	}
	return cloneCharacter(data), nil
}

// cloneCharacter copies without going through JSON.
//
// Characters get their own clone because one fixture deliberately holds a
// MALFORMED json.RawMessage to exercise the upstream drop path, and malformed
// raw JSON cannot be marshalled. That fixture is reachable in process but not
// from a real JSON store, so modelling it costs a struct copy rather than a
// round trip.
func cloneCharacter(in *character.Data) *character.Data {
	out := *in
	out.Conditions = append([]json.RawMessage(nil), in.Conditions...)
	out.Features = append([]json.RawMessage(nil), in.Features...)
	out.Inventory = append([]character.InventoryItemData(nil), in.Inventory...)
	return &out
}

func (f *fakeCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	f.byID[data.ID] = data
	return nil
}

// dwarfCharacter builds a minimal stored character.
//
// A DWARF on purpose. Speed is not a field of character.Data at all — it is
// derived from race by the loaded character — and a dwarf's 25 is distinct
// both from a human's 30 and from the 30 that GetSpeed falls back to when race
// data is missing. So an assertion of 25 can only pass if the stored bytes were
// genuinely reconstituted, which is the thing worth proving.
func dwarfCharacter(id string) *character.Data {
	return &character.Data{
		ID:               id,
		PlayerID:         "player-" + id,
		Name:             "Alice",
		Level:            3,
		ProficiencyBonus: 2,
		RaceID:           races.Dwarf,
		ClassID:          classes.Fighter,
		HitPoints:        24,
		MaxHitPoints:     28,
		ArmorClass:       16,
	}
}

// testCharacters holds the cast the suites join through Join. Members placed
// directly into a world fixture never pass through the repository, so only the
// ones a test actually joins need to exist here.
func testCharacters() *fakeCharacters {
	return newFakeCharacters(
		dwarfCharacter("alice"),
		dwarfCharacter("bob"),
		dwarfCharacter("carol"),
		dwarfCharacter("dave"),
		dwarfCharacter("erin"),
	)
}

// fakeStream records what was published.
type fakeStream struct {
	published []session.Event
}

func (f *fakeStream) Publish(_ context.Context, events []session.Event) error {
	f.published = append(f.published, events...)
	return nil
}

// ManagerTestSuite covers construction (S8): the manager refuses to exist
// without what it needs, and says which piece is missing.
type ManagerTestSuite struct {
	suite.Suite
}

// TestNilConfigRejected distinguishes a nil config from an incomplete one. They
// are different mistakes — one is a bad call site, the other an incomplete
// wiring decision — and collapsing them into one error would send whoever hits
// it looking in the wrong place.
func (s *ManagerTestSuite) TestNilConfigRejected() {
	mgr, err := session.NewManager(nil)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNilConfig)
	s.NotErrorIs(err, session.ErrIncompleteConfig, "a nil config is not an incomplete one")
	s.Nil(mgr)
}

// TestEachRequirementIsCheckedByName is the heart of S8. Every required
// dependency gets its own row, so a check that silently stopped validating one
// of them fails here rather than surfacing as a nil-pointer panic mid-turn in
// production.
//
// The assertion is on the NAME appearing in the message, not merely on the
// sentinel: a host wiring several at once needs to be told which one, and an
// error that only says "incomplete config" makes that a guessing game.
func (s *ManagerTestSuite) TestEachRequirementIsCheckedByName() {
	cases := []struct {
		name   string
		config *session.Config
		expect string
	}{
		{
			name: "sessions absent",
			config: &session.Config{Encounters: newFakeEncounters(),
				Characters: testCharacters(), Events: session.DiscardEvents{},
			},
			expect: "Sessions",
		},
		{
			name: "encounters absent",
			config: &session.Config{Sessions: newFakeSessions(),
				Characters: testCharacters(), Events: session.DiscardEvents{},
			},
			expect: "Encounters",
		},
		{
			name: "characters absent",
			config: &session.Config{Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
				Events: session.DiscardEvents{},
			},
			expect: "Characters",
		},
		{
			name: "events absent",
			config: &session.Config{Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
				Characters: testCharacters(),
			},
			expect: "Events",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			mgr, err := session.NewManager(tc.config)
			s.Require().Error(err)
			s.ErrorIs(err, session.ErrIncompleteConfig)
			s.Contains(err.Error(), tc.expect, "the error must name what is missing")
			s.Nil(mgr)
		})
	}
}

// TestMissingReportIsDeterministic pins that a config missing several things
// always names the same one first. A host fixing its wiring should see a stable
// sequence rather than a message that changes between runs — the difference
// between "fix these in order" and "guess again."
func (s *ManagerTestSuite) TestMissingReportIsDeterministic() {
	for i := 0; i < 20; i++ {
		_, err := session.NewManager(&session.Config{})
		s.Require().Error(err)
		s.Contains(err.Error(), "Sessions", "the first missing name must be stable across runs")
	}
}

// TestDiscardEventsIsAcceptedAsAStream is the must-accept row.
//
// The stream is required, which makes the over-tightening risk real: a check
// that demanded a "real" stream — anything beyond non-nil — would break every
// test, every headless simulation, and every analysis run, while passing every
// rejection row above.
//
// The point of DiscardEvents is that wanting no delivery stays possible while
// being a stated decision. A nil reads as an oversight; this reads as a choice,
// and it greps.
func (s *ManagerTestSuite) TestDiscardEventsIsAcceptedAsAStream() {
	mgr, err := session.NewManager(&session.Config{
		Sessions:   newFakeSessions(),
		Encounters: newFakeEncounters(),
		Characters: testCharacters(),
		Events:     session.DiscardEvents{},
	})
	s.Require().NoError(err, "an explicit no-op stream is a legitimate configuration")
	s.NotNil(mgr)
}

// TestFullyWiredConstructs is the other positive control: a real stream,
// which is what any actual game supplies.
func (s *ManagerTestSuite) TestFullyWiredConstructs() {
	mgr, err := session.NewManager(&session.Config{
		Sessions:   newFakeSessions(),
		Encounters: newFakeEncounters(),
		Characters: testCharacters(),
		Events:     &fakeStream{},
	})
	s.Require().NoError(err)
	s.NotNil(mgr)
}

func TestManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerTestSuite))
}
