package npcs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/stretchr/testify/suite"
)

type VendorSuite struct {
	suite.Suite
}

func TestVendorSuite(t *testing.T) {
	suite.Run(t, new(VendorSuite))
}

func (s *VendorSuite) vendorConfig() npcs.VendorConfig {
	return npcs.VendorConfig{
		NPC: npc.Config{
			Ref:         refs.NPCs.Vendor(),
			DisplayName: "Blacksmith",
			Capabilities: []npc.Capability{
				npc.CapabilityVendor,
			},
		},
		Inventory: npcs.VendorInventoryConfig{Entries: blacksmithStock()},
	}
}

func blacksmithStock() []npcs.StockEntryData {
	return []npcs.StockEntryData{
		limitedWeapon(weapons.Longsword),
		limitedWeapon(weapons.Greatsword),
		limitedWeapon(weapons.Longbow),
		{
			Type: shared.EquipmentTypeAmmunition,
			ID:   string(ammunition.Arrows20),
			Availability: npcs.Availability{
				Mode: npcs.StockModeUnlimited,
			},
		},
	}
}

func limitedWeapon(id weapons.WeaponID) npcs.StockEntryData {
	return npcs.StockEntryData{
		Type: shared.EquipmentTypeWeapon,
		ID:   string(id),
		Availability: npcs.Availability{
			Mode:     npcs.StockModeLimited,
			Quantity: 1,
		},
	}
}

func (s *VendorSuite) TestNewVendorBuildsConfiguredVendor() {
	vendor, err := npcs.NewVendor(s.vendorConfig())

	s.Require().NoError(err)
	base := vendor.NPC()
	s.Require().NotNil(base)
	s.Equal(refs.NPCs.Vendor().String(), base.Ref().String())
	s.Equal("Blacksmith", base.DisplayName())
	s.Equal([]npc.Capability{npc.CapabilityVendor}, base.Capabilities())
	s.Equal(npc.CombatPolicyNonCombatant, base.CombatPolicy())
	s.Equal(npc.ObservationPolicySubjectOnly, base.ObservationPolicy())
	s.Equal(npc.DispositionPolicyNeutral, base.DispositionPolicy())
	s.Equal(npc.MovementPolicyBlocking, base.MovementPolicy())
}

func (s *VendorSuite) TestNewVendorAddsVendorCapability() {
	config := s.vendorConfig()
	config.NPC.Capabilities = nil

	vendor, err := npcs.NewVendor(config)

	s.Require().NoError(err)
	s.Equal([]npc.Capability{npc.CapabilityVendor}, vendor.NPC().Capabilities())
}

func (s *VendorSuite) TestLoadVendorAddsVendorCapability() {
	vendor, err := npcs.NewVendor(s.vendorConfig())
	s.Require().NoError(err)
	data := vendor.ToData()
	data.NPC.Capabilities = nil

	loaded, err := npcs.LoadVendor(data)

	s.Require().NoError(err)
	s.Equal([]npc.Capability{npc.CapabilityVendor}, loaded.NPC().Capabilities())
}

func (s *VendorSuite) TestVendorDataRoundTrips() {
	vendor, err := npcs.NewVendor(s.vendorConfig())
	s.Require().NoError(err)

	loaded, err := npcs.LoadVendor(vendor.ToData())

	s.Require().NoError(err)
	s.Equal(vendor.NPC().Ref().String(), loaded.NPC().Ref().String())
	s.Equal(vendor.NPC().DisplayName(), loaded.NPC().DisplayName())
	s.Equal(vendor.View(), loaded.View())
}

func (s *VendorSuite) TestLoadVendorRequiresData() {
	_, err := npcs.LoadVendor(nil)
	s.ErrorIs(err, npcs.ErrNoVendorData)

	_, err = npcs.LoadVendor(&npcs.VendorData{})
	s.ErrorIs(err, npcs.ErrNoVendorNPC)

	_, err = npcs.LoadVendor(&npcs.VendorData{NPC: s.mustNPCData()})
	s.ErrorIs(err, npcs.ErrNoInventory)
}

func (s *VendorSuite) TestInventoryValidation() {
	_, err := npcs.NewVendorInventory(npcs.VendorInventoryConfig{})
	s.ErrorIs(err, npcs.ErrNoStock)

	tests := []struct {
		name string
		edit func(*npcs.StockEntryData)
		want error
	}{
		{
			name: "missing type",
			edit: func(data *npcs.StockEntryData) {
				data.Type = ""
			},
			want: npcs.ErrNoEquipmentType,
		},
		{
			name: "missing id",
			edit: func(data *npcs.StockEntryData) {
				data.ID = ""
			},
			want: npcs.ErrNoEquipmentID,
		},
		{
			name: "missing stock mode",
			edit: func(data *npcs.StockEntryData) {
				data.Availability.Mode = ""
			},
			want: npcs.ErrNoStockMode,
		},
		{
			name: "unknown stock mode",
			edit: func(data *npcs.StockEntryData) {
				data.Availability.Mode = "seasonal"
			},
			want: npcs.ErrUnknownStockMode,
		},
		{
			name: "limited stock without quantity",
			edit: func(data *npcs.StockEntryData) {
				data.Availability.Quantity = 0
			},
			want: npcs.ErrInvalidStockQuantity,
		},
		{
			name: "unknown equipment",
			edit: func(data *npcs.StockEntryData) {
				data.ID = "missing-sword"
			},
			want: npcs.ErrEquipmentNotFound,
		},
		{
			name: "type mismatch",
			edit: func(data *npcs.StockEntryData) {
				data.Type = shared.EquipmentTypeArmor
			},
			want: npcs.ErrEquipmentTypeMismatch,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			entry := limitedWeapon(weapons.Longsword)
			tt.edit(&entry)

			_, err := npcs.NewVendorInventory(npcs.VendorInventoryConfig{
				Entries: []npcs.StockEntryData{entry},
			})

			s.ErrorIs(err, tt.want)
		})
	}
}

