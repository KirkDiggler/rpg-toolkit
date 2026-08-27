// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

// ragingBarbarian is a level-1 barbarian carrying Rage, with a weapon so the
// Attack declaration compiles alongside.
func ragingBarbarian(id string, charges int) *character.Data {
	return &character.Data{
		ID:       id,
		PlayerID: "player-" + id,
		Name:     id,
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 16,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 15, MaxHitPoints: 15, ArmorClass: 15, ProficiencyBonus: 2,
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Greataxe), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{
			character.SlotMainHand: string(weapons.Greataxe),
		},
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.RageCharges: {
				Current: charges, Maximum: 2, ResetType: coreResources.ResetLongRest,
			},
		},
		Features: []json.RawMessage{json.RawMessage(
			`{"ref":{"module":"dnd5e","type":"features","id":"rage"},` +
				`"id":"rage","name":"Rage","level":1}`)},
	}
}

func activations(decls []session.Declaration) []session.Declaration {
	out := make([]session.Declaration, 0, len(decls))
	for _, d := range decls {
		if d.Verb == session.VerbActivate {
			out = append(out, d)
		}
	}
	return out
}

func activationFor(t *testing.T, decls []session.Declaration, ref string) session.Declaration {
	t.Helper()
	for _, d := range activations(decls) {
		if d.Ability != nil && d.Ability.Ref == ref {
			return d
		}
	}
	t.Fatalf("no activation declaration for %q in %d declarations", ref, len(decls))
	return session.Declaration{}
}

func affordFor(t *testing.T, data *character.Data) *session.AffordOutput {
	t.Helper()
	mgr, _, _, _ := aFight(t, data, []int{1, 1})
	out, err := mgr.Afford(context.Background(), &session.AffordInput{
		Session: "sess", Member: data.ID,
	})
	require.NoError(t, err)
	return out
}

// THE POINT OF THE SLICE: a barbarian's whole level-1 surface reaches the wire.
func TestABarbariansWholeActivatableSurfaceIsOffered(t *testing.T) {
	out := affordFor(t, ragingBarbarian("alice", 2))

	offered := make([]string, 0)
	for _, d := range activations(out.Declarations) {
		offered = append(offered, d.Ability.Ref)
	}

	require.ElementsMatch(t, []string{
		"dnd5e:combat_abilities:dash",
		"dnd5e:combat_abilities:disengage",
		"dnd5e:combat_abilities:dodge",
		"dnd5e:combat_abilities:help",
		"dnd5e:combat_abilities:hide",
		"dnd5e:features:rage",
	}, offered)
}

// Attack is a combat ability every character carries, and it must NOT appear as
// an activation: swinging is VerbAttack at this seam, and two buttons for one
// thing is how a player ends up spending an action to bank swings the seam
// already banked.
func TestTheAttackAbilityIsNotOfferedAsAnActivation(t *testing.T) {
	out := affordFor(t, ragingBarbarian("alice", 2))

	for _, d := range activations(out.Declarations) {
		require.NotEqual(t, refs.CombatAbilities.Attack().String(), d.Ability.Ref)
	}
	// ...and Attack is still offered as itself.
	require.NotEmpty(t, requireSingleDeclaration(t, out.Declarations, session.VerbAttack).ID)
}

// ONE VERB, TWO SHAPES, LIVE AT THE SAME MOMENT — which is what forces
// multiple rows per verb rather than making it a preference. Rage draws the
// bonus action while Dodge draws the action, and a single row per verb could
// only carry one of the two Slots.
func TestOneVerbCarriesBothShapesAtOnce(t *testing.T) {
	out := affordFor(t, ragingBarbarian("alice", 2))

	rage := activationFor(t, out.Declarations, "dnd5e:features:rage")
	dodge := activationFor(t, out.Declarations, "dnd5e:combat_abilities:dodge")

	require.Equal(t, session.SlotBonus, rage.Slot)
	require.Equal(t, session.SlotAction, dodge.Slot)
	require.True(t, rage.Available)
	require.True(t, dodge.Available)
}

// Every offer gets its own selector, or execution could not tell them apart —
// and the ref is the only thing that differs between two activations by the
// same member on the same turn.
func TestEveryActivationCarriesItsOwnSelector(t *testing.T) {
	out := affordFor(t, ragingBarbarian("alice", 2))

	seen := map[string]string{}
	for _, d := range activations(out.Declarations) {
		require.NotEmpty(t, d.ID, "%s has no selector", d.Ability.Ref)
		prior, clash := seen[d.ID]
		require.False(t, clash, "%s and %s share a selector", prior, d.Ability.Ref)
		seen[d.ID] = d.Ability.Ref
	}
	require.Len(t, seen, 6)
}

// Help prompts for somebody; the other five prompt for nobody. The rulebook
// draws a further line between "targets the actor" and "targets nothing" that
// this seam deliberately collapses — both mean DO NOT PROMPT.
func TestOnlyHelpAsksForATarget(t *testing.T) {
	out := affordFor(t, ragingBarbarian("alice", 2))

	for _, d := range activations(out.Declarations) {
		want := session.TargetNone
		if d.Ability.Ref == "dnd5e:combat_abilities:help" {
			want = session.TargetMember
		}
		require.Equal(t, want, d.TargetKind, "target kind for %s", d.Ability.Ref)
	}
}

