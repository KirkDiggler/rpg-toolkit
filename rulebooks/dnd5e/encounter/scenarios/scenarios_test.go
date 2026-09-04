// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package scenarios_test

// scenarios_test.go pins the two things that must stay true of EVERY
// scenario, now and after the second one lands (rpg-project#368, design
// §3.2; the form ruled 2026-09-01).
//
// Both run over [scenarios.All] rather than over a named scenario, so the
// next scenario cannot skip them by not being mentioned here.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/scenarios"
)

// TestEveryScenarioDescribesItself is the shape pin: a descriptor a builder
// cannot render is a form nobody can fill in.
func TestEveryScenarioDescribesItself(t *testing.T) {
	all := scenarios.All()
	require.NotEmpty(t, all)

	seen := map[string]bool{}
	for _, s := range all {
		t.Run(s.ID(), func(t *testing.T) {
			require.NotEmpty(t, s.ID())
			require.False(t, seen[s.ID()], "two scenarios share the id %q", s.ID())
			seen[s.ID()] = true

			require.NotEmpty(t, s.Name(), "the picker shows this")
			require.NotEmpty(t, s.Fields(), "a scenario with no form binds nothing")

			keys := map[string]bool{}
			for _, f := range s.Fields() {
				require.NotEmpty(t, f.Key)
				require.False(t, keys[f.Key], "two fields share the key %q", f.Key)
				keys[f.Key] = true
				require.NotEmpty(t, f.Label)
				require.NotEmpty(t, f.Guidance, "the guidance IS the refusal; a field without one cannot refuse")
				require.Contains(t,
					[]scenarios.FieldType{scenarios.FieldEntityRef, scenarios.FieldCheck}, f.Type)
				if f.Type == scenarios.FieldEntityRef {
					require.NotEmpty(t, f.Kind, "an entity_ref picker has to know what to list")
				}
			}
		})

		found, ok := scenarios.Lookup(s.ID())
		require.True(t, ok, "All and Lookup must agree")
		require.Equal(t, s.ID(), found.ID())
	}
}

// TestDescriptorAndConfigAgreeBothWays is THE PIN design §3.2 asks for, and
// it runs in both directions:
//
//	descriptor → config: filling in exactly the declared fields constructs.
//	config → descriptor: leaving out any ONE declared field refuses, in that
//	field's own guidance.
//
// The second direction is what makes the first mean something. A scenario
// that declared a field it never read would pass a one-way check and hand
// the author a control that does nothing.
func TestDescriptorAndConfigAgreeBothWays(t *testing.T) {
	facts := fullDungeon()

	for _, s := range scenarios.All() {
		t.Run(s.ID(), func(t *testing.T) {
			complete := completeConfig(t, s)

			t.Run("the whole form constructs", func(t *testing.T) {
				declared, err := s.New(complete, facts)
				require.NoError(t, err)
				require.NotEmpty(t, declared.Endings, "a scenario declares at least one ending")
				for _, e := range declared.Endings {
					require.NotEmpty(t, e.Key)
					require.NotNil(t, e.Trigger)
				}
			})

			for _, f := range s.Fields() {
				t.Run("without "+f.Key, func(t *testing.T) {
					partial := map[string]string{}
					for k, v := range complete {
						if k != f.Key {
							partial[k] = v
						}
					}
					_, err := s.New(partial, facts)
					require.Error(t, err, "field %q is declared but never read", f.Key)
					require.Contains(t, err.Error(), f.Key,
						"the refusal names the field the form asked about")
					require.Contains(t, err.Error(), f.Guidance,
						"the guidance and the refusal are one sentence, or they will drift")
				})
			}

			t.Run("nothing is defaulted", func(t *testing.T) {
				_, err := s.New(map[string]string{}, facts)
				require.Error(t, err, "an empty form is not a scenario")
			})

			t.Run("no dungeon to bind to", func(t *testing.T) {
				_, err := s.New(complete, nil)
				require.Error(t, err)
			})
		})
	}
}

// completeConfig fills in every declared field with something the fixture
// dungeon below actually has, chosen by the field's KIND — which is the only
// thing a generic filler is allowed to read, and therefore a second check
// that the kind vocabulary means what the picker will assume.
func completeConfig(t *testing.T, s scenarios.Scenario) map[string]string {
	t.Helper()
	cfg := map[string]string{}
	for _, f := range s.Fields() {
		switch {
		case f.Type == scenarios.FieldEntityRef && f.Kind == "prop":
			cfg[f.Key] = takeableID
		case f.Type == scenarios.FieldEntityRef && f.Kind == "exit":
			cfg[f.Key] = exitID
		default:
			t.Fatalf("scenario %q declares a field this test cannot fill: %+v — "+
				"add the case here when the scenario that needs it lands", s.ID(), f)
		}
	}
	return cfg
}