func (s *VendorSuite) TestInventoryResolvesEquipmentAndNormalizesUnlimitedQuantity() {
	inventory, err := npcs.NewVendorInventory(npcs.VendorInventoryConfig{
		Entries: []npcs.StockEntryData{{
			Type: shared.EquipmentTypeAmmunition,
			ID:   string(ammunition.Arrows20),
			Availability: npcs.Availability{
				Mode:     npcs.StockModeUnlimited,
				Quantity: 99,
			},
		}},
	})

	s.Require().NoError(err)
	entries := inventory.Entries()
	s.Require().Len(entries, 1)
	s.Equal("Arrows (20)", entries[0].Equipment().EquipmentName())
	s.Equal(npcs.Availability{Mode: npcs.StockModeUnlimited}, entries[0].Availability())
	s.Equal(npcs.Availability{Mode: npcs.StockModeUnlimited}, inventory.ToData().Entries[0].Availability)
}

func (s *VendorSuite) TestVendorViewUsesResolvedEquipmentWithoutPrices() {
	vendor, err := npcs.NewVendor(s.vendorConfig())
	s.Require().NoError(err)

	view := vendor.View()

	s.Equal(refs.NPCs.Vendor().String(), view.Ref.String())
	s.Equal("Blacksmith", view.DisplayName)
	s.Equal([]npcs.StockEntryView{
		{
			Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword),
			Name: "Longsword", Mode: npcs.StockModeLimited, Quantity: 1,
		},
		{
			Type: shared.EquipmentTypeWeapon, ID: string(weapons.Greatsword),
			Name: "Greatsword", Mode: npcs.StockModeLimited, Quantity: 1,
		},
		{
			Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longbow),
			Name: "Longbow", Mode: npcs.StockModeLimited, Quantity: 1,
		},
		{
			Type: shared.EquipmentTypeAmmunition, ID: string(ammunition.Arrows20),
			Name: "Arrows (20)", Mode: npcs.StockModeUnlimited,
		},
	}, view.Inventory.Entries)
}

func (s *VendorSuite) TestAccessorsAreCopyOut() {
	vendor, err := npcs.NewVendor(s.vendorConfig())
	s.Require().NoError(err)

	base := vendor.NPC()
	baseData := base.ToData()
	baseData.DisplayName = "changed"

	inventory := vendor.Inventory()
	entries := inventory.Entries()
	entries[0] = entries[1]

	data := vendor.ToData()
	data.Inventory.Entries[0].ID = string(weapons.Dagger)

	s.Equal("Blacksmith", vendor.NPC().DisplayName())
	s.Equal(string(weapons.Longsword), vendor.Inventory().ToData().Entries[0].ID)
}

func (s *VendorSuite) TestVendorScenarioDeclaresVisibleWorldEntity() {
	vendor, err := npcs.NewVendor(s.vendorConfig())
	s.Require().NoError(err)

	scenario, err := npcs.VendorScenario(npcs.VendorScenarioConfig{
		EntityID:   "ashford-blacksmith",
		Vendor:     vendor,
		Membership: graph.Relation("belongs-to"),
	})

	s.Require().NoError(err)
	w, err := world.New(world.Config{
		Scenario: scenario,
		Resolver: fixedResolver{},
		Witness:  emptyWitness{},
	})
	s.Require().NoError(err)
	s.True(w.Truth().Visible("ashford-blacksmith"))
	s.True(w.View("observer").Visible("ashford-blacksmith"))
}

func (s *VendorSuite) TestVendorScenarioValidation() {
	vendor, err := npcs.NewVendor(s.vendorConfig())
	s.Require().NoError(err)

	_, err = npcs.VendorScenario(npcs.VendorScenarioConfig{
		Vendor:     vendor,
		Membership: graph.Relation("belongs-to"),
	})
	s.ErrorIs(err, npcs.ErrNoWorldEntity)

	_, err = npcs.VendorScenario(npcs.VendorScenarioConfig{
		EntityID:   "ashford-blacksmith",
		Membership: graph.Relation("belongs-to"),
	})
	s.ErrorIs(err, npcs.ErrNoWorldVendor)

	_, err = npcs.VendorScenario(npcs.VendorScenarioConfig{
		EntityID:   "ashford-blacksmith",
		Vendor:     &npcs.Vendor{},
		Membership: graph.Relation("belongs-to"),
	})
	s.ErrorIs(err, npcs.ErrNoWorldVendor)

	_, err = npcs.VendorScenario(npcs.VendorScenarioConfig{
		EntityID: "ashford-blacksmith",
		Vendor:   vendor,
	})
	s.ErrorIs(err, npcs.ErrNoWorldMembership)
}

func (s *VendorSuite) mustNPCData() *npc.Data {
	base, err := npc.New(s.vendorConfig().NPC)
	s.Require().NoError(err)
	return base.ToData()
}

type fixedResolver struct{}

func (fixedResolver) Resolve(context.Context, world.Attempt) (journal.Outcome, error) {
	return journal.Outcome{}, errors.New("unused")
}

type emptyWitness struct{}

func (emptyWitness) Bystanders(
	context.Context,
	journal.EntityID,
	journal.EntityID,
	world.Witnessing,
) ([]journal.EntityID, error) {
	return nil, nil
}
