// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// goldenAttackDefinition mirrors the validated fixture in the actions package
// (rulebooks/dnd5e/combat/actions/definition_test.go) so the Attack selector
// golden pins the canonical form of a real, complete Definition — not a
// hand-authored JSON string.
func goldenAttackDefinition() combatActions.Definition {
	profile := combatActions.AttackProfile{
		Category: combatActions.AttackCategoryWeapon,
		Delivery: combatActions.AttackDelivery{
			Melee: &combatActions.MeleeDelivery{ReachFeet: 5},
		},
		AttackBonus: 5,
		Ability: &combatActions.AbilityContribution{
			Ability:  abilities.STR,
			Modifier: 3,
		},
		Weapon: &combatActions.WeaponContext{Ref: refs.Weapons.Longsword()},
		Damage: []damage.Damage{{
			Dice:       "1d8",
			Type:       damage.Slashing,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		}},
	}
	return combatActions.Definition{
		Ref:    *refs.Weapons.Longsword(),
		Name:   "Longsword",
		Cost:   &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1}},
		Attack: &profile,
	}
}

func TestMoveDeclarationIDGolden(t *testing.T) {
	got, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbMove, Slot: SlotNone,
	})
	require.NoError(t, err)
	require.Equal(t, "v1.Mhnl9aRJjeAvMxtlbRFFHVH-XoMkgR4l1pasCSyzrjc", got)
}

func TestEndTurnDeclarationIDGolden(t *testing.T) {
	got, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbEndTurn, Slot: SlotNone,
	})
	require.NoError(t, err)
	require.Equal(t, "v1.yZ1FHWnV7SnulpNVV4kz1BSkslDxfXr-lEINSahGUDo", got)
}

func TestAttackDeclarationIDGolden(t *testing.T) {
	def := goldenAttackDefinition()
	require.NoError(t, def.Validate())

	got, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction, Attack: &def,
	})
	require.NoError(t, err)
	require.Equal(t, "v1.ZmGXGvVkJHmrxzEhR7xYqlIEkrB6xRsawMp6l4UX6fM", got)
}

func TestDeclarationIDMapInsertionOrderIsCanonical(t *testing.T) {
	base := goldenAttackDefinition()

	// Two profiles with the same multi-key SpendProfile content, written with
	// different map-literal key order. RFC 8785 canonicalization fixes key
	// order, so the selector IDs must agree regardless of how the maps were
	// assembled.
	defA := base
	defA.Cost = &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{
			combat.CapacityAttack:   1,
			combat.CapacityMovement: 5,
		},
	}
	defB := base
	defB.Cost = &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{
			combat.CapacityMovement: 5,
			combat.CapacityAttack:   1,
		},
	}

	idA, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction, Attack: &defA,
	})
	require.NoError(t, err)
	idB, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction, Attack: &defB,
	})
	require.NoError(t, err)
	require.Equal(t, idA, idB, "map insertion order must not affect the selector ID")
}

func TestDeclarationIDNilVsEmptySpendProfileNormalization(t *testing.T) {
	base := goldenAttackDefinition()

	// nil Cost: the "cost" key is omitted entirely by omitempty.
	nilCost := base
	nilCost.Cost = nil

	// Non-nil empty SpendProfile: all internal maps are nil, so omitempty
	// leaves "cost": {} — distinct from a nil cost.
	emptyCost := base
	emptyCost.Cost = &combat.SpendProfile{}

	// Non-nil SpendProfile with empty (non-nil) maps: omitempty omits empty
	// maps, so this canonicalizes to "cost": {} too — the SAME selector
	// material as the all-nil-maps profile above.
	emptyMapCost := base
	emptyMapCost.Cost = &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{},
	}

	in := declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction,
	}

	idNil, err := declarationID(withAttack(in, &nilCost))
	require.NoError(t, err)
	idEmpty, err := declarationID(withAttack(in, &emptyCost))
	require.NoError(t, err)
	idEmptyMap, err := declarationID(withAttack(in, &emptyMapCost))
	require.NoError(t, err)

	require.NotEqual(t, idNil, idEmpty,
		"a nil cost and a non-nil empty SpendProfile are distinct selector material")
	require.Equal(t, idEmpty, idEmptyMap,
		"nil and empty maps within a non-nil SpendProfile normalize to the same selector material")
}

func TestDeclarationIDEmbeddedRawJSONCanonicalization(t *testing.T) {
	base := goldenAttackDefinition()

	// Two equivalent condition-parameter blobs written with different key
	// order. The canonicalizer normalizes object key order, so the IDs agree.
	// Each variant is deep-copied from the base fixture so the shared Attack
	// pointer is not aliased across the two mutations.
	defA := base.Clone()
	defA.Attack.OnHit = []combatActions.ConditionApplication{{
		Ref:        *refs.Conditions.Prone(),
		Parameters: json.RawMessage(`{"duration":1,"level":2}`),
		Save:       saves.NewSaveGate(abilities.STR, 11),
	}}
	defB := base.Clone()
	defB.Attack.OnHit = []combatActions.ConditionApplication{{
		Ref:        *refs.Conditions.Prone(),
		Parameters: json.RawMessage(`{"level":2,"duration":1}`),
		Save:       saves.NewSaveGate(abilities.STR, 11),
	}}

	in := declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction,
	}
	idA, err := declarationID(withAttack(in, &defA))
	require.NoError(t, err)
	idB, err := declarationID(withAttack(in, &defB))
	require.NoError(t, err)
	require.Equal(t, idA, idB,
		"embedded raw JSON with different key order must canonicalize to the same selector ID")
}

