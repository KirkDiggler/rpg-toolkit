// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestABlobNamedInspiredUnderAnotherTypeIsNotADie: the question is whether the
// stored ref IS Inspired, all of it. A blob whose id is "inspired" under the
// features type is a different noun. HoldsRef reads refs and loads nothing, so
// the unregistered ref is not an error here, only a no.
func TestABlobNamedInspiredUnderAnotherTypeIsNotADie(t *testing.T) {
	repo := &equipmentCharacters{byID: map[string]*character.Data{
		"ally": {ID: "ally", Conditions: []json.RawMessage{
			json.RawMessage(`{"ref":{"module":"dnd5e","type":"features","id":"inspired"}}`),
		}},
	}}
	m := &Manager{characters: repo}

	held, err := m.holdsInspiration(context.Background(), "ally")
	require.NoError(t, err)
	require.False(t, held, "dnd5e:features:inspired is not dnd5e:conditions:inspired")
}

// TestAnInspiredBlobIsADie is the other half of the full-ref comparison: the
// real condition's blob is found.
func TestAnInspiredBlobIsADie(t *testing.T) {
	blob, err := conditions.NewInspiredCondition("ally", "bard", "").ToJSON()
	require.NoError(t, err)
	repo := &equipmentCharacters{byID: map[string]*character.Data{
		"ally": {ID: "ally", Conditions: []json.RawMessage{blob}},
	}}
	m := &Manager{characters: repo}

	held, err := m.holdsInspiration(context.Background(), "ally")
	require.NoError(t, err)
	require.True(t, held, refs.Conditions.Inspired().String())
}

// inspirationAllies builds the candidate universe for a bard beside one ally,
// over the given repository: the ally is seen, standing and in reach, so the
// only thing left to decide is the sheet.
func inspirationAllies(t *testing.T, repo CharacterRepository) ([]targetPreflight, error) {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 14, 14)},
		},
		Members: []encounter.MemberInput{
			{ID: "bard", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
			{ID: "ally", Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 5}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sheets:        encStandStill{},
			Sight:         encStatedSight{},
			Equipment:     encNoHandsObserved{},
			Conditions:    encNoHandsObserved{},
			Initiative:    walkOrderAsGiven{},
			Driver:        passDriver{},
			Participation: walkEveryoneStanding{},
			Actors: encounter.Actors{
				Striker:   encounter.RefusingStriker{},
				Mover:     encounter.RefusingMover{},
				Announcer: encQuietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	roster := []encounter.Member{
		{ID: "bard", Kind: encounter.KindPlayer},
		{ID: "ally", Kind: encounter.KindPlayer},
	}
	positions := map[string]spatial.Position{
		"bard": {X: 5, Y: 5}, "ally": {X: 6, Y: 5},
	}
	holdings := []perception.Holding{
		{Subject: "ally", CurrentVia: []perception.Channel{perception.Sight}},
	}
	m := &Manager{characters: repo}
	return m.inspirationCandidates(
		context.Background(), enc, walkEveryoneStanding{}, roster, positions, holdings, "bard")
}

// TestAnUnreadableAllySheetRefusesInspirationCandidates: an ally whose sheet
// cannot be read makes the candidate build fail. "We could not check" is not
// "they hold none", and an available row over it would let a second die land
// on the first. Only ErrNoCharacter means "no sheet".
func TestAnUnreadableAllySheetRefusesInspirationCandidates(t *testing.T) {
	t.Run("stored conditions that do not parse", func(t *testing.T) {
		// The blob must not parse: `{}` or `{"ref":null}` parses, compares
		// unequal and answers no.
		repo := &equipmentCharacters{byID: map[string]*character.Data{
			"ally": {ID: "ally", Conditions: []json.RawMessage{json.RawMessage("not json")}},
		}}

		_, err := inspirationAllies(t, repo)

		require.Error(t, err)
		require.Contains(t, err.Error(), "inspiration offers")
	})

	t.Run("a repository that fails some other way", func(t *testing.T) {
		boom := errors.New("repository down")

		_, err := inspirationAllies(t, failingCharacters{err: boom})

		require.ErrorIs(t, err, boom)
	})
}

// TestAnAllyWithNoSheetIsAnAvailableCandidate: a member the repository does
// not hold holds no die this seam can see, so the row stays available.
func TestAnAllyWithNoSheetIsAnAvailableCandidate(t *testing.T) {
	got, err := inspirationAllies(t, &equipmentCharacters{byID: map[string]*character.Data{}})

	require.NoError(t, err)
	var ally *targetPreflight
	for i := range got {
		if got[i].member == "ally" {
			ally = &got[i]
		}
	}
	require.NotNil(t, ally, "the ally keeps their row")
	require.True(t, ally.available)
}

type failingCharacters struct{ err error }

func (f failingCharacters) GetCharacter(context.Context, string) (*character.Data, error) {
	return nil, f.err
}

func (failingCharacters) SaveCharacter(context.Context, *character.Data) error { return nil }
