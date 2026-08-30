// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/require"
)

// refContractTable is one live instance of every condition [LoadJSON] can
// route. Kept honest by TestRefContractCoversEveryLoadedCondition.
func refContractTable() map[string]dnd5eEvents.ConditionBehavior {
	roller := dice.NewRoller()

	return map[string]dnd5eEvents.ConditionBehavior{
		"raging":            &RagingCondition{CharacterID: "m1"},
		"brutal_critical":   NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: "m1", Level: 9, Roller: roller}),
		"unarmored_defense": NewUnarmoredDefenseCondition(UnarmoredDefenseInput{MemberID: "m1", Type: UnarmoredDefenseBarbarian}),
		"fs_archery":        NewFightingStyleArcheryCondition("m1"),
		"fs_defense":        NewFightingStyleDefenseCondition("m1"),
		"fs_dueling":        NewFightingStyleDuelingCondition("m1"),
		"fs_gwf":            NewFightingStyleGreatWeaponFightingCondition("m1", roller),
		"fs_protection":     NewFightingStyleProtectionCondition("m1"),
		"fs_twf":            NewFightingStyleTwoWeaponFightingCondition("m1"),
		"improved_critical": NewImprovedCriticalCondition(ImprovedCriticalInput{MemberID: "m1"}),
		"reckless_attack":   NewRecklessAttackCondition("m1"),
		"martial_arts":      NewMartialArtsCondition(MartialArtsInput{MemberID: "m1", MonkLevel: 5, Roller: roller}),
		"unarmored_move":    NewUnarmoredMovementCondition(UnarmoredMovementInput{MemberID: "m1", MonkLevel: 5}),
		"sneak_attack":      NewSneakAttackCondition(SneakAttackInput{MemberID: "m1", Level: 5, Roller: roller}),
		"disengaging":       NewDisengagingCondition("m1"),
		"dodging":           NewDodgingCondition("m1"),
		"prone":             NewProneCondition("m1"),
		"hidden":            NewHiddenCondition("m1"),
		"helped":            NewHelpedCondition("m1", "helper-1"),
		"unconscious":       NewUnconsciousCondition("m1", roller),
		"opportunity":       NewOpportunityAttackCondition("m1"),
		"shield_spell":      NewShieldSpellCondition("m1"),
	}
}

// TestEveryConditionRefMatchesItsToJSON pins the contract both keepers now
// depend on: [dnd5eEvents.ConditionBehavior.Ref] returns the same ref that
// condition's ToJSON embeds.
//
// It is the paired half of rpg-project#319 Phase 6's filter conversion. Before
// it, the character keeper recovered a condition's ref by round-tripping the
// condition through ToJSON on every removal, so the two could not disagree —
// the JSON was the only answer either keeper asked for. Both keepers now ask
// Ref() directly, which is cheaper and cannot fail mid-list, and the price of
// that is that a condition whose Ref() drifted from its ToJSON would stop
// matching its own removals. Silently, and only in production.
//
// So the disagreement is checked here instead, where it is loud, and for EVERY
// condition rather than the seven that publish removals: any of them can sit in
// a keeper's list and be removed by ref, and monstertraits.LoadJSON routes any
// conditions-typed ref straight into LoadJSON, so a monster can carry any of
// them too.
func TestEveryConditionRefMatchesItsToJSON(t *testing.T) {
	for name, condition := range refContractTable() {
		t.Run(name, func(t *testing.T) {
			ref := condition.Ref()
			require.NotNil(t, ref, "Ref() must never be nil: a keeper cannot match a removal against nothing")

			blob, err := condition.ToJSON()
			require.NoError(t, err)

			var embedded struct {
				Ref core.Ref `json:"ref"`
			}
			require.NoError(t, json.Unmarshal(blob, &embedded))

			require.Equal(t, ref.String(), embedded.Ref.String(),
				"Ref() and the ref ToJSON embeds must agree, or this condition cannot be removed by ref")
		})
	}
}

// TestRefContractCoversEveryLoadedCondition keeps that table honest.
//
// A condition added to LoadJSON's switch but not to the table would leave the
// contract unchecked for exactly the conditions most likely to be new. Rather
// than trust a reader to notice, this reads the switch and requires one table
// entry per routed case — the "the list is the rule" shape monstertraits uses
// for its trait refs, and the source-reading shape resolution/truth_test.go
// uses for its pins.
func TestRefContractCoversEveryLoadedCondition(t *testing.T) {
	src, err := os.ReadFile("loader.go")
	require.NoError(t, err)

	cases := regexp.MustCompile(`(?m)^\tcase refs\.[A-Za-z]+\.[A-Za-z]+\(\)\.ID:`).FindAllString(string(src), -1)
	require.NotEmpty(t, cases, "found no case labels — this test's regex has drifted from loader.go")

	table := refContractTable()
	require.Len(t, table, len(cases),
		"LoadJSON routes %d conditions but refContractTable constructs %d; add the new one to the table",
		len(cases), len(table))

	seen := map[string]string{}
	for name, condition := range table {
		id := condition.Ref().String()
		if other, dup := seen[id]; dup {
			t.Fatalf("table entries %q and %q are both %s, so the count above is met without covering everything",
				other, name, id)
		}
		seen[id] = name
	}
}
