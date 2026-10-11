// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// probeRef is the ref a hand-built descriptor names in these tests.
func probeRef() *core.Ref {
	return &core.Ref{Module: refs.Module, Type: refs.TypeConditions, ID: "probe"}
}

// validProbe is a well-formed descriptor that bears on nothing.
func validProbe() descriptor {
	return descriptor{
		ref:     probeRef(),
		display: Display{Name: "Probe"},
		load:    loaderOf[ProneCondition](),
	}
}

func TestNewRegistryRefusesAMalformedList(t *testing.T) {
	answering := func() descriptor {
		entry := validProbe()
		entry.ownAction = answersNow
		entry.display.Detail = "Does something."
		return entry
	}

	cases := map[string]func() []descriptor{
		"a nil ref": func() []descriptor {
			entry := validProbe()
			entry.ref = nil
			return []descriptor{entry}
		},
		"a duplicate ref": func() []descriptor {
			return []descriptor{validProbe(), validProbe()}
		},
		"a nil load": func() []descriptor {
			entry := validProbe()
			entry.load = nil
			return []descriptor{entry}
		},
		"an empty display name": func() []descriptor {
			entry := validProbe()
			entry.display.Name = ""
			return []descriptor{entry}
		},
		"answersNow with an empty detail": func() []descriptor {
			entry := answering()
			entry.display.Detail = ""
			return []descriptor{entry}
		},
		"answering againstHolder with no heldRule": func() []descriptor {
			entry := answering()
			entry.againstHolder = answersNow
			return []descriptor{entry}
		},
		"a heldRule on notBearing": func() []descriptor {
			entry := validProbe()
			entry.heldRule = newProneHeldRule
			return []descriptor{entry}
		},
	}
	for name, list := range cases {
		t.Run(name, func(t *testing.T) {
			keyed, err := newRegistry(list())
			require.Error(t, err)
			require.Nil(t, keyed)
		})
	}

	t.Run("a well-formed list builds", func(t *testing.T) {
		entry := answering()
		entry.againstHolder = answersNow
		entry.heldRule = newProneHeldRule
		keyed, err := newRegistry([]descriptor{entry})
		require.NoError(t, err)
		require.Contains(t, keyed, probeRef().String())
	})

	t.Run("the real descriptors build", func(t *testing.T) {
		keyed, err := newRegistry(descriptors)
		require.NoError(t, err)
		require.Len(t, keyed, len(descriptors), "no two descriptors share a ref")
	})

	t.Run("mustRegistry panics with the refusal", func(t *testing.T) {
		require.Panics(t, func() { mustRegistry([]descriptor{{}}) })
	})
}

// unregisteredCondition is a condition that names a ref nothing registers.
type unregisteredCondition struct{}

func (unregisteredCondition) Ref() *core.Ref {
	return &core.Ref{Module: refs.Module, Type: refs.TypeConditions, ID: "nope"}
}
func (unregisteredCondition) IsApplied() bool                               { return false }
func (unregisteredCondition) Apply(context.Context, events.EventBus) error  { return nil }
func (unregisteredCondition) Remove(context.Context, events.EventBus) error { return nil }
func (unregisteredCondition) ToJSON() (json.RawMessage, error) {
	return json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"nope"}}`), nil
}

func TestAnUnregisteredRefIsAnErrorEverywhere(t *testing.T) {
	blob := json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"nope"},"member_id":"gob"}`)
	fake := unregisteredCondition{}
	nope := fake.Ref().String()

	t.Run("LoadJSON", func(t *testing.T) {
		_, err := LoadJSON(blob)
		require.ErrorIs(t, err, ErrUnknownCondition)
		require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
	})
	t.Run("CreateFromRef", func(t *testing.T) {
		_, err := CreateFromRef(&CreateFromRefInput{Ref: nope, MemberID: "gob"})
		require.ErrorIs(t, err, ErrUnknownCondition)
	})
	t.Run("HeldAddresses", func(t *testing.T) {
		held, err := HeldAddresses("gob", []json.RawMessage{blob})
		require.ErrorIs(t, err, ErrUnknownCondition)
		require.Nil(t, held)
	})
	t.Run("WeaponAttackOverride", func(t *testing.T) {
		_, err := WeaponAttackOverride(&WeaponAttackOverrideInput{
			Condition: fake, Swing: &weaponattack.OverrideInput{Slot: "main_hand"},
		})
		require.ErrorIs(t, err, ErrUnknownCondition)
	})
	t.Run("AssessActionEffects", func(t *testing.T) {
		_, err := AssessActionEffects(&AssessActionEffectsInput{
			Conditions: []dnd5eEvents.ConditionBehavior{fake}, Frame: rogueFrame(false),
		})
		require.ErrorIs(t, err, ErrUnknownCondition)
	})
	t.Run("AssessTargetHeldEffects", func(t *testing.T) {
		_, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{
			Frame: gobFrame(contributions.HeldCondition{Ref: nope}),
		})
		require.ErrorIs(t, err, ErrUnknownCondition)
	})
	t.Run("ExecuteHeldEffect", func(t *testing.T) {
		held := contributions.HeldCondition{Ref: nope}
		_, err := ExecuteHeldEffect(&ExecuteHeldEffectInput{Holder: "gob", Held: held, Frame: gobFrame(held)})
		require.ErrorIs(t, err, ErrUnknownCondition)
	})
}

