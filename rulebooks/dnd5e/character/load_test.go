// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type PureLoadTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *PureLoadTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// ragingBlob is a persisted Raging condition in the exact bytes its own
// serializer produces, so that a byte comparison downstream is a claim about
// the loader carrying it rather than about this test's JSON matching the
// serializer's field order.
func ragingBlob(s *suite.Suite) json.RawMessage {
	raging := &conditions.RagingCondition{
		CharacterID: "char-load",
		DamageBonus: 2,
		Level:       3,
		Source:      "rage",
	}

	raw, err := raging.ToJSON()
	s.Require().NoError(err)

	return raw
}

// rageBlob is a persisted Rage feature, canonicalized the same way.
func rageBlob(s *suite.Suite) json.RawMessage {
	seed, err := json.Marshal(features.RageData{
		Ref:   refs.Features.Rage(),
		ID:    "rage-1",
		Name:  "Rage",
		Level: 3,
	})
	s.Require().NoError(err)

	feature, err := features.LoadJSON(seed)
	s.Require().NoError(err)

	raw, err := feature.ToJSON()
	s.Require().NoError(err)

	return raw
}

// brutalCriticalBlob is a second persisted condition, canonicalized the same
// way, for the tests that need a sheet carrying more than one.
func brutalCriticalBlob(s *suite.Suite) json.RawMessage {
	brutal := &conditions.BrutalCriticalCondition{
		MemberID:  "char-load",
		Level:     9,
		ExtraDice: 1,
	}

	raw, err := brutal.ToJSON()
	s.Require().NoError(err)

	return raw
}

// fullSheet is character data with something in every field a loader is
// responsible for carrying — the ones that are only a copy, and the ones
// (inventory, features, conditions, resources) that have to be reconstituted
// and then serialized back.
func fullSheet(s *suite.Suite) *Data {
	sword, err := equipment.GetByID(weapons.Longsword)
	s.Require().NoError(err)

	return &Data{
		ID:               "char-load",
		PlayerID:         "player-load",
		Name:             "Round Tripper",
		Level:            3,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Barbarian,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    22,
		MaxHitPoints: 34,
		ArmorClass:   15,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
		},
		Languages:           []languages.Language{languages.Common, languages.Orc},
		ArmorProficiencies:  []proficiencies.Armor{proficiencies.ArmorLight},
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponMartial},
		Inventory: []InventoryItemData{
			{Type: sword.EquipmentType(), ID: sword.EquipmentID(), Quantity: 1},
		},
		Wallet:         currency.FromGold(15),
		EquipmentSlots: EquipmentSlots{SlotMainHand: string(weapons.Longsword)},
		SpellSlots:     map[int]SpellSlotData{1: {Max: 2, Used: 1}},
		ClassResources: map[shared.ClassResourceType]ResourceData{
			shared.ClassResourceRage: {Name: "Rage", Max: 3, Current: 2, Resets: shared.ResetTypeLongRest},
		},
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			resources.RageCharges: {Current: 2, Maximum: 3, ResetType: coreResources.ResetLongRest},
		},
		Features:   []json.RawMessage{rageBlob(s)},
		Conditions: []json.RawMessage{ragingBlob(s)},
		ActionEconomy: &ActionEconomyData{
			TurnNumber:            4,
			ActionsRemaining:      1,
			BonusActionsRemaining: 1,
			ReactionsRemaining:    1,
			MovementRemaining:     30,
		},
	}
}

// comparable strips UpdatedAt because ToData re-stamps it on purpose. All
// persisted character fields must otherwise survive the load unchanged.
func comparable(d *Data) *Data {
	stripped := *d
	stripped.UpdatedAt = time.Time{}

	return &stripped
}

func marshalData(s *suite.Suite, d *Data) string {
	raw, err := json.Marshal(comparable(d))
	s.Require().NoError(err)

	return string(raw)
}

// The whole point of a pure load: data in, the same data out, with no bus
// anywhere in the call. Byte-for-byte, because "equivalent" is what a sheet
// that has quietly dropped a condition also looks like (rpg-toolkit#948).
func (s *PureLoadTestSuite) TestRoundTripsByteIdenticalWithNoBus() {
	data := fullSheet(&s.Suite)

	char, err := Load(s.ctx, data)
	s.Require().NoError(err)

	s.Require().Equal(marshalData(&s.Suite, data), marshalData(&s.Suite, char.ToData()))
}

