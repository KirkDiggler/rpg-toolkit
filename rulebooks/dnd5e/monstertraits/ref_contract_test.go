// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/require"
)

// traitContractTable is one live instance of every TRAIT [LoadJSON] can build
// itself. Conditions routed on to conditions.LoadJSON are covered by that
// package's own TestEveryConditionRefMatchesItsToJSON; these four are the ones
// only this package constructs, and until rpg-project#319 Phase 6 they were
// covered by nothing.
func traitContractTable() map[string]dnd5eEvents.ConditionBehavior {
	return map[string]dnd5eEvents.ConditionBehavior{
		"immunity":        &immunityCondition{ownerID: "m1", damageType: damage.Fire},
		"vulnerability":   &vulnerabilityCondition{ownerID: "m1", damageType: damage.Cold},
		"packTactics":     &packTacticsCondition{ownerID: "m1"},
		"undeadFortitude": &undeadFortitudeCondition{ownerID: "m1", conModifier: 3, roller: dice.NewRoller()},
	}
}

// TestEveryTraitRefMatchesItsToJSON is the monstertraits half of the contract
// [Monster.AddLoadedCondition] depends on: Ref() returns the same ref its
// ToJSON embeds, and never nil.
//
// It exists because the conditions package's version does not reach here.
// LoadJSON routes anything with a conditions-typed ref on to
// conditions.LoadJSON — covered there — and builds these four itself, which
// nothing covered. That gap is exactly what let a "the loaders cannot produce
// a nameless condition" argument look true while these four sat outside it.
func TestEveryTraitRefMatchesItsToJSON(t *testing.T) {
	for name, trait := range traitContractTable() {
		t.Run(name, func(t *testing.T) {
			ref := trait.Ref()
			require.NotNil(t, ref, "Ref() must never be nil: a sheet will refuse this trait outright")

			blob, err := trait.ToJSON()
			require.NoError(t, err)

			var embedded struct {
				Ref core.Ref `json:"ref"`
			}
			require.NoError(t, json.Unmarshal(blob, &embedded))

			require.Equal(t, ref.String(), embedded.Ref.String(),
				"Ref() and the ref ToJSON embeds must agree, or this trait cannot be removed by ref")
		})
	}
}

// TestTraitContractCoversEveryLoadedTrait keeps that table honest, the same way
// conditions.TestRefContractCoversEveryLoadedCondition keeps its own: it reads
// the switch rather than trusting a reader to notice a new case.
//
// Only the trait cases count. The conditions-typed branch above the switch is
// covered by the conditions package's contract test, not by this table.
func TestTraitContractCoversEveryLoadedTrait(t *testing.T) {
	src, err := os.ReadFile("loader.go")
	require.NoError(t, err)

	cases := regexp.MustCompile(`(?m)^\tcase refs\.MonsterTraits\.[A-Za-z]+\(\)\.ID:`).FindAllString(string(src), -1)
	require.NotEmpty(t, cases, "found no trait case labels — this test's regex has drifted from loader.go")

	table := traitContractTable()
	require.Len(t, table, len(cases),
		"LoadJSON builds %d traits but traitContractTable constructs %d; add the new one to the table",
		len(cases), len(table))

	seen := map[string]string{}
	for name, trait := range table {
		id := trait.Ref().String()
		if other, dup := seen[id]; dup {
			t.Fatalf("table entries %q and %q are both %s, so the count above is met without covering everything",
				other, name, id)
		}
		seen[id] = name
	}
}