// A known id under another type is another ref: the registry is keyed by the
// whole string, so nothing routes by id alone.
func TestAKnownIDUnderAnotherTypeIsUnknown(t *testing.T) {
	_, err := CreateFromRef(&CreateFromRefInput{Ref: "dnd5e:features:raging", MemberID: "barb"})
	require.ErrorIs(t, err, ErrUnknownCondition)

	_, err = LoadJSON(json.RawMessage(`{"ref":{"module":"dnd5e","type":"features","id":"sneak_attack"}}`))
	require.ErrorIs(t, err, ErrUnknownCondition)

	_, err = CreateFromRef(&CreateFromRefInput{Ref: "other:conditions:raging", MemberID: "barb"})
	require.ErrorIs(t, err, ErrUnknownCondition, "a known id under another module is unknown too")
}

func TestATypeWithNoCreateIsNotCreatable(t *testing.T) {
	for _, ref := range []*core.Ref{refs.Conditions.OpportunityAttack(), refs.MonsterTraits.Immunity()} {
		_, err := CreateFromRef(&CreateFromRefInput{Ref: ref.String(), MemberID: "gob"})
		require.ErrorIs(t, err, ErrNotCreatable, ref.String())
		require.NotErrorIs(t, err, ErrUnknownCondition, "registered, so not unknown")
	}
}

func TestBlessAndBaneAreCreatedFromARef(t *testing.T) {
	for name, tc := range map[string]struct{ condition, spell *core.Ref }{
		"bless": {refs.Conditions.Blessed(), refs.Spells.Bless()},
		"bane":  {refs.Conditions.Baned(), refs.Spells.Bane()},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := CreateFromRef(&CreateFromRefInput{
				Ref: tc.condition.String(), MemberID: "gob",
				Config: json.RawMessage(`{"source_id":"cleric"}`), SourceRef: tc.spell.String(),
			})
			require.NoError(t, err)
			require.Equal(t, tc.condition.String(), out.Condition.Ref().String())
			address := ConditionAddressOf("gob", out.Condition)
			require.Equal(t, "cleric", address.SourceID)
			require.Equal(t, "gob", address.MemberID)
		})
	}

	t.Run("a wrong source spell is refused", func(t *testing.T) {
		_, err := CreateFromRef(&CreateFromRefInput{
			Ref: refs.Conditions.Blessed().String(), MemberID: "gob",
			Config: json.RawMessage(`{"source_id":"cleric"}`), SourceRef: refs.Spells.Bane().String(),
		})
		require.Error(t, err)
	})
}