const (
	takeableID = "heirloom"
	sceneryID  = "pillar"
	exitID     = "front-gate"
)

// fullDungeon is a dungeon with one of everything a scenario can bind to,
// plus a prop that is NOT takeable so the wrong-kind refusals have something
// real to be about.
func fullDungeon() *scenarios.DungeonFacts {
	return scenarios.FactsFrom(encounter.FieldInput{
		Props: []encounter.PropInput{
			{ID: takeableID, Takeable: true, Ref: "dnd5e:props:reliquary"},
			{ID: sceneryID, Ref: "dnd5e:props:pillar"},
			// A prop the author never named: not bindable, and not in the
			// facts at all.
			{Ref: "dnd5e:props:candles"},
		},
		Exits: []encounter.FieldExit{{ID: exitID}},
	})
}

// TestFactsFromNarrowsToIdsAndTakeability pins the one place a compiled
// dungeon is narrowed to what a scenario may ask about.
func TestFactsFromNarrowsToIdsAndTakeability(t *testing.T) {
	facts := fullDungeon()

	require.True(t, facts.Props[takeableID])
	require.False(t, facts.Props[sceneryID])
	require.Len(t, facts.Props, 2, "a prop the author never named cannot be bound to")
	require.True(t, facts.Exits[exitID])
	require.Len(t, facts.Exits, 1)
}

// TestRecoverTheArtifactRefusals is this scenario's own refusal set, each in
// form-filler words (design §3.2).
func TestRecoverTheArtifactRefusals(t *testing.T) {
	facts := fullDungeon()
	s, ok := scenarios.Lookup(scenarios.RecoverTheArtifactID)
	require.True(t, ok)

	good := map[string]string{
		scenarios.FieldArtifact: takeableID,
		scenarios.FieldExitKey:  exitID,
	}

	t.Run("it declares one ending, on its own key", func(t *testing.T) {
		declared, err := s.New(good, facts)
		require.NoError(t, err)
		require.Len(t, declared.Endings, 1)
		require.Equal(t, scenarios.RecoverTheArtifactID, declared.Endings[0].Key)
		require.Equal(t,
			encounter.TriggerExitedHolding{Exit: exitID, Item: takeableID},
			declared.Endings[0].Trigger)
		require.Equal(t, takeableID, declared.Artifact)
		require.Equal(t, exitID, declared.Exit)
	})

	t.Run("an artifact this dungeon does not place", func(t *testing.T) {
		_, err := s.New(with(good, scenarios.FieldArtifact, "chalice"), facts)
		require.ErrorContains(t, err, "not a thing this dungeon places")
		require.ErrorContains(t, err, "chalice")
	})

	t.Run("an artifact that is scenery", func(t *testing.T) {
		_, err := s.New(with(good, scenarios.FieldArtifact, sceneryID), facts)
		require.ErrorContains(t, err, "scenery")
		require.ErrorContains(t, err, "Mark it takeable",
			"the refusal says what to do about it, not just what is wrong")
	})

	t.Run("an exit this dungeon does not declare", func(t *testing.T) {
		_, err := s.New(with(good, scenarios.FieldExitKey, "back-door"), facts)
		require.ErrorContains(t, err, "not a way out this dungeon declares")
	})

	t.Run("every refusal reads as a sentence to somebody filling in a form", func(t *testing.T) {
		for _, cfg := range []map[string]string{
			{},
			with(good, scenarios.FieldArtifact, ""),
			with(good, scenarios.FieldArtifact, sceneryID),
			with(good, scenarios.FieldExitKey, "back-door"),
		} {
			_, err := s.New(cfg, facts)
			require.Error(t, err)
			require.True(t,
				strings.Contains(err.Error(), "this scenario needs"),
				"refusal does not tell the author what the scenario needs: %q", err)
		}
	})
}

// with copies a config with one key changed — so a scene never mutates the
// map the next scene reads.
func with(base map[string]string, key, value string) map[string]string {
	out := make(map[string]string, len(base))
	for k, v := range base {
		out[k] = v
	}
	out[key] = value
	return out
}
