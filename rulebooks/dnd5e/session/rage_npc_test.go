// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// rage_npc_test.go plays "a creature rages" (rpg-project#1131, slice
// rpg-project#1292) on the site of activate_npc_test.go with two pieces of
// text replaced: the Lieutenant's stat block lets him rage (two uses, a
// damage bonus of 2) and his table's first row says `activate: rage`. The
// module adds no word: the rules' table of named abilities carries Rage
// (rulebooks/dnd5e v0.252.0) and its refusal of a second one (v0.253.0).
//
// THE DICE are secondWindDice, as in activate_npc_test.go. The fighter has 60
// of 60 hit points and only ends her turn, so one EndTurn of hers is one
// whole turn of his.

const (
	secondWindLets = "      - use: second wind\n        with: { level: 1 }\n" +
		"        uses: { max: 1, refills: { on: [short rest, long rest] } }\n"
	rageLets = "      - use: rage\n        with: { damage_bonus: 2 }\n" +
		"        uses: { max: 2, refills: { on: [long rest] } }\n"
	rageRow = "{ activate: rage }"
)

// rageSite is the Lieutenant who rages.
func rageSite(t *testing.T) []byte {
	t.Helper()
	raw := string(usesVariant(t, secondWindLets, rageLets))
	return []byte(strings.Replace(raw, activateRow, rageRow, 1))
}

// rageVariant is the raging site with one piece of text replaced, once.
func rageVariant(t *testing.T, from, to string) []byte {
	t.Helper()
	raw := string(rageSite(t))
	require.Equal(t, 1, strings.Count(raw, from), "precondition: %q occurs once in the site", from)
	return []byte(strings.Replace(raw, from, to, 1))
}

// Test 1. The content check and the launch.
func TestTheContentCheckAndTheLaunchJudgeARagingCreature(t *testing.T) {
	check := func(raw []byte) *session.CheckContentOutput {
		compiled := compileSite(t, raw)
		out, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
		require.NoError(t, err)
		return out
	}

	t.Run("the site as written", func(t *testing.T) {
		out := check(rageSite(t))
		require.Empty(t, out.Activations)
		require.Empty(t, out.Refusals)
		require.Empty(t, out.Items)
		require.Empty(t, out.Templates)
		s, err := newLieutenantScene(t, compileSite(t, rageSite(t)))
		require.NoError(t, err)
		require.Equal(t, 2, s.sheet("lieutenant").Left["rage"], "he launches with both uses")
	})

	t.Run("a damage bonus of barbarian level is the barbarian's own", func(t *testing.T) {
		raw := rageVariant(t, "damage_bonus: 2", `damage_bonus: barbarian level`)
		out := check(raw)
		require.Len(t, out.Templates, 1)
		require.Equal(t, "gate-lieutenant", out.Templates[0].TemplateID)
		require.Contains(t, out.Templates[0].Message, `rage damage_bonus is "barbarian level"; it is a whole number of at least 1`)
		require.Contains(t, out.Templates[0].Message, `stat block "gate-lieutenant"`)
		t.Logf("content check: %s", out.Templates[0].Message)

		s, err := newLieutenantScene(t, compileSite(t, raw))
		require.Error(t, err)
		require.ErrorContains(t, err, `rage damage_bonus is "barbarian level"; it is a whole number of at least 1`)
		require.Empty(t, s.sessions.byID, "no session was stored")
	})

	t.Run("a rage with no values", func(t *testing.T) {
		raw := rageVariant(t, "        with: { damage_bonus: 2 }\n", "")
		out := check(raw)
		require.Len(t, out.Templates, 1)
		require.Contains(t, out.Templates[0].Message, "rage requires parameters")
		t.Logf("content check: %s", out.Templates[0].Message)

		s, err := newLieutenantScene(t, compileSite(t, raw))
		require.Error(t, err)
		require.ErrorContains(t, err, "rage requires parameters")
		require.Empty(t, s.sessions.byID, "no session was stored")
	})
}

// Test 1b. A row for an ability the stat block does not carry is not refused
// and never offered, as it is for Second Wind.
func TestARageRowForACreatureThatCannotRageIsNotRefusedAndNeverOffered(t *testing.T) {
	raw, err := os.ReadFile(lieutenantSitePath)
	require.NoError(t, err)
	older := string(raw)
	row := "        - { when: { enemy: walk }, toward: enemy }\n"
	require.Equal(t, 1, strings.Count(older, row))
	older = strings.Replace(older, row, "        - "+rageRow+"\n"+row, 1)
	lets := "    lets:\n      - use: second wind\n        with: { level: 1 }\n        uses: { max: 1, refills: { on: [short rest, long rest] } }\n"
	require.Equal(t, 1, strings.Count(older, lets))
	older = strings.Replace(older, lets, "", 1)
	compiled := compileSite(t, []byte(older))

	checked, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
	require.NoError(t, err)
	require.Empty(t, checked.Activations)
	require.Empty(t, checked.Templates)

	s := newUsesScene(t, compiled)
	require.Nil(t, s.sheet("lieutenant").Kit)
	s.play(2)
	require.Empty(t, beatsNamed(s.all(), encounter.BeatActivated), "he has nothing to use")
	require.NotEmpty(t, by(s.all(), "lieutenant", "struck", "missed", encounter.BeatMoved))
}