// A CHARGE THAT RAN OUT IS A LEDGER, JUST NOT ONE OF THE TURN'S THREE. It is
// NoBudget with CurrencyCharges — not Unavailable — because a client should
// tell the player "come back after a rest", which is a different sentence from
// "this will never light while you are raging".
func TestNoChargesIsABudgetRefusalInACurrencyOfItsOwn(t *testing.T) {
	out := affordFor(t, ragingBarbarian("alice", 0))

	rage := activationFor(t, out.Declarations, "dnd5e:features:rage")

	require.False(t, rage.Available)
	require.NotNil(t, rage.Why)
	require.Equal(t, session.ShortfallNoBudget, rage.Why.Reason)
	require.Equal(t, session.CurrencyCharges, rage.Why.Currency)
	require.Equal(t, 1, rage.Why.Needed)
	require.Equal(t, 0, rage.Why.Left)
	// THE PROSE SURVIVES because the structure deliberately does not name WHICH
	// resource ran out — this seam does not enumerate the rulebook's resource
	// keys — so the ability's own words carry the one fact a client cannot
	// reconstruct from reason/currency/needed/left.
	require.Contains(t, rage.Why.Text, "rage")
}

// A refused ability still gets its ROW, with a selector, the way a
// budget-blocked Attack keeps its candidates. A menu that changed size as the
// turn went on would be a worse client experience than a dimmed button.
func TestARefusedActivationKeepsItsRow(t *testing.T) {
	out := affordFor(t, ragingBarbarian("alice", 0))

	require.Len(t, activations(out.Declarations), 6, "still six, one of them dark")
	rage := activationFor(t, out.Declarations, "dnd5e:features:rage")
	require.NotEmpty(t, rage.ID, "a compiled-but-refused offer still carries its selector")
	require.NotNil(t, rage.Ability, "and still says what it is")
	require.Equal(t, "Rage", rage.Ability.Name)
}

// --- Help's candidate universe (rpg-project#300, Kirk's ruling) ---

func helpOffer(t *testing.T, mgr *session.Manager, member string) session.Declaration {
	t.Helper()
	out, err := mgr.Afford(context.Background(), &session.AffordInput{
		Session: "sess", Member: member,
	})
	require.NoError(t, err)
	for _, d := range out.Declarations {
		if d.Verb == session.VerbActivate &&
			d.Ability != nil && d.Ability.Ref == "dnd5e:combat_abilities:help" {
			return d
		}
	}
	t.Fatal("no Help offer")
	return session.Declaration{}
}

// AN ALLY STANDING NEXT TO YOU IS SOMEBODY YOU CAN HELP. Kirk's ruling: "I
// think that ally next to us is fine. we can add complexity later if we want."
func TestHelpOffersTheAllyStandingNextToYou(t *testing.T) {
	alice, bob := ragingBarbarian("alice", 2), ragingBarbarian("bob", 2)
	mgr, _ := aTwoPlayerFightAt(t, alice, spatial.Position{X: 1, Y: 1},
		bob, spatial.Position{X: 2, Y: 1})

	help := helpOffer(t, mgr, "alice")

	require.True(t, help.Available)
	require.Len(t, help.Candidates, 1)
	require.Equal(t, "bob", help.Candidates[0].Member)
	require.True(t, help.Candidates[0].Available)
}

// An ally across the room KEEPS HIS ROW and says why, the way Attack's
// candidates do — the panel shows who is there and why they cannot be helped,
// rather than a list that changes length as people move.
func TestHelpKeepsAnOutOfReachAllyAsARowWithAReason(t *testing.T) {
	alice, bob := ragingBarbarian("alice", 2), ragingBarbarian("bob", 2)
	mgr, _ := aTwoPlayerFightAt(t, alice, spatial.Position{X: 1, Y: 1},
		bob, spatial.Position{X: 5, Y: 5})

	help := helpOffer(t, mgr, "alice")

	require.Len(t, help.Candidates, 1)
	require.Equal(t, "bob", help.Candidates[0].Member)
	require.False(t, help.Candidates[0].Available)
	require.NotNil(t, help.Candidates[0].Why)
	require.Equal(t, session.ShortfallTargetOutOfReach, help.Candidates[0].Why.Reason)

	// And the declaration itself goes dark with the declaration-level reason —
	// the same precedence Attack keeps.
	require.False(t, help.Available)
	require.NotNil(t, help.Why)
	require.Equal(t, session.ShortfallNoTargetInReach, help.Why.Reason)
}

// A MONSTER IS NOT AN ALLY. Same-kind is the whole of hostility this seam has,
// and it is what keeps a barbarian from offering to steady a skeleton.
func TestHelpDoesNotOfferAMonsterAsAnAlly(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, _ := aFight(t, alice, []int{1, 1})

	help := helpOffer(t, mgr, "alice")

	// The skeleton spawned adjacent to her is in reach and in sight, and is
	// still not a candidate.
	require.Empty(t, help.Candidates, "a monster is never a Help candidate")
	require.False(t, help.Available)
	require.Equal(t, session.ShortfallNoTargetInReach, help.Why.Reason)
}

// Help still carries its selector when it is dark, like every other refused
// offer — so a client renders a disabled button rather than dropping the row.
func TestARefusedHelpKeepsItsSelector(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	mgr, _, _, _ := aFight(t, alice, []int{1, 1})

	help := helpOffer(t, mgr, "alice")

	require.False(t, help.Available)
	require.NotEmpty(t, help.ID)
	require.Equal(t, "Help", help.Ability.Name)
}