// Named separately because it is the one this migration exists to protect: the
// conditions survive a load that never saw a bus, which is what lets the attach
// loop move out of the loader.
func (s *PureLoadTestSuite) TestConditionsSurviveALoadWithNoBus() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)

	s.Require().Len(char.GetConditions(), 1)
	s.Require().Equal(ragingBlob(&s.Suite), char.ToData().Conditions[0])
}

// A loaded sheet is inert. Nothing is applied, nothing is subscribed, and no
// bus is held — the conditions are parsed and waiting, not running.
func (s *PureLoadTestSuite) TestLoadAppliesNothing() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)

	s.Require().Nil(char.bus, "a pure load holds no bus")
	s.Require().Empty(char.subscriptionIDs, "a pure load subscribes to nothing")

	for _, cond := range char.GetConditions() {
		s.Require().False(cond.IsApplied(), "a pure load applies no condition")
	}
	for key, resource := range char.resources {
		s.Require().False(resource.IsApplied(), "a pure load applies no resource: %s", key)
	}
}

// The strict path refuses a condition blob it cannot read, and the error says
// which blob. Refusing is the whole divergence: a sheet that comes back short
// one condition looks exactly like a sheet that never had it.
func (s *PureLoadTestSuite) TestStrictLoadRefusesAMalformedCondition() {
	data := fullSheet(&s.Suite)
	data.Conditions = append(data.Conditions, json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"nope"}}`))

	_, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), `"id":"nope"`, "the error names the blob that failed")
}

// The same blob on the legacy path loads, and the condition is gone. This is
// not a bug being pinned as correct — it is the behaviour ~20 callers have,
// asserted so that the divergence is a decision rather than a surprise.
func (s *PureLoadTestSuite) TestLegacyLoadDropsAMalformedCondition() {
	data := fullSheet(&s.Suite)
	data.Conditions = append(data.Conditions, json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"nope"}}`))

	char, err := LoadFromData(s.ctx, data, events.NewEventBus())

	s.Require().NoError(err)
	s.Require().Len(char.GetConditions(), 1, "the unreadable condition is silently dropped")
}

// Features are the same species of loss as conditions, and get the same
// treatment: a blob with no loader here fails the strict load.
func (s *PureLoadTestSuite) TestStrictLoadRefusesAFeatureFromAnotherModule() {
	data := fullSheet(&s.Suite)
	data.Features = append(data.Features, json.RawMessage(`{"ref":{"module":"artificer","type":"features","id":"infusion"}}`))

	_, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "artificer")
}

func (s *PureLoadTestSuite) TestLegacyLoadDropsAFeatureFromAnotherModule() {
	data := fullSheet(&s.Suite)
	data.Features = append(data.Features, json.RawMessage(`{"ref":{"module":"artificer","type":"features","id":"infusion"}}`))

	char, err := LoadFromData(s.ctx, data, events.NewEventBus())

	s.Require().NoError(err)
	s.Require().Len(char.ToData().Features, 1, "the foreign feature is silently dropped")
}

// An inventory item the catalog does not know disappears from the sheet on the
// legacy path, which is how an item goes missing between two saves.
func (s *PureLoadTestSuite) TestStrictLoadRefusesAnUnknownInventoryItem() {
	data := fullSheet(&s.Suite)
	data.Inventory = append(data.Inventory, InventoryItemData{ID: "vorpal-spork", Quantity: 1})

	_, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "vorpal-spork")
}

func (s *PureLoadTestSuite) TestStrictLoadRejectsNonpositiveInventoryQuantity() {
	for _, quantity := range []int{0, -1} {
		s.Run(fmt.Sprintf("quantity %d", quantity), func() {
			data := fullSheet(&s.Suite)
			data.Inventory = append(data.Inventory, InventoryItemData{
				ID:       string(weapons.Handaxe),
				Quantity: quantity,
			})

			loaded, err := Load(s.ctx, data)

			s.Require().Error(err)
			s.Nil(loaded)
			s.Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
			s.Contains(err.Error(), `inventory item 1 ("handaxe")`,
				"the error must identify the malformed row by index and item")
			s.Equal(map[string]any{
				"index":    1,
				"item_id":  "handaxe",
				"quantity": quantity,
			}, rpgerr.GetMeta(err))
		})
	}
}