func TestRetiredKindsAreDropped(t *testing.T) {
	for name, blob := range map[string]json.RawMessage{
		"unarmored movement": json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"unarmored_movement"},"member_id":"m1","monk_level":3}`),
		"shield spell":       json.RawMessage(`{"ref":{"module":"dnd5e","type":"spells","id":"shield"},"member_id":"m1"}`),
	} {
		t.Run(name, func(t *testing.T) {
			require.True(t, IsRetired(blob))
			_, err := LoadJSON(blob)
			require.ErrorIs(t, err, ErrRetiredCondition)
			require.NotErrorIs(t, err, ErrUnknownCondition, "retired is not unregistered")
		})
	}
}

func TestWeaponAttackOverrideAsksTheEntry(t *testing.T) {
	t.Run("martial arts offers its die on an empty hand", func(t *testing.T) {
		out, err := WeaponAttackOverride(&WeaponAttackOverrideInput{
			Condition: NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1"}),
			Swing:     &weaponattack.OverrideInput{Slot: "main_hand", Levels: monkLevels(1)},
		})
		require.NoError(t, err)
		require.Equal(t, &weaponattack.Override{Dice: "1d4", Ability: abilities.DEX}, out.Override)
	})

	t.Run("raging makes no offer", func(t *testing.T) {
		out, err := WeaponAttackOverride(&WeaponAttackOverrideInput{
			Condition: &RagingCondition{CharacterID: "barb-1"},
			Swing:     &weaponattack.OverrideInput{Slot: "main_hand", Levels: monkLevels(1)},
		})
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Nil(t, out.Override)
	})

	t.Run("shillelagh offers through its entry", func(t *testing.T) {
		enchanted, err := NewShillelaghCondition("druid-1", testShillelaghConfig())
		require.NoError(t, err)
		out, err := WeaponAttackOverride(&WeaponAttackOverrideInput{
			Condition: enchanted,
			Swing:     &weaponattack.OverrideInput{Slot: "main_hand", ItemID: "club"},
		})
		require.NoError(t, err)
		require.NotNil(t, out.Override)
	})

	t.Run("a nil input, condition or ref is refused", func(t *testing.T) {
		for name, in := range map[string]*WeaponAttackOverrideInput{
			"nil input":     nil,
			"nil condition": {Swing: &weaponattack.OverrideInput{}},
			"nil ref":       {Condition: &namelessCondition{}, Swing: &weaponattack.OverrideInput{}},
		} {
			_, err := WeaponAttackOverride(in)
			require.Error(t, err, name)
			require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err), name)
		}
	})
}

// namelessCondition is a condition that cannot name itself.
type namelessCondition struct{ unregisteredCondition }

func (namelessCondition) Ref() *core.Ref { return nil }

func TestEveryFightingStyleNamesARegisteredCondition(t *testing.T) {
	registered := RegisteredRefs()
	for _, style := range fightingstyles.All() {
		ref, ok := fightingstyles.ConditionRef(style)
		require.True(t, ok, style)
		require.Contains(t, registered, ref.String(), style)
	}

	_, ok := fightingstyles.ConditionRef(fightingstyles.Unspecified)
	require.False(t, ok, "no style chosen names no condition")
	_, ok = fightingstyles.ConditionRef("not_a_style")
	require.False(t, ok)
}

func TestUnarmoredDefenseRefusesAMissingOrUnknownVariant(t *testing.T) {
	for name, config := range map[string]json.RawMessage{
		"no config":      nil,
		"empty config":   json.RawMessage(`{}`),
		"unknown":        json.RawMessage(`{"variant":"druid"}`),
		"wrong case":     json.RawMessage(`{"variant":"Monk"}`),
		"a non-variant":  json.RawMessage(`{"variant":""}`),
		"another's name": json.RawMessage(`{"type":"monk"}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := CreateFromRef(&CreateFromRefInput{
				Ref: refs.Conditions.UnarmoredDefense().String(), MemberID: "m1", Config: config,
				SourceRef: "dnd5e:classes:monk",
			})
			require.Error(t, err)
			require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
		})
	}

	_, err := CreateFromRef(&CreateFromRefInput{
		Ref: refs.Conditions.UnarmoredDefense().String(), MemberID: "m1", Config: json.RawMessage(`{"variant":`),
	})
	require.Error(t, err, "an unreadable config is refused too")

	for variant, want := range map[string]UnarmoredDefenseType{
		"barbarian": UnarmoredDefenseBarbarian, "monk": UnarmoredDefenseMonk,
	} {
		out, err := CreateFromRef(&CreateFromRefInput{
			Ref: refs.Conditions.UnarmoredDefense().String(), MemberID: "m1",
			Config: json.RawMessage(`{"variant":"` + variant + `"}`),
		})
		require.NoError(t, err, variant)
		unarmored, ok := out.Condition.(*UnarmoredDefenseCondition)
		require.True(t, ok)
		require.Equal(t, want, unarmored.Type)
	}
}

// TestSneakAttackHasOneRef: the rogue grant, the condition's own name, its
// persisted form and its damage line all name the one condition ref.
func TestSneakAttackHasOneRef(t *testing.T) {
	want := refs.Conditions.SneakAttack().String()

	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue-1"})
	require.Equal(t, want, sneak.Ref().String())

	blob, err := sneak.ToJSON()
	require.NoError(t, err)
	var stored struct {
		Ref core.Ref `json:"ref"`
	}
	require.NoError(t, json.Unmarshal(blob, &stored))
	require.Equal(t, want, stored.Ref.String())

	out, err := CreateFromRef(&CreateFromRefInput{Ref: want, MemberID: "rogue-1"})
	require.NoError(t, err)
	require.Equal(t, want, out.Condition.Ref().String())
}

func TestFreeReactionRefsAreTheFreeReactions(t *testing.T) {
	want := make([]string, 0)
	for _, reaction := range FreeReactions("m1") {
		want = append(want, reaction.Ref().String())
	}
	require.Equal(t, want, FreeReactionRefs())
	require.Contains(t, FreeReactionRefs(), refs.Conditions.OpportunityAttack().String())
}

func TestRegisteredRefsAreSortedAndEveryDescriptorIsListed(t *testing.T) {
	listed := RegisteredRefs()
	require.IsIncreasing(t, listed)
	require.Len(t, listed, len(descriptors))
	for _, entry := range descriptors {
		require.Contains(t, listed, entry.ref.String())
	}
}
