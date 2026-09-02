package npcs_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

type MerchantSuite struct {
	suite.Suite
}

func TestMerchantSuite(t *testing.T) {
	suite.Run(t, new(MerchantSuite))
}

// TestNilConfigReturnsAValidatedDemoMerchant pins shape B: nil is the
// caller's explicit request for the toolkit's own default, not something a
// caller can trigger by accident with a zero-value struct.
func (s *MerchantSuite) TestNilConfigReturnsAValidatedDemoMerchant() {
	vendor, err := npcs.NewMerchant(nil)

	s.Require().NoError(err)
	base := vendor.NPC()
	s.Require().NotNil(base)
	s.Equal(refs.NPCs.Merchant().String(), base.Ref().String())
	s.Equal("Demo Merchant", base.DisplayName())
	s.Equal([]npc.Capability{npc.CapabilityVendor}, base.Capabilities())
	s.Equal(npc.CombatPolicyNonCombatant, base.CombatPolicy())

	entries := vendor.Inventory().Entries()
	s.Require().Len(entries, 3)
	s.Equal(string(weapons.Longsword), entries[0].Equipment().EquipmentID())
	s.Equal(shared.EquipmentTypeWeapon, entries[0].Equipment().EquipmentType())
	s.Equal(string(weapons.Longbow), entries[1].Equipment().EquipmentID())
	s.Equal(string(ammunition.Arrows20), entries[2].Equipment().EquipmentID())
	s.Equal(npcs.Availability{Mode: npcs.StockModeUnlimited}, entries[2].Availability())
}

func (s *MerchantSuite) explicitConfig() npcs.MerchantConfig {
	return npcs.MerchantConfig{
		NPC: npc.Config{
			Ref:         refs.NPCs.Vendor(),
			DisplayName: "Ashford Trading Post",
			Capabilities: []npc.Capability{
				npc.CapabilityVendor,
			},
		},
		Inventory: npcs.VendorInventoryConfig{Entries: blacksmithStock()},
	}
}

// TestNonNilConfigBuildsExactlyWhatWasAsked proves the general path stays
// caller-authored — NewMerchant(config) does not fold in any demo default
// when a real config is given.
func (s *MerchantSuite) TestNonNilConfigBuildsExactlyWhatWasAsked() {
	config := s.explicitConfig()

	vendor, err := npcs.NewMerchant(&config)

	s.Require().NoError(err)
	base := vendor.NPC()
	s.Equal(refs.NPCs.Vendor().String(), base.Ref().String())
	s.Equal("Ashford Trading Post", base.DisplayName())
	s.NotEqual("Demo Merchant", base.DisplayName())
}

// TestNonNilConfigMissingARequiredFieldStillErrors is the load-bearing test:
// an incomplete explicit config must be rejected, never silently completed
// into the demo. The nil/non-nil distinction only means anything if this
// holds.
func (s *MerchantSuite) TestNonNilConfigMissingARequiredFieldStillErrors() {
	config := s.explicitConfig()
	config.NPC.Ref = nil

	_, err := npcs.NewMerchant(&config)

	s.Require().Error(err)
	s.NotContains(err.Error(), "Demo Merchant")
}