func (s *PureLoadTestSuite) TestLenientLoadWarnsAndDropsNonpositiveInventoryQuantity() {
	for _, quantity := range []int{0, -1} {
		s.Run(fmt.Sprintf("quantity %d", quantity), func() {
			logs := captureWarnings(s.T())
			data := fullSheet(&s.Suite)
			data.Inventory = append(data.Inventory, InventoryItemData{
				ID:       string(weapons.Handaxe),
				Quantity: quantity,
			})

			loaded, err := LoadFromData(s.ctx, data, events.NewEventBus())

			s.Require().NoError(err)
			s.Require().Len(loaded.ToData().Inventory, 1,
				"the malformed row is dropped rather than defaulted to one")
			s.Require().Len(logs.records, 1)
			got := attrs(logs.records[0])
			s.Equal("inventory item", got["dropped"])
			s.Equal("1", got["index"])
			s.Equal("handaxe", got["item"])
			s.Equalf(fmt.Sprint(quantity), got["quantity"],
				"the warning must preserve the malformed persisted value")
		})
	}
}

func (s *PureLoadTestSuite) TestLoadRejectsNilData() {
	_, err := Load(s.ctx, nil)

	s.Require().Error(err)
}

func (s *PureLoadTestSuite) TestStrictLoadRejectsPersistedOwnerResourceBoundsBeforeStatusView() {
	tests := []struct {
		name string
		data RecoverableResourceData
	}{
		{name: "negative current", data: RecoverableResourceData{Current: -1, Maximum: 3}},
		{name: "negative maximum", data: RecoverableResourceData{Current: 0, Maximum: -1}},
		{name: "current above maximum", data: RecoverableResourceData{Current: 4, Maximum: 3}},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			data := &Data{
				ID: "fighter-bounds", ClassID: classes.Fighter,
				Resources: map[coreResources.ResourceKey]RecoverableResourceData{resources.HitDice: tc.data},
			}
			loaded, err := Load(s.ctx, data)
			s.Require().Error(err)
			s.Nil(loaded, "strict load must reject before a StatusView can expose normalized counts")
			s.Contains(err.Error(), string(resources.HitDice))
		})
	}
}

func (s *PureLoadTestSuite) TestLenientLoadDropsMalformedOwnerResourcesWithoutNormalizingThem() {
	tests := []RecoverableResourceData{
		{Current: -1, Maximum: 3},
		{Current: 0, Maximum: -1},
		{Current: 4, Maximum: 3},
	}
	for _, malformed := range tests {
		data := &Data{
			ID: "fighter-lenient-bounds", ClassID: classes.Fighter,
			Resources: map[coreResources.ResourceKey]RecoverableResourceData{
				resources.HitDice: malformed,
			},
		}
		loaded, err := LoadFromData(s.ctx, data, events.NewEventBus())
		s.Require().NoError(err)

		out, err := loaded.StatusView(&StatusViewInput{})
		s.Require().NoError(err)
		s.Require().NotNil(out)
		s.Empty(out.View.Resources,
			"malformed persisted counts are dropped, never clamped into a valid-looking row")
		s.Empty(loaded.ToData().Resources)
	}
}

func (s *PureLoadTestSuite) TestStrictLoadRejectsFeaturePrivateResourceBoundsBeforeStatusView() {
	tests := []struct {
		name string
		blob func() json.RawMessage
	}{
		{
			name: "second wind negative uses",
			blob: func() json.RawMessage {
				raw, err := json.Marshal(features.SecondWindData{
					Ref: refs.Features.SecondWind(), ID: "second_wind", Name: "Second Wind",
					CharacterID: "fighter-private", Uses: -1, MaxUses: 1,
				})
				s.Require().NoError(err)
				return raw
			},
		},
		{
			name: "action surge uses above maximum",
			blob: func() json.RawMessage {
				raw, err := json.Marshal(features.ActionSurgeData{
					Ref: refs.Features.ActionSurge(), ID: "action_surge", Name: "Action Surge",
					CharacterID: "fighter-private", Uses: 2, MaxUses: 1,
				})
				s.Require().NoError(err)
				return raw
			},
		},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			data := &Data{ID: "fighter-private", ClassID: classes.Fighter, Features: []json.RawMessage{tc.blob()}}
			loaded, err := Load(s.ctx, data)
			s.Require().Error(err)
			s.Nil(loaded, "strict load must reject before malformed private uses reach StatusView")
		})
	}
}