func TestDeclarationIDChangedProfileChangesID(t *testing.T) {
	base := goldenAttackDefinition()

	// Deep-copy so the reach mutation does not alias the base Attack pointer.
	changed := base.Clone()
	changed.Attack.Delivery.Melee.ReachFeet = 10

	in := declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction,
	}
	idBase, err := declarationID(withAttack(in, &base))
	require.NoError(t, err)
	idChanged, err := declarationID(withAttack(in, &changed))
	require.NoError(t, err)
	require.NotEqual(t, idBase, idChanged,
		"a changed attack profile must produce a different selector ID")
}

func TestDeclarationIDSameStateRecurrence(t *testing.T) {
	in := declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction, Attack: ptrDefinition(goldenAttackDefinition()),
	}
	first, err := declarationID(in)
	require.NoError(t, err)
	second, err := declarationID(in)
	require.NoError(t, err)
	require.Equal(t, first, second,
		"the same selector input must receive the same ID when state recurs")
}

func TestDeclarationIDFullDigestPayload(t *testing.T) {
	got, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbMove, Slot: SlotNone,
	})
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(got, "v1."),
		"selector ID must carry the v1. prefix; got %q", got)
	digest := strings.TrimPrefix(got, "v1.")
	require.Len(t, digest, 43,
		"the base64url digest payload must be the full 43-character SHA-256 encoding; got %q", digest)
	require.NotContains(t, digest, "=",
		"the digest payload must be unpadded base64url; got %q", digest)
}

func TestDeclarationIDRejectsUnsupportedVerb(t *testing.T) {
	_, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: Verb("fly"), Slot: SlotNone,
	})
	require.Error(t, err)
}

func TestDeclarationIDRejectsUnsupportedSlot(t *testing.T) {
	_, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbMove, Slot: Slot("legendary"),
	})
	require.Error(t, err)
}

func TestDeclarationIDRejectsAttackWithoutDefinition(t *testing.T) {
	_, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction,
	})
	require.Error(t, err)
}

func TestDeclarationIDRejectsMoveWithAttackDefinition(t *testing.T) {
	def := goldenAttackDefinition()
	_, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbMove, Slot: SlotNone, Attack: &def,
	})
	require.Error(t, err)
}

func TestDeclarationIDRejectsUnvalidatedAttackDefinition(t *testing.T) {
	def := goldenAttackDefinition()
	def.Attack = nil // invalid: exactly one profile required
	_, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction, Attack: &def,
	})
	require.Error(t, err)
}

func TestDeclarationIDRejectsMalformedEmbeddedRawJSON(t *testing.T) {
	def := goldenAttackDefinition()
	def.Attack.OnHit = []combatActions.ConditionApplication{{
		Ref:        *refs.Conditions.Prone(),
		Parameters: json.RawMessage(`{"duration":1,`), // malformed
		Save:       saves.NewSaveGate(abilities.STR, 11),
	}}
	// Definition.Validate rejects malformed Parameters before serialization.
	_, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction, Attack: &def,
	})
	require.Error(t, err)
}

func TestDeclarationIDRejectsNonRFCNumber(t *testing.T) {
	// A SpendProfile amount is an int and always RFC 8785-interoperable, so the
	// canonicalizer itself is the guard that fails on a non-interoperable
	// number. We exercise it by feeding a variant whose embedded raw JSON
	// carries a number outside RFC 8785's exact integer range; the
	// canonicalizer must reject it rather than approximate.
	def := goldenAttackDefinition()
	def.Attack.OnHit = []combatActions.ConditionApplication{{
		Ref:        *refs.Conditions.Prone(),
		Parameters: json.RawMessage(`{"overflow":1e999}`),
		Save:       saves.NewSaveGate(abilities.STR, 11),
	}}
	// json.Valid accepts 1e999, so Definition.Validate does not catch it; the
	// canonicalizer is the guard that fails closed.
	require.True(t, json.Valid(def.Attack.OnHit[0].Parameters))

	_, err := declarationID(declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Slot: SlotAction, Attack: &def,
	})
	require.Error(t, err, "a number outside RFC 8785's exact range must fail rather than approximate")
}

func TestDeclarationIDSlotsDistinct(t *testing.T) {
	def := goldenAttackDefinition()
	in := declarationIDInput{
		Session: "session-1", Member: "fighter-1",
		Verb: VerbAttack, Attack: &def,
	}
	idAction, err := declarationID(withSlot(in, SlotAction))
	require.NoError(t, err)
	idBonus, err := declarationID(withSlot(in, SlotBonus))
	require.NoError(t, err)
	idNone, err := declarationID(withSlot(in, SlotNone))
	require.NoError(t, err)
	require.NotEqual(t, idAction, idBonus)
	require.NotEqual(t, idAction, idNone)
	require.NotEqual(t, idBonus, idNone)
}