// Test 2. He rages on his first turn and still fights.
func TestTheLieutenantRagesOnHisFirstTurnAndStillFights(t *testing.T) {
	s := newUsesScene(t, compileSite(t, rageSite(t)))
	require.Equal(t, 1, s.sheet("lieutenant").SlotsLeft(coreCombat.ActionBonus))

	s.play(1)
	turn := s.rounds[0]
	t.Logf("beats of his first turn: %v", beatNames(turn))

	activated := by(turn, "lieutenant", encounter.BeatActivated)
	require.Len(t, activated, 1, "exactly one activated beat, his")
	var told struct {
		Ability session.SpellRef `json:"ability"`
		Target  string           `json:"target"`
	}
	activated[0].decode(t, &told)
	require.Equal(t, "Rage", told.Ability.Name)
	require.Equal(t, refs.Features.Rage().String(), told.Ability.Ref)
	require.Empty(t, told.Target, "it lands on its user")

	results := by(turn, "lieutenant", encounter.BeatActivationResult)
	require.Len(t, results, 1)
	result := resultOf(t, results[0])
	require.Equal(t, encounter.ResultConditionApplied, result.Kind)
	require.Equal(t, "lieutenant", result.Target)
	require.Equal(t, refs.Conditions.Raging().String(), result.Ref)
	require.Equal(t, "Raging", result.Name)

	sheet := s.sheet("lieutenant")
	require.Equal(t, 1, sheet.Left["rage"], "one use is left")
	require.Zero(t, sheet.SlotsLeft(coreCombat.ActionBonus), "the bonus action is spent")
	status := ragingStatusOf(t, sheet.Conditions)
	require.NotNil(t, status, "his stored sheet holds the raging status")
	require.Equal(t, refs.Features.Rage().String(), status.Origin.Ref)
	require.Equal(t, "lieutenant", status.Origin.EntityID)

	require.NotEmpty(t, by(turn, "lieutenant", "struck", "missed"), "he swung in the same turn")
}

// Test 3. The player is told it as a player's Rage is told.
func TestThePlayerIsToldTheLieutenantsRageAsAPlayersIs(t *testing.T) {
	s := newUsesScene(t, compileSite(t, rageSite(t)))
	s.play(1)

	var told []session.ActivateAct
	for _, act := range s.acts.of("alice") {
		if body, ok := act.Body.(session.ActivateAct); ok {
			told = append(told, body)
			require.Equal(t, "activate", act.Verb)
		}
	}
	require.Len(t, told, 1, "her client was handed his activation live")
	require.Equal(t, session.Party{Kind: session.PartyMember, ID: "lieutenant"}, told[0].By)
	require.Equal(t, "Rage", told[0].Ability.Name)
	require.Equal(t, session.Party{}, told[0].Target)
	require.Len(t, told[0].Results, 1)
	require.Equal(t, string(encounter.ResultConditionApplied), told[0].Results[0].Kind)
	require.Equal(t, session.Party{Kind: session.PartyMember, ID: "lieutenant"}, told[0].Results[0].Target)
	require.Equal(t, "Raging", told[0].Results[0].Name)
	t.Logf("sent to her: %+v", told[0])
}

// Test 4. He is not asked again while he rages: on his next turn his bonus
// action is back and a use is left, and his fact for Rage is not payable.
func TestTheLieutenantIsNotAskedToRageAgainWhileHeRages(t *testing.T) {
	s := newUsesScene(t, compileSite(t, rageSite(t)))
	s.play(2)
	t.Logf("beats of his second turn: %v", beatNames(s.rounds[1]))

	require.Len(t, by(s.all(), "lieutenant", encounter.BeatActivated), 1, "he raged once, on the first turn")
	require.Empty(t, by(s.rounds[1], "lieutenant", encounter.BeatActivated), "and was not asked again")
	sheet := s.sheet("lieutenant")
	require.Equal(t, 1, sheet.Left["rage"], "the use is still there")
	require.Equal(t, 1, sheet.SlotsLeft(coreCombat.ActionBonus), "his bonus action came back")
	require.NotNil(t, ragingStatusOf(t, sheet.Conditions), "he is still raging")
	read, err := monster.AbilitiesOf(&monster.AbilitiesOfInput{Sheet: sheet})
	require.NoError(t, err)
	require.Len(t, read.Abilities, 1)
	require.Equal(t, "rage", read.Abilities[0].Word)
	require.True(t, read.Abilities[0].Payable, "the sheet could pay for it")
	require.Equal(t, "already raging", read.Abilities[0].Refused, "and the rules refuse it: so the fact says not payable")
	require.NotEmpty(t, by(s.rounds[1], "lieutenant", "struck", "missed"), "he swings")
}

// Test 5. His hit carries it: the struck beat's damage components list the
// raging status, sourced to him, with its 2, and the blow is the d8's 8 plus 2.
func TestTheRagingLieutenantsHitCarriesTheRagingStatusDamage(t *testing.T) {
	s := newUsesScene(t, compileSite(t, rageSite(t)))
	s.play(1)

	hits := by(s.rounds[0], "lieutenant", "struck")
	require.Len(t, hits, 1, "one blow, and it lands (every die is a 10)")
	var struck struct {
		Amount     int `json:"amount"`
		Components []struct {
			Source string `json:"source"`
			Roll   struct {
				Source struct {
					Ref      string `json:"ref"`
					SourceID string `json:"source_id"`
				} `json:"source"`
				Modifier int `json:"modifier"`
			} `json:"roll"`
		} `json:"damage_components"`
	}
	hits[0].decode(t, &struck)

	var raging []int
	for _, c := range struck.Components {
		if c.Source == "condition" && c.Roll.Source.Ref == refs.Conditions.Raging().String() {
			require.Equal(t, "lieutenant", c.Roll.Source.SourceID)
			raging = append(raging, c.Roll.Modifier)
		}
	}
	require.Equal(t, []int{2}, raging, "one component from the raging status, worth 2")
	require.Equal(t, 10, struck.Amount, "the d8's 8 and the raging 2")
	t.Logf("a raging Lieutenant's hit: amount %d, %d components", struck.Amount, len(struck.Components))
}