func (s *PureLoadTestSuite) TestLenientLoadDropsMalformedFeaturePrivateResource() {
	raw, err := json.Marshal(features.SecondWindData{
		Ref: refs.Features.SecondWind(), ID: "second_wind", Name: "Second Wind",
		CharacterID: "fighter-private-lenient", Uses: 2, MaxUses: 1,
	})
	s.Require().NoError(err)
	loaded, err := LoadFromData(s.ctx, &Data{
		ID: "fighter-private-lenient", ClassID: classes.Fighter, Features: []json.RawMessage{raw},
	}, events.NewEventBus())
	s.Require().NoError(err)

	out, err := loaded.StatusView(&StatusViewInput{})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Empty(out.View.Features)
	s.Empty(out.View.Resources)
	s.Empty(loaded.ToData().Features, "existing lenient policy drops the malformed feature blob")
}

func (s *PureLoadTestSuite) TestStrictLoadPreservesCharacterMetadata() {
	data := fullSheet(&s.Suite)
	data.BackgroundID = backgrounds.Soldier
	data.CreatedAt = time.Date(2026, 8, 14, 12, 0, 0, 123456789, time.FixedZone("seed", -7*60*60))
	data.UpdatedAt = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	beforeWrite := time.Now()

	char, err := Load(s.ctx, data)
	s.Require().NoError(err)

	out := char.ToData()
	s.Equal(backgrounds.Soldier, out.BackgroundID)
	s.Equal(data.CreatedAt, out.CreatedAt, "CreatedAt must preserve the exact persisted instant and location")
	s.False(out.UpdatedAt.Before(beforeWrite), "UpdatedAt must be freshly generated by ToData")
}

// TestWalletRoundTripsAndDefaultsToZero pins Wave 3's whole scope: the
// field beside Inventory (rpg-toolkit#1532) round-trips through Load/ToData
// exactly, and a character with no persisted wallet loads as Money{} — broke
// by zero value, not a distinct "never had one" state, matching every other
// numeric field on this struct.
func (s *PureLoadTestSuite) TestWalletRoundTripsAndDefaultsToZero() {
	data := fullSheet(&s.Suite)
	data.Wallet = currency.FromGold(37).Add(currency.FromSilver(4))

	char, err := Load(s.ctx, data)
	s.Require().NoError(err)
	s.Equal(data.Wallet, char.ToData().Wallet)

	data.Wallet = currency.Money{}
	char, err = Load(s.ctx, data)
	s.Require().NoError(err)
	s.Equal(currency.Money{}, char.ToData().Wallet)
}

func (s *PureLoadTestSuite) TestLenientLoadPreservesCharacterMetadata() {
	data := fullSheet(&s.Suite)
	data.BackgroundID = backgrounds.Soldier
	data.CreatedAt = time.Date(2026, 8, 14, 12, 0, 0, 123456789, time.FixedZone("seed", -7*60*60))

	char, err := LoadFromData(s.ctx, data, events.NewEventBus())
	s.Require().NoError(err)

	out := char.ToData()
	s.Equal(backgrounds.Soldier, out.BackgroundID)
	s.Equal(data.CreatedAt, out.CreatedAt)
}

// A character loaded strictly and then attached is a character loaded the old
// way: the compatibility claim runs in both directions, so the recomposition
// cannot drift from the path it replaced.
func (s *PureLoadTestSuite) TestLoadThenAttachMatchesLoadFromData() {
	pure, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, pure, events.NewEventBus()))

	legacy, err := LoadFromData(s.ctx, fullSheet(&s.Suite), events.NewEventBus())
	s.Require().NoError(err)

	s.Require().Equal(marshalData(&s.Suite, legacy.ToData()), marshalData(&s.Suite, pure.ToData()))
}