// testOffer is the minimal identity the collision index carries in these tests.
// Task 6 will substitute its own compiledOffer; here we only prove the
// fail-closed primitive over an injected ID function.
type testOffer struct {
	name string
}

func TestCompiledOfferCollisionDuplicateIDFailClosed(t *testing.T) {
	equalByName := func(a, b testOffer) bool { return a.name == b.name }

	t.Run("non-identical offers with the same ID fail closed", func(t *testing.T) {
		// Injected ID function forces a collision: every offer maps to the
		// same selector ID.
		idx := newIndexCompiledOffers(
			func(testOffer) (string, error) { return "v1.colliding", nil },
			equalByName,
		)

		require.NoError(t, idx.add(testOffer{name: "alpha"}))
		err := idx.add(testOffer{name: "beta"})
		require.ErrorIs(t, err, errDeclarationIDCollision)
	})

	t.Run("identical offers with the same ID are recurrence, not collision", func(t *testing.T) {
		idx := newIndexCompiledOffers(
			func(testOffer) (string, error) { return "v1.recurring", nil },
			equalByName,
		)

		require.NoError(t, idx.add(testOffer{name: "alpha"}))
		require.NoError(t, idx.add(testOffer{name: "alpha"}))
	})

	t.Run("RFC8785-equivalent map order is recurrence, not collision", func(t *testing.T) {
		left := goldenAttackDefinition()
		left.Attack.OnHit = []combatActions.ConditionApplication{{
			Ref: *refs.Conditions.Prone(), Parameters: json.RawMessage(`{"first":1,"second":2}`),
			Save: saves.NewSaveGate(abilities.STR, 11),
		}}
		right := left.Clone()
		right.Attack.OnHit[0].Parameters = json.RawMessage(`{"second":2,"first":1}`)

		leftID, err := declarationID(declarationIDInput{
			Session: "session-1", Member: "fighter-1", Verb: VerbAttack, Slot: SlotAction, Attack: &left,
		})
		require.NoError(t, err)
		rightID, err := declarationID(declarationIDInput{
			Session: "session-1", Member: "fighter-1", Verb: VerbAttack, Slot: SlotAction, Attack: &right,
		})
		require.NoError(t, err)
		require.Equal(t, leftID, rightID, "RFC8785 canonical selectors are equal")
		leftVariant, err := selectorVariant(VerbAttack, &left, "")
		require.NoError(t, err)
		rightVariant, err := selectorVariant(VerbAttack, &right, "")
		require.NoError(t, err)
		require.NotEqual(t, string(leftVariant), string(rightVariant),
			"the fixture must reach equality with different raw object order")

		offers := []compiledOffer{
			{declaration: Declaration{ID: leftID}, verb: VerbAttack, slot: SlotAction, variant: leftVariant},
			{declaration: Declaration{ID: rightID}, verb: VerbAttack, slot: SlotAction, variant: rightVariant},
		}
		require.NoError(t, guardOfferCollisions(offers),
			"raw JSON object order must not turn equivalent offers into a collision")
	})

	t.Run("ID function error propagates", func(t *testing.T) {
		idx := newIndexCompiledOffers(
			func(testOffer) (string, error) { return "", errInjectedID },
			equalByName,
		)
		err := idx.add(testOffer{name: "alpha"})
		require.ErrorIs(t, err, errInjectedID)
	})

	t.Run("real declarationID distinguishes distinct Move offers", func(t *testing.T) {
		// Integration: the injected ID function is the real declarationID over
		// a Move offer, proving the primitive composes with the selector.
		idFor := func(o testOffer) (string, error) {
			return declarationID(declarationIDInput{
				Session: "session-1", Member: o.name,
				Verb: VerbMove, Slot: SlotNone,
			})
		}
		idx := newIndexCompiledOffers(idFor, equalByName)

		require.NoError(t, idx.add(testOffer{name: "fighter-1"}))
		require.NoError(t, idx.add(testOffer{name: "rogue-2"}))
		// A second distinct member colliding is impossible here because the
		// IDs differ; this just exercises the happy path through the real
		// selector.
	})
}

// withAttack returns a copy of in carrying the given attack definition, so the
// table-style tests above can vary one field without repeating the whole input.
func withAttack(in declarationIDInput, def *combatActions.Definition) declarationIDInput {
	out := in
	out.Attack = def
	return out
}

func withSlot(in declarationIDInput, slot Slot) declarationIDInput {
	out := in
	out.Slot = slot
	return out
}

func ptrDefinition(d combatActions.Definition) *combatActions.Definition {
	return &d
}

// errInjectedID is a sentinel the collision tests inject through the ID
// function to prove errors propagate unchanged.
var errInjectedID = newInjectedIDError()

func newInjectedIDError() error { return &injectedIDError{msg: "injected id failure"} }

type injectedIDError struct{ msg string }

func (e *injectedIDError) Error() string { return e.msg }