// Attach is what applies the conditions and attachable features a load parsed;
// character-owned resources stay under the Character rest verbs.
func (s *PureLoadTestSuite) TestAttachAppliesWhatLoadParsed() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)

	s.Require().NoError(Attach(s.ctx, char, events.NewEventBus()))

	s.Require().Len(char.GetConditions(), 1)
	s.Require().True(char.GetConditions()[0].IsApplied())
	s.Require().False(char.GetResource(resources.RageCharges).IsApplied(),
		"Attach does not apply character-owned resources")
	s.Require().NotEmpty(char.subscriptionIDs, "the sheet keeper subscribed")
}

// The load's pending effects are drained, not read: what has been put on a bus
// is not waiting to be put on another one.
func (s *PureLoadTestSuite) TestAttachDrainsWhatItApplied() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)

	s.Require().NoError(Attach(s.ctx, char, events.NewEventBus()))

	s.Require().Empty(char.pendingEffects)
	s.Require().Len(char.GetConditions(), 1, "draining does not take the conditions off the sheet")
}

// Attaching the same sheet twice is refused rather than silently doubling every
// handler it has.
func (s *PureLoadTestSuite) TestAttachingTwiceIsRefused() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, char, events.NewEventBus()))

	s.Require().Error(Attach(s.ctx, char, events.NewEventBus()))
}

func (s *PureLoadTestSuite) TestAttachRejectsANilBus() {
	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)

	s.Require().Error(Attach(s.ctx, char, nil))
}

// Attribution, pinned: every condition goes on through the bus scoped to the
// ref its loader routed on, and the ref comes from the load. A ConditionBehavior
// cannot name itself, so if this pairing is not carried from load to attach,
// per-effect attribution is not recoverable later.
func (s *PureLoadTestSuite) TestAttachScopesEachConditionToItsRef() {
	bus := newRecordingBus()

	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, char, bus))

	s.Require().Equal(
		[]core.Ref{*refs.Conditions.Raging(), *refs.Conditions.OpportunityAttack()},
		bus.record.asked, "the sheet's own condition, then the carried reaction")
	s.Require().Contains(bus.record.byRef[*refs.Conditions.Raging()], events.Topic("dnd5e.saves.chain"))
}

// Attachable features use the same attribution seam as conditions. The feature
// names itself directly, so Attach scopes its lifecycle with Feature.Ref and
// does not infer identity from its persistence bytes.
func (s *PureLoadTestSuite) TestAttachScopesAttachableFeatureToItsRef() {
	secondWind, err := json.Marshal(features.SecondWindData{
		Ref: refs.Features.SecondWind(), ID: "scoped-second-wind", Name: "Second Wind",
		CharacterID: "scoped-fighter", Uses: 0, MaxUses: 1,
	})
	s.Require().NoError(err)
	bus := newRecordingBus()

	char, err := Load(s.ctx, &Data{
		ID: "scoped-fighter", ClassID: classes.Fighter, Features: []json.RawMessage{secondWind},
	})
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, char, bus))

	s.Require().Equal(
		[]core.Ref{*refs.Features.SecondWind(), *refs.Conditions.OpportunityAttack()},
		bus.record.asked, "the feature attaches before the carried reaction")
	s.Require().Contains(bus.record.byRef[*refs.Features.SecondWind()], events.Topic("dnd5e.rest"))
}

// The sheet keeper's own hooks are not laundered into an effect's name: they
// land unattributed, where a registration ledger can tell them apart from what
// an effect subscribed.
func (s *PureLoadTestSuite) TestKeeperHooksAreNotAttributedToAnEffect() {
	bus := newRecordingBus()

	char, err := Load(s.ctx, fullSheet(&s.Suite))
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, char, bus))

	s.Require().NotEmpty(bus.record.byRef[unattributed])
	s.Require().NotContains(bus.record.byRef[unattributed], events.Topic("dnd5e.saves.chain"))
}

func TestPureLoadSuite(t *testing.T) {
	suite.Run(t, new(PureLoadTestSuite))
}
